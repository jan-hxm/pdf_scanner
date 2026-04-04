package search

import (
	"context"
	"html"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/gen2brain/go-fitz"
	"github.com/hbollon/go-edlib"
)

// Rect represents a bounding box for a text match position on a PDF page.
type Rect struct {
	X0 float64 `json:"x0"`
	Y0 float64 `json:"y0"`
	X1 float64 `json:"x1"`
	Y1 float64 `json:"y1"`
}

// SearchResult mirrors the Python reader.py JSON output shape exactly.
type SearchResult struct {
	File       string  `json:"file"`
	FileName   string  `json:"fileName"`
	Page       int     `json:"page"`
	Found      bool    `json:"found"`
	Match      string  `json:"match"`
	Confidence int     `json:"confidence"`
	FoundWord  string  `json:"foundWord"`
	Positions  []Rect  `json:"positions"`
	Context    string  `json:"context"`
}

var (
	reNormalize  = regexp.MustCompile(`\s+|\.|,`)
	reSciStep1   = regexp.MustCompile(`([a-z])\.\s*([a-z])`)
	reSciStep2   = regexp.MustCompile(`[\s.]+`)
	reBasicWords = regexp.MustCompile(`\b[A-Za-z]+(?:['\.\-]\w+)*\b`)
	reScientific = regexp.MustCompile(`\b[A-Za-z]\.\s?[A-Za-z][a-z]+\b`)
	rePhrases    = regexp.MustCompile(`\b(?:\w+[\s\.\-])+\w+\b`)
	reAbbrev     = regexp.MustCompile(`\b(?:[A-Za-z]\.){1,}[A-Za-z]?[a-z]*\b`)
	reSciNotation = regexp.MustCompile(`[A-Za-z]\.\s?[A-Za-z]`)
	reSciNoSpace  = regexp.MustCompile(`([A-Za-z])\.\s+([A-Za-z])`)
	reSciNoPeriod = regexp.MustCompile(`([A-Za-z])\.\s*([A-Za-z])`)
)

func normalizeText(text string) string {
	s := strings.ToLower(strings.TrimSpace(text))
	return reNormalize.ReplaceAllString(s, "")
}

func normalizeScientificName(text string) string {
	s := strings.ToLower(strings.TrimSpace(text))
	s = reSciStep1.ReplaceAllString(s, "${1}${2}")
	s = reSciStep2.ReplaceAllString(s, "")
	return s
}

func tokenizeText(line string) []string {
	words := reBasicWords.FindAllString(line, -1)
	scientific := reScientific.FindAllString(line, -1)
	phrases := rePhrases.FindAllString(line, -1)
	abbreviations := reAbbrev.FindAllString(line, -1)
	result := make([]string, 0, len(words)+len(scientific)+len(phrases)+len(abbreviations))
	result = append(result, words...)
	result = append(result, scientific...)
	result = append(result, phrases...)
	result = append(result, abbreviations...)
	return result
}

func generateNgrams(text string, n int) []string {
	runes := []rune(text)
	if len(runes) < n {
		return []string{text}
	}
	ngrams := make([]string, 0, len(runes)-n+1)
	for i := 0; i <= len(runes)-n; i++ {
		ngrams = append(ngrams, string(runes[i:i+n]))
	}
	return ngrams
}

func ngramSimilarity(text1, text2 string, n int) float64 {
	norm1 := normalizeScientificName(text1)
	norm2 := normalizeScientificName(text2)

	ngrams1 := generateNgrams(norm1, n)
	ngrams2 := generateNgrams(norm2, n)

	if len(ngrams2) == 0 {
		return 0
	}

	set1 := make(map[string]struct{}, len(ngrams1))
	for _, ng := range ngrams1 {
		set1[ng] = struct{}{}
	}
	set2 := make(map[string]struct{}, len(ngrams2))
	for _, ng := range ngrams2 {
		set2[ng] = struct{}{}
	}

	intersection := 0
	for ng := range set1 {
		if _, ok := set2[ng]; ok {
			intersection++
		}
	}
	union := len(set1) + len(set2) - intersection
	if union == 0 {
		return 0
	}
	return float64(intersection) / float64(union) * 100
}

func fuzzyRatio(s1, s2 string) float64 {
	if s1 == "" && s2 == "" {
		return 100
	}
	if s1 == "" || s2 == "" {
		return 0
	}
	sim, err := edlib.StringsSimilarity(s1, s2, edlib.Levenshtein)
	if err != nil {
		return 0
	}
	return float64(sim) * 100
}

func fuzzyPartialRatio(s1, s2 string) float64 {
	r1 := []rune(s1)
	r2 := []rune(s2)
	if len(r1) >= len(r2) {
		return fuzzyRatio(s1, s2)
	}
	best := 0.0
	for i := 0; i <= len(r2)-len(r1); i++ {
		sub := string(r2[i : i+len(r1)])
		if score := fuzzyRatio(s1, sub); score > best {
			best = score
		}
	}
	return best
}

func advancedMatch(line, keyword string) (float64, string) {
	lineLower := strings.ToLower(line)
	keyLower := strings.ToLower(keyword)

	// Strategy 1: Exact match (confidence 100)
	if strings.Contains(lineLower, keyLower) {
		re := regexp.MustCompile(`(?i)` + regexp.QuoteMeta(keyword))
		if m := re.FindString(line); m != "" {
			return 100, m
		}
	}

	// Strategy 2: Scientific name normalized match (confidence 95)
	normLine := normalizeScientificName(line)
	normKeyword := normalizeScientificName(keyword)
	if strings.Contains(normLine, normKeyword) {
		parts := strings.ReplaceAll(strings.ReplaceAll(keyword, ".", ""), " ", "")
		patternStr := ""
		for _, c := range parts {
			cl := strings.ToLower(string(c))
			cu := strings.ToUpper(string(c))
			patternStr += `[` + cl + cu + `]\.?\s*`
		}
		// trim trailing \s*
		if len(patternStr) > 4 {
			patternStr = patternStr[:len(patternStr)-4]
		}
		re, err := regexp.Compile(patternStr)
		if err == nil {
			for _, m := range re.FindAllString(line, -1) {
				if normalizeScientificName(m) == normKeyword {
					return 95, m
				}
			}
		}
	}

	// Strategy 3: Tokenized match (confidence 90)
	words := tokenizeText(line)
	for _, word := range words {
		if normalizeScientificName(word) == normKeyword {
			return 90, word
		}
	}

	// Strategy 4: Token-based fuzzy matching
	bestScore := 0.0
	var bestMatch string

	for _, word := range words {
		if len([]rune(word)) > 2 {
			normWord := normalizeScientificName(word)
			ratio := fuzzyRatio(normWord, normKeyword)
			partial := fuzzyPartialRatio(normWord, normKeyword)
			score := math.Max(ratio, partial)
			if score > bestScore {
				bestScore = score
				bestMatch = word
			}
		}
	}

	// Strategy 5: N-gram similarity fallback
	ngramScore := ngramSimilarity(line, keyword, 3)
	if ngramScore > bestScore {
		tokens := strings.Fields(line)
		var bestNgramMatch string
		bestNgramScore := 0.0
		for i := 0; i < len(tokens); i++ {
			maxSpan := 4
			if len(tokens)-i < maxSpan {
				maxSpan = len(tokens) - i
			}
			for span := 1; span < maxSpan; span++ {
				phrase := strings.Join(tokens[i:i+span], " ")
				score := ngramSimilarity(phrase, keyword, 3)
				if score > bestNgramScore {
					bestNgramScore = score
					bestNgramMatch = phrase
				}
			}
		}
		bestScore = ngramScore
		if bestNgramMatch != "" {
			bestMatch = bestNgramMatch
		}
	}

	return bestScore, bestMatch
}

// reHTMLParagraph matches MuPDF's stext HTML output:
// <p style="position:absolute;white-space:pre;top:Y.Ypt;left:X.Xpt;">...<span ...>text</span>...</p>
var reHTMLParagraph = regexp.MustCompile(`<p style="position:absolute;white-space:pre;top:([\d.]+)pt;left:([\d.]+)pt;">([\s\S]*?)</p>`)
var reHTMLSpan = regexp.MustCompile(`<span style="[^"]*font-size:([\d.]+)pt[^"]*">([^<]*)</span>`)
var reStripTags = regexp.MustCompile(`<[^>]+>`)

// findPositionsInHTML parses MuPDF's HTML output to find approximate bounding boxes
// for lines containing the search term. Coordinates are in PDF user units (pt).
func findPositionsInHTML(pageHTML, term string) []Rect {
	termLower := strings.ToLower(html.UnescapeString(term))
	var results []Rect

	for _, pMatch := range reHTMLParagraph.FindAllStringSubmatch(pageHTML, -1) {
		top, err1 := strconv.ParseFloat(pMatch[1], 64)
		left, err2 := strconv.ParseFloat(pMatch[2], 64)
		if err1 != nil || err2 != nil {
			continue
		}
		innerHTML := pMatch[3]

		// Build plain text and extract font size from spans
		fontSize := 12.0
		lineText := ""
		for _, spanMatch := range reHTMLSpan.FindAllStringSubmatch(innerHTML, -1) {
			if sz, err := strconv.ParseFloat(spanMatch[1], 64); err == nil {
				fontSize = sz
			}
			lineText += html.UnescapeString(spanMatch[2])
		}
		if lineText == "" {
			lineText = html.UnescapeString(reStripTags.ReplaceAllString(innerHTML, ""))
		}

		lineTextLower := strings.ToLower(lineText)
		idx := strings.Index(lineTextLower, termLower)
		if idx < 0 {
			continue
		}

		// Estimate character width: average ~0.5× font-size for Latin fonts
		charWidth := fontSize * 0.5
		runesBefore := utf8.RuneCountInString(lineText[:idx])
		runesTerm := utf8.RuneCountInString(term)

		results = append(results, Rect{
			X0: left + float64(runesBefore)*charWidth,
			Y0: top,
			X1: left + float64(runesBefore+runesTerm)*charWidth,
			Y1: top + fontSize*1.2,
		})
	}
	return results
}

func processPDF(filePath, keyword string) []SearchResult {
	var results []SearchResult
	type seenKey struct {
		page int
		word string
	}
	seen := make(map[seenKey]struct{})

	doc, err := fitz.New(filePath)
	if err != nil {
		return results
	}
	defer doc.Close()

	fileName := filepath.Base(filePath)

	for pageNum := 0; pageNum < doc.NumPage(); pageNum++ {
		text, err := doc.Text(pageNum)
		if err != nil {
			continue
		}

		// Fetch HTML once per page for position extraction
		pageHTML, _ := doc.HTML(pageNum, false)

		lines := strings.Split(text, "\n")
		for _, line := range lines {
			if len([]rune(line)) <= 1 {
				continue
			}

			confidence, foundWord := advancedMatch(line, keyword)
			if confidence < 85 {
				continue
			}

			key := seenKey{pageNum + 1, foundWord}
			if _, exists := seen[key]; exists {
				continue
			}
			seen[key] = struct{}{}

			var positions []Rect

			if foundWord != "" {
				searchTerms := []string{foundWord}
				if reSciNotation.MatchString(foundWord) {
					noSpace := reSciNoSpace.ReplaceAllString(foundWord, "${1}.${2}")
					noPeriod := reSciNoPeriod.ReplaceAllString(foundWord, "${1} ${2}")
					searchTerms = append(searchTerms, noSpace, noPeriod)
				}

				for _, term := range searchTerms {
					positions = findPositionsInHTML(pageHTML, term)
					if len(positions) > 0 {
						break
					}
				}

				// Fallback: try individual words
				if len(positions) == 0 {
					for _, word := range strings.Fields(foundWord) {
						if len([]rune(word)) > 2 {
							if rects := findPositionsInHTML(pageHTML, word); len(rects) > 0 {
								positions = append(positions, rects...)
							}
						}
					}
				}
			}

			if positions == nil {
				positions = []Rect{}
			}

			results = append(results, SearchResult{
				File:       filePath,
				FileName:   fileName,
				Page:       pageNum + 1,
				Found:      true,
				Match:      "content",
				Confidence: int(math.Round(confidence)),
				FoundWord:  foundWord,
				Positions:  positions,
				Context:    strings.TrimSpace(line),
			})
		}
	}

	return results
}

// SearchPDFs searches all PDFs in folder for keyword, calling progressCb with progress 0–100.
func SearchPDFs(ctx context.Context, folder, keyword string, progressCb func(int)) ([]SearchResult, error) {
	var pdfFiles []string
	err := filepath.Walk(folder, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() && strings.ToLower(filepath.Ext(path)) == ".pdf" {
			pdfFiles = append(pdfFiles, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	total := len(pdfFiles)
	if total == 0 {
		return []SearchResult{}, nil
	}

	type indexedResult struct {
		results []SearchResult
	}

	resultCh := make(chan indexedResult, total)
	var wg sync.WaitGroup
	var mu sync.Mutex
	completed := 0

	for _, path := range pdfFiles {
		wg.Add(1)
		go func(p string) {
			defer wg.Done()
			r := processPDF(p, keyword)
			resultCh <- indexedResult{results: r}

			mu.Lock()
			completed++
			progress := int(float64(completed) / float64(total) * 100)
			mu.Unlock()
			progressCb(progress)
		}(path)
	}

	go func() {
		wg.Wait()
		close(resultCh)
	}()

	var allResults []SearchResult
	for r := range resultCh {
		allResults = append(allResults, r.results...)
	}

	sort.Slice(allResults, func(i, j int) bool {
		return allResults[i].Confidence > allResults[j].Confidence
	})

	if allResults == nil {
		allResults = []SearchResult{}
	}
	return allResults, nil
}

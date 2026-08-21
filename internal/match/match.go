// Package match scores a line of text against a search keyword.
//
// It is deliberately free of any PDF dependency: the scoring is pure string
// work, so it can be tested without cgo and without a MuPDF toolchain.
package match

import (
	"math"
	"regexp"
	"strings"

	"github.com/hbollon/go-edlib"
)

// MinConfidence is the score a line must reach to count as a hit.
//
// The value was inherited from the Python build, where it was tuned against
// rapidfuzz. go-edlib's Levenshtein similarity is a different scale, so this
// number is a starting point rather than a measured optimum — see the
// threshold item in To-Do.md.
const MinConfidence = 85

var (
	reSciStep1   = regexp.MustCompile(`([a-z])\.\s*([a-z])`)
	reSciStep2   = regexp.MustCompile(`[\s.]+`)
	reBasicWords = regexp.MustCompile(`\b[A-Za-z]+(?:['\.\-]\w+)*\b`)
	reScientific = regexp.MustCompile(`\b[A-Za-z]\.\s?[A-Za-z][a-z]+\b`)
	rePhrases    = regexp.MustCompile(`\b(?:\w+[\s\.\-])+\w+\b`)
	reAbbrev     = regexp.MustCompile(`\b(?:[A-Za-z]\.){1,}[A-Za-z]?[a-z]*\b`)
)

// NormalizeScientificName folds "E. coli", "E.coli" and "e coli" onto the same
// string, so an abbreviated genus matches however it was typeset.
func NormalizeScientificName(text string) string {
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

// NgramSimilarity is the Jaccard overlap of the two strings' character
// n-grams, as a percentage.
func NgramSimilarity(text1, text2 string, n int) float64 {
	norm1 := NormalizeScientificName(text1)
	norm2 := NormalizeScientificName(text2)

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

// Score rates how well line matches keyword and returns the score together
// with the substring of line that earned it.
//
// Five strategies are tried in order; the first three short-circuit with a
// fixed confidence, the last two compute one:
//
//  1. case-insensitive substring          100
//  2. normalised scientific name           95
//  3. token equality after normalisation    90
//  4. Levenshtein ratio / partial ratio    computed
//  5. character 3-gram Jaccard overlap     computed
func Score(line, keyword string) (float64, string) {
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
	normLine := NormalizeScientificName(line)
	normKeyword := NormalizeScientificName(keyword)
	if strings.Contains(normLine, normKeyword) {
		// Match the keyword's letters in order, allowing an optional period and
		// any whitespace between them — "Ecoli" then matches "E. coli",
		// "E.coli" and "e coli" alike. The separator goes *between* letters
		// only; appending it after the last one would demand a trailing period.
		parts := []rune(strings.ReplaceAll(strings.ReplaceAll(keyword, ".", ""), " ", ""))
		var pattern strings.Builder
		for i, c := range parts {
			if i > 0 {
				pattern.WriteString(`\.?\s*`)
			}
			pattern.WriteString(`[` + strings.ToLower(string(c)) + strings.ToUpper(string(c)) + `]`)
		}
		re, err := regexp.Compile(pattern.String())
		if err == nil {
			for _, m := range re.FindAllString(line, -1) {
				if NormalizeScientificName(m) == normKeyword {
					return 95, m
				}
			}
		}
	}

	// Strategy 3: Tokenized match (confidence 90)
	words := tokenizeText(line)
	for _, word := range words {
		if NormalizeScientificName(word) == normKeyword {
			return 90, word
		}
	}

	// Strategy 4: Token-based fuzzy matching
	bestScore := 0.0
	var bestMatch string

	for _, word := range words {
		if len([]rune(word)) > 2 {
			normWord := NormalizeScientificName(word)
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
	ngramScore := NgramSimilarity(line, keyword, 3)
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
				score := NgramSimilarity(phrase, keyword, 3)
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

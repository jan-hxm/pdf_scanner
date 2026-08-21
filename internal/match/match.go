// Package match scores a line of text against a search keyword.
//
// It is deliberately free of any PDF dependency: the scoring is pure string
// work, so it can be tested without cgo and without a MuPDF toolchain.
package match

import (
	"regexp"
	"strings"
	"unicode"

	"github.com/hbollon/go-edlib"
)

// MinConfidence is the score a line must reach to count as a hit.
//
// The value was inherited from the Python build, where it was tuned against
// rapidfuzz, and go-edlib's Levenshtein similarity is a different scale — so it
// has now been measured rather than assumed.
const MinConfidence = 85

var (
	reSciStep1   = regexp.MustCompile(`([a-z])\.\s*([a-z])`)
	reSciStep2   = regexp.MustCompile(`[\s.]+`)
	reBasicWords = regexp.MustCompile(`\b[A-Za-z]+(?:['\.\-]\w+)*\b`)
	reScientific = regexp.MustCompile(`\b[A-Za-z]\.\s?[A-Za-z][a-z]+\b`)
	rePhrases    = regexp.MustCompile(`\b(?:\w+[\s\.\-])+\w+\b`)
	reAbbrev     = regexp.MustCompile(`\b(?:[A-Za-z]\.){1,}[A-Za-z]?[a-z]*\b`)
)

// NormalizeScientificName folds "L. orem", "L.orem" and "l orem" onto the same
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

// partialRatio is the best fuzzyRatio between needle and any needle-length
// window of haystack, i.e. "how well does the best part of haystack match
// needle". It is only meaningful with haystack the longer of the two; the
// caller enforces that, because the reverse reading is what produced the false
// positives described in Score.
func partialRatio(needle, haystack string) float64 {
	n := []rune(needle)
	h := []rune(haystack)
	if len(n) >= len(h) {
		return fuzzyRatio(needle, haystack)
	}
	best := 0.0
	for i := 0; i <= len(h)-len(n); i++ {
		if score := fuzzyRatio(needle, string(h[i:i+len(n)])); score > best {
			best = score
		}
	}
	return best
}

// maxPartialLengthFactor caps how much longer than the keyword a token may be
// before partialRatio stops being applied to it. tokenizeText emits whole
// multi-word phrases, and sliding the keyword along a paragraph-sized one costs
// a Levenshtein pass per offset for no benefit: an exact occurrence inside such
// a run is already strategy 1's, and a misspelt one is found by the span scan
// in strategy 5, which windows the line properly.
const maxPartialLengthFactor = 3

// lengthPlausible reports whether candidate is close enough to keyword in
// length to be worth scoring as a match for it. Normalised lengths within half
// to double: outside that band the edit distance is dominated by the missing or
// surplus characters, so the comparison only costs time.
func lengthPlausible(candidate, keyword string) bool {
	c := len([]rune(NormalizeScientificName(candidate)))
	k := len([]rune(NormalizeScientificName(keyword)))
	return c*2 >= k && c <= k*2
}

// spans locates the whitespace-separated tokens of line as byte ranges, so a
// run of them can be sliced straight back out of the line. strings.Fields plus
// strings.Join cannot: it collapses the original spacing, and the result is
// then not a substring of the line — which the viewer relies on to find and
// highlight the match.
func spans(line string) [][2]int {
	var out [][2]int
	start := -1
	for i, r := range line {
		if unicode.IsSpace(r) {
			if start >= 0 {
				out = append(out, [2]int{start, i})
				start = -1
			}
			continue
		}
		if start < 0 {
			start = i
		}
	}
	if start >= 0 {
		out = append(out, [2]int{start, len(line)})
	}
	return out
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
//  4. Levenshtein ratio of a token, or of the keyword slid along a longer
//     token                                computed
//  5. character 3-gram Jaccard overlap of a short token span
//     computed
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
		// any whitespace between them — "Lorem" then matches "L. orem",
		// "L.orem" and "l orem" alike. The separator goes *between* letters
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

	// Strategy 4: fuzzy token matching.
	//
	// The comparison is directional: a partial ratio slides the *keyword* along a
	// longer token, answering "does this token contain something like the
	// keyword" — the question the search is actually asking. Sliding a shorter
	// token along the keyword answers the reverse, and answers it with 100 for
	// any token that happens to be a piece of the keyword. A token
	// shorter than the keyword is scored by the plain ratio instead, which
	// charges it for the length it is missing.
	bestScore := 0.0
	var bestMatch string

	keyLen := len([]rune(normKeyword))
	for _, word := range words {
		if len([]rune(word)) <= 2 {
			continue
		}
		normWord := NormalizeScientificName(word)
		score := fuzzyRatio(normWord, normKeyword)
		if n := len([]rune(normWord)); n > keyLen && n <= keyLen*maxPartialLengthFactor {
			if p := partialRatio(normKeyword, normWord); p > score {
				score = p
			}
		}
		if score > bestScore {
			bestScore = score
			bestMatch = word
		}
	}

	// Strategy 5: n-gram similarity over short token spans.
	//
	// Scored per span, not per line. The old code compared the whole line to the
	// keyword and then reported a span as the matched word, so the confidence
	// described one string while the highlight searched for another. It also
	// scaled badly: MuPDF extracts some pages as a single paragraph-long "line",
	// whose n-gram set dilutes any keyword to nothing no matter what it contains.
	//
	// Spans are sliced out of the line by offset so the result stays a substring
	// of it, and only spans of a plausible length are scored.
	maxSpan := len(strings.Fields(keyword)) + 1
	toks := spans(line)
	for i := range toks {
		for n := 1; n <= maxSpan && i+n <= len(toks); n++ {
			phrase := line[toks[i][0]:toks[i+n-1][1]]
			if !lengthPlausible(phrase, keyword) {
				continue
			}
			if score := NgramSimilarity(phrase, keyword, 3); score > bestScore {
				bestScore = score
				bestMatch = phrase
			}
		}
	}

	return bestScore, bestMatch
}

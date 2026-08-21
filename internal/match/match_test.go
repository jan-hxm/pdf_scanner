package match

import (
	"math"
	"testing"
)

func TestNormalizeScientificName(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"abbreviated genus", "E. coli", "ecoli"},
		{"no space", "E.coli", "ecoli"},
		{"no period", "e coli", "ecoli"},
		{"already normalised", "ecoli", "ecoli"},
		{"surrounding space", "  E. Coli  ", "ecoli"},
		{"full binomial", "Escherichia coli", "escherichiacoli"},
		{"empty", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NormalizeScientificName(tt.input); got != tt.want {
				t.Errorf("NormalizeScientificName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// The abbreviated forms must all collapse onto one string — that is the whole
// point of the normalisation, and what strategy 2 of Score relies on.
func TestNormalizeScientificNameFoldsVariants(t *testing.T) {
	variants := []string{"E. coli", "E.coli", "e. Coli", "E .coli", "e coli"}
	want := NormalizeScientificName(variants[0])
	for _, v := range variants[1:] {
		if got := NormalizeScientificName(v); got != want {
			t.Errorf("NormalizeScientificName(%q) = %q, want %q (same as %q)", v, got, want, variants[0])
		}
	}
}

func TestNgramSimilarity(t *testing.T) {
	tests := []struct {
		name    string
		a, b    string
		n       int
		wantMin float64
		wantMax float64
	}{
		{"identical", "escherichia", "escherichia", 3, 100, 100},
		{"nothing in common", "escherichia", "xyzzyxyzzy", 3, 0, 0},
		{"one typo", "escherichia", "escherichea", 3, 50, 99.9},
		{"shorter than n falls back to whole string", "ab", "ab", 3, 100, 100},
		{"empty needle", "escherichia", "", 3, 0, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NgramSimilarity(tt.a, tt.b, tt.n)
			if got < tt.wantMin || got > tt.wantMax {
				t.Errorf("NgramSimilarity(%q, %q, %d) = %.2f, want within [%.2f, %.2f]",
					tt.a, tt.b, tt.n, got, tt.wantMin, tt.wantMax)
			}
		})
	}
}

func TestNgramSimilarityIsSymmetricForEqualLengths(t *testing.T) {
	a, b := "escherichia", "escherichea"
	if x, y := NgramSimilarity(a, b, 3), NgramSimilarity(b, a, 3); math.Abs(x-y) > 1e-9 {
		t.Errorf("NgramSimilarity is asymmetric: %.4f vs %.4f", x, y)
	}
}

func TestScoreStrategies(t *testing.T) {
	tests := []struct {
		name      string
		line      string
		keyword   string
		wantScore float64
		wantWord  string
	}{
		{
			name:      "exact substring scores 100",
			line:      "Probe enthielt Escherichia coli in hoher Zahl",
			keyword:   "Escherichia coli",
			wantScore: 100,
			wantWord:  "Escherichia coli",
		},
		{
			name:      "exact substring ignores case",
			line:      "Probe enthielt ESCHERICHIA COLI",
			keyword:   "escherichia coli",
			wantScore: 100,
			wantWord:  "ESCHERICHIA COLI",
		},
		{
			name:      "abbreviated genus scores 95",
			line:      "Nachweis von E. coli im Trinkwasser",
			keyword:   "E.coli",
			wantScore: 95,
			wantWord:  "E. coli",
		},
		{
			name:      "keyword written with a space matches text without one",
			line:      "Nachweis von E.coli im Trinkwasser",
			keyword:   "E. coli",
			wantScore: 95,
			wantWord:  "E.coli",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score, word := Score(tt.line, tt.keyword)
			if score != tt.wantScore {
				t.Errorf("Score(%q, %q) score = %.1f, want %.1f", tt.line, tt.keyword, score, tt.wantScore)
			}
			if word != tt.wantWord {
				t.Errorf("Score(%q, %q) word = %q, want %q", tt.line, tt.keyword, word, tt.wantWord)
			}
		})
	}
}

// A hit must clear MinConfidence, and unrelated text must not — this is the
// behaviour the search loop actually filters on.
func TestScoreThresholdBehaviour(t *testing.T) {
	tests := []struct {
		name    string
		line    string
		keyword string
		wantHit bool
	}{
		{"exact", "Der Wert für Salmonella lag darunter", "Salmonella", true},
		{"single typo in a long word", "Der Wert für Salmonela lag darunter", "Salmonella", true},
		{"abbreviated genus", "Kolonien von E. coli sichtbar", "E.coli", true},
		{"unrelated line", "Die Temperatur betrug 21 Grad Celsius", "Salmonella", false},
		{"empty line", "", "Salmonella", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score, word := Score(tt.line, tt.keyword)
			gotHit := score >= MinConfidence
			if gotHit != tt.wantHit {
				t.Errorf("Score(%q, %q) = %.1f (word %q); hit = %v, want %v",
					tt.line, tt.keyword, score, word, gotHit, tt.wantHit)
			}
			if gotHit && word == "" {
				t.Errorf("Score(%q, %q) cleared the threshold at %.1f but returned no matched word",
					tt.line, tt.keyword, score)
			}
		})
	}
}

// Whatever Score returns as the matched word is what the viewer searches the
// page for, so it has to be a real substring of the line.
func TestScoreReturnsSubstringOfLine(t *testing.T) {
	lines := []string{
		"Nachweis von E. coli im Trinkwasser",
		"Probe enthielt Escherichia coli in hoher Zahl",
		"Der Wert für Salmonela lag darunter",
	}
	keywords := []string{"E.coli", "Escherichia coli", "Salmonella"}

	for _, line := range lines {
		for _, keyword := range keywords {
			score, word := Score(line, keyword)
			if score < MinConfidence || word == "" {
				continue
			}
			if !containsSubstring(line, word) {
				t.Errorf("Score(%q, %q) returned %q, which is not a substring of the line",
					line, keyword, word)
			}
		}
	}
}

func containsSubstring(haystack, needle string) bool {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}

func BenchmarkScore(b *testing.B) {
	line := "Die Untersuchung ergab einen Nachweis von Escherichia coli sowie Salmonella enterica"
	for i := 0; i < b.N; i++ {
		Score(line, "E.coli")
	}
}

package search

import (
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"

	"github.com/gen2brain/go-fitz"
	"pdf_scanner/internal/match"
	"pdf_scanner/internal/ocr/textcache"
)

// Where a hit's text came from. A result carries this so the UI can say that a
// scanned document was searched through recognised text, which is worth
// knowing: OCR misreads characters, and a hit found that way deserves more
// scepticism than one lifted from a real text layer.
const (
	MatchContent = "content"
	MatchOCR     = "ocr"
)

// SearchResult is one match: a keyword found on a page of a file.
//
// It carries no rectangles. The match is located and boxed in the frontend from
// PDF.js' text layer, which knows the real glyph metrics, the page rotation and
// the CropBox origin — none of which are recoverable from MuPDF's HTML dump.
type SearchResult struct {
	File       string `json:"file"`
	FileName   string `json:"fileName"`
	Page       int    `json:"page"`
	Found      bool   `json:"found"`
	Match      string `json:"match"`
	Confidence int    `json:"confidence"`
	FoundWord  string `json:"foundWord"`
	Context    string `json:"context"`
	// Occurrences is how often the word was found on this page. Lines that
	// repeat the same word on the same page fold into this counter instead of
	// being dropped, so a file's total is the number of hits it really
	// contains rather than the number of pages that contain one.
	Occurrences int `json:"occurrences"`
}

// FileFailure records a PDF that could not be read, so the UI can say so
// instead of silently returning fewer results.
type FileFailure struct {
	File     string `json:"file"`
	FileName string `json:"fileName"`
	Reason   string `json:"reason"`
}

// Outcome is everything one search run produced. Cancelled marks a run that
// was aborted, so the caller does not present a partial list as complete.
type Outcome struct {
	Results  []SearchResult `json:"results"`
	Failures []FileFailure  `json:"failures"`
	// Textless lists files MuPDF opened happily but got no text out of —
	// scans, in other words. They are not failures and not misses either:
	// searching them can only ever return nothing, whatever the keyword, and
	// saying so is the difference between "your term is not in this file" and
	// "this file cannot be searched at all". Without it a scanned PDF is
	// indistinguishable from one that simply does not mention the term.
	//
	// A scan whose text has already been recognised does not appear here: it
	// is searched through its cached OCR text like any other file.
	Textless []FileFailure `json:"textless"`
	// OCRFiles counts the files searched through recognised text rather than
	// through a text layer of their own.
	OCRFiles  int  `json:"ocrFiles"`
	Scanned   int  `json:"scanned"`
	Cancelled bool `json:"cancelled"`
}

// countOccurrences counts how often word occurs in line, case-insensitively.
// match.Score returns a word that is a substring of the line, so the count is
// exact; a word that somehow is not found still counts as the one hit that
// scoring already established.
func countOccurrences(line, word string) int {
	if word == "" {
		return 1
	}
	if n := strings.Count(strings.ToLower(line), strings.ToLower(word)); n > 0 {
		return n
	}
	return 1
}

// scanPages scores every line of every page and collects the survivors.
//
// It takes page text rather than a document on purpose: text extracted live
// from a PDF and text recognised earlier by OCR then run through exactly the
// same scoring, thresholds and folding, so a scanned file behaves like every
// other file once it has been recognised.
func scanPages(ctx context.Context, pages []string, filePath, keyword, source string) ([]SearchResult, error) {
	var results []SearchResult

	type seenKey struct {
		page int
		word string
	}
	// Index into results rather than a presence set: a repeated (page, word)
	// adds to that hit's count instead of disappearing.
	index := make(map[seenKey]int)

	fileName := filepath.Base(filePath)

	for pageNum, text := range pages {
		if err := ctx.Err(); err != nil {
			return results, err
		}

		for _, line := range strings.Split(text, "\n") {
			if len([]rune(line)) <= 1 {
				continue
			}

			confidence, foundWord := match.Score(line, keyword)
			if confidence < match.MinConfidence {
				continue
			}

			key := seenKey{pageNum + 1, foundWord}
			hits := countOccurrences(line, foundWord)
			if i, exists := index[key]; exists {
				results[i].Occurrences += hits
				continue
			}
			index[key] = len(results)

			results = append(results, SearchResult{
				File:        filePath,
				FileName:    fileName,
				Page:        pageNum + 1,
				Found:       true,
				Match:       source,
				Confidence:  int(math.Round(confidence)),
				FoundWord:   foundWord,
				Context:     strings.TrimSpace(line),
				Occurrences: hits,
			})
		}
	}

	return results, nil
}

// extractPages pulls the text layer out of every page. A page MuPDF cannot
// read contributes an empty string rather than aborting the file, so one bad
// page does not cost the other thirty.
func extractPages(ctx context.Context, filePath string) (pages []string, hasText bool, err error) {
	doc, err := fitz.New(filePath)
	if err != nil {
		return nil, false, err
	}
	defer doc.Close()

	pages = make([]string, doc.NumPage())
	for pageNum := range pages {
		if err := ctx.Err(); err != nil {
			return pages, hasText, err
		}

		text, err := doc.Text(pageNum)
		if err != nil {
			continue
		}
		pages[pageNum] = text
		if strings.TrimSpace(text) != "" {
			hasText = true
		}
	}
	return pages, hasText, nil
}

// processPDF scans every page of one file. A file MuPDF cannot open is an
// error, not an empty result — the caller reports it to the user.
//
// source says where the text came from: MatchContent for a real text layer,
// MatchOCR for a scan served out of the OCR cache, and an empty string for a
// scan with no recognised text, which the caller reports as textless.
func processPDF(ctx context.Context, filePath, keyword, ocrCacheDir string) (results []SearchResult, source string, err error) {
	pages, hasText, err := extractPages(ctx, filePath)
	if err != nil {
		return nil, "", err
	}

	source = MatchContent
	if !hasText {
		// Only scans are hashed for a cache lookup, so that cost falls on the
		// handful of files that need it rather than on every file in the
		// folder on every search.
		entry, ok := textcache.LookupFile(ocrCacheDir, filePath)
		if !ok {
			return nil, "", nil
		}
		pages, source = entry.Pages, MatchOCR
	}

	results, err = scanPages(ctx, pages, filePath, keyword, source)
	return results, source, err
}

// collectPDFs lists every *.pdf under folder, recursively.
func collectPDFs(folder string) ([]string, error) {
	info, err := os.Stat(folder)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("Ordner %q existiert nicht", folder)
		}
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%q ist kein Ordner", folder)
	}

	var pdfFiles []string
	err = filepath.Walk(folder, func(path string, info os.FileInfo, err error) error {
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
	return pdfFiles, nil
}

// SearchPDFs searches every PDF under folder for keyword, calling progressCb
// with the number of files completed and the total. A file with no text layer
// is searched through its recognised text when ocrCacheDir holds an entry for
// it; passing an empty cache directory disables that lookup. Concurrency is
// bounded to NumCPU because every worker holds an open MuPDF document through
// cgo. The run stops early once ctx is cancelled.
func SearchPDFs(ctx context.Context, folder, keyword, ocrCacheDir string, progressCb func(completed, total int)) (Outcome, error) {
	pdfFiles, err := collectPDFs(folder)
	if err != nil {
		return Outcome{}, err
	}

	outcome := Outcome{
		Results:  []SearchResult{},
		Failures: []FileFailure{},
		Textless: []FileFailure{},
		Scanned:  len(pdfFiles),
	}
	if len(pdfFiles) == 0 {
		return outcome, nil
	}

	workers := runtime.NumCPU()
	if workers > len(pdfFiles) {
		workers = len(pdfFiles)
	}

	var (
		wg        sync.WaitGroup
		mu        sync.Mutex
		completed int
	)
	sem := make(chan struct{}, workers)

	for _, path := range pdfFiles {
		if ctx.Err() != nil {
			break
		}

		wg.Add(1)
		sem <- struct{}{}
		go func(p string) {
			defer wg.Done()
			defer func() { <-sem }()

			results, source, err := processPDF(ctx, p, keyword, ocrCacheDir)

			mu.Lock()
			outcome.Results = append(outcome.Results, results...)
			switch {
			case err != nil && ctx.Err() == nil:
				outcome.Failures = append(outcome.Failures, FileFailure{
					File:     p,
					FileName: filepath.Base(p),
					Reason:   err.Error(),
				})
			case err == nil && source == "":
				outcome.Textless = append(outcome.Textless, FileFailure{
					File:     p,
					FileName: filepath.Base(p),
					Reason:   "kein durchsuchbarer Text (vermutlich ein Scan)",
				})
			case err == nil && source == MatchOCR:
				outcome.OCRFiles++
			}
			completed++
			done := completed
			mu.Unlock()

			progressCb(done, len(pdfFiles))
		}(path)
	}

	wg.Wait()

	if err := ctx.Err(); err != nil {
		return outcome, err
	}

	// Confidence first, then a total order on the rest. Without the tie-break
	// the order of equally-confident hits came from whichever worker finished
	// first, so the same search listed the same files differently twice in a
	// row.
	sort.Slice(outcome.Results, func(i, j int) bool {
		a, b := &outcome.Results[i], &outcome.Results[j]
		switch {
		case a.Confidence != b.Confidence:
			return a.Confidence > b.Confidence
		case a.File != b.File:
			return a.File < b.File
		case a.Page != b.Page:
			return a.Page < b.Page
		default:
			return a.FoundWord < b.FoundWord
		}
	})
	sort.Slice(outcome.Failures, func(i, j int) bool {
		return outcome.Failures[i].File < outcome.Failures[j].File
	})
	sort.Slice(outcome.Textless, func(i, j int) bool {
		return outcome.Textless[i].File < outcome.Textless[j].File
	})
	return outcome, nil
}

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
	Results   []SearchResult `json:"results"`
	Failures  []FileFailure  `json:"failures"`
	Scanned   int            `json:"scanned"`
	Cancelled bool           `json:"cancelled"`
}

// processPDF scans every page of one file. A file MuPDF cannot open is an
// error, not an empty result — the caller reports it to the user.
func processPDF(ctx context.Context, filePath, keyword string) ([]SearchResult, error) {
	doc, err := fitz.New(filePath)
	if err != nil {
		return nil, err
	}
	defer doc.Close()

	type seenKey struct {
		page int
		word string
	}
	seen := make(map[seenKey]struct{})

	var results []SearchResult
	fileName := filepath.Base(filePath)

	for pageNum := 0; pageNum < doc.NumPage(); pageNum++ {
		if err := ctx.Err(); err != nil {
			return results, err
		}

		text, err := doc.Text(pageNum)
		if err != nil {
			continue
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
			if _, exists := seen[key]; exists {
				continue
			}
			seen[key] = struct{}{}

			results = append(results, SearchResult{
				File:       filePath,
				FileName:   fileName,
				Page:       pageNum + 1,
				Found:      true,
				Match:      "content",
				Confidence: int(math.Round(confidence)),
				FoundWord:  foundWord,
				Context:    strings.TrimSpace(line),
			})
		}
	}

	return results, nil
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
// with progress 0–100. Concurrency is bounded to NumCPU because every worker
// holds an open MuPDF document through cgo. The run stops early once ctx is
// cancelled.
func SearchPDFs(ctx context.Context, folder, keyword string, progressCb func(int)) (Outcome, error) {
	pdfFiles, err := collectPDFs(folder)
	if err != nil {
		return Outcome{}, err
	}

	outcome := Outcome{
		Results:  []SearchResult{},
		Failures: []FileFailure{},
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

			results, err := processPDF(ctx, p, keyword)

			mu.Lock()
			outcome.Results = append(outcome.Results, results...)
			if err != nil && ctx.Err() == nil {
				outcome.Failures = append(outcome.Failures, FileFailure{
					File:     p,
					FileName: filepath.Base(p),
					Reason:   err.Error(),
				})
			}
			completed++
			progress := int(float64(completed) / float64(len(pdfFiles)) * 100)
			mu.Unlock()

			progressCb(progress)
		}(path)
	}

	wg.Wait()

	if err := ctx.Err(); err != nil {
		return outcome, err
	}

	sort.Slice(outcome.Results, func(i, j int) bool {
		return outcome.Results[i].Confidence > outcome.Results[j].Confidence
	})
	return outcome, nil
}

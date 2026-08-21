// Package ocr recognises text in PDFs that carry no text layer of their own,
// and stores the result through internal/ocr/textcache so the cost is paid
// once per document rather than once per search.
package ocr

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/gen2brain/go-fitz"
	"pdf_scanner/internal/ocr/textcache"
)

// DefaultDPI is the resolution pages are rasterised at before recognition.
// 300 is the resolution Tesseract's training data assumes; below it accuracy
// falls off sharply, above it the pages get slower without getting better.
const DefaultDPI = 300

// Progress reports how far a run has got. PagesDone and PagesTotal span the
// whole run, so the caller can drive a single bar across several files, while
// the per-file fields let it name what is being worked on.
type Progress struct {
	FileIndex int    `json:"fileIndex"`
	FileTotal int    `json:"fileTotal"`
	FileName  string `json:"fileName"`
	Page      int    `json:"page"`
	PageTotal int    `json:"pageTotal"`
	PagesDone int    `json:"pagesDone"`
	PagesLeft int    `json:"pagesTotal"`
}

// FileResult is what a run did to one file.
type FileResult struct {
	File        string `json:"file"`
	FileName    string `json:"fileName"`
	Pages       int    `json:"pages"`
	Chars       int    `json:"chars"`
	FailedPages int    `json:"failedPages"`
	// Cached marks a file that already had usable text under this language and
	// was not run again.
	Cached bool   `json:"cached"`
	Error  string `json:"error"`
}

// Report is everything a run produced.
type Report struct {
	Files     []FileResult `json:"files"`
	Cancelled bool         `json:"cancelled"`
}

// Request is one OCR run. Languages is passed to Tesseract verbatim, so it
// takes the engine's own "deu+eng" form for a document in two languages.
type Request struct {
	Files     []string
	Languages string
	CacheDir  string
	DPI       float64
}

// Run recognises text in every requested file and caches it. Files already
// cached under the same language are skipped rather than re-run.
//
// Recognition is the expensive half by a wide margin, so rasterising happens
// on the calling goroutine — MuPDF serialises it per document anyway — while
// pages are recognised by a pool of one process per core.
func Run(ctx context.Context, req Request, progressCb func(Progress)) (Report, error) {
	report := Report{Files: []FileResult{}}

	if req.Languages == "" {
		return report, errors.New("keine Sprache für die Texterkennung angegeben")
	}
	exe := Locate()
	if exe == "" {
		return report, errors.New("Tesseract OCR ist nicht installiert")
	}
	if req.DPI == 0 {
		req.DPI = DefaultDPI
	}

	plan, total := planRun(req)

	pagesDone := 0
	for i, item := range plan {
		if ctx.Err() != nil {
			report.Cancelled = true
			return report, nil
		}
		if item.result.Error != "" || item.result.Cached {
			report.Files = append(report.Files, item.result)
			continue
		}

		result, done := recognizeFile(ctx, exe, req, item, i, len(plan), pagesDone, total, progressCb)
		pagesDone = done
		report.Files = append(report.Files, result)

		if ctx.Err() != nil {
			report.Cancelled = true
			return report, nil
		}
	}
	return report, nil
}

// planItem is one file's page count, established before any recognition starts
// so the progress bar knows its total from the first tick rather than growing
// as files are opened.
type planItem struct {
	path   string
	pages  int
	result FileResult
	hash   string
}

// planRun opens each file to count its pages and resolve its cache state,
// returning the plan and the number of pages actually to be recognised.
func planRun(req Request) ([]planItem, int) {
	plan := make([]planItem, 0, len(req.Files))
	total := 0

	for _, path := range req.Files {
		item := planItem{
			path: path,
			result: FileResult{
				File:     path,
				FileName: filepath.Base(path),
			},
		}

		hash, err := textcache.HashFile(path)
		if err != nil {
			item.result.Error = err.Error()
			plan = append(plan, item)
			continue
		}
		item.hash = hash

		// An entry recognised in the language being asked for is the answer
		// already. A different language is not, so it is replaced.
		if entry, ok := textcache.Load(req.CacheDir, hash); ok && entry.Languages == req.Languages {
			item.result.Cached = true
			item.result.Pages = len(entry.Pages)
			item.result.Chars = countChars(entry.Pages)
			plan = append(plan, item)
			continue
		}

		doc, err := fitz.New(path)
		if err != nil {
			item.result.Error = err.Error()
			plan = append(plan, item)
			continue
		}
		item.pages = doc.NumPage()
		doc.Close()

		total += item.pages
		plan = append(plan, item)
	}
	return plan, total
}

// recognizeFile runs one file and stores its text. It returns the running
// page count so the caller can keep the overall progress continuous.
func recognizeFile(ctx context.Context, exe string, req Request, item planItem, index, fileTotal, pagesDone, pagesTotal int, progressCb func(Progress)) (FileResult, int) {
	result := item.result

	doc, err := fitz.New(item.path)
	if err != nil {
		result.Error = err.Error()
		return result, pagesDone
	}
	defer doc.Close()

	workers := runtime.NumCPU()
	if workers > item.pages {
		workers = item.pages
	}
	if workers < 1 {
		workers = 1
	}

	var (
		wg      sync.WaitGroup
		mu      sync.Mutex
		pages   = make([]string, item.pages)
		failed  int
		lastErr error
	)
	sem := make(chan struct{}, workers)

	for page := 0; page < item.pages; page++ {
		if ctx.Err() != nil {
			break
		}

		png, err := doc.ImagePNG(page, req.DPI)
		if err != nil {
			mu.Lock()
			failed++
			lastErr = err
			mu.Unlock()
			continue
		}

		wg.Add(1)
		sem <- struct{}{}
		go func(idx int, img []byte) {
			defer wg.Done()
			defer func() { <-sem }()

			text, err := recognize(ctx, exe, img, req.Languages)

			mu.Lock()
			if err != nil {
				if ctx.Err() == nil {
					failed++
					lastErr = err
				}
			} else {
				pages[idx] = text
			}
			done := pagesDone + idx + 1
			mu.Unlock()

			progressCb(Progress{
				FileIndex: index + 1,
				FileTotal: fileTotal,
				FileName:  result.FileName,
				Page:      idx + 1,
				PageTotal: item.pages,
				PagesDone: done,
				PagesLeft: pagesTotal,
			})
		}(page, png)
	}

	wg.Wait()
	pagesDone += item.pages

	// A cancelled run holds only part of the document. Caching it would make
	// the gap permanent — every later search would read the truncated text
	// back as though it were the whole file.
	if ctx.Err() != nil {
		return result, pagesDone
	}

	result.Pages = item.pages
	result.Chars = countChars(pages)
	result.FailedPages = failed

	if failed >= item.pages {
		result.Error = fmt.Sprintf("keine Seite konnte erkannt werden: %v", lastErr)
		return result, pagesDone
	}

	entry := textcache.Entry{
		Hash:      item.hash,
		FileName:  result.FileName,
		Languages: req.Languages,
		DPI:       int(req.DPI),
		Engine:    version(exe),
		Pages:     pages,
	}
	if err := textcache.Store(req.CacheDir, entry); err != nil {
		result.Error = fmt.Sprintf("Text erkannt, aber nicht gespeichert: %v", err)
	}
	return result, pagesDone
}

func countChars(pages []string) int {
	n := 0
	for _, p := range pages {
		n += len(strings.TrimSpace(p))
	}
	return n
}

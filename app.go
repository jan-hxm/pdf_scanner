package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	"github.com/wailsapp/wails/v2/pkg/runtime"
	"pdf_scanner/internal/ocr"
	"pdf_scanner/internal/opener"
	"pdf_scanner/internal/search"
)

type App struct {
	ctx context.Context

	// mu guards cancelSearch, which aborts the search run currently in flight.
	// searchGen identifies that run, so a finishing search only clears its own
	// cancel func and never the one a newer search installed.
	mu           sync.Mutex
	cancelSearch context.CancelFunc
	searchGen    uint64

	// cancelOCR aborts the recognition run in flight. It is separate from
	// cancelSearch on purpose: OCR outlives the search that discovered the
	// scan, so cancelling one must not cancel the other.
	cancelOCR context.CancelFunc
	ocrGen    uint64
}

func NewApp() *App { return &App{} }

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	migrateLegacySettings()

	// Create the PDFs folder up front, so a fresh install does not greet the
	// user with an empty search against a folder that does not exist.
	if folder := a.GetPDFsFolder(); folder != "" {
		_ = os.MkdirAll(folder, 0755)
	}

	// Native file drop: WebView2 has no File.path, so paths come from Wails.
	runtime.OnFileDrop(ctx, func(x, y int, paths []string) {
		runtime.EventsEmit(ctx, "files-dropped", paths)
	})
}

type SearchHistoryEntry struct {
	Timestamp string `json:"timestamp"`
	Keyword   string `json:"keyword"`
}

type Settings struct {
	Theme          string               `json:"theme"`
	Language       string               `json:"language"`
	SearchHistory  []SearchHistoryEntry `json:"searchHistory"`
	HighlightColor string               `json:"highlightColor"`
	PDFsFolder     string               `json:"pdfsFolder"`
}

func settingsDir() string {
	configDir, err := os.UserConfigDir()
	if err != nil {
		configDir = os.TempDir()
	}
	return filepath.Join(configDir, "pdf_scanner")
}

func settingsPath() string {
	return filepath.Join(settingsDir(), "settings.json")
}

// ocrCacheDir is where recognised text is kept, next to the settings. It is
// deliberately not inside the PDFs folder: the cache is derived data keyed on
// file content, and dropping it into the folder the user browses would put
// files there that they did not put there.
func ocrCacheDir() string {
	return filepath.Join(settingsDir(), "ocr-cache")
}

// migrateLegacySettings moves the settings written under the app's former name
// ("pdf-searcher") to the current directory, so an existing install keeps its
// search history and PDFs folder across the rename. Best effort: if the new
// directory already exists, it wins and the old one is left untouched.
func migrateLegacySettings() {
	current := settingsDir()
	if _, err := os.Stat(current); err == nil {
		return
	}
	legacy := filepath.Join(filepath.Dir(current), "pdf-searcher")
	if _, err := os.Stat(filepath.Join(legacy, "settings.json")); err != nil {
		return
	}
	_ = os.Rename(legacy, current)
}

func (a *App) LoadSettings() Settings {
	defaultPDFsFolder := filepath.Join(os.Getenv("USERPROFILE"), "Documents", "PDFs")
	defaults := Settings{
		Theme:          "light",
		Language:       "de",
		SearchHistory:  []SearchHistoryEntry{},
		HighlightColor: "#9B59B6",
		PDFsFolder:     defaultPDFsFolder,
	}

	data, err := os.ReadFile(settingsPath())
	if err != nil {
		return defaults
	}

	var s Settings
	if err := json.Unmarshal(data, &s); err != nil {
		return defaults
	}
	if s.SearchHistory == nil {
		s.SearchHistory = []SearchHistoryEntry{}
	}
	if s.PDFsFolder == "" {
		s.PDFsFolder = defaultPDFsFolder
	}
	return s
}

func (a *App) SaveSettings(s Settings) error {
	path := settingsPath()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	// Non-destructive merge with existing settings
	existing := a.LoadSettings()
	if s.Theme != "" {
		existing.Theme = s.Theme
	}
	if s.Language != "" {
		existing.Language = s.Language
	}
	if s.SearchHistory != nil {
		existing.SearchHistory = s.SearchHistory
	}
	if s.HighlightColor != "" {
		existing.HighlightColor = s.HighlightColor
	}
	if s.PDFsFolder != "" {
		existing.PDFsFolder = s.PDFsFolder
	}

	data, err := json.MarshalIndent(existing, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func (a *App) GetPDFsFolder() string {
	return a.LoadSettings().PDFsFolder
}

// SelectPDFsFolder asks for a new PDFs folder and persists it. Returns the
// folder in use afterwards — unchanged if the dialog was cancelled.
func (a *App) SelectPDFsFolder() (string, error) {
	current := a.GetPDFsFolder()
	chosen, err := runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title:            "PDF-Ordner auswählen",
		DefaultDirectory: current,
	})
	if err != nil {
		return current, err
	}
	if chosen == "" {
		return current, nil
	}
	if err := a.SaveSettings(Settings{PDFsFolder: chosen}); err != nil {
		return current, err
	}
	return chosen, nil
}

// SearchPDFs searches all PDFs in folder for keyword. Progress ticks are
// emitted as "search-progress" events; the results come back as the return
// value, so a superseded run cannot overwrite a newer one's output. Starting a
// search cancels the one before it.
func (a *App) SearchPDFs(folder, keyword string) (search.Outcome, error) {
	a.mu.Lock()
	if a.cancelSearch != nil {
		a.cancelSearch()
	}
	ctx, cancel := context.WithCancel(a.ctx)
	a.searchGen++
	gen := a.searchGen
	a.cancelSearch = cancel
	a.mu.Unlock()

	defer func() {
		cancel()
		a.mu.Lock()
		if a.searchGen == gen {
			a.cancelSearch = nil
		}
		a.mu.Unlock()
	}()

	outcome, err := search.SearchPDFs(ctx, folder, keyword, ocrCacheDir(), func(completed, total int) {
		progress := 0
		if total > 0 {
			progress = int(float64(completed) / float64(total) * 100)
		}
		runtime.EventsEmit(a.ctx, "search-progress", map[string]interface{}{
			"progress":  progress,
			"completed": completed,
			"total":     total,
		})
	})
	if err != nil {
		if ctx.Err() != nil {
			// Superseded by a newer search, or cancelled by the user. Not an
			// error to show — but the partial results must not be presented as
			// a finished run either.
			outcome.Cancelled = true
			return outcome, nil
		}
		return outcome, err
	}
	return outcome, nil
}

// CancelSearch aborts the search currently in flight, if any.
func (a *App) CancelSearch() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.cancelSearch != nil {
		a.cancelSearch()
		a.cancelSearch = nil
	}
}

// OCRStatus reports whether text recognition is possible on this machine and
// which languages are installed. It never fails: a missing engine is a state
// the UI renders, not an error to raise.
func (a *App) OCRStatus() ocr.Info {
	return ocr.Status()
}

// RecognizeText runs OCR over the given files and caches the result, so every
// later search reads the text back instead of recognising it again.
//
// It is called after a search has already returned, not during one: recognition
// takes seconds per page, and a 31-page scan would otherwise stall the search
// that discovered it for minutes. The search result the user is looking at
// stays on screen while this runs, and the frontend re-runs the search once it
// finishes.
//
// languages is Tesseract's own language string, so "deu+eng" recognises a
// document written in both.
func (a *App) RecognizeText(paths []string, languages string) (ocr.Report, error) {
	if len(paths) == 0 {
		return ocr.Report{Files: []ocr.FileResult{}}, nil
	}

	a.mu.Lock()
	if a.cancelOCR != nil {
		a.cancelOCR()
	}
	ctx, cancel := context.WithCancel(a.ctx)
	a.ocrGen++
	gen := a.ocrGen
	a.cancelOCR = cancel
	a.mu.Unlock()

	defer func() {
		cancel()
		a.mu.Lock()
		if a.ocrGen == gen {
			a.cancelOCR = nil
		}
		a.mu.Unlock()
	}()

	report, err := ocr.Run(ctx, ocr.Request{
		Files:     paths,
		Languages: languages,
		CacheDir:  ocrCacheDir(),
	}, func(p ocr.Progress) {
		progress := 0
		if p.PagesLeft > 0 {
			progress = int(float64(p.PagesDone) / float64(p.PagesLeft) * 100)
		}
		runtime.EventsEmit(a.ctx, "ocr-progress", map[string]interface{}{
			"progress":   progress,
			"fileIndex":  p.FileIndex,
			"fileTotal":  p.FileTotal,
			"fileName":   p.FileName,
			"page":       p.Page,
			"pageTotal":  p.PageTotal,
			"pagesDone":  p.PagesDone,
			"pagesTotal": p.PagesLeft,
		})
	})
	if err != nil {
		return report, err
	}
	return report, nil
}

// CancelOCR aborts the recognition run currently in flight, if any. A file
// interrupted part-way is not cached, so the next run starts it over rather
// than searching half a document.
func (a *App) CancelOCR() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.cancelOCR != nil {
		a.cancelOCR()
		a.cancelOCR = nil
	}
}

// OpenPDF opens the PDF at path on the given page in an external reader.
func (a *App) OpenPDF(path string, page int) error {
	return opener.OpenPDFAtPage(path, page)
}

// LoadPDF returns raw PDF bytes for in-app viewing via PDF.js.
func (a *App) LoadPDF(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("PDF konnte nicht gelesen werden: %w", err)
	}
	return data, nil
}

// SelectFiles opens a multi-file dialog filtered to .pdf files.
func (a *App) SelectFiles() ([]string, error) {
	files, err := runtime.OpenMultipleFilesDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "PDF-Dateien auswählen",
		Filters: []runtime.FileFilter{
			{
				DisplayName: "PDF Dateien (*.pdf)",
				Pattern:     "*.pdf",
			},
		},
	})
	if err != nil {
		return []string{}, err
	}
	if files == nil {
		return []string{}, nil
	}
	return files, nil
}

// CopyFileToPDFsFolder copies src into the PDFs folder and returns the name it
// was stored under. An existing file of the same name is never overwritten —
// the copy is de-duplicated as "name (2).pdf".
func (a *App) CopyFileToPDFsFolder(src string) (string, error) {
	folder := a.GetPDFsFolder()
	if err := os.MkdirAll(folder, 0755); err != nil {
		return "", err
	}

	srcFile, err := os.Open(src)
	if err != nil {
		return "", err
	}
	defer srcFile.Close()

	dst := uniquePath(folder, filepath.Base(src))

	// O_EXCL so a file appearing between uniquePath and Create is not clobbered.
	dstFile, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		return "", err
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return "", err
	}
	return filepath.Base(dst), nil
}

// uniquePath returns a path in folder that no file occupies yet, appending
// " (2)", " (3)", … to the base name as needed.
func uniquePath(folder, name string) string {
	ext := filepath.Ext(name)
	base := name[:len(name)-len(ext)]

	candidate := filepath.Join(folder, name)
	for i := 2; ; i++ {
		if _, err := os.Stat(candidate); os.IsNotExist(err) {
			return candidate
		}
		candidate = filepath.Join(folder, fmt.Sprintf("%s (%d)%s", base, i, ext))
	}
}

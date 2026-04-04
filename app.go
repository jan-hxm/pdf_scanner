package main

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"

	"github.com/wailsapp/wails/v2/pkg/runtime"
	"pdf-searcher/internal/opener"
	"pdf-searcher/internal/search"
)

type App struct {
	ctx context.Context
}

func NewApp() *App { return &App{} }

func (a *App) startup(ctx context.Context) { a.ctx = ctx }

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

func settingsPath() string {
	configDir, err := os.UserConfigDir()
	if err != nil {
		configDir = os.TempDir()
	}
	return filepath.Join(configDir, "pdf-searcher", "settings.json")
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

// SearchPDFs searches all PDFs in folder for keyword.
// Emits "search-progress" Wails events during processing.
func (a *App) SearchPDFs(folder, keyword string) []search.SearchResult {
	results, _ := search.SearchPDFs(a.ctx, folder, keyword, func(progress int) {
		runtime.EventsEmit(a.ctx, "search-progress", map[string]interface{}{
			"progress": progress,
		})
	})
	runtime.EventsEmit(a.ctx, "search-progress", map[string]interface{}{
		"results": results,
	})
	return results
}

// OpenPDF opens the PDF at path on the given page in Adobe Reader.
func (a *App) OpenPDF(path string, page int) error {
	return opener.OpenPDFAtPage(path, page)
}

// LoadPDF returns raw PDF bytes for in-app viewing via PDF.js.
func (a *App) LoadPDF(path string) []byte {
	data, _ := os.ReadFile(path)
	return data
}

// SelectFiles opens a multi-file dialog filtered to .pdf files.
func (a *App) SelectFiles() []string {
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
		return []string{}
	}
	if files == nil {
		return []string{}
	}
	return files
}

// MoveFileToPDFsFolder copies src into the PDFs folder.
func (a *App) MoveFileToPDFsFolder(src string) error {
	folder := a.GetPDFsFolder()
	if err := os.MkdirAll(folder, 0755); err != nil {
		return err
	}
	dst := filepath.Join(folder, filepath.Base(src))

	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	return err
}

package textcache

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeFile drops content at a path inside dir and returns the path.
func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("writing %s: %v", name, err)
	}
	return path
}

func TestHashFileIdentifiesContentNotPath(t *testing.T) {
	dir := t.TempDir()

	original := writeFile(t, dir, "scan.pdf", "the same bytes")
	renamed := writeFile(t, dir, "scan (2).pdf", "the same bytes")
	edited := writeFile(t, dir, "edited.pdf", "the same bytes, plus one more")

	hashOriginal, err := HashFile(original)
	if err != nil {
		t.Fatalf("HashFile: %v", err)
	}
	hashRenamed, err := HashFile(renamed)
	if err != nil {
		t.Fatalf("HashFile: %v", err)
	}
	hashEdited, err := HashFile(edited)
	if err != nil {
		t.Fatalf("HashFile: %v", err)
	}

	// The point of hashing content: a copy under a different name is the same
	// document and must reuse the text that was already recognised for it.
	if hashOriginal != hashRenamed {
		t.Errorf("identical content hashed differently:\n %s\n %s", hashOriginal, hashRenamed)
	}
	// And the point of not caching by path: an edited file is a different
	// document, so its stale text must not be served.
	if hashOriginal == hashEdited {
		t.Error("different content produced the same hash")
	}
	if len(hashOriginal) != 64 {
		t.Errorf("expected a 64-character SHA-256, got %d characters", len(hashOriginal))
	}
}

func TestHashFileMissing(t *testing.T) {
	if _, err := HashFile(filepath.Join(t.TempDir(), "absent.pdf")); err == nil {
		t.Error("expected an error hashing a file that does not exist")
	}
}

func TestStoreLoadRoundTrip(t *testing.T) {
	cacheDir := filepath.Join(t.TempDir(), "ocr-cache")

	want := Entry{
		Hash:      strings.Repeat("a", 64),
		FileName:  "Fortbildung Antidepressiva.pdf",
		Languages: "deu",
		DPI:       300,
		Engine:    "tesseract 5.3.3",
		Pages:     []string{"Seite eins", "", "Seite drei mit Ümläuten"},
	}

	// Store creates the cache directory itself: it is written long after
	// startup, the first time a scan is actually recognised.
	if err := Store(cacheDir, want); err != nil {
		t.Fatalf("Store: %v", err)
	}

	got, ok := Load(cacheDir, want.Hash)
	if !ok {
		t.Fatal("Load missed an entry that was just stored")
	}
	if got.Languages != want.Languages || got.DPI != want.DPI || got.Engine != want.Engine {
		t.Errorf("metadata did not survive the round trip: %+v", got)
	}
	if len(got.Pages) != len(want.Pages) {
		t.Fatalf("expected %d pages, got %d", len(want.Pages), len(got.Pages))
	}
	for i := range want.Pages {
		if got.Pages[i] != want.Pages[i] {
			t.Errorf("page %d: got %q, want %q", i+1, got.Pages[i], want.Pages[i])
		}
	}
	if got.Created.IsZero() {
		t.Error("Store did not stamp Created")
	}
}

func TestStoreReplacesEntryForSameFile(t *testing.T) {
	cacheDir := t.TempDir()
	hash := strings.Repeat("b", 64)

	if err := Store(cacheDir, Entry{Hash: hash, Languages: "deu", Pages: []string{"deutscher Text"}}); err != nil {
		t.Fatalf("Store: %v", err)
	}
	if err := Store(cacheDir, Entry{Hash: hash, Languages: "eng", Pages: []string{"english text"}}); err != nil {
		t.Fatalf("Store: %v", err)
	}

	// One entry per file, not one per language. Two would leave the search
	// with no way to choose between them, so re-running in another language
	// replaces what was there.
	entries, err := filepath.Glob(filepath.Join(cacheDir, "*.json"))
	if err != nil {
		t.Fatalf("Glob: %v", err)
	}
	if len(entries) != 1 {
		t.Errorf("expected 1 cache entry, got %d: %v", len(entries), entries)
	}

	got, ok := Load(cacheDir, hash)
	if !ok {
		t.Fatal("Load missed the replaced entry")
	}
	if got.Languages != "eng" || got.Pages[0] != "english text" {
		t.Errorf("the later run did not win: %+v", got)
	}
}

func TestStoreLeavesNoTemporaryFiles(t *testing.T) {
	cacheDir := t.TempDir()
	if err := Store(cacheDir, Entry{Hash: strings.Repeat("c", 64), Pages: []string{"text"}}); err != nil {
		t.Fatalf("Store: %v", err)
	}

	names, err := os.ReadDir(cacheDir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	for _, name := range names {
		if strings.HasSuffix(name.Name(), ".tmp") {
			t.Errorf("left a temporary file behind: %s", name.Name())
		}
	}
}

// A damaged entry has to read as a miss. The cost of a miss is one re-run; the
// cost of trusting a damaged entry is a document that silently searches wrong
// for as long as the entry survives.
func TestLoadTreatsUnusableEntriesAsMisses(t *testing.T) {
	cacheDir := t.TempDir()

	stale := Entry{Version: entryVersion + 1, Hash: strings.Repeat("d", 64), Pages: []string{"text"}}
	staleJSON, err := json.Marshal(stale)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	cases := []struct {
		name string
		hash string
		body string
	}{
		{"absent", strings.Repeat("0", 64), ""},
		{"not json", strings.Repeat("1", 64), "{ this is not json"},
		{"wrong version", stale.Hash, string(staleJSON)},
		{"no pages", strings.Repeat("2", 64), fmt.Sprintf(`{"version":%d,"pages":[]}`, entryVersion)},
		{"truncated write", strings.Repeat("3", 64), fmt.Sprintf(`{"version":%d,"pages":["half`, entryVersion)},
		// Text produced by a superseded pipeline. Version 1 predates --psm 1, so
		// its entries for a sideways scan are rotated gibberish that reads as a
		// perfectly well-formed hit.
		{"superseded pipeline", strings.Repeat("4", 64), `{"version":1,"pages":["younz Bunjpueyeqsuolsseideq"]}`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.body != "" {
				writeFile(t, cacheDir, tc.hash+".json", tc.body)
			}
			if _, ok := Load(cacheDir, tc.hash); ok {
				t.Error("expected a miss")
			}
		})
	}
}

func TestLookupFile(t *testing.T) {
	dir := t.TempDir()
	cacheDir := filepath.Join(dir, "ocr-cache")
	scan := writeFile(t, dir, "scan.pdf", "pretend this is a scanned PDF")

	if _, ok := LookupFile(cacheDir, scan); ok {
		t.Error("expected a miss before anything was stored")
	}

	hash, err := HashFile(scan)
	if err != nil {
		t.Fatalf("HashFile: %v", err)
	}
	if err := Store(cacheDir, Entry{Hash: hash, Languages: "deu", Pages: []string{"erkannter Text"}}); err != nil {
		t.Fatalf("Store: %v", err)
	}

	entry, ok := LookupFile(cacheDir, scan)
	if !ok {
		t.Fatal("expected a hit for the file that was stored")
	}
	if entry.Pages[0] != "erkannter Text" {
		t.Errorf("got %q", entry.Pages[0])
	}

	// The same bytes under another name are the same document.
	copied := writeFile(t, dir, "scan - Kopie.pdf", "pretend this is a scanned PDF")
	if _, ok := LookupFile(cacheDir, copied); !ok {
		t.Error("a renamed copy should reuse the cached text")
	}

	// Different bytes are not.
	edited := writeFile(t, dir, "scan-v2.pdf", "pretend this is a scanned PDF, revised")
	if _, ok := LookupFile(cacheDir, edited); ok {
		t.Error("an edited file must not be served the old text")
	}
}

func TestStatCountsOnlyEntries(t *testing.T) {
	cacheDir := t.TempDir()

	if got := Stat(cacheDir); got.Entries != 0 || got.Bytes != 0 {
		t.Errorf("expected an empty cache to stat as empty, got %+v", got)
	}
	if got := Stat(filepath.Join(cacheDir, "never-created")); got.Entries != 0 {
		t.Errorf("a cache directory that does not exist is an empty cache, got %+v", got)
	}

	for i, hash := range []string{strings.Repeat("a", 64), strings.Repeat("b", 64)} {
		if err := Store(cacheDir, Entry{Hash: hash, Pages: []string{fmt.Sprintf("Seite %d", i)}}); err != nil {
			t.Fatalf("Store: %v", err)
		}
	}
	// Anything that is not an entry is not the cache's size, and the number the
	// user is shown has to be the one the button will free.
	writeFile(t, cacheDir, "notes.txt", strings.Repeat("x", 4096))

	got := Stat(cacheDir)
	if got.Entries != 2 {
		t.Errorf("expected 2 entries, got %d", got.Entries)
	}
	if got.Bytes == 0 {
		t.Error("expected the entries to have a size")
	}
	if got.Bytes >= 4096 {
		t.Errorf("the stray file was counted: %d bytes for two small entries", got.Bytes)
	}
}

func TestClearRemovesEntriesAndNothingElse(t *testing.T) {
	cacheDir := t.TempDir()

	for _, hash := range []string{strings.Repeat("a", 64), strings.Repeat("b", 64)} {
		if err := Store(cacheDir, Entry{Hash: hash, Pages: []string{"erkannter Text"}}); err != nil {
			t.Fatalf("Store: %v", err)
		}
	}
	// A temporary left by an interrupted Store belongs to this package and goes
	// with the rest; the settings file next door emphatically does not.
	writeFile(t, cacheDir, strings.Repeat("c", 64)+".123.tmp", "half a write")
	keep := writeFile(t, cacheDir, "settings.json.bak", "not ours")

	removed, err := Clear(cacheDir)
	if err != nil {
		t.Fatalf("Clear: %v", err)
	}
	if removed != 2 {
		t.Errorf("expected 2 entries removed, got %d", removed)
	}
	if got := Stat(cacheDir); got.Entries != 0 {
		t.Errorf("expected an empty cache afterwards, got %+v", got)
	}
	if _, err := os.Stat(keep); err != nil {
		t.Errorf("Clear removed a file it does not own: %v", err)
	}
	if matches, _ := filepath.Glob(filepath.Join(cacheDir, "*.tmp")); len(matches) != 0 {
		t.Errorf("temporaries survived: %v", matches)
	}

	// Clearing twice, or clearing a cache that was never written, is what the
	// button does on an idle install — neither is an error.
	if removed, err := Clear(cacheDir); err != nil || removed != 0 {
		t.Errorf("second Clear: removed %d, err %v", removed, err)
	}
	if removed, err := Clear(filepath.Join(cacheDir, "never-created")); err != nil || removed != 0 {
		t.Errorf("Clear on a missing directory: removed %d, err %v", removed, err)
	}
	if removed, err := Clear(""); err != nil || removed != 0 {
		t.Errorf("Clear with no cache directory: removed %d, err %v", removed, err)
	}
}

func TestLookupFileWithoutCacheDir(t *testing.T) {
	dir := t.TempDir()
	scan := writeFile(t, dir, "scan.pdf", "content")

	// An empty cache directory disables the lookup, which is how a caller
	// turns the OCR cache off without a second code path.
	if _, ok := LookupFile("", scan); ok {
		t.Error("expected a miss with no cache directory configured")
	}
	if _, ok := LookupFile(filepath.Join(dir, "nope"), scan); ok {
		t.Error("expected a miss against a cache directory that does not exist")
	}
}

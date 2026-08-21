// Package textcache persists text recognised from PDFs that have none of
// their own.
//
// Scanned PDFs yield nothing from MuPDF's text extraction, so no amount of
// scoring will ever find a keyword in them. OCR fixes that, but it costs
// seconds per page — far too much to pay on every search. The cost is
// therefore paid once per document and kept: recognised text is stored under a
// hash of the file's bytes, so every later search reads it back instead of
// running the engine again.
//
// Keying on content rather than path means renaming, moving or re-importing a
// file reuses its cached text, while editing it misses and re-runs. The
// language used is recorded in the entry rather than folded into the key: two
// entries for one file would leave the search with no way to choose between
// them, so re-running OCR in a different language replaces the entry instead.
//
// This is a package of its own so that it stays free of the PDF dependency:
// go-fitz loads MuPDF at init, through cgo or through a DLL, and a storage
// layer that cannot be tested without a C toolchain is a storage layer that
// stops being tested.
package textcache

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"time"
)

// entryVersion is bumped when the stored shape changes. An entry written by an
// older version is ignored rather than misread, which costs one re-run.
const entryVersion = 1

// Entry is one document's recognised text, one string per page.
type Entry struct {
	Version int `json:"version"`
	// Hash is the SHA-256 of the source file, and also the entry's file name.
	Hash string `json:"hash"`
	// FileName is the name the file had when it was recognised. Diagnostics
	// only — the hash is what identifies the entry.
	FileName  string    `json:"fileName"`
	Languages string    `json:"languages"`
	DPI       int       `json:"dpi"`
	Engine    string    `json:"engine"`
	Created   time.Time `json:"created"`
	Pages     []string  `json:"pages"`
}

// HashFile returns the SHA-256 of the file's contents as lowercase hex.
func HashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// entryPath is where the entry for hash lives inside cacheDir.
func entryPath(cacheDir, hash string) string {
	return filepath.Join(cacheDir, hash+".json")
}

// Load returns the cached entry for hash. A missing, unreadable, malformed or
// out-of-date entry is reported as a plain miss: the worst case is one
// unnecessary re-run, whereas surfacing the error would put a cache detail in
// front of the user for no benefit.
func Load(cacheDir, hash string) (Entry, bool) {
	data, err := os.ReadFile(entryPath(cacheDir, hash))
	if err != nil {
		return Entry{}, false
	}

	var e Entry
	if err := json.Unmarshal(data, &e); err != nil {
		return Entry{}, false
	}
	if e.Version != entryVersion || len(e.Pages) == 0 {
		return Entry{}, false
	}
	return e, true
}

// LookupFile hashes path and returns its cached text, if any. Callers use it
// on files that yielded no text of their own, so the hashing cost falls only
// on scans.
func LookupFile(cacheDir, path string) (Entry, bool) {
	if cacheDir == "" {
		return Entry{}, false
	}
	hash, err := HashFile(path)
	if err != nil {
		return Entry{}, false
	}
	return Load(cacheDir, hash)
}

// Store writes an entry, replacing any earlier one for the same file. The
// write goes to a temporary file first and is renamed into place, so an
// interrupted write cannot leave a half-written entry that later reads as a
// hit.
func Store(cacheDir string, e Entry) error {
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return err
	}
	e.Version = entryVersion
	if e.Created.IsZero() {
		e.Created = time.Now()
	}

	data, err := json.MarshalIndent(e, "", "  ")
	if err != nil {
		return err
	}

	final := entryPath(cacheDir, e.Hash)
	tmp, err := os.CreateTemp(cacheDir, e.Hash+".*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	// Windows will not rename onto an existing file.
	_ = os.Remove(final)
	return os.Rename(tmpName, final)
}

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

// entryVersion is bumped when the stored shape changes, or when the pipeline
// that produced the text changes enough that an old entry is wrong rather than
// merely old. An entry written by an older version is ignored rather than
// misread, which costs one re-run.
//
// 2: recognition asks for orientation detection (--psm 1). Version 1 entries
// for a sideways scan hold rotated gibberish, and a cache is exactly where that
// would otherwise stay forever — nothing about a hit invites re-running it.
const entryVersion = 2

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

// Stats describes what the cache holds. Bytes is what the entries occupy on
// disk, which is the number that answers "is this worth clearing".
type Stats struct {
	Entries int   `json:"entries"`
	Bytes   int64 `json:"bytes"`
}

// Stat counts the cache. A missing or unreadable directory is an empty cache:
// there is nothing the user could do about it and nothing to report.
func Stat(cacheDir string) Stats {
	var s Stats
	if cacheDir == "" {
		return s
	}
	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		return s
	}
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		s.Entries++
		s.Bytes += info.Size()
	}
	return s
}

// Clear removes every cached recognition and returns how many entries went.
//
// Only the files this package writes are touched — entries and the temporaries
// Store may have left behind by an interrupted write. The directory sits next
// to settings.json rather than in a directory of its own, so emptying it
// wholesale would be a licence to delete something that is not ours.
//
// A failure to remove one entry does not stop the rest: a cache half-cleared is
// still better than a cache the button could not touch, and the count returned
// says what actually happened.
func Clear(cacheDir string) (int, error) {
	if cacheDir == "" {
		return 0, nil
	}
	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}

	removed := 0
	var firstErr error
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		switch filepath.Ext(e.Name()) {
		case ".json", ".tmp":
		default:
			continue
		}
		if err := os.Remove(filepath.Join(cacheDir, e.Name())); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		if filepath.Ext(e.Name()) == ".json" {
			removed++
		}
	}
	return removed, firstErr
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

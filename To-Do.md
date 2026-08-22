# To-Do

Findings from an audit of the state after the Go/Wails rework (`a6ea5a7`), with a pass of
fixes applied on top. Completed items are summarized one line each below; what remains open
is ordered by severity further down.

---

## Completed

- **1. Drag & drop upload** — replaced the Electron-only `File.path` drop handler with Wails' native file-drop API.
- **2+3. Highlight rectangles** — highlight geometry moved to the frontend, deriving boxes from PDF.js' text-layer transforms instead of a fragile HTML-regex/`fontSize × 0.5` guess.
- **4. Swallowed backend errors** — bound methods now return real `error`s instead of failing silently.
- **5. `errorMessage` never set** — `useError.js` actually assigns and clears the error message now.
- **7. `.gitignore`** — Electron/Python entries dropped, Wails build artifacts added.
- **8. `build/` directory** — the app icon, manifest and version template moved to `packaging/` under version control, and `build.ps1` copies them into the generated `build/` that Wails insists on reading them from (the ProductName/CompanyName strings still don't surface in Explorer — tracked below).
- **10. App had four different names** — settled on `pdf_scanner` as the identifier and "PDF Scanner" as the only user-facing name, with legacy settings migrated automatically.
- **11. `gofmt`** — the tree is clean; enforcement now just waits on CI (item 6).
- **12. Tests** — `internal/match` and `internal/ocr/textcache` got table-driven unit tests that run without a C toolchain, and immediately caught a real scoring regex bug (remaining coverage gaps tracked below).
- **13. Unbounded goroutine fan-out** — the PDF worker pool is now bounded to `runtime.NumCPU()`.
- **14. Cancellation** — searches are cancellable and a new search safely supersedes an in-flight one.
- **15. Eager HTML extraction** — removed outright rather than made lazy, once highlighting stopped needing it.
- **17. Acrobat-only opener** — the opener now probes the registry, standard install paths, and finally the system handler, so a file always opens.
- **18. `MoveFileToPDFsFolder`** — renamed from `CopyFileToPDFsFolder`, now de-duplicates collisions instead of overwriting.
- **19. PDFs folder in the UI** — the folder is now editable from the sidebar via a native picker and created on startup.
- **22. Dead and inconsistent code** — a cleanup pass removed unused normalisation code, dead refs, and stray console logging.
- **23. Re-running the same keyword** — the stale same-keyword guard is gone, so re-running a search just works.
- **24. Upload feedback** — messages now clear themselves after 5s without leaking `null` into a string ref.
- **25. Confidence** — each hit now shows its match confidence, with the exact score in the tooltip.
- **26. The 85-point threshold** — the threshold itself was fine; the real bug was a non-directional fuzzy match that returned 100 for keyword fragments, now fixed with a measured `[82, 85]` error-free band.
- **27. Scanned PDFs (OCR)** — on-demand Tesseract OCR with a content-hashed cache makes scans searchable, and the engine is now bundled with the build.

---

## P1 — Build and repo hygiene

### 6. No CI — **partly fixed**
[build.ps1](build.ps1) now scripts the local build: it verifies go / node / npm / wails /
gcc and the Wails CLI version, runs `gofmt -l .`, `go vet ./internal/...` and
`go test ./internal/match/`, exports the `__intrinsic_setjmpex` workaround and calls
`wails build -clean`.

**Still open:** nothing runs it automatically. A GitHub Actions workflow should invoke the
same steps — the first three are cheap and catch most regressions without a MuPDF
toolchain, so they can run on `ubuntu-latest`, while the `wails build` job needs a Windows
runner with gcc, Node and the same `CGO_LDFLAGS`.

### 8. Version-info strings don't surface in Explorer
The *string* fields (ProductName, CompanyName, LegalCopyright, Comments) do not show up in
Explorer's file properties even with the language block corrected from `0000` to `0409`. The
fixed `file_version` resolves from the same template, so `wails.json` is being read — this
looks like a Wails 2.15 versioninfo quirk rather than a config error. Worth a bug report.

### 9. Version number — one copy left
`wails.json` → `info.productVersion` is the single source: Vite reads it and defines
`__APP_VERSION__`, and it stamps the `.exe`. `frontend/package.json` still keeps its own
`"version"`, which is inert (the package is `private`) but drifts. Drop it, or script it to
follow `wails.json`.

### 12. Remaining test coverage gaps
Nothing covers `internal/search` (a small fixture PDF under `testdata/` would do it), the
frontend highlight math, or `internal/ocr`'s recognition path — the last of which needs both
the MuPDF toolchain and an installed Tesseract to run at all, so it wants a build-tagged
integration test rather than a unit test.

---

## P2 — Robustness and performance

### 16. Whole-PDF base64 round-trip to the viewer
[app.go](app.go) `LoadPDF` + [usePdfViewer.js](frontend/src/composables/usePdfViewer.js).
The byte-by-byte `for` loop is gone (`Uint8Array.from(atob(b), c => c.charCodeAt(0))`), but
the whole file still crosses the bridge as base64 for every open. For a 50 MB scan that is
still two copies and a visible pause.

Fix: serve PDFs over the Wails asset server (or a custom handler) and hand PDF.js a URL, so
it can range-request instead.

---

## P3 — UI / settings gaps

### 20. Theme is persisted twice and never synced
`Settings.Theme` ([app.go](app.go)) is written to `settings.json` but read by nobody;
[useTheme.js](frontend/src/composables/useTheme.js) uses `localStorage` instead. Two stores,
one of which is a lie. Pick `settings.json` and delete the `localStorage` path, or drop
`Theme` from the Go struct.

### 21. `Settings.Language` is unused; UI is hardcoded German
Either wire up i18n or remove the field.

# To-Do

Findings from an audit of the state after the Go/Wails rework (`a6ea5a7`). Ordered by
severity. Each item names the file so it can be picked up directly.

---

## P0 — Broken functionality

### 1. Drag & drop upload cannot work in Wails
[frontend/src/components/UploadComponent.vue:60-79](frontend/src/components/UploadComponent.vue#L60-L79)

`handleFileDrop` reads `files[i].path`. `File.path` was a **non-standard Electron
extension** — in WebView2 it is `undefined`, so every drop hits the `else` branch and
alerts "Bitte nur PDF-Dateien verschieben", even for valid PDFs.

Fix: enable Wails' native file-drop instead. Set `DragAndDrop: &options.DragAndDrop{
EnableFileDrop: true }` in [main.go](main.go), subscribe with `runtime.OnFileDrop`, and have
the Go side receive real paths. Then delete the DOM `@drop` handler.

### 2. Highlight rectangles are guessed, not measured
[internal/search/search.go:253-297](internal/search/search.go#L253-L297)

`findPositionsInHTML` regex-parses MuPDF's HTML and computes x-offsets as
`runesBefore × (fontSize × 0.5)`. That constant is wrong for every proportional font, so
highlights drift horizontally — increasingly so the further into the line the match sits.
The old Python build used `page.search_for()`, which returns exact quads.

Fix: use go-fitz's structured text output (`doc.TextHTML`/stext with per-span bboxes, or
`doc.SearchTextOnPage`-equivalent) to get real rectangles instead of reconstructing them
from a rendered HTML string. Also drop the fragile `<p style="position:absolute;...">`
regex, which breaks the moment MuPDF changes its HTML serialiser.

### 3. Highlight coordinate space is not reconciled with PDF.js
[frontend/src/composables/usePdfViewer.js:60-76](frontend/src/composables/usePdfViewer.js#L60-L76)

Positions from MuPDF (top-left origin, PDF points) are multiplied by `scale` and used as
raw CSS pixels. This ignores page rotation and a non-zero CropBox origin. Rotated or
cropped pages will highlight the wrong region.

Fix: map through `pdfPage.getViewport(...).convertToViewportPoint()` rather than a bare
`× scale`.

### 4. Backend errors are swallowed — user sees "no results"
- [app.go:109-119](app.go#L109-L119) — `search.SearchPDFs`'s `error` is discarded with `_`.
  A missing/unreadable PDFs folder produces an empty list, and the UI reports
  "Keine Resultate gefunden!" instead of the real cause. The default folder
  (`%USERPROFILE%\Documents\PDFs`) does not exist on a fresh machine, so this is the
  **first thing a new user hits**.
- [app.go:127-130](app.go#L127-L130) — `LoadPDF` discards `os.ReadFile`'s error and returns
  `nil`. The frontend then calls `atob(null)` → `"null"` → PDF.js fails with an unrelated
  parse error.
- [internal/search/search.go:307-311](internal/search/search.go#L307-L311) — a PDF that
  MuPDF cannot open is skipped silently; corrupt/encrypted files vanish without a word.

Fix: return `(T, error)` from these bound methods (Wails maps a Go error to a rejected
promise) and surface it. Collect per-file failures into the result payload so the UI can
say "3 files could not be read".

### 5. `errorMessage` is never set, so inline errors never render
[frontend/src/composables/useError.js:20-25](frontend/src/composables/useError.js#L20-L25)

`throwError` logs and shows a toast but never assigns `errorMessage.value`. Every
`v-if="errorMessage"` block — in
[SearchComponent.vue:16](frontend/src/components/SearchComponent.vue#L16) and
[PdfViewer.vue:23](frontend/src/components/PdfViewer.vue#L23) — is therefore dead markup.
[SearchComponent.vue:99](frontend/src/components/SearchComponent.vue#L99) makes it worse by
assigning the boolean `false` to a string ref.

Fix: have `throwError` set `errorMessage.value = msg`, add a `clearError()`, and use it
instead of `errorMessage.value = false`.

---

## P1 — Build, packaging, repo hygiene

### 6. Clean checkout does not build without the Wails CLI
Verified: `go build ./...` fails (`pattern all:frontend/dist: no matching files found`) and
`npm run build` fails (`Could not resolve "../wailsjs/go/main/App"`). This is normal for
Wails, but nothing in the repo said so — now documented in the README. Remaining work:
add a CI workflow that installs the Wails CLI and runs `wails build`, so the break is
caught rather than discovered by the next person to clone.

### 7. `.gitignore` still describes the Electron/Python app
[.gitignore:26-32](.gitignore#L26-L32)

`release/`, `dist-electron/`, `WPy64*`, `Winpython64*`,
`electron/resources/production_workdir.json` are all obsolete. Nothing ignores the Go/Wails
outputs.

Fix: drop the stale entries, add `build/bin/`, `frontend/wailsjs/`, `*.exe`, `*.syso`.

### 8. No `build/` directory committed
Wails looks for `build/windows/icon.ico`, `wails.exe.manifest` and `info.json`. Without
them the produced `.exe` gets Wails' default icon and no version metadata — even though
`frontend/src/assets/images/logo.png` exists (and is currently unused by anything).

Fix: run `wails generate` / commit a `build/` tree, wire `logo.png` in as `appicon.png`, and
fill `info.json` with product name, version and company.

### 9. Version number is duplicated in three places
`1.3.1` appears in [frontend/package.json:6](frontend/package.json#L6) and is hardcoded as
`v1.3.1` in [frontend/src/App.vue:7](frontend/src/App.vue#L7). [wails.json](wails.json) has
no version at all.

Fix: single source (`wails.json` `info.productVersion`), injected at build time via
`-ldflags`, exposed to the UI through a bound method or a Vite define.

### 10. The app has four different names
`pdf_scanner` (repo) / `pdf-searcher` (go module, npm) / `PDF-Searcher` (wails.json,
window title) / `PDF Searcher` ([frontend/index.html:6](frontend/index.html#L6)) /
`📄PDF Scanner` ([HeaderComponent.vue:3](frontend/src/components/HeaderComponent.vue#L3)).
Pick one.

### 11. All Go files fail `gofmt`
`gofmt -l .` flags [app.go](app.go), [main.go](main.go),
[internal/search/search.go](internal/search/search.go),
[internal/opener/opener.go](internal/opener/opener.go) — mainly the misaligned struct fields
in `SearchResult` and the `var (...)` regex block.

Fix: `gofmt -w .`, then add it to CI.

### 12. No tests anywhere
`advancedMatch`, `normalizeScientificName`, `ngramSimilarity` and `findPositionsInHTML` are
pure functions with tricky behaviour and zero coverage. They are the single best place to
start: table-driven tests plus a small fixture PDF under `internal/search/testdata/`.

---

## P2 — Robustness and performance

### 13. Unbounded goroutine fan-out
[internal/search/search.go:421-434](internal/search/search.go#L421-L434)

One goroutine per PDF, each opening a MuPDF document through cgo. A folder with a few
hundred files will spike memory hard and can exhaust the cgo thread pool.

Fix: bound concurrency to `runtime.NumCPU()` with a semaphore or worker pool.

### 14. Search cannot be cancelled
`SearchPDFs` takes a `context.Context`
([internal/search/search.go:392](internal/search/search.go#L392)) and never reads it. There
is no way to abort a long scan, and starting a second search while the first runs leaves
both writing to the same event channel.

Fix: check `ctx.Err()` in the per-file loop, store a `context.CancelFunc` on `App`, cancel
the previous run when a new search starts, and add a "Suche abbrechen" button.

### 15. Page HTML is extracted even when nothing matches
[internal/search/search.go:322](internal/search/search.go#L322)

`doc.HTML(pageNum, false)` runs for *every* page before any line is scored. HTML
serialisation is the expensive part of the loop and it is wasted on pages with no hits.

Fix: extract it lazily on the first match of a page.

### 16. Whole-PDF base64 round-trip to the viewer
[app.go:127](app.go#L127) + [usePdfViewer.js:128-133](frontend/src/composables/usePdfViewer.js#L128-L133)

`[]byte` crosses the bridge as base64, is `atob`'d, then copied byte-by-byte in a JS `for`
loop. For a 50 MB scan that is three copies in memory and a visible freeze.

Fix: serve PDFs over the Wails asset server (or a custom handler) and hand PDF.js a URL, so
it can range-request. If the bridge stays, at least replace the char loop with
`Uint8Array.from(binary, c => c.charCodeAt(0))`.

### 17. Acrobat-only opener with hardcoded paths
[internal/opener/opener.go:10-24](internal/opener/opener.go#L10-L24)

Four literal paths. Acrobat installed elsewhere, a different Acrobat version, or any other
PDF reader → "Adobe Acrobat Reader not found". There is no fallback to the system default
handler.

Fix: probe the registry (`HKLM\SOFTWARE\...\AcroRd32.exe`) first, then fall back to
`rundll32 url.dll,FileProtocolHandler` / `runtime.BrowserOpenURL` — losing the page jump but
still opening the file. Also `cmd.Start()` has no matching `Wait`, leaking the process
handle.

### 18. `MoveFileToPDFsFolder` neither moves nor guards
[app.go:153-174](app.go#L153-L174)

It *copies*, despite the name, and `os.Create` silently truncates an existing file of the
same name. [UploadComponent.vue:46](frontend/src/components/UploadComponent.vue#L46) then
tells the user the file was "verschoben".

Fix: rename to `CopyFileToPDFsFolder`, detect a name collision and either prompt or
de-duplicate (`name (2).pdf`), and fix the German string.

---

## P3 — UI / settings gaps

### 19. The PDFs folder cannot be changed from the UI
`pdfsFolder` is in `Settings` and `GetPDFsFolder` is bound, but
[SettingsSidebar.vue](frontend/src/components/SettingsSidebar.vue) only renders
`ThemeToggle` and `SearchHistory`. Users are stuck with
`%USERPROFILE%\Documents\PDFs`.

Fix: add a folder row using `runtime.OpenDirectoryDialog`, and create the folder on first
run so an empty search is not the default experience.

### 20. Theme is persisted twice and never synced
`Settings.Theme` ([app.go:29](app.go#L29)) is written to `settings.json` but read by nobody;
[useTheme.js:3](frontend/src/composables/useTheme.js#L3) uses `localStorage` instead. Two
stores, one of which is a lie.

Fix: pick `settings.json` (it already round-trips) and delete the `localStorage` path, or
drop `Theme` from the Go struct.

### 21. `Settings.Language` is unused; UI is hardcoded German
Either wire up i18n or remove the field.

### 22. Dead and inconsistent code
- `normalizeText` / `reNormalize` — [internal/search/search.go:42,54](internal/search/search.go#L54) — never called.
- `indexedResult` — [internal/search/search.go:412](internal/search/search.go#L412) — a one-field struct wrapping a slice for no reason.
- `frontend/src/assets/images/logo.png` — referenced nowhere.
- `groupedResults` is initialised as `ref({})` then reassigned to an array — [SearchComponent.vue:94](frontend/src/components/SearchComponent.vue#L94). `v-if="groupedResults.length > 0"` only works because `{}.length` is `undefined`. Initialise it as `ref([])`.
- `onMounted` imported but unused in [SearchComponent.vue:84](frontend/src/components/SearchComponent.vue#L84).
- `console.log` of full result sets on every search — [SearchComponent.vue:119](frontend/src/components/SearchComponent.vue#L119).
- `useError.js` comment says "remove toast after 5 seconds"; the timeout is 2000 ms.

### 23. Re-running the same keyword silently does nothing
[SearchComponent.vue:100](frontend/src/components/SearchComponent.vue#L100) guards on
`searchKeyword !== activeSearchTerm`, and the `else` branch deliberately blanks the error
message in that case. Adding a PDF and searching the same term again appears to do
nothing at all.

Fix: allow a re-run, or show "bereits gesucht — Ergebnisse unverändert".

### 24. Upload feedback never clears
`successMessage` / `errorMessage` in
[UploadComponent.vue](frontend/src/components/UploadComponent.vue) are set and left. They
also use `null` where the ref was initialised to `""`.

### 25. Confidence is computed but never shown
Every result carries `confidence` and it drives the sort order, yet the UI displays only
the found word and the page. Showing it (a badge, or a "fuzzy" marker below 100) would
explain why a seemingly unrelated hit ranks where it does.

### 26. The 85-point threshold is hardcoded and unexplained
[internal/search/search.go:331](internal/search/search.go#L331). It was tuned for Python's
`rapidfuzz`; go-edlib's Levenshtein similarity is a different scale, so the port may be
letting through noise or dropping good hits. Worth measuring against a fixture set and
exposing as a setting.

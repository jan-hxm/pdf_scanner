# To-Do

Findings from an audit of the state after the Go/Wails rework (`a6ea5a7`), with a pass of
fixes applied on top. Items 1–5, 7, 8, 11, 12, 13, 14, 15, 17, 18, 19, 24, 25, 26 and 27 are
done; what is left is ordered by severity below.

---

## Done

### 1. Drag & drop upload — **fixed**
`File.path` was an Electron extension and is `undefined` in WebView2. Replaced with Wails'
native file drop: `DragAndDrop{EnableFileDrop: true, DisableWebViewDrop: true}` in
[main.go](main.go), `runtime.OnFileDrop` in `startup` re-emitting a `files-dropped` event,
and a `--wails-drop-target: drop` element in
[UploadComponent.vue](frontend/src/components/UploadComponent.vue). The DOM `@drop` handler
is gone.

### 2 + 3. Highlight rectangles — **fixed, differently than proposed**
The suggested fix was to read per-span bboxes out of go-fitz. **That API does not exist** —
neither v1.24.15 nor v1.28.2 exposes structured text; `Document` offers only `Text`, `HTML`,
`SVG`, `Image`, `Links`, `ToC`, `Metadata` and `Bound`.

So the geometry moved to the frontend instead. `SearchResult` no longer carries `Positions`
at all; [usePdfViewer.js](frontend/src/composables/usePdfViewer.js) locates the term in
PDF.js' own text layer and derives each box from the text item's `transform` composed with
`viewport.transform`. This fixes items 2 and 3 together: real glyph metrics instead of
`fontSize × 0.5`, and rotation plus CropBox handled by the viewport rather than a bare
`× scale`. The fragile `<p style="position:absolute;…">` regex is deleted, and as a
side-effect item 15 disappears — `doc.HTML` is no longer called at all.

Verified against a synthetic 612×792 page at scale 1 and 1.2 and rotations 0/90/180: boxes
land on the glyphs, and sub-ranges ("coli" within "Hello E. coli!") are placed
proportionally within the run.

Two consequences worth knowing:
- Highlights now survive zooming and follow you across pages, which they did not before.
- A scanned page with no text layer gets no highlight. The viewer says so instead of
  drawing a box in the wrong place.

### 4. Swallowed backend errors — **fixed**
`SearchPDFs`, `LoadPDF`, `SelectFiles`, `CopyFileToPDFsFolder` and `SelectPDFsFolder` all
return `error` now. A missing folder is reported as such. Files MuPDF cannot open are
collected into `Outcome.Failures` and shown as "⚠️ *n* Dateien konnten nicht gelesen
werden: …". The PDFs folder is also created on startup, so the default is no longer a
folder that does not exist.

### 5. `errorMessage` never set — **fixed**
[useError.js](frontend/src/composables/useError.js) now assigns `errorMessage.value` and
exports `clearError()`. The `errorMessage.value = false` assignment is gone.

### 7. `.gitignore` — **fixed**
Electron/Python entries dropped; `build/bin/`, `frontend/wailsjs/`, `*.exe`, `*.syso` and
`frontend/package.json.md5` added.

### 8. `build/` directory — **partly fixed**
Committed, with `frontend/src/assets/images/logo.png` wired in as `build/appicon.png`
(which also retires the unused-asset half of item 22). `wails.json` has an `info` block, and
the `.exe` now carries the app icon and a `FileVersion` from `wails.json`.

**Still open:** the *string* fields (ProductName, CompanyName, LegalCopyright, Comments) do
not surface in Explorer even with the language block corrected from `0000` to `0409`. The
fixed `file_version` resolves from the same template, so `wails.json` is being read — this
looks like a Wails 2.15 versioninfo quirk rather than a config error. Worth a bug report.

### 11. `gofmt` — **fixed**
`gofmt -l .` is clean. Still needs to be enforced by CI (item 6).

### 12. Tests — **started**
The scoring logic moved into a new `internal/match` package with **no PDF dependency**, so
its tests run without cgo or a MuPDF toolchain — `go test ./internal/match/` works on a bare
checkout. Table-driven coverage for `NormalizeScientificName`, `NgramSimilarity` and
`Score`, including threshold behaviour and the invariant that the returned word is always a
substring of the line (the viewer searches the page for it).

**These tests immediately found a real bug**, now fixed: strategy 2 built the pattern
`[eE]\.?\s*…[iI]\.?\s*` and then trimmed 4 characters to drop the trailing `\s*` — but
`\s*` is 3 characters, so the trim also ate the `?` and left a **mandatory** `\.`. "E. coli"
therefore only matched when followed by a period, and otherwise fell through to strategy 3,
scoring 90 instead of 95 and returning a different matched word. The pattern is now built
with the separator between letters only.

Item 27 added a second package on the same principle: `internal/ocr/textcache` holds the
recognised-text cache and imports no PDF code, so it tests on a bare checkout too.

**Still open:** nothing covers `internal/search` (a small fixture PDF under `testdata/`
would do it), the frontend highlight math, or `internal/ocr`'s recognition path — the last
of which needs both the MuPDF toolchain and an installed Tesseract to run at all, so it
wants a build-tagged integration test rather than a unit test.

### 13. Unbounded goroutine fan-out — **fixed**
Worker pool bounded to `runtime.NumCPU()` via a semaphore.

### 14. Cancellation — **fixed**
`processPDF` checks `ctx.Err()` per page. `App` keeps a `context.CancelFunc` plus a
generation counter, so starting a search cancels the previous one and a finishing search
never clears a newer one's cancel func. A `CancelSearch` binding backs a "Suche abbrechen"
button. Results now come back as the bound method's **return value** rather than an event,
so a superseded run cannot overwrite a newer one's output; `Outcome.Cancelled` marks a
partial run so it is discarded rather than shown as complete.

### 15. Eager HTML extraction — **gone**
Not made lazy — removed. See item 2.

### 17. Acrobat-only opener — **fixed**
[opener.go](internal/opener/opener.go) probes the four `App Paths` registry keys first, then
the standard install paths, then falls back to `rundll32 url.dll,FileProtocolHandler` so the
file always opens (losing only the page jump). `cmd.Start()` is paired with
`Process.Release()`, so the handle is no longer leaked.

### 18. `MoveFileToPDFsFolder` — **fixed**
Renamed `CopyFileToPDFsFolder`, returns the name it stored under, and de-duplicates
collisions as `name (2).pdf` using `O_EXCL` rather than truncating. The German string says
"kopiert".

### 19. PDFs folder in the UI — **fixed**
`SelectPDFsFolder` (native `OpenDirectoryDialog`, persisted) plus a
[PdfsFolder.vue](frontend/src/components/PdfsFolder.vue) row in the settings sidebar. The
folder is created on startup.

### 24. Upload feedback — **fixed**
Messages clear themselves after 5s and no longer mix `null` into a `""` ref.

### 25. Confidence — **fixed**
Each hit shows "exakt" or "~*n*%", with the exact score in the tooltip.

### 26. The 85-point threshold — **measured; the real bug was elsewhere**
The threshold was not the problem. `Score` was.

Strategy 4 called `fuzzyPartialRatio(token, keyword)`, which — when the token was *shorter*
than the keyword — slid the token along the keyword and returned the best window. That asks
"is this token a piece of the keyword", the reverse of the question the search is asking,
and it returns 100 for any token that happens to be a fragment. Searching a real file the search term matched various abbriviations, each at confidence
100 — indistinguishable from a verbatim match, so *no* threshold separated them. Item 26
was blaming the gate for a bug behind it.

Fixed by making the comparison directional: the keyword is slid along tokens *longer* than
it, and shorter tokens get the plain ratio, which charges them for the length they are
missing. The same search now returns **1 hit, the correct one**. Fuzzy recall is unaffected.

The error-free band is **[82, 85]** and `MinConfidence` stays at **85**, at its top edge, on
purpose: above it the errors are missed hits, below it they are near-miss. The floor (85.71) is a one-letter typo in a
seven-letter word — below roughly that length edit distance genuinely cannot tell a typo
from a different word, which is not something a threshold can fix.

The sweep fails if `MinConfidence` ever drifts out of the band, and fails with a distinct
message if *no* threshold classifies the set — i.e. if the scorer regresses the way it had.

### 27. Scanned PDFs cannot be searched at all — **fixed with on-demand OCR**

Found while investigating item 26. A scanned pdf yields **zero** characters from `doc.Text` on every page: a pure image scan with no
text layer, which no amount of scoring could ever search.

Of the two options this item listed, **filename matching was explicitly rejected** — it
cannot say which page, and a file that happens to be *named* after the term is not the same
as a file that contains it. OCR was built instead.

**How it runs.** Not during the search. Recognition costs seconds per page, so scanning a file with numerous pages
takes minutes; a search that stopped to OCR would hang on the first run after a scan is
added with nothing on screen. The search returns and reports its textless files, the new
[OcrPanel.vue](frontend/src/components/OcrPanel.vue) offers to recognise them, and the
results already on screen stay readable while it works. When it finishes, `SearchComponent`
re-runs the same keyword and the scan has hits — marked `OCR`, since a scan has no text
layer for the viewer to highlight.

**The language is asked for every run.** Tesseract has no language identification: `--psm 0`
detects script and orientation, and German and English are both Latin. Something has to name
the model, and a remembered answer is silently wrong for the next document. The picker lists
the installed languages and offers a combined `deu+eng` when both are there, because
Tesseract takes several at once and lets them compete per word. Deliberately not persisted —
`Settings.Language` is the *UI* locale from item 21 and conflating the two would bite later.

**The cache is what makes it viable.** [internal/ocr/textcache](internal/ocr/textcache/)
stores one JSON file per document under `%AppData%\pdf_scanner\ocr-cache\`, named for the
SHA-256 of its bytes:

- Keyed on content, so a rename, a move or a re-import reuses the text and an edit re-runs.
- Language recorded *in* the entry, not folded into the key — two entries for one document
  would leave the search with no way to choose between them, so re-running in another
  language replaces it.
- Only textless files are hashed, so the cost lands on the handful of scans rather than on
  every file in the folder on every search.
- A cancelled run caches nothing. Half a document would otherwise read back as a whole one
  forever.
- Writes go via a temp file and a rename; a damaged, truncated or out-of-date entry reads as
  a miss, because the cost of a miss is one re-run and the cost of trusting it is a document
  that silently searches wrong.

**Tesseract is a subprocess, not a library.** gosseract would link libtesseract through cgo,
a second native dependency in a build that already needs a linker workaround for MuPDF. One
process per page is a rounding error next to recognition. `ocr.Locate()` prefers a copy
bundled at `<exe dir>/tesseract/`, then `PATH`, then the usual installer paths. Pages are
rasterised with `doc.ImagePNG(page, 300)` and piped in on stdin — no temp files. Each
process gets `OMP_THREAD_LIMIT=1` with one process per core, since letting them each fan out
again oversubscribes the machine enough to be slower than running them serially.

**Verified** against a real file: the search still returns all files scanned and a single
correct hit for the one textless file. With a cache entry seeded for that
scan it returns three hits — two from the scan at `Match: "ocr"` with correct page numbers
and occurrence counts — and reports zero textless files. `internal/ocr/textcache` has
table-driven coverage for hashing, the round trip, replacement, and every way an entry can
be unusable; it imports no PDF code, so like `internal/match` it tests without a C
toolchain. [build.ps1](build.ps1) runs both.

**Still open:** Tesseract is not bundled — the app detects its absence and says which files
are scans and what to install, but the user has to install it. Bundling it plus
`deu.traineddata` in `build/` would make OCR work out of the box; the loader already prefers
a bundled copy, so it is a packaging step rather than a code change.

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

### 9. Version number — **mostly fixed, one copy left**
`wails.json` → `info.productVersion` is now the single source: Vite reads it and defines
`__APP_VERSION__`, and it stamps the `.exe`. [App.vue](frontend/src/App.vue) no longer
hardcodes a version.

**Still open:** `frontend/package.json` keeps its own `"version"`, which is inert (the
package is `private`) but drifts — it was already at `2.0.0` while `wails.json` said
`1.3.1`. `wails.json` has been aligned to `2.0.0`; the npm field should just be dropped or
scripted to follow.

### 10. The app has four different names — **fixed**
Settled on one identifier and one display name:

- `pdf_scanner` — repo, Go module ([go.mod](go.mod)), npm package
  ([frontend/package.json](frontend/package.json)), `wails.json` → `name`, and the settings
  directory `%AppData%\pdf_scanner\`.
- **PDF Scanner** — everything the user sees: window title ([main.go](main.go)),
  `<title>` and the noscript notice ([frontend/index.html](frontend/index.html)), the header
  ([HeaderComponent.vue](frontend/src/components/HeaderComponent.vue)), `productName` (so
  the `.exe` properties read it too), and the binary `PDF Scanner.exe`.

The settings location moved with the rename. `migrateLegacySettings()` in
[app.go](app.go) renames an existing `%AppData%\pdf-searcher\` to the new directory on
startup, once, and only when the new one does not already exist — so an existing install
keeps its search history and PDFs folder.

The stale `build/bin/PDF-Searcher.exe` from a pre-rename build is gone —
[build.ps1](build.ps1) passes `-clean`, so every build starts from an empty `build/bin`.

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

### 22. Dead and inconsistent code — **mostly fixed**
Cleared: `normalizeText`/`reNormalize`, `indexedResult`, the unused `logo.png` (now the app
icon), `groupedResults` initialised as `ref([])`, the unused `onMounted` import, and the
`console.log` of full result sets. `useError.js`'s comment matches its 2000 ms timeout.

**Still open:** nothing known — re-audit after the next feature.

### 23. Re-running the same keyword — **fixed**
The `searchKeyword !== activeSearchTerm` guard is gone; a re-run just runs. Added a PDF and
want to search the same term again? It works.

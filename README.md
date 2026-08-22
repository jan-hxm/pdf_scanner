# PDF Scanner

A Windows desktop app that fuzzy-searches a folder of PDFs for a keyword, lists every hit
grouped by file and page, and shows the match either in the built-in PDF.js viewer with the
hit highlighted, or in an external reader on the right page.

- Recursive search over a configurable PDF folder, with a progress bar and a cancel button
- Fuzzy matching: exact substring, normalised scientific names (`L. orem` ≡ `l.orem`),
  token equality, Levenshtein and 3-gram similarity — each hit shows its confidence
- Results grouped per file, expandable per page, with the matching line quoted underneath
  and the term marked in it
- Counts say what they mean: a file's badge is every occurrence it contains, the page count
  sits next to it, and the heading totals both across the run
- Sort by relevance, hit count or file name; ties resolve the same way on every run
- In-app viewer with highlights that survive zoom and page changes, plus a colour picker
- Scanned PDFs are detected, and can be made searchable with on-demand OCR whose result is
  cached per document — recognition is paid once, not once per search
- Add PDFs by drag & drop or a native file dialog; name collisions are de-duplicated
- Light/dark theme and a search history in the ⚙️ sidebar

The UI is German. Built with Go + [Wails v2](https://wails.io/), Vue 3 and pdfjs-dist;
text extraction uses MuPDF through [go-fitz](https://github.com/gen2brain/go-fitz), and
OCR shells out to Tesseract, which the build ships beside the `.exe`.

## Requirements

- Windows 10/11
- Go 1.27+, Node.js 20+, and a C toolchain (`go-fitz` is cgo — MSYS2/mingw-w64 `gcc`)
- Wails CLI v2.15+: `go install github.com/wailsapp/wails/v2/cmd/wails@latest`
- WebView2 runtime (preinstalled on Windows 11)
- Adobe Acrobat Reader — optional; without it, "öffnen" falls back to the default PDF
  handler and loses the jump to the page
- [Tesseract OCR](https://github.com/UB-Mannheim/tesseract/wiki) — needed only to search
  scanned PDFs, and **the build bundles it for you** (see below), so there is nothing to
  install. A build made with `-SkipTesseract` falls back to an installed copy or a
  `tesseract\` folder next to the `.exe`; with neither, the app still reports which files
  are scans, it just cannot read them.
- [7-Zip](https://www.7-zip.org/) — only to bundle Tesseract, and only when there is no
  Tesseract installed to copy from: `winget install -e --id 7zip.7zip`

## Running it

[build.ps1](build.ps1) wraps the Wails CLI with everything a build needs — a toolchain
check, the Go checks, the mingw-w64 link workaround, a clean `build/bin`, the packaging
assets and the bundled OCR engine:

```powershell
.\build.ps1                  # production binary -> "build/bin/PDF Scanner.exe"
.\build.ps1 -Dev             # live-reload development
.\build.ps1 -SkipChecks      # skip gofmt / go vet / go test
.\build.ps1 -SkipTesseract   # build without bundling the OCR engine
```

If the script is blocked by the execution policy, run it as
`powershell -ExecutionPolicy Bypass -File .\build.ps1`.

Underneath it is just the Wails CLI — it generates the Go↔JS bindings, builds the frontend
and compiles Go in one step. Plain `go build ./...` or `npm run build` do not work on a
clean checkout. To drive it by hand:

```sh
wails doctor   # verify gcc, Node and WebView2
wails dev      # live-reload development
wails build    # production binary -> "build/bin/PDF Scanner.exe"
```

On recent mingw-w64 (v12+), linking fails with `undefined reference to
'__intrinsic_setjmpex'`. Alias the symbol at link time — this is what the script sets for
you:

```sh
CGO_LDFLAGS="-Wl,--defsym=__intrinsic_setjmpex=_setjmpex" wails build
```

### The bundled OCR engine

[Get-Tesseract.ps1](Get-Tesseract.ps1) stages Tesseract, with German and English language
data, into `build/tesseract`; `build.ps1` then copies that tree beside the `.exe`, which is
the first place the app looks for an engine. OCR therefore works on a machine where nothing
is installed. The step runs on every build and does nothing when the tree is already there.

Tesseract comes from the first source that answers: a directory named with `-From`, an
installation already on the machine, or the UB Mannheim installer, downloaded once into
`build/.cache` and unpacked with 7-Zip. Unpacking is not installing — 7-Zip reads the NSIS
archive without running it, so nothing lands on the system and no elevation is involved.
The script also runs on its own:

```powershell
.\Get-Tesseract.ps1                                        # deu + eng
.\Get-Tesseract.ps1 -Languages deu,eng,fra                 # more languages
.\Get-Tesseract.ps1 -From 'C:\Program Files\Tesseract-OCR' # copy an existing installation
.\Get-Tesseract.ps1 -TessdataRepo tessdata_best            # slower, more accurate models
```

The staged copy is verified by running it: it has to report every requested language
through `--list-langs`, which is the question `OCRStatus` asks at runtime, and it has to
read a rendered word back correctly. A build whose Tesseract cannot be staged still
succeeds — it warns, and the app goes on reporting a missing engine as a state rather than
an error.

The bundle is about 165 MB next to a 22 MB `.exe`; the training tools and language models
nobody asked for are stripped, which is 74 MB of what the installer carries. If that
matters more than out-of-the-box OCR, `-SkipTesseract` leaves it out.

## Using it

1. On first run the app searches `%USERPROFILE%\Documents\PDFs`, creating it if missing.
   Change the folder under ⚙️ → **PDF-Ordner**.
2. **📄 Neue PDF-Datei hinzufügen** — click the drop area to pick files, or drag PDFs onto it.
3. Type a keyword and hit **🔍 Suchen**.
4. Click a filename to expand its pages, then a page to render it with the match highlighted,
   or **öffnen** to open it in an external reader at that page.
5. If the search reports files with no searchable text, pick a language and press
   **Text erkennen**. Recognition runs after the results are already on screen and takes a
   few seconds per page; when it finishes the search re-runs by itself and those files
   return hits, marked **OCR**. The text is kept, so every later search finds it at once.

Settings live in `%AppData%\pdf_scanner\settings.json`, recognised text in
`%AppData%\pdf_scanner\ocr-cache\`.

Open items are tracked in [To-Do.md](To-Do.md).

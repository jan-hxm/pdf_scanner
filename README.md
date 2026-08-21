# PDF Scanner

A Windows desktop app that fuzzy-searches a folder of PDFs for a keyword, lists every hit
grouped by file and page, and shows the match either in the built-in PDF.js viewer with the
hit highlighted, or in an external reader on the right page.

- Recursive search over a configurable PDF folder, with a progress bar and a cancel button
- Fuzzy matching: exact substring, normalised scientific names (`E. coli` ≡ `e.coli`),
  token equality, Levenshtein and 3-gram similarity — each hit shows its confidence
- Results grouped per file, expandable per page
- In-app viewer with highlights that survive zoom and page changes, plus a colour picker
- Add PDFs by drag & drop or a native file dialog; name collisions are de-duplicated
- Light/dark theme and a search history in the ⚙️ sidebar

The UI is German. Built with Go + [Wails v2](https://wails.io/), Vue 3 and pdfjs-dist;
text extraction uses MuPDF through [go-fitz](https://github.com/gen2brain/go-fitz).

## Requirements

- Windows 10/11
- Go 1.27+, Node.js 20+, and a C toolchain (`go-fitz` is cgo — MSYS2/mingw-w64 `gcc`)
- Wails CLI v2.15+: `go install github.com/wailsapp/wails/v2/cmd/wails@latest`
- WebView2 runtime (preinstalled on Windows 11)
- Adobe Acrobat Reader — optional; without it, "öffnen" falls back to the default PDF
  handler and loses the jump to the page

## Running it

[build.ps1](build.ps1) wraps the Wails CLI with everything a build needs — a toolchain
check, the Go checks, the mingw-w64 link workaround and a clean `build/bin`:

```powershell
.uild.ps1                 # production binary -> "build/bin/PDF Scanner.exe"
.uild.ps1 -Dev            # live-reload development
.uild.ps1 -SkipChecks     # skip gofmt / go vet / go test
```

If the script is blocked by the execution policy, run it as
`powershell -ExecutionPolicy Bypass -File .uild.ps1`.

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

## Using it

1. On first run the app searches `%USERPROFILE%\Documents\PDFs`, creating it if missing.
   Change the folder under ⚙️ → **PDF-Ordner**.
2. **📄 Neue PDF-Datei hinzufügen** — click the drop area to pick files, or drag PDFs onto it.
3. Type a keyword and hit **🔍 Suchen**.
4. Click a filename to expand its pages, then a page to render it with the match highlighted,
   or **öffnen** to open it in an external reader at that page.

Settings live in `%AppData%\pdf_scanner\settings.json`.

Open items are tracked in [To-Do.md](To-Do.md).

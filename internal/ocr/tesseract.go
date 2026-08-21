package ocr

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Tesseract is invoked as a subprocess rather than linked in through gosseract.
// Linking would add a second native dependency to a build that already needs a
// workaround to get MuPDF's static library past mingw, and buys nothing here:
// one process per page is a rounding error next to the recognition itself.
const exeName = "tesseract.exe"

// bundledDir is where a copy shipped with the app is looked for, relative to
// the running executable. Dropping a Tesseract installation there makes OCR
// work without the user installing anything system-wide.
const bundledDir = "tesseract"

// standardPaths are the locations the common Windows installers use, tried
// after the bundled copy and PATH.
func standardPaths() []string {
	paths := []string{
		`C:\Program Files\Tesseract-OCR\` + exeName,
		`C:\Program Files (x86)\Tesseract-OCR\` + exeName,
	}
	if local := os.Getenv("LOCALAPPDATA"); local != "" {
		paths = append(paths,
			filepath.Join(local, "Programs", "Tesseract-OCR", exeName),
			filepath.Join(local, "Tesseract-OCR", exeName),
		)
	}
	return paths
}

// Locate returns the path to the Tesseract executable, or "" if it is not
// installed. A bundled copy wins over an installed one, so shipping a known
// version cannot be silently overridden by whatever is on PATH.
func Locate() string {
	if exe, err := os.Executable(); err == nil {
		bundled := filepath.Join(filepath.Dir(exe), bundledDir, exeName)
		if _, err := os.Stat(bundled); err == nil {
			return bundled
		}
	}
	if path, err := exec.LookPath("tesseract"); err == nil {
		return path
	}
	for _, path := range standardPaths() {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return ""
}

// Info describes the engine available on this machine. Reason explains an
// unavailable engine in words the UI can show directly.
type Info struct {
	Available bool     `json:"available"`
	Path      string   `json:"path"`
	Version   string   `json:"version"`
	Languages []string `json:"languages"`
	Reason    string   `json:"reason"`
}

// Status probes for Tesseract and reports what it can do. It never returns an
// error: "not installed" is an expected state the UI has to render, not a
// failure to hand to the error toast.
func Status() Info {
	exe := Locate()
	if exe == "" {
		return Info{
			Languages: []string{},
			Reason:    "Tesseract OCR ist nicht installiert.",
		}
	}

	langs, err := listLanguages(exe)
	if err != nil {
		return Info{
			Path:      exe,
			Languages: []string{},
			Reason:    fmt.Sprintf("Tesseract wurde gefunden (%s), lässt sich aber nicht ausführen: %v", exe, err),
		}
	}
	if len(langs) == 0 {
		return Info{
			Path:      exe,
			Version:   version(exe),
			Languages: []string{},
			Reason:    "Tesseract ist installiert, aber es sind keine Sprachdaten vorhanden.",
		}
	}

	return Info{
		Available: true,
		Path:      exe,
		Version:   version(exe),
		Languages: langs,
	}
}

// command builds a Tesseract invocation. OMP_THREAD_LIMIT=1 keeps each process
// single-threaded: the caller already runs one process per core, and letting
// every one of them fan out again oversubscribes the machine badly enough to
// be slower than running them one at a time.
func command(ctx context.Context, exe string, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, exe, args...)
	cmd.Env = append(os.Environ(), "OMP_THREAD_LIMIT=1")

	// A bundled copy needs to be told where its language data lives; an
	// installed one already knows.
	if tessdata := filepath.Join(filepath.Dir(exe), "tessdata"); dirExists(tessdata) {
		cmd.Env = append(cmd.Env, "TESSDATA_PREFIX="+tessdata)
	}
	hideWindow(cmd)
	return cmd
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// version returns the engine version, or "" if it cannot be determined. Only
// ever shown to the user, so a failure is not worth propagating.
func version(exe string) string {
	out, err := command(context.Background(), exe, "--version").Output()
	if err != nil {
		return ""
	}
	line, _, _ := strings.Cut(string(out), "\n")
	return strings.TrimSpace(line)
}

// listLanguages returns the installed language codes. Tesseract prints a
// header line before them, and "osd" is orientation and script detection
// rather than a language, so neither belongs in a language picker.
func listLanguages(exe string) ([]string, error) {
	out, err := command(context.Background(), exe, "--list-langs").Output()
	if err != nil {
		return nil, err
	}

	langs := []string{}
	for _, line := range strings.Split(string(out), "\n") {
		code := strings.TrimSpace(line)
		if code == "" || strings.HasSuffix(code, ":") || code == "osd" {
			continue
		}
		langs = append(langs, code)
	}
	return langs, nil
}

// recognize runs one page image through Tesseract and returns its text. The
// image goes in on stdin and the text comes back on stdout, so no temporary
// files are involved.
func recognize(ctx context.Context, exe string, png []byte, languages string) (string, error) {
	cmd := command(ctx, exe, "-", "-", "-l", languages)
	cmd.Stdin = bytes.NewReader(png)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return "", ctxErr
		}
		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			return "", err
		}
		return "", fmt.Errorf("%v: %s", err, detail)
	}
	return stdout.String(), nil
}

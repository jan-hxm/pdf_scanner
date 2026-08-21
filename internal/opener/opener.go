package opener

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// standardPaths are the usual Acrobat install locations, tried after the
// registry lookup.
var standardPaths = []string{
	`C:\Program Files (x86)\Adobe\Acrobat Reader DC\Reader\AcroRd32.exe`,
	`C:\Program Files\Adobe\Acrobat Reader DC\Reader\AcroRd32.exe`,
	`C:\Program Files\Adobe\Acrobat DC\Acrobat\Acrobat.exe`,
	`C:\Program Files (x86)\Adobe\Acrobat DC\Acrobat\Acrobat.exe`,
	`C:\Program Files\Adobe\Acrobat\Acrobat\Acrobat.exe`,
}

// registryKeys are App Paths entries Windows itself uses to resolve these
// executables, so they find Acrobat wherever it was installed.
var registryKeys = []string{
	`HKLM\SOFTWARE\Microsoft\Windows\CurrentVersion\App Paths\AcroRd32.exe`,
	`HKLM\SOFTWARE\Microsoft\Windows\CurrentVersion\App Paths\Acrobat.exe`,
	`HKLM\SOFTWARE\WOW6432Node\Microsoft\Windows\CurrentVersion\App Paths\AcroRd32.exe`,
	`HKLM\SOFTWARE\WOW6432Node\Microsoft\Windows\CurrentVersion\App Paths\Acrobat.exe`,
}

// findAcrobat locates Acrobat, or returns "" if it is not installed.
func findAcrobat() string {
	for _, key := range registryKeys {
		if path := queryRegistryDefault(key); path != "" {
			if _, err := os.Stat(path); err == nil {
				return path
			}
		}
	}
	for _, path := range standardPaths {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return ""
}

// queryRegistryDefault reads a key's (Default) value via reg.exe, which avoids
// pulling in golang.org/x/sys/windows/registry for four lookups.
func queryRegistryDefault(key string) string {
	out, err := exec.Command("reg", "query", key, "/ve").Output()
	if err != nil {
		return ""
	}
	// Output looks like:  (Default)    REG_SZ    C:\...\AcroRd32.exe
	_, after, found := strings.Cut(string(out), "REG_SZ")
	if !found {
		return ""
	}
	line, _, _ := strings.Cut(after, "\n")
	return strings.Trim(strings.TrimSpace(line), `"`)
}

// OpenPDFAtPage opens the PDF at the given path on the specified page.
//
// Acrobat is preferred because it is the only common reader that honours the
// page argument. Without it the file still opens — in whatever application
// Windows has registered for PDFs — just on page 1.
func OpenPDFAtPage(pdfPath string, page int) error {
	absPath, err := filepath.Abs(pdfPath)
	if err != nil {
		absPath = pdfPath
	}
	if _, err := os.Stat(absPath); err != nil {
		return err
	}

	if acrobatPath := findAcrobat(); acrobatPath != "" {
		if err := start(exec.Command(acrobatPath, "/A", "page="+strconv.Itoa(page), absPath)); err == nil {
			return nil
		}
	}

	// Fallback: hand the file to the system's default PDF handler.
	return start(exec.Command("rundll32", "url.dll,FileProtocolHandler", absPath))
}

// start launches cmd and releases the process handle. The reader outlives this
// app, so there is nothing to Wait for — but the handle must not be leaked.
func start(cmd *exec.Cmd) error {
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}

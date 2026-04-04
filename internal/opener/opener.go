package opener

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
)

var standardPaths = []string{
	`C:\Program Files (x86)\Adobe\Acrobat Reader DC\Reader\AcroRd32.exe`,
	`C:\Program Files\Adobe\Acrobat Reader DC\Reader\Acrobat.exe`,
	`C:\Program Files (x86)\Adobe\Acrobat DC\Acrobat\AcroRd32.exe`,
	`C:\Program Files\Adobe\Acrobat DC\Acrobat\Acrobat.exe`,
}

func findAcrobat() string {
	for _, path := range standardPaths {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return ""
}

// OpenPDFAtPage opens the PDF at the given path on the specified page in Adobe Reader.
func OpenPDFAtPage(pdfPath string, page int) error {
	acrobatPath := findAcrobat()
	if acrobatPath == "" {
		return fmt.Errorf("Adobe Acrobat Reader not found")
	}

	pageArg := "page=" + strconv.Itoa(page)
	cmd := exec.Command(acrobatPath, "/A", pageArg, pdfPath)
	return cmd.Start()
}

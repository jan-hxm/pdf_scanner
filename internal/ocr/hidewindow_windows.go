//go:build windows

package ocr

import (
	"os/exec"
	"syscall"
)

// hideWindow keeps Tesseract from flashing a console window on screen. A
// 31-page scan means 31 processes, and without this the user watches 31 black
// rectangles appear and vanish.
func hideWindow(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
}

//go:build !windows

package ocr

import "os/exec"

// hideWindow is a no-op off Windows. The app only ships for Windows, but the
// cheap Go checks are meant to run on a Linux CI runner, and SysProcAttr's
// HideWindow field does not exist there.
func hideWindow(cmd *exec.Cmd) {}

//go:build windows

package procutil

import (
	"os/exec"
	"syscall"
)

// Hide keeps cmd's console window from showing, keeping any other process
// attributes already set.
func Hide(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.HideWindow = true
}

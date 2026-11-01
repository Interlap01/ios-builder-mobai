//go:build !windows

package procutil

import "os/exec"

// Hide does nothing: only Windows gives console programs a window.
func Hide(cmd *exec.Cmd) {}

// Package procutil starts console programs without a window of their own.
// A GUI program on Windows that embeds this module (the MobAI desktop app)
// would otherwise flash a console window for every git, flutter or npx run.
package procutil

import (
	"context"
	"os/exec"
)

// Command is exec.Command with the program's console window hidden.
func Command(name string, arg ...string) *exec.Cmd {
	cmd := exec.Command(name, arg...)
	Hide(cmd)
	return cmd
}

// CommandContext is exec.CommandContext with the program's console window
// hidden.
func CommandContext(ctx context.Context, name string, arg ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, name, arg...)
	Hide(cmd)
	return cmd
}

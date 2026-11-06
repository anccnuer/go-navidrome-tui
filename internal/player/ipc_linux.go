//go:build linux

package player

import (
	"os/exec"
	"syscall"
)

// decorateCmd asks the kernel to deliver SIGTERM to mpv if this process dies
// for any reason (including SIGKILL), so audio never keeps playing in the
// background after a crash.
func decorateCmd(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Pdeathsig: syscall.SIGTERM}
}

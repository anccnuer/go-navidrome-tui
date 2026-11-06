//go:build !windows && !linux

package player

import "os/exec"

// decorateCmd is a no-op on platforms without a parent-death signal
// (macOS/BSD). Orphan mpv processes are prevented by handling SIGHUP in main
// and by the explicit Close() on normal exit.
func decorateCmd(_ *exec.Cmd) {}

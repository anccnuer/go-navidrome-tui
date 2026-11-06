//go:build !windows

package player

import "os"

// assignToKillJob is a no-op outside Windows: Linux uses a parent-death signal
// and macOS relies on SIGHUP handling plus the explicit Close() on exit.
func assignToKillJob(_ *os.Process) error { return nil }

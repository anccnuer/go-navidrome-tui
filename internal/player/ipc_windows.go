//go:build windows

package player

import (
	"net"
	"os"
	"os/exec"
	"strconv"
	"syscall"
	"time"

	winio "github.com/Microsoft/go-winio"
)

// ipcAddress returns a per-process named pipe so multiple instances do not
// fight over the same endpoint.
func ipcAddress(name string) string {
	if name == "" {
		name = "ndtui-mpv"
	}
	return `\\.\pipe\` + name + "-" + strconv.Itoa(os.Getpid())
}

func dialIPC(address string, timeout time.Duration) (net.Conn, error) {
	return winio.DialPipe(address, &timeout)
}

// decorateCmd keeps mpv from flashing a console window behind the TUI.
func decorateCmd(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: 0x08000000, // CREATE_NO_WINDOW
	}
}

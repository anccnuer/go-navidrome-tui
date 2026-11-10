//go:build !windows

package player

import (
	"net"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

func ipcAddress(name string) string {
	if name == "" {
		name = "ndtui-mpv"
	}
	return filepath.Join(os.TempDir(), name+"-"+strconv.Itoa(os.Getpid())+".sock")
}

func dialIPC(address string, timeout time.Duration) (net.Conn, error) {
	return net.DialTimeout("unix", address, timeout)
}

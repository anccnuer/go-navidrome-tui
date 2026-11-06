//go:build windows

package player

import (
	"os"
	"os/exec"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

// TestKillOnCloseJobTerminatesChild proves the mechanism that prevents orphan
// mpv processes: closing the last job handle terminates every member.
func TestKillOnCloseJobTerminatesChild(t *testing.T) {
	job, err := newKillOnCloseJob()
	if err != nil {
		t.Fatalf("create job: %v", err)
	}
	cmd := exec.Command(os.Args[0], "-test.run=TestPlayerHelperProcess")
	cmd.Env = append(os.Environ(), "SUBSONIC_TUI_HELPER=1")
	decorateCmd(cmd)
	if err := cmd.Start(); err != nil {
		_ = windows.CloseHandle(job)
		t.Fatalf("start child: %v", err)
	}
	if err := assignToJob(job, cmd.Process); err != nil {
		_ = cmd.Process.Kill()
		_ = windows.CloseHandle(job)
		t.Skipf("cannot assign child to a job in this environment: %v", err)
	}
	done := make(chan struct{})
	go func() {
		_ = cmd.Wait()
		close(done)
	}()

	if err := windows.CloseHandle(job); err != nil {
		t.Fatalf("close job: %v", err)
	}
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		_ = cmd.Process.Kill()
		t.Fatal("child survived closing the kill-on-close job")
	}
}

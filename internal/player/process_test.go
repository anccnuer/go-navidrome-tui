package player

import (
	"io"
	"log/slog"
	"os"
	"os/exec"
	"testing"
	"time"
)

// TestPlayerHelperProcess is re-executed as a long-lived child process so the
// lifecycle tests do not depend on external tools.
func TestPlayerHelperProcess(t *testing.T) {
	if os.Getenv("NAVIDROME_TUI_HELPER") != "1" {
		return
	}
	time.Sleep(30 * time.Second)
}

func TestKillProcessTerminatesChild(t *testing.T) {
	cmd := exec.Command(os.Args[0], "-test.run=TestPlayerHelperProcess")
	cmd.Env = append(os.Environ(), "NAVIDROME_TUI_HELPER=1")
	if err := cmd.Start(); err != nil {
		t.Fatalf("start child: %v", err)
	}
	p := &mpvPlayer{
		log:      slog.New(slog.NewTextHandler(io.Discard, nil)),
		cmd:      cmd,
		procDone: make(chan struct{}),
	}
	go func() {
		_ = cmd.Wait()
		close(p.procDone)
	}()

	if cmd.ProcessState != nil {
		t.Fatal("child exited before killProcess")
	}
	p.killProcess()

	// Exited() is not portable here: on Unix a SIGKILLed process is
	// "signaled", so Exited() stays false. A non-nil ProcessState means
	// Wait() reaped the child, i.e. it is gone on every platform.
	if cmd.ProcessState == nil {
		t.Fatal("child process was not terminated")
	}
}

func TestCloseIsIdempotentAndTerminatesChild(t *testing.T) {
	cmd := exec.Command(os.Args[0], "-test.run=TestPlayerHelperProcess")
	cmd.Env = append(os.Environ(), "NAVIDROME_TUI_HELPER=1")
	if err := cmd.Start(); err != nil {
		t.Fatalf("start child: %v", err)
	}
	p := &mpvPlayer{
		log:      slog.New(slog.NewTextHandler(io.Discard, nil)),
		stop:     make(chan struct{}),
		events:   make(chan Event, 8),
		cmd:      cmd,
		procDone: make(chan struct{}),
	}
	go func() {
		_ = cmd.Wait()
		close(p.procDone)
	}()

	if err := p.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	if err := p.Close(); err != nil {
		t.Fatalf("second close should be a no-op: %v", err)
	}
	if cmd.ProcessState == nil {
		t.Fatal("Close did not terminate the child process")
	}
}

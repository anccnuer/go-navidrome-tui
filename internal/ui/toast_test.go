package ui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/anccnuer/go-navidrome-tui/internal/ui/components"
)

func TestToastLevelsDedupAndCap(t *testing.T) {
	m := testModel(t, true)
	m.setStatusOK("已保存")
	if line := m.statusLine(); !strings.Contains(line, "✓") || !strings.Contains(line, "已保存") {
		t.Fatalf("success toast = %q", line)
	}
	if m.statusWarn {
		t.Fatal("success toast must not mark the status bar as warning")
	}
	m.setStatus("网络错误", true)
	if line := m.statusLine(); !strings.Contains(line, "✕") {
		t.Fatalf("error toast = %q", line)
	}
	if !m.statusWarn {
		t.Fatal("error toast should mark the status bar as warning")
	}
	before := len(m.toasts)
	m.setStatus("网络错误", true)
	if len(m.toasts) != before {
		t.Fatal("a repeated message should extend the existing toast, not append")
	}
	for i := 0; i < 6; i++ {
		m.setStatusOK(fmt.Sprintf("消息-%d", i))
	}
	if len(m.toasts) > maxToasts {
		t.Fatalf("toast queue grew to %d", len(m.toasts))
	}
}

func TestToastReservesOwnRow(t *testing.T) {
	m := testModel(t, true)
	base := m.bodyHeight()
	m.setStatusOK("一条提示")
	if got := m.bodyHeight(); got != base-1 {
		t.Fatalf("bodyHeight = %d, want %d (one row reserved for the toast)", got, base-1)
	}
	view := m.View()
	if rows := len(strings.Split(view, "\n")); rows != m.height {
		t.Fatalf("frame rows = %d, want %d", rows, m.height)
	}
	if !strings.Contains(view, "一条提示") {
		t.Fatal("toast is missing from the frame")
	}
	if !strings.Contains(view, "音乐库") {
		t.Fatal("toast must not overwrite the sidebar/list content")
	}
	if got := m.hint(); got == "一条提示" {
		t.Fatal("status bar hint should no longer be taken over by a toast")
	}
}

func TestToastFadeGeometryAndExpiry(t *testing.T) {
	m := testModel(t, true)
	base := time.Now()
	m.toasts = []toast{{text: "即将淡出的提示", level: ToastOK, born: base, until: base.Add(2 * time.Second)}}
	normal := m.toastLine(base, 80)
	faded := m.toastLine(base.Add(1500*time.Millisecond), 80)
	if normal == faded {
		t.Fatal("the last second should use the faded style")
	}
	for _, line := range []string{normal, faded} {
		if w := components.VisualWidth(line); w != 80 {
			t.Fatalf("toast width = %d", w)
		}
	}
	m.cfg.UI.Animations = false
	if static := m.toastLine(base.Add(1500*time.Millisecond), 80); static == faded {
		t.Fatal("animations=false should disable fading")
	}
	if got := m.toastLine(base, 10); got != "" {
		t.Fatal("toast should be hidden on very narrow terminals")
	}
	m.pruneToasts(base.Add(3 * time.Second))
	if len(m.toasts) != 0 {
		t.Fatal("expired toasts should be pruned by the tick")
	}
}

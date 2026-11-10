package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/subsonic-tui/subsonic-tui/internal/config"
)

func wizardType(m *Model, text string) {
	for _, r := range text {
		m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
}

func wizardPaste(m *Model, text string) {
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(text), Paste: true})
}

func submitWizard(t *testing.T, m *Model) wizardResultMsg {
	t.Helper()
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Enter should produce a save command")
	}
	res, ok := cmd().(wizardResultMsg)
	if !ok || res.err != nil {
		t.Fatalf("wizard result = %#v", cmd())
	}
	return res
}

func fillDemoServer(t *testing.T, m *Model, address string) {
	t.Helper()
	wizardType(m, address)
	m.Update(tea.KeyMsg{Type: tea.KeyTab})
	wizardType(m, "demo")
	m.Update(tea.KeyMsg{Type: tea.KeyTab})
	wizardType(m, "demo")
}

// Regression: a full https:// URL was previously rejected because mouse
// escape sequences (unconsumed when mouse handling was missing) polluted the
// field value and validation used the raw string.
func TestWizardAcceptsHTTPSAddress(t *testing.T) {
	m := testModel(t, false)
	fillDemoServer(t, m, "https://demo.navidrome.org")
	res := submitWizard(t, m)
	if res.server.URL != "https://demo.navidrome.org" {
		t.Fatalf("URL = %q", res.server.URL)
	}
	if res.server.Username != "demo" || res.server.PlainPassword() != "demo" {
		t.Fatalf("credentials = %+v", res.server)
	}
}

func TestWizardCleansPastedMouseNoise(t *testing.T) {
	m := testModel(t, false)
	// Simulate an SGR mouse report leaking into a bracketed paste.
	wizardPaste(m, "\x1b[<0;12;8Mhttps://demo.navidrome.org\x1b[201~")
	wizardType(m, "")
	m.Update(tea.KeyMsg{Type: tea.KeyTab})
	wizardType(m, "demo")
	m.Update(tea.KeyMsg{Type: tea.KeyTab})
	wizardType(m, "demo")
	res := submitWizard(t, m)
	if res.server.URL != "https://demo.navidrome.org" {
		t.Fatalf("URL = %q", res.server.URL)
	}
}

func TestValidateURLNormalizesCommonInputs(t *testing.T) {
	cases := map[string]string{
		"https://demo.navidrome.org":         "https://demo.navidrome.org",
		" https://demo.navidrome.org/ ":      "https://demo.navidrome.org",
		"demo.navidrome.org":                 "https://demo.navidrome.org",
		"localhost:4533":                     "http://localhost:4533",
		"ｈｔｔｐｓ：／／demo.navidrome.org":         "https://demo.navidrome.org",
		"[<0;9;6Mhttps://demo.navidrome.org": "https://demo.navidrome.org",
	}
	for in, want := range cases {
		got, err := validateURL(in)
		if err != nil {
			t.Fatalf("validateURL(%q) error: %v", in, err)
		}
		if got != want {
			t.Fatalf("validateURL(%q) = %q, want %q", in, got, want)
		}
	}
	if _, err := validateURL("ftp://demo.navidrome.org"); err == nil {
		t.Fatal("unsupported schemes should be rejected with a diagnostic")
	}
	if _, err := validateURL(""); err == nil {
		t.Fatal("empty addresses should be rejected")
	}
}

func TestCleanInputAndSecret(t *testing.T) {
	if got := cleanInput("  ｈｔｔｐｓ：／／host  "); got != "  https://host  " {
		t.Fatalf("cleanInput = %q", got)
	}
	if got := cleanSecret("Pａss\u200bword"); got != "Pａssword" {
		t.Fatalf("cleanSecret must preserve full-width characters: %q", got)
	}
	if got := cleanInput("[200~[201~"); got != "" {
		t.Fatalf("paste markers not removed: %q", got)
	}
}

// With bracketed paste disabled the terminal delivers pasted text as plain
// runes (no \x1b[200~ / \x1b[201~ markers). Make sure the wizard still
// accepts a clean URL and strips stray mouse reports that may leak in.
func TestWizardPasteWithoutBracketedMarkers(t *testing.T) {
	for _, pasted := range []string{
		"https://demo.navidrome.org",
		"\x1b[<0;12;8Mhttps://demo.navidrome.org",
	} {
		m := testModel(t, false)
		wizardPaste(m, pasted)
		m.Update(tea.KeyMsg{Type: tea.KeyTab})
		wizardType(m, "demo")
		m.Update(tea.KeyMsg{Type: tea.KeyTab})
		wizardType(m, "demo")
		res := submitWizard(t, m)
		if res.server.URL != "https://demo.navidrome.org" {
			t.Fatalf("pasted %q produced URL %q", pasted, res.server.URL)
		}
	}
}

func TestWizardMouseClickActivatesField(t *testing.T) {
	m := testModel(t, false)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	g := wizardLayout(100, 30)
	if m.wizard.focus != 1 {
		t.Fatalf("fresh wizard should focus the address field, got %d", m.wizard.focus)
	}
	m.Update(tea.MouseMsg{
		Action: tea.MouseActionPress, Button: tea.MouseButtonLeft,
		X: g.cardX + 20, Y: g.firstFieldRow + 2,
	})
	if m.wizard.focus != 2 {
		t.Fatalf("clicking the third row should focus username, got %d", m.wizard.focus)
	}
	wizardType(m, "demo")
	if m.wizard.fields[2].value != "demo" {
		t.Fatalf("typed text went to the wrong field: %+v", m.wizard.fields)
	}
}

func TestWizardButtonClickSubmits(t *testing.T) {
	m := testModel(t, false)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	g := wizardLayout(100, 30)
	fillDemoServer(t, m, "https://demo.navidrome.org")
	_, cmd := m.Update(tea.MouseMsg{
		Action: tea.MouseActionPress, Button: tea.MouseButtonLeft,
		X: g.cardX + 5, Y: g.buttonRow,
	})
	if cmd == nil {
		t.Fatal("clicking the connect button should dispatch a save command")
	}
	if res, ok := cmd().(wizardResultMsg); !ok || res.err != nil {
		t.Fatalf("button submit result = %#v", cmd())
	}
}

func TestWizardPrefilledValueIsReplaced(t *testing.T) {
	w := newWizard(config.Server{
		Name: "old", URL: "http://old.example.com", Username: "olduser",
	})
	if w.focus != 3 {
		t.Fatalf("re-login should focus the empty password field, got %d", w.focus)
	}
	w.focusField(1)
	if !w.fields[1].selectAll {
		t.Fatal("prefilled URL should be selected on focus")
	}
	w.update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("https://demo.navidrome.org")})
	if w.fields[1].value != "https://demo.navidrome.org" {
		t.Fatalf("prefilled URL was appended instead of replaced: %q", w.fields[1].value)
	}
}

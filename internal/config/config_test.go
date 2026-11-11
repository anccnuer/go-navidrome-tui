package config

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestDefaultAndValidate(t *testing.T) {
	c := Default()
	if err := c.Validate(); err != nil {
		t.Fatalf("default config should be valid: %v", err)
	}
	if c.Cache.TTLDuration() != 5*time.Minute {
		t.Fatalf("ttl = %v", c.Cache.TTLDuration())
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	c := Default()
	c.SetServer(Server{Name: "home", URL: "https://music.example.com/", Username: "alice", Password: EncodeSecret("s3cret")})
	c.SetServer(Server{Name: "office", URL: "http://10.0.0.2:4533", Username: "bob", Password: "plain"})
	c.Active = "office"
	c.Theme.Name = "dracula"
	c.Keybindings.App["next"] = []any{"alt+n", "n"}

	if err := c.Save(path); err != nil {
		t.Fatalf("save: %v", err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded.Active != "office" {
		t.Fatalf("active = %q", loaded.Active)
	}
	if len(loaded.Servers) != 2 {
		t.Fatalf("servers = %d", len(loaded.Servers))
	}
	if loaded.Servers[0].URL != "https://music.example.com" {
		t.Fatalf("trailing slash not trimmed: %q", loaded.Servers[0].URL)
	}
	if loaded.Servers[0].PlainPassword() != "s3cret" {
		t.Fatalf("encoded password did not round trip: %q", loaded.Servers[0].PlainPassword())
	}
	if loaded.Theme.Name != "dracula" {
		t.Fatalf("theme = %q", loaded.Theme.Name)
	}
	keys, ok := loaded.Keybindings.App["next"].([]any)
	if !ok || len(keys) != 2 {
		t.Fatalf("keybinding did not round trip: %#v", loaded.Keybindings.App["next"])
	}
}

func TestLoadMissingFileReturnsDefaults(t *testing.T) {
	c, err := Load(filepath.Join(t.TempDir(), "nope.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if c.Cache.PageSize != 60 || !c.UI.ShowCover || c.Theme.Name != "musicfox" {
		t.Fatalf("defaults were lost: %+v", c)
	}
}

func TestLoadMergesDefaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	content := "[theme]\nname = \"nord\"\n\n[[servers]]\nname = \"x\"\nurl = \"http://localhost:4533\"\nusername = \"u\"\npassword = \"p\"\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	c, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if c.Theme.Name != "nord" {
		t.Fatalf("theme = %q", c.Theme.Name)
	}
	if c.Cache.PageSize != 60 || c.Player.Volume != 80 {
		t.Fatalf("defaults were lost: %+v / %+v", c.Cache, c.Player)
	}
	if c.Active != "x" {
		t.Fatalf("active should fall back to first server, got %q", c.Active)
	}
}

func TestCoverProtocolNormalization(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte("[ui]\ncoverProtocol = ' Sixel '\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	c, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if c.UI.CoverProtocol != "sixel" {
		t.Fatalf("coverProtocol = %q, want sixel", c.UI.CoverProtocol)
	}
	c.UI.CoverProtocol = "unknown"
	c.normalize()
	if c.UI.CoverProtocol != "text" {
		t.Fatalf("unknown protocol should fall back to text, got %q", c.UI.CoverProtocol)
	}
}

func TestEnvOverride(t *testing.T) {
	t.Setenv("NAVIDROME_TUI_URL", "https://env.example.com/")
	t.Setenv("NAVIDROME_TUI_USER", "envuser")
	t.Setenv("NAVIDROME_TUI_PASSWORD", "envpass")
	t.Setenv("NAVIDROME_TUI_THEME", "gruvbox")
	c, err := Load(filepath.Join(t.TempDir(), "missing.toml"))
	if err != nil {
		t.Fatal(err)
	}
	s, ok := c.ActiveServer()
	if !ok || s.URL != "https://env.example.com" || s.Username != "envuser" || s.PlainPassword() != "envpass" {
		t.Fatalf("env override failed: %+v, %v", s, ok)
	}
	if c.Theme.Name != "gruvbox" {
		t.Fatalf("theme override failed: %q", c.Theme.Name)
	}
}

// The old variable names must keep working for existing scripts/shells.
func TestLegacyEnvOverride(t *testing.T) {
	t.Setenv("SUBSONIC_TUI_URL", "https://legacy.example.com/")
	t.Setenv("SUBSONIC_TUI_USER", "legacyuser")
	t.Setenv("SUBSONIC_TUI_THEME", "nord")
	c, err := Load(filepath.Join(t.TempDir(), "missing.toml"))
	if err != nil {
		t.Fatal(err)
	}
	s, ok := c.ActiveServer()
	if !ok || s.URL != "https://legacy.example.com" || s.Username != "legacyuser" {
		t.Fatalf("legacy env override failed: %+v, %v", s, ok)
	}
	if c.Theme.Name != "nord" {
		t.Fatalf("legacy theme override failed: %q", c.Theme.Name)
	}
}

func TestLegacyConfigPathFallback(t *testing.T) {
	dir := t.TempDir()
	if runtime.GOOS == "windows" {
		t.Setenv("APPDATA", dir)
	} else {
		t.Setenv("XDG_CONFIG_HOME", dir)
	}
	legacy := filepath.Join(dir, "subsonic-tui")
	if err := os.MkdirAll(legacy, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacy, "config.toml"), []byte("active = \"home\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := Path()
	if err != nil {
		t.Fatal(err)
	}
	if got != filepath.Join(legacy, "config.toml") {
		t.Fatalf("legacy config not reused: %q", got)
	}
}

func TestValidateRejectsBadURL(t *testing.T) {
	c := Default()
	c.Servers = []Server{{Name: "bad", URL: "ftp://host", Username: "u"}}
	if err := c.Validate(); err == nil {
		t.Fatal("expected an error for ftp URL")
	}
	c.Servers = []Server{{Name: "ok", URL: "https://host:4533", Username: ""}}
	if err := c.Validate(); err == nil || !strings.Contains(err.Error(), "username") {
		t.Fatalf("expected username error, got %v", err)
	}
}

func TestAtomicSaveOverExistingFile(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("windows-specific replace path")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	c := Default()
	c.Theme.Name = "aurora"
	if err := c.Save(path); err != nil {
		t.Fatal(err)
	}
	c.Theme.Name = "nord"
	if err := c.Save(path); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(path)
	if err != nil || loaded.Theme.Name != "nord" {
		t.Fatalf("second save failed: %v %+v", err, loaded)
	}
}

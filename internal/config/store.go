package config

import (
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/pelletier/go-toml/v2"
)

// Load reads path, merges it over Default values, applies environment
// overrides and validates the result. A missing file is not an error.
func Load(path string) (*Config, error) {
	cfg := Default()
	data, err := os.ReadFile(path)
	switch {
	case err == nil:
		if err := toml.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("解析配置文件 %s: %w", path, err)
		}
	case errors.Is(err, os.ErrNotExist):
		// First run: the login wizard will create it.
	default:
		return nil, fmt.Errorf("读取配置文件 %s: %w", path, err)
	}
	cfg.normalize()
	cfg.applyEnv()
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

// Save writes the configuration atomically with 0600 permissions.
func (c *Config) Save(path string) error {
	c.normalize()
	if err := c.Validate(); err != nil {
		return err
	}
	data, err := toml.Marshal(c)
	if err != nil {
		return fmt.Errorf("序列化配置: %w", err)
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".config-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpName, 0o600); err != nil && !isWindows() {
		return err
	}

	// Windows cannot rename over an existing file. Keep the old contents so
	// we can roll back if the replacement fails.
	var backup []byte
	if isWindows() {
		backup, _ = os.ReadFile(path)
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	if err := os.Rename(tmpName, path); err != nil {
		if isWindows() && backup != nil {
			_ = os.WriteFile(path, backup, 0o600)
		}
		return fmt.Errorf("写入配置: %w", err)
	}
	return nil
}

// Validate rejects obviously broken values early, before any network call.
func (c *Config) Validate() error {
	for i, s := range c.Servers {
		if strings.TrimSpace(s.Name) == "" {
			return fmt.Errorf("servers[%d].name 不能为空", i)
		}
		if strings.TrimSpace(s.URL) == "" {
			return fmt.Errorf("服务器 %q 的 url 不能为空", s.Name)
		}
		u, err := url.Parse(s.URL)
		if err != nil || u.Scheme == "" || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
			return fmt.Errorf("服务器 %q 的 url 无效: %q（应为 http(s)://host[:port]）", s.Name, s.URL)
		}
		if strings.TrimSpace(s.Username) == "" {
			return fmt.Errorf("服务器 %q 的 username 不能为空", s.Name)
		}
	}
	if c.Player.Volume < 0 {
		c.Player.Volume = 0
	}
	if c.Player.Volume > 100 {
		c.Player.Volume = 100
	}
	if c.Cache.PageSize < 10 {
		c.Cache.PageSize = 10
	}
	if c.Cache.Size < 16 {
		c.Cache.Size = 16
	}
	if c.UI.CompactWidth < 40 {
		c.UI.CompactWidth = 40
	}
	if c.UI.CoverWidth < 8 || c.UI.CoverWidth > 80 {
		c.UI.CoverWidth = 26
	}
	if c.LastFM.ScrobbleAt < 0 || c.LastFM.ScrobbleAt > 100 {
		c.LastFM.ScrobbleAt = 50
	}
	return nil
}

// LogLevel maps the configured string to a slog.Level.
func (c *Config) LogLevel() slog.Level {
	switch strings.ToLower(c.Log.Level) {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "error":
		return slog.LevelError
	default:
		return slog.LevelWarn
	}
}

func (c *Config) normalize() {
	c.Servers = dedupServers(c.Servers)
	if c.Keybindings.App == nil {
		c.Keybindings.App = map[string]any{}
	}
	if c.Theme.Name == "" {
		c.Theme.Name = "aurora"
	}
	if c.Player.SocketName == "" {
		c.Player.SocketName = "ndtui-mpv"
	}
	c.UI.CoverProtocol = strings.ToLower(strings.TrimSpace(c.UI.CoverProtocol))
	if c.UI.CoverProtocol != "sixel" {
		c.UI.CoverProtocol = "text"
	}
	if c.UI.SixelCellWidth <= 0 {
		c.UI.SixelCellWidth = 8
	}
	if c.UI.SixelCellHeight <= 0 {
		c.UI.SixelCellHeight = 16
	}
	if c.UI.SixelColors < 2 || c.UI.SixelColors > 256 {
		c.UI.SixelColors = 256
	}
	if c.Player.Mode == "" {
		c.Player.Mode = "sequential"
	}
	if c.Cache.TTL == "" {
		c.Cache.TTL = "5m"
	}
	if c.Log.Level == "" {
		c.Log.Level = "warn"
	}
	if _, ok := c.ActiveServer(); ok && c.Active == "" {
		c.Active = c.Servers[0].Name
	}
}

func dedupServers(in []Server) []Server {
	seen := make(map[string]bool, len(in))
	out := make([]Server, 0, len(in))
	for _, s := range in {
		s.Name = strings.TrimSpace(s.Name)
		s.URL = strings.TrimRight(strings.TrimSpace(s.URL), "/")
		if s.Name == "" || seen[s.Name] {
			continue
		}
		seen[s.Name] = true
		out = append(out, s)
	}
	return out
}

// applyEnv lets CI and power users override the active profile without
// editing the file. Credentials never have to be written to disk this way.
func (c *Config) applyEnv() {
	get := func(keys ...string) string {
		for _, k := range keys {
			if v := os.Getenv(k); v != "" {
				return v
			}
		}
		return ""
	}
	// New NAVIDROME_TUI_* names win; old SUBSONIC_TUI_* names stay as aliases
	// so existing scripts and shells keep working after the rename.
	if v := get("NAVIDROME_TUI_URL", "SUBSONIC_TUI_URL"); v != "" {
		s, ok := c.ActiveServer()
		if !ok {
			s = Server{Name: "default"}
		}
		s.URL = strings.TrimRight(v, "/")
		if v := get("NAVIDROME_TUI_USER", "NAVIDROME_TUI_USERNAME", "SUBSONIC_TUI_USER", "SUBSONIC_TUI_USERNAME"); v != "" {
			s.Username = v
		}
		if v := get("NAVIDROME_TUI_PASSWORD", "SUBSONIC_TUI_PASSWORD"); v != "" {
			s.Password = v
		}
		if n := get("NAVIDROME_TUI_SERVER_NAME", "SUBSONIC_TUI_SERVER_NAME"); n != "" {
			s.Name = n
		}
		c.SetServer(s)
		c.Active = s.Name
	}
	if v := get("NAVIDROME_TUI_THEME", "SUBSONIC_TUI_THEME"); v != "" {
		c.Theme.Name = v
	}
	if v := get("NAVIDROME_TUI_LOG_LEVEL", "SUBSONIC_TUI_LOG_LEVEL"); v != "" {
		c.Log.Level = v
	}
	if v := get("NAVIDROME_TUI_MPV", "SUBSONIC_TUI_MPV"); v != "" {
		c.Player.MPVPath = v
	}
}

func isWindows() bool { return runtime.GOOS == "windows" }

// EncodeSecret obfuscates a password so it is not stored as readable plain
// text. It is NOT encryption: use NAVIDROME_TUI_PASSWORD for real secrecy.
func EncodeSecret(plain string) string {
	return "enc:" + hex.EncodeToString([]byte(plain))
}

// PlainPassword decodes the "enc:" form, or returns a plain configured value.
func (s Server) PlainPassword() string {
	if !strings.HasPrefix(s.Password, "enc:") {
		return s.Password
	}
	raw, err := hex.DecodeString(strings.TrimPrefix(s.Password, "enc:"))
	if err != nil {
		return ""
	}
	return string(raw)
}

// ServerBaseURL returns the configured URL with a stable trailing form.
func (s Server) ServerBaseURL() string { return strings.TrimRight(s.URL, "/") }

// NowUTC is kept here so configuration consumers share one clock source.
func NowUTC() time.Time { return time.Now().UTC() }

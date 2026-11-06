// Package config owns the TOML configuration, its default values, platform
// paths and atomic persistence.
package config

import "time"

// Config is the on-disk configuration file layout.
type Config struct {
	Active      string       `toml:"active" comment:"当前使用的服务器名称"`
	Servers     []Server     `toml:"servers" comment:"可配置多个 Navidrome 服务器"`
	Theme       ThemeConfig  `toml:"theme"`
	Keybindings KeyConfig    `toml:"keybindings"`
	Player      PlayerConfig `toml:"player"`
	Cache       CacheConfig  `toml:"cache"`
	UI          UIConfig     `toml:"ui"`
	LastFM      LastFMConfig `toml:"lastfm"`
	Log         LogConfig    `toml:"log"`
}

// Server is one Navidrome profile.
type Server struct {
	Name     string `toml:"name"`
	URL      string `toml:"url"`
	Username string `toml:"username"`
	// Password is either a plain password (only for a manually edited file)
	// or "enc:" followed by hex bytes, which is what the login wizard writes.
	Password string `toml:"password"`
	Insecure bool   `toml:"insecure" comment:"跳过 TLS 证书校验，仅用于自签名证书的内网服务"`
}

// ThemeConfig selects the palette and allows an optional custom TOML palette.
type ThemeConfig struct {
	Name       string `toml:"name"`
	CustomPath string `toml:"customPath"`
}

// KeyConfig holds user overrides. Values may be a string or an array of
// strings; an empty string disables the default binding for that action.
type KeyConfig struct {
	UseDefaultBindings bool           `toml:"useDefaultKeyBindings"`
	App                map[string]any `toml:"app"`
}

// PlayerConfig configures the mpv subprocess and initial playback state.
type PlayerConfig struct {
	MPVPath    string `toml:"mpvPath" comment:"留空则自动从 PATH 查找 mpv"`
	Volume     int    `toml:"volume"`
	Mode       string `toml:"mode" comment:"sequential | loopAll | loopOne | shuffle"`
	SocketName string `toml:"socketName"`
}

// CacheConfig tunes memory usage and API paging.
type CacheConfig struct {
	TTL            string `toml:"ttl" comment:"Go duration, e.g. 5m"`
	Size           int    `toml:"size"`
	PageSize       int    `toml:"pageSize"`
	CoverCacheSize int    `toml:"coverCacheSize"`
}

// TTLDuration parses the TTL, falling back to the default on bad input.
func (c CacheConfig) TTLDuration() time.Duration {
	d, err := time.ParseDuration(c.TTL)
	if err != nil || d < 0 {
		return 5 * time.Minute
	}
	return d
}

// UIConfig controls purely visual behaviour.
type UIConfig struct {
	EnableMouse  bool `toml:"enableMouse"`
	ShowCover    bool `toml:"showCover"`
	DoubleColumn bool `toml:"doubleColumn"`
	Animations   bool `toml:"animations"`
	TrueColor    bool `toml:"trueColor" comment:"关闭可强制 256 色降级"`
	CoverWidth   int  `toml:"coverWidth"`
	CompactWidth int  `toml:"compactWidth" comment:"窄于该列数时收起侧栏"`
}

// LastFMConfig enables optional scrobbling to Last.fm.
type LastFMConfig struct {
	Enabled    bool   `toml:"enabled"`
	APIKey     string `toml:"apiKey"`
	APISecret  string `toml:"apiSecret"`
	SessionKey string `toml:"sessionKey"`
	ScrobbleAt int    `toml:"scrobbleAt" comment:"播放进度百分比达到该值时上报，0 表示仅在播放结束时"`
}

// LogConfig configures the file-only slog output.
type LogConfig struct {
	Level string `toml:"level"`
	File  string `toml:"file" comment:"留空使用用户缓存目录下的 subsonic-tui.log"`
}

// Default returns a fully populated configuration.
func Default() *Config {
	return &Config{
		Servers: []Server{},
		Theme: ThemeConfig{
			Name: "musicfox",
		},
		Keybindings: KeyConfig{
			UseDefaultBindings: true,
			App:                map[string]any{},
		},
		Player: PlayerConfig{
			Volume:     80,
			Mode:       "sequential",
			SocketName: "subsonic-tui-mpv",
		},
		Cache: CacheConfig{
			TTL:            "5m",
			Size:           512,
			PageSize:       60,
			CoverCacheSize: 64,
		},
		UI: UIConfig{
			EnableMouse:  true,
			ShowCover:    true,
			DoubleColumn: true,
			Animations:   true,
			TrueColor:    true,
			CoverWidth:   26,
			CompactWidth: 60,
		},
		LastFM: LastFMConfig{ScrobbleAt: 50},
		Log:    LogConfig{Level: "warn"},
	}
}

// ActiveServer returns the profile selected by Active, falling back to the
// first one when Active is empty or stale.
func (c *Config) ActiveServer() (Server, bool) {
	if len(c.Servers) == 0 {
		return Server{}, false
	}
	if c.Active != "" {
		for _, s := range c.Servers {
			if s.Name == c.Active {
				return s, true
			}
		}
	}
	return c.Servers[0], true
}

// SetServer upserts a profile by name.
func (c *Config) SetServer(s Server) {
	for i := range c.Servers {
		if c.Servers[i].Name == s.Name {
			c.Servers[i] = s
			return
		}
	}
	c.Servers = append(c.Servers, s)
}

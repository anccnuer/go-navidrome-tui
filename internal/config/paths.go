package config

import (
	"os"
	"path/filepath"
)

const (
	// AppName is used for the config/cache directory and the log file.
	AppName = "navidrome-tui"
	// EnvConfig overrides the config file location.
	EnvConfig = "NAVIDROME_TUI_CONFIG"

	// Legacy names are still read so an existing installation does not lose
	// its saved servers after the rename.
	legacyAppName   = "subsonic-tui"
	legacyEnvConfig = "SUBSONIC_TUI_CONFIG"
)

// Dir returns the platform user config directory for the app:
// Windows %AppData%\navidrome-tui, Linux $XDG_CONFIG_HOME/navidrome-tui,
// macOS ~/Library/Application Support/navidrome-tui.
func Dir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, AppName), nil
}

// Path returns the config.toml path (honouring NAVIDROME_TUI_CONFIG). If the
// new location has no config yet, a config written by the old name is used.
func Path() (string, error) {
	if p := os.Getenv(EnvConfig); p != "" {
		return p, nil
	}
	if p := os.Getenv(legacyEnvConfig); p != "" {
		return p, nil
	}
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	primary := filepath.Join(dir, "config.toml")
	if _, err := os.Stat(primary); err == nil {
		return primary, nil
	}
	legacy := filepath.Join(filepath.Dir(dir), legacyAppName, "config.toml")
	if _, err := os.Stat(legacy); err == nil {
		return legacy, nil
	}
	return primary, nil
}

// CacheDir returns the directory used for cover thumbnails and the log file.
func CacheDir() (string, error) {
	base, err := os.UserCacheDir()
	if err != nil {
		// Some minimal containers have no cache dir; fall back to config dir.
		dir, derr := Dir()
		if derr != nil {
			return "", err
		}
		return filepath.Join(dir, "cache"), nil
	}
	return filepath.Join(base, AppName), nil
}

// LogPath returns the effective log file path.
func LogPath(cfg LogConfig) (string, error) {
	if cfg.File != "" {
		return cfg.File, nil
	}
	dir, err := CacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, AppName+".log"), nil
}

// EnsureDirs creates the config and cache directories with user-only perms.
func EnsureDirs() error {
	confDir, err := Dir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(confDir, 0o700); err != nil {
		return err
	}
	cacheDir, err := CacheDir()
	if err != nil {
		return err
	}
	return os.MkdirAll(cacheDir, 0o700)
}

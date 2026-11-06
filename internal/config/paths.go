package config

import (
	"os"
	"path/filepath"
)

const (
	// AppName is used for the config/cache directory and the log file.
	AppName = "subsonic-tui"
	// EnvConfig overrides the config file location.
	EnvConfig = "SUBSONIC_TUI_CONFIG"
)

// Dir returns the platform user config directory for the app:
// Windows %AppData%\subsonic-tui, Linux $XDG_CONFIG_HOME/subsonic-tui,
// macOS ~/Library/Application Support/subsonic-tui.
func Dir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, AppName), nil
}

// Path returns the config.toml path (honouring SUBSONIC_TUI_CONFIG).
func Path() (string, error) {
	if p := os.Getenv(EnvConfig); p != "" {
		return p, nil
	}
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.toml"), nil
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

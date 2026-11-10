// Package app wires configuration, API, player, scrobbler and UI together.
package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/anccnuer/go-navidrome-tui/internal/config"
	"github.com/anccnuer/go-navidrome-tui/internal/cover/sixel"
	"github.com/anccnuer/go-navidrome-tui/internal/keymap"
	"github.com/anccnuer/go-navidrome-tui/internal/player"
	"github.com/anccnuer/go-navidrome-tui/internal/scrobbler"
	"github.com/anccnuer/go-navidrome-tui/internal/subsonic"
	"github.com/anccnuer/go-navidrome-tui/internal/ui"
	tea "github.com/charmbracelet/bubbletea"
)

// Options controls startup.
type Options struct {
	ConfigPath string
	Debug      bool
}

// Result holds the ready-to-run program and its shutdown hook.
type Result struct {
	Program *tea.Program
	Model   *ui.Model
	Cleanup func()
}

// App owns long-lived resources that outlive a single frame.
type App struct {
	cfg       *config.Config
	path      string
	logger    *slog.Logger
	logFile   *os.File
	deps      *ui.Deps
	player    player.Player
	scrobbler *scrobbler.Service
	// Immutable snapshots used by newClient, which may be called from a
	// tea.Cmd goroutine while the UI writes other Config fields.
	cacheTTL  time.Duration
	cacheSize int
}

// Build loads configuration and assembles every dependency. It performs no
// blocking network call: the first Ping happens asynchronously from Init.
func Build(opts Options) (*Result, error) {
	path := opts.ConfigPath
	if path == "" {
		resolved, err := config.Path()
		if err != nil {
			return nil, fmt.Errorf("无法确定配置文件路径: %w", err)
		}
		path = resolved
	}
	cfg, err := config.Load(path)
	if err != nil {
		return nil, err
	}
	if err := config.EnsureDirs(); err != nil {
		return nil, fmt.Errorf("创建配置目录失败: %w", err)
	}

	logPath, err := config.LogPath(cfg.Log)
	if err != nil {
		return nil, err
	}
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, fmt.Errorf("打开日志文件失败: %w", err)
	}
	level := cfg.LogLevel()
	if opts.Debug {
		level = slog.LevelDebug
	}
	logger := slog.New(slog.NewTextHandler(logFile, &slog.HandlerOptions{Level: level}))
	logger.Info("启动", "config", path, "servers", len(cfg.Servers))

	keys, warnings := keymap.Build(cfg.Keybindings.UseDefaultBindings, cfg.Keybindings.App)
	for _, warning := range warnings {
		logger.Warn("快捷键配置", "detail", warning)
	}

	a := &App{
		cfg: cfg, path: path, logger: logger, logFile: logFile,
		cacheTTL: cfg.Cache.TTLDuration(), cacheSize: cfg.Cache.Size,
	}
	deps := &ui.Deps{
		Config:     cfg,
		ConfigPath: path,
		Keys:       keys,
		Logger:     logger,
		Verify:     a.verify,
		Commit:     a.commit,
		Save:       a.save,
	}
	a.deps = deps

	if server, ok := cfg.ActiveServer(); ok {
		client, err := a.newClient(server)
		if err != nil {
			logger.Warn("创建 API 客户端失败", "err", err)
		} else {
			deps.SetClient(client)
		}
	}

	p, err := player.New(player.Config{
		MPVPath:    cfg.Player.MPVPath,
		SocketName: cfg.Player.SocketName,
		Volume:     cfg.Player.Volume,
		Mode:       player.ParseMode(cfg.Player.Mode),
		Logger:     logger,
	}, deps.StreamURL)
	if err != nil {
		logFile.Close()
		return nil, fmt.Errorf("初始化播放器失败: %w", err)
	}
	a.player = p
	deps.Player = p

	a.scrobbler = scrobbler.New(cfg.LastFM, deps.API(), logger)
	deps.Scrobbler = a.scrobbler

	m := ui.New(deps)

	// Sixel graphics are opt-in. When enabled, DCS payloads are appended after
	// each text frame by the wrapped writer instead of being embedded in
	// View() (the standard renderer would truncate/diff them away).
	sixelEnabled := cfg.UI.ShowCover && cfg.UI.CoverProtocol == "sixel" &&
		sixel.ReadEnvironment().LikelySupported()
	sixelLayer := sixel.NewLayer(sixelEnabled)
	m.SetSixelLayer(sixelLayer)
	output := &sixel.Writer{Out: os.Stdout, Layer: sixelLayer}
	if !sixelEnabled && cfg.UI.CoverProtocol == "sixel" {
		logger.Warn("Sixel 已配置但当前终端环境未通过检测，回退为纯文本封面")
	}

	// Bracketed paste can leave the input parser waiting for a terminating
	// \x1b[201~ when mouse/CSI sequences get interleaved (seen on Windows
	// Terminal + ConPTY), which makes every subsequent key appear dead.
	// Input fields already clean pasted text via cleanInput, so disabling the
	// protocol costs no functionality and removes that failure mode.
	options := []tea.ProgramOption{
		tea.WithAltScreen(), tea.WithoutBracketedPaste(), tea.WithOutput(output),
	}
	if cfg.UI.EnableMouse {
		options = append(options, tea.WithMouseCellMotion())
	}
	program := tea.NewProgram(m, options...)

	var cleanupOnce sync.Once
	cleanup := func() {
		cleanupOnce.Do(func() {
			// mpv first: it is the only side effect that outlives the process.
			_ = p.Close()
			a.scrobbler.Close()
			logger.Info("退出")
			_ = logFile.Close()
		})
	}
	return &Result{Program: program, Model: m, Cleanup: cleanup}, nil
}

func (a *App) newClient(server config.Server) (subsonic.Client, error) {
	// Validate a single profile without mutating the global config.
	probe := &config.Config{Servers: []config.Server{server}}
	if err := probe.Validate(); err != nil {
		return nil, err
	}
	return subsonic.New(subsonic.Options{
		BaseURL:       server.URL,
		Username:      server.Username,
		Password:      server.PlainPassword(),
		Insecure:      server.Insecure,
		CacheTTL:      a.cacheTTL,
		CacheSize:     a.cacheSize,
		MaxConcurrent: 8,
		Logger:        a.logger,
	})
}

// verify only performs network work and may run on a tea.Cmd goroutine.
// It must never mutate a.cfg: the UI reads that struct on every frame.
func (a *App) verify(server config.Server) (subsonic.Client, error) {
	client, err := a.newClient(server)
	if err != nil {
		return nil, err
	}
	var last error
	for attempt := 0; attempt < 2; attempt++ {
		ctx, cancel := context.WithTimeout(context.Background(), 18*time.Second)
		last = client.Ping(ctx)
		cancel()
		if last == nil {
			return client, nil
		}
		if !isTimeoutError(last) {
			break // auth / HTTP errors will not improve by retrying
		}
		if attempt == 0 {
			a.logger.Warn("连接超时，准备重试一次", "url", server.URL)
			time.Sleep(300 * time.Millisecond)
		}
	}
	if isTimeoutError(last) {
		return nil, fmt.Errorf("连接超时（服务器无响应，公共实例可能负载较高），请稍后重试：%w", last)
	}
	return nil, fmt.Errorf("连接失败：%w", last)
}

func isTimeoutError(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, context.DeadlineExceeded) ||
		strings.Contains(err.Error(), "Client.Timeout") ||
		strings.Contains(err.Error(), "deadline exceeded")
}

// commit runs on the UI goroutine after a successful verify: persisting the
// profile and swapping the client are the only writers of a.cfg, so there is
// no concurrent access and no lost update.
func (a *App) commit(server config.Server, client subsonic.Client) error {
	if client == nil {
		return fmt.Errorf("客户端未初始化")
	}
	a.cfg.SetServer(server)
	a.cfg.Active = server.Name
	if err := a.save(); err != nil {
		return fmt.Errorf("保存配置失败：%w", err)
	}
	a.deps.SetClient(client)
	a.scrobbler.SetClient(client)
	a.logger.Info("切换服务器", "name", server.Name, "url", server.URL)
	return nil
}

func (a *App) save() error { return a.cfg.Save(a.path) }

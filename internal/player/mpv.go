package player

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os/exec"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/anccnuer/go-navidrome-tui/internal/model"
)

// ErrMPVUnavailable is returned by playback commands before mpv is ready or
// when mpv is not installed. The UI maps it to an actionable hint.
var ErrMPVUnavailable = errors.New("mpv 未就绪：请安装 mpv 并确保其在 PATH 中（或配置 player.mpvPath）")

const installHint = "未找到 mpv。请安装 mpv（https://mpv.io/installation/）并加入 PATH，或在 config.toml 中设置 player.mpvPath 为可执行文件完整路径。"

// Config configures the mpv backend.
type Config struct {
	MPVPath    string
	SocketName string
	Volume     int
	Mode       PlayMode
	Logger     *slog.Logger
}

type mpvPlayer struct {
	cfg    Config
	urlFor func(model.Song) string
	log    *slog.Logger
	q      *queue

	events chan Event
	stop   chan struct{}
	stopMu sync.Once

	stateMu sync.RWMutex
	state   PlayerState

	ipcMu sync.Mutex
	ipc   *ipcConn

	procMu   sync.Mutex
	cmd      *exec.Cmd
	procDone chan struct{}

	available atomic.Bool
	closed    atomic.Bool
}

// New creates the player and starts mpv in the background. Startup never
// blocks the UI; readiness is reported through Events().
func New(cfg Config, urlFor func(model.Song) string) (Player, error) {
	if urlFor == nil {
		return nil, fmt.Errorf("缺少流地址解析器")
	}
	if cfg.Volume < 0 {
		cfg.Volume = 0
	}
	if cfg.Volume > 100 {
		cfg.Volume = 100
	}
	if cfg.SocketName == "" {
		cfg.SocketName = "ndtui-mpv"
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	p := &mpvPlayer{
		cfg:      cfg,
		urlFor:   urlFor,
		log:      cfg.Logger,
		q:        newQueue(cfg.Mode),
		events:   make(chan Event, 128),
		stop:     make(chan struct{}),
		procDone: make(chan struct{}),
	}
	p.state = PlayerState{Volume: cfg.Volume, Mode: cfg.Mode, Index: -1}
	go p.start()
	return p, nil
}

func (p *mpvPlayer) start() {
	path := p.cfg.MPVPath
	if path == "" {
		found, err := exec.LookPath("mpv")
		if err != nil {
			p.log.Warn("mpv 未安装")
			p.emit(Event{Kind: EventUnavailable, Err: ErrMPVUnavailable, Text: installHint})
			return
		}
		path = found
	}
	address := ipcAddress(p.cfg.SocketName)
	args := []string{
		"--idle=yes",
		"--no-video",
		"--no-terminal",
		"--really-quiet",
		"--msg-level=all=error",
		"--no-config",
		"--load-scripts=no",
		"--force-window=no",
		"--keep-open=no",
		"--audio-display=no",
		"--input-ipc-server=" + address,
		"--volume=" + strconv.Itoa(p.cfg.Volume),
	}
	cmd := exec.Command(path, args...)
	decorateCmd(cmd)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		p.log.Warn("启动 mpv 失败", "err", err, "path", path)
		p.emit(Event{Kind: EventUnavailable, Err: err, Text: fmt.Sprintf("启动 mpv 失败：%v（%s）", err, path)})
		return
	}
	p.procMu.Lock()
	p.cmd = cmd
	p.procMu.Unlock()
	// On Windows this puts mpv into a job object that is terminated when this
	// process exits for any reason, preventing orphaned audio playback.
	if err := assignToKillJob(cmd.Process); err != nil {
		p.log.Warn("无法将 mpv 加入进程守护 Job，退出时将直接结束进程", "err", err)
	}
	go func() {
		_ = cmd.Wait()
		close(p.procDone)
	}()

	var conn net.Conn
	var err error
	for i := 0; i < 50; i++ {
		if p.closed.Load() {
			// Close() may have run before mpv finished starting up; make sure
			// the process does not survive this early return.
			p.killProcess()
			return
		}
		conn, err = dialIPC(address, 200*time.Millisecond)
		if err == nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if conn == nil {
		p.log.Warn("连接 mpv IPC 失败", "err", err)
		p.emit(Event{Kind: EventUnavailable, Err: err, Text: "已启动 mpv 但无法连接 IPC：" + fmt.Sprint(err)})
		p.killProcess()
		return
	}

	ipc := newIPC(conn)
	p.ipcMu.Lock()
	p.ipc = ipc
	p.ipcMu.Unlock()

	for id, name := range []string{"time-pos", "duration", "pause", "volume"} {
		if err := ipc.observe(id+1, name); err != nil {
			p.log.Warn("订阅 mpv 属性失败", "property", name, "err", err)
		}
	}
	if _, err := ipc.call("set_property", "volume", p.cfg.Volume); err != nil {
		p.log.Warn("设置初始音量失败", "err", err)
	}
	p.available.Store(true)
	p.updateState(func(s *PlayerState) {
		s.Available = true
		s.Volume = p.cfg.Volume
		s.Mode = p.cfg.Mode
	})
	p.emit(Event{Kind: EventReady, Text: path})
	go p.consume(ipc)
}

func (p *mpvPlayer) consume(ipc *ipcConn) {
	for {
		select {
		case ev := <-ipc.events:
			p.handle(ev)
		case <-ipc.done:
			p.available.Store(false)
			p.updateState(func(s *PlayerState) { s.Available = false })
			p.emit(Event{Kind: EventExit})
			return
		case <-p.stop:
			return
		}
	}
}

func (p *mpvPlayer) handle(ev Event) {
	switch ev.Kind {
	case EventPosition:
		p.updateState(func(s *PlayerState) { s.Position = int(ev.Position) })
	case EventDuration:
		p.updateState(func(s *PlayerState) { s.Duration = int(ev.Duration) })
	case EventPause:
		p.updateState(func(s *PlayerState) {
			if ev.Pause {
				s.Status = StatusPaused
			} else if s.Status != StatusStopped {
				s.Status = StatusPlaying
			}
		})
	case EventVolume:
		p.updateState(func(s *PlayerState) { s.Volume = int(ev.Volume) })
	case EventFileLoaded:
		p.updateState(func(s *PlayerState) {
			if s.Status != StatusPaused {
				s.Status = StatusPlaying
			}
		})
	case EventEndFile:
		if song, ok := p.q.advance(); ok {
			if err := p.load(song); err != nil {
				p.emit(Event{Kind: EventError, Err: err, Text: err.Error()})
			}
		} else {
			p.updateState(func(s *PlayerState) { s.Status = StatusStopped; s.Position = 0 })
		}
	case EventError:
		p.log.Warn("mpv 播放错误", "detail", ev.Text)
		p.updateState(func(s *PlayerState) { s.Status = StatusError })
	case EventExit:
		p.available.Store(false)
		p.updateState(func(s *PlayerState) { s.Available = false })
	}
	p.emit(ev)
}

func (p *mpvPlayer) ipcConn() *ipcConn {
	p.ipcMu.Lock()
	defer p.ipcMu.Unlock()
	return p.ipc
}

func (p *mpvPlayer) command(cmd ...any) error {
	ipc := p.ipcConn()
	if ipc == nil || !p.available.Load() {
		return ErrMPVUnavailable
	}
	_, err := ipc.call(cmd...)
	return err
}

func (p *mpvPlayer) load(song model.Song) error {
	ipc := p.ipcConn()
	if ipc == nil || !p.available.Load() {
		return ErrMPVUnavailable
	}
	url := p.urlFor(song)
	if url == "" {
		return fmt.Errorf("无法生成曲目 %s 的流地址", song.ID)
	}
	if _, err := ipc.call("loadfile", url, "replace"); err != nil {
		return fmt.Errorf("播放失败: %w", err)
	}
	songs, index, _ := p.q.snapshot()
	p.updateState(func(s *PlayerState) {
		s.Song = song
		s.Status = StatusLoading
		s.Position = 0
		s.Duration = song.Duration
		s.Index = index
		s.QueueLength = len(songs)
	})
	p.emit(Event{Kind: EventQueueChanged})
	return nil
}

func (p *mpvPlayer) updateState(fn func(*PlayerState)) {
	p.stateMu.Lock()
	defer p.stateMu.Unlock()
	fn(&p.state)
}

func (p *mpvPlayer) emit(ev Event) {
	select {
	case p.events <- ev:
	case <-p.stop:
	default:
	}
}

// ---- Player interface ----------------------------------------------------

func (p *mpvPlayer) Play(song model.Song) error {
	p.q.set([]model.Song{song}, 0)
	return p.load(song)
}

func (p *mpvPlayer) SetQueue(songs []model.Song, start int) error {
	song, ok := p.q.set(songs, start)
	if !ok {
		return fmt.Errorf("播放队列为空")
	}
	return p.load(song)
}

func (p *mpvPlayer) Toggle() error {
	if err := p.command("cycle", "pause"); err != nil {
		return err
	}
	p.updateState(func(s *PlayerState) {
		if s.Status == StatusPaused {
			s.Status = StatusPlaying
		} else if s.Status == StatusPlaying {
			s.Status = StatusPaused
		}
	})
	return nil
}

func (p *mpvPlayer) Next() error {
	song, ok := p.q.skip(1)
	if !ok {
		return fmt.Errorf("已经是最后一首")
	}
	return p.load(song)
}

func (p *mpvPlayer) Prev() error {
	song, ok := p.q.skip(-1)
	if !ok {
		return fmt.Errorf("队列为空")
	}
	return p.load(song)
}

func (p *mpvPlayer) Seek(seconds int) error {
	if seconds < 0 {
		seconds = 0
	}
	return p.command("seek", seconds, "absolute")
}

func (p *mpvPlayer) SeekBy(delta int) error {
	return p.command("seek", delta, "relative")
}

func (p *mpvPlayer) SetVolume(v int) error {
	if v < 0 {
		v = 0
	}
	if v > 100 {
		v = 100
	}
	if err := p.command("set_property", "volume", v); err != nil {
		return err
	}
	p.updateState(func(s *PlayerState) { s.Volume = v })
	return nil
}

func (p *mpvPlayer) AdjustVolume(delta int) error {
	p.stateMu.RLock()
	v := p.state.Volume
	p.stateMu.RUnlock()
	return p.SetVolume(v + delta)
}

func (p *mpvPlayer) SetMode(m PlayMode) {
	p.q.setMode(m)
	p.updateState(func(s *PlayerState) { s.Mode = m })
	p.emit(Event{Kind: EventQueueChanged})
}

func (p *mpvPlayer) Queue() []model.Song {
	songs, _, _ := p.q.snapshot()
	return songs
}

func (p *mpvPlayer) CurrentIndex() int {
	_, index, _ := p.q.snapshot()
	return index
}

func (p *mpvPlayer) InsertNext(songs []model.Song) error {
	p.q.insertNext(songs)
	p.syncQueueState()
	return nil
}

func (p *mpvPlayer) Append(songs []model.Song) error {
	p.q.appendSongs(songs)
	p.syncQueueState()
	return nil
}

func (p *mpvPlayer) RemoveAt(index int) error {
	p.q.removeAt(index)
	p.syncQueueState()
	return nil
}

func (p *mpvPlayer) ClearQueue() error {
	p.q.clear()
	_ = p.command("stop")
	p.syncQueueState()
	return nil
}

func (p *mpvPlayer) syncQueueState() {
	songs, index, mode := p.q.snapshot()
	p.updateState(func(s *PlayerState) {
		s.Index = index
		s.QueueLength = len(songs)
		s.Mode = mode
	})
	p.emit(Event{Kind: EventQueueChanged})
}

func (p *mpvPlayer) State() PlayerState {
	p.stateMu.RLock()
	defer p.stateMu.RUnlock()
	return p.state
}

func (p *mpvPlayer) Events() <-chan Event { return p.events }

func (p *mpvPlayer) Available() bool { return p.available.Load() }

func (p *mpvPlayer) Close() error {
	if !p.closed.CompareAndSwap(false, true) {
		return nil
	}
	p.stopMu.Do(func() { close(p.stop) })
	if ipc := p.ipcConn(); ipc != nil {
		ipc.close()
	}
	p.killProcess()
	p.available.Store(false)
	return nil
}

func (p *mpvPlayer) killProcess() {
	p.procMu.Lock()
	cmd := p.cmd
	p.procMu.Unlock()
	if cmd == nil || cmd.Process == nil {
		return
	}
	p.log.Debug("结束 mpv 进程", "pid", cmd.Process.Pid)
	if err := cmd.Process.Kill(); err != nil {
		p.log.Warn("结束 mpv 进程失败", "pid", cmd.Process.Pid, "err", err)
	}
	select {
	case <-p.procDone:
	case <-time.After(2 * time.Second):
		p.log.Warn("mpv 未在超时内退出", "pid", cmd.Process.Pid)
	}
}

// Package scrobbler reports playback to Navidrome and (optionally) Last.fm.
// All network work happens on a background worker so the TUI never blocks.
package scrobbler

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/anccnuer/go-navidrome-tui/internal/config"
	"github.com/anccnuer/go-navidrome-tui/internal/model"
)

// NavidromeClient is the narrow subset of the Subsonic client we need.
type NavidromeClient interface {
	Scrobble(ctx context.Context, songID string, submission bool, started time.Time) error
}

type taskKind int

const (
	taskNowPlaying taskKind = iota
	taskScrobble
)

type task struct {
	kind    taskKind
	song    model.Song
	started time.Time
	retries int
}

// Service fans out playback events to Navidrome and Last.fm.
type Service struct {
	cfg      config.LastFMConfig
	clientMu sync.RWMutex
	client   NavidromeClient
	http     *http.Client
	lastfmEP string
	logger   *slog.Logger

	tasks chan task
	done  chan struct{}
	once  sync.Once
	wg    sync.WaitGroup
}

// New starts the background worker. Pass a nil client to disable Navidrome
// reporting (e.g. before the login wizard has configured a server).
func New(cfg config.LastFMConfig, client NavidromeClient, logger *slog.Logger) *Service {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	s := &Service{
		cfg:      cfg,
		client:   client,
		http:     &http.Client{Timeout: 15 * time.Second},
		lastfmEP: lastfmEndpoint,
		logger:   logger,
		tasks:    make(chan task, 64),
		done:     make(chan struct{}),
	}
	s.wg.Add(1)
	go s.loop()
	return s
}

// SetClient swaps the API client after a server switch. It is safe to call
// while the background worker is processing tasks.
func (s *Service) SetClient(client NavidromeClient) {
	s.clientMu.Lock()
	s.client = client
	s.clientMu.Unlock()
}

func (s *Service) navidrome() NavidromeClient {
	s.clientMu.RLock()
	defer s.clientMu.RUnlock()
	return s.client
}

// NowPlaying announces a track start. Failures are logged but never retried,
// as required by the Last.fm spec.
func (s *Service) NowPlaying(song model.Song) {
	s.enqueue(task{kind: taskNowPlaying, song: song, started: time.Now()})
}

// Submitted reports a completed play.
func (s *Service) Submitted(song model.Song, started time.Time) {
	s.enqueue(task{kind: taskScrobble, song: song, started: started})
}

// ShouldScrobble implements Last.fm's default rule: tracks longer than 30
// seconds, played for at least half their duration or four minutes, whichever
// comes first.
func ShouldScrobble(song model.Song, positionSeconds int) bool {
	return ShouldScrobbleAt(song, positionSeconds, 50)
}

// ShouldScrobbleAt honours the configured percentage. A threshold of 0 means
// "only report when the track ends" (the UI submits on mpv's end-file event),
// so progress-based checks always return false.
func ShouldScrobbleAt(song model.Song, positionSeconds, thresholdPercent int) bool {
	if song.Duration < 30 || positionSeconds <= 0 || thresholdPercent <= 0 || thresholdPercent > 100 {
		return false
	}
	if positionSeconds >= 240 {
		return true
	}
	return positionSeconds*100 >= song.Duration*thresholdPercent
}

// ShouldScrobble implements ui.ScrobbleService with the configured threshold.
func (s *Service) ShouldScrobble(song model.Song, positionSeconds int) bool {
	return ShouldScrobbleAt(song, positionSeconds, s.cfg.ScrobbleAt)
}

func (s *Service) enqueue(t task) {
	select {
	case <-s.done:
		return
	default:
	}
	select {
	case s.tasks <- t:
	default:
		s.logger.Warn("播放记录队列已满，丢弃一条上报")
	}
}

func (s *Service) loop() {
	defer s.wg.Done()
	for {
		select {
		case t := <-s.tasks:
			s.process(t)
		case <-s.done:
			// Drain what is already queued, then stop.
			for {
				select {
				case t := <-s.tasks:
					s.process(t)
				default:
					return
				}
			}
		}
	}
}

func (s *Service) process(t task) {
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()

	if client := s.navidrome(); client != nil && t.song.ID != "" {
		if err := client.Scrobble(ctx, t.song.ID, t.kind == taskScrobble, t.started); err != nil {
			s.logger.Warn("Navidrome 播放记录上报失败", "song", t.song.ID, "err", err)
		}
	}
	if !s.cfg.Enabled || s.cfg.APIKey == "" || s.cfg.APISecret == "" || s.cfg.SessionKey == "" {
		return
	}
	var err error
	if t.kind == taskNowPlaying {
		err = s.updateNowPlaying(ctx, t.song)
	} else {
		err = s.scrobble(ctx, t.song, t.started)
	}
	if err == nil {
		return
	}
	if isRetryable(err) && t.retries < 3 {
		t.retries++
		s.logger.Debug("Last.fm 上报将重试", "retries", t.retries, "err", err)
		// Requeue with a small backoff without blocking the worker.
		go func() {
			timer := time.NewTimer(time.Duration(t.retries) * 2 * time.Second)
			defer timer.Stop()
			select {
			case <-timer.C:
				s.enqueue(t)
			case <-s.done:
			}
		}()
		return
	}
	s.logger.Warn("Last.fm 上报失败", "method", t.kind, "err", err)
}

// Close drains the queue and stops the worker.
func (s *Service) Close() {
	s.once.Do(func() { close(s.done) })
	s.wg.Wait()
}

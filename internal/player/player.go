// Package player owns the playback queue state machine and the mpv JSON IPC
// backend. The UI only talks to the Player interface and consumes Events.
package player

import (
	"math/rand"
	"sync"
	"time"

	"github.com/subsonic-tui/subsonic-tui/internal/model"
)

// PlayMode and PlayerStatus are aliases so callers can stay within one package.
type PlayMode = model.PlayMode
type PlayerStatus = model.PlayerStatus

const (
	ModeSequential = model.ModeSequential
	ModeLoopAll    = model.ModeLoopAll
	ModeLoopOne    = model.ModeLoopOne
	ModeShuffle    = model.ModeShuffle

	StatusStopped = model.StatusStopped
	StatusLoading = model.StatusLoading
	StatusPlaying = model.StatusPlaying
	StatusPaused  = model.StatusPaused
	StatusError   = model.StatusError
)

// ParseMode maps the config string to a PlayMode.
func ParseMode(s string) PlayMode {
	switch s {
	case "loopAll", "loopall", "list":
		return ModeLoopAll
	case "loopOne", "loopone", "single":
		return ModeLoopOne
	case "shuffle", "random":
		return ModeShuffle
	default:
		return ModeSequential
	}
}

// ModeString maps a PlayMode back to its config value.
func ModeString(m PlayMode) string {
	switch m {
	case ModeLoopAll:
		return "loopAll"
	case ModeLoopOne:
		return "loopOne"
	case ModeShuffle:
		return "shuffle"
	default:
		return "sequential"
	}
}

// PlayerState is an immutable snapshot returned by State().
type PlayerState struct {
	Status      PlayerStatus
	Song        model.Song
	Position    int // seconds
	Duration    int // seconds
	Volume      int
	Mode        PlayMode
	Index       int
	QueueLength int
	Available   bool
}

// EventKind identifies a playback event.
type EventKind int

const (
	EventReady EventKind = iota
	EventUnavailable
	EventPosition
	EventDuration
	EventPause
	EventVolume
	EventFileLoaded
	EventEndFile
	EventError
	EventExit
	EventQueueChanged
)

// Event is emitted by the player and fed into the Bubble Tea update loop.
type Event struct {
	Kind     EventKind
	Position float64
	Duration float64
	Volume   float64
	Pause    bool
	Err      error
	Text     string
}

// Player is the playback contract used by the UI layer.
type Player interface {
	Play(song model.Song) error
	Toggle() error
	Next() error
	Prev() error
	Seek(seconds int) error
	SeekBy(delta int) error
	SetVolume(v int) error
	AdjustVolume(delta int) error
	SetMode(m PlayMode)
	SetQueue(songs []model.Song, start int) error
	Queue() []model.Song
	CurrentIndex() int
	InsertNext(songs []model.Song) error
	Append(songs []model.Song) error
	RemoveAt(index int) error
	ClearQueue() error
	State() PlayerState
	Events() <-chan Event
	Available() bool
	Close() error
}

// queue is a deterministic playback-order state machine. All exported
// behaviour of the player funnels through it, which makes it unit-testable
// without mpv.
type queue struct {
	mu           sync.Mutex
	songs        []model.Song
	index        int
	mode         PlayMode
	shuffleOrder []int
	shufflePos   int
	history      []int
	rng          *rand.Rand
}

func newQueue(mode PlayMode) *queue {
	return &queue{
		index: -1,
		mode:  mode,
		rng:   rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

func (q *queue) set(songs []model.Song, start int) (model.Song, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.songs = append([]model.Song(nil), songs...)
	q.history = nil
	q.shuffleOrder = nil
	q.shufflePos = 0
	if len(q.songs) == 0 {
		q.index = -1
		return model.Song{}, false
	}
	if start < 0 || start >= len(q.songs) {
		start = 0
	}
	q.index = start
	q.buildShuffleLocked(start)
	return q.songs[q.index], true
}

func (q *queue) buildShuffleLocked(first int) {
	rest := make([]int, 0, len(q.songs)-1)
	for i := range q.songs {
		if i != first {
			rest = append(rest, i)
		}
	}
	q.rng.Shuffle(len(rest), func(i, j int) { rest[i], rest[j] = rest[j], rest[i] })
	q.shuffleOrder = append([]int{first}, rest...)
	q.shufflePos = 0
}

// advance moves to the track that should play after the current one ends.
func (q *queue) advance() (model.Song, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.advanceLocked()
}

func (q *queue) advanceLocked() (model.Song, bool) {
	if len(q.songs) == 0 || q.index < 0 {
		return model.Song{}, false
	}
	if q.mode == ModeLoopOne {
		return q.songs[q.index], true
	}
	if q.mode == ModeShuffle {
		q.shufflePos++
		if q.shufflePos >= len(q.shuffleOrder) {
			q.buildShuffleLocked(q.index)
			q.shufflePos = 1
			if q.shufflePos >= len(q.shuffleOrder) {
				return q.songs[q.index], false
			}
		}
		previous := q.index
		q.index = q.shuffleOrder[q.shufflePos]
		q.history = append(q.history, previous)
		return q.songs[q.index], true
	}
	next := q.index + 1
	if next >= len(q.songs) {
		if q.mode != ModeLoopAll {
			return model.Song{}, false
		}
		next = 0
	}
	previous := q.index
	q.index = next
	q.history = append(q.history, previous)
	return q.songs[next], true
}

// skip implements manual next/prev. Positive delta repeats advance.
func (q *queue) skip(delta int) (model.Song, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.songs) == 0 || q.index < 0 {
		return model.Song{}, false
	}
	if delta > 0 {
		var song model.Song
		var ok bool
		for i := 0; i < delta; i++ {
			song, ok = q.advanceLocked()
			if !ok {
				return model.Song{}, false
			}
		}
		return song, true
	}
	if len(q.history) > 0 {
		q.index = q.history[len(q.history)-1]
		q.history = q.history[:len(q.history)-1]
	} else {
		q.index--
		if q.index < 0 {
			q.index = 0
		}
	}
	if q.mode == ModeShuffle {
		// Re-anchor the shuffle order at the restored track so the next
		// advance does not jump back to a stale shufflePos.
		q.buildShuffleLocked(q.index)
	}
	return q.songs[q.index], true
}

func (q *queue) current() (model.Song, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.index < 0 || q.index >= len(q.songs) {
		return model.Song{}, false
	}
	return q.songs[q.index], true
}

func (q *queue) setMode(mode PlayMode) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.mode == mode {
		return
	}
	q.mode = mode
	if mode == ModeShuffle && len(q.songs) > 0 && q.index >= 0 {
		q.buildShuffleLocked(q.index)
	}
}

func (q *queue) appendSongs(songs []model.Song) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.songs = append(q.songs, songs...)
	if q.index < 0 && len(q.songs) > 0 {
		q.index = 0
		q.buildShuffleLocked(0)
	}
}

func (q *queue) insertNext(songs []model.Song) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(songs) == 0 {
		return
	}
	if q.index < 0 || len(q.songs) == 0 {
		q.songs = append(q.songs, songs...)
		q.index = 0
		q.buildShuffleLocked(0)
		return
	}
	at := q.index + 1
	tail := append([]model.Song(nil), q.songs[at:]...)
	q.songs = append(q.songs[:at:at], songs...)
	q.songs = append(q.songs, tail...)
	q.history = nil // index-based history is no longer reliable after an insert
	q.buildShuffleLocked(q.index)
}

func (q *queue) removeAt(i int) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if i < 0 || i >= len(q.songs) {
		return
	}
	q.songs = append(q.songs[:i], q.songs[i+1:]...)
	q.history = nil
	if i < q.index {
		q.index--
	} else if i == q.index && q.index >= len(q.songs) {
		q.index = len(q.songs) - 1
	}
	if len(q.songs) == 0 {
		q.index = -1
	}
	if q.index >= 0 {
		q.buildShuffleLocked(q.index)
	}
}

func (q *queue) clear() {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.songs = nil
	q.index = -1
	q.history = nil
	q.shuffleOrder = nil
	q.shufflePos = 0
}

func (q *queue) snapshot() ([]model.Song, int, PlayMode) {
	q.mu.Lock()
	defer q.mu.Unlock()
	out := append([]model.Song(nil), q.songs...)
	return out, q.index, q.mode
}

package player

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const ipcCallTimeout = 3 * time.Second

type ipcReply struct {
	err  string
	data json.RawMessage
}

// ipcConn multiplexes mpv's request/response pairs and its unsolicited event
// stream over a single socket or named pipe.
type ipcConn struct {
	conn   net.Conn
	writer *bufio.Writer
	mu     sync.Mutex // serialises writes
	nextID atomic.Int64

	pendingMu sync.Mutex
	pending   map[int64]chan ipcReply

	events chan Event
	done   chan struct{}
	once   sync.Once
}

type ipcMessage struct {
	RequestID *int64          `json:"request_id"`
	Error     string          `json:"error"`
	Data      json.RawMessage `json:"data"`
	Event     string          `json:"event"`
	Name      string          `json:"name"`
	// mpv sends "reason" as a top-level field of end-file events, not inside
	// "data" (which only carries property-change values).
	Reason string `json:"reason"`
}

func newIPC(conn net.Conn) *ipcConn {
	c := &ipcConn{
		conn:    conn,
		writer:  bufio.NewWriter(conn),
		pending: make(map[int64]chan ipcReply),
		events:  make(chan Event, 128),
		done:    make(chan struct{}),
	}
	go c.readLoop()
	return c
}

// call sends a JSON IPC command and waits for the matching response.
func (c *ipcConn) call(command ...any) (json.RawMessage, error) {
	id := c.nextID.Add(1)
	replyCh := make(chan ipcReply, 1)
	c.pendingMu.Lock()
	c.pending[id] = replyCh
	c.pendingMu.Unlock()
	defer func() {
		c.pendingMu.Lock()
		delete(c.pending, id)
		c.pendingMu.Unlock()
	}()

	payload := map[string]any{
		"command":    command,
		"request_id": id,
	}
	line, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	c.mu.Lock()
	_, err = c.writer.Write(append(line, '\n'))
	if err == nil {
		err = c.writer.Flush()
	}
	c.mu.Unlock()
	if err != nil {
		return nil, fmt.Errorf("写入 mpv IPC 失败: %w", err)
	}

	timer := time.NewTimer(ipcCallTimeout)
	defer timer.Stop()
	select {
	case reply := <-replyCh:
		if reply.err != "" && reply.err != "success" {
			return nil, fmt.Errorf("mpv: %s", reply.err)
		}
		return reply.data, nil
	case <-timer.C:
		return nil, fmt.Errorf("mpv IPC 响应超时")
	case <-c.done:
		return nil, fmt.Errorf("mpv IPC 已断开")
	}
}

// observe subscribes to property-change events using a stable numeric id.
func (c *ipcConn) observe(id int, name string) error {
	_, err := c.call("observe_property", id, name)
	return err
}

func (c *ipcConn) readLoop() {
	defer c.close()
	scanner := bufio.NewScanner(c.conn)
	scanner.Buffer(make([]byte, 64*1024), 512*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var msg ipcMessage
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			continue
		}
		if msg.RequestID != nil {
			c.pendingMu.Lock()
			ch := c.pending[*msg.RequestID]
			c.pendingMu.Unlock()
			if ch != nil {
				select {
				case ch <- ipcReply{err: msg.Error, data: msg.Data}:
				default:
				}
			}
			continue
		}
		if ev, ok := parseEvent(msg); ok {
			select {
			case c.events <- ev:
			default: // never let a slow UI stall the IPC reader
			}
		}
	}
	c.emit(Event{Kind: EventExit, Text: "mpv 连接已断开"})
}

func parseEvent(msg ipcMessage) (Event, bool) {
	if msg.Event == "property-change" {
		switch msg.Name {
		case "time-pos", "playback-time":
			var v float64
			if json.Unmarshal(msg.Data, &v) == nil {
				return Event{Kind: EventPosition, Position: v}, true
			}
		case "duration":
			var v float64
			if json.Unmarshal(msg.Data, &v) == nil {
				return Event{Kind: EventDuration, Duration: v}, true
			}
		case "pause":
			var v bool
			if json.Unmarshal(msg.Data, &v) == nil {
				return Event{Kind: EventPause, Pause: v}, true
			}
		case "volume":
			var v float64
			if json.Unmarshal(msg.Data, &v) == nil {
				return Event{Kind: EventVolume, Volume: v}, true
			}
		}
		return Event{}, false
	}
	switch msg.Event {
	case "file-loaded":
		return Event{Kind: EventFileLoaded}, true
	case "end-file":
		// "reason" / "error" are top-level fields on real mpv end-file
		// events; keep accepting a data-embedded variant for compatibility.
		reason, errText := msg.Reason, msg.Error
		var d struct {
			Reason string `json:"reason"`
			Error  string `json:"error"`
		}
		_ = json.Unmarshal(msg.Data, &d)
		if reason == "" {
			reason = d.Reason
		}
		if errText == "" {
			errText = d.Error
		}
		switch reason {
		case "eof":
			return Event{Kind: EventEndFile, Text: "eof"}, true
		case "error":
			return Event{Kind: EventError, Text: errText}, true
		default:
			return Event{}, false // replace / stop / quit / redirect
		}
	case "shutdown", "exit":
		return Event{Kind: EventExit}, true
	case "log-message":
		return Event{}, false
	}
	return Event{}, false
}

func (c *ipcConn) emit(ev Event) {
	select {
	case c.events <- ev:
	case <-c.done:
	default:
	}
}

func (c *ipcConn) close() {
	c.once.Do(func() {
		close(c.done)
		_ = c.conn.Close()
		c.pendingMu.Lock()
		for _, ch := range c.pending {
			select {
			case ch <- ipcReply{err: "connection closed"}:
			default:
			}
		}
		c.pending = make(map[int64]chan ipcReply)
		c.pendingMu.Unlock()
	})
}

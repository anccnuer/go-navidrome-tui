package player

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"testing"
	"time"

	"github.com/subsonic-tui/subsonic-tui/internal/model"
)

func testSongs(n int) []model.Song {
	out := make([]model.Song, n)
	for i := range out {
		out[i] = model.Song{ID: fmt.Sprintf("s%d", i), Title: fmt.Sprintf("Track %d", i)}
	}
	return out
}

func TestQueueSequentialAndStopAtEnd(t *testing.T) {
	q := newQueue(ModeSequential)
	song, ok := q.set(testSongs(3), 0)
	if !ok || song.ID != "s0" {
		t.Fatalf("set = %+v %v", song, ok)
	}
	for _, want := range []string{"s1", "s2"} {
		song, ok = q.advance()
		if !ok || song.ID != want {
			t.Fatalf("advance = %+v %v, want %s", song, ok, want)
		}
	}
	if _, ok := q.advance(); ok {
		t.Fatal("sequential mode must stop after the last track")
	}
}

func TestQueueLoopModes(t *testing.T) {
	q := newQueue(ModeLoopOne)
	q.set(testSongs(2), 0)
	for i := 0; i < 3; i++ {
		song, ok := q.advance()
		if !ok || song.ID != "s0" {
			t.Fatalf("loop one escaped to %s", song.ID)
		}
	}

	q = newQueue(ModeLoopAll)
	q.set(testSongs(2), 0)
	q.advance() // s1
	if song, ok := q.advance(); !ok || song.ID != "s0" {
		t.Fatalf("loop all did not wrap: %s %v", song.ID, ok)
	}
}

func TestQueueShuffleCoversEveryTrack(t *testing.T) {
	q := newQueue(ModeShuffle)
	q.set(testSongs(6), 0)
	seen := map[string]bool{"s0": true}
	for i := 0; i < 5; i++ {
		song, ok := q.advance()
		if !ok {
			t.Fatal("shuffle stopped early")
		}
		seen[song.ID] = true
	}
	if len(seen) != 6 {
		t.Fatalf("shuffle repeated tracks: %v", seen)
	}
}

func TestQueuePrevUsesHistory(t *testing.T) {
	q := newQueue(ModeSequential)
	q.set(testSongs(4), 0)
	q.skip(1) // s1
	q.skip(1) // s2
	if song, _ := q.skip(-1); song.ID != "s1" {
		t.Fatalf("prev = %s, want s1", song.ID)
	}
	if song, _ := q.skip(-1); song.ID != "s0" {
		t.Fatalf("prev = %s, want s0", song.ID)
	}
}

func TestQueueEditOperations(t *testing.T) {
	q := newQueue(ModeSequential)
	q.set(testSongs(3), 1) // current s1
	q.insertNext([]model.Song{{ID: "x"}})
	songs, index, _ := q.snapshot()
	if len(songs) != 4 || songs[2].ID != "x" || index != 1 {
		t.Fatalf("insertNext = %v index=%d", ids(songs), index)
	}
	q.removeAt(0)
	songs, index, _ = q.snapshot()
	if len(songs) != 3 || songs[0].ID != "s1" || index != 0 {
		t.Fatalf("removeAt = %v index=%d", ids(songs), index)
	}
	q.appendSongs([]model.Song{{ID: "y"}})
	songs, _, _ = q.snapshot()
	if len(songs) != 4 || songs[3].ID != "y" {
		t.Fatalf("append = %v", ids(songs))
	}
	q.clear()
	if songs, index, _ := q.snapshot(); len(songs) != 0 || index != -1 {
		t.Fatalf("clear = %v index=%d", songs, index)
	}
}

func ids(songs []model.Song) []string {
	out := make([]string, len(songs))
	for i, s := range songs {
		out[i] = s.ID
	}
	return out
}

func TestParseEvent(t *testing.T) {
	cases := []struct {
		msg  ipcMessage
		kind EventKind
		ok   bool
	}{
		{ipcMessage{Event: "property-change", Name: "time-pos", Data: json.RawMessage("12.5")}, EventPosition, true},
		{ipcMessage{Event: "property-change", Name: "pause", Data: json.RawMessage("true")}, EventPause, true},
		{ipcMessage{Event: "property-change", Name: "volume", Data: json.RawMessage("73")}, EventVolume, true},
		{ipcMessage{Event: "file-loaded"}, EventFileLoaded, true},
		// Real mpv events carry "reason" at the top level.
		{ipcMessage{Event: "end-file", Reason: "eof"}, EventEndFile, true},
		{ipcMessage{Event: "end-file", Reason: "replace"}, EventEndFile, false},
		{ipcMessage{Event: "end-file", Reason: "error", Error: "failed"}, EventError, true},
		// Compatibility with a data-embedded variant.
		{ipcMessage{Event: "end-file", Data: json.RawMessage(`{"reason":"eof"}`)}, EventEndFile, true},
		{ipcMessage{Event: "shutdown"}, EventExit, true},
	}
	for i, tc := range cases {
		ev, ok := parseEvent(tc.msg)
		if ok != tc.ok || (ok && ev.Kind != tc.kind) {
			t.Fatalf("case %d: got %+v ok=%v", i, ev, ok)
		}
	}
	ev, _ := parseEvent(ipcMessage{Event: "property-change", Name: "time-pos", Data: json.RawMessage("12.5")})
	if ev.Position != 12.5 {
		t.Fatalf("position = %v", ev.Position)
	}
}

// TestRealMPVEndFileJSON guards the exact wire format: mpv puts "reason" at
// the top level, so a data-only parser silently drops every end-of-track.
func TestRealMPVEndFileJSON(t *testing.T) {
	var msg ipcMessage
	if err := json.Unmarshal([]byte(`{"event":"end-file","reason":"eof"}`), &msg); err != nil {
		t.Fatal(err)
	}
	if msg.Reason != "eof" {
		t.Fatalf("reason = %q", msg.Reason)
	}
	if ev, ok := parseEvent(msg); !ok || ev.Kind != EventEndFile {
		t.Fatalf("event = %+v ok=%v", ev, ok)
	}
}

// TestShuffleAutoAdvancesOnRealEndFileEvent is an end-to-end check of the
// reported bug: a natural end-of-file in shuffle mode must load the next song.
func TestShuffleAutoAdvancesOnRealEndFileEvent(t *testing.T) {
	client, server := net.Pipe()
	defer server.Close()
	ipc := newIPC(client)
	defer ipc.close()

	p := &mpvPlayer{
		q:        newQueue(ModeShuffle),
		urlFor:   func(s model.Song) string { return "http://stream/" + s.ID },
		log:      slog.New(slog.NewTextHandler(io.Discard, nil)),
		events:   make(chan Event, 16),
		stop:     make(chan struct{}),
		procDone: make(chan struct{}),
	}
	p.available.Store(true)
	p.ipc = ipc
	p.q.set(testSongs(3), 0)
	go p.consume(ipc)

	type request struct {
		RequestID int64 `json:"request_id"`
		Command   []any `json:"command"`
	}
	commands := make(chan request, 1)
	go func() {
		line, err := bufio.NewReader(server).ReadString('\n')
		if err != nil {
			return
		}
		var req request
		if json.Unmarshal([]byte(line), &req) == nil {
			commands <- req
		}
	}()

	if _, err := server.Write([]byte(`{"event":"end-file","reason":"eof"}` + "\n")); err != nil {
		t.Fatal(err)
	}
	select {
	case req := <-commands:
		if len(req.Command) < 2 || req.Command[0] != "loadfile" {
			t.Fatalf("expected a loadfile command, got %v", req.Command)
		}
		if _, err := fmt.Fprintf(server, `{"error":"success","request_id":%d}`+"\n", req.RequestID); err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("no loadfile after end-file: playback stops instead of advancing")
	}
	if p.CurrentIndex() == 0 {
		t.Fatal("shuffle queue did not advance to the next track")
	}
}

func TestIPCCallAndEventStream(t *testing.T) {
	client, server := net.Pipe()
	defer server.Close()
	ipc := newIPC(client)
	defer ipc.close()

	go func() {
		reader := bufio.NewReader(server)
		line, err := reader.ReadString('\n')
		if err != nil {
			return
		}
		var req struct {
			RequestID int64 `json:"request_id"`
			Command   []any `json:"command"`
		}
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			return
		}
		if len(req.Command) == 0 || req.Command[0] != "get_property" {
			return
		}
		_, _ = fmt.Fprintf(server, `{"error":"success","data":42,"request_id":%d}`+"\n", req.RequestID)
		_, _ = server.Write([]byte(`{"event":"property-change","name":"time-pos","data":3.5}` + "\n"))
	}()

	data, err := ipc.call("get_property", "time-pos")
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "42" {
		t.Fatalf("data = %s", data)
	}
	select {
	case ev := <-ipc.events:
		if ev.Kind != EventPosition || ev.Position != 3.5 {
			t.Fatalf("event = %+v", ev)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no event received")
	}
}

func TestParseModeRoundTrip(t *testing.T) {
	for _, s := range []string{"sequential", "loopAll", "loopOne", "shuffle"} {
		if got := ModeString(ParseMode(s)); got != s {
			t.Fatalf("%s -> %s", s, got)
		}
	}
}

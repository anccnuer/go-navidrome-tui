package player

import (
	"bufio"
	"encoding/json"
	"fmt"
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
		{ipcMessage{Event: "end-file", Data: json.RawMessage(`{"reason":"eof"}`)}, EventEndFile, true},
		{ipcMessage{Event: "end-file", Data: json.RawMessage(`{"reason":"replace"}`)}, EventEndFile, false},
		{ipcMessage{Event: "end-file", Data: json.RawMessage(`{"reason":"error","error":"failed"}`)}, EventError, true},
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

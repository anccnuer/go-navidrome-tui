// Package lyrics parses LRC and OpenSubsonic structured lyrics into the
// time-indexed model used by the lyrics view.
package lyrics

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/anccnuer/go-navidrome-tui/internal/model"
)

var (
	timestampRe = regexp.MustCompile(`\[(\d{1,3}):(\d{1,2})(?:[.:](\d{1,3}))?\]`)
	metaRe      = regexp.MustCompile(`^\[(ar|ti|al|by|offset|length|re|ve):([^\]]*)\]`)
	// Enhanced LRC word-level markers are stripped but the line is kept.
	wordTimeRe = regexp.MustCompile(`<\d{1,3}:\d{1,2}(?:[.:]\d{1,3})?>`)
)

// ParseLRC parses classic LRC (including multiple timestamps per line and
// [offset:] metadata) into a normalized, sorted lyric sheet.
func ParseLRC(text string) model.Lyrics {
	result := model.Lyrics{Synced: true, Source: "LRC"}
	offset := 0
	for _, raw := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		if m := metaRe.FindStringSubmatch(line); m != nil {
			if strings.EqualFold(m[1], "offset") {
				if v, err := strconv.Atoi(strings.TrimSpace(m[2])); err == nil {
					offset = v
				}
			}
			continue
		}
		matches := timestampRe.FindAllStringSubmatchIndex(line, -1)
		if len(matches) == 0 {
			continue
		}
		body := strings.TrimSpace(line[matches[len(matches)-1][1]:])
		body = wordTimeRe.ReplaceAllString(body, "")
		body = strings.TrimSpace(body)
		for _, m := range matches {
			fragment := line[m[0]:m[1]]
			ts := timestampRe.FindStringSubmatch(fragment)
			if ts == nil {
				continue
			}
			minutes, _ := strconv.Atoi(ts[1])
			seconds, _ := strconv.Atoi(ts[2])
			ms := 0
			if ts[3] != "" {
				digits := ts[3]
				if len(digits) == 1 {
					ms, _ = strconv.Atoi(digits)
					ms *= 100
				} else if len(digits) == 2 {
					ms, _ = strconv.Atoi(digits)
					ms *= 10
				} else {
					ms, _ = strconv.Atoi(digits[:3])
				}
			}
			timeMS := (minutes*60+seconds)*1000 + ms
			result.Lines = append(result.Lines, model.LyricsLine{Time: timeMS, Text: body})
		}
	}
	result = result.Normalize()
	if offset != 0 {
		result = result.Offset(-offset)
	}
	return result
}

// FromStructured converts OpenSubsonic getLyricsBySongId lines. The API
// reports "start" in milliseconds. Synced=false means there are no usable
// timestamps, in which case lines are displayed as plain text.
func FromStructured(lines []model.LyricsLine, synced bool, source string) model.Lyrics {
	if source == "" {
		source = "OpenSubsonic"
	}
	out := model.Lyrics{Synced: synced, Source: source}
	for _, line := range lines {
		text := strings.TrimSpace(line.Text)
		if text == "" {
			continue
		}
		t := line.Time
		if !synced {
			t = 0
		}
		out.Lines = append(out.Lines, model.LyricsLine{Time: t, Text: text})
	}
	if synced {
		return out.Normalize()
	}
	return out
}

// FromPlain wraps unsynced text (e.g. the legacy getLyrics "value" field).
func FromPlain(text string) model.Lyrics {
	out := model.Lyrics{Synced: false, Source: "Subsonic"}
	for _, raw := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		// Some servers still return LRC inside the plain value field.
		if timestampRe.MatchString(line) {
			return ParseLRC(text)
		}
		out.Lines = append(out.Lines, model.LyricsLine{Text: wordTimeRe.ReplaceAllString(line, "")})
	}
	return out
}

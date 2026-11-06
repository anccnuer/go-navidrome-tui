package ui

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

// Terminal input can be polluted by mouse escape sequences (when a terminal
// emits SGR mouse reports that the program does not consume) or by leftover
// bracketed-paste markers. Cleaning every keystroke keeps field values
// predictable and makes URL validation reliable.
var (
	csiSequence   = regexp.MustCompile(`\x1b\[[0-9;?<>=]*[ -/]*[@-~]`)
	mouseResidue  = regexp.MustCompile(`\[<\d+;\d+;\d+[Mm]`)
	bracketMarker = regexp.MustCompile(`\[20[01]~`)
)

func stripTerminalNoise(s string) string {
	s = csiSequence.ReplaceAllString(s, "")
	s = mouseResidue.ReplaceAllString(s, "")
	s = bracketMarker.ReplaceAllString(s, "")
	return strings.ReplaceAll(s, "\x1b", "")
}

func isZeroWidth(r rune) bool {
	switch {
	case r >= 0x200B && r <= 0x200F, r >= 0x202A && r <= 0x202E, r >= 0x2060 && r <= 0x2064:
		return true
	case r == 0xFEFF, r == 0x00AD:
		return true
	}
	return false
}

// cleanInput strips control/ANSI noise and folds full-width ASCII to ASCII,
// so pasting "ｈｔｔｐｓ：／／…" still yields a usable URL.
func cleanInput(s string) string {
	s = stripTerminalNoise(s)
	var b strings.Builder
	for _, r := range s {
		switch {
		case r == '\r' || r == '\n' || r == '\t' || r < 0x20 || r == 0x7f || isZeroWidth(r):
			continue
		case r >= 0xFF01 && r <= 0xFF5E:
			r -= 0xFEE0
		case r == 0x3000:
			r = ' '
		}
		b.WriteRune(r)
	}
	return b.String()
}

// cleanSecret keeps punctuation and full-width characters intact (they are
// legal in a password) but removes control and paste noise.
func cleanSecret(s string) string {
	s = stripTerminalNoise(s)
	var b strings.Builder
	for _, r := range s {
		if r == '\r' || r == '\n' || r == '\t' || r < 0x20 || r == 0x7f || isZeroWidth(r) {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func preferredScheme(raw string) string {
	host := raw
	if i := strings.IndexAny(host, "/:"); i >= 0 {
		host = host[:i]
	}
	host = strings.ToLower(host)
	switch {
	case host == "localhost", host == "127.0.0.1", host == "::1", host == "[::1]",
		strings.HasSuffix(host, ".local"), strings.HasPrefix(host, "192.168."),
		strings.HasPrefix(host, "10."), strings.HasPrefix(host, "172.16."),
		strings.HasPrefix(host, "172.17."), strings.HasPrefix(host, "172.18."),
		strings.HasPrefix(host, "172.19."), strings.HasPrefix(host, "172.2"),
		strings.HasPrefix(host, "172.30."), strings.HasPrefix(host, "172.31."):
		return "http://"
	}
	return "https://"
}

// normalizeURL accepts both "demo.navidrome.org" and a full URL, removes
// pasted noise/whitespace and fills in a sensible scheme.
func normalizeURL(raw string) string {
	v := strings.TrimSpace(cleanInput(raw))
	v = strings.TrimRight(v, "/")
	if v == "" {
		return ""
	}
	if !strings.Contains(v, "://") {
		v = preferredScheme(v) + v
	}
	return v
}

// validateURL returns the canonical URL or a diagnostic error that echoes the
// value actually parsed, making bad input obvious instead of a generic hint.
func validateURL(raw string) (string, error) {
	v := normalizeURL(raw)
	if v == "" {
		return "", fmt.Errorf("服务器地址不能为空")
	}
	u, err := url.Parse(v)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return "", fmt.Errorf("地址格式不正确：%q（示例 https://demo.navidrome.org）", v)
	}
	return strings.TrimRight(u.String(), "/"), nil
}

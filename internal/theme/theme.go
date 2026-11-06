// Package theme defines the palette system and all reusable Lip Gloss styles.
// Styles are built once per theme switch and reused on every frame.
package theme

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"github.com/pelletier/go-toml/v2"
)

// ColorDepth controls how the half-block cover renderer emits ANSI colours.
type ColorDepth int

const (
	DepthTrueColor ColorDepth = iota
	DepthANSI256
	DepthANSI16
)

// DetectDepth asks termenv what the terminal can actually display.
func DetectDepth(allowTrueColor bool) ColorDepth {
	profile := lipgloss.ColorProfile()
	switch profile {
	case termenv.TrueColor:
		if !allowTrueColor {
			return DepthANSI256
		}
		return DepthTrueColor
	case termenv.ANSI256:
		return DepthANSI256
	default:
		return DepthANSI16
	}
}

// Theme is a complete colour palette. All colours are hex strings so they
// work with lipgloss (which downgrades automatically) and with our own ANSI
// renderer in the cover package.
type Theme struct {
	Name        string   `toml:"name"`
	Background  string   `toml:"background"`
	Surface     string   `toml:"surface"`
	Panel       string   `toml:"panel"`
	Elevated    string   `toml:"elevated"`
	Border      string   `toml:"border"`
	BorderFocus string   `toml:"borderFocus"`
	Text        string   `toml:"text"`
	Muted       string   `toml:"muted"`
	Subtle      string   `toml:"subtle"`
	Primary     string   `toml:"primary"`
	Secondary   string   `toml:"secondary"`
	Accent      string   `toml:"accent"`
	Success     string   `toml:"success"`
	Warning     string   `toml:"warning"`
	Danger      string   `toml:"danger"`
	Info        string   `toml:"info"`
	SelectedBg  string   `toml:"selectedBg"`
	SelectedFg  string   `toml:"selectedFg"`
	Gradient    []string `toml:"gradient"`
}

// Names lists the built-in palettes in display order.
func Names() []string { return []string{"musicfox", "aurora", "dracula", "nord", "gruvbox", "mono"} }

// Builtin returns a palette by name, falling back to aurora.
func Builtin(name string) Theme {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "musicfox":
		// Inspired by go-musicfox's default NetEase-red palette: near-black
		// background, red primary, low-contrast selected fill.
		return Theme{
			Name: "musicfox", Background: "#1A1A1A", Surface: "#242424", Panel: "#242424", Elevated: "#2E2E2E",
			Border: "#333333", BorderFocus: "#EA403F", Text: "#FFFFFF", Muted: "#A0A0A0", Subtle: "#6E6E6E",
			Primary: "#EA403F", Secondary: "#FF7A6E", Accent: "#FFB86B", Success: "#34D399", Warning: "#FBBF24",
			Danger: "#F87171", Info: "#38BDF8", SelectedBg: "#4A2424", SelectedFg: "#FFFFFF",
			Gradient: []string{"#EA403F", "#FF8A65", "#FFD166"},
		}
	case "dracula":
		return Theme{
			Name: "dracula", Background: "#1E1F29", Surface: "#282A36", Panel: "#313442", Elevated: "#3B3F52",
			Border: "#44475A", BorderFocus: "#BD93F9", Text: "#F8F8F2", Muted: "#A6A8BE", Subtle: "#6272A4",
			Primary: "#BD93F9", Secondary: "#8BE9FD", Accent: "#FF79C6", Success: "#50FA7B", Warning: "#F1FA8C",
			Danger: "#FF5555", Info: "#8BE9FD", SelectedBg: "#44475A", SelectedFg: "#FFFFFF",
			Gradient: []string{"#BD93F9", "#8BE9FD", "#50FA7B", "#FF79C6"},
		}
	case "nord":
		return Theme{
			Name: "nord", Background: "#242933", Surface: "#2E3440", Panel: "#3B4252", Elevated: "#434C5E",
			Border: "#4C566A", BorderFocus: "#88C0D0", Text: "#ECEFF4", Muted: "#D8DEE9", Subtle: "#7B88A1",
			Primary: "#88C0D0", Secondary: "#81A1C1", Accent: "#B48EAD", Success: "#A3BE8C", Warning: "#EBCB8B",
			Danger: "#BF616A", Info: "#5E81AC", SelectedBg: "#434C5E", SelectedFg: "#FFFFFF",
			Gradient: []string{"#88C0D0", "#81A1C1", "#B48EAD", "#A3BE8C"},
		}
	case "gruvbox":
		return Theme{
			Name: "gruvbox", Background: "#1D2021", Surface: "#282828", Panel: "#32302F", Elevated: "#3C3836",
			Border: "#504945", BorderFocus: "#FE8019", Text: "#EBDBB2", Muted: "#A89984", Subtle: "#7C6F64",
			Primary: "#FE8019", Secondary: "#8EC07C", Accent: "#FABD2F", Success: "#B8BB26", Warning: "#FABD2F",
			Danger: "#FB4934", Info: "#83A598", SelectedBg: "#504945", SelectedFg: "#FFFFFF",
			Gradient: []string{"#FE8019", "#FABD2F", "#B8BB26", "#8EC07C"},
		}
	case "mono":
		return Theme{
			Name: "mono", Background: "#0C0C0C", Surface: "#161616", Panel: "#1F1F1F", Elevated: "#2A2A2A",
			Border: "#3A3A3A", BorderFocus: "#DDDDDD", Text: "#F5F5F5", Muted: "#9A9A9A", Subtle: "#666666",
			Primary: "#F5F5F5", Secondary: "#C9C9C9", Accent: "#FFFFFF", Success: "#C9C9C9", Warning: "#E0E0E0",
			Danger: "#FFFFFF", Info: "#BFBFBF", SelectedBg: "#333333", SelectedFg: "#FFFFFF",
			Gradient: []string{"#777777", "#BBBBBB", "#FFFFFF"},
		}
	default:
		return Theme{
			Name: "aurora", Background: "#0B0E14", Surface: "#131826", Panel: "#1B2233", Elevated: "#222B40",
			Border: "#2C3550", BorderFocus: "#7C5CFF", Text: "#E6EAF2", Muted: "#8B93A7", Subtle: "#5A6380",
			Primary: "#7C5CFF", Secondary: "#22D3EE", Accent: "#F472B6", Success: "#34D399", Warning: "#FBBF24",
			Danger: "#F87171", Info: "#38BDF8", SelectedBg: "#2A2350", SelectedFg: "#FFFFFF",
			Gradient: []string{"#7C5CFF", "#22D3EE", "#F472B6"},
		}
	}
}

// Next cycles through the built-in palette names.
func Next(name string) string {
	names := Names()
	for i, n := range names {
		if strings.EqualFold(n, name) {
			return names[(i+1)%len(names)]
		}
	}
	return names[0]
}

// LoadCustom overlays a user TOML palette on top of the aurora defaults so a
// partially specified file still produces a usable theme.
func LoadCustom(path string) (Theme, error) {
	t := Builtin("aurora")
	data, err := os.ReadFile(path)
	if err != nil {
		return t, fmt.Errorf("读取自定义主题 %s: %w", path, err)
	}
	if err := toml.Unmarshal(data, &t); err != nil {
		return t, fmt.Errorf("解析自定义主题 %s: %w", path, err)
	}
	if len(t.Gradient) == 0 {
		t.Gradient = Builtin("aurora").Gradient
	}
	if t.Name == "" {
		t.Name = "custom"
	}
	return t, nil
}

// Resolve loads the configured theme, tolerating a broken custom file.
func Resolve(name, customPath string) (Theme, error) {
	if strings.TrimSpace(customPath) != "" {
		t, err := LoadCustom(customPath)
		if err == nil {
			return t, nil
		}
		return Builtin(name), err
	}
	return Builtin(name), nil
}

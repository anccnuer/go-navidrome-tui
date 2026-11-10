package ui

import (
	"strings"

	"github.com/anccnuer/go-navidrome-tui/internal/config"
	"github.com/anccnuer/go-navidrome-tui/internal/theme"
	"github.com/anccnuer/go-navidrome-tui/internal/ui/components"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type wizardField struct {
	label       string
	placeholder string
	value       string
	secret      bool
	cursor      int
	prefilled   bool // value came from a saved profile
	selectAll   bool // next typed character replaces the prefilled value
}

type wizardModel struct {
	fields [4]wizardField
	focus  int
	note   string
	err    string
	busy   bool
}

func newWizard(defaults config.Server) wizardModel {
	w := wizardModel{
		note: "填写 Navidrome 服务器信息，连接成功后会保存到 config.toml",
	}
	w.fields = [4]wizardField{
		{label: "名称", placeholder: "home", value: defaults.Name, prefilled: defaults.Name != ""},
		{label: "地址", placeholder: "https://demo.navidrome.org", value: defaults.URL, prefilled: defaults.URL != ""},
		{label: "用户名", placeholder: "demo", value: defaults.Username, prefilled: defaults.Username != ""},
		{label: "密码", placeholder: "••••••", secret: true},
	}
	if w.fields[0].value == "" {
		w.fields[0].value = "home"
		w.fields[0].cursor = 4
	}
	for i := range w.fields {
		w.fields[i].cursor = len([]rune(w.fields[i].value))
	}
	// Focus the first empty field so typing/pasting an address does not land
	// in the prefilled name field.
	w.focus = 0
	for i := range w.fields {
		if w.fields[i].value == "" {
			w.focus = i
			break
		}
	}
	w.focusField(w.focus)
	return w
}

// focusField activates a field. A value loaded from config is selected so the
// first keystroke replaces it instead of appending to it.
func (w *wizardModel) focusField(i int) {
	if i < 0 || i >= len(w.fields) {
		return
	}
	w.focus = i
	f := &w.fields[i]
	f.cursor = len([]rune(f.value))
	f.selectAll = f.prefilled && f.value != ""
	w.err = ""
}

func (w *wizardModel) server() config.Server {
	address, err := validateURL(w.fields[1].value)
	if err != nil {
		address = normalizeURL(w.fields[1].value)
	}
	return config.Server{
		Name:     strings.TrimSpace(cleanInput(w.fields[0].value)),
		URL:      address,
		Username: strings.TrimSpace(cleanInput(w.fields[2].value)),
		Password: config.EncodeSecret(cleanSecret(w.fields[3].value)),
	}
}

// submit validates the form and, when valid, marks the wizard busy. The caller
// then dispatches the async verify command.
func (w *wizardModel) submit() bool {
	if w.busy {
		return false
	}
	w.err = ""
	address, err := validateURL(w.fields[1].value)
	if err != nil {
		w.focusField(1)
		w.err = err.Error()
		return false
	}
	w.fields[1].value = address
	w.fields[1].cursor = len([]rune(address))
	w.fields[1].prefilled = false

	for _, check := range []struct {
		index int
		value string
		msg   string
	}{
		{0, cleanInput(w.fields[0].value), "名称不能为空"},
		{2, cleanInput(w.fields[2].value), "用户名不能为空"},
		{3, cleanSecret(w.fields[3].value), "密码不能为空（也可用环境变量 NAVIDROME_TUI_PASSWORD）"},
	} {
		if strings.TrimSpace(check.value) == "" {
			w.focusField(check.index)
			w.err = check.msg
			return false
		}
	}
	w.busy = true
	w.note = "正在验证连接并保存配置…"
	return true
}

// update returns true when the user asks to connect and save.
func (w *wizardModel) update(msg tea.KeyMsg) bool {
	if w.busy {
		return false
	}
	switch msg.Type {
	case tea.KeyTab, tea.KeyDown:
		w.focusField((w.focus + 1) % len(w.fields))
		return false
	case tea.KeyShiftTab, tea.KeyUp:
		w.focusField((w.focus + len(w.fields) - 1) % len(w.fields))
		return false
	case tea.KeyEnter, tea.KeyCtrlS:
		return w.submit()
	case tea.KeyHome, tea.KeyCtrlA:
		w.fields[w.focus].cursor = 0
		w.fields[w.focus].selectAll = false
		return false
	case tea.KeyEnd, tea.KeyCtrlE:
		w.fields[w.focus].cursor = len([]rune(w.fields[w.focus].value))
		w.fields[w.focus].selectAll = false
		return false
	case tea.KeyLeft:
		w.fields[w.focus].selectAll = false
		if w.fields[w.focus].cursor > 0 {
			w.fields[w.focus].cursor--
		}
		return false
	case tea.KeyRight:
		w.fields[w.focus].selectAll = false
		if f := &w.fields[w.focus]; f.cursor < len([]rune(f.value)) {
			f.cursor++
		}
		return false
	case tea.KeyBackspace, tea.KeyDelete:
		f := &w.fields[w.focus]
		if f.selectAll {
			f.value, f.cursor, f.selectAll = "", 0, false
		} else if f.cursor > 0 {
			runes := []rune(f.value)
			f.value = string(runes[:f.cursor-1]) + string(runes[f.cursor:])
			f.cursor--
		}
		w.err = ""
		return false
	case tea.KeyCtrlU:
		f := &w.fields[w.focus]
		f.value, f.cursor, f.selectAll, f.prefilled = "", 0, false, false
		w.err = ""
		return false
	case tea.KeyCtrlW:
		f := &w.fields[w.focus]
		runes := []rune(f.value)
		head := runes[:min(f.cursor, len(runes))]
		for len(head) > 0 && head[len(head)-1] == ' ' {
			head = head[:len(head)-1]
		}
		for len(head) > 0 && head[len(head)-1] != ' ' {
			head = head[:len(head)-1]
		}
		f.value = string(head) + string(runes[min(f.cursor, len(runes)):])
		f.cursor = len([]rune(string(head)))
		f.selectAll, f.prefilled = false, false
		w.err = ""
		return false
	case tea.KeyRunes:
		insert := cleanInput(string(msg.Runes))
		if w.fields[w.focus].secret {
			insert = cleanSecret(string(msg.Runes))
		}
		if insert == "" {
			return false
		}
		f := &w.fields[w.focus]
		if f.selectAll {
			f.value, f.cursor, f.selectAll = "", 0, false
		}
		runes := []rune(f.value)
		pos := min(f.cursor, len(runes))
		f.value = string(runes[:pos]) + insert + string(runes[pos:])
		f.cursor = pos + len([]rune(insert))
		f.prefilled = false
		w.err = ""
		return false
	default:
		// Some terminals deliver a paste as a key type other than KeyRunes.
		if len(msg.Runes) > 0 {
			return w.update(tea.KeyMsg{Type: tea.KeyRunes, Runes: msg.Runes})
		}
		return false
	}
}

// wizardLayout is shared by render and hit-testing so mouse coordinates always
// match what is drawn on screen.
type wizardLayoutInfo struct {
	cardX, cardY  int
	cardW, cardH  int
	boxW          int
	inputW        int
	firstFieldRow int
	buttonRow     int
	contentRows   int
}

func wizardLayout(width, height int) wizardLayoutInfo {
	cardW := min(66, width-4)
	if cardW < 36 {
		cardW = max(30, width-2)
	}
	cardH := min(20, height-2)
	if cardH < 13 {
		cardH = max(9, height-1)
	}
	boxW := cardW - 2 // border width is already included in this value
	inputW := cardW - 2 - 12
	if inputW < 14 {
		inputW = 14
	}
	g := wizardLayoutInfo{
		cardW: cardW, cardH: cardH, boxW: boxW, inputW: inputW,
		cardX: max(0, (width-boxW)/2), cardY: max(0, (height-cardH)/2),
	}
	// Content rows: 0 title, 1 blank, 2 note, 3 blank, 4..7 fields,
	// 8 blank, 9 status, 10 blank, 11 button, 12 hint.
	g.firstFieldRow = g.cardY + 1 + 4
	g.buttonRow = g.cardY + 1 + 11
	g.contentRows = cardH - 2
	return g
}

// click activates the field under the pointer, or submits when the button row
// is clicked. Coordinates are zero-based terminal cells.
func (w *wizardModel) click(msg tea.MouseMsg, width, height int) (submit, handled bool) {
	if w.busy {
		return false, false
	}
	g := wizardLayout(width, height)
	if msg.X < g.cardX || msg.X >= g.cardX+g.boxW {
		return false, false
	}
	if row := msg.Y - g.firstFieldRow; row >= 0 && row < min(len(w.fields), g.contentRows-4) {
		w.focusField(row)
		f := &w.fields[row]
		// Approximate the caret position from the clicked column; a click on
		// the label simply puts the caret at the start.
		col := msg.X - (g.cardX + 1 + 11 + 2)
		f.cursor = max(0, min(col, len([]rune(f.value))))
		f.selectAll = false
		return false, true
	}
	if g.contentRows > 11 && msg.Y == g.buttonRow {
		return true, true
	}
	return false, false
}

func (w wizardModel) render(s *theme.Styles, width, height int) string {
	g := wizardLayout(width, height)
	innerW := g.cardW - 2
	inputW := g.inputW

	lines := make([]string, 0, 14)
	lines = append(lines, " "+s.Gradient("◈  NAVIDROME  ·  连接向导"))
	lines = append(lines, "")
	lines = append(lines, " "+s.Muted.Render(components.Truncate(w.note, innerW-2)))
	lines = append(lines, "")
	for i, f := range w.fields {
		label := s.Muted.Render("  " + f.label + "   ")
		display := f.value
		if f.secret {
			display = strings.Repeat("•", len([]rune(f.value)))
		}
		if i == w.focus {
			if display == "" {
				display = "▏" + f.placeholder
			} else if f.selectAll {
				display = "▏" + display
			} else {
				runes := []rune(display)
				pos := min(f.cursor, len(runes))
				display = string(runes[:pos]) + "▏" + string(runes[pos:])
			}
		} else if display == "" {
			display = s.Subtle.Render(f.placeholder)
		}
		style := s.Input
		if i == w.focus {
			style = s.InputFocus
		}
		box := style.Width(inputW).Render(" " + components.PadRight(display, inputW-2))
		lines = append(lines, label+box)
	}
	lines = append(lines, "")
	if w.err != "" {
		lines = append(lines, " "+s.Danger.Render("⚠ "+components.Truncate(w.err, innerW-4)))
	} else if w.busy {
		lines = append(lines, " "+s.Warning.Render("◌ 正在连接…"))
	} else {
		lines = append(lines, " "+s.Subtle.Render("密码以 enc: 形式保存在本地，并非加密存储"))
	}
	lines = append(lines, "")
	lines = append(lines, " "+s.BadgeActive.Render(" [ 连接并保存 ] "))
	lines = append(lines, " "+s.Subtle.Render("点击输入框切换 · Tab 切换 · Enter 连接 · Ctrl+C 退出"))

	content := components.FitLines(strings.Join(lines, "\n"), innerW-2, g.contentRows)
	box := s.PanelFocus.Width(innerW - 2).Height(g.contentRows).Render(content)
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, box)
}

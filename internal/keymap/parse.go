package keymap

import (
	"fmt"
	"strings"
)

// fullwidthAliases lets Chinese/Japanese IME input work without switching to
// ASCII, exactly like go-musicfox does.
var fullwidthAliases = map[string]string{
	"／": "/", "？": "?", "，": ",", "。": ".", "【": "[", "】": "]",
	"＜": "<", "＞": ">", "〈": "<", "〉": ">", "《": "<", "》": ">",
	"«": "<", "»": ">", "；": ";", "：": ":", "ー": "-", "−": "-",
	"＝": "=", "＋": "+", "、": "/", "｢": "[", "｣": "]",
}

// Normalize converts a key into its canonical lookup form. Single characters
// preserve case (so "a" and "A" are distinct); named and chord keys are
// case-insensitive.
func Normalize(key string) string {
	if key == " " {
		return "space"
	}
	k := strings.TrimSpace(key)
	if k == "" {
		return ""
	}
	if alias, ok := fullwidthAliases[k]; ok {
		k = alias
	}
	if strings.ContainsRune(k, '+') || len([]rune(k)) > 1 {
		k = strings.ToLower(k)
	}
	switch k {
	case " ":
		return "space"
	case "escape":
		return "esc"
	case "pageup", "pgup":
		return "pgup"
	case "pagedown", "pgdn":
		return "pgdown"
	case "return", "cr":
		return "enter"
	case "spacebar":
		return "space"
	default:
		return k
	}
}

func parseKeys(value any) ([]string, error) {
	switch v := value.(type) {
	case nil:
		return nil, nil
	case string:
		if strings.TrimSpace(v) == "" {
			return nil, nil
		}
		return []string{v}, nil
	case []string:
		out := make([]string, 0, len(v))
		for _, s := range v {
			if strings.TrimSpace(s) != "" {
				out = append(out, s)
			}
		}
		return out, nil
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			s, ok := item.(string)
			if !ok {
				return nil, fmt.Errorf("键位数组只能包含字符串，实际为 %T", item)
			}
			if strings.TrimSpace(s) != "" {
				out = append(out, s)
			}
		}
		return out, nil
	default:
		return nil, fmt.Errorf("不支持的键位值类型 %T（应为字符串或字符串数组）", value)
	}
}

func knownBindings() map[Action]Binding {
	out := make(map[Action]Binding, len(DefaultBindings))
	for _, b := range DefaultBindings {
		out[b.Action] = b
	}
	return out
}

func builtinActions() map[Action]bool {
	out := make(map[Action]bool)
	for _, b := range DefaultBindings {
		if b.Builtin {
			out[b.Action] = true
		}
	}
	return out
}

// Build produces the effective key table. warnings contains user-facing
// diagnostics for unknown actions, bad values and key conflicts.
func Build(useDefault bool, overrides map[string]any) (*Resolver, []string) {
	r := &Resolver{
		byKey:      make(map[string]Action, 128),
		actionKeys: make(map[Action][]string, len(DefaultBindings)),
	}
	var warnings []string
	builtins := builtinActions()

	for _, b := range DefaultBindings {
		if b.Builtin {
			r.register(b, &warnings)
		}
	}
	if useDefault {
		for _, b := range DefaultBindings {
			if !b.Builtin {
				r.register(b, &warnings)
			}
		}
	}

	known := knownBindings()
	for name, raw := range overrides {
		action := Action(name)
		if builtins[action] {
			warnings = append(warnings, fmt.Sprintf("内置操作 %q 不可自定义，已忽略", name))
			continue
		}
		_, ok := known[action]
		if !ok {
			warnings = append(warnings, fmt.Sprintf("未知操作 %q，已忽略", name))
			continue
		}
		keys, err := parseKeys(raw)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("操作 %q 的键位无效：%v", name, err))
			continue
		}
		r.removeAction(action)
		for _, key := range keys {
			norm := Normalize(key)
			if existing, ok := r.byKey[norm]; ok && existing != action {
				if builtins[existing] {
					warnings = append(warnings, fmt.Sprintf("键位 %q 已被内置操作 %q 占用，已忽略", key, existing))
					continue
				}
			}
			if r.byKey[norm] != "" && r.byKey[norm] != action {
				r.conflicts = append(r.conflicts, fmt.Sprintf("%q 同时绑定到 %q 和 %q", key, r.byKey[norm], action))
				continue
			}
			r.byKey[norm] = action
			r.actionKeys[action] = append(r.actionKeys[action], key)
		}
	}

	r.finalize()
	return r, warnings
}

func (r *Resolver) register(b Binding, warnings *[]string) {
	for _, key := range b.Keys {
		norm := Normalize(key)
		if existing, ok := r.byKey[norm]; ok && existing != b.Action {
			msg := fmt.Sprintf("键位 %q 冲突：%q 与 %q（保留 %q）", key, existing, b.Action, existing)
			r.conflicts = append(r.conflicts, msg)
			*warnings = append(*warnings, msg)
			continue
		}
		r.byKey[norm] = b.Action
		r.actionKeys[b.Action] = append(r.actionKeys[b.Action], key)
	}
}

func (r *Resolver) removeAction(a Action) {
	for _, key := range r.actionKeys[a] {
		norm := Normalize(key)
		if r.byKey[norm] == a {
			delete(r.byKey, norm)
		}
	}
	delete(r.actionKeys, a)
}

// finalize builds the display-order binding list from the effective keys.
func (r *Resolver) finalize() {
	seen := make(map[Action]bool, len(DefaultBindings))
	r.bindings = make([]Binding, 0, len(DefaultBindings))
	for _, b := range DefaultBindings {
		keys := r.actionKeys[b.Action]
		if len(keys) == 0 {
			continue
		}
		seen[b.Action] = true
		r.bindings = append(r.bindings, Binding{
			Action: b.Action, Keys: append([]string(nil), keys...),
			Label: b.Label, Group: b.Group, Builtin: b.Builtin,
		})
	}
	for action, keys := range r.actionKeys {
		if seen[action] || len(keys) == 0 {
			continue
		}
		r.bindings = append(r.bindings, Binding{Action: action, Keys: keys, Label: string(action), Group: GroupSystem})
	}
}

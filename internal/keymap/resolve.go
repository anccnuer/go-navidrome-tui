package keymap

// Resolve returns the first action bound to any of the candidate key strings.
// UI code passes tea.KeyMsg.String() plus a few compatibility aliases.
func (r *Resolver) Resolve(candidates ...string) (Action, bool) {
	for _, candidate := range candidates {
		norm := Normalize(candidate)
		if norm == "" {
			continue
		}
		if action, ok := r.byKey[norm]; ok {
			return action, true
		}
	}
	return "", false
}

// KeyCandidates expands a raw terminal key into all forms the resolver knows.
func KeyCandidates(raw string) []string {
	out := []string{raw}
	norm := Normalize(raw)
	if norm != raw {
		out = append(out, norm)
	}
	switch norm {
	case "space":
		out = append(out, " ")
	case "esc":
		out = append(out, "escape")
	case "enter":
		out = append(out, "return")
	case "pgup":
		out = append(out, "pageup")
	case "pgdown":
		out = append(out, "pagedown")
	}
	return out
}

// Bindings returns the effective bindings in display order.
func (r *Resolver) Bindings() []Binding { return r.bindings }

// KeysFor returns the keys currently bound to an action (original spelling).
func (r *Resolver) KeysFor(a Action) []string {
	return append([]string(nil), r.actionKeys[a]...)
}

// Has reports whether an action has at least one key.
func (r *Resolver) Has(a Action) bool { return len(r.actionKeys[a]) > 0 }

// Conflicts returns all detected duplicate key declarations.
func (r *Resolver) Conflicts() []string { return append([]string(nil), r.conflicts...) }

// LabelFor looks up the human readable label and group of an action.
func (r *Resolver) LabelFor(a Action) (label, group string) {
	for _, b := range r.bindings {
		if b.Action == a {
			return b.Label, b.Group
		}
	}
	return string(a), GroupSystem
}

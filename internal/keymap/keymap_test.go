package keymap

import (
	"strings"
	"testing"
)

func defaultResolver(t *testing.T) *Resolver {
	t.Helper()
	r, warnings := Build(true, nil)
	if len(warnings) != 0 {
		t.Fatalf("default bindings should be conflict-free: %v", warnings)
	}
	return r
}

func TestDefaultBindings(t *testing.T) {
	r := defaultResolver(t)
	cases := []struct {
		key  string
		want Action
	}{
		{" ", ActionPlayToggle}, {"space", ActionPlayToggle},
		{"[", ActionPrev}, {"【", ActionPrev}, {"]", ActionNext}, {"】", ActionNext},
		{"?", ActionHelp}, {"？", ActionHelp},
		{"/", ActionSearch}, {"／", ActionSearch}, {"、", ActionSearch},
		{"ctrl+u", ActionPageUp}, {"pgup", ActionPageUp},
		{"q", ActionQuit}, {"G", ActionMoveBottom}, {"g", ActionMoveTop},
		{"tab", ActionAddToPlaylist}, {"shift+tab", ActionRemoveFromPlaylist},
		{"esc", ActionBack}, {"enter", ActionEnter},
		{",", ActionLikePlaying}, {"<", ActionLikeSelected},
		{"p", ActionSwitchMode}, {"c", ActionPlaylistView},
		{"y", ActionLyricsView}, {"ctrl+f", ActionGlobalSearch},
	}
	for _, tc := range cases {
		got, ok := r.Resolve(KeyCandidates(tc.key)...)
		if !ok || got != tc.want {
			t.Errorf("Resolve(%q) = %q, %v; want %q", tc.key, got, ok, tc.want)
		}
	}
	if _, ok := r.Resolve("alt+z"); ok {
		t.Fatal("unbound key should not resolve")
	}
	// Album/artist browsing is intentionally not part of this build.
	for _, key := range []string{"a", "s", "A", "S"} {
		if action, ok := r.Resolve(key); ok {
			t.Fatalf("%q should be unbound, resolves to %q", key, action)
		}
	}
}

func TestBuiltinCannotBeOverridden(t *testing.T) {
	r, warnings := Build(true, map[string]any{"quit": "alt+q"})
	if len(warnings) == 0 {
		t.Fatal("expected a warning for overriding a built-in action")
	}
	if a, _ := r.Resolve("q"); a != ActionQuit {
		t.Fatalf("quit lost its binding: %q", a)
	}
	if a, ok := r.Resolve("alt+q"); ok {
		t.Fatalf("alt+q should not be bound, got %q", a)
	}
}

func TestOverrideReplacesDefaultKeys(t *testing.T) {
	r, warnings := Build(true, map[string]any{
		"next": []any{"alt+n", "n"},
		"help": []any{"?", "ctrl+h"},
	})
	joined := strings.Join(warnings, " ")
	if !strings.Contains(joined, "内置操作") {
		t.Fatalf("expected a warning about the built-in 'n' key, got %v", warnings)
	}
	if a, ok := r.Resolve("]"); ok && a == ActionNext {
		t.Fatal("default ']' binding should have been replaced")
	}
	if a, _ := r.Resolve("alt+n"); a != ActionNext {
		t.Fatalf("alt+n = %q", a)
	}
	if a, _ := r.Resolve("ctrl+h"); a != ActionHelp {
		t.Fatalf("ctrl+h = %q", a)
	}
}

func TestDisableBindingAndUseDefaultFalse(t *testing.T) {
	r, _ := Build(true, map[string]any{"help": ""})
	if r.Has(ActionHelp) {
		t.Fatal("help should have been disabled")
	}
	r2, _ := Build(false, map[string]any{"next": "]"})
	if !r2.Has(ActionQuit) || !r2.Has(ActionSearch) {
		t.Fatal("built-ins must survive useDefaultKeyBindings=false")
	}
	if r2.Has(ActionPlayToggle) {
		t.Fatal("custom-only mode should not contain default bindings")
	}
	if a, _ := r2.Resolve("]"); a != ActionNext {
		t.Fatalf("next = %q", a)
	}
}

func TestConflictAndUnknownActionWarnings(t *testing.T) {
	r, warnings := Build(true, map[string]any{
		"next":         "p",
		"doesNotExist": "z",
		"previous":     42,
	})
	if len(warnings) < 2 {
		t.Fatalf("expected warnings, got %v", warnings)
	}
	if len(r.Conflicts()) == 0 {
		t.Fatal("expected a recorded conflict")
	}
	if a, _ := r.Resolve("p"); a != ActionSwitchMode {
		t.Fatalf("conflict should keep the first binding, got %q", a)
	}
}

func TestHelpGroups(t *testing.T) {
	r := defaultResolver(t)
	groups := r.HelpGroups()
	if len(groups) < 5 {
		t.Fatalf("groups = %d", len(groups))
	}
	if groups[0].Title != GroupNavigation || len(groups[0].Entries) < 10 {
		t.Fatalf("navigation group = %+v", groups[0])
	}
	filtered := r.FilterHelp("播放 / 暂停")
	total := 0
	for _, g := range filtered {
		total += len(g.Entries)
	}
	if total != 1 {
		t.Fatalf("filter returned %d entries", total)
	}
	if keys := r.KeysFor(ActionNext); len(keys) == 0 {
		t.Fatal("KeysFor(next) is empty")
	}
}

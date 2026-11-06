package keymap

import "strings"

// HelpEntry is one row of the auto-generated shortcut sheet.
type HelpEntry struct {
	Label  string
	Keys   []string
	Action Action
}

// HelpGroup is a titled section of the shortcut sheet.
type HelpGroup struct {
	Title   string
	Entries []HelpEntry
}

var groupOrder = []string{
	GroupNavigation,
	GroupPlayback,
	GroupQueue,
	GroupAnnotation,
	GroupViews,
	GroupSystem,
}

// HelpGroups builds a grouped shortcut sheet from the effective bindings.
func (r *Resolver) HelpGroups() []HelpGroup {
	return r.FilterHelp("")
}

// FilterHelp builds the sheet keeping only rows matching query (label, action
// name or key). An empty query returns everything.
func (r *Resolver) FilterHelp(query string) []HelpGroup {
	query = strings.ToLower(strings.TrimSpace(query))
	byGroup := make(map[string][]HelpEntry, len(groupOrder))
	for _, b := range r.bindings {
		if query != "" && !entryMatches(b, query) {
			continue
		}
		byGroup[b.Group] = append(byGroup[b.Group], HelpEntry{
			Label:  b.Label,
			Keys:   append([]string(nil), b.Keys...),
			Action: b.Action,
		})
	}
	out := make([]HelpGroup, 0, len(groupOrder))
	for _, title := range groupOrder {
		entries := byGroup[title]
		if len(entries) == 0 {
			continue
		}
		out = append(out, HelpGroup{Title: title, Entries: entries})
		delete(byGroup, title)
	}
	// Custom / unknown groups are appended in a stable order if any remain.
	for _, title := range []string{GroupViews, GroupSystem} {
		if entries := byGroup[title]; len(entries) > 0 {
			out = append(out, HelpGroup{Title: title, Entries: entries})
			delete(byGroup, title)
		}
	}
	return out
}

func entryMatches(b Binding, query string) bool {
	if strings.Contains(strings.ToLower(b.Label), query) ||
		strings.Contains(strings.ToLower(string(b.Action)), query) ||
		strings.Contains(strings.ToLower(b.Group), query) {
		return true
	}
	for _, key := range b.Keys {
		if strings.Contains(strings.ToLower(key), query) {
			return true
		}
	}
	return false
}

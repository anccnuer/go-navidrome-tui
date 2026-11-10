package ui

import (
	"github.com/anccnuer/go-navidrome-tui/internal/model"
	tea "github.com/charmbracelet/bubbletea"
)

// pushNode enters a new navigation level and starts loading it asynchronously.
func (m *Model) pushNode(kind model.NodeKind, id, title string) tea.Cmd {
	node := &Node{Kind: kind, ID: id, Title: title}
	m.stack = append(m.stack, node)
	m.view = model.ViewLibrary
	// Entering a library node always lands on the main screen; do not leave a
	// stale immersive-return flag behind.
	m.overlayReturn = false
	m.filterActive = false
	m.filterInput = ""
	return m.loadNodeCmd(node, 0)
}

// popNode returns to the parent level. false means we are already at the root.
func (m *Model) popNode() bool {
	if len(m.stack) <= 1 {
		return false
	}
	m.stack = m.stack[:len(m.stack)-1]
	return true
}

// currentNode returns the node at the top of the stack.
func (m *Model) currentNode() *Node {
	if len(m.stack) == 0 {
		return nil
	}
	return m.stack[len(m.stack)-1]
}

// rootNode returns the first level of the navigation stack.
func (m *Model) rootNode() *Node {
	if len(m.stack) == 0 {
		return nil
	}
	return m.stack[0]
}

// breadcrumbs builds the "艺术家 / 专辑 / 曲目" trail.
func (m *Model) breadcrumbs() string {
	parts := make([]string, 0, len(m.stack))
	for _, node := range m.stack {
		if node.Title != "" {
			parts = append(parts, node.Title)
		}
	}
	return joinWith(parts, "  ›  ")
}

func joinWith(parts []string, sep string) string {
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += sep
		}
		out += p
	}
	return out
}

// reloadCurrent re-fetches the active node, e.g. after a star mutation.
func (m *Model) reloadCurrent() tea.Cmd {
	node := m.currentNode()
	if node == nil {
		return nil
	}
	node.Page = 0
	node.Items = nil
	return m.loadNodeCmd(node, 0)
}

// loadNextPage appends the following page when the cursor reaches the end.
func (m *Model) loadNextPage() tea.Cmd {
	node := m.currentNode()
	if node == nil || node.Loading || !node.HasMore {
		return nil
	}
	node.Page++
	return m.loadNodeCmd(node, node.Page)
}

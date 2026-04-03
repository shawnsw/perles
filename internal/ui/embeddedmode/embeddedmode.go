// Package embeddedmode provides a view shown when beads is using dolt embedded mode.
// Embedded mode takes an exclusive file lock, so perles cannot access the database
// concurrently. The user needs to switch to dolt server mode.
package embeddedmode

import (
	"github.com/zjrosen/perles/internal/keys"
	"github.com/zjrosen/perles/internal/ui/shared/chainart"
	"github.com/zjrosen/perles/internal/ui/styles"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Model holds the embedded-mode view state.
type Model struct {
	width  int
	height int
}

// New creates a view explaining that embedded dolt mode is not supported.
func New() Model {
	return Model{}
}

// Init returns the initial command.
func (m Model) Init() tea.Cmd {
	return nil
}

// Update handles messages.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
		switch {
		case key.Matches(msg, keys.Common.Quit), key.Matches(msg, keys.Common.Escape):
			return m, tea.Quit
		}
	}
	return m, nil
}

// View renders the embedded-mode state.
func (m Model) View() string {
	if m.width == 0 || m.height == 0 {
		return ""
	}

	art := chainart.BuildChainArt()

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(styles.TextPrimaryColor)

	messageStyle := lipgloss.NewStyle().
		Foreground(styles.TextDescriptionColor)

	cmdStyle := lipgloss.NewStyle().
		Foreground(styles.StatusSuccessColor).
		Bold(true)

	hintStyle := lipgloss.NewStyle().
		Foreground(styles.TextMutedColor).
		Italic(true)

	content := lipgloss.JoinVertical(
		lipgloss.Center,
		art,
		"\n",
		titleStyle.Render("Oh no! Looks like there's a break in the chain!"),
		"",
		messageStyle.Render("Beads is using embedded dolt mode, which takes an exclusive lock"),
		messageStyle.Render("on the database. Perles needs server mode for concurrent access."),
		"",
		messageStyle.Render("Re-initialize your project in server mode:"),
		"",
		"  "+cmdStyle.Render("bd init --server"),
		"\n",
		hintStyle.Render("Press q to quit"),
	)

	containerStyle := lipgloss.NewStyle().
		Width(m.width).
		Height(m.height).
		Align(lipgloss.Center, lipgloss.Center)

	return containerStyle.Render(content)
}

// SetSize updates the view dimensions.
func (m Model) SetSize(width, height int) Model {
	m.width = width
	m.height = height
	return m
}

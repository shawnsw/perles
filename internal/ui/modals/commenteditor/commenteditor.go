// Package commenteditor provides a modal for creating issue comments.
package commenteditor

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/zjrosen/perles/internal/task"
	"github.com/zjrosen/perles/internal/ui/shared/formmodal"
	"github.com/zjrosen/perles/internal/ui/shared/issuebadge"
)

// Model holds the comment editor state.
type Model struct {
	issue task.Issue
	form  formmodal.Model
}

// SaveMsg is sent when the user confirms comment creation.
type SaveMsg struct {
	IssueID string
	Author  string
	Text    string
}

// CancelMsg is sent when the user cancels the modal.
type CancelMsg struct{}

// New creates a new comment editor with vim mode disabled by default.
func New(issue task.Issue) Model {
	return NewWithVimMode(issue, false)
}

// NewWithVimMode creates a new comment editor with the given vim mode setting.
func NewWithVimMode(issue task.Issue, vimEnabled bool) Model {
	m := Model{issue: issue}

	cfg := formmodal.FormConfig{
		Title: "Add Comment",
		TitleContent: func(width int) string {
			return issuebadge.RenderBadge(m.issue)
		},
		Fields: []formmodal.FieldConfig{
			{
				Key:         "author",
				Type:        formmodal.FieldTypeText,
				Label:       "Author",
				Hint:        "required",
				Placeholder: "Your name",
				MaxLength:   80,
			},
			{
				Key:         "text",
				Type:        formmodal.FieldTypeTextArea,
				Label:       "Comment",
				Hint:        "Ctrl+G for editor",
				Placeholder: "Write a comment...",
				VimEnabled:  vimEnabled,
				MaxHeight:   12,
			},
		},
		SubmitLabel: "Add Comment",
		MinWidth:    56,
		Validate: func(values map[string]any) error {
			author := strings.TrimSpace(values["author"].(string))
			if author == "" {
				return fmt.Errorf("author is required")
			}
			text := values["text"].(string)
			if strings.TrimSpace(text) == "" {
				return fmt.Errorf("comment is required")
			}
			return nil
		},
		OnSubmit: func(values map[string]any) tea.Msg {
			return SaveMsg{
				IssueID: m.issue.ID,
				Author:  strings.TrimSpace(values["author"].(string)),
				Text:    values["text"].(string),
			}
		},
		OnCancel: func() tea.Msg { return CancelMsg{} },
	}

	m.form = formmodal.New(cfg)
	return m
}

// SetSize sets the viewport dimensions for overlay rendering.
func (m Model) SetSize(width, height int) Model {
	m.form = m.form.SetSize(width, height)
	return m
}

// Init initializes the model.
func (m Model) Init() tea.Cmd {
	return nil
}

// Update handles messages.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	var cmd tea.Cmd
	m.form, cmd = m.form.Update(msg)
	return m, cmd
}

// View renders the comment editor modal.
func (m Model) View() string {
	return m.form.View()
}

// Overlay renders the comment editor on top of a background view.
func (m Model) Overlay(background string) string {
	return m.form.Overlay(background)
}

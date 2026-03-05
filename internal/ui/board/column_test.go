package board

import (
	"errors"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"
	zone "github.com/lrstanley/bubblezone"
	"github.com/stretchr/testify/require"

	"github.com/zjrosen/perles/internal/task"
)

func TestColumn_NewColumn(t *testing.T) {
	c := NewColumn("Test")
	require.Equal(t, "Test", c.title)
}

func TestColumn_SetItems(t *testing.T) {
	c := NewColumn("Test")
	issues := []task.Issue{
		{ID: "bd-1", TitleText: "Issue 1"},
		{ID: "bd-2", TitleText: "Issue 2"},
	}
	c = c.SetItems(issues)
	require.Len(t, c.Items(), 2)
}

func TestColumn_SetItems_Empty(t *testing.T) {
	c := NewColumn("Test")
	c = c.SetItems([]task.Issue{})
	require.Empty(t, c.Items())
}

func TestColumn_SelectedItem_Empty(t *testing.T) {
	c := NewColumn("Test")
	require.Nil(t, c.SelectedItem(), "expected nil selected item on empty column")
}

func TestColumn_SelectedItem_WithItems(t *testing.T) {
	c := NewColumn("Test")
	issues := []task.Issue{
		{ID: "bd-1", TitleText: "Issue 1"},
		{ID: "bd-2", TitleText: "Issue 2"},
	}
	c = c.SetItems(issues)
	selected := c.SelectedItem()
	require.NotNil(t, selected, "expected non-nil selected item")
	require.Equal(t, "bd-1", selected.ID, "expected first item selected")
}

func TestColumn_SelectByID(t *testing.T) {
	c := NewColumn("Test")
	issues := []task.Issue{
		{ID: "bd-1", TitleText: "Issue 1"},
		{ID: "bd-2", TitleText: "Issue 2"},
		{ID: "bd-3", TitleText: "Issue 3"},
	}
	c = c.SetItems(issues)

	c, found := c.SelectByID("bd-2")
	require.True(t, found, "expected to find bd-2")
	selected := c.SelectedItem()
	require.NotNil(t, selected, "expected selected item")
	require.Equal(t, "bd-2", selected.ID, "expected bd-2 to be selected")
}

func TestColumn_SelectByID_NotFound(t *testing.T) {
	c := NewColumn("Test")
	issues := []task.Issue{{ID: "bd-1", TitleText: "Issue 1"}}
	c = c.SetItems(issues)

	_, found := c.SelectByID("nonexistent")
	require.False(t, found, "expected not to find nonexistent issue")
}

func TestColumn_SetFocused(t *testing.T) {
	c := NewColumn("Test")
	c = c.SetFocused(true).(Column)
	require.True(t, *c.focused, "expected column to be focused")
	c = c.SetFocused(false).(Column)
	require.False(t, *c.focused, "expected column to be unfocused")
}

func TestColumn_SetSize(t *testing.T) {
	c := NewColumn("Test")
	c = c.SetSize(50, 20).(Column)
	require.Equal(t, 50, c.width)
	require.Equal(t, 20, c.height)
}

func TestColumn_Title_Empty(t *testing.T) {
	c := NewColumn("Test")
	title := c.Title()
	require.Equal(t, "Test (0)", title)
}

func TestColumn_View_Empty(t *testing.T) {
	c := NewColumn("Test")
	c = c.SetSize(30, 10).(Column)
	view := c.View()
	require.Contains(t, view, "No issues")
}

func TestColumn_Title_WithItems(t *testing.T) {
	c := NewColumn("Ready")
	issues := []task.Issue{
		{ID: "bd-1", TitleText: "Issue 1", Priority: task.PriorityHigh, Type: task.TypeTask},
		{ID: "bd-2", TitleText: "Issue 2", Priority: task.PriorityMedium, Type: task.TypeBug},
	}
	c = c.SetItems(issues)
	title := c.Title()
	require.Equal(t, "Ready (2)", title)
}

func TestColumn_View_WithItems(t *testing.T) {
	c := NewColumn("Ready")
	c = c.SetSize(50, 20).(Column)
	issues := []task.Issue{
		{ID: "bd-1", TitleText: "Issue 1", Priority: task.PriorityHigh, Type: task.TypeTask},
		{ID: "bd-2", TitleText: "Issue 2", Priority: task.PriorityMedium, Type: task.TypeBug},
	}
	c = c.SetItems(issues)
	view := c.View()
	// View now returns only content, not header
	require.NotEmpty(t, view, "expected non-empty view")
}

func TestColumn_SetShowCounts(t *testing.T) {
	c := NewColumn("Test")
	c = c.SetShowCounts(false).(Column)
	require.NotNil(t, c.showCounts)
	require.False(t, *c.showCounts, "expected showCounts to be false")
	c = c.SetShowCounts(true).(Column)
	require.True(t, *c.showCounts, "expected showCounts to be true")
}

func TestColumn_Title_ShowCountsFalse(t *testing.T) {
	c := NewColumn("Ready")
	issues := []task.Issue{
		{ID: "bd-1", TitleText: "Issue 1"},
		{ID: "bd-2", TitleText: "Issue 2"},
	}
	c = c.SetItems(issues)
	c = c.SetShowCounts(false).(Column)
	title := c.Title()
	// Should show just title without count
	require.Equal(t, "Ready", title)
}

func TestColumn_Title_ShowCountsTrue(t *testing.T) {
	c := NewColumn("Ready")
	issues := []task.Issue{
		{ID: "bd-1", TitleText: "Issue 1"},
		{ID: "bd-2", TitleText: "Issue 2"},
	}
	c = c.SetItems(issues)
	c = c.SetShowCounts(true).(Column)
	title := c.Title()
	// Should show title with count
	require.Equal(t, "Ready (2)", title)
}

func TestColumn_Title_ShowCountsDefault(t *testing.T) {
	// When showCounts is nil (not set), should default to showing counts
	c := NewColumn("Ready")
	issues := []task.Issue{
		{ID: "bd-1", TitleText: "Issue 1"},
	}
	c = c.SetItems(issues)
	// Don't call SetShowCounts - leave as nil
	title := c.Title()
	require.Equal(t, "Ready (1)", title)
}

func TestColumn_Update_NavigateDown(t *testing.T) {
	c := NewColumn("Test")
	issues := []task.Issue{
		{ID: "bd-1", TitleText: "Issue 1"},
		{ID: "bd-2", TitleText: "Issue 2"},
	}
	c = c.SetItems(issues)

	updated, _ := c.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	c = updated.(Column)
	selected := c.SelectedItem()
	require.NotNil(t, selected)
	require.Equal(t, "bd-2", selected.ID, "expected bd-2 after down navigation")
}

func TestColumn_Update_NavigateUp(t *testing.T) {
	c := NewColumn("Test")
	issues := []task.Issue{
		{ID: "bd-1", TitleText: "Issue 1"},
		{ID: "bd-2", TitleText: "Issue 2"},
	}
	c = c.SetItems(issues)

	// Navigate down first
	updated, _ := c.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	c = updated.(Column)
	// Then up
	updated, _ = c.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	c = updated.(Column)
	selected := c.SelectedItem()
	require.NotNil(t, selected)
	require.Equal(t, "bd-1", selected.ID, "expected bd-1 after up navigation")
}

func TestColumn_Items(t *testing.T) {
	c := NewColumn("Test")
	issues := []task.Issue{
		{ID: "bd-1", TitleText: "Issue 1"},
		{ID: "bd-2", TitleText: "Issue 2"},
	}
	c = c.SetItems(issues)
	items := c.Items()
	require.Len(t, items, 2)
	require.Equal(t, "bd-1", items[0].ID, "expected first item bd-1")
}

// TestColumn_View_Golden uses teatest golden file comparison
// Run with -update flag to update golden files: go test -update ./internal/ui/board/...
func TestColumn_View_Golden(t *testing.T) {
	c := NewColumn("Ready").SetSize(30, 15).(Column)
	view := zone.Scan(c.View()) // Scan to strip zone markers (matches production behavior)
	teatest.RequireEqualOutput(t, []byte(view))
}

// TestColumn_View_WithIssues_Golden tests column with sample issues
func TestColumn_View_WithIssues_Golden(t *testing.T) {
	c := NewColumn("Ready").SetSize(30, 15).(Column).SetFocused(true).(Column)
	issues := []task.Issue{
		{ID: "bd-1", TitleText: "First Issue", Priority: task.PriorityHigh, Type: task.TypeBug},
		{ID: "bd-2", TitleText: "Second Issue", Priority: task.PriorityMedium, Type: task.TypeTask},
		{ID: "bd-3", TitleText: "Third Issue", Priority: task.PriorityLow, Type: task.TypeFeature},
	}
	c = c.SetItems(issues)
	view := zone.Scan(c.View()) // Scan to strip zone markers (matches production behavior)
	teatest.RequireEqualOutput(t, []byte(view))
}

// Tests for BQL self-loading functionality

func TestColumn_NewColumnWithExecutor(t *testing.T) {
	// NewColumnWithExecutor should create a column with executor and query set
	// We pass nil executor for unit test (actual execution tested elsewhere)
	c := NewColumnWithExecutor("Ready", "status = open", nil)
	require.Equal(t, "Ready", c.title)
	require.Equal(t, "status = open", c.Query())
	require.Nil(t, c.executor)
}

func TestColumn_SetQuery(t *testing.T) {
	c := NewColumn("Test")
	c = c.SetQuery("status = open and ready = true")
	require.Equal(t, "status = open and ready = true", c.Query())
}

func TestColumn_SetExecutor(t *testing.T) {
	c := NewColumn("Test")
	// We can't easily create a real executor without a DB, so just test the setter
	c = c.SetExecutor(nil)
	require.Nil(t, c.executor)
}

func TestColumn_LoadError(t *testing.T) {
	c := NewColumn("Test")

	// Default should have no error
	require.Nil(t, c.LoadError())
}

func TestColumn_LoadIssues_NoExecutor(t *testing.T) {
	// Without executor, LoadIssues should be a no-op
	c := NewColumn("Test")
	c = c.SetQuery("status = open")

	// Should return unchanged column
	c2 := c.LoadIssues()
	require.Empty(t, c2.Items())
}

func TestColumn_LoadIssues_NoQuery(t *testing.T) {
	// Without query, LoadIssues should be a no-op
	c := NewColumn("Test")
	// Don't set query

	c2 := c.LoadIssues()
	require.Empty(t, c2.Items())
}

func TestColumn_LoadIssuesCmd_NoExecutor(t *testing.T) {
	c := NewColumn("Test")
	c = c.SetQuery("status = open")

	// Should return nil command
	cmd := c.LoadIssuesCmd()
	require.Nil(t, cmd)
}

func TestColumn_LoadIssuesCmd_NoQuery(t *testing.T) {
	c := NewColumn("Test")
	// Don't set query

	cmd := c.LoadIssuesCmd()
	require.Nil(t, cmd)
}

func TestColumnLoadedMsg_Structure(t *testing.T) {
	// Test that ColumnLoadedMsg can be constructed correctly
	issues := []task.Issue{{ID: "test-1", TitleText: "Test"}}
	msg := ColumnLoadedMsg{
		ColumnTitle: "Ready",
		Issues:      issues,
		Err:         nil,
	}

	require.Equal(t, "Ready", msg.ColumnTitle)
	require.Len(t, msg.Issues, 1)
	require.Nil(t, msg.Err)
}

func TestColumnLoadedMsg_WithError(t *testing.T) {
	msg := ColumnLoadedMsg{
		ColumnTitle: "Ready",
		Issues:      nil,
		Err:         errors.New("test error"),
	}

	require.Equal(t, "Ready", msg.ColumnTitle)
	require.Nil(t, msg.Issues)
	require.Error(t, msg.Err)
}

package search

import (
	"errors"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"
	"github.com/stretchr/testify/require"

	"github.com/zjrosen/perles/internal/mocks"
	"github.com/zjrosen/perles/internal/mode"
	"github.com/zjrosen/perles/internal/mode/shared"
	"github.com/zjrosen/perles/internal/task"
	"github.com/zjrosen/perles/internal/ui/details"
	"github.com/zjrosen/perles/internal/ui/tree"
)

// errTest is a sentinel error for testing
var errTest = errors.New("test error")

// testClockTime for deterministic timestamps in tests.
var testClockTime = time.Date(2025, 1, 15, 12, 0, 0, 0, time.UTC)

// testCreatedAt is 2 days before testClockTime for "2d ago" display
var testCreatedAt = time.Date(2025, 1, 13, 12, 0, 0, 0, time.UTC)

// newTestClock creates a MockClock that always returns testClockTime.
func newTestClock(t *testing.T) shared.Clock {
	clock := mocks.NewMockClock(t)
	clock.EXPECT().Now().Return(testClockTime).Maybe()
	return clock
}

// Golden tests for tree sub-mode rendering.
// Run with -update flag to update golden files: go test -update ./internal/mode/search/...

// createTestModelInTreeMode creates a model in tree sub-mode for testing.
func createTestModelInTreeMode(t *testing.T) Model {
	m := createTestModel(t)
	m.subMode = mode.SubModeTree
	m.focus = FocusResults
	return m
}

// buildIssueMap creates a map from issues slice for tree.New.
func buildIssueMap(issues []task.Issue) map[string]*task.Issue {
	m := make(map[string]*task.Issue)
	for i := range issues {
		m[issues[i].ID] = &issues[i]
	}
	return m
}

// createTestModelWithTree creates a model with a loaded tree.
func createTestModelWithTree(t *testing.T, rootIssue task.Issue, issues []task.Issue) Model {
	m := createTestModelInTreeMode(t)
	m.treeRoot = &rootIssue

	// Build tree
	issueMap := buildIssueMap(issues)
	m.tree = tree.New(rootIssue.ID, issueMap, tree.DirectionDown, tree.ModeDeps, newTestClock(t))
	return m
}

func TestSearch_TreeView_Golden_Loading(t *testing.T) {
	m := createTestModelInTreeMode(t)
	m = m.SetSize(160, 30)
	// No tree loaded yet - should show loading state
	m.treeRoot = &task.Issue{ID: "test-root"}

	view := m.View()
	teatest.RequireEqualOutput(t, []byte(view))
}

func TestSearch_TreeView_Golden_WithTree(t *testing.T) {
	rootIssue := task.Issue{
		ID:        "epic-1",
		TitleText: "Epic: Implement new feature",
		Type:      task.TypeEpic,
		Status:    task.StatusOpen,
		Priority:  1,
		Children:  []string{"task-1", "task-2", "task-3"},
	}
	issues := []task.Issue{
		rootIssue,
		{ID: "task-1", TitleText: "Design API", Type: task.TypeTask, Status: task.StatusClosed, Priority: 1, ParentID: "epic-1"},
		{ID: "task-2", TitleText: "Implement backend", Type: task.TypeTask, Status: task.StatusInProgress, Priority: 1, ParentID: "epic-1"},
		{ID: "task-3", TitleText: "Add tests", Type: task.TypeTask, Status: task.StatusOpen, Priority: 2, ParentID: "epic-1"},
	}

	m := createTestModelWithTree(t, rootIssue, issues)
	m = m.SetSize(160, 30)

	view := m.View()
	teatest.RequireEqualOutput(t, []byte(view))
}

func TestSearch_TreeView_Golden_DownDirection(t *testing.T) {
	rootIssue := task.Issue{
		ID:        "parent-1",
		TitleText: "Parent Issue",
		Type:      task.TypeTask,
		Status:    task.StatusOpen,
		Children:  []string{"child-1", "child-2"},
	}
	issues := []task.Issue{
		rootIssue,
		{ID: "child-1", TitleText: "Child A", Type: task.TypeTask, Status: task.StatusClosed, ParentID: "parent-1"},
		{ID: "child-2", TitleText: "Child B", Type: task.TypeTask, Status: task.StatusOpen, ParentID: "parent-1"},
	}

	m := createTestModelWithTree(t, rootIssue, issues)
	issueMap := buildIssueMap(issues)
	m.tree = tree.New(rootIssue.ID, issueMap, tree.DirectionDown, tree.ModeDeps, newTestClock(t))
	m = m.SetSize(160, 30)

	view := m.View()
	teatest.RequireEqualOutput(t, []byte(view))
}

func TestSearch_TreeView_Golden_UpDirection(t *testing.T) {
	rootIssue := task.Issue{
		ID:        "child-1",
		TitleText: "Child Issue",
		Type:      task.TypeTask,
		Status:    task.StatusOpen,
		ParentID:  "parent-1",
	}
	parentIssue := task.Issue{
		ID:        "parent-1",
		TitleText: "Parent Issue",
		Type:      task.TypeEpic,
		Status:    task.StatusOpen,
		Children:  []string{"child-1"},
	}
	issues := []task.Issue{parentIssue, rootIssue}

	m := createTestModelWithTree(t, rootIssue, issues)
	issueMap := buildIssueMap(issues)
	m.tree = tree.New(rootIssue.ID, issueMap, tree.DirectionUp, tree.ModeDeps, newTestClock(t))
	m = m.SetSize(160, 30)

	view := m.View()
	teatest.RequireEqualOutput(t, []byte(view))
}

func TestSearch_TreeView_Golden_ChildrenMode(t *testing.T) {
	// Create an epic with children and also a dependency
	rootIssue := task.Issue{
		ID:        "epic-1",
		TitleText: "Epic: Implement new feature",
		Type:      task.TypeEpic,
		Priority:  1,
		Status:    task.StatusOpen,
		Children:  []string{"task-1", "task-2"},
		Blocks:    []string{"task-3"}, // This should NOT appear in children mode
	}
	issues := []task.Issue{
		rootIssue,
		{ID: "task-1", TitleText: "Design API", Type: task.TypeTask, Priority: 1, Status: task.StatusClosed, ParentID: "epic-1"},
		{ID: "task-2", TitleText: "Implement backend", Type: task.TypeTask, Priority: 1, Status: task.StatusInProgress, ParentID: "epic-1"},
		{ID: "task-3", TitleText: "Blocked task (dependency)", Type: task.TypeTask, Priority: 2, Status: task.StatusOpen}, // No ParentID - pure dependency
	}

	m := createTestModelWithTree(t, rootIssue, issues)
	issueMap := buildIssueMap(issues)
	// Use children mode - should only show task-1 and task-2, NOT task-3
	m.tree = tree.New(rootIssue.ID, issueMap, tree.DirectionDown, tree.ModeChildren, newTestClock(t))
	m = m.SetSize(160, 30)

	view := m.View()
	teatest.RequireEqualOutput(t, []byte(view))
}

// TestSearch_TreeView_Golden_WithLongAssignee tests tree view with a long assignee that might wrap.
func TestSearch_TreeView_Golden_WithLongAssignee(t *testing.T) {
	rootIssue := task.Issue{
		ID:              "epic-1",
		TitleText:       "Epic: Implement new feature",
		DescriptionText: "This is a test issue with a long assignee.",
		Type:            task.TypeEpic,
		Status:          task.StatusOpen,
		Priority:        1,
		Assignee:        "this/is/a/verylong/assignee/name/that/will/wrap",
		Children:        []string{"task-1", "task-2"},
		CreatedAt:       testCreatedAt,
		UpdatedAt:       testCreatedAt,
	}
	issues := []task.Issue{
		rootIssue,
		{ID: "task-1", TitleText: "Design API", Type: task.TypeTask, Status: task.StatusClosed, Priority: 1, ParentID: "epic-1", CreatedAt: testCreatedAt},
		{ID: "task-2", TitleText: "Implement backend", Type: task.TypeTask, Status: task.StatusInProgress, Priority: 1, ParentID: "epic-1", CreatedAt: testCreatedAt},
	}

	m := createTestModelWithTree(t, rootIssue, issues)
	// Use 220 width to ensure right panel gets >= 100 chars for two-column layout in details
	// rightWidth = 220 - 110 - 1 = 109, but details gets rightWidth-2 = 107 for border
	m = m.SetSize(220, 30)
	// Set up details panel with the root issue
	// Width 107 (109-2 for border) is above minTwoColumnWidth (100) so two-column layout is used
	m.details = details.New(rootIssue, m.services.QueryExecutor, nil, m.services.TaskExecutor).SetSize(107, 28)
	m.hasDetail = true
	m.focus = FocusDetails

	view := m.View()
	teatest.RequireEqualOutput(t, []byte(view))
}

// Unit tests for renderCompactProgress
func TestRenderCompactProgress(t *testing.T) {
	tests := []struct {
		name           string
		closed         int
		total          int
		expectedSuffix string // The percentage and count suffix
		expectedEmpty  bool   // Whether result should be empty
	}{
		{"EmptyTotal", 0, 0, "", true},
		{"NoneClosedOf5", 0, 5, " 0% (0/5)", false},
		{"OneOfFive", 1, 5, " 20% (1/5)", false},
		{"HalfClosedOf4", 2, 4, " 50% (2/4)", false},
		{"ThreeOfFour", 3, 4, " 75% (3/4)", false},
		{"AllClosedOf5", 5, 5, " 100% (5/5)", false},
		{"OneOfTwo", 1, 2, " 50% (1/2)", false},
		{"AllClosedOf1", 1, 1, " 100% (1/1)", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := renderCompactProgress(tc.closed, tc.total)
			if tc.expectedEmpty {
				require.Empty(t, result)
			} else {
				// Check suffix (percentage and counts)
				require.Contains(t, result, tc.expectedSuffix)
				// Check that progress bar characters are present (either filled or empty)
				hasBar := strings.Contains(result, "█") || strings.Contains(result, "░")
				require.True(t, hasBar, "progress bar should contain bar characters")
			}
		})
	}
}

func TestSearch_TreeSubMode_Initialization(t *testing.T) {
	m := createTestModel(t)

	// Enter tree sub-mode via EnterMsg
	m, _ = m.Update(EnterMsg{SubMode: mode.SubModeTree, IssueID: "test-123"})

	require.Equal(t, mode.SubModeTree, m.subMode)
	require.Equal(t, FocusResults, m.focus, "should focus tree panel when entering tree mode from kanban")
	require.NotNil(t, m.treeRoot)
	require.Equal(t, "test-123", m.treeRoot.ID)
}

func TestSearch_TreeSubMode_EnterListClearsTreeState(t *testing.T) {
	rootIssue := task.Issue{ID: "root", TitleText: "Root"}
	m := createTestModelWithTree(t, rootIssue, []task.Issue{rootIssue})

	// Verify tree state is set
	require.Equal(t, mode.SubModeTree, m.subMode)
	require.NotNil(t, m.tree)
	require.NotNil(t, m.treeRoot)

	// EnterMsg with list mode should clear tree state
	m, _ = m.Update(EnterMsg{SubMode: mode.SubModeList, Query: "status = open"})

	require.Equal(t, mode.SubModeList, m.subMode)
	require.Equal(t, FocusSearch, m.focus)
	require.Nil(t, m.tree)
	require.Nil(t, m.treeRoot)
}

func TestSearch_EnterTreeMode_ClearsOldTreeState(t *testing.T) {
	// Bug scenario: User views Task A in tree mode, returns to kanban,
	// then opens Epic B. Without clearing m.tree, handleTreeLoaded()
	// would restore selection to Task A (if it's a child of Epic B).

	rootIssue := task.Issue{ID: "task-1", TitleText: "Task A"}
	m := createTestModelWithTree(t, rootIssue, []task.Issue{rootIssue})

	// Verify tree state exists (simulating previous tree session)
	require.NotNil(t, m.tree, "precondition: tree should be set")
	require.Equal(t, "task-1", m.treeRoot.ID)

	// User enters tree mode for a DIFFERENT issue (Epic B)
	m, _ = m.Update(EnterMsg{SubMode: mode.SubModeTree, IssueID: "epic-1"})

	// m.tree should be nil to prevent handleTreeLoaded from restoring stale selection
	require.Nil(t, m.tree, "EnterMsg should clear tree state")
	require.Equal(t, "epic-1", m.treeRoot.ID, "treeRoot should be set to new issue")
	require.Equal(t, mode.SubModeTree, m.subMode, "should remain in tree mode")
	require.Equal(t, FocusResults, m.focus, "focus should be on results")
}

// Key handling tests for tree sub-mode

// createTreeTestModel creates a model in tree sub-mode with multiple children for key testing.
func createTreeTestModel(t *testing.T) Model {
	rootIssue := task.Issue{
		ID:        "root-1",
		TitleText: "Root Issue",
		Type:      task.TypeEpic,
		Status:    task.StatusOpen,
		Children:  []string{"child-1", "child-2", "child-3"},
	}
	issues := []task.Issue{
		rootIssue,
		{ID: "child-1", TitleText: "First Child", Type: task.TypeTask, Status: task.StatusClosed, ParentID: "root-1"},
		{ID: "child-2", TitleText: "Second Child", Type: task.TypeTask, Status: task.StatusInProgress, ParentID: "root-1"},
		{ID: "child-3", TitleText: "Third Child", Type: task.TypeTask, Status: task.StatusOpen, ParentID: "root-1"},
	}

	m := createTestModelWithTree(t, rootIssue, issues)
	m = m.SetSize(100, 30)
	return m
}

func TestTreeSubMode_JKey_MovesCursorDown(t *testing.T) {
	m := createTreeTestModel(t)
	initialID := m.tree.SelectedNode().Issue.ID

	// Press j to move down
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})

	// Selected node should have changed (moved to first child)
	require.NotEqual(t, initialID, m.tree.SelectedNode().Issue.ID, "j should move cursor down")
}

func TestTreeSubMode_KKey_MovesCursorUp(t *testing.T) {
	m := createTreeTestModel(t)

	// First move down, then test k
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	idAfterJ := m.tree.SelectedNode().Issue.ID

	// Press k to move up
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})

	// Should be back at root
	require.NotEqual(t, idAfterJ, m.tree.SelectedNode().Issue.ID, "k should move cursor up")
	require.Equal(t, "root-1", m.tree.SelectedNode().Issue.ID, "should be back at root")
}

func TestTreeSubMode_DownArrow_MovesCursorDown(t *testing.T) {
	m := createTreeTestModel(t)
	initialID := m.tree.SelectedNode().Issue.ID

	// Press down arrow
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})

	require.NotEqual(t, initialID, m.tree.SelectedNode().Issue.ID, "down arrow should move cursor down")
}

func TestTreeSubMode_UpArrow_MovesCursorUp(t *testing.T) {
	m := createTreeTestModel(t)

	// First move down
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})

	// Press up arrow
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})

	require.Equal(t, "root-1", m.tree.SelectedNode().Issue.ID, "up arrow should move cursor up to root")
}

func TestTreeSubMode_SlashKey_SwitchesToListMode(t *testing.T) {
	m := createTreeTestModel(t)

	// Press / to switch to list mode
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})

	require.Equal(t, mode.SubModeList, m.subMode, "/ should switch to list sub-mode")
	require.Equal(t, FocusSearch, m.focus, "/ should focus search input")
}

func TestTreeSubMode_DKey_TogglesDirection(t *testing.T) {
	m := createTreeTestModel(t)
	initialDirection := m.tree.Direction()

	// Press d to toggle direction
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})

	// Direction should have changed
	require.NotEqual(t, initialDirection, m.tree.Direction(), "d should toggle direction")
}

func TestTreeSubMode_MKey_TogglesMode(t *testing.T) {
	m := createTreeTestModel(t)
	initialMode := m.tree.Mode()
	require.Equal(t, tree.ModeDeps, initialMode, "should start in deps mode")

	// Press m to toggle mode
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}})

	// Mode should have changed to children
	require.Equal(t, tree.ModeChildren, m.tree.Mode(), "m should toggle to children mode")

	// Press m again to toggle back
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}})

	// Mode should have changed back to deps
	require.Equal(t, tree.ModeDeps, m.tree.Mode(), "m should toggle back to deps mode")
}

func TestTreeSubMode_LKey_FocusesDetails(t *testing.T) {
	m := createTreeTestModel(t)
	require.Equal(t, FocusResults, m.focus, "should start with focus on results")

	// Press l to move focus to details
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})

	require.Equal(t, FocusDetails, m.focus, "l should move focus to details")
}

func TestTreeSubMode_RightArrow_FocusesDetails(t *testing.T) {
	m := createTreeTestModel(t)

	// Press right arrow
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})

	require.Equal(t, FocusDetails, m.focus, "right arrow should move focus to details")
}

func TestTreeSubMode_TabKey_FocusesDetails(t *testing.T) {
	m := createTreeTestModel(t)

	// Press tab
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})

	require.Equal(t, FocusDetails, m.focus, "tab should move focus to details")
}

func TestTreeSubMode_TabKey_CyclesBetweenTreeAndDetails(t *testing.T) {
	m := createTreeTestModel(t)

	// Start on tree (FocusResults)
	require.Equal(t, FocusResults, m.focus, "should start on tree")

	// Tab to details
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	require.Equal(t, FocusDetails, m.focus, "tab should move focus to details")

	// Tab back to tree (not search input, since tree mode has no search input)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	require.Equal(t, FocusResults, m.focus, "tab should cycle back to tree, not search input")
}

func TestTreeSubMode_EscKey_ReturnsToKanban(t *testing.T) {
	m := createTreeTestModel(t)

	// Press esc - should return command that sends ExitToKanbanMsg
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})

	// Execute the command to get the message
	require.NotNil(t, cmd, "esc should return a command")
	msg := cmd()
	_, ok := msg.(ExitToKanbanMsg)
	require.True(t, ok, "esc should return ExitToKanbanMsg")
}

func TestTreeSubMode_HelpKey_ShowsHelp(t *testing.T) {
	m := createTreeTestModel(t)

	// Press ? to show help
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})

	require.Equal(t, ViewHelp, m.view, "? should switch to help view")
}

func TestTreeSubMode_CtrlC_ReturnsRequestQuitMsg(t *testing.T) {
	m := createTreeTestModel(t)

	// Press ctrl+c - should return mode.RequestQuitMsg
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})

	require.NotNil(t, cmd, "expected quit request command")
	result := cmd()
	_, isRequestQuit := result.(mode.RequestQuitMsg)
	require.True(t, isRequestQuit, "ctrl+c should return mode.RequestQuitMsg")
}

func TestTreeSubMode_NotFocused_KeysPassThrough(t *testing.T) {
	m := createTreeTestModel(t)
	m.focus = FocusDetails // Not focused on tree

	initialID := m.tree.SelectedNode().Issue.ID

	// Press j while not focused on tree
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})

	// Cursor should not move since tree isn't focused
	require.Equal(t, initialID, m.tree.SelectedNode().Issue.ID, "j should not move cursor when tree not focused")
}

func TestTreeSubMode_EnterKey_RefocusesTree(t *testing.T) {
	m := createTreeTestModel(t)
	originalRootID := m.tree.Root().Issue.ID

	// Move cursor to first child
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	selectedBeforeEnter := m.tree.SelectedNode().Issue.ID
	require.NotEqual(t, originalRootID, selectedBeforeEnter, "should have moved to child")

	// Press Enter to refocus tree on selected node
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	// The tree should now be focused on the child
	// After refocus, the root becomes the previously selected node
	newRootID := m.tree.Root().Issue.ID
	require.Equal(t, selectedBeforeEnter, newRootID, "enter should refocus tree on selected node")
	// treeRoot should also be updated
	require.Equal(t, selectedBeforeEnter, m.treeRoot.ID, "treeRoot should match new root")
}

func TestTreeSubMode_UKey_GoesBack(t *testing.T) {
	m := createTreeTestModel(t)
	originalRootID := m.tree.Root().Issue.ID

	// First refocus to a child (Enter on child)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	newRootID := m.tree.Root().Issue.ID
	require.NotEqual(t, originalRootID, newRootID, "should have refocused to child")

	// Press u to go back
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'u'}})

	// Should be back at original root
	require.Equal(t, originalRootID, m.tree.Root().Issue.ID, "u should go back to previous root")
	require.Equal(t, originalRootID, m.treeRoot.ID, "treeRoot should be updated")
}

func TestTreeSubMode_UCapitalKey_GoesToOriginal(t *testing.T) {
	m := createTreeTestModel(t)
	originalRootID := m.tree.Root().Issue.ID

	// First refocus to a child
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	// Refocus again to grandchild (simulate deep navigation)
	// Note: In our test data, children don't have grandchildren,
	// so we just verify that U returns to original after one level
	currentRootID := m.tree.Root().Issue.ID
	require.NotEqual(t, originalRootID, currentRootID, "should be at child level")

	// Press U to go to original root
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'U'}})

	// Should be back at original root
	require.Equal(t, originalRootID, m.tree.Root().Issue.ID, "U should go to original root")
	require.Equal(t, originalRootID, m.treeRoot.ID, "treeRoot should be updated")
}

// Tests for handleIssueDeleted tree-aware deletion handling

func TestHandleIssueDeleted_TreeMode_NonRootDeletion(t *testing.T) {
	// Setup: Tree mode with a root and children
	m := createTreeTestModel(t)
	require.Equal(t, mode.SubModeTree, m.subMode)
	require.NotNil(t, m.treeRoot)
	rootID := m.treeRoot.ID

	// Delete a non-root issue (wasTreeRoot=false)
	msg := issueDeletedMsg{
		issueID:     "child-1",
		parentID:    "root-1",
		wasTreeRoot: false,
		err:         nil,
	}
	m, cmd := m.handleIssueDeleted(msg)

	// Verify state after deletion
	require.Equal(t, ViewSearch, m.view, "view should return to search")
	require.Nil(t, m.selectedIssue, "selectedIssue should be cleared")

	// Verify command is returned (loadTree with same root + toast)
	require.NotNil(t, cmd, "should return command")

	// The tree root should still be the same (refreshed with same root)
	require.Equal(t, rootID, m.treeRoot.ID, "treeRoot should remain unchanged for non-root deletion")
}

func TestHandleIssueDeleted_TreeMode_RootDeletionWithParent(t *testing.T) {
	// Setup: Tree mode where root has a parent
	rootIssue := task.Issue{
		ID:        "child-root",
		TitleText: "Child as Root",
		Type:      task.TypeTask,
		Status:    task.StatusOpen,
		ParentID:  "parent-1", // Has a parent
	}
	issues := []task.Issue{rootIssue}

	m := createTestModelWithTree(t, rootIssue, issues)
	require.Equal(t, mode.SubModeTree, m.subMode)

	// Delete the root issue (wasTreeRoot=true, has parent)
	msg := issueDeletedMsg{
		issueID:     "child-root",
		parentID:    "parent-1",
		wasTreeRoot: true,
		err:         nil,
	}
	m, cmd := m.handleIssueDeleted(msg)

	// Verify state
	require.Equal(t, ViewSearch, m.view, "view should return to search")
	require.Nil(t, m.selectedIssue, "selectedIssue should be cleared")

	// Should return loadTree command with parentID as new root
	require.NotNil(t, cmd, "should return command for re-rooting to parent")
}

func TestHandleIssueDeleted_TreeMode_RootDeletionWithoutParent(t *testing.T) {
	// Setup: Tree mode where root has no parent (orphan root)
	rootIssue := task.Issue{
		ID:        "orphan-root",
		TitleText: "Orphan Root",
		Type:      task.TypeTask,
		Status:    task.StatusOpen,
		ParentID:  "", // No parent
	}
	issues := []task.Issue{rootIssue}

	m := createTestModelWithTree(t, rootIssue, issues)
	require.Equal(t, mode.SubModeTree, m.subMode)

	// Delete the root issue (wasTreeRoot=true, no parent)
	msg := issueDeletedMsg{
		issueID:     "orphan-root",
		parentID:    "",
		wasTreeRoot: true,
		err:         nil,
	}
	m, cmd := m.handleIssueDeleted(msg)

	// Verify state
	require.Equal(t, ViewSearch, m.view, "view should return to search")
	require.Nil(t, m.selectedIssue, "selectedIssue should be cleared")

	// Should return ExitToKanbanMsg command
	require.NotNil(t, cmd, "should return command")

	// Execute the batch command - one of them should be ExitToKanbanMsg
	// Note: tea.Batch returns multiple commands, we check that it exists
}

func TestHandleIssueDeleted_ListMode_Deletion(t *testing.T) {
	// Setup: List mode (default)
	m := createTestModelWithResults(t)
	require.Equal(t, mode.SubModeList, m.subMode)

	// Delete an issue in list mode
	msg := issueDeletedMsg{
		issueID:     "test-1",
		parentID:    "",
		wasTreeRoot: false,
		err:         nil,
	}
	m, cmd := m.handleIssueDeleted(msg)

	// Verify state
	require.Equal(t, ViewSearch, m.view, "view should return to search")
	require.Nil(t, m.selectedIssue, "selectedIssue should be cleared")

	// Should return executeSearch command + toast
	require.NotNil(t, cmd, "should return command for list refresh")
}

func TestHandleIssueDeleted_Error(t *testing.T) {
	// Setup: Any mode
	m := createTreeTestModel(t)

	// Delete fails with error
	msg := issueDeletedMsg{
		issueID:     "any-issue",
		parentID:    "",
		wasTreeRoot: false,
		err:         errTest,
	}
	m, cmd := m.handleIssueDeleted(msg)

	// Verify state
	require.Equal(t, ViewSearch, m.view, "view should return to search")
	require.Nil(t, m.selectedIssue, "selectedIssue should be cleared")

	// Should return error toast command
	require.NotNil(t, cmd, "should return command for error toast")
}

// =============================================================================
// Edit Key ('ctrl+e') Tests - Tree Sub-Mode
// =============================================================================

func TestTreeSubMode_EditKey_EmitsOpenEditMenuMsg(t *testing.T) {
	m := createTreeTestModel(t)
	require.Equal(t, mode.SubModeTree, m.subMode, "should be in tree sub-mode")
	require.Equal(t, FocusResults, m.focus, "should be focused on tree/results")

	// Get the currently selected node's issue
	selectedNode := m.tree.SelectedNode()
	require.NotNil(t, selectedNode, "should have a selected node")
	selectedIssue := &selectedNode.Issue

	// Press 'ctrl+e' while focused on tree results
	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlE})

	// Should return a command that emits OpenEditMenuMsg
	require.NotNil(t, cmd, "expected a command to be returned")

	// Execute the command to get the message
	msg := cmd()
	editMsg, ok := msg.(details.OpenEditMenuMsg)
	require.True(t, ok, "expected OpenEditMenuMsg, got %T", msg)

	// Verify the message contains correct issue data from tree selection
	require.Equal(t, selectedIssue.ID, editMsg.Issue.ID, "issue ID should match selected tree node")
	require.Equal(t, selectedIssue.Labels, editMsg.Issue.Labels, "labels should match")
	require.Equal(t, selectedIssue.Priority, editMsg.Issue.Priority, "priority should match")
	require.Equal(t, selectedIssue.Status, editMsg.Issue.Status, "status should match")
}

func TestTreeSubMode_EditKey_NavigatedChild_EmitsCorrectIssue(t *testing.T) {
	m := createTreeTestModel(t)

	// Navigate to first child
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})

	// Verify we moved to a child
	selectedNode := m.tree.SelectedNode()
	require.NotNil(t, selectedNode, "should have a selected node")
	require.Equal(t, "child-1", selectedNode.Issue.ID, "should have navigated to child-1")

	// Press 'ctrl+e' to edit the child
	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlE})

	require.NotNil(t, cmd, "expected a command to be returned")

	// Execute the command to get the message
	msg := cmd()
	editMsg, ok := msg.(details.OpenEditMenuMsg)
	require.True(t, ok, "expected OpenEditMenuMsg, got %T", msg)

	// Should edit the child, not the root
	require.Equal(t, "child-1", editMsg.Issue.ID, "should edit the navigated-to child")
}

func TestTreeSubMode_EditKey_TreeLoading_NoOp(t *testing.T) {
	// Create a model in tree sub-mode but with no tree loaded yet (loading state)
	m := createTestModelInTreeMode(t)
	m.focus = FocusResults
	m.treeRoot = &task.Issue{ID: "loading-root"}
	// m.tree is nil (loading state)
	require.Nil(t, m.tree, "precondition: tree should be nil (loading)")

	// Press 'ctrl+e' during loading state
	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlE})

	// Should be a no-op since getSelectedIssue returns nil when tree is nil
	require.Nil(t, cmd, "expected no command when tree is loading")
}

func TestTreeSubMode_DeleteKey_TreeLoading_NoOp(t *testing.T) {
	// Create a model in tree sub-mode but with no tree loaded yet (loading state)
	// Note: In tree sub-mode, 'd' normally toggles direction, but during loading
	// (m.tree == nil) we can't toggle direction either, so it falls through to
	// the general 'd' handler which checks focus and selected issue.
	m := createTestModelInTreeMode(t)
	m.focus = FocusResults
	m.treeRoot = &task.Issue{ID: "loading-root"}
	// m.tree is nil (loading state)
	require.Nil(t, m.tree, "precondition: tree should be nil (loading)")

	// Press 'd' during loading state
	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})

	// Should be a no-op since getSelectedIssue returns nil when tree is nil
	// And tree direction toggle also requires m.tree != nil
	require.Nil(t, cmd, "expected no command when tree is loading")
}

// =============================================================================
// Delete Key ('d') Tests - Tree Sub-Mode (Regression Tests)
// =============================================================================

func TestTreeSubMode_DeleteKey_TogglesDirection_NotDelete(t *testing.T) {
	// This is a regression test to verify that in tree sub-mode, 'd' key
	// toggles direction rather than emitting DeleteIssueMsg
	m := createTreeTestModel(t)
	require.Equal(t, mode.SubModeTree, m.subMode, "should be in tree sub-mode")
	require.Equal(t, FocusResults, m.focus, "should be focused on tree/results")

	// Capture initial direction
	initialDirection := m.tree.Direction()

	// Get the selected issue for verification
	selectedNode := m.tree.SelectedNode()
	require.NotNil(t, selectedNode, "should have a selected node")

	// Press 'd' while focused on tree results
	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})

	// Direction should have changed (toggle happened)
	require.NotEqual(t, initialDirection, m.tree.Direction(), "d should toggle direction in tree sub-mode")

	// Should NOT emit DeleteIssueMsg
	if cmd != nil {
		msg := cmd()
		_, isDeleteMsg := msg.(details.DeleteIssueMsg)
		require.False(t, isDeleteMsg, "should NOT emit DeleteIssueMsg in tree sub-mode (d toggles direction)")
	}
}

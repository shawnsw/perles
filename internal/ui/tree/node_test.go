package tree

import (
	"testing"

	"github.com/zjrosen/perles/internal/task"

	"github.com/stretchr/testify/require"
)

// bi is a shorthand for task.BuildIssuePtr to keep test issue map literals concise.
var bi = task.BuildIssuePtr

func TestBuildTree_Down_Basic(t *testing.T) {
	// Epic -> Task1, Task2
	issueMap := map[string]*task.Issue{
		"epic-1": bi("epic-1", task.WithChildren("task-1", "task-2")),
		"task-1": bi("task-1", task.WithStatus(task.StatusClosed), task.WithParentID("epic-1")),
		"task-2": bi("task-2", task.WithParentID("epic-1")),
	}

	root, err := BuildTree(issueMap, "epic-1", DirectionDown, ModeDeps)
	require.NoError(t, err)
	require.NotNil(t, root)
	require.Equal(t, "epic-1", root.Issue.ID)
	require.Len(t, root.Children, 2)
	require.Equal(t, 0, root.Depth)
	require.Nil(t, root.Parent)

	// Check children
	require.Equal(t, "task-1", root.Children[0].Issue.ID)
	require.Equal(t, 1, root.Children[0].Depth)
	require.Equal(t, root, root.Children[0].Parent)

	require.Equal(t, "task-2", root.Children[1].Issue.ID)
	require.Equal(t, 1, root.Children[1].Depth)
}

func TestBuildTree_Down_WithBlocks(t *testing.T) {
	// Task blocks another task
	issueMap := map[string]*task.Issue{
		"task-1": bi("task-1", task.WithStatus(task.StatusClosed), task.WithBlocks("task-2")),
		"task-2": bi("task-2", task.WithBlockedBy("task-1")),
	}

	root, err := BuildTree(issueMap, "task-1", DirectionDown, ModeDeps)
	require.NoError(t, err)
	require.Len(t, root.Children, 1)
	require.Equal(t, "task-2", root.Children[0].Issue.ID)
}

func TestBuildTree_Down_Combined(t *testing.T) {
	// Epic with children AND blocks
	issueMap := map[string]*task.Issue{
		"epic-1":     bi("epic-1", task.WithChildren("task-1"), task.WithBlocks("external-1")),
		"task-1":     bi("task-1", task.WithParentID("epic-1")),
		"external-1": bi("external-1", task.WithBlockedBy("epic-1")),
	}

	root, err := BuildTree(issueMap, "epic-1", DirectionDown, ModeDeps)
	require.NoError(t, err)
	require.Len(t, root.Children, 2) // Both child and blocked issue
	ids := []string{root.Children[0].Issue.ID, root.Children[1].Issue.ID}
	require.Contains(t, ids, "task-1")
	require.Contains(t, ids, "external-1")
}

func TestBuildTree_Up_Basic(t *testing.T) {
	// Task -> Parent epic
	issueMap := map[string]*task.Issue{
		"epic-1": bi("epic-1", task.WithChildren("task-1")),
		"task-1": bi("task-1", task.WithParentID("epic-1")),
	}

	root, err := BuildTree(issueMap, "task-1", DirectionUp, ModeDeps)
	require.NoError(t, err)
	require.Equal(t, "task-1", root.Issue.ID)
	require.Len(t, root.Children, 1)
	require.Equal(t, "epic-1", root.Children[0].Issue.ID)
}

func TestBuildTree_Up_WithBlockedBy(t *testing.T) {
	// Task blocked by another task
	issueMap := map[string]*task.Issue{
		"task-1":    bi("task-1", task.WithBlockedBy("blocker-1")),
		"blocker-1": bi("blocker-1", task.WithStatus(task.StatusClosed), task.WithBlocks("task-1")),
	}

	root, err := BuildTree(issueMap, "task-1", DirectionUp, ModeDeps)
	require.NoError(t, err)
	require.Len(t, root.Children, 1)
	require.Equal(t, "blocker-1", root.Children[0].Issue.ID)
}

func TestBuildTree_Up_Combined(t *testing.T) {
	// Task with parent AND blockedBy
	issueMap := map[string]*task.Issue{
		"epic-1":    bi("epic-1", task.WithChildren("task-1")),
		"task-1":    bi("task-1", task.WithBlockedBy("blocker-1"), task.WithParentID("epic-1")),
		"blocker-1": bi("blocker-1", task.WithStatus(task.StatusClosed), task.WithBlocks("task-1")),
	}

	root, err := BuildTree(issueMap, "task-1", DirectionUp, ModeDeps)
	require.NoError(t, err)
	require.Len(t, root.Children, 2) // Both parent and blocker
	ids := []string{root.Children[0].Issue.ID, root.Children[1].Issue.ID}
	require.Contains(t, ids, "epic-1")
	require.Contains(t, ids, "blocker-1")
}

func TestBuildTree_MissingRoot(t *testing.T) {
	issueMap := map[string]*task.Issue{
		"task-1": bi("task-1"),
	}

	_, err := BuildTree(issueMap, "nonexistent", DirectionDown, ModeDeps)
	require.Error(t, err)
	require.Contains(t, err.Error(), "root issue nonexistent not found")
}

func TestBuildTree_CycleDetection(t *testing.T) {
	// Create a cycle: A -> B -> C -> A
	issueMap := map[string]*task.Issue{
		"a": bi("a", task.WithChildren("b")),
		"b": bi("b", task.WithChildren("c"), task.WithParentID("a")),
		"c": bi("c", task.WithChildren("a"), task.WithParentID("b")), // Cycle back to A
	}

	// Should not infinite loop - cycle detection should prevent
	root, err := BuildTree(issueMap, "a", DirectionDown, ModeDeps)
	require.NoError(t, err)
	require.NotNil(t, root)

	// Tree should have a -> b -> c, but c should NOT have a as child (cycle prevented)
	require.Len(t, root.Children, 1) // b
	require.Equal(t, "b", root.Children[0].Issue.ID)
	require.Len(t, root.Children[0].Children, 1) // c
	require.Equal(t, "c", root.Children[0].Children[0].Issue.ID)
	require.Empty(t, root.Children[0].Children[0].Children) // No children - cycle stopped
}

func TestBuildTree_MissingRelated(t *testing.T) {
	// Issue references a child not in the map (shouldn't crash)
	issueMap := map[string]*task.Issue{
		"epic-1": bi("epic-1", task.WithChildren("missing-task")),
	}

	root, err := BuildTree(issueMap, "epic-1", DirectionDown, ModeDeps)
	require.NoError(t, err)
	require.Empty(t, root.Children) // Missing reference skipped
}

func TestBuildTree_DeepTree(t *testing.T) {
	// Create 5-level deep tree
	issueMap := map[string]*task.Issue{
		"l0": bi("l0", task.WithChildren("l1")),
		"l1": bi("l1", task.WithChildren("l2"), task.WithParentID("l0")),
		"l2": bi("l2", task.WithChildren("l3"), task.WithParentID("l1")),
		"l3": bi("l3", task.WithChildren("l4"), task.WithParentID("l2")),
		"l4": bi("l4", task.WithParentID("l3")),
	}

	root, err := BuildTree(issueMap, "l0", DirectionDown, ModeDeps)
	require.NoError(t, err)

	// Traverse and verify depths
	node := root
	for depth := 0; depth <= 4; depth++ {
		require.Equal(t, depth, node.Depth)
		if depth < 4 {
			require.Len(t, node.Children, 1)
			node = node.Children[0]
		}
	}
}

func TestFlatten(t *testing.T) {
	issueMap := map[string]*task.Issue{
		"root": bi("root", task.WithChildren("a", "b")),
		"a":    bi("a", task.WithChildren("a1"), task.WithParentID("root")),
		"b":    bi("b", task.WithParentID("root")),
		"a1":   bi("a1", task.WithParentID("a")),
	}

	root, _ := BuildTree(issueMap, "root", DirectionDown, ModeDeps)
	flat := root.Flatten()

	require.Len(t, flat, 4)
	require.Equal(t, "root", flat[0].Issue.ID)
	require.Equal(t, "a", flat[1].Issue.ID)
	require.Equal(t, "a1", flat[2].Issue.ID)
	require.Equal(t, "b", flat[3].Issue.ID)
}

func TestCalculateProgress_AllOpen(t *testing.T) {
	issueMap := map[string]*task.Issue{
		"root": bi("root", task.WithChildren("a", "b")),
		"a":    bi("a", task.WithParentID("root")),
		"b":    bi("b", task.WithParentID("root")),
	}

	root, _ := BuildTree(issueMap, "root", DirectionDown, ModeDeps)
	closed, total := root.CalculateProgress()

	require.Equal(t, 0, closed)
	require.Equal(t, 3, total)
}

func TestCalculateProgress_AllClosed(t *testing.T) {
	issueMap := map[string]*task.Issue{
		"root": bi("root", task.WithStatus(task.StatusClosed), task.WithChildren("a", "b")),
		"a":    bi("a", task.WithStatus(task.StatusClosed), task.WithParentID("root")),
		"b":    bi("b", task.WithStatus(task.StatusClosed), task.WithParentID("root")),
	}

	root, _ := BuildTree(issueMap, "root", DirectionDown, ModeDeps)
	closed, total := root.CalculateProgress()

	require.Equal(t, 3, closed)
	require.Equal(t, 3, total)
}

func TestCalculateProgress_Mixed(t *testing.T) {
	issueMap := map[string]*task.Issue{
		"root": bi("root", task.WithChildren("a", "b", "c")),
		"a":    bi("a", task.WithStatus(task.StatusClosed), task.WithParentID("root")),
		"b":    bi("b", task.WithParentID("root")),
		"c":    bi("c", task.WithStatus(task.StatusClosed), task.WithParentID("root")),
	}

	root, _ := BuildTree(issueMap, "root", DirectionDown, ModeDeps)
	closed, total := root.CalculateProgress()

	require.Equal(t, 2, closed)
	require.Equal(t, 4, total)
}

func TestCalculateProgress_Nested(t *testing.T) {
	issueMap := map[string]*task.Issue{
		"root": bi("root", task.WithChildren("a")),
		"a":    bi("a", task.WithStatus(task.StatusClosed), task.WithChildren("b"), task.WithParentID("root")),
		"b":    bi("b", task.WithStatus(task.StatusClosed), task.WithParentID("a")),
	}

	root, _ := BuildTree(issueMap, "root", DirectionDown, ModeDeps)
	closed, total := root.CalculateProgress()

	require.Equal(t, 2, closed)
	require.Equal(t, 3, total)
}

func TestDirection_String(t *testing.T) {
	require.Equal(t, "down", DirectionDown.String())
	require.Equal(t, "up", DirectionUp.String())
}

func TestBuildTree_Down_SiblingBlocking(t *testing.T) {
	// Epic with two task children where one blocks the other.
	// task-impl blocks task-tests (tests must wait for impl).
	// In the tree, task-impl should appear first and task-tests
	// should be nested under it (not a sibling).
	issueMap := map[string]*task.Issue{
		"epic-1":     bi("epic-1", task.WithChildren("task-tests", "task-impl")),
		"task-impl":  bi("task-impl", task.WithBlocks("task-tests"), task.WithParentID("epic-1")),
		"task-tests": bi("task-tests", task.WithBlockedBy("task-impl"), task.WithParentID("epic-1")),
	}

	root, err := BuildTree(issueMap, "epic-1", DirectionDown, ModeDeps)
	require.NoError(t, err)
	require.Equal(t, "epic-1", root.Issue.ID)

	// The epic should have only ONE direct child: task-impl (the blocker)
	// task-tests should be a child of task-impl, not a direct child of epic
	require.Len(t, root.Children, 1, "epic should have 1 direct child (the blocker)")
	require.Equal(t, "task-impl", root.Children[0].Issue.ID)

	// task-impl should have task-tests as its child (via blocking relationship)
	require.Len(t, root.Children[0].Children, 1, "blocker should have blocked issue as child")
	require.Equal(t, "task-tests", root.Children[0].Children[0].Issue.ID)

	// Verify depths
	require.Equal(t, 0, root.Depth)
	require.Equal(t, 1, root.Children[0].Depth)
	require.Equal(t, 2, root.Children[0].Children[0].Depth)
}

func TestBuildTree_Down_ChainedBlocking(t *testing.T) {
	// Epic with three tasks in a blocking chain: A -> B -> C
	// All are children of epic, but the tree should show the chain.
	issueMap := map[string]*task.Issue{
		"epic":   bi("epic", task.WithChildren("task-c", "task-b", "task-a")),
		"task-a": bi("task-a", task.WithBlocks("task-b"), task.WithParentID("epic")),
		"task-b": bi("task-b", task.WithBlocks("task-c"), task.WithBlockedBy("task-a"), task.WithParentID("epic")),
		"task-c": bi("task-c", task.WithBlockedBy("task-b"), task.WithParentID("epic")),
	}

	root, err := BuildTree(issueMap, "epic", DirectionDown, ModeDeps)
	require.NoError(t, err)

	// Epic should have only task-a as direct child (first in chain)
	require.Len(t, root.Children, 1, "epic should have 1 direct child")
	require.Equal(t, "task-a", root.Children[0].Issue.ID)

	// task-a -> task-b
	require.Len(t, root.Children[0].Children, 1)
	require.Equal(t, "task-b", root.Children[0].Children[0].Issue.ID)

	// task-b -> task-c
	require.Len(t, root.Children[0].Children[0].Children, 1)
	require.Equal(t, "task-c", root.Children[0].Children[0].Children[0].Issue.ID)
}

func TestBuildTree_Down_WithDiscovered(t *testing.T) {
	// Issue with discovered-from relationships
	// bug-1 was discovered while working on feature-1 (discovered-from relationship)
	// Down traversal from feature-1 shows bug-1 via Discovered field
	issueMap := map[string]*task.Issue{
		"feature-1": bi("feature-1", task.WithDiscovered("bug-1")),
		"bug-1":     bi("bug-1", task.WithDiscoveredFrom("feature-1")),
	}

	root, err := BuildTree(issueMap, "feature-1", DirectionDown, ModeDeps)
	require.NoError(t, err)
	require.Len(t, root.Children, 1)
	require.Equal(t, "bug-1", root.Children[0].Issue.ID)
}

func TestBuildTree_Up_WithDiscoveredFrom(t *testing.T) {
	// Issue with discovered-from relationships (up direction)
	// bug-1 was discovered from feature-1, so traversing up from bug-1 shows feature-1
	// Up traversal from bug-1 shows feature-1 via DiscoveredFrom field
	issueMap := map[string]*task.Issue{
		"feature-1": bi("feature-1", task.WithDiscovered("bug-1")),
		"bug-1":     bi("bug-1", task.WithDiscoveredFrom("feature-1")),
	}

	root, err := BuildTree(issueMap, "bug-1", DirectionUp, ModeDeps)
	require.NoError(t, err)
	require.Len(t, root.Children, 1)
	require.Equal(t, "feature-1", root.Children[0].Issue.ID)
}

func TestBuildTree_Down_DiscoveredNotInChildrenMode(t *testing.T) {
	// Discovered issues should NOT appear in ModeChildren (only parent-child hierarchy)
	issueMap := map[string]*task.Issue{
		"feature-1": bi("feature-1", task.WithDiscovered("bug-1")),
		"bug-1":     bi("bug-1", task.WithDiscoveredFrom("feature-1")),
	}

	root, err := BuildTree(issueMap, "feature-1", DirectionDown, ModeChildren)
	require.NoError(t, err)
	// No children in children-only mode since there's no parent-child relationship
	require.Empty(t, root.Children)
}

func TestBuildTree_Down_CombinedDiscoveredAndBlocks(t *testing.T) {
	// Issue with both blocks and discovered-from relationships
	issueMap := map[string]*task.Issue{
		"feature-1": bi("feature-1", task.WithBlocks("feature-2"), task.WithDiscovered("bug-1")),
		"feature-2": bi("feature-2", task.WithBlockedBy("feature-1")),
		"bug-1":     bi("bug-1", task.WithDiscoveredFrom("feature-1")),
	}

	root, err := BuildTree(issueMap, "feature-1", DirectionDown, ModeDeps)
	require.NoError(t, err)
	require.Len(t, root.Children, 2) // Both blocked issue and discovered issue
	ids := []string{root.Children[0].Issue.ID, root.Children[1].Issue.ID}
	require.Contains(t, ids, "feature-2") // blocked by feature-1
	require.Contains(t, ids, "bug-1")     // discovered from feature-1
}

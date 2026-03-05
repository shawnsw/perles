// Package tree provides tree visualization components for issue dependency graphs.
package tree

import (
	"fmt"

	"github.com/zjrosen/perles/internal/task"
)

// Direction controls tree traversal direction.
type Direction string

const (
	// DirectionDown traverses Children + Blocks (what depends on this issue).
	DirectionDown Direction = "down"
	// DirectionUp traverses ParentID + BlockedBy (what this issue depends on).
	DirectionUp Direction = "up"
)

// TreeMode controls which relationships the tree shows.
type TreeMode string

const (
	// ModeDeps shows dependency relationships (blocks/blocked-by + parent/children).
	ModeDeps TreeMode = "deps"
	// ModeChildren shows only parent-child hierarchy (no dependencies).
	ModeChildren TreeMode = "children"
)

// String returns the mode as a display string.
func (m TreeMode) String() string {
	return string(m)
}

// String returns the direction as a string.
func (d Direction) String() string {
	return string(d)
}

// TreeNode represents a node in the dependency tree.
type TreeNode struct {
	Issue    task.Issue  // Issue data
	Children []*TreeNode // Child nodes in tree
	Depth    int         // Nesting level (0 = root)
	Parent   *TreeNode   // Parent node in tree (nil for root)
}

// BuildTree constructs a TreeNode hierarchy from an issue map.
// The issueMap contains issues returned by BQL expand query.
// Direction determines which relationships to traverse:
//   - DirectionDown: children + blocked issues (what depends on this)
//   - DirectionUp: parent + blocking issues (what this depends on)
//
// Mode determines which relationship types to include:
//   - ModeDeps: all relationships (parent/child + blocks/blocked-by)
//   - ModeChildren: only parent-child hierarchy
func BuildTree(issueMap map[string]*task.Issue, rootID string, dir Direction, mode TreeMode) (*TreeNode, error) {
	rootIssue, ok := issueMap[rootID]
	if !ok {
		return nil, fmt.Errorf("root issue %s not found", rootID)
	}

	seen := make(map[string]bool) // Prevent cycles
	return buildNode(rootIssue, issueMap, dir, mode, 0, seen, nil), nil
}

func buildNode(
	issue *task.Issue,
	issueMap map[string]*task.Issue,
	dir Direction,
	mode TreeMode,
	depth int,
	seen map[string]bool,
	parent *TreeNode,
) *TreeNode {
	if seen[issue.ID] {
		return nil // Cycle detected
	}
	seen[issue.ID] = true

	node := &TreeNode{
		Issue:  *issue,
		Depth:  depth,
		Parent: parent,
	}

	// Get related IDs based on direction and mode
	var relatedIDs []string
	if dir == DirectionDown {
		// Down direction
		relatedIDs = append(relatedIDs, issue.Children...)
		if mode == ModeDeps {
			// Include dependency relationships
			relatedIDs = append(relatedIDs, issue.Blocks...)
			// Include discovered-from relationships (down = issues discovered from this one)
			relatedIDs = append(relatedIDs, issue.Discovered...)
		}
	} else {
		// Up direction
		if issue.ParentID != "" {
			relatedIDs = append(relatedIDs, issue.ParentID)
		}
		if mode == ModeDeps {
			// Include dependency relationships
			relatedIDs = append(relatedIDs, issue.BlockedBy...)
			// Include discovered-from relationships (up = issues this was discovered from)
			relatedIDs = append(relatedIDs, issue.DiscoveredFrom...)
		}
	}

	// Sort relatedIDs so blockers come before blocked issues.
	// This ensures the tree structure properly represents dependency order:
	// if A blocks B and both are in relatedIDs, A should be processed first
	// so B appears as a descendant of A (via A's Blocks), not as a sibling.
	if mode == ModeDeps && len(relatedIDs) > 1 {
		relatedIDs = sortByBlockOrder(relatedIDs, issueMap, dir)
	}

	// Build child nodes (only for issues that exist in the map)
	for _, relatedID := range relatedIDs {
		if relatedIssue, ok := issueMap[relatedID]; ok {
			if child := buildNode(relatedIssue, issueMap, dir, mode, depth+1, seen, node); child != nil {
				node.Children = append(node.Children, child)
			}
		}
	}

	return node
}

// sortByBlockOrder performs a topological sort on IDs so blockers come before blocked issues.
// For down direction: issues that block others come first.
// For up direction: issues that are blocked by others come first.
// This ensures proper tree structure where blocked issues appear as descendants of their blockers.
func sortByBlockOrder(ids []string, issueMap map[string]*task.Issue, dir Direction) []string {
	if len(ids) <= 1 {
		return ids
	}

	// Build a set of IDs we're sorting
	idSet := make(map[string]bool, len(ids))
	for _, id := range ids {
		idSet[id] = true
	}

	// Build dependency graph: edges[a] = [b, c] means a must come before b and c
	// For down: if A blocks B, A must come before B
	// For up: if A is blocked by B, A must come before B (we want blocked issues first)
	edges := make(map[string][]string)
	inDegree := make(map[string]int)

	// Initialize in-degree for all IDs
	for _, id := range ids {
		inDegree[id] = 0
	}

	for _, id := range ids {
		issue, ok := issueMap[id]
		if !ok {
			continue
		}

		if dir == DirectionDown {
			// A blocks B means A should come before B
			for _, blockedID := range issue.Blocks {
				if idSet[blockedID] {
					edges[id] = append(edges[id], blockedID)
					inDegree[blockedID]++
				}
			}
		} else {
			// A is blocked by B means B should come before A
			// So we add edge from B to A
			for _, blockerID := range issue.BlockedBy {
				if idSet[blockerID] {
					edges[blockerID] = append(edges[blockerID], id)
					inDegree[id]++
				}
			}
		}
	}

	// Kahn's algorithm for topological sort
	var result []string
	var queue []string

	// Start with nodes that have no incoming edges (no blockers)
	for _, id := range ids {
		if inDegree[id] == 0 {
			queue = append(queue, id)
		}
	}

	for len(queue) > 0 {
		// Pop from queue
		node := queue[0]
		queue = queue[1:]
		result = append(result, node)

		// Remove edges from this node
		for _, neighbor := range edges[node] {
			inDegree[neighbor]--
			if inDegree[neighbor] == 0 {
				queue = append(queue, neighbor)
			}
		}
	}

	// If there's a cycle, some nodes won't be in result - add them at the end
	if len(result) < len(ids) {
		resultSet := make(map[string]bool)
		for _, id := range result {
			resultSet[id] = true
		}
		for _, id := range ids {
			if !resultSet[id] {
				result = append(result, id)
			}
		}
	}

	return result
}

// Flatten returns a slice of all nodes in tree order.
func (n *TreeNode) Flatten() []*TreeNode {
	var result []*TreeNode
	result = append(result, n)

	for _, child := range n.Children {
		result = append(result, child.Flatten()...)
	}

	return result
}

// CalculateProgress returns the count of closed issues and total issues
// in this subtree (including this node).
func (n *TreeNode) CalculateProgress() (closed, total int) {
	if n.Issue.Status == task.StatusClosed {
		closed++
	}
	total++

	for _, child := range n.Children {
		c, t := child.CalculateProgress()
		closed += c
		total += t
	}

	return closed, total
}

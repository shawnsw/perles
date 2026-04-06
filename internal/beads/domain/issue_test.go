package domain

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestIssue_FieldAccess(t *testing.T) {
	issue := Issue{
		ID:           "bd-123",
		TitleText:    "Test issue",
		Status:       StatusOpen,
		Priority:     PriorityHigh,
		Type:         TypeTask,
		Labels:       []string{"bug", "urgent"},
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
		BlockedBy:    []string{"bd-100"},
		Blocks:       []string{"bd-200"},
		Children:     []string{"bd-300"},
		ParentID:     "bd-parent",
		CommentCount: 5,
	}

	require.Equal(t, "bd-123", issue.ID)
	require.Equal(t, "Test issue", issue.TitleText)
	require.Equal(t, StatusOpen, issue.Status)
	require.Equal(t, PriorityHigh, issue.Priority)
	require.Equal(t, TypeTask, issue.Type)
	require.ElementsMatch(t, []string{"bug", "urgent"}, issue.Labels)
	require.Len(t, issue.BlockedBy, 1)
	require.Len(t, issue.Blocks, 1)
	require.Len(t, issue.Children, 1)
	require.Equal(t, "bd-parent", issue.ParentID)
	require.Equal(t, 5, issue.CommentCount)
}

func TestComment_FieldAccess(t *testing.T) {
	now := time.Now()
	comment := Comment{
		ID:        1,
		Author:    "alice",
		Text:      "This is a comment",
		CreatedAt: now,
	}

	require.Equal(t, 1, comment.ID)
	require.Equal(t, "alice", comment.Author)
	require.Equal(t, "This is a comment", comment.Text)
	require.Equal(t, now, comment.CreatedAt)
}

func TestCreateResult_FieldAccess(t *testing.T) {
	result := CreateResult{
		ID:    "bd-new-123",
		Title: "New Issue",
	}

	require.Equal(t, "bd-new-123", result.ID)
	require.Equal(t, "New Issue", result.Title)
}

func TestStatus_Values(t *testing.T) {
	require.Equal(t, Status("open"), StatusOpen)
	require.Equal(t, Status("in_progress"), StatusInProgress)
	require.Equal(t, Status("closed"), StatusClosed)
}

func TestPriority_Values(t *testing.T) {
	require.Equal(t, Priority(0), PriorityCritical)
	require.Equal(t, Priority(1), PriorityHigh)
	require.Equal(t, Priority(2), PriorityMedium)
	require.Equal(t, Priority(3), PriorityLow)
	require.Equal(t, Priority(4), PriorityBacklog)
}

func TestIssueType_Values(t *testing.T) {
	require.Equal(t, IssueType("bug"), TypeBug)
	require.Equal(t, IssueType("feature"), TypeFeature)
	require.Equal(t, IssueType("task"), TypeTask)
	require.Equal(t, IssueType("epic"), TypeEpic)
	require.Equal(t, IssueType("chore"), TypeChore)
	require.Equal(t, IssueType("milestone"), TypeMilestone)
	require.Equal(t, IssueType("story"), TypeStory)
	require.Equal(t, IssueType("spike"), TypeSpike)
	require.Equal(t, IssueType("molecule"), TypeMolecule)
	require.Equal(t, IssueType("convoy"), TypeConvoy)
	require.Equal(t, IssueType("agent"), TypeAgent)
}

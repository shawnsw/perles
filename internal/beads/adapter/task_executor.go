package adapter

import (
	"fmt"

	appbeads "github.com/zjrosen/perles/internal/beads/application"
	"github.com/zjrosen/perles/internal/task"
)

// Compile-time check.
var _ task.TaskExecutor = (*BeadsTaskExecutor)(nil)

// BeadsTaskExecutor wraps a beads IssueExecutor to implement task.TaskExecutor.
type BeadsTaskExecutor struct {
	inner                 appbeads.IssueExecutor
	commentReader         appbeads.CommentReader // optional, primary reader
	fallbackCommentReader appbeads.CommentReader // optional, CLI-compatible fallback
}

// TaskExecutorOption configures a BeadsTaskExecutor.
type TaskExecutorOption func(*BeadsTaskExecutor)

// WithCommentReader sets the comment reader for loading issue comments.
// The BDExecutor (CLI tool) cannot read comments directly, so comments
// are provided by the database client which implements appbeads.CommentReader.
func WithCommentReader(cr appbeads.CommentReader) TaskExecutorOption {
	return func(e *BeadsTaskExecutor) {
		e.commentReader = cr
	}
}

// WithFallbackCommentReader sets a fallback comment reader used when the
// primary reader cannot serve comments for the current backend/schema.
func WithFallbackCommentReader(cr appbeads.CommentReader) TaskExecutorOption {
	return func(e *BeadsTaskExecutor) {
		e.fallbackCommentReader = cr
	}
}

// NewBeadsTaskExecutor creates a new BeadsTaskExecutor wrapping the given IssueExecutor.
func NewBeadsTaskExecutor(inner appbeads.IssueExecutor, opts ...TaskExecutorOption) *BeadsTaskExecutor {
	e := &BeadsTaskExecutor{inner: inner}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

func (e *BeadsTaskExecutor) GetComments(issueID string) ([]task.Comment, error) {
	if e.commentReader != nil {
		comments, err := e.commentReader.GetComments(issueID)
		if err == nil {
			return ToTaskComments(comments), nil
		}
		if e.fallbackCommentReader == nil {
			return nil, err
		}
		fallbackComments, fallbackErr := e.fallbackCommentReader.GetComments(issueID)
		if fallbackErr == nil {
			return ToTaskComments(fallbackComments), nil
		}
		return nil, fmt.Errorf("reading comments failed: %w; fallback failed: %v", err, fallbackErr)
	}
	if e.fallbackCommentReader == nil {
		return nil, nil
	}
	comments, err := e.fallbackCommentReader.GetComments(issueID)
	if err != nil {
		return nil, err
	}
	return ToTaskComments(comments), nil
}

func (e *BeadsTaskExecutor) ShowIssue(issueID string) (*task.Issue, error) {
	issue, err := e.inner.ShowIssue(issueID)
	if err != nil {
		return nil, err
	}
	out := ToTaskIssue(*issue)
	return &out, nil
}

func (e *BeadsTaskExecutor) UpdateStatus(issueID string, status task.Status) error {
	return e.inner.UpdateStatus(issueID, FromTaskStatus(status))
}

func (e *BeadsTaskExecutor) UpdatePriority(issueID string, priority task.Priority) error {
	return e.inner.UpdatePriority(issueID, FromTaskPriority(priority))
}

func (e *BeadsTaskExecutor) UpdateType(issueID string, issueType task.IssueType) error {
	return e.inner.UpdateType(issueID, FromTaskIssueType(issueType))
}

func (e *BeadsTaskExecutor) UpdateTitle(issueID, title string) error {
	return e.inner.UpdateTitle(issueID, title)
}

func (e *BeadsTaskExecutor) UpdateDescription(issueID, description string) error {
	return e.inner.UpdateDescription(issueID, description)
}

func (e *BeadsTaskExecutor) UpdateNotes(issueID, notes string) error {
	return e.inner.UpdateNotes(issueID, notes)
}

func (e *BeadsTaskExecutor) CloseIssue(issueID, reason string) error {
	return e.inner.CloseIssue(issueID, reason)
}

func (e *BeadsTaskExecutor) ReopenIssue(issueID string) error {
	return e.inner.ReopenIssue(issueID)
}

func (e *BeadsTaskExecutor) SetLabels(issueID string, labels []string) error {
	return e.inner.SetLabels(issueID, labels)
}

func (e *BeadsTaskExecutor) AddComment(issueID, author, text string) error {
	return e.inner.AddComment(issueID, author, text)
}

func (e *BeadsTaskExecutor) CreateEpic(title, description string, labels []string) (task.CreateResult, error) {
	result, err := e.inner.CreateEpic(title, description, labels)
	if err != nil {
		return task.CreateResult{}, err
	}
	return ToTaskCreateResult(result), nil
}

func (e *BeadsTaskExecutor) CreateTask(title, description, parentID, assignee string, labels []string) (task.CreateResult, error) {
	result, err := e.inner.CreateTask(title, description, parentID, assignee, labels)
	if err != nil {
		return task.CreateResult{}, err
	}
	return ToTaskCreateResult(result), nil
}

func (e *BeadsTaskExecutor) DeleteIssues(issueIDs []string) error {
	return e.inner.DeleteIssues(issueIDs)
}

func (e *BeadsTaskExecutor) AddDependency(taskID, dependsOnID string) error {
	return e.inner.AddDependency(taskID, dependsOnID)
}

func (e *BeadsTaskExecutor) UpdateIssue(issueID string, opts task.UpdateOptions) error {
	return e.inner.UpdateIssue(issueID, FromTaskUpdateOptions(opts))
}

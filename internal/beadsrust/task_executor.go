package beadsrust

import (
	"github.com/zjrosen/perles/internal/task"
)

// Compile-time check that beadsRustTaskExecutor implements task.TaskExecutor.
var _ task.TaskExecutor = (*beadsRustTaskExecutor)(nil)

// beadsRustTaskExecutor composes SQLiteReader (reads) with BRExecutor (writes)
// to implement the full task.TaskExecutor interface.
type beadsRustTaskExecutor struct {
	reader *SQLiteReader
	writer *BRExecutor
}

// newTaskExecutor creates a composite TaskExecutor.
func newTaskExecutor(reader *SQLiteReader, writer *BRExecutor) *beadsRustTaskExecutor {
	return &beadsRustTaskExecutor{reader: reader, writer: writer}
}

// --- Reads (delegated to SQLiteReader) ---

func (e *beadsRustTaskExecutor) ShowIssue(issueID string) (*task.Issue, error) {
	return e.reader.ShowIssue(issueID)
}

func (e *beadsRustTaskExecutor) GetComments(issueID string) ([]task.Comment, error) {
	return e.reader.GetComments(issueID)
}

// --- Writes (delegated to BRExecutor) ---

func (e *beadsRustTaskExecutor) UpdateStatus(issueID string, status task.Status) error {
	return e.writer.UpdateStatus(issueID, status)
}

func (e *beadsRustTaskExecutor) UpdatePriority(issueID string, priority task.Priority) error {
	return e.writer.UpdatePriority(issueID, priority)
}

func (e *beadsRustTaskExecutor) UpdateType(issueID string, issueType task.IssueType) error {
	return e.writer.UpdateType(issueID, issueType)
}

func (e *beadsRustTaskExecutor) UpdateTitle(issueID, title string) error {
	return e.writer.UpdateTitle(issueID, title)
}

func (e *beadsRustTaskExecutor) UpdateDescription(issueID, description string) error {
	return e.writer.UpdateDescription(issueID, description)
}

func (e *beadsRustTaskExecutor) UpdateNotes(issueID, notes string) error {
	return e.writer.UpdateNotes(issueID, notes)
}

func (e *beadsRustTaskExecutor) UpdateIssue(issueID string, opts task.UpdateOptions) error {
	return e.writer.UpdateIssue(issueID, opts)
}

func (e *beadsRustTaskExecutor) CloseIssue(issueID, reason string) error {
	return e.writer.CloseIssue(issueID, reason)
}

func (e *beadsRustTaskExecutor) ReopenIssue(issueID string) error {
	return e.writer.ReopenIssue(issueID)
}

func (e *beadsRustTaskExecutor) DeleteIssues(issueIDs []string) error {
	return e.writer.DeleteIssues(issueIDs)
}

func (e *beadsRustTaskExecutor) SetLabels(issueID string, labels []string) error {
	return e.writer.SetLabels(issueID, labels)
}

func (e *beadsRustTaskExecutor) AddComment(issueID, author, text string) error {
	return e.writer.AddComment(issueID, author, text)
}

func (e *beadsRustTaskExecutor) CreateEpic(title, description string, labels []string) (task.CreateResult, error) {
	return e.writer.CreateEpic(title, description, labels)
}

func (e *beadsRustTaskExecutor) CreateTask(title, description, parentID, assignee string, labels []string) (task.CreateResult, error) {
	return e.writer.CreateTask(title, description, parentID, assignee, labels)
}

func (e *beadsRustTaskExecutor) AddDependency(taskID, dependsOnID string) error {
	return e.writer.AddDependency(taskID, dependsOnID)
}

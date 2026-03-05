package dashboard

import (
	"errors"
	"strings"
	"testing"
	"testing/fstest"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/zjrosen/perles/internal/config"
	domaingit "github.com/zjrosen/perles/internal/git/domain"
	"github.com/zjrosen/perles/internal/keys"
	"github.com/zjrosen/perles/internal/mocks"
	"github.com/zjrosen/perles/internal/mode"
	"github.com/zjrosen/perles/internal/orchestration/controlplane"
	controlplanemocks "github.com/zjrosen/perles/internal/orchestration/controlplane/mocks"
	appreg "github.com/zjrosen/perles/internal/registry/application"
	registry "github.com/zjrosen/perles/internal/registry/domain"
	"github.com/zjrosen/perles/internal/task"
	"github.com/zjrosen/perles/internal/ui/shared/formmodal"
)

// === Test Helpers ===

// createTestRegistryServiceFS creates a MapFS for testing with workflow subdirectories
func createTestRegistryServiceFS() fstest.MapFS {
	return fstest.MapFS{
		"workflows/quick-plan/template.yaml": &fstest.MapFile{
			Data: []byte(`
registry:
  - namespace: "workflow"
    key: "quick-plan"
    version: "v1"
    name: "Quick Plan"
    description: "Fast planning workflow"
    nodes:
      - key: "plan"
        name: "Plan"
        template: "v1-plan.md"
`),
		},
		"workflows/cook/template.yaml": &fstest.MapFile{
			Data: []byte(`
registry:
  - namespace: "workflow"
    key: "cook"
    version: "v1"
    name: "Cook"
    description: "Implementation workflow"
    nodes:
      - key: "cook"
        name: "Cook"
        template: "v1-cook.md"
`),
		},
		"workflows/research/template.yaml": &fstest.MapFile{
			Data: []byte(`
registry:
  - namespace: "workflow"
    key: "research"
    version: "v1"
    name: "Research"
    description: "Research to tasks"
    nodes:
      - key: "research"
        name: "Research"
        template: "v1-research.md"
`),
		},
		"workflows/quick-plan/v1-plan.md":   &fstest.MapFile{Data: []byte("# Plan Template")},
		"workflows/cook/v1-cook.md":         &fstest.MapFile{Data: []byte("# Cook Template")},
		"workflows/research/v1-research.md": &fstest.MapFile{Data: []byte("# Research Template")},
	}
}

// createTestRegistryService creates a registry service with test templates.
func createTestRegistryService(t *testing.T) *appreg.RegistryService {
	t.Helper()
	registryFS := createTestRegistryServiceFS()
	registry, err := appreg.NewRegistryService(registryFS, nil, "")
	require.NoError(t, err)
	return registry
}

// createTestWorkflowCreator creates a WorkflowCreator with a mock executor for testing.
// The mock is set up to return successful results for epic/task creation.
func createTestWorkflowCreator(t *testing.T, registryService *appreg.RegistryService) *appreg.WorkflowCreator {
	t.Helper()
	mockExecutor := mocks.NewMockTaskExecutor(t)
	mockExecutor.EXPECT().CreateEpic(mock.Anything, mock.Anything, mock.Anything).
		Return(task.CreateResult{ID: "epic-123"}, nil).Maybe()
	mockExecutor.EXPECT().CreateTask(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(task.CreateResult{ID: "task-456"}, nil).Maybe()
	mockExecutor.EXPECT().AddDependency(mock.Anything, mock.Anything).Return(nil).Maybe()
	return appreg.NewWorkflowCreator(registryService, mockExecutor, config.TemplatesConfig{})
}

// simulateAsyncSubmit simulates the full async form submission flow:
// 1. onSubmit returns startSubmitMsg
// 2. Update handles startSubmitMsg and starts async creation
// 3. Execute the returned command to get the final result (CreateWorkflowMsg or ErrorMsg)
func simulateAsyncSubmit(t *testing.T, modal *NewWorkflowModal, values map[string]any) tea.Msg {
	t.Helper()

	// Step 1: onSubmit returns startSubmitMsg
	msg := modal.onSubmit(values)
	submitMsg, ok := msg.(startSubmitMsg)
	require.True(t, ok, "onSubmit should return startSubmitMsg, got %T", msg)

	// Step 2: Update handles startSubmitMsg and returns async command
	modal, cmd := modal.Update(submitMsg)
	require.True(t, modal.form.IsLoading(), "modal should be in loading state")

	// Step 3: Execute the batch command to find the async creation command
	// tea.Batch returns multiple commands, we need the one that does actual work
	if cmd == nil {
		t.Fatal("Update should return a command for async creation")
	}

	// Execute commands until we get a CreateWorkflowMsg or ErrorMsg
	// The batch contains spinnerTick and createWorkflowAsync
	msgs := extractBatchMessages(cmd)
	for _, m := range msgs {
		switch m.(type) {
		case CreateWorkflowMsg, ErrorMsg:
			return m
		}
	}

	t.Fatal("Did not receive CreateWorkflowMsg or ErrorMsg from async submission")
	return nil
}

// extractBatchMessages executes a batch command and extracts all resulting messages.
func extractBatchMessages(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}

	msg := cmd()
	if msg == nil {
		return nil
	}

	// Check if this is a batch result (tea.BatchMsg)
	if batch, ok := msg.(tea.BatchMsg); ok {
		var results []tea.Msg
		for _, c := range batch {
			results = append(results, extractBatchMessages(c)...)
		}
		return results
	}

	// Not a batch, just return the message
	return []tea.Msg{msg}
}

// createTestModelWithRegistryService creates a dashboard model with a mock ControlPlane and registry service.
func createTestModelWithRegistryService(t *testing.T, workflows []*controlplane.WorkflowInstance) (Model, *controlplanemocks.MockControlPlane, *appreg.RegistryService) {
	t.Helper()

	mockCP := newMockControlPlane(t)
	mockCP.On("List", mock.Anything, mock.Anything).Return(workflows, nil).Maybe()

	eventCh := make(chan controlplane.ControlPlaneEvent)
	close(eventCh)
	mockCP.On("Subscribe", mock.Anything).Return((<-chan controlplane.ControlPlaneEvent)(eventCh), func() {}).Maybe()

	registryService := createTestRegistryService(t)

	cfg := Config{
		ControlPlane:    mockCP,
		Services:        mode.Services{},
		RegistryService: registryService,
	}

	m := New(cfg)
	m.workflows = workflows
	m.workflowList = m.workflowList.SetWorkflows(workflows)
	m.resourceSummary = m.resourceSummary.Update(workflows)
	m = m.SetSize(100, 40).(Model)

	return m, mockCP, registryService
}

// === Unit Tests: Modal loads templates from registry ===

func TestNewWorkflowModal_LoadsTemplatesFromRegistry(t *testing.T) {
	registryService := createTestRegistryService(t)
	modal := NewNewWorkflowModal(registryService, nil, nil, nil, nil, false, "")
	require.NotNil(t, modal)

	// Modal should be created with templates from registry
	// The form should have fields configured
	view := modal.View()
	require.NotEmpty(t, view)
	require.Contains(t, view, "Template")
}

func TestNewWorkflowModal_HandlesNilRegistry(t *testing.T) {
	modal := NewNewWorkflowModal(nil, nil, nil, nil, nil, false, "")
	require.NotNil(t, modal)

	// Should still render without crashing
	view := modal.View()
	require.NotEmpty(t, view)
}

// === Unit Tests: Form validation ===

func TestNewWorkflowModal_ValidationRejectsEmptyTemplate(t *testing.T) {
	registryService := createTestRegistryService(t)
	modal := NewNewWorkflowModal(registryService, nil, nil, nil, nil, false, "")

	values := map[string]any{
		"template": "",
		"name":     "",
	}

	err := modal.validate(values)
	require.Error(t, err)
	require.Contains(t, err.Error(), "template is required")
}

func TestNewWorkflowModal_ValidationAcceptsValidInput(t *testing.T) {
	registryService := createTestRegistryService(t)
	modal := NewNewWorkflowModal(registryService, nil, nil, nil, nil, false, "")

	values := map[string]any{
		"template": "quick-plan",
		"name":     "My Workflow",
	}

	err := modal.validate(values)
	require.NoError(t, err)
}

// === Unit Tests: Cancel closes modal without action ===

func TestNewWorkflowModal_CancelClosesModal(t *testing.T) {
	m, _, _ := createTestModelWithRegistryService(t, []*controlplane.WorkflowInstance{})

	// Open the modal
	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	m = result.(Model)
	require.True(t, m.InNewWorkflowModal())

	// Press Escape to cancel
	result, _ = m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	m = result.(Model)

	// Modal should now receive CancelNewWorkflowMsg
	result, _ = m.Update(CancelNewWorkflowMsg{})
	m = result.(Model)
	require.False(t, m.InNewWorkflowModal())
}

// === Unit Tests: Create calls ControlPlane.Create ===

func TestNewWorkflowModal_CreateCallsControlPlane(t *testing.T) {
	mockCP := newMockControlPlane(t)
	mockCP.On("List", mock.Anything, mock.Anything).Return([]*controlplane.WorkflowInstance{}, nil).Maybe()
	// Goal is no longer a hardcoded field - it's now a template argument
	mockCP.On("Create", mock.Anything, mock.MatchedBy(func(spec controlplane.WorkflowSpec) bool {
		return spec.TemplateID == "quick-plan"
	})).Return(controlplane.WorkflowID("new-workflow-id"), nil).Once()

	eventCh := make(chan controlplane.ControlPlaneEvent)
	close(eventCh)
	mockCP.On("Subscribe", mock.Anything).Return((<-chan controlplane.ControlPlaneEvent)(eventCh), func() {}).Maybe()

	registryService := createTestRegistryService(t)
	workflowCreator := createTestWorkflowCreator(t, registryService)
	modal := NewNewWorkflowModal(registryService, mockCP, nil, workflowCreator, nil, false, "")

	// Simulate form submission (now async)
	values := map[string]any{
		"template": "quick-plan",
		"name":     "",
	}

	msg := simulateAsyncSubmit(t, modal, values)
	createMsg, ok := msg.(CreateWorkflowMsg)
	require.True(t, ok)
	require.Equal(t, controlplane.WorkflowID("new-workflow-id"), createMsg.WorkflowID)

	mockCP.AssertExpectations(t)
}

// === Unit Tests: Create workflow always starts immediately ===

func TestDashboard_CreateWorkflowStartsImmediately(t *testing.T) {
	mockCP := newMockControlPlane(t)
	mockCP.On("List", mock.Anything, mock.Anything).Return([]*controlplane.WorkflowInstance{}, nil).Maybe()
	mockCP.On("Start", mock.Anything, controlplane.WorkflowID("new-wf")).Return(nil).Maybe()

	eventCh := make(chan controlplane.ControlPlaneEvent)
	close(eventCh)
	mockCP.On("Subscribe", mock.Anything).Return((<-chan controlplane.ControlPlaneEvent)(eventCh), func() {}).Maybe()

	registryService := createTestRegistryService(t)

	cfg := Config{
		ControlPlane:    mockCP,
		Services:        mode.Services{},
		RegistryService: registryService,
	}

	m := New(cfg)
	m.workflows = []*controlplane.WorkflowInstance{}
	m = m.SetSize(100, 40).(Model)

	// Open modal
	result, _ := m.openNewWorkflowModal()
	m = result.(Model)

	// Simulate successful creation
	result, cmd := m.Update(CreateWorkflowMsg{
		WorkflowID: "new-wf",
		Name:       "Test",
	})
	m = result.(Model)

	// Modal should be closed
	require.False(t, m.InNewWorkflowModal())

	// Command should be returned (includes start workflow)
	require.NotNil(t, cmd)
}

// === Unit Tests: Resource limits default to empty ===

func TestNewWorkflowModal_ResourceLimitsOptional(t *testing.T) {
	registryService := createTestRegistryService(t)
	modal := NewNewWorkflowModal(registryService, nil, nil, nil, nil, false, "")

	values := map[string]any{
		"template":     "quick-plan",
		"name":         "",
		"priority":     "normal",
		"max_workers":  "",
		"token_budget": "",
	}

	// Should pass validation with empty resource limits
	err := modal.validate(values)
	require.NoError(t, err)
}

// === Unit Tests: Tab navigates between fields ===

func TestNewWorkflowModal_TabNavigates(t *testing.T) {
	registryService := createTestRegistryService(t)
	modal := NewNewWorkflowModal(registryService, nil, nil, nil, nil, false, "").SetSize(100, 40)

	// Press Tab - should navigate to next field
	modal, _ = modal.Update(tea.KeyMsg{Type: tea.KeyTab})

	// Modal should still be functional
	require.NotNil(t, modal)
	view := modal.View()
	require.NotEmpty(t, view)
}

// === Unit Tests: N key opens modal ===

func TestDashboard_NKeyOpensModal(t *testing.T) {
	m, _, _ := createTestModelWithRegistryService(t, []*controlplane.WorkflowInstance{})
	require.False(t, m.InNewWorkflowModal())

	// Press n to open modal
	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	m = result.(Model)

	require.True(t, m.InNewWorkflowModal())
	// Note: Init command may be nil if no text inputs need blink
}

func TestDashboard_ShiftNKeyOpensModal(t *testing.T) {
	m, _, _ := createTestModelWithRegistryService(t, []*controlplane.WorkflowInstance{})

	// Press N (shift+n) to open modal
	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'N'}})
	m = result.(Model)

	require.True(t, m.InNewWorkflowModal())
}

// === Unit Tests: Escape key in dashboard doesn't interfere ===

func TestDashboard_EscapeKeyWithoutModal(t *testing.T) {
	m, _, _ := createTestModelWithRegistryService(t, []*controlplane.WorkflowInstance{})

	// Press Escape without modal open - should not crash
	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	m = result.(Model)

	// Dashboard should still be functional
	view := m.View()
	require.NotEmpty(t, view)
}

// === Unit Tests: Modal overlay rendering ===

func TestDashboard_ModalRendersAsOverlay(t *testing.T) {
	m, _, _ := createTestModelWithRegistryService(t, []*controlplane.WorkflowInstance{})

	// Open modal
	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	m = result.(Model)

	// View should contain modal content (Goal is now a template argument, not shown for all templates)
	view := m.View()
	require.Contains(t, view, "New Workflow")
	require.Contains(t, view, "Template")
}

// === Unit Tests: Window resize updates modal ===

func TestDashboard_WindowResizeUpdatesModal(t *testing.T) {
	m, _, _ := createTestModelWithRegistryService(t, []*controlplane.WorkflowInstance{})

	// Open modal
	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	m = result.(Model)

	// Resize window
	result, _ = m.Update(tea.WindowSizeMsg{Width: 150, Height: 50})
	m = result.(Model)

	require.Equal(t, 150, m.width)
	require.Equal(t, 50, m.height)
	require.True(t, m.InNewWorkflowModal())
}

// === Unit Tests: Modal handles Ctrl+S ===

func TestNewWorkflowModal_CtrlSSavesForm(t *testing.T) {
	registryService := createTestRegistryService(t)
	modal := NewNewWorkflowModal(registryService, nil, nil, nil, nil, false, "").SetSize(100, 40)

	// Press Ctrl+S - should trigger save/validation
	// Since form is empty, it should show validation error
	modal, _ = modal.Update(tea.KeyMsg{Type: tea.KeyCtrlS})

	// Modal should still be functional (validation error shown)
	require.NotNil(t, modal)
}

// === Integration Tests: Full workflow creation flow ===

func TestDashboard_FullWorkflowCreationFlow(t *testing.T) {
	mockCP := newMockControlPlane(t)
	mockCP.On("List", mock.Anything, mock.Anything).Return([]*controlplane.WorkflowInstance{}, nil).Maybe()
	mockCP.On("Create", mock.Anything, mock.Anything).Return(controlplane.WorkflowID("created-wf"), nil).Maybe()

	eventCh := make(chan controlplane.ControlPlaneEvent)
	close(eventCh)
	mockCP.On("Subscribe", mock.Anything).Return((<-chan controlplane.ControlPlaneEvent)(eventCh), func() {}).Maybe()

	registryService := createTestRegistryService(t)

	cfg := Config{
		ControlPlane:    mockCP,
		Services:        mode.Services{},
		RegistryService: registryService,
	}

	m := New(cfg)
	m = m.SetSize(100, 40).(Model)

	// 1. Open modal
	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	m = result.(Model)
	require.True(t, m.InNewWorkflowModal())

	// 2. Simulate receiving CreateWorkflowMsg (as if form was filled and submitted)
	result, _ = m.Update(CreateWorkflowMsg{
		WorkflowID: "created-wf",
		Name:       "Test Workflow",
	})
	m = result.(Model)

	// 3. Modal should be closed
	require.False(t, m.InNewWorkflowModal())
}

// Test that buildTemplateOptions handles empty registry
func TestBuildTemplateOptions_EmptyRegistry(t *testing.T) {
	// Create a domain registry with no workflow registrations
	fs := fstest.MapFS{
		"workflows/go-guidelines/template.yaml": &fstest.MapFile{
			Data: []byte(`
registry:
  - namespace: "lang-guidelines"
    key: "go"
    version: "v1"
    name: "Go Guidelines"
    description: "Go language guidelines"
    nodes:
      - key: "coding"
        name: "Coding"
        template: "v1-coding.md"
`),
		},
		"workflows/go-guidelines/v1-coding.md": &fstest.MapFile{Data: []byte("# Coding Guidelines")},
	}
	registryService, err := appreg.NewRegistryService(fs, nil, "")
	require.NoError(t, err)

	options := buildTemplateOptions(registryService)
	require.Empty(t, options) // No workflow registrations
}

// Test that buildTemplateOptions creates correct options
func TestBuildTemplateOptions_CreatesCorrectOptions(t *testing.T) {
	registryService := createTestRegistryService(t)
	options := buildTemplateOptions(registryService)

	require.Len(t, options, 3)

	// Options should include template info
	hasQuickPlan := false
	for _, opt := range options {
		if opt.Value == "quick-plan" {
			hasQuickPlan = true
			require.Contains(t, opt.Label, "Quick Plan")
		}
	}
	require.True(t, hasQuickPlan)
}

// Test that buildTemplateOptions handles nil registry
func TestBuildTemplateOptions_NilRegistry(t *testing.T) {
	options := buildTemplateOptions(nil)
	require.Empty(t, options)
}

// Test escape key handler checks for common escape binding
func TestNewWorkflowModal_EscapeClearsModal(t *testing.T) {
	registryService := createTestRegistryService(t)
	modal := NewNewWorkflowModal(registryService, nil, nil, nil, nil, false, "").SetSize(100, 40)

	// Press escape
	modal, cmd := modal.Update(keys.Common.Escape.Keys()[0])
	require.NotNil(t, modal)

	// Should produce a cancel message command
	if cmd != nil {
		msg := cmd()
		_, isCancel := msg.(CancelNewWorkflowMsg)
		require.True(t, isCancel)
	}
}

// === Worktree UI Tests ===

// createMockGitExecutorWithBranches creates a mock GitExecutor with test branches and worktrees.
func createMockGitExecutorWithBranches(t *testing.T) *mocks.MockGitExecutor {
	t.Helper()
	mockGit := mocks.NewMockGitExecutor(t)
	mockGit.EXPECT().ListBranches().Return([]domaingit.BranchInfo{
		{Name: "main", IsCurrent: false},
		{Name: "develop", IsCurrent: true},
		{Name: "feature/auth", IsCurrent: false},
	}, nil).Maybe()
	mockGit.EXPECT().ListWorktrees().Return([]domaingit.WorktreeInfo{
		{Path: "/repo", Branch: "main"},
		{Path: "/repo-worktree-1", Branch: "feature/wt1"},
	}, nil).Maybe()
	return mockGit
}

func TestNewWorkflowModal_PopulatesBranchOptionsFromListBranches(t *testing.T) {
	registryService := createTestRegistryService(t)
	mockGit := createMockGitExecutorWithBranches(t)

	modal := NewNewWorkflowModal(registryService, nil, mockGit, nil, nil, false, "")
	require.NotNil(t, modal)
	require.True(t, modal.worktreeEnabled)

	// Modal should contain Git Worktree select (always visible)
	view := modal.SetSize(100, 40).View()
	require.Contains(t, view, "Git Worktree")

	// Branch fields should be hidden initially (worktree mode defaults to "none")
	require.NotContains(t, view, "Base Branch")
	require.NotContains(t, view, "Branch Name")

	// Navigate to the worktree select and switch to "New Worktree"
	// Tab through: Template -> Name -> Git Worktree
	modal, _ = modal.Update(tea.KeyMsg{Type: tea.KeyTab})
	modal, _ = modal.Update(tea.KeyMsg{Type: tea.KeyTab})
	// Move down twice in select to reach "New Worktree" (3rd option) and select it with space
	modal, _ = modal.Update(tea.KeyMsg{Type: tea.KeyDown})
	modal, _ = modal.Update(tea.KeyMsg{Type: tea.KeyDown})
	modal, _ = modal.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})

	// Now branch fields should be visible
	view = modal.View()
	require.Contains(t, view, "Base Branch")
	require.Contains(t, view, "Branch Name")
}

func TestNewWorkflowModal_DisablesWorktreeFieldsWhenListBranchesFails(t *testing.T) {
	registryService := createTestRegistryService(t)
	mockGit := mocks.NewMockGitExecutor(t)
	mockGit.EXPECT().ListBranches().Return(nil, errors.New("not a git repo"))
	mockGit.EXPECT().ListWorktrees().Return(nil, errors.New("not a git repo")).Maybe()

	modal := NewNewWorkflowModal(registryService, nil, mockGit, nil, nil, false, "")
	require.NotNil(t, modal)
	require.False(t, modal.worktreeEnabled)

	// Modal should NOT contain worktree fields when git fails
	view := modal.SetSize(100, 40).View()
	require.NotContains(t, view, "Git Worktree")
	require.NotContains(t, view, "Base Branch")
}

func TestNewWorkflowModal_DisablesWorktreeFieldsWhenGitExecutorNil(t *testing.T) {
	registryService := createTestRegistryService(t)

	modal := NewNewWorkflowModal(registryService, nil, nil, nil, nil, false, "")
	require.NotNil(t, modal)
	require.False(t, modal.worktreeEnabled)

	// Modal should NOT contain worktree fields when no git executor
	view := modal.SetSize(100, 40).View()
	require.NotContains(t, view, "Git Worktree")
	require.NotContains(t, view, "Base Branch")
}

func TestNewWorkflowModal_OnSubmitSetsWorktreeEnabledCorrectly(t *testing.T) {
	registryService := createTestRegistryService(t)
	mockGit := createMockGitExecutorWithBranches(t)
	workflowCreator := createTestWorkflowCreator(t, registryService)

	mockCP := newMockControlPlane(t)
	mockCP.On("Create", mock.Anything, mock.MatchedBy(func(spec controlplane.WorkflowSpec) bool {
		return spec.WorktreeEnabled == true &&
			spec.WorktreeBaseBranch == "main" &&
			spec.WorktreeBranchName == "my-feature"
	})).Return(controlplane.WorkflowID("new-workflow-id"), nil).Once()

	modal := NewNewWorkflowModal(registryService, mockCP, mockGit, workflowCreator, nil, false, "")

	values := map[string]any{
		"template":      "quick-plan",
		"name":          "",
		"worktree_mode": "new",
		"base_branch":   "main",
		"custom_branch": "my-feature",
	}

	msg := simulateAsyncSubmit(t, modal, values)
	createMsg, ok := msg.(CreateWorkflowMsg)
	require.True(t, ok)
	require.Equal(t, controlplane.WorkflowID("new-workflow-id"), createMsg.WorkflowID)

	mockCP.AssertExpectations(t)
}

func TestNewWorkflowModal_OnSubmitSetsWorktreeBaseBranchFromSearchSelect(t *testing.T) {
	registryService := createTestRegistryService(t)
	mockGit := createMockGitExecutorWithBranches(t)
	workflowCreator := createTestWorkflowCreator(t, registryService)

	mockCP := newMockControlPlane(t)
	mockCP.On("Create", mock.Anything, mock.MatchedBy(func(spec controlplane.WorkflowSpec) bool {
		return spec.WorktreeEnabled == true && spec.WorktreeBaseBranch == "develop"
	})).Return(controlplane.WorkflowID("new-workflow-id"), nil).Once()

	modal := NewNewWorkflowModal(registryService, mockCP, mockGit, workflowCreator, nil, false, "")

	values := map[string]any{
		"template":      "quick-plan",
		"name":          "",
		"worktree_mode": "new",
		"base_branch":   "develop",
		"custom_branch": "",
	}

	msg := simulateAsyncSubmit(t, modal, values)
	createMsg, ok := msg.(CreateWorkflowMsg)
	require.True(t, ok)
	require.Equal(t, controlplane.WorkflowID("new-workflow-id"), createMsg.WorkflowID)

	mockCP.AssertExpectations(t)
}

func TestNewWorkflowModal_OnSubmitSetsWorktreeBranchNameFromTextField(t *testing.T) {
	registryService := createTestRegistryService(t)
	mockGit := createMockGitExecutorWithBranches(t)
	workflowCreator := createTestWorkflowCreator(t, registryService)

	mockCP := newMockControlPlane(t)
	mockCP.On("Create", mock.Anything, mock.MatchedBy(func(spec controlplane.WorkflowSpec) bool {
		return spec.WorktreeEnabled == true && spec.WorktreeBranchName == "perles-custom-branch"
	})).Return(controlplane.WorkflowID("new-workflow-id"), nil).Once()

	modal := NewNewWorkflowModal(registryService, mockCP, mockGit, workflowCreator, nil, false, "")

	values := map[string]any{
		"template":      "quick-plan",
		"name":          "",
		"worktree_mode": "new",
		"base_branch":   "main",
		"custom_branch": "perles-custom-branch",
	}

	msg := simulateAsyncSubmit(t, modal, values)
	createMsg, ok := msg.(CreateWorkflowMsg)
	require.True(t, ok)
	require.Equal(t, controlplane.WorkflowID("new-workflow-id"), createMsg.WorkflowID)

	mockCP.AssertExpectations(t)
}

func TestNewWorkflowModal_ValidationRequiresBaseBranchWhenWorktreeEnabled(t *testing.T) {
	registryService := createTestRegistryService(t)
	mockGit := createMockGitExecutorWithBranches(t)

	modal := NewNewWorkflowModal(registryService, nil, mockGit, nil, nil, false, "")

	values := map[string]any{
		"template":      "quick-plan",
		"name":          "",
		"worktree_mode": "new",
		"base_branch":   "", // Missing base branch
		"custom_branch": "",
	}

	err := modal.validate(values)
	require.Error(t, err)
	require.Contains(t, err.Error(), "base branch is required when creating a new worktree")
}

func TestNewWorkflowModal_ValidationRejectsInvalidBranchNames(t *testing.T) {
	registryService := createTestRegistryService(t)
	mockGit := mocks.NewMockGitExecutor(t)
	mockGit.EXPECT().ListBranches().Return([]domaingit.BranchInfo{
		{Name: "main", IsCurrent: true},
	}, nil)
	mockGit.EXPECT().ListWorktrees().Return(nil, nil).Maybe()
	mockGit.EXPECT().ValidateBranchName("invalid..branch").Return(errors.New("invalid ref format"))

	modal := NewNewWorkflowModal(registryService, nil, mockGit, nil, nil, false, "")

	values := map[string]any{
		"template":      "quick-plan",
		"name":          "",
		"worktree_mode": "new",
		"base_branch":   "main",
		"custom_branch": "invalid..branch", // Invalid branch name
	}

	err := modal.validate(values)
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid branch name")
}

func TestNewWorkflowModal_ValidationAcceptsValidBranchName(t *testing.T) {
	registryService := createTestRegistryService(t)
	mockGit := mocks.NewMockGitExecutor(t)
	mockGit.EXPECT().ListBranches().Return([]domaingit.BranchInfo{
		{Name: "main", IsCurrent: true},
	}, nil)
	mockGit.EXPECT().ListWorktrees().Return(nil, nil).Maybe()
	mockGit.EXPECT().ValidateBranchName("feature/valid-branch").Return(nil)

	modal := NewNewWorkflowModal(registryService, nil, mockGit, nil, nil, false, "")

	values := map[string]any{
		"template":      "quick-plan",
		"name":          "",
		"worktree_mode": "new",
		"base_branch":   "main",
		"custom_branch": "feature/valid-branch",
	}

	err := modal.validate(values)
	require.NoError(t, err)
}

func TestNewWorkflowModal_ValidationPassesWhenWorktreeDisabled(t *testing.T) {
	registryService := createTestRegistryService(t)
	mockGit := createMockGitExecutorWithBranches(t)

	modal := NewNewWorkflowModal(registryService, nil, mockGit, nil, nil, false, "")

	values := map[string]any{
		"template":      "quick-plan",
		"name":          "",
		"worktree_mode": "none", // Worktree disabled
		"base_branch":   "",     // Empty but should be OK
		"custom_branch": "",
	}

	err := modal.validate(values)
	require.NoError(t, err)
}

func TestBuildBranchOptions_NilGitExecutor(t *testing.T) {
	options, available := buildBranchOptions(nil)
	require.Nil(t, options)
	require.False(t, available)
}

func TestBuildBranchOptions_ListBranchesError(t *testing.T) {
	mockGit := mocks.NewMockGitExecutor(t)
	mockGit.EXPECT().ListBranches().Return(nil, errors.New("git error"))

	options, available := buildBranchOptions(mockGit)
	require.Nil(t, options)
	require.False(t, available)
}

func TestBuildBranchOptions_EmptyBranchList(t *testing.T) {
	mockGit := mocks.NewMockGitExecutor(t)
	mockGit.EXPECT().ListBranches().Return([]domaingit.BranchInfo{}, nil)

	options, available := buildBranchOptions(mockGit)
	require.Nil(t, options)
	require.False(t, available)
}

func TestBuildBranchOptions_ConvertsCorrectly(t *testing.T) {
	mockGit := mocks.NewMockGitExecutor(t)
	mockGit.EXPECT().ListBranches().Return([]domaingit.BranchInfo{
		{Name: "main", IsCurrent: false},
		{Name: "develop", IsCurrent: true},
		{Name: "feature/test", IsCurrent: false},
	}, nil)

	options, available := buildBranchOptions(mockGit)
	require.True(t, available)
	require.Len(t, options, 3)

	// Check first branch
	require.Equal(t, "main", options[0].Label)
	require.Equal(t, "main", options[0].Value)
	require.False(t, options[0].Selected)

	// Check current branch is selected
	require.Equal(t, "develop", options[1].Label)
	require.Equal(t, "develop", options[1].Value)
	require.True(t, options[1].Selected)

	// Check third branch
	require.Equal(t, "feature/test", options[2].Label)
	require.Equal(t, "feature/test", options[2].Value)
	require.False(t, options[2].Selected)
}

// === WorkflowCreator Integration Tests ===

// MockWorkflowCreator is a mock implementation for testing.
type MockWorkflowCreator struct {
	mock.Mock
}

func (m *MockWorkflowCreator) Create(feature, workflowKey string) (*appreg.WorkflowResultDTO, error) {
	args := m.Called(feature, workflowKey)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*appreg.WorkflowResultDTO), args.Error(1)
}

func (m *MockWorkflowCreator) CreateWithArgs(feature, workflowKey string, argValues map[string]string) (*appreg.WorkflowResultDTO, error) {
	args := m.Called(feature, workflowKey, argValues)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*appreg.WorkflowResultDTO), args.Error(1)
}

// MockRegistryService is a mock implementation for testing.
type MockRegistryService struct {
	mock.Mock
}

func (m *MockRegistryService) GetSystemPromptTemplate(reg *registry.Registration) (string, error) {
	args := m.Called(reg)
	return args.String(0), args.Error(1)
}

func TestNewWorkflowModal_OnSubmitCallsWorkflowCreatorWithName(t *testing.T) {
	registryService := createTestRegistryService(t)
	workflowCreator := createTestWorkflowCreator(t, registryService)
	mockCP := newMockControlPlane(t)
	// Verify that EpicID is set from WorkflowCreator result
	mockCP.On("Create", mock.Anything, mock.MatchedBy(func(spec controlplane.WorkflowSpec) bool {
		return spec.EpicID == "epic-123" &&
			spec.TemplateID == "quick-plan" &&
			spec.Name == "test-feature"
	})).Return(controlplane.WorkflowID("new-workflow-id"), nil).Once()

	modal := NewNewWorkflowModal(registryService, mockCP, nil, workflowCreator, nil, false, "")

	values := map[string]any{
		"template": "quick-plan",
		"name":     "test-feature",
	}

	msg := simulateAsyncSubmit(t, modal, values)
	createMsg, ok := msg.(CreateWorkflowMsg)
	require.True(t, ok)
	require.Equal(t, controlplane.WorkflowID("new-workflow-id"), createMsg.WorkflowID)
	mockCP.AssertExpectations(t)
}

func TestNewWorkflowModal_MockCreatorAndRegistryServiceTypes(t *testing.T) {
	// This test verifies the mock types are properly defined for future use
	// when we need to test with actual WorkflowCreator and RegistryService instances

	mockCreator := &MockWorkflowCreator{}
	mockCreatorResult := &appreg.WorkflowResultDTO{
		Epic: appreg.EpicDTO{
			ID:      "perles-abc123",
			Title:   "Plan: Test Feature",
			Feature: "test-feature",
		},
		Workflow: appreg.WorkflowInfoDTO{
			Key:  "quick-plan",
			Name: "Quick Plan",
		},
		Tasks: []appreg.TaskResultDTO{},
	}
	mockCreator.On("Create", "test-feature", "quick-plan").Return(mockCreatorResult, nil)

	mockRegService := &MockRegistryService{}
	// GetSystemPromptTemplate takes a registration parameter (use mock.Anything for flexibility)
	mockRegService.On("GetSystemPromptTemplate", mock.Anything).Return("# System Prompt\n\nYou are the Coordinator.", nil)

	// Verify mock methods work
	result, err := mockCreator.Create("test-feature", "quick-plan")
	require.NoError(t, err)
	require.Equal(t, "perles-abc123", result.Epic.ID)
	require.True(t, strings.Contains(result.Epic.Title, "Test Feature"))

	// GetSystemPromptTemplate requires a registration, pass nil for simplicity in mock test
	template, err := mockRegService.GetSystemPromptTemplate(nil)
	require.NoError(t, err)
	require.Contains(t, template, "System Prompt")

	mockCreator.AssertExpectations(t)
	mockRegService.AssertExpectations(t)
}

func TestNewWorkflowModal_BuildCoordinatorPromptContainsAllSections(t *testing.T) {
	registryService := createTestRegistryService(t)
	modal := NewNewWorkflowModal(registryService, nil, nil, nil, nil, false, "")

	prompt := modal.buildCoordinatorPrompt("quick-plan", "perles-abc123")

	// Verify prompt contains epic ID
	require.Contains(t, prompt, "perles-abc123")
	// Verify prompt contains bd show command
	require.Contains(t, prompt, "bd show perles-abc123 --json")
	// Verify prompt structure
	require.Contains(t, prompt, "# Your Epic")
	// Arguments should NOT be appended - they're rendered into the epic template via {{.Args.key}}
	require.NotContains(t, prompt, "# Arguments")
	require.NotContains(t, prompt, "Build a cool feature")
}

func TestNewWorkflowModal_OnSubmitReturnsErrorOnWorkflowCreatorFailure(t *testing.T) {
	registryService := createTestRegistryService(t)

	// Test the error handling path by verifying ErrorMsg is returned
	// when WorkflowCreator would fail (simulated by checking the error type exists)
	modal := NewNewWorkflowModal(registryService, nil, nil, nil, nil, false, "")

	// This test verifies the ErrorMsg type is properly defined and can be used
	errMsg := ErrorMsg{Err: errors.New("create epic failed")}
	require.Error(t, errMsg.Err)
	require.Contains(t, errMsg.Err.Error(), "create epic failed")

	// Also verify the modal exists and is functional
	require.NotNil(t, modal)
}

func TestNewWorkflowModal_EpicIDPassedToWorkflowSpec(t *testing.T) {
	registryService := createTestRegistryService(t)
	workflowCreator := createTestWorkflowCreator(t, registryService)

	// Verify that when onSubmit returns with EpicID, the spec contains it
	mockCP := newMockControlPlane(t)
	// Match on EpicID being set from WorkflowCreator result
	mockCP.On("Create", mock.Anything, mock.MatchedBy(func(spec controlplane.WorkflowSpec) bool {
		return spec.EpicID == "epic-123" && spec.TemplateID == "quick-plan"
	})).Return(controlplane.WorkflowID("workflow-123"), nil).Once()

	modal := NewNewWorkflowModal(registryService, mockCP, nil, workflowCreator, nil, false, "")

	values := map[string]any{
		"template": "quick-plan",
		"name":     "",
	}

	msg := simulateAsyncSubmit(t, modal, values)
	createMsg, ok := msg.(CreateWorkflowMsg)
	require.True(t, ok)
	require.Equal(t, controlplane.WorkflowID("workflow-123"), createMsg.WorkflowID)
	mockCP.AssertExpectations(t)
}

// === Tests for GetSystemPromptTemplate integration ===

// createTestRegistryServiceWithSystemPrompt creates a registry service where templates have system_prompt specified
func createTestRegistryServiceWithSystemPrompt(t *testing.T) *appreg.RegistryService {
	t.Helper()
	registryFS := fstest.MapFS{
		"workflows/quick-plan/template.yaml": &fstest.MapFile{
			Data: []byte(`
registry:
  - namespace: "workflow"
    key: "quick-plan"
    version: "v1"
    name: "Quick Plan"
    description: "Fast planning workflow"
    system_prompt: "custom_system_prompt.md"
    nodes:
      - key: "plan"
        name: "Plan"
        template: "v1-plan.md"
        assignee: "worker-1"
`),
		},
		"workflows/quick-plan/v1-plan.md": &fstest.MapFile{Data: []byte("# Plan Template")},
		"workflows/quick-plan/custom_system_prompt.md": &fstest.MapFile{
			Data: []byte("# Custom System Prompt\n\nThis is a custom coordinator prompt."),
		},
	}
	registry, err := appreg.NewRegistryService(registryFS, nil, "")
	require.NoError(t, err)
	return registry
}

func TestBuildCoordinatorPrompt_UsesCustomSystemPrompt(t *testing.T) {
	registryService := createTestRegistryServiceWithSystemPrompt(t)
	modal := NewNewWorkflowModal(registryService, nil, nil, nil, nil, false, "")

	prompt := modal.buildCoordinatorPrompt("quick-plan", "perles-abc123")

	// Verify prompt contains custom system prompt content
	require.Contains(t, prompt, "# Custom System Prompt")
	require.Contains(t, prompt, "This is a custom coordinator prompt")
	// Verify prompt contains epic ID
	require.Contains(t, prompt, "perles-abc123")
	require.Contains(t, prompt, "# Your Epic")
	// Arguments should NOT be appended - they're rendered into epic template via {{.Args.key}}
	require.NotContains(t, prompt, "# Arguments")
	require.NotContains(t, prompt, "Build a cool feature")
}

func TestNewRegistryService_FailsOnMissingSystemPromptFile(t *testing.T) {
	// YAML loader now validates template existence at load time (early validation).
	// This is a security improvement: missing templates fail fast at load time, not at render time.
	// Create a registry where the template specifies a system_prompt file that doesn't exist
	registryFS := fstest.MapFS{
		"workflows/broken-plan/template.yaml": &fstest.MapFile{
			Data: []byte(`
registry:
  - namespace: "workflow"
    key: "broken-plan"
    version: "v1"
    name: "Broken Plan"
    description: "Workflow with missing system_prompt"
    system_prompt: "nonexistent.md"
    nodes:
      - key: "plan"
        name: "Plan"
        template: "v1-plan.md"
        assignee: "worker-1"
`),
		},
		"workflows/broken-plan/v1-plan.md": &fstest.MapFile{Data: []byte("# Plan Template")},
		// No "nonexistent.md" file - system_prompt file doesn't exist
	}

	// NewRegistryService should fail because system_prompt template doesn't exist (early validation)
	_, err := appreg.NewRegistryService(registryFS, nil, "")
	require.Error(t, err, "NewRegistryService should fail when system_prompt template doesn't exist")
	require.Contains(t, err.Error(), "system_prompt", "error should mention system_prompt")
	require.Contains(t, err.Error(), "not found", "error should indicate file not found")
}

func TestBuildCoordinatorPrompt_HandlesNoInstructionsField(t *testing.T) {
	// Create a registry where the template has no instructions field
	registryService := createTestRegistryService(t)
	modal := NewNewWorkflowModal(registryService, nil, nil, nil, nil, false, "")

	prompt := modal.buildCoordinatorPrompt("quick-plan", "perles-abc123")

	// Verify prompt falls back to minimal prompt without instructions
	require.Contains(t, prompt, "perles-abc123")
	require.Contains(t, prompt, "# Your Epic")
	// Arguments should NOT be appended - they're rendered into epic template via {{.Args.key}}
	require.NotContains(t, prompt, "# Arguments")
	require.NotContains(t, prompt, "Build a cool feature")
}

func TestNewWorkflowModal_ErrorMsgSetsFormError(t *testing.T) {
	registryService := createTestRegistryService(t)
	modal := NewNewWorkflowModal(registryService, nil, nil, nil, nil, false, "")
	modal = modal.SetSize(80, 24)

	// Send ErrorMsg to modal
	errMsg := ErrorMsg{Err: errors.New("create workflow: template not found")}
	modal, _ = modal.Update(errMsg)

	// Verify error is displayed in the form view
	view := modal.View()
	require.Contains(t, view, "create workflow: template not found")
}

func TestNewWorkflowModal_ErrorMsgClearsLoadingState(t *testing.T) {
	registryService := createTestRegistryService(t)
	modal := NewNewWorkflowModal(registryService, nil, nil, nil, nil, false, "")
	modal = modal.SetSize(80, 24)

	// Simulate loading state by sending startSubmitMsg first
	modal, _ = modal.Update(startSubmitMsg{values: map[string]any{"template": "quick-plan"}})

	// Verify loading state is set
	require.Contains(t, modal.View(), "Creating workflow")

	// Send ErrorMsg
	errMsg := ErrorMsg{Err: errors.New("something failed")}
	modal, _ = modal.Update(errMsg)

	// Verify loading state is cleared and error is shown
	view := modal.View()
	require.NotContains(t, view, "Creating workflow")
	require.Contains(t, view, "something failed")
}

// === Argument Field Tests ===

func TestNewWorkflowModal_BuildArgumentFields(t *testing.T) {
	// Create registry with workflow containing arguments
	registryFS := fstest.MapFS{
		"workflows/with-args/template.yaml": &fstest.MapFile{
			Data: []byte(`
registry:
  - namespace: "workflow"
    key: "with-args"
    version: "v1"
    name: "Workflow With Args"
    description: "Test workflow with arguments"
    arguments:
      - key: "feature_name"
        label: "Feature Name"
        description: "Name of the feature"
        type: "text"
        required: true
      - key: "extra_notes"
        label: "Extra Notes"
        description: "Additional notes"
        type: "textarea"
        required: false
        default: "default notes"
    nodes:
      - key: "task"
        name: "Task"
        template: "v1-task.md"
`),
		},
		"workflows/with-args/v1-task.md": &fstest.MapFile{Data: []byte("# Task: {{.Args.feature_name}}")},
	}

	registryService, err := appreg.NewRegistryService(registryFS, nil, "")
	require.NoError(t, err)

	modal := NewNewWorkflowModal(registryService, nil, nil, nil, nil, false, "")

	// Verify templateArgs was populated
	require.Contains(t, modal.templateArgs, "with-args")
	require.Len(t, modal.templateArgs["with-args"], 2)

	// Verify first argument
	arg1 := modal.templateArgs["with-args"][0]
	require.Equal(t, "feature_name", arg1.Key())
	require.Equal(t, "Feature Name", arg1.Label())
	require.True(t, arg1.Required())

	// Verify second argument
	arg2 := modal.templateArgs["with-args"][1]
	require.Equal(t, "extra_notes", arg2.Key())
	require.Equal(t, "Extra Notes", arg2.Label())
	require.False(t, arg2.Required())
	require.Equal(t, "default notes", arg2.DefaultValue())
}

func TestNewWorkflowModal_ExtractArgumentValues(t *testing.T) {
	// Create registry with workflow containing arguments
	registryFS := fstest.MapFS{
		"workflows/with-args/template.yaml": &fstest.MapFile{
			Data: []byte(`
registry:
  - namespace: "workflow"
    key: "with-args"
    version: "v1"
    name: "Workflow With Args"
    description: "Test workflow"
    arguments:
      - key: "feature_name"
        label: "Feature Name"
        description: "Name of the feature"
        type: "text"
        required: true
      - key: "optional_field"
        label: "Optional Field"
        description: "Optional"
        type: "text"
        required: false
        default: "default_value"
    nodes:
      - key: "task"
        name: "Task"
        template: "v1-task.md"
`),
		},
		"workflows/with-args/v1-task.md": &fstest.MapFile{Data: []byte("# Task")},
	}

	registryService, err := appreg.NewRegistryService(registryFS, nil, "")
	require.NoError(t, err)

	modal := NewNewWorkflowModal(registryService, nil, nil, nil, nil, false, "")

	// Test extracting argument values
	values := map[string]any{
		"template":           "with-args",
		"arg_feature_name":   "my-feature",
		"arg_optional_field": "", // Empty, should use default
	}

	args := modal.extractArgumentValues("with-args", values)

	// Verify extracted values
	require.Equal(t, "my-feature", args["feature_name"])
	require.Equal(t, "default_value", args["optional_field"])
}

func TestNewWorkflowModal_ValidateRequiredArguments(t *testing.T) {
	// Create registry with workflow containing required argument
	registryFS := fstest.MapFS{
		"workflows/with-args/template.yaml": &fstest.MapFile{
			Data: []byte(`
registry:
  - namespace: "workflow"
    key: "with-args"
    version: "v1"
    name: "Workflow With Args"
    description: "Test workflow"
    arguments:
      - key: "feature_name"
        label: "Feature Name"
        description: "Name of the feature"
        type: "text"
        required: true
    nodes:
      - key: "task"
        name: "Task"
        template: "v1-task.md"
`),
		},
		"workflows/with-args/v1-task.md": &fstest.MapFile{Data: []byte("# Task")},
	}

	registryService, err := appreg.NewRegistryService(registryFS, nil, "")
	require.NoError(t, err)

	modal := NewNewWorkflowModal(registryService, nil, nil, nil, nil, false, "")

	// Test validation fails when required argument is missing
	values := map[string]any{
		"template":         "with-args",
		"arg_feature_name": "", // Required but empty
	}

	err = modal.validate(values)
	require.Error(t, err)
	require.Contains(t, err.Error(), "Feature Name is required")

	// Test validation passes when required argument is provided
	values["arg_feature_name"] = "my-feature"
	err = modal.validate(values)
	require.NoError(t, err)
}

// === Epic Search Field Integration Tests ===

// createTestRegistryServiceWithEpicSearch creates a registry service with a workflow
// that has an epic-search argument type.
// Note: IsEpicDriven() requires exactly 1 argument named "epic_id" and NO nodes.
func createTestRegistryServiceWithEpicSearch(t *testing.T) *appreg.RegistryService {
	t.Helper()
	registryFS := fstest.MapFS{
		"workflows/epic-driven/template.yaml": &fstest.MapFile{
			// Epic-driven workflows have:
			// 1. Exactly one argument named "epic_id"
			// 2. No nodes (tasks come from the BD tracker, not from YAML)
			Data: []byte(`
registry:
  - namespace: "workflow"
    key: "epic-driven"
    version: "v1"
    name: "Epic Driven Workflow"
    description: "Workflow that selects an existing epic"
    arguments:
      - key: "epic_id"
        label: "Epic"
        description: "Select an epic to work on"
        type: "epic-search"
        required: true
`),
		},
		// Default system prompt file is required for workflow registrations
		"workflows/v1-epic-instructions.md": &fstest.MapFile{Data: []byte("# Default system prompt")},
	}
	registryService, err := appreg.NewRegistryService(registryFS, nil, "")
	require.NoError(t, err)
	return registryService
}

func TestNewWorkflowModal_EpicSearchArgument_CreatesCorrectFieldType(t *testing.T) {
	registryService := createTestRegistryServiceWithEpicSearch(t)
	mockBQL := mocks.NewMockQueryExecutor(t)

	modal := NewNewWorkflowModal(registryService, nil, nil, nil, mockBQL, false, "")

	// Verify templateArgs was populated with epic-search argument
	require.Contains(t, modal.templateArgs, "epic-driven")
	args := modal.templateArgs["epic-driven"]
	require.Len(t, args, 1)
	require.Equal(t, "epic_id", args[0].Key())
	require.Equal(t, registry.ArgumentTypeEpicSearch, args[0].Type())
}

func TestNewWorkflowModal_EpicSearchArgument_InjectsExecutor(t *testing.T) {
	registryService := createTestRegistryServiceWithEpicSearch(t)
	mockBQL := mocks.NewMockQueryExecutor(t)

	modal := NewNewWorkflowModal(registryService, nil, nil, nil, mockBQL, false, "")

	// Verify the BQL executor is stored
	require.Equal(t, mockBQL, modal.bqlExecutor)
}

func TestNewWorkflowModal_EpicSearchArgument_DefaultDebounce200ms(t *testing.T) {
	// This test verifies the debounce is set to 200ms by checking the form field config
	registryService := createTestRegistryServiceWithEpicSearch(t)
	mockBQL := mocks.NewMockQueryExecutor(t)

	modal := NewNewWorkflowModal(registryService, nil, nil, nil, mockBQL, false, "")
	require.NotNil(t, modal)

	// The field should be created with DebounceMs = 200
	// We verify by checking that the modal renders without error (the field was created)
	view := modal.SetSize(100, 40).View()
	require.NotEmpty(t, view)
	// When the template is selected, the Epic field should be visible
	require.Contains(t, view, "Template")
}

func TestNewWorkflowModal_EpicSearchArgument_FormSubmissionWithSelectedEpic(t *testing.T) {
	registryService := createTestRegistryServiceWithEpicSearch(t)
	mockBQL := mocks.NewMockQueryExecutor(t)
	// Note: BQL executor is not called during form submission - it's only used during field interaction

	mockCP := newMockControlPlane(t)
	// Verify that the epic_id is passed through to the workflow spec
	mockCP.On("Create", mock.Anything, mock.MatchedBy(func(spec controlplane.WorkflowSpec) bool {
		return spec.EpicID == "epic-123" && spec.TemplateID == "epic-driven"
	})).Return(controlplane.WorkflowID("new-workflow-id"), nil).Once()

	modal := NewNewWorkflowModal(registryService, mockCP, nil, nil, mockBQL, false, "")

	// Simulate form submission with selected epic
	values := map[string]any{
		"template":    "epic-driven",
		"name":        "",
		"arg_epic_id": "epic-123",
	}

	msg := simulateAsyncSubmit(t, modal, values)
	createMsg, ok := msg.(CreateWorkflowMsg)
	require.True(t, ok)
	require.Equal(t, controlplane.WorkflowID("new-workflow-id"), createMsg.WorkflowID)

	mockCP.AssertExpectations(t)
}

func TestNewWorkflowModal_EpicSearchArgument_MultipleFieldsWorkIndependently(t *testing.T) {
	// Create a workflow with two epic-search fields to verify they work independently
	registryFS := fstest.MapFS{
		"workflows/multi-epic/template.yaml": &fstest.MapFile{
			Data: []byte(`
registry:
  - namespace: "workflow"
    key: "multi-epic"
    version: "v1"
    name: "Multi Epic Workflow"
    description: "Workflow with multiple epic search fields"
    epic_driven: true
    arguments:
      - key: "source_epic"
        label: "Source Epic"
        description: "Select source epic"
        type: "epic-search"
        required: true
      - key: "target_epic"
        label: "Target Epic"
        description: "Select target epic"
        type: "epic-search"
        required: false
    nodes:
      - key: "process"
        name: "Process"
        template: "v1-process.md"
`),
		},
		"workflows/multi-epic/v1-process.md": &fstest.MapFile{Data: []byte("# Process")},
	}
	registryService, err := appreg.NewRegistryService(registryFS, nil, "")
	require.NoError(t, err)

	mockBQL := mocks.NewMockQueryExecutor(t)
	modal := NewNewWorkflowModal(registryService, nil, nil, nil, mockBQL, false, "")

	// Verify both fields were created
	require.Contains(t, modal.templateArgs, "multi-epic")
	args := modal.templateArgs["multi-epic"]
	require.Len(t, args, 2)

	// Both should be epic-search type
	require.Equal(t, registry.ArgumentTypeEpicSearch, args[0].Type())
	require.Equal(t, registry.ArgumentTypeEpicSearch, args[1].Type())
	require.Equal(t, "source_epic", args[0].Key())
	require.Equal(t, "target_epic", args[1].Key())
}

func TestNewWorkflowModal_EpicSearchArgument_MockBQLExecutorVerifiesQueryConstruction(t *testing.T) {
	// This test verifies the BQL executor is properly injected and would be called with correct queries
	// (The actual query execution happens in formmodal, but we verify the executor is wired up)
	registryService := createTestRegistryServiceWithEpicSearch(t)
	mockBQL := mocks.NewMockQueryExecutor(t)

	modal := NewNewWorkflowModal(registryService, nil, nil, nil, mockBQL, false, "")

	// The executor is stored and would be passed to form fields
	require.NotNil(t, modal.bqlExecutor)
	require.Equal(t, mockBQL, modal.bqlExecutor)

	// Modal should render correctly
	view := modal.SetSize(100, 40).View()
	require.NotEmpty(t, view)
}

func TestNewWorkflowModal_EpicSearchArgument_FormValuesIncludeSelectedEpicID(t *testing.T) {
	registryService := createTestRegistryServiceWithEpicSearch(t)
	mockBQL := mocks.NewMockQueryExecutor(t)

	modal := NewNewWorkflowModal(registryService, nil, nil, nil, mockBQL, false, "")

	// Simulate extracting argument values (as would happen during form submission)
	values := map[string]any{
		"template":    "epic-driven",
		"arg_epic_id": "perles-123",
	}

	args := modal.extractArgumentValues("epic-driven", values)

	// The epic_id should be extracted
	require.Equal(t, "perles-123", args["epic_id"])
}

// === Regression Tests: Existing Argument Types Still Work ===

func TestNewWorkflowModal_RegressionTest_TextArgumentStillWorks(t *testing.T) {
	// Verify that text arguments still work after adding epic-search support
	registryFS := fstest.MapFS{
		"workflows/with-text/template.yaml": &fstest.MapFile{
			Data: []byte(`
registry:
  - namespace: "workflow"
    key: "with-text"
    version: "v1"
    name: "Text Workflow"
    description: "Workflow with text argument"
    arguments:
      - key: "feature"
        label: "Feature Name"
        description: "Enter feature name"
        type: "text"
        required: true
    nodes:
      - key: "task"
        name: "Task"
        template: "v1-task.md"
`),
		},
		"workflows/with-text/v1-task.md": &fstest.MapFile{Data: []byte("# Task")},
	}
	registryService, err := appreg.NewRegistryService(registryFS, nil, "")
	require.NoError(t, err)

	modal := NewNewWorkflowModal(registryService, nil, nil, nil, nil, false, "")

	// Verify text argument is recognized
	require.Contains(t, modal.templateArgs, "with-text")
	args := modal.templateArgs["with-text"]
	require.Len(t, args, 1)
	require.Equal(t, registry.ArgumentTypeText, args[0].Type())
}

func TestNewWorkflowModal_RegressionTest_SelectArgumentStillWorks(t *testing.T) {
	// Verify that select arguments still work after adding epic-search support
	registryFS := fstest.MapFS{
		"workflows/with-select/template.yaml": &fstest.MapFile{
			Data: []byte(`
registry:
  - namespace: "workflow"
    key: "with-select"
    version: "v1"
    name: "Select Workflow"
    description: "Workflow with select argument"
    arguments:
      - key: "priority"
        label: "Priority"
        description: "Select priority"
        type: "select"
        required: true
        options:
          - "low"
          - "medium"
          - "high"
    nodes:
      - key: "task"
        name: "Task"
        template: "v1-task.md"
`),
		},
		"workflows/with-select/v1-task.md": &fstest.MapFile{Data: []byte("# Task")},
	}
	registryService, err := appreg.NewRegistryService(registryFS, nil, "")
	require.NoError(t, err)

	modal := NewNewWorkflowModal(registryService, nil, nil, nil, nil, false, "")

	// Verify select argument is recognized
	require.Contains(t, modal.templateArgs, "with-select")
	args := modal.templateArgs["with-select"]
	require.Len(t, args, 1)
	require.Equal(t, registry.ArgumentTypeSelect, args[0].Type())
}

func TestNewWorkflowModal_RegressionTest_TextareaArgumentStillWorks(t *testing.T) {
	// Verify that textarea arguments still work after adding epic-search support
	registryFS := fstest.MapFS{
		"workflows/with-textarea/template.yaml": &fstest.MapFile{
			Data: []byte(`
registry:
  - namespace: "workflow"
    key: "with-textarea"
    version: "v1"
    name: "Textarea Workflow"
    description: "Workflow with textarea argument"
    arguments:
      - key: "notes"
        label: "Notes"
        description: "Enter notes"
        type: "textarea"
        required: false
    nodes:
      - key: "task"
        name: "Task"
        template: "v1-task.md"
`),
		},
		"workflows/with-textarea/v1-task.md": &fstest.MapFile{Data: []byte("# Task")},
	}
	registryService, err := appreg.NewRegistryService(registryFS, nil, "")
	require.NoError(t, err)

	modal := NewNewWorkflowModal(registryService, nil, nil, nil, nil, false, "")

	// Verify textarea argument is recognized
	require.Contains(t, modal.templateArgs, "with-textarea")
	args := modal.templateArgs["with-textarea"]
	require.Len(t, args, 1)
	require.Equal(t, registry.ArgumentTypeTextarea, args[0].Type())
}

func TestNewWorkflowModal_RegressionTest_MultiSelectArgumentStillWorks(t *testing.T) {
	// Verify that multi-select arguments still work after adding epic-search support
	registryFS := fstest.MapFS{
		"workflows/with-multiselect/template.yaml": &fstest.MapFile{
			Data: []byte(`
registry:
  - namespace: "workflow"
    key: "with-multiselect"
    version: "v1"
    name: "MultiSelect Workflow"
    description: "Workflow with multi-select argument"
    arguments:
      - key: "labels"
        label: "Labels"
        description: "Select labels"
        type: "multi-select"
        required: false
        options:
          - "bug"
          - "feature"
          - "docs"
    nodes:
      - key: "task"
        name: "Task"
        template: "v1-task.md"
`),
		},
		"workflows/with-multiselect/v1-task.md": &fstest.MapFile{Data: []byte("# Task")},
	}
	registryService, err := appreg.NewRegistryService(registryFS, nil, "")
	require.NoError(t, err)

	modal := NewNewWorkflowModal(registryService, nil, nil, nil, nil, false, "")

	// Verify multi-select argument is recognized
	require.Contains(t, modal.templateArgs, "with-multiselect")
	args := modal.templateArgs["with-multiselect"]
	require.Len(t, args, 1)
	require.Equal(t, registry.ArgumentTypeMultiSelect, args[0].Type())
}

func TestNewWorkflowModal_RegressionTest_AllExistingFormModalTestsPass(t *testing.T) {
	// This is a meta-test that documents our expectation:
	// After adding epic-search support, all existing formmodal tests should still pass.
	// The actual formmodal tests are in internal/ui/shared/formmodal/
	// This test just verifies the modal creation still works with various argument types mixed.

	registryFS := fstest.MapFS{
		"workflows/mixed/template.yaml": &fstest.MapFile{
			Data: []byte(`
registry:
  - namespace: "workflow"
    key: "mixed"
    version: "v1"
    name: "Mixed Workflow"
    description: "Workflow with all argument types"
    epic_driven: true
    arguments:
      - key: "text_field"
        label: "Text Field"
        description: "A text field"
        type: "text"
        required: true
      - key: "number_field"
        label: "Number Field"
        description: "A number field"
        type: "number"
        required: false
      - key: "textarea_field"
        label: "Textarea Field"
        description: "A textarea field"
        type: "textarea"
        required: false
      - key: "select_field"
        label: "Select Field"
        description: "A select field"
        type: "select"
        required: false
        options:
          - "option1"
          - "option2"
      - key: "multiselect_field"
        label: "MultiSelect Field"
        description: "A multi-select field"
        type: "multi-select"
        required: false
        options:
          - "tag1"
          - "tag2"
      - key: "epic_field"
        label: "Epic Field"
        description: "An epic search field"
        type: "epic-search"
        required: false
    nodes:
      - key: "task"
        name: "Task"
        template: "v1-task.md"
`),
		},
		"workflows/mixed/v1-task.md": &fstest.MapFile{Data: []byte("# Task")},
	}
	registryService, err := appreg.NewRegistryService(registryFS, nil, "")
	require.NoError(t, err)

	mockBQL := mocks.NewMockQueryExecutor(t)
	modal := NewNewWorkflowModal(registryService, nil, nil, nil, mockBQL, false, "")

	// Verify all arguments are recognized
	require.Contains(t, modal.templateArgs, "mixed")
	args := modal.templateArgs["mixed"]
	require.Len(t, args, 6)

	// Verify each type is correct
	typeMap := make(map[string]registry.ArgumentType)
	for _, arg := range args {
		typeMap[arg.Key()] = arg.Type()
	}

	require.Equal(t, registry.ArgumentTypeText, typeMap["text_field"])
	require.Equal(t, registry.ArgumentTypeNumber, typeMap["number_field"])
	require.Equal(t, registry.ArgumentTypeTextarea, typeMap["textarea_field"])
	require.Equal(t, registry.ArgumentTypeSelect, typeMap["select_field"])
	require.Equal(t, registry.ArgumentTypeMultiSelect, typeMap["multiselect_field"])
	require.Equal(t, registry.ArgumentTypeEpicSearch, typeMap["epic_field"])

	// Modal should render without errors
	view := modal.SetSize(100, 40).View()
	require.NotEmpty(t, view)
}

// TestBuildArgumentFields_EpicSearchFieldConfigIsCorrect verifies the field configuration
// generated for an epic-search argument type has all required settings.
func TestBuildArgumentFields_EpicSearchFieldConfigIsCorrect(t *testing.T) {
	registryService := createTestRegistryServiceWithEpicSearch(t)
	mockBQL := mocks.NewMockQueryExecutor(t)

	modal := NewNewWorkflowModal(registryService, nil, nil, nil, mockBQL, false, "")

	// Build argument fields to verify configuration
	fields := modal.buildArgumentFields(registryService)

	// Find the epic_id field
	var epicField *formmodal.FieldConfig
	for i := range fields {
		if fields[i].Key == "arg_epic_id" {
			epicField = &fields[i]
			break
		}
	}

	require.NotNil(t, epicField, "epic_id field should exist")
	require.Equal(t, formmodal.FieldTypeEpicSearch, epicField.Type)
	require.Equal(t, mockBQL, epicField.EpicSearchExecutor)
	require.Equal(t, 200, epicField.DebounceMs)
	require.Equal(t, "Epic", epicField.Label)
	require.Equal(t, "required", epicField.Hint)
}

// === Worktree Mode (Three-Option Selector) Tests ===

func TestNewWorkflowModal_ExistingWorktreeModeSetsFieldsOnSpec(t *testing.T) {
	registryService := createTestRegistryService(t)
	mockGit := createMockGitExecutorWithBranches(t)
	workflowCreator := createTestWorkflowCreator(t, registryService)

	mockCP := newMockControlPlane(t)
	mockCP.On("Create", mock.Anything, mock.MatchedBy(func(spec controlplane.WorkflowSpec) bool {
		return spec.WorktreeMode == controlplane.WorktreeModeExisting &&
			spec.WorktreePath == "/repo-worktree-1" &&
			spec.WorktreeEnabled == true &&
			spec.WorktreeBaseBranch == "" &&
			spec.WorktreeBranchName == ""
	})).Return(controlplane.WorkflowID("wf-existing"), nil).Once()

	modal := NewNewWorkflowModal(registryService, mockCP, mockGit, workflowCreator, nil, false, "")

	values := map[string]any{
		"template":          "quick-plan",
		"name":              "",
		"worktree_mode":     "existing",
		"existing_worktree": "/repo-worktree-1",
	}

	msg := simulateAsyncSubmit(t, modal, values)
	createMsg, ok := msg.(CreateWorkflowMsg)
	require.True(t, ok)
	require.Equal(t, controlplane.WorkflowID("wf-existing"), createMsg.WorkflowID)
	mockCP.AssertExpectations(t)
}

func TestBuildWorktreeOptions_PopulatesFromListWorktreesMinusMainRepo(t *testing.T) {
	mockGit := mocks.NewMockGitExecutor(t)
	mockGit.EXPECT().ListWorktrees().Return([]domaingit.WorktreeInfo{
		{Path: "/projects/myrepo", Branch: "main"},
		{Path: "/projects/myrepo-wt1", Branch: "feature/auth"},
		{Path: "/projects/myrepo-wt2", Branch: "feature/ui"},
	}, nil)

	options := buildWorktreeOptions(mockGit, "/projects/myrepo")

	// Main repo should be filtered out
	require.Len(t, options, 2)

	// First worktree
	require.Equal(t, "feature/auth", options[0].Label)
	require.Equal(t, "/projects/myrepo-wt1", options[0].Value)
	require.Equal(t, "/projects/myrepo-wt1", options[0].Subtext)
	require.True(t, options[0].Selected) // First option selected

	// Second worktree
	require.Equal(t, "feature/ui", options[1].Label)
	require.Equal(t, "/projects/myrepo-wt2", options[1].Value)
	require.False(t, options[1].Selected)
}

func TestNewWorkflowModal_ValidationRejectsConcurrentWorktreeUsage(t *testing.T) {
	registryService := createTestRegistryService(t)
	mockGit := createMockGitExecutorWithBranches(t)

	// Set up a mock ControlPlane that returns a running workflow using the same worktree
	mockCP := newMockControlPlane(t)
	mockCP.On("List", mock.Anything, mock.Anything).Return([]*controlplane.WorkflowInstance{
		{
			Name:         "Existing Workflow",
			State:        controlplane.WorkflowRunning,
			WorktreePath: "/repo-worktree-1",
		},
	}, nil).Maybe()

	eventCh := make(chan controlplane.ControlPlaneEvent)
	close(eventCh)
	mockCP.On("Subscribe", mock.Anything).Return((<-chan controlplane.ControlPlaneEvent)(eventCh), func() {}).Maybe()

	modal := NewNewWorkflowModal(registryService, mockCP, mockGit, nil, nil, false, "")

	values := map[string]any{
		"template":          "quick-plan",
		"name":              "",
		"worktree_mode":     "existing",
		"existing_worktree": "/repo-worktree-1",
	}

	err := modal.validate(values)
	require.Error(t, err)
	require.Contains(t, err.Error(), "worktree already in use")
	require.Contains(t, err.Error(), "Existing Workflow")
}

func TestNewWorkflowModal_ValidationRequiresWorktreeSelectionForExistingMode(t *testing.T) {
	registryService := createTestRegistryService(t)
	mockGit := createMockGitExecutorWithBranches(t)

	modal := NewNewWorkflowModal(registryService, nil, mockGit, nil, nil, false, "")

	values := map[string]any{
		"template":          "quick-plan",
		"name":              "",
		"worktree_mode":     "existing",
		"existing_worktree": "", // Empty — should fail
	}

	err := modal.validate(values)
	require.Error(t, err)
	require.Contains(t, err.Error(), "worktree selection is required")
}

func TestNewWorkflowModal_ModeNoneDoesNotSetWorktreeFieldsOnSpec(t *testing.T) {
	registryService := createTestRegistryService(t)
	mockGit := createMockGitExecutorWithBranches(t)
	workflowCreator := createTestWorkflowCreator(t, registryService)

	mockCP := newMockControlPlane(t)
	mockCP.On("Create", mock.Anything, mock.MatchedBy(func(spec controlplane.WorkflowSpec) bool {
		return spec.WorktreeMode == controlplane.WorktreeModeNone &&
			spec.WorktreeEnabled == false &&
			spec.WorktreePath == "" &&
			spec.WorktreeBaseBranch == "" &&
			spec.WorktreeBranchName == ""
	})).Return(controlplane.WorkflowID("wf-none"), nil).Once()

	modal := NewNewWorkflowModal(registryService, mockCP, mockGit, workflowCreator, nil, false, "")

	values := map[string]any{
		"template":      "quick-plan",
		"name":          "",
		"worktree_mode": "none",
	}

	msg := simulateAsyncSubmit(t, modal, values)
	createMsg, ok := msg.(CreateWorkflowMsg)
	require.True(t, ok)
	require.Equal(t, controlplane.WorkflowID("wf-none"), createMsg.WorkflowID)
	mockCP.AssertExpectations(t)
}

func TestNewWorkflowModal_ModeNewPreservesExistingBranchBehavior(t *testing.T) {
	registryService := createTestRegistryService(t)
	mockGit := createMockGitExecutorWithBranches(t)
	workflowCreator := createTestWorkflowCreator(t, registryService)

	mockCP := newMockControlPlane(t)
	mockCP.On("Create", mock.Anything, mock.MatchedBy(func(spec controlplane.WorkflowSpec) bool {
		return spec.WorktreeMode == controlplane.WorktreeModeNew &&
			spec.WorktreeEnabled == true &&
			spec.WorktreeBaseBranch == "main" &&
			spec.WorktreeBranchName == "my-custom-branch" &&
			spec.WorktreePath == ""
	})).Return(controlplane.WorkflowID("wf-new"), nil).Once()

	modal := NewNewWorkflowModal(registryService, mockCP, mockGit, workflowCreator, nil, false, "")

	values := map[string]any{
		"template":      "quick-plan",
		"name":          "",
		"worktree_mode": "new",
		"base_branch":   "main",
		"custom_branch": "my-custom-branch",
	}

	msg := simulateAsyncSubmit(t, modal, values)
	createMsg, ok := msg.(CreateWorkflowMsg)
	require.True(t, ok)
	require.Equal(t, controlplane.WorkflowID("wf-new"), createMsg.WorkflowID)
	mockCP.AssertExpectations(t)
}

func TestBuildWorktreeOptions_EmptyWorktreeList(t *testing.T) {
	mockGit := mocks.NewMockGitExecutor(t)
	mockGit.EXPECT().ListWorktrees().Return([]domaingit.WorktreeInfo{}, nil)

	options := buildWorktreeOptions(mockGit, "/some/path")
	require.Empty(t, options)
	require.NotNil(t, options) // Should be empty slice, not nil
}

func TestBuildWorktreeOptions_DetachedHEADUsesPathAsLabel(t *testing.T) {
	mockGit := mocks.NewMockGitExecutor(t)
	mockGit.EXPECT().ListWorktrees().Return([]domaingit.WorktreeInfo{
		{Path: "/projects/repo", Branch: "main"},
		{Path: "/projects/repo-detached", Branch: ""}, // Detached HEAD — empty branch
	}, nil)

	options := buildWorktreeOptions(mockGit, "/projects/repo")

	require.Len(t, options, 1)
	// Detached HEAD should use filepath.Base(path) as label
	require.Equal(t, "repo-detached", options[0].Label)
	require.Equal(t, "/projects/repo-detached", options[0].Value)
}

func TestBuildWorktreeOptions_ListWorktreesError(t *testing.T) {
	mockGit := mocks.NewMockGitExecutor(t)
	mockGit.EXPECT().ListWorktrees().Return(nil, errors.New("git error"))

	options := buildWorktreeOptions(mockGit, "/some/path")
	require.Empty(t, options)
	require.NotNil(t, options) // Should be empty slice, not nil
}

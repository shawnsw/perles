package dashboard

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	zone "github.com/lrstanley/bubblezone"
	"github.com/stretchr/testify/require"

	"github.com/zjrosen/perles/internal/orchestration/controlplane"
)

func requireDashboardActionZoneInfo(t *testing.T, zoneID string, render func()) *zone.ZoneInfo {
	t.Helper()

	var z *zone.ZoneInfo
	for retries := 0; retries < 10; retries++ {
		render()
		z = zone.Get(zoneID)
		if z != nil && !z.IsZero() {
			return z
		}
		time.Sleep(time.Millisecond)
	}

	require.NotNil(t, z, "zone %q should be registered", zoneID)
	require.False(t, z.IsZero(), "zone %q should not be zero", zoneID)
	return z
}

func createDashboardCoordinatorTestModel(t *testing.T) Model {
	t.Helper()

	workflows := []*controlplane.WorkflowInstance{
		createTestWorkflow("wf-1", "Workflow 1", controlplane.WorkflowRunning),
	}

	m, _ := createTestModel(t, workflows)
	m = m.SetSize(120, 40).(Model)

	panel := NewCoordinatorPanel(false, true, true, nil)
	panel.SetWorkflow(workflows[0].ID, nil)
	m.coordinatorPanel = panel
	m.showCoordinatorPanel = true
	m.focus = FocusTable
	m.updateComponentFocusStates()

	return m
}

func TestDashboard_MouseClick_HeaderActionsMaximizeAndRestorePanes(t *testing.T) {
	tests := []struct {
		name       string
		build      func(t *testing.T) Model
		zoneID     string
		pane       dashboardPane
		assertPane func(t *testing.T, m Model)
	}{
		{
			name: "workflow table",
			build: func(t *testing.T) Model {
				m, _ := createTestModel(t, []*controlplane.WorkflowInstance{
					createTestWorkflow("wf-1", "Workflow 1", controlplane.WorkflowRunning),
				})
				return m
			},
			zoneID: zoneWorkflowAction,
			pane:   dashboardPaneWorkflowTable,
			assertPane: func(t *testing.T, m Model) {
				require.Equal(t, FocusTable, m.focus, "workflow table should take focus")
			},
		},
		{
			name: "epic tree",
			build: func(t *testing.T) Model {
				m := createEpicTreeTestModelWithTree(t)
				m.focus = FocusTable
				m.epicViewFocus = EpicFocusDetails
				m.updateComponentFocusStates()
				return m
			},
			zoneID: zoneEpicTreeAction,
			pane:   dashboardPaneEpicTree,
			assertPane: func(t *testing.T, m Model) {
				require.Equal(t, FocusEpicView, m.focus, "epic tree should take epic focus")
				require.Equal(t, EpicFocusTree, m.epicViewFocus, "epic tree sub-focus should be selected")
			},
		},
		{
			name: "epic details",
			build: func(t *testing.T) Model {
				m := createEpicTreeTestModelWithTree(t)
				m.focus = FocusTable
				m.epicViewFocus = EpicFocusTree
				m.updateComponentFocusStates()
				return m
			},
			zoneID: zoneEpicDetailsAction,
			pane:   dashboardPaneEpicDetails,
			assertPane: func(t *testing.T, m Model) {
				require.Equal(t, FocusEpicView, m.focus, "epic details should take epic focus")
				require.Equal(t, EpicFocusDetails, m.epicViewFocus, "epic details sub-focus should be selected")
			},
		},
		{
			name:   "coordinator content",
			build:  createDashboardCoordinatorTestModel,
			zoneID: zoneCoordinatorContentAction,
			pane:   dashboardPaneCoordinatorContent,
			assertPane: func(t *testing.T, m Model) {
				require.Equal(t, FocusCoordinator, m.focus, "coordinator content should take coordinator focus")
				require.NotNil(t, m.coordinatorPanel, "coordinator panel should remain available")
				require.True(t, m.coordinatorPanel.IsFocused(), "coordinator panel should be focused")
			},
		},
		{
			name:   "coordinator input",
			build:  createDashboardCoordinatorTestModel,
			zoneID: zoneCoordinatorInputAction,
			pane:   dashboardPaneCoordinatorInput,
			assertPane: func(t *testing.T, m Model) {
				require.Equal(t, FocusCoordinator, m.focus, "coordinator input should take coordinator focus")
				require.NotNil(t, m.coordinatorPanel, "coordinator panel should remain available")
				require.True(t, m.coordinatorPanel.IsFocused(), "coordinator panel should be focused")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := tt.build(t)

			_ = m.View()
			z := requireDashboardActionZoneInfo(t, tt.zoneID, func() { _ = m.View() })

			controller, cmd := m.Update(tea.MouseMsg{
				X:      z.StartX + 1,
				Y:      z.StartY,
				Button: tea.MouseButtonLeft,
				Action: tea.MouseActionRelease,
			})

			require.Nil(t, cmd, "header action click should not emit a command")
			m = controller.(Model)
			require.Equal(t, tt.pane, m.maximizedPane, "clicked pane should become fullscreen")
			tt.assertPane(t, m)

			_ = m.View()
			z = requireDashboardActionZoneInfo(t, tt.zoneID, func() { _ = m.View() })

			controller, cmd = m.Update(tea.MouseMsg{
				X:      z.StartX + 1,
				Y:      z.StartY,
				Button: tea.MouseButtonLeft,
				Action: tea.MouseActionRelease,
			})

			require.Nil(t, cmd, "restore click should not emit a command")
			m = controller.(Model)
			require.Equal(t, dashboardPaneNone, m.maximizedPane, "second click should restore the normal layout")
		})
	}
}

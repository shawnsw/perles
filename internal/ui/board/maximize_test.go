package board

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	zone "github.com/lrstanley/bubblezone"
	"github.com/stretchr/testify/require"

	"github.com/zjrosen/perles/internal/config"
)

func requireBoardActionZoneInfo(t *testing.T, zoneID string, render func()) *zone.ZoneInfo {
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

func TestBoard_MouseClick_HeaderActionMaximizesAndRestoresColumn(t *testing.T) {
	m := NewFromViews(config.DefaultViews(), nil, nil).SetSize(120, 30)

	actionZoneID := makeHeaderActionZoneID(0)
	_ = m.View()
	z := requireBoardActionZoneInfo(t, actionZoneID, func() { _ = m.View() })

	m, cmd := m.Update(tea.MouseMsg{
		X:      z.StartX + 1,
		Y:      z.StartY,
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionRelease,
	})

	require.Nil(t, cmd, "header action click should not emit a command")
	require.Equal(t, 0, m.MaximizedColumn(), "clicked column should become fullscreen")
	require.Equal(t, 0, m.FocusedColumn(), "fullscreen column should receive focus")
	require.Equal(t, []int{0}, m.visibleColumnIndices(), "only the fullscreen column should remain visible")

	_ = m.View()
	z = requireBoardActionZoneInfo(t, actionZoneID, func() { _ = m.View() })

	m, cmd = m.Update(tea.MouseMsg{
		X:      z.StartX + 1,
		Y:      z.StartY,
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionRelease,
	})

	require.Nil(t, cmd, "restore click should not emit a command")
	require.Equal(t, -1, m.MaximizedColumn(), "second click should restore the normal layout")
	require.Len(t, m.visibleColumnIndices(), m.ColCount(), "all columns should be visible again")
}

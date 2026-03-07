package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

func TestSaveColumns_CreatesNewFile(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, ".perles.yaml")

	columns := []ColumnConfig{
		{Name: "Test", Query: "status = open", Color: "#FF0000"},
	}

	err := SaveColumns(configPath, columns)
	require.NoError(t, err)

	// Verify file exists
	_, err = os.Stat(configPath)
	require.NoError(t, err)

	// Verify content
	data, err := os.ReadFile(configPath)
	require.NoError(t, err)
	require.Contains(t, string(data), "name: Test")
	require.Contains(t, string(data), "query: status = open")
	require.Contains(t, string(data), "color: '#FF0000'")
}

func TestSaveColumns_PreservesOtherConfig(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, ".perles.yaml")

	// Create initial config with various settings
	initial := `theme:
  highlight: "#FF0000"
  subtle: "#888888"
ui:
  show_counts: false
`
	err := os.WriteFile(configPath, []byte(initial), 0644)
	require.NoError(t, err)

	// Save new columns
	columns := []ColumnConfig{
		{Name: "Ready", Query: "status = open and ready = true", Color: "#00FF00"},
	}
	err = SaveColumns(configPath, columns)
	require.NoError(t, err)

	// Verify other settings preserved
	data, err := os.ReadFile(configPath)
	require.NoError(t, err)
	content := string(data)

	require.Contains(t, content, "highlight:")
	require.Contains(t, content, "show_counts: false")
	// And columns are there
	require.Contains(t, content, "name: Ready")
}

func TestSaveColumns_Roundtrip(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, ".perles.yaml")

	original := []ColumnConfig{
		{
			Name:  "Ready",
			Query: "status = open and ready = true",
			Color: "#73F59F",
		},
		{
			Name:  "In Progress",
			Query: "status = in_progress",
			Color: "#54A0FF",
		},
	}

	// Save
	err := SaveColumns(configPath, original)
	require.NoError(t, err)

	// Load back using Viper (now stored under views[0].columns)
	v := viper.New()
	v.SetConfigFile(configPath)
	err = v.ReadInConfig()
	require.NoError(t, err)

	var loaded []ViewConfig
	err = v.UnmarshalKey("views", &loaded)
	require.NoError(t, err)

	// Verify roundtrip
	require.Len(t, loaded, 1)
	require.Len(t, loaded[0].Columns, 2)

	require.Equal(t, original[0].Name, loaded[0].Columns[0].Name)
	require.Equal(t, original[0].Query, loaded[0].Columns[0].Query)
	require.Equal(t, original[0].Color, loaded[0].Columns[0].Color)

	require.Equal(t, original[1].Name, loaded[0].Columns[1].Name)
	require.Equal(t, original[1].Query, loaded[0].Columns[1].Query)
}

func TestSaveColumns_AtomicWrite(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, ".perles.yaml")

	// Create initial file
	initial := []ColumnConfig{{Name: "Initial", Query: "status = open"}}
	err := SaveColumns(configPath, initial)
	require.NoError(t, err)

	// Save again - should work without leaving temp files
	columns := []ColumnConfig{{Name: "Updated", Query: "status = closed"}}
	err = SaveColumns(configPath, columns)
	require.NoError(t, err)

	// Check no temp files left behind
	entries, err := os.ReadDir(tempDir)
	require.NoError(t, err)

	for _, entry := range entries {
		require.False(t, filepath.Ext(entry.Name()) == ".tmp", "temp file left behind: %s", entry.Name())
	}

	// Verify content
	data, err := os.ReadFile(configPath)
	require.NoError(t, err)
	require.Contains(t, string(data), "name: Updated")
}

func TestSaveColumns_CreatesDirectory(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "subdir", "nested", ".perles.yaml")

	columns := []ColumnConfig{{Name: "Test", Query: "status = open"}}
	err := SaveColumns(configPath, columns)
	require.NoError(t, err)

	// Verify file exists
	_, err = os.Stat(configPath)
	require.NoError(t, err)
}

func TestSaveColumns_OmitsEmptyFields(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, ".perles.yaml")

	// Column with minimal fields (name and query are required)
	columns := []ColumnConfig{
		{Name: "Minimal", Query: "status = open"},
	}

	err := SaveColumns(configPath, columns)
	require.NoError(t, err)

	data, err := os.ReadFile(configPath)
	require.NoError(t, err)
	content := string(data)

	// Should have name and query
	require.Contains(t, content, "name: Minimal")
	require.Contains(t, content, "query: status = open")

	// Should NOT have empty color
	require.NotContains(t, content, "color:")
}

func TestSwapColumnsInView(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, ".perles.yaml")

	columns := []ColumnConfig{
		{Name: "Blocked", Query: "blocked = true", Color: "#FF0000"},
		{Name: "Ready", Query: "ready = true", Color: "#00FF00"},
		{Name: "Done", Query: "status = closed", Color: "#0000FF"},
	}

	// Save initial columns
	err := SaveColumns(configPath, columns)
	require.NoError(t, err)

	// Swap first and second columns
	err = SwapColumnsInView(configPath, 0, 0, 1, columns, nil)
	require.NoError(t, err)

	// Load and verify (now stored under views[0].columns)
	v := viper.New()
	v.SetConfigFile(configPath)
	err = v.ReadInConfig()
	require.NoError(t, err)

	var loaded []ViewConfig
	err = v.UnmarshalKey("views", &loaded)
	require.NoError(t, err)

	require.Len(t, loaded, 1)
	require.Len(t, loaded[0].Columns, 3)
	require.Equal(t, "Ready", loaded[0].Columns[0].Name)
	require.Equal(t, "Blocked", loaded[0].Columns[1].Name)
	require.Equal(t, "Done", loaded[0].Columns[2].Name)
}

func TestSwapColumnsInView_SameIndex(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, ".perles.yaml")

	columns := []ColumnConfig{
		{Name: "One", Query: "status = open"},
		{Name: "Two", Query: "status = closed"},
	}

	// Save initial
	err := SaveColumns(configPath, columns)
	require.NoError(t, err)

	// Swap same index should be a no-op
	err = SwapColumnsInView(configPath, 0, 1, 1, columns, nil)
	require.NoError(t, err)

	// Load and verify nothing changed (now stored under views[0].columns)
	v := viper.New()
	v.SetConfigFile(configPath)
	err = v.ReadInConfig()
	require.NoError(t, err)

	var loaded []ViewConfig
	err = v.UnmarshalKey("views", &loaded)
	require.NoError(t, err)

	require.Len(t, loaded, 1)
	require.Len(t, loaded[0].Columns, 2)
	require.Equal(t, "One", loaded[0].Columns[0].Name)
	require.Equal(t, "Two", loaded[0].Columns[1].Name)
}

func TestSwapColumnsInView_OutOfRange(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, ".perles.yaml")

	columns := []ColumnConfig{
		{Name: "One", Query: "status = open"},
		{Name: "Two", Query: "status = closed"},
	}

	// Index too high
	err := SwapColumnsInView(configPath, 0, 0, 5, columns, nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "out of range")

	// Index negative
	err = SwapColumnsInView(configPath, 0, -1, 1, columns, nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "out of range")
}

func TestAddView(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, ".perles.yaml")

	// Create existing views
	existingViews := []ViewConfig{
		{
			Name: "Default",
			Columns: []ColumnConfig{
				{Name: "Open", Query: "status = open"},
			},
		},
	}

	// Save initial views
	err := SaveViews(configPath, existingViews)
	require.NoError(t, err)

	// Add new view
	newView := ViewConfig{
		Name: "Bugs",
		Columns: []ColumnConfig{
			{Name: "All Bugs", Query: "type = bug", Color: "#FF0000"},
		},
	}
	err = AddView(configPath, newView, existingViews)
	require.NoError(t, err)

	// Load and verify
	v := viper.New()
	v.SetConfigFile(configPath)
	err = v.ReadInConfig()
	require.NoError(t, err)

	var loaded []ViewConfig
	err = v.UnmarshalKey("views", &loaded)
	require.NoError(t, err)

	require.Len(t, loaded, 2)
	require.Equal(t, "Default", loaded[0].Name)
	require.Equal(t, "Bugs", loaded[1].Name)
	require.Len(t, loaded[1].Columns, 1)
	require.Equal(t, "All Bugs", loaded[1].Columns[0].Name)
	require.Equal(t, "type = bug", loaded[1].Columns[0].Query)
	require.Equal(t, "#FF0000", loaded[1].Columns[0].Color)
}

func TestAddView_Empty(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, ".perles.yaml")

	// No existing views
	var existingViews []ViewConfig

	// Add first view
	newView := ViewConfig{
		Name: "First View",
		Columns: []ColumnConfig{
			{Name: "All Issues", Query: "status != closed"},
		},
	}
	err := AddView(configPath, newView, existingViews)
	require.NoError(t, err)

	// Load and verify
	v := viper.New()
	v.SetConfigFile(configPath)
	err = v.ReadInConfig()
	require.NoError(t, err)

	var loaded []ViewConfig
	err = v.UnmarshalKey("views", &loaded)
	require.NoError(t, err)

	require.Len(t, loaded, 1)
	require.Equal(t, "First View", loaded[0].Name)
	require.Len(t, loaded[0].Columns, 1)
	require.Equal(t, "All Issues", loaded[0].Columns[0].Name)
}

func TestDeleteView(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, ".perles.yaml")

	// Create views
	views := []ViewConfig{
		{
			Name: "Default",
			Columns: []ColumnConfig{
				{Name: "Open", Query: "status = open"},
			},
		},
		{
			Name: "Bugs",
			Columns: []ColumnConfig{
				{Name: "All Bugs", Query: "type = bug", Color: "#FF0000"},
			},
		},
		{
			Name: "Features",
			Columns: []ColumnConfig{
				{Name: "All Features", Query: "type = feature"},
			},
		},
	}

	// Save initial views
	err := SaveViews(configPath, views)
	require.NoError(t, err)

	// Delete the middle view (Bugs)
	err = DeleteView(configPath, 1, views)
	require.NoError(t, err)

	// Load and verify
	v := viper.New()
	v.SetConfigFile(configPath)
	err = v.ReadInConfig()
	require.NoError(t, err)

	var loaded []ViewConfig
	err = v.UnmarshalKey("views", &loaded)
	require.NoError(t, err)

	require.Len(t, loaded, 2)
	require.Equal(t, "Default", loaded[0].Name)
	require.Equal(t, "Features", loaded[1].Name)
}

func TestDeleteView_FirstView(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, ".perles.yaml")

	// Create two views
	views := []ViewConfig{
		{
			Name: "First",
			Columns: []ColumnConfig{
				{Name: "Col1", Query: "status = open"},
			},
		},
		{
			Name: "Second",
			Columns: []ColumnConfig{
				{Name: "Col2", Query: "status = closed"},
			},
		},
	}

	// Save initial views
	err := SaveViews(configPath, views)
	require.NoError(t, err)

	// Delete the first view (index 0)
	err = DeleteView(configPath, 0, views)
	require.NoError(t, err)

	// Load and verify
	v := viper.New()
	v.SetConfigFile(configPath)
	err = v.ReadInConfig()
	require.NoError(t, err)

	var loaded []ViewConfig
	err = v.UnmarshalKey("views", &loaded)
	require.NoError(t, err)

	require.Len(t, loaded, 1)
	require.Equal(t, "Second", loaded[0].Name)
}

func TestDeleteView_LastView(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, ".perles.yaml")

	// Create only one view
	views := []ViewConfig{
		{
			Name: "Only",
			Columns: []ColumnConfig{
				{Name: "Col", Query: "status = open"},
			},
		},
	}

	err := DeleteView(configPath, 0, views)
	require.Error(t, err)
	require.Contains(t, err.Error(), "cannot delete the only view")
}

func TestDeleteView_OutOfRange(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, ".perles.yaml")

	views := []ViewConfig{
		{
			Name: "One",
			Columns: []ColumnConfig{
				{Name: "Col1", Query: "status = open"},
			},
		},
		{
			Name: "Two",
			Columns: []ColumnConfig{
				{Name: "Col2", Query: "status = closed"},
			},
		},
	}

	// Index too high
	err := DeleteView(configPath, 5, views)
	require.Error(t, err)
	require.Contains(t, err.Error(), "out of range")

	// Index negative
	err = DeleteView(configPath, -1, views)
	require.Error(t, err)
	require.Contains(t, err.Error(), "out of range")
}

// Tests for InsertColumnInView

func TestInsertColumnInView_InsertsAtPosition0(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, ".perles.yaml")

	views := []ViewConfig{
		{
			Name: "Default",
			Columns: []ColumnConfig{
				{Name: "Existing1", Query: "status = open"},
				{Name: "Existing2", Query: "status = closed"},
			},
		},
	}

	// Save initial views
	err := SaveViews(configPath, views)
	require.NoError(t, err)

	// Insert at position 0
	newCol := ColumnConfig{Name: "First", Query: "priority = 0", Color: "#FF0000"}
	err = InsertColumnInView(configPath, 0, 0, newCol, views)
	require.NoError(t, err)

	// Load and verify
	v := viper.New()
	v.SetConfigFile(configPath)
	err = v.ReadInConfig()
	require.NoError(t, err)

	var loaded []ViewConfig
	err = v.UnmarshalKey("views", &loaded)
	require.NoError(t, err)

	require.Len(t, loaded, 1)
	require.Len(t, loaded[0].Columns, 3)
	require.Equal(t, "First", loaded[0].Columns[0].Name)
	require.Equal(t, "#FF0000", loaded[0].Columns[0].Color)
	require.Equal(t, "Existing1", loaded[0].Columns[1].Name)
	require.Equal(t, "Existing2", loaded[0].Columns[2].Name)
}

func TestInsertColumnInView_PreservesExistingColumns(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, ".perles.yaml")

	views := []ViewConfig{
		{
			Name: "Default",
			Columns: []ColumnConfig{
				{Name: "Blocked", Query: "blocked = true", Color: "#FF0000"},
				{Name: "Ready", Query: "ready = true", Color: "#00FF00"},
				{Name: "Done", Query: "status = closed", Color: "#0000FF"},
			},
		},
	}

	// Save initial views
	err := SaveViews(configPath, views)
	require.NoError(t, err)

	// Insert at position 0
	newCol := ColumnConfig{Name: "New Column", Query: "new = true", Color: "#AABBCC"}
	err = InsertColumnInView(configPath, 0, 0, newCol, views)
	require.NoError(t, err)

	// Load and verify existing columns are unchanged
	v := viper.New()
	v.SetConfigFile(configPath)
	err = v.ReadInConfig()
	require.NoError(t, err)

	var loaded []ViewConfig
	err = v.UnmarshalKey("views", &loaded)
	require.NoError(t, err)

	require.Len(t, loaded, 1)
	require.Len(t, loaded[0].Columns, 4)

	// Verify original columns preserved with correct data
	require.Equal(t, "New Column", loaded[0].Columns[0].Name)
	require.Equal(t, "Blocked", loaded[0].Columns[1].Name)
	require.Equal(t, "#FF0000", loaded[0].Columns[1].Color)
	require.Equal(t, "blocked = true", loaded[0].Columns[1].Query)
	require.Equal(t, "Ready", loaded[0].Columns[2].Name)
	require.Equal(t, "#00FF00", loaded[0].Columns[2].Color)
	require.Equal(t, "Done", loaded[0].Columns[3].Name)
	require.Equal(t, "#0000FF", loaded[0].Columns[3].Color)
}

func TestInsertColumnInView_PreservesOtherConfig(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, ".perles.yaml")

	// Create initial config with other settings
	initial := `theme:
  highlight: "#FF0000"
views:
  - name: Default
    columns:
      - name: Open
        query: status = open
`
	err := os.WriteFile(configPath, []byte(initial), 0644)
	require.NoError(t, err)

	// Load views to get proper struct
	views := []ViewConfig{
		{
			Name: "Default",
			Columns: []ColumnConfig{
				{Name: "Open", Query: "status = open"},
			},
		},
	}

	// Insert new column
	newCol := ColumnConfig{Name: "New", Query: "new = true"}
	err = InsertColumnInView(configPath, 0, 0, newCol, views)
	require.NoError(t, err)

	// Verify other settings preserved
	data, err := os.ReadFile(configPath)
	require.NoError(t, err)
	content := string(data)

	require.Contains(t, content, "highlight:")
	require.Contains(t, content, "name: New")
	require.Contains(t, content, "name: Open")
}

func TestInsertColumnInView_InvalidViewIndex(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, ".perles.yaml")

	views := []ViewConfig{
		{
			Name: "Default",
			Columns: []ColumnConfig{
				{Name: "Open", Query: "status = open"},
			},
		},
	}

	newCol := ColumnConfig{Name: "New", Query: "new = true"}

	// Index too high
	err := InsertColumnInView(configPath, 5, 0, newCol, views)
	require.Error(t, err)
	require.Contains(t, err.Error(), "out of range")

	// Index negative
	err = InsertColumnInView(configPath, -1, 0, newCol, views)
	require.Error(t, err)
	require.Contains(t, err.Error(), "out of range")
}

func TestInsertColumnInView_InvalidPosition(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, ".perles.yaml")

	views := []ViewConfig{
		{
			Name: "Default",
			Columns: []ColumnConfig{
				{Name: "Open", Query: "status = open"},
			},
		},
	}

	newCol := ColumnConfig{Name: "New", Query: "new = true"}

	// Position too high (only valid: 0, 1)
	err := InsertColumnInView(configPath, 0, 5, newCol, views)
	require.Error(t, err)
	require.Contains(t, err.Error(), "out of range")

	// Position negative
	err = InsertColumnInView(configPath, 0, -1, newCol, views)
	require.Error(t, err)
	require.Contains(t, err.Error(), "out of range")
}

func TestInsertColumnInView_AtomicWrite(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, ".perles.yaml")

	views := []ViewConfig{
		{
			Name: "Default",
			Columns: []ColumnConfig{
				{Name: "Open", Query: "status = open"},
			},
		},
	}

	// Save initial views
	err := SaveViews(configPath, views)
	require.NoError(t, err)

	// Insert new column
	newCol := ColumnConfig{Name: "New", Query: "new = true", Color: "#73F59F"}
	err = InsertColumnInView(configPath, 0, 0, newCol, views)
	require.NoError(t, err)

	// Check no temp files left behind
	entries, err := os.ReadDir(tempDir)
	require.NoError(t, err)

	for _, entry := range entries {
		name := entry.Name()
		require.False(t, name != ".perles.yaml" && filepath.Ext(name) == ".tmp",
			"temp file left behind: %s", name)
	}

	// Verify content was written correctly
	data, err := os.ReadFile(configPath)
	require.NoError(t, err)
	require.Contains(t, string(data), "name: New")
}

func TestInsertColumnInView_MultipleViews(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, ".perles.yaml")

	views := []ViewConfig{
		{
			Name: "Default",
			Columns: []ColumnConfig{
				{Name: "Default Col", Query: "status = open"},
			},
		},
		{
			Name: "Bugs",
			Columns: []ColumnConfig{
				{Name: "Bug Col", Query: "type = bug"},
			},
		},
	}

	// Save initial views
	err := SaveViews(configPath, views)
	require.NoError(t, err)

	// Insert into second view (index 1)
	newCol := ColumnConfig{Name: "New Bug Col", Query: "new = true", Color: "#FF0000"}
	err = InsertColumnInView(configPath, 1, 0, newCol, views)
	require.NoError(t, err)

	// Load and verify
	v := viper.New()
	v.SetConfigFile(configPath)
	err = v.ReadInConfig()
	require.NoError(t, err)

	var loaded []ViewConfig
	err = v.UnmarshalKey("views", &loaded)
	require.NoError(t, err)

	require.Len(t, loaded, 2)
	// First view should be unchanged
	require.Len(t, loaded[0].Columns, 1)
	require.Equal(t, "Default Col", loaded[0].Columns[0].Name)
	// Second view should have new column at front
	require.Len(t, loaded[1].Columns, 2)
	require.Equal(t, "New Bug Col", loaded[1].Columns[0].Name)
	require.Equal(t, "Bug Col", loaded[1].Columns[1].Name)
}

func TestInsertColumnInView_EmptyColumns(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, ".perles.yaml")

	views := []ViewConfig{
		{
			Name:    "Empty View",
			Columns: []ColumnConfig{},
		},
	}

	// Save initial views
	err := SaveViews(configPath, views)
	require.NoError(t, err)

	// Insert into empty column list
	newCol := ColumnConfig{Name: "First", Query: "status = open", Color: "#00FF00"}
	err = InsertColumnInView(configPath, 0, 0, newCol, views)
	require.NoError(t, err)

	// Load and verify
	v := viper.New()
	v.SetConfigFile(configPath)
	err = v.ReadInConfig()
	require.NoError(t, err)

	var loaded []ViewConfig
	err = v.UnmarshalKey("views", &loaded)
	require.NoError(t, err)

	require.Len(t, loaded, 1)
	require.Len(t, loaded[0].Columns, 1)
	require.Equal(t, "First", loaded[0].Columns[0].Name)
}

func TestRenameView(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, ".perles.yaml")

	// Create views
	views := []ViewConfig{
		{
			Name: "Default",
			Columns: []ColumnConfig{
				{Name: "Open", Query: "status = open"},
			},
		},
		{
			Name: "Bugs",
			Columns: []ColumnConfig{
				{Name: "All Bugs", Query: "type = bug"},
			},
		},
	}

	// Save initial views
	err := SaveViews(configPath, views)
	require.NoError(t, err)

	// Rename the second view
	err = RenameView(configPath, 1, "Critical Bugs", views)
	require.NoError(t, err)

	// Load and verify
	v := viper.New()
	v.SetConfigFile(configPath)
	err = v.ReadInConfig()
	require.NoError(t, err)

	var loaded []ViewConfig
	err = v.UnmarshalKey("views", &loaded)
	require.NoError(t, err)

	require.Len(t, loaded, 2)
	require.Equal(t, "Default", loaded[0].Name)
	require.Equal(t, "Critical Bugs", loaded[1].Name)
}

func TestRenameView_FirstView(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, ".perles.yaml")

	// Create views
	views := []ViewConfig{
		{
			Name: "First",
			Columns: []ColumnConfig{
				{Name: "Col1", Query: "status = open"},
			},
		},
		{
			Name: "Second",
			Columns: []ColumnConfig{
				{Name: "Col2", Query: "status = closed"},
			},
		},
	}

	// Save initial views
	err := SaveViews(configPath, views)
	require.NoError(t, err)

	// Rename the first view
	err = RenameView(configPath, 0, "Renamed First", views)
	require.NoError(t, err)

	// Load and verify
	v := viper.New()
	v.SetConfigFile(configPath)
	err = v.ReadInConfig()
	require.NoError(t, err)

	var loaded []ViewConfig
	err = v.UnmarshalKey("views", &loaded)
	require.NoError(t, err)

	require.Len(t, loaded, 2)
	require.Equal(t, "Renamed First", loaded[0].Name)
	require.Equal(t, "Second", loaded[1].Name)
}

func TestRenameView_OutOfRange(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, ".perles.yaml")

	views := []ViewConfig{
		{
			Name: "One",
			Columns: []ColumnConfig{
				{Name: "Col1", Query: "status = open"},
			},
		},
	}

	// Index too high
	err := RenameView(configPath, 5, "New Name", views)
	require.Error(t, err)
	require.Contains(t, err.Error(), "out of range")

	// Index negative
	err = RenameView(configPath, -1, "New Name", views)
	require.Error(t, err)
	require.Contains(t, err.Error(), "out of range")
}

func TestSaveColumns_TreeColumnType(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, ".perles.yaml")

	// Create columns with tree type
	columns := []ColumnConfig{
		{Name: "BQL Column", Type: "bql", Query: "status = open", Color: "#FF0000"},
		{Name: "Tree Column", Type: "tree", IssueID: "perles-123", TreeMode: "deps", Color: "#00FF00"},
		{Name: "Tree Child", Type: "tree", IssueID: "perles-456", TreeMode: "child"},
	}

	err := SaveColumns(configPath, columns)
	require.NoError(t, err)

	// Verify file content
	data, err := os.ReadFile(configPath)
	require.NoError(t, err)
	content := string(data)

	// BQL column should have query, no type (backward compat)
	require.Contains(t, content, "name: BQL Column")
	require.Contains(t, content, "query: status = open")

	// Tree column should have type and issue_id
	require.Contains(t, content, "name: Tree Column")
	require.Contains(t, content, "type: tree")
	require.Contains(t, content, "issue_id: perles-123")

	// Tree child should have tree_mode: child (not default)
	require.Contains(t, content, "tree_mode: child")

	// Load and verify roundtrip
	v := viper.New()
	v.SetConfigFile(configPath)
	err = v.ReadInConfig()
	require.NoError(t, err)

	var loaded []ViewConfig
	err = v.UnmarshalKey("views", &loaded)
	require.NoError(t, err)

	require.Len(t, loaded, 1)
	require.Len(t, loaded[0].Columns, 3)

	// Verify BQL column
	require.Equal(t, "BQL Column", loaded[0].Columns[0].Name)
	require.Equal(t, "status = open", loaded[0].Columns[0].Query)
	require.Equal(t, "#FF0000", loaded[0].Columns[0].Color)

	// Verify Tree column
	require.Equal(t, "Tree Column", loaded[0].Columns[1].Name)
	require.Equal(t, "tree", loaded[0].Columns[1].Type)
	require.Equal(t, "perles-123", loaded[0].Columns[1].IssueID)
	require.Equal(t, "#00FF00", loaded[0].Columns[1].Color)

	// Verify Tree child column
	require.Equal(t, "Tree Child", loaded[0].Columns[2].Name)
	require.Equal(t, "tree", loaded[0].Columns[2].Type)
	require.Equal(t, "perles-456", loaded[0].Columns[2].IssueID)
	require.Equal(t, "child", loaded[0].Columns[2].TreeMode)
}

func TestSaveColumns_TreeColumnOmitsQuery(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, ".perles.yaml")

	// Tree column should not have query field in YAML
	columns := []ColumnConfig{
		{Name: "Tree Only", Type: "tree", IssueID: "test-1"},
	}

	err := SaveColumns(configPath, columns)
	require.NoError(t, err)

	// Read raw YAML and check there's no query field for tree columns
	data, err := os.ReadFile(configPath)
	require.NoError(t, err)
	content := string(data)

	require.Contains(t, content, "name: Tree Only")
	require.Contains(t, content, "type: tree")
	require.Contains(t, content, "issue_id: test-1")
	// Should NOT contain query for tree columns
	require.NotContains(t, content, "query:")
}

func TestSaveColumns_TreeColumnOmitsDefaultMode(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, ".perles.yaml")

	// Tree column with default mode should not include tree_mode in YAML
	columns := []ColumnConfig{
		{Name: "Tree Deps", Type: "tree", IssueID: "test-1", TreeMode: "deps"},
		{Name: "Tree Empty Mode", Type: "tree", IssueID: "test-2", TreeMode: ""},
	}

	err := SaveColumns(configPath, columns)
	require.NoError(t, err)

	// Read raw YAML
	data, err := os.ReadFile(configPath)
	require.NoError(t, err)
	content := string(data)

	// Should NOT contain tree_mode field (deps is default, empty should be omitted)
	require.NotContains(t, content, "tree_mode:")
}

func TestSaveColumns_BQLColumnBackwardCompatibility(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, ".perles.yaml")

	// BQL columns (type empty or "bql") should not include type field for backward compatibility
	columns := []ColumnConfig{
		{Name: "No Type", Query: "status = open"},              // Type empty
		{Name: "BQL Type", Type: "bql", Query: "ready = true"}, // Type explicitly bql
	}

	err := SaveColumns(configPath, columns)
	require.NoError(t, err)

	// Read raw YAML
	data, err := os.ReadFile(configPath)
	require.NoError(t, err)
	content := string(data)

	// Should have queries
	require.Contains(t, content, "query: status = open")
	require.Contains(t, content, "query: ready = true")

	// Should NOT have type field for BQL columns
	require.NotContains(t, content, "type:")
}

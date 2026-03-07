package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"

	"github.com/zjrosen/perles/internal/ui/styles"
)

// TestThemeConfig_WithPreset tests loading a config file with a preset.
func TestThemeConfig_WithPreset(t *testing.T) {
	configYAML := `
theme:
  preset: catppuccin-mocha
`
	cfg := loadConfigFromYAML(t, configYAML)

	require.Equal(t, "catppuccin-mocha", cfg.Theme.Preset)

	// Apply theme and verify colors changed
	themeCfg := styles.ThemeConfig{
		Preset: cfg.Theme.Preset,
		Mode:   cfg.Theme.Mode,
		Colors: cfg.Theme.FlattenedColors(),
	}
	err := styles.ApplyTheme(themeCfg)
	require.NoError(t, err)

	// Catppuccin Mocha uses #CDD6F4 for text.primary
	require.Equal(t, "#CDD6F4", styles.TextPrimaryColor.Dark)
}

// TestThemeConfig_WithColorOverrides tests applying color overrides programmatically.
func TestThemeConfig_WithColorOverrides(t *testing.T) {
	cfg := Config{
		Theme: ThemeConfig{
			Colors: map[string]any{
				"text": map[string]any{
					"primary": "#FF0000",
				},
				"status": map[string]any{
					"error": "#00FF00",
				},
			},
		},
	}

	require.NotNil(t, cfg.Theme.Colors)
	flattened := cfg.Theme.FlattenedColors()
	require.Equal(t, "#FF0000", flattened["text.primary"])
	require.Equal(t, "#00FF00", flattened["status.error"])

	// Apply theme and verify colors applied
	themeCfg := styles.ThemeConfig{
		Preset: cfg.Theme.Preset,
		Mode:   cfg.Theme.Mode,
		Colors: flattened,
	}
	err := styles.ApplyTheme(themeCfg)
	require.NoError(t, err)

	require.Equal(t, "#FF0000", styles.TextPrimaryColor.Dark)
	require.Equal(t, "#00FF00", styles.StatusErrorColor.Dark)
}

// TestThemeConfig_WithColorOverridesFromYAML tests that dotted color tokens
// in YAML config files are correctly parsed. Viper interprets unquoted dotted keys
// as nested structures, which FlattenedColors() handles correctly.
func TestThemeConfig_WithColorOverridesFromYAML(t *testing.T) {
	// Viper treats "text.primary" as nested: text -> primary
	configYAML := `
theme:
  colors:
    text.primary: "#FF0000"
    status.error: "#00FF00"
    selection.indicator: "#0000FF"
`
	cfg := loadConfigFromYAML(t, configYAML)

	require.NotNil(t, cfg.Theme.Colors)

	// Verify flattening produces correct dot-notation keys
	flattened := cfg.Theme.FlattenedColors()
	require.Equal(t, "#FF0000", flattened["text.primary"])
	require.Equal(t, "#00FF00", flattened["status.error"])
	require.Equal(t, "#0000FF", flattened["selection.indicator"])

	// Apply theme and verify colors applied
	themeCfg := styles.ThemeConfig{
		Preset: cfg.Theme.Preset,
		Mode:   cfg.Theme.Mode,
		Colors: flattened,
	}
	err := styles.ApplyTheme(themeCfg)
	require.NoError(t, err)

	require.Equal(t, "#FF0000", styles.TextPrimaryColor.Dark)
	require.Equal(t, "#00FF00", styles.StatusErrorColor.Dark)
	require.Equal(t, "#0000FF", styles.SelectionIndicatorColor.Dark)
}

// TestThemeConfig_PresetWithOverrides tests that color overrides take precedence over preset.
func TestThemeConfig_PresetWithOverrides(t *testing.T) {
	cfg := Config{
		Theme: ThemeConfig{
			Preset: "dracula",
			Colors: map[string]any{
				"text": map[string]any{
					"primary": "#123456",
				},
			},
		},
	}

	require.Equal(t, "dracula", cfg.Theme.Preset)
	flattened := cfg.Theme.FlattenedColors()
	require.Equal(t, "#123456", flattened["text.primary"])

	// Apply theme
	themeCfg := styles.ThemeConfig{
		Preset: cfg.Theme.Preset,
		Mode:   cfg.Theme.Mode,
		Colors: flattened,
	}
	err := styles.ApplyTheme(themeCfg)
	require.NoError(t, err)

	// Override should take precedence
	require.Equal(t, "#123456", styles.TextPrimaryColor.Dark)
	// Dracula's status error should still be applied (#FF5555)
	require.Equal(t, "#FF5555", styles.StatusErrorColor.Dark)
}

// TestThemeConfig_InvalidPreset tests that invalid preset returns error.
func TestThemeConfig_InvalidPreset(t *testing.T) {
	configYAML := `
theme:
  preset: nonexistent-theme
`
	cfg := loadConfigFromYAML(t, configYAML)

	themeCfg := styles.ThemeConfig{
		Preset: cfg.Theme.Preset,
		Mode:   cfg.Theme.Mode,
		Colors: cfg.Theme.FlattenedColors(),
	}
	err := styles.ApplyTheme(themeCfg)
	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown theme preset")
}

// TestThemeConfig_InvalidColorToken tests that invalid color token returns error.
func TestThemeConfig_InvalidColorToken(t *testing.T) {
	cfg := Config{
		Theme: ThemeConfig{
			Colors: map[string]any{
				"invalid": map[string]any{
					"token": map[string]any{
						"name": "#FF0000",
					},
				},
			},
		},
	}

	themeCfg := styles.ThemeConfig{
		Preset: cfg.Theme.Preset,
		Mode:   cfg.Theme.Mode,
		Colors: cfg.Theme.FlattenedColors(),
	}
	err := styles.ApplyTheme(themeCfg)
	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown color token")
}

// TestThemeConfig_InvalidHexColor tests that invalid hex color returns error.
func TestThemeConfig_InvalidHexColor(t *testing.T) {
	cfg := Config{
		Theme: ThemeConfig{
			Colors: map[string]any{
				"text": map[string]any{
					"primary": "not-a-color",
				},
			},
		},
	}

	themeCfg := styles.ThemeConfig{
		Preset: cfg.Theme.Preset,
		Mode:   cfg.Theme.Mode,
		Colors: cfg.Theme.FlattenedColors(),
	}
	err := styles.ApplyTheme(themeCfg)
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid hex color")
}

// TestThemeConfig_EmptyConfig tests that empty theme config applies defaults.
func TestThemeConfig_EmptyConfig(t *testing.T) {
	configYAML := `
beads_dir: ""
`
	cfg := loadConfigFromYAML(t, configYAML)

	// Empty theme should result in empty/nil values
	require.Empty(t, cfg.Theme.Preset)
	require.Nil(t, cfg.Theme.Colors)

	// Apply should succeed with default colors
	themeCfg := styles.ThemeConfig{
		Preset: cfg.Theme.Preset,
		Mode:   cfg.Theme.Mode,
		Colors: cfg.Theme.FlattenedColors(),
	}
	err := styles.ApplyTheme(themeCfg)
	require.NoError(t, err)

	// Default preset should be applied (#CCCCCC for text.primary)
	require.Equal(t, "#CCCCCC", styles.TextPrimaryColor.Dark)
}

// TestThemeConfig_AllPresets tests that all built-in presets load correctly.
func TestThemeConfig_AllPresets(t *testing.T) {
	presets := []string{
		"default",
		"catppuccin-mocha",
		"catppuccin-latte",
		"dracula",
		"nord",
		"high-contrast",
	}

	for _, preset := range presets {
		t.Run(preset, func(t *testing.T) {
			configYAML := `
theme:
  preset: ` + preset + `
`
			if preset == "default" {
				configYAML = `
theme:
  preset: ""
`
			}
			cfg := loadConfigFromYAML(t, configYAML)

			themeCfg := styles.ThemeConfig{
				Preset: cfg.Theme.Preset,
				Mode:   cfg.Theme.Mode,
				Colors: cfg.Theme.FlattenedColors(),
			}
			err := styles.ApplyTheme(themeCfg)
			require.NoError(t, err, "preset %s should apply without error", preset)
		})
	}
}

// TestThemeConfig_NestedYAMLColorOverrides tests that nested YAML color overrides
// are properly flattened to dot-notation keys.
func TestThemeConfig_NestedYAMLColorOverrides(t *testing.T) {
	configYAML := `
theme:
  preset: dracula
  colors:
    text:
      primary: "#FF0000"
      secondary: "#00FF00"
    status:
      error: "#0000FF"
`
	cfg := loadConfigFromYAML(t, configYAML)

	require.Equal(t, "dracula", cfg.Theme.Preset)
	require.NotNil(t, cfg.Theme.Colors)

	// Verify flattening works
	flattened := cfg.Theme.FlattenedColors()
	require.Equal(t, "#FF0000", flattened["text.primary"])
	require.Equal(t, "#00FF00", flattened["text.secondary"])
	require.Equal(t, "#0000FF", flattened["status.error"])

	// Apply theme and verify colors applied
	themeCfg := styles.ThemeConfig{
		Preset: cfg.Theme.Preset,
		Mode:   cfg.Theme.Mode,
		Colors: flattened,
	}
	err := styles.ApplyTheme(themeCfg)
	require.NoError(t, err)

	require.Equal(t, "#FF0000", styles.TextPrimaryColor.Dark)
	require.Equal(t, "#00FF00", styles.TextSecondaryColor.Dark)
	require.Equal(t, "#0000FF", styles.StatusErrorColor.Dark)
}

// loadConfigFromYAML is a helper to load config from YAML string.
func loadConfigFromYAML(t *testing.T, yaml string) Config {
	t.Helper()

	// Create temp file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	err := os.WriteFile(configPath, []byte(yaml), 0644)
	require.NoError(t, err)

	// Reset viper for each test
	v := viper.New()
	v.SetConfigFile(configPath)
	err = v.ReadInConfig()
	require.NoError(t, err)

	// Unmarshal to Config struct
	var cfg Config
	err = v.Unmarshal(&cfg)
	require.NoError(t, err)

	return cfg
}

package panes

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/exp/teatest"
	"github.com/stretchr/testify/require"
)

// Test colors for bordered pane tests
var (
	testColorBlue   = lipgloss.AdaptiveColor{Light: "#54A0FF", Dark: "#54A0FF"}
	testColorGreen  = lipgloss.AdaptiveColor{Light: "#43BF6D", Dark: "#73F59F"}
	testColorPurple = lipgloss.AdaptiveColor{Light: "#7D56F4", Dark: "#7D56F4"}
)

// =============================================================================
// Unit Tests for BorderedPane
// =============================================================================

func TestBorderedPane_BasicRendering(t *testing.T) {
	cfg := BorderConfig{
		Content: "Hello World",
		Width:   20,
		Height:  5,
	}

	result := BorderedPane(cfg)

	// Should contain border characters (rounded by default)
	require.Contains(t, result, "╭", "missing top-left corner")
	require.Contains(t, result, "╮", "missing top-right corner")
	require.Contains(t, result, "╰", "missing bottom-left corner")
	require.Contains(t, result, "╯", "missing bottom-right corner")
	require.Contains(t, result, "│", "missing vertical border")

	// Should contain content
	require.Contains(t, result, "Hello World", "missing content")

	// Should have correct line count
	lines := strings.Split(result, "\n")
	require.Len(t, lines, 5, "expected 5 lines for height 5")
}

func TestBorderedPane_TopLeftTitle(t *testing.T) {
	cfg := BorderConfig{
		Content: "content",
		Width:   30,
		Height:  5,
		TopLeft: "My Title",
	}

	result := BorderedPane(cfg)

	require.Contains(t, result, "My Title", "missing top-left title")
	require.Contains(t, result, "╭", "missing top-left corner")
}

func TestBorderedPane_TopRightTitle(t *testing.T) {
	cfg := BorderConfig{
		Content:  "content",
		Width:    30,
		Height:   5,
		TopRight: "Status",
	}

	result := BorderedPane(cfg)

	require.Contains(t, result, "Status", "missing top-right title")
}

func TestBorderedPane_BottomLeftTitle(t *testing.T) {
	cfg := BorderConfig{
		Content:    "content",
		Width:      30,
		Height:     5,
		BottomLeft: "Footer",
	}

	result := BorderedPane(cfg)

	require.Contains(t, result, "Footer", "missing bottom-left title")
	require.Contains(t, result, "╰", "missing bottom-left corner")
}

func TestBorderedPane_BottomRightTitle(t *testing.T) {
	cfg := BorderConfig{
		Content:     "content",
		Width:       30,
		Height:      5,
		BottomRight: "Page 1/5",
	}

	result := BorderedPane(cfg)

	require.Contains(t, result, "Page 1/5", "missing bottom-right title")
}

func TestBorderedPane_DualTitles(t *testing.T) {
	cfg := BorderConfig{
		Content:     "content",
		Width:       40,
		Height:      5,
		TopLeft:     "Title",
		TopRight:    "Info",
		BottomLeft:  "Help",
		BottomRight: "Status",
	}

	result := BorderedPane(cfg)

	require.Contains(t, result, "Title", "missing top-left title")
	require.Contains(t, result, "Info", "missing top-right title")
	require.Contains(t, result, "Help", "missing bottom-left title")
	require.Contains(t, result, "Status", "missing bottom-right title")
}

func TestBorderedPane_HeaderAction(t *testing.T) {
	cfg := BorderConfig{
		Content: "content",
		Width:   40,
		Height:  5,
		TopLeft: "Title",
		HeaderAction: HeaderAction{
			Label: "[+]",
		},
	}

	result := BorderedPane(cfg)

	require.Contains(t, result, "Title", "missing top-left title")
	require.Contains(t, result, "[+]", "missing header action")
}

func TestBorderedPane_FocusedState(t *testing.T) {
	cfgUnfocused := BorderConfig{
		Content:            "content",
		Width:              20,
		Height:             5,
		TopLeft:            "Test",
		Focused:            false,
		BorderColor:        testColorBlue,
		FocusedBorderColor: testColorGreen,
	}

	cfgFocused := BorderConfig{
		Content:            "content",
		Width:              20,
		Height:             5,
		TopLeft:            "Test",
		Focused:            true,
		BorderColor:        testColorBlue,
		FocusedBorderColor: testColorGreen,
	}

	unfocusedResult := BorderedPane(cfgUnfocused)
	focusedResult := BorderedPane(cfgFocused)

	// Both should have valid structure
	require.Contains(t, unfocusedResult, "╭", "unfocused missing border")
	require.Contains(t, focusedResult, "╭", "focused missing border")
	require.Contains(t, unfocusedResult, "Test", "unfocused missing title")
	require.Contains(t, focusedResult, "Test", "focused missing title")

	// Results should have same line count but may differ in ANSI color codes
	unfocusedLines := strings.Split(unfocusedResult, "\n")
	focusedLines := strings.Split(focusedResult, "\n")
	require.Equal(t, len(unfocusedLines), len(focusedLines), "focused and unfocused should have same line count")
}

func TestBorderedPane_CustomColors(t *testing.T) {
	cfg := BorderConfig{
		Content:     "content",
		Width:       20,
		Height:      5,
		TopLeft:     "Test",
		TitleColor:  testColorPurple,
		BorderColor: testColorBlue,
	}

	result := BorderedPane(cfg)

	// Should render without error
	require.Contains(t, result, "Test", "missing title")
	require.Contains(t, result, "content", "missing content")
}

func TestBorderedPane_NilColors(t *testing.T) {
	// All nil colors should use defaults
	cfg := BorderConfig{
		Content:            "content",
		Width:              20,
		Height:             5,
		TopLeft:            "Test",
		TitleColor:         nil,
		BorderColor:        nil,
		FocusedBorderColor: nil,
	}

	result := BorderedPane(cfg)

	// Should render without error using defaults
	require.Contains(t, result, "Test", "missing title")
	require.Contains(t, result, "content", "missing content")
}

func TestBorderedPane_EmptyContent(t *testing.T) {
	cfg := BorderConfig{
		Content: "",
		Width:   20,
		Height:  5,
		TopLeft: "Empty",
	}

	result := BorderedPane(cfg)

	// Should still render valid border
	require.Contains(t, result, "╭", "missing top-left corner")
	require.Contains(t, result, "╯", "missing bottom-right corner")
	require.Contains(t, result, "Empty", "missing title")

	// Should have correct line count
	lines := strings.Split(result, "\n")
	require.Len(t, lines, 5, "expected 5 lines for height 5")
}

func TestBorderedPane_NarrowWidth(t *testing.T) {
	cfg := BorderConfig{
		Content: "x",
		Width:   5,
		Height:  3,
	}

	result := BorderedPane(cfg)

	// Should render without panic
	require.Contains(t, result, "╭", "missing top-left corner")
	require.Contains(t, result, "╯", "missing bottom-right corner")

	lines := strings.Split(result, "\n")
	require.Len(t, lines, 3, "expected 3 lines for height 3")
}

func TestBorderedPane_MinimumWidth(t *testing.T) {
	cfg := BorderConfig{
		Content: "x",
		Width:   3, // Minimum: just corners
		Height:  3,
	}

	result := BorderedPane(cfg)

	// Should render without panic even at minimum width
	require.NotEmpty(t, result, "result should not be empty")
}

func TestBorderedPane_SmallHeight(t *testing.T) {
	cfg := BorderConfig{
		Content: "content",
		Width:   20,
		Height:  3, // Minimum usable height
	}

	result := BorderedPane(cfg)

	lines := strings.Split(result, "\n")
	require.Len(t, lines, 3, "expected 3 lines for height 3")
}

func TestBorderedPane_ContentTruncation(t *testing.T) {
	// Content wider than inner width should be truncated
	cfg := BorderConfig{
		Content: "This is a very long line that should be truncated to fit within the border",
		Width:   20,
		Height:  3,
	}

	result := BorderedPane(cfg)

	// Each line should fit within the width
	lines := strings.Split(result, "\n")
	for _, line := range lines {
		lineWidth := lipgloss.Width(line)
		require.LessOrEqual(t, lineWidth, 20, "line width exceeds border width")
	}
}

func TestBorderedPane_MultilineContent(t *testing.T) {
	cfg := BorderConfig{
		Content: "Line 1\nLine 2\nLine 3",
		Width:   20,
		Height:  5,
	}

	result := BorderedPane(cfg)

	require.Contains(t, result, "Line 1", "missing line 1")
	require.Contains(t, result, "Line 2", "missing line 2")
	require.Contains(t, result, "Line 3", "missing line 3")
}

func TestBorderedPane_NoTitle(t *testing.T) {
	cfg := BorderConfig{
		Content: "content",
		Width:   20,
		Height:  5,
		// No titles set
	}

	result := BorderedPane(cfg)

	// Should render plain border without titles
	require.Contains(t, result, "╭", "missing top-left corner")
	require.Contains(t, result, "╮", "missing top-right corner")
	require.Contains(t, result, "content", "missing content")

	// First line should be plain border (no title text between corners)
	lines := strings.Split(result, "\n")
	require.GreaterOrEqual(t, len(lines), 1, "expected at least 1 line")
}

// =============================================================================
// Unit Tests for resolveBorderColor
// =============================================================================

func TestResolveBorderColor_BothNil(t *testing.T) {
	// Both nil: should return default color
	result := resolveBorderColor(nil, nil, false)
	require.NotNil(t, result, "should return non-nil color")

	result = resolveBorderColor(nil, nil, true)
	require.NotNil(t, result, "should return non-nil color when focused")
}

func TestResolveBorderColor_OnlyBorderColor(t *testing.T) {
	// BorderColor set, FocusedBorderColor nil: inherit BorderColor for both states
	result := resolveBorderColor(testColorBlue, nil, false)
	require.Equal(t, testColorBlue, result, "unfocused should use BorderColor")

	result = resolveBorderColor(testColorBlue, nil, true)
	require.Equal(t, testColorBlue, result, "focused should inherit BorderColor")
}

func TestResolveBorderColor_OnlyFocusedBorderColor(t *testing.T) {
	// BorderColor nil, FocusedBorderColor set
	result := resolveBorderColor(nil, testColorGreen, false)
	require.NotNil(t, result, "unfocused should use default")

	result = resolveBorderColor(nil, testColorGreen, true)
	require.Equal(t, testColorGreen, result, "focused should use FocusedBorderColor")
}

func TestResolveBorderColor_BothSet(t *testing.T) {
	// Both set: use appropriately based on focused flag
	result := resolveBorderColor(testColorBlue, testColorGreen, false)
	require.Equal(t, testColorBlue, result, "unfocused should use BorderColor")

	result = resolveBorderColor(testColorBlue, testColorGreen, true)
	require.Equal(t, testColorGreen, result, "focused should use FocusedBorderColor")
}

// =============================================================================
// Unit Tests for buildTopBorder
// =============================================================================

func TestBuildTopBorder_EmptyTitle(t *testing.T) {
	borderStyle := lipgloss.NewStyle()
	titleStyle := lipgloss.NewStyle()

	result := buildTopBorder("", 10, borderStyle, titleStyle)

	require.Contains(t, result, "╭", "missing top-left corner")
	require.Contains(t, result, "╮", "missing top-right corner")
	require.Contains(t, result, "─", "missing horizontal border")
}

func TestBuildTopBorder_WithTitle(t *testing.T) {
	borderStyle := lipgloss.NewStyle()
	titleStyle := lipgloss.NewStyle()

	result := buildTopBorder("Test", 15, borderStyle, titleStyle)

	require.Contains(t, result, "╭", "missing top-left corner")
	require.Contains(t, result, "╮", "missing top-right corner")
	require.Contains(t, result, "Test", "missing title")
}

func TestBuildTopBorder_NarrowWidth(t *testing.T) {
	borderStyle := lipgloss.NewStyle()
	titleStyle := lipgloss.NewStyle()

	// Width too narrow for title
	result := buildTopBorder("Title", 3, borderStyle, titleStyle)

	// Should render border without title
	require.Contains(t, result, "╭", "missing top-left corner")
	require.Contains(t, result, "╮", "missing top-right corner")
}

func TestBuildTopBorder_ZeroWidth(t *testing.T) {
	borderStyle := lipgloss.NewStyle()
	titleStyle := lipgloss.NewStyle()

	result := buildTopBorder("Title", 0, borderStyle, titleStyle)

	// Should render minimal border
	require.Contains(t, result, "╭", "missing top-left corner")
	require.Contains(t, result, "╮", "missing top-right corner")
}

func TestBuildTopBorder_LongTitle(t *testing.T) {
	borderStyle := lipgloss.NewStyle()
	titleStyle := lipgloss.NewStyle()

	// Title longer than available space
	result := buildTopBorder("Very Long Title That Should Be Truncated", 15, borderStyle, titleStyle)

	require.Contains(t, result, "╭", "missing top-left corner")
	require.Contains(t, result, "╮", "missing top-right corner")
	// Title should be truncated, full title should not appear
	require.NotContains(t, result, "Truncated", "title should be truncated")
}

// =============================================================================
// Unit Tests for buildBottomBorder
// =============================================================================

func TestBuildBottomBorder_EmptyTitle(t *testing.T) {
	borderStyle := lipgloss.NewStyle()
	titleStyle := lipgloss.NewStyle()

	result := buildBottomBorder("", 10, borderStyle, titleStyle)

	require.Contains(t, result, "╰", "missing bottom-left corner")
	require.Contains(t, result, "╯", "missing bottom-right corner")
	require.Contains(t, result, "─", "missing horizontal border")
}

func TestBuildBottomBorder_WithTitle(t *testing.T) {
	borderStyle := lipgloss.NewStyle()
	titleStyle := lipgloss.NewStyle()

	result := buildBottomBorder("Footer", 15, borderStyle, titleStyle)

	require.Contains(t, result, "╰", "missing bottom-left corner")
	require.Contains(t, result, "╯", "missing bottom-right corner")
	require.Contains(t, result, "Footer", "missing title")
}

// =============================================================================
// Unit Tests for buildDualTitleTopBorder
// =============================================================================

func TestBuildDualTitleTopBorder_BothEmpty(t *testing.T) {
	borderStyle := lipgloss.NewStyle()
	titleStyle := lipgloss.NewStyle()

	result := buildDualTitleTopBorder("", "", 20, borderStyle, titleStyle)

	require.Contains(t, result, "╭", "missing top-left corner")
	require.Contains(t, result, "╮", "missing top-right corner")
}

func TestBuildDualTitleTopBorder_LeftOnly(t *testing.T) {
	borderStyle := lipgloss.NewStyle()
	titleStyle := lipgloss.NewStyle()

	result := buildDualTitleTopBorder("Left", "", 20, borderStyle, titleStyle)

	require.Contains(t, result, "Left", "missing left title")
	require.Contains(t, result, "╭", "missing top-left corner")
}

func TestBuildDualTitleTopBorder_RightOnly(t *testing.T) {
	borderStyle := lipgloss.NewStyle()
	titleStyle := lipgloss.NewStyle()

	result := buildDualTitleTopBorder("", "Right", 20, borderStyle, titleStyle)

	require.Contains(t, result, "Right", "missing right title")
	require.Contains(t, result, "╮", "missing top-right corner")
}

func TestBuildDualTitleTopBorder_Both(t *testing.T) {
	borderStyle := lipgloss.NewStyle()
	titleStyle := lipgloss.NewStyle()

	result := buildDualTitleTopBorder("Left", "Right", 30, borderStyle, titleStyle)

	require.Contains(t, result, "Left", "missing left title")
	require.Contains(t, result, "Right", "missing right title")
}

func TestBuildDualTitleTopBorder_TooNarrow(t *testing.T) {
	borderStyle := lipgloss.NewStyle()
	titleStyle := lipgloss.NewStyle()

	// Width too narrow for both titles
	result := buildDualTitleTopBorder("LeftTitle", "RightTitle", 10, borderStyle, titleStyle)

	// Should fall back to single title or plain border
	require.Contains(t, result, "╭", "missing top-left corner")
	require.Contains(t, result, "╮", "missing top-right corner")
}

func TestBuildDualTitleTopBorderWithAction(t *testing.T) {
	borderStyle := lipgloss.NewStyle()
	titleStyle := lipgloss.NewStyle()
	actionStyle := lipgloss.NewStyle().Bold(true)

	result := buildDualTitleTopBorderWithAction(
		"Left", "Right", HeaderAction{Label: "[+]"},
		36, borderStyle, titleStyle, actionStyle,
	)

	require.Contains(t, result, "Left", "missing left title")
	require.Contains(t, result, "Right", "missing right title")
	require.Contains(t, result, "[+]", "missing header action")
}

// =============================================================================
// Unit Tests for buildDualTitleBottomBorder
// =============================================================================

func TestBuildDualTitleBottomBorder_BothEmpty(t *testing.T) {
	borderStyle := lipgloss.NewStyle()
	titleStyle := lipgloss.NewStyle()

	result := buildDualTitleBottomBorder("", "", 20, borderStyle, titleStyle)

	require.Contains(t, result, "╰", "missing bottom-left corner")
	require.Contains(t, result, "╯", "missing bottom-right corner")
}

func TestBuildDualTitleBottomBorder_Both(t *testing.T) {
	borderStyle := lipgloss.NewStyle()
	titleStyle := lipgloss.NewStyle()

	result := buildDualTitleBottomBorder("Help", "Page 1", 30, borderStyle, titleStyle)

	require.Contains(t, result, "Help", "missing left title")
	require.Contains(t, result, "Page 1", "missing right title")
}

// =============================================================================
// Edge Case Tests
// =============================================================================

func TestBorderedPane_WidthEqualsContentWidth(t *testing.T) {
	// Content exactly fills inner width
	cfg := BorderConfig{
		Content: "12345678", // 8 chars, with 2-char border = width 10
		Width:   10,
		Height:  3,
	}

	result := BorderedPane(cfg)

	require.Contains(t, result, "12345678", "content should be present")
}

func TestBorderedPane_UnicodeContent(t *testing.T) {
	cfg := BorderConfig{
		Content: "Hello 世界",
		Width:   20,
		Height:  3,
	}

	result := BorderedPane(cfg)

	require.Contains(t, result, "Hello", "missing English text")
	require.Contains(t, result, "世界", "missing Unicode content")
}

func TestBorderedPane_UnicodeTitle(t *testing.T) {
	cfg := BorderConfig{
		Content: "content",
		Width:   30,
		Height:  3,
		TopLeft: "日本語",
	}

	result := BorderedPane(cfg)

	require.Contains(t, result, "日本語", "missing Unicode title")
}

func TestBorderedPane_SpecialCharactersInContent(t *testing.T) {
	cfg := BorderConfig{
		Content: "Tab:\tNewline:\n<>&\"'",
		Width:   30,
		Height:  5,
	}

	result := BorderedPane(cfg)

	// Should render without panic
	require.Contains(t, result, "╭", "missing border")
}

// =============================================================================
// Golden Tests
// =============================================================================

func TestBorderedPane_Golden_Basic(t *testing.T) {
	cfg := BorderConfig{
		Content: "Hello World",
		Width:   30,
		Height:  5,
	}

	result := BorderedPane(cfg)
	teatest.RequireEqualOutput(t, []byte(result))
}

func TestBorderedPane_Golden_TopLeftTitle(t *testing.T) {
	cfg := BorderConfig{
		Content: "Some content here",
		Width:   40,
		Height:  5,
		TopLeft: "My Panel",
	}

	result := BorderedPane(cfg)
	teatest.RequireEqualOutput(t, []byte(result))
}

func TestBorderedPane_Golden_AllTitles(t *testing.T) {
	cfg := BorderConfig{
		Content:     "Centered content",
		Width:       50,
		Height:      7,
		TopLeft:     "Title",
		TopRight:    "Status",
		BottomLeft:  "Help: ?",
		BottomRight: "1/10",
	}

	result := BorderedPane(cfg)
	teatest.RequireEqualOutput(t, []byte(result))
}

func TestBorderedPane_Golden_MultilineContent(t *testing.T) {
	cfg := BorderConfig{
		Content: "Line 1: Hello\nLine 2: World\nLine 3: Test\nLine 4: Content",
		Width:   40,
		Height:  8,
		TopLeft: "Log Output",
	}

	result := BorderedPane(cfg)
	teatest.RequireEqualOutput(t, []byte(result))
}

func TestBorderedPane_Golden_Narrow(t *testing.T) {
	cfg := BorderConfig{
		Content: "x",
		Width:   10,
		Height:  5,
		TopLeft: "N",
	}

	result := BorderedPane(cfg)
	teatest.RequireEqualOutput(t, []byte(result))
}

func TestBorderedPane_Golden_Empty(t *testing.T) {
	cfg := BorderConfig{
		Content: "",
		Width:   30,
		Height:  5,
		TopLeft: "Empty",
	}

	result := BorderedPane(cfg)
	teatest.RequireEqualOutput(t, []byte(result))
}

func TestBorderedPane_Golden_LongTitle(t *testing.T) {
	cfg := BorderConfig{
		Content: "content",
		Width:   30,
		Height:  5,
		TopLeft: "This is a very long title that exceeds the available width",
	}

	result := BorderedPane(cfg)
	teatest.RequireEqualOutput(t, []byte(result))
}

func TestBorderedPane_Golden_DualTopTitles(t *testing.T) {
	cfg := BorderConfig{
		Content:  "Panel with dual top titles",
		Width:    50,
		Height:   5,
		TopLeft:  "Messages",
		TopRight: "12 unread",
	}

	result := BorderedPane(cfg)
	teatest.RequireEqualOutput(t, []byte(result))
}

func TestBorderedPane_Golden_DualBottomTitles(t *testing.T) {
	cfg := BorderConfig{
		Content:     "Panel with dual bottom titles",
		Width:       50,
		Height:      5,
		BottomLeft:  "Press ? for help",
		BottomRight: "Page 5/20",
	}

	result := BorderedPane(cfg)
	teatest.RequireEqualOutput(t, []byte(result))
}

// =============================================================================
// Unit Tests for Tab Struct
// =============================================================================

func TestTab_ZeroValue(t *testing.T) {
	// Zero value Tab should have sensible defaults
	var tab Tab

	// Label should be empty string (valid, will render as empty tab)
	require.Equal(t, "", tab.Label, "zero value Label should be empty string")

	// Content should be empty string (valid, will render as empty content)
	require.Equal(t, "", tab.Content, "zero value Content should be empty string")

	// Color should be nil (will use default color)
	require.Nil(t, tab.Color, "zero value Color should be nil")
}

// =============================================================================
// Unit Tests for BorderConfig Tab Fields
// =============================================================================

func TestBorderConfig_TabFields_ZeroValue(t *testing.T) {
	// Zero value BorderConfig should maintain backward compatibility
	// All tab fields should be zero values that result in classic mode behavior
	var cfg BorderConfig

	// Tabs should be nil (classic mode, not tab mode)
	require.Nil(t, cfg.Tabs, "zero value Tabs should be nil")
	require.Len(t, cfg.Tabs, 0, "zero value Tabs should have length 0")

	// ActiveTab should be 0 (first tab if tabs were present)
	require.Equal(t, 0, cfg.ActiveTab, "zero value ActiveTab should be 0")

	// ActiveTabColor should be nil (will use default: BorderHighlightFocusColor)
	require.Nil(t, cfg.ActiveTabColor, "zero value ActiveTabColor should be nil")

	// InactiveTabColor should be nil (will use default: TextMutedColor)
	require.Nil(t, cfg.InactiveTabColor, "zero value InactiveTabColor should be nil")
}

func TestBorderConfig_TabFields_BackwardCompatibility(t *testing.T) {
	// Existing BorderConfig usage without tab fields should still work
	cfg := BorderConfig{
		Content: "Hello World",
		Width:   30,
		Height:  5,
		TopLeft: "Title",
	}

	// Tab fields should be zero values (not set)
	require.Nil(t, cfg.Tabs, "Tabs should be nil when not explicitly set")
	require.Equal(t, 0, cfg.ActiveTab, "ActiveTab should be 0 when not set")
	require.Nil(t, cfg.ActiveTabColor, "ActiveTabColor should be nil when not set")
	require.Nil(t, cfg.InactiveTabColor, "InactiveTabColor should be nil when not set")

	// Should render normally (classic mode)
	result := BorderedPane(cfg)
	require.Contains(t, result, "Title", "should render with title in classic mode")
	require.Contains(t, result, "Hello World", "should render content in classic mode")
}

// =============================================================================
// Unit Tests for resolveActiveTabStyle
// =============================================================================

func TestResolveActiveTabStyle_Default(t *testing.T) {
	// nil inputs should use BorderHighlightFocusColor (styles package)
	style := resolveActiveTabStyle(nil, nil)

	// Verify style properties using getter methods
	require.True(t, style.GetBold(), "active tab style should be bold")
	require.NotNil(t, style.GetForeground(), "active tab style should have a foreground color")
}

func TestResolveActiveTabStyle_CustomColor(t *testing.T) {
	// customColor should override default
	style := resolveActiveTabStyle(testColorGreen, nil)

	// Verify style properties
	require.True(t, style.GetBold(), "active tab style should be bold")

	// Foreground should be the custom color
	fg := style.GetForeground()
	require.Equal(t, testColorGreen, fg, "foreground should be customColor when tabColor is nil")
}

func TestResolveActiveTabStyle_TabColor(t *testing.T) {
	// tabColor should override customColor
	style := resolveActiveTabStyle(testColorGreen, testColorPurple)

	// Verify style properties
	require.True(t, style.GetBold(), "active tab style should be bold")

	// Foreground should be tabColor (overrides customColor)
	fg := style.GetForeground()
	require.Equal(t, testColorPurple, fg, "foreground should be tabColor (overrides customColor)")
}

func TestResolveActiveTabStyle_IsBold(t *testing.T) {
	// All active tab styles should have Bold(true)
	// Test with nil inputs
	style := resolveActiveTabStyle(nil, nil)
	require.True(t, style.GetBold(), "active tab style should be bold (nil inputs)")

	// Test with customColor only
	styleWithCustom := resolveActiveTabStyle(testColorGreen, nil)
	require.True(t, styleWithCustom.GetBold(), "active tab style should be bold (with customColor)")

	// Test with tabColor
	styleWithTab := resolveActiveTabStyle(testColorGreen, testColorPurple)
	require.True(t, styleWithTab.GetBold(), "active tab style should be bold (with tabColor)")
}

// =============================================================================
// Unit Tests for resolveInactiveTabStyle
// =============================================================================

func TestResolveInactiveTabStyle_Default(t *testing.T) {
	// nil input should use TextMutedColor (styles package)
	style := resolveInactiveTabStyle(nil)

	// Verify style properties
	require.False(t, style.GetBold(), "inactive tab style should not be bold")
	require.NotNil(t, style.GetForeground(), "inactive tab style should have a foreground color")
}

func TestResolveInactiveTabStyle_CustomColor(t *testing.T) {
	// customColor should override default
	style := resolveInactiveTabStyle(testColorGreen)

	// Verify style properties
	require.False(t, style.GetBold(), "inactive tab style should not be bold")

	// Foreground should be the custom color
	fg := style.GetForeground()
	require.Equal(t, testColorGreen, fg, "foreground should be customColor")
}

func TestResolveInactiveTabStyle_NotBold(t *testing.T) {
	// Inactive tab styles should NOT be bold
	// Compare with active style which IS bold
	inactiveStyle := resolveInactiveTabStyle(nil)
	activeStyle := resolveActiveTabStyle(nil, nil)

	// Verify bold difference directly
	require.False(t, inactiveStyle.GetBold(), "inactive tab style should not be bold")
	require.True(t, activeStyle.GetBold(), "active tab style should be bold")
}

// =============================================================================
// Unit Tests for buildTabTitleTopBorder
// =============================================================================

func TestBuildTabTitleTopBorder_TwoTabs(t *testing.T) {
	// Basic two-tab rendering with first tab active
	borderStyle := lipgloss.NewStyle()
	activeStyle := lipgloss.NewStyle().Bold(true)
	inactiveStyle := lipgloss.NewStyle()

	tabs := []Tab{
		{Label: "Overview"},
		{Label: "Comments"},
	}

	result := buildTabTitleTopBorder(tabs, 0, 40, borderStyle, activeStyle, inactiveStyle)

	// Should contain both tab labels
	require.Contains(t, result, "Overview", "missing first tab label")
	require.Contains(t, result, "Comments", "missing second tab label")

	// Should contain border corners
	require.Contains(t, result, "╭", "missing top-left corner")
	require.Contains(t, result, "╮", "missing top-right corner")

	// Should contain separator between tabs
	require.Contains(t, result, " ─ ", "missing separator between tabs")
}

func TestBuildTabTitleTopBorder_ThreeTabs(t *testing.T) {
	// Three tabs with middle tab (index 1) active
	borderStyle := lipgloss.NewStyle()
	activeStyle := lipgloss.NewStyle().Bold(true)
	inactiveStyle := lipgloss.NewStyle()

	tabs := []Tab{
		{Label: "Overview"},
		{Label: "Comments"},
		{Label: "History"},
	}

	result := buildTabTitleTopBorder(tabs, 1, 50, borderStyle, activeStyle, inactiveStyle)

	// Should contain all three tab labels
	require.Contains(t, result, "Overview", "missing first tab label")
	require.Contains(t, result, "Comments", "missing second (active) tab label")
	require.Contains(t, result, "History", "missing third tab label")

	// Should contain border corners
	require.Contains(t, result, "╭", "missing top-left corner")
	require.Contains(t, result, "╮", "missing top-right corner")
}

func TestBuildTabTitleTopBorderWithAction(t *testing.T) {
	borderStyle := lipgloss.NewStyle()
	activeStyle := lipgloss.NewStyle().Bold(true)
	inactiveStyle := lipgloss.NewStyle()
	actionStyle := lipgloss.NewStyle().Bold(true)

	tabs := []Tab{
		{Label: "Overview"},
		{Label: "Comments"},
	}

	result := buildTabTitleTopBorderWithAction(
		tabs, 0, HeaderAction{Label: "[+]"},
		44, borderStyle, activeStyle, inactiveStyle, actionStyle,
	)

	require.Contains(t, result, "Overview", "missing first tab label")
	require.Contains(t, result, "Comments", "missing second tab label")
	require.Contains(t, result, "[+]", "missing header action")
}

func TestBuildTabTitleTopBorder_SingleTab(t *testing.T) {
	// Single tab renders correctly
	borderStyle := lipgloss.NewStyle()
	activeStyle := lipgloss.NewStyle().Bold(true)
	inactiveStyle := lipgloss.NewStyle()

	tabs := []Tab{
		{Label: "Details"},
	}

	result := buildTabTitleTopBorder(tabs, 0, 30, borderStyle, activeStyle, inactiveStyle)

	// Should contain the single tab label
	require.Contains(t, result, "Details", "missing tab label")

	// Should contain border corners
	require.Contains(t, result, "╭", "missing top-left corner")
	require.Contains(t, result, "╮", "missing top-right corner")

	// Should NOT contain separator (only one tab)
	// Count occurrences of " ─ " - should be minimal (only in border structure, not between tabs)
	parts := strings.Split(result, " ─ ")
	require.LessOrEqual(t, len(parts), 2, "should have no separator between tabs with single tab")
}

func TestBuildTabTitleTopBorder_ActiveTabClamping(t *testing.T) {
	// Out-of-bounds activeTab (too high) should be clamped silently
	borderStyle := lipgloss.NewStyle()
	activeStyle := lipgloss.NewStyle().Bold(true)
	inactiveStyle := lipgloss.NewStyle()

	tabs := []Tab{
		{Label: "Tab1"},
		{Label: "Tab2"},
	}

	// activeTab = 10 is out of bounds (max is 1)
	result := buildTabTitleTopBorder(tabs, 10, 30, borderStyle, activeStyle, inactiveStyle)

	// Should render without panic
	require.Contains(t, result, "Tab1", "missing first tab label")
	require.Contains(t, result, "Tab2", "missing second tab label")
	require.Contains(t, result, "╭", "missing top-left corner")
	require.Contains(t, result, "╮", "missing top-right corner")
}

func TestBuildTabTitleTopBorder_NegativeActiveTab(t *testing.T) {
	// Negative activeTab should be clamped to 0
	borderStyle := lipgloss.NewStyle()
	activeStyle := lipgloss.NewStyle().Bold(true)
	inactiveStyle := lipgloss.NewStyle()

	tabs := []Tab{
		{Label: "Tab1"},
		{Label: "Tab2"},
	}

	// activeTab = -5 should be clamped to 0
	result := buildTabTitleTopBorder(tabs, -5, 30, borderStyle, activeStyle, inactiveStyle)

	// Should render without panic
	require.Contains(t, result, "Tab1", "missing first tab label")
	require.Contains(t, result, "Tab2", "missing second tab label")
	require.Contains(t, result, "╭", "missing top-left corner")
	require.Contains(t, result, "╮", "missing top-right corner")
}

func TestBuildTabTitleTopBorder_LongLabels(t *testing.T) {
	// Labels should truncate when exceeding available width
	borderStyle := lipgloss.NewStyle()
	activeStyle := lipgloss.NewStyle().Bold(true)
	inactiveStyle := lipgloss.NewStyle()

	tabs := []Tab{
		{Label: "This is a very long tab label that should be truncated"},
		{Label: "Another extremely long label here"},
	}

	// Width 40 is not enough for both full labels
	result := buildTabTitleTopBorder(tabs, 0, 40, borderStyle, activeStyle, inactiveStyle)

	// Should render without panic
	require.Contains(t, result, "╭", "missing top-left corner")
	require.Contains(t, result, "╮", "missing top-right corner")

	// Full labels should NOT appear (they should be truncated)
	require.NotContains(t, result, "should be truncated", "label should be truncated")
	require.NotContains(t, result, "extremely long label", "label should be truncated")

	// Should contain ellipsis from truncation
	require.Contains(t, result, "...", "truncated labels should have ellipsis")

	// Total width should not exceed innerWidth + 2 (for corners)
	lineWidth := lipgloss.Width(result)
	require.LessOrEqual(t, lineWidth, 42, "result width should not exceed expected width")
}

func TestBuildTabTitleTopBorder_UnicodeLabels(t *testing.T) {
	// Unicode (emoji, CJK) labels should measure width correctly
	borderStyle := lipgloss.NewStyle()
	activeStyle := lipgloss.NewStyle().Bold(true)
	inactiveStyle := lipgloss.NewStyle()

	tabs := []Tab{
		{Label: "概要"}, // CJK chars (4 visual width)
		{Label: "📝"},  // Emoji (2 visual width)
	}

	result := buildTabTitleTopBorder(tabs, 0, 30, borderStyle, activeStyle, inactiveStyle)

	// Should contain Unicode labels
	require.Contains(t, result, "概要", "missing CJK label")
	require.Contains(t, result, "📝", "missing emoji label")

	// Should contain border corners
	require.Contains(t, result, "╭", "missing top-left corner")
	require.Contains(t, result, "╮", "missing top-right corner")
}

func TestBuildTabTitleTopBorder_ExactFit(t *testing.T) {
	// Labels that exactly fit should not be truncated
	borderStyle := lipgloss.NewStyle()
	activeStyle := lipgloss.NewStyle().Bold(true)
	inactiveStyle := lipgloss.NewStyle()

	tabs := []Tab{
		{Label: "Tab1"},
		{Label: "Tab2"},
	}

	// Calculate exact width needed:
	// "╭─ Tab1 ─ Tab2 ─╮"
	// Corners: 2, "─ " prefix: 2, "Tab1": 4, " ─ " sep: 3, "Tab2": 4, " ─" suffix: 2 = 17 total
	// innerWidth = total - 2 (corners) = 15
	// But we also need trailing dashes, so let's give it a bit more
	result := buildTabTitleTopBorder(tabs, 0, 18, borderStyle, activeStyle, inactiveStyle)

	// Labels should NOT be truncated (no ellipsis)
	require.Contains(t, result, "Tab1", "missing first tab label")
	require.Contains(t, result, "Tab2", "missing second tab label")
	require.NotContains(t, result, "...", "labels should not be truncated when they fit")
}

func TestBuildTabTitleTopBorder_NarrowWidth(t *testing.T) {
	// Very narrow width should be handled gracefully
	borderStyle := lipgloss.NewStyle()
	activeStyle := lipgloss.NewStyle().Bold(true)
	inactiveStyle := lipgloss.NewStyle()

	tabs := []Tab{
		{Label: "Tab1"},
		{Label: "Tab2"},
	}

	// Very narrow width - should fall back to plain border or handle gracefully
	result := buildTabTitleTopBorder(tabs, 0, 5, borderStyle, activeStyle, inactiveStyle)

	// Should render without panic
	require.Contains(t, result, "╭", "missing top-left corner")
	require.Contains(t, result, "╮", "missing top-right corner")
}

// =============================================================================
// Unit Tests for BorderedPane Tab Mode
// =============================================================================

func TestBorderedPane_TabMode_BasicRendering(t *testing.T) {
	// Tabs render in title bar
	cfg := BorderConfig{
		Width:  40,
		Height: 5,
		Tabs: []Tab{
			{Label: "Overview", Content: "Overview content here"},
			{Label: "Comments", Content: "Comments content here"},
		},
		ActiveTab: 0,
	}

	result := BorderedPane(cfg)

	// Should contain both tab labels in the title area
	require.Contains(t, result, "Overview", "missing first tab label")
	require.Contains(t, result, "Comments", "missing second tab label")

	// Should contain border characters
	require.Contains(t, result, "╭", "missing top-left corner")
	require.Contains(t, result, "╮", "missing top-right corner")
	require.Contains(t, result, "╰", "missing bottom-left corner")
	require.Contains(t, result, "╯", "missing bottom-right corner")

	// Should contain content from active tab
	require.Contains(t, result, "Overview content", "missing active tab content")
}

func TestBorderedPane_TabMode_ActiveTabHighlighted(t *testing.T) {
	// Active tab should be styled differently (bold)
	cfg := BorderConfig{
		Width:  40,
		Height: 5,
		Tabs: []Tab{
			{Label: "Tab1", Content: "Content 1"},
			{Label: "Tab2", Content: "Content 2"},
		},
		ActiveTab: 1, // Second tab is active
	}

	result := BorderedPane(cfg)

	// Both tabs should be present
	require.Contains(t, result, "Tab1", "missing first tab label")
	require.Contains(t, result, "Tab2", "missing second tab label")

	// Content should be from the active tab (Tab2)
	require.Contains(t, result, "Content 2", "missing active tab content")
	require.NotContains(t, result, "Content 1", "should not show inactive tab content")
}

func TestBorderedPane_TabMode_ContentFromActiveTab(t *testing.T) {
	// Correct content displayed based on ActiveTab index
	cfg := BorderConfig{
		Width:  50,
		Height: 5,
		Tabs: []Tab{
			{Label: "First", Content: "First tab content"},
			{Label: "Second", Content: "Second tab content"},
			{Label: "Third", Content: "Third tab content"},
		},
		ActiveTab: 2, // Third tab is active
	}

	result := BorderedPane(cfg)

	// Should display content from third tab
	require.Contains(t, result, "Third tab content", "missing third tab content")
	require.NotContains(t, result, "First tab content", "should not show first tab content")
	require.NotContains(t, result, "Second tab content", "should not show second tab content")
}

func TestBorderedPane_TabMode_ZeroTabs(t *testing.T) {
	// Empty tabs slice should fall back to classic mode
	cfg := BorderConfig{
		Content: "Classic content",
		Width:   30,
		Height:  5,
		TopLeft: "Classic Title",
		Tabs:    []Tab{}, // Empty slice, not nil
	}

	result := BorderedPane(cfg)

	// Should render in classic mode with TopLeft title
	require.Contains(t, result, "Classic Title", "missing classic mode title")
	require.Contains(t, result, "Classic content", "missing classic mode content")
}

func TestBorderedPane_TabMode_IgnoresTopLeftTitle(t *testing.T) {
	// TopLeft, TopRight, TitleColor should be ignored in tab mode
	cfg := BorderConfig{
		Content:  "This should be ignored",
		Width:    40,
		Height:   5,
		TopLeft:  "IGNORED TITLE",
		TopRight: "IGNORED RIGHT",
		Tabs: []Tab{
			{Label: "TabLabel", Content: "Tab content"},
		},
		ActiveTab: 0,
	}

	result := BorderedPane(cfg)

	// Tab label should appear
	require.Contains(t, result, "TabLabel", "missing tab label")

	// TopLeft and TopRight titles should NOT appear
	require.NotContains(t, result, "IGNORED TITLE", "TopLeft should be ignored in tab mode")
	require.NotContains(t, result, "IGNORED RIGHT", "TopRight should be ignored in tab mode")

	// Tab content should appear, not cfg.Content
	require.Contains(t, result, "Tab content", "missing tab content")
	require.NotContains(t, result, "This should be ignored", "cfg.Content should be ignored in tab mode")
}

func TestBorderedPane_TabMode_BottomTitlesWork(t *testing.T) {
	// BottomLeft and BottomRight should work in tab mode
	cfg := BorderConfig{
		Width:       40,
		Height:      5,
		BottomLeft:  "Help: ?",
		BottomRight: "Page 1/3",
		Tabs: []Tab{
			{Label: "Details", Content: "Details content"},
		},
		ActiveTab: 0,
	}

	result := BorderedPane(cfg)

	// Tab label should appear
	require.Contains(t, result, "Details", "missing tab label")

	// Bottom titles should appear
	require.Contains(t, result, "Help: ?", "missing bottom-left title")
	require.Contains(t, result, "Page 1/3", "missing bottom-right title")
}

func TestBorderedPane_TabMode_FocusedBorder(t *testing.T) {
	// Focused flag should affect border color, not tab styling
	cfgUnfocused := BorderConfig{
		Width:              40,
		Height:             5,
		Focused:            false,
		BorderColor:        testColorBlue,
		FocusedBorderColor: testColorGreen,
		Tabs: []Tab{
			{Label: "Tab1", Content: "Content"},
		},
		ActiveTab: 0,
	}

	cfgFocused := BorderConfig{
		Width:              40,
		Height:             5,
		Focused:            true,
		BorderColor:        testColorBlue,
		FocusedBorderColor: testColorGreen,
		Tabs: []Tab{
			{Label: "Tab1", Content: "Content"},
		},
		ActiveTab: 0,
	}

	unfocusedResult := BorderedPane(cfgUnfocused)
	focusedResult := BorderedPane(cfgFocused)

	// Both should have valid structure
	require.Contains(t, unfocusedResult, "Tab1", "unfocused missing tab label")
	require.Contains(t, focusedResult, "Tab1", "focused missing tab label")
	require.Contains(t, unfocusedResult, "Content", "unfocused missing content")
	require.Contains(t, focusedResult, "Content", "focused missing content")

	// Both should have same line count
	unfocusedLines := strings.Split(unfocusedResult, "\n")
	focusedLines := strings.Split(focusedResult, "\n")
	require.Equal(t, len(unfocusedLines), len(focusedLines), "focused and unfocused should have same line count")
}

func TestBorderedPane_TabMode_CustomActiveColor(t *testing.T) {
	// ActiveTabColor should be used for active tab styling
	cfg := BorderConfig{
		Width:          40,
		Height:         5,
		ActiveTabColor: testColorPurple,
		Tabs: []Tab{
			{Label: "Active", Content: "Active content"},
			{Label: "Inactive", Content: "Inactive content"},
		},
		ActiveTab: 0,
	}

	result := BorderedPane(cfg)

	// Should render without error
	require.Contains(t, result, "Active", "missing active tab label")
	require.Contains(t, result, "Inactive", "missing inactive tab label")
	require.Contains(t, result, "Active content", "missing active tab content")
}

func TestBorderedPane_TabMode_CustomInactiveColor(t *testing.T) {
	// InactiveTabColor should be used for inactive tab styling
	cfg := BorderConfig{
		Width:            40,
		Height:           5,
		InactiveTabColor: testColorGreen,
		Tabs: []Tab{
			{Label: "Active", Content: "Active content"},
			{Label: "Inactive", Content: "Inactive content"},
		},
		ActiveTab: 0,
	}

	result := BorderedPane(cfg)

	// Should render without error
	require.Contains(t, result, "Active", "missing active tab label")
	require.Contains(t, result, "Inactive", "missing inactive tab label")
	require.Contains(t, result, "Active content", "missing active tab content")
}

func TestBorderedPane_TabMode_PerTabCustomColor(t *testing.T) {
	// Tab.Color should override ActiveTabColor for that specific tab
	cfg := BorderConfig{
		Width:          40,
		Height:         5,
		ActiveTabColor: testColorBlue, // Global active color
		Tabs: []Tab{
			{Label: "CustomColor", Content: "Content 1", Color: testColorPurple}, // Per-tab custom color
			{Label: "Normal", Content: "Content 2"},
		},
		ActiveTab: 0, // First tab is active, which has a custom color
	}

	result := BorderedPane(cfg)

	// Should render without error
	require.Contains(t, result, "CustomColor", "missing tab with custom color")
	require.Contains(t, result, "Normal", "missing normal tab")
	require.Contains(t, result, "Content 1", "missing active tab content")
}

// =============================================================================
// Golden Tests for Tab Mode
// =============================================================================

func TestBorderedPane_Golden_TabMode_TwoTabs(t *testing.T) {
	cfg := BorderConfig{
		Width:  50,
		Height: 7,
		Tabs: []Tab{
			{Label: "Overview", Content: "This is the overview content.\nWith multiple lines."},
			{Label: "Comments", Content: "Comments would appear here."},
		},
		ActiveTab: 0,
	}

	result := BorderedPane(cfg)
	teatest.RequireEqualOutput(t, []byte(result))
}

func TestBorderedPane_Golden_TabMode_ThreeTabs(t *testing.T) {
	cfg := BorderConfig{
		Width:  60,
		Height: 7,
		Tabs: []Tab{
			{Label: "Overview", Content: "Overview content"},
			{Label: "Comments", Content: "Comments content"},
			{Label: "History", Content: "History content"},
		},
		ActiveTab: 1, // Middle tab is active
	}

	result := BorderedPane(cfg)
	teatest.RequireEqualOutput(t, []byte(result))
}

func TestBorderedPane_Golden_TabMode_NarrowWidth(t *testing.T) {
	cfg := BorderConfig{
		Width:  25,
		Height: 5,
		Tabs: []Tab{
			{Label: "Tab1", Content: "Short"},
			{Label: "Tab2", Content: "Content"},
		},
		ActiveTab: 0,
	}

	result := BorderedPane(cfg)
	teatest.RequireEqualOutput(t, []byte(result))
}

func TestBorderedPane_Golden_TabMode_WithBottom(t *testing.T) {
	cfg := BorderConfig{
		Width:       50,
		Height:      7,
		BottomLeft:  "Help: ?",
		BottomRight: "1/3",
		Tabs: []Tab{
			{Label: "Details", Content: "Detail information here.\nMore details below."},
			{Label: "Files", Content: "File listing here."},
		},
		ActiveTab: 0,
	}

	result := BorderedPane(cfg)
	teatest.RequireEqualOutput(t, []byte(result))
}

func TestBorderedPane_Golden_TabMode_SingleTab(t *testing.T) {
	cfg := BorderConfig{
		Width:  40,
		Height: 5,
		Tabs: []Tab{
			{Label: "Only Tab", Content: "Single tab content here."},
		},
		ActiveTab: 0,
	}

	result := BorderedPane(cfg)
	teatest.RequireEqualOutput(t, []byte(result))
}

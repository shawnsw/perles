package bql

import (
	"github.com/zjrosen/perles/internal/ui/styles"

	"github.com/charmbracelet/lipgloss"
)

// Token highlight styles for BQL syntax highlighting.
// Uses centralized color constants from the styles package.
// These are package-level vars that get rebuilt when theme changes.
var (
	KeywordStyle  lipgloss.Style
	OperatorStyle lipgloss.Style
	FieldStyle    lipgloss.Style
	StringStyle   lipgloss.Style
	LiteralStyle  lipgloss.Style
	ParenStyle    lipgloss.Style
	CommaStyle    lipgloss.Style
	DefaultStyle  lipgloss.Style
)

func init() {
	RebuildStyles()
	// Register with styles package so BQL styles get rebuilt when theme changes
	styles.RegisterStyleRebuilder(RebuildStyles)
}

// RebuildStyles recreates all BQL styles from current color values.
// Called by styles.ApplyTheme after colors are updated.
func RebuildStyles() {
	// KeywordStyle for logical operators: and, or, not, in, order, by, asc, desc
	KeywordStyle = lipgloss.NewStyle().
		Foreground(styles.BQLKeywordColor).
		Bold(true)

	// OperatorStyle for comparison operators: =, !=, <, >, <=, >=, ~, !~
	OperatorStyle = lipgloss.NewStyle().
		Foreground(styles.BQLOperatorColor)

	// FieldStyle for field names (identifiers)
	FieldStyle = lipgloss.NewStyle().
		Foreground(styles.BQLFieldColor)

	// StringStyle for quoted string values
	StringStyle = lipgloss.NewStyle().
		Foreground(styles.BQLStringColor)

	// LiteralStyle for boolean and numeric values: true, false, numbers
	LiteralStyle = lipgloss.NewStyle().
		Foreground(styles.BQLLiteralColor)

	// ParenStyle for parentheses
	ParenStyle = lipgloss.NewStyle().
		Foreground(styles.BQLParenColor).
		Bold(true)

	// CommaStyle for comma separators
	CommaStyle = lipgloss.NewStyle().
		Foreground(styles.BQLCommaColor)

	// DefaultStyle for unrecognized tokens
	DefaultStyle = lipgloss.NewStyle()
}

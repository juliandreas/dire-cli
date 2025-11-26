package constants

import "github.com/charmbracelet/lipgloss"

// Colors.

func Gray() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{
		Light: "#666666",
		Dark:  "#808080",
	})
}

func Yellow() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{
		Light: "#B8860B",
		Dark:  "#FBFBB8",
	})
}

func Blue() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{
		Light: "#0066CC",
		Dark:  "#53A4FF",
	})
}

// Styling.

func Bold() lipgloss.Style {
	return lipgloss.NewStyle().Bold(true)
}

// Symbols.

func Star() string {
	return Yellow().Render("✦")
}

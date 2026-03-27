package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// HelpOverlay displays keyboard shortcuts
type HelpOverlay struct {
	visible bool
	width   int
	height  int
}

// NewHelpOverlay creates a new help overlay
func NewHelpOverlay() *HelpOverlay {
	return &HelpOverlay{}
}

// Toggle shows/hides the overlay
func (h *HelpOverlay) Toggle() {
	h.visible = !h.visible
}

// Show displays the overlay
func (h *HelpOverlay) Show() {
	h.visible = true
}

// Hide hides the overlay
func (h *HelpOverlay) Hide() {
	h.visible = false
}

// IsVisible returns whether the overlay is visible
func (h *HelpOverlay) IsVisible() bool {
	return h.visible
}

// SetSize updates the dimensions
func (h *HelpOverlay) SetSize(width, height int) {
	h.width = width
	h.height = height
}

// View renders the help overlay
func (h *HelpOverlay) View() string {
	if !h.visible {
		return ""
	}

	dialogWidth := 60

	dialogStyle := lipgloss.NewStyle().
		Border(lipgloss.DoubleBorder()).
		BorderForeground(DefaultTheme.Primary).
		Padding(1, 2).
		Width(dialogWidth)

	sectionStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(DefaultTheme.Primary).
		MarginTop(1)

	keyStyle := lipgloss.NewStyle().
		Foreground(DefaultTheme.Accent).
		Width(12)

	descStyle := lipgloss.NewStyle().
		Foreground(DefaultTheme.Text)

	var content strings.Builder
	content.WriteString(S.Title.Render(Icons.Helm+" Keyboard Shortcuts") + "\n")

	// Global shortcuts
	content.WriteString(sectionStyle.Render("Global") + "\n")
	content.WriteString(renderShortcut(keyStyle, descStyle, "0 / w", "Welcome / About page"))
	content.WriteString(renderShortcut(keyStyle, descStyle, "1", "Releases view"))
	content.WriteString(renderShortcut(keyStyle, descStyle, "2", "Explore view"))
	content.WriteString(renderShortcut(keyStyle, descStyle, "3", "Repos view"))
	content.WriteString(renderShortcut(keyStyle, descStyle, "4", "Settings view"))
	content.WriteString(renderShortcut(keyStyle, descStyle, "?", "Toggle this help"))
	content.WriteString(renderShortcut(keyStyle, descStyle, "q", "Quit / Back"))
	content.WriteString(renderShortcut(keyStyle, descStyle, "Esc", "Back / Cancel"))

	// Releases view
	content.WriteString(sectionStyle.Render("Releases View") + "\n")
	content.WriteString(renderShortcut(keyStyle, descStyle, "Enter", "Open release details"))
	content.WriteString(renderShortcut(keyStyle, descStyle, "Space", "Toggle select for bulk operations"))
	content.WriteString(renderShortcut(keyStyle, descStyle, "U", "Bulk upgrade selected releases"))
	content.WriteString(renderShortcut(keyStyle, descStyle, "c", "Clear all selections"))
	content.WriteString(renderShortcut(keyStyle, descStyle, "d", "Delete release"))
	content.WriteString(renderShortcut(keyStyle, descStyle, "n", "Cycle namespaces"))
	content.WriteString(renderShortcut(keyStyle, descStyle, "r", "Refresh list"))
	content.WriteString(renderShortcut(keyStyle, descStyle, "j/k", "Navigate up/down"))
	content.WriteString(renderShortcut(keyStyle, descStyle, "/", "Filter releases"))

	// Release detail view
	content.WriteString(sectionStyle.Render("Release Details") + "\n")
	content.WriteString(renderShortcut(keyStyle, descStyle, "Tab", "Cycle: Info→History→Values→Resources"))
	content.WriteString(renderShortcut(keyStyle, descStyle, "u", "Upgrade release (opens editor)"))
	content.WriteString(renderShortcut(keyStyle, descStyle, "d", "Show diff preview"))
	content.WriteString(renderShortcut(keyStyle, descStyle, "r", "Rollback selected revision"))
	content.WriteString(renderShortcut(keyStyle, descStyle, "R", "View chart README"))
	content.WriteString(renderShortcut(keyStyle, descStyle, "p", "Refresh pod status"))
	content.WriteString(renderShortcut(keyStyle, descStyle, "l", "View pod logs"))
	content.WriteString(renderShortcut(keyStyle, descStyle, "x", "Exec into pod (shell access)"))
	content.WriteString(renderShortcut(keyStyle, descStyle, "f", "Port forward manager"))
	content.WriteString(renderShortcut(keyStyle, descStyle, "c", "Compare selected revision with current"))

	// Explore view
	content.WriteString(sectionStyle.Render("Explore View") + "\n")
	content.WriteString(renderShortcut(keyStyle, descStyle, "Enter", "Search / Load chart"))
	content.WriteString(renderShortcut(keyStyle, descStyle, "Tab", "Switch panes"))
	content.WriteString(renderShortcut(keyStyle, descStyle, "i", "Install chart"))
	content.WriteString(renderShortcut(keyStyle, descStyle, "a", "Add chart to multi-install queue"))
	content.WriteString(renderShortcut(keyStyle, descStyle, "M", "Open multi-chart install dialog"))
	content.WriteString(renderShortcut(keyStyle, descStyle, "v", "Select chart version"))
	content.WriteString(renderShortcut(keyStyle, descStyle, "s", "Security scan (Trivy)"))
	content.WriteString(renderShortcut(keyStyle, descStyle, "D", "View dependency tree"))
	content.WriteString(renderShortcut(keyStyle, descStyle, "S", "Browse values schema"))
	content.WriteString(renderShortcut(keyStyle, descStyle, "r", "View README"))
	content.WriteString(renderShortcut(keyStyle, descStyle, "t", "Template to file (GitOps)"))
	content.WriteString(descStyle.Render("  Supports: repo/chart, oci://registry/chart, local path") + "\n")

	// Multi-chart install
	content.WriteString(sectionStyle.Render("Multi-Chart Install") + "\n")
	content.WriteString(renderShortcut(keyStyle, descStyle, "j/k", "Navigate queue"))
	content.WriteString(renderShortcut(keyStyle, descStyle, "J/K", "Reorder charts in queue"))
	content.WriteString(renderShortcut(keyStyle, descStyle, "d", "Remove from queue"))
	content.WriteString(renderShortcut(keyStyle, descStyle, "e", "Edit chart values"))
	content.WriteString(renderShortcut(keyStyle, descStyle, "w", "Toggle wait for ready"))
	content.WriteString(renderShortcut(keyStyle, descStyle, "c", "Clear queue"))

	// Install dialog
	content.WriteString(sectionStyle.Render("Install Dialog") + "\n")
	content.WriteString(renderShortcut(keyStyle, descStyle, "Tab", "Next field"))
	content.WriteString(renderShortcut(keyStyle, descStyle, "j/k", "Select template (if available)"))
	content.WriteString(renderShortcut(keyStyle, descStyle, "e", "Edit values in $EDITOR"))
	content.WriteString(renderShortcut(keyStyle, descStyle, "f", "Import values from file"))
	content.WriteString(renderShortcut(keyStyle, descStyle, "d", "Dry-run preview"))
	content.WriteString(renderShortcut(keyStyle, descStyle, "T", "Save values as template"))
	content.WriteString(renderShortcut(keyStyle, descStyle, "Enter", "Confirm / Install"))

	// README viewer
	content.WriteString(sectionStyle.Render("README Viewer") + "\n")
	content.WriteString(renderShortcut(keyStyle, descStyle, "/", "Start search"))
	content.WriteString(renderShortcut(keyStyle, descStyle, "n", "Next match"))
	content.WriteString(renderShortcut(keyStyle, descStyle, "N", "Previous match"))
	content.WriteString(renderShortcut(keyStyle, descStyle, "j/k", "Scroll up/down"))
	content.WriteString(renderShortcut(keyStyle, descStyle, "g/G", "Go to top/bottom"))
	content.WriteString(renderShortcut(keyStyle, descStyle, "q/Esc", "Close"))

	// Repos view
	content.WriteString(sectionStyle.Render("Repos View") + "\n")
	content.WriteString(renderShortcut(keyStyle, descStyle, "a", "Add repository"))
	content.WriteString(renderShortcut(keyStyle, descStyle, "d", "Delete repository"))
	content.WriteString(renderShortcut(keyStyle, descStyle, "u", "Update selected"))
	content.WriteString(renderShortcut(keyStyle, descStyle, "U", "Update all repos"))

	// Settings view
	content.WriteString(sectionStyle.Render("Settings View") + "\n")
	content.WriteString(renderShortcut(keyStyle, descStyle, "a", "Add registry"))
	content.WriteString(renderShortcut(keyStyle, descStyle, "d", "Delete registry"))
	content.WriteString(renderShortcut(keyStyle, descStyle, "e", "Edit settings"))
	content.WriteString(renderShortcut(keyStyle, descStyle, "t", "Change theme"))
	content.WriteString(renderShortcut(keyStyle, descStyle, "T", "Manage install templates"))
	content.WriteString(renderShortcut(keyStyle, descStyle, "s", "Toggle security scanning"))
	content.WriteString(renderShortcut(keyStyle, descStyle, "r", "Reset to defaults"))
	content.WriteString(descStyle.Render("  Config: ~/.config/helmx/config.yaml") + "\n")

	content.WriteString("\n" + S.Muted.Render("Press ? or Esc to close"))

	dialog := dialogStyle.Render(content.String())

	return lipgloss.Place(h.width, h.height, lipgloss.Center, lipgloss.Center, dialog)
}

func renderShortcut(keyStyle, descStyle lipgloss.Style, key, desc string) string {
	return keyStyle.Render(key) + descStyle.Render(desc) + "\n"
}

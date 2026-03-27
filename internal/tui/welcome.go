package tui

import (
	"strings"

	"github.com/yasharora2020/helmx/internal/config"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// WelcomeView displays the welcome page with app info and keybindings
type WelcomeView struct {
	width         int
	height        int
	viewport      viewport.Model
	showOnStart   bool
	config        *config.Config
	contentHeight int
}

// NewWelcomeView creates a new welcome view
func NewWelcomeView(cfg *config.Config) *WelcomeView {
	vp := viewport.New(80, 20)
	vp.Style = lipgloss.NewStyle()

	return &WelcomeView{
		viewport:    vp,
		showOnStart: cfg.ShowWelcomeOnStart,
		config:      cfg,
	}
}

// SetSize updates the dimensions
func (w *WelcomeView) SetSize(width, height int) {
	w.width = width
	w.height = height

	// Reserve space for footer (checkbox + hint)
	contentHeight := height - 4
	if contentHeight < 10 {
		contentHeight = 10
	}

	w.viewport.Width = width - 4
	w.viewport.Height = contentHeight
	w.viewport.SetContent(w.buildContent())
	w.contentHeight = contentHeight
}

// Update handles input events
func (w *WelcomeView) Update(msg tea.Msg) (*WelcomeView, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case " ":
			// Toggle "show on startup" checkbox
			w.showOnStart = !w.showOnStart
			w.config.ShowWelcomeOnStart = w.showOnStart
			_ = w.config.Save() // Ignore error - best effort save
			return w, nil
		case "j", "down":
			w.viewport.LineDown(1)
			return w, nil
		case "k", "up":
			w.viewport.LineUp(1)
			return w, nil
		case "g":
			w.viewport.GotoTop()
			return w, nil
		case "G":
			w.viewport.GotoBottom()
			return w, nil
		case "ctrl+d":
			w.viewport.HalfViewDown()
			return w, nil
		case "ctrl+u":
			w.viewport.HalfViewUp()
			return w, nil
		}
	}

	var cmd tea.Cmd
	w.viewport, cmd = w.viewport.Update(msg)
	return w, cmd
}

// View renders the welcome page
func (w *WelcomeView) View() string {
	// Main content in viewport
	contentArea := w.viewport.View()

	// Checkbox for "show on startup"
	checkbox := "[ ]"
	if w.showOnStart {
		checkbox = "[" + S.Success.Render("x") + "]"
	}
	checkboxLine := lipgloss.NewStyle().
		Foreground(DefaultTheme.Text).
		Render(checkbox + " Show this page on startup")

	// Footer hint
	footerHint := S.Muted.Render("Space:toggle  j/k:scroll  Enter/1-4:continue  q:quit")

	// Scroll indicator
	scrollInfo := ""
	if w.viewport.TotalLineCount() > w.viewport.Height {
		scrollPct := int(w.viewport.ScrollPercent() * 100)
		scrollInfo = S.Muted.Render(" (" + lipgloss.NewStyle().Render(string(rune('0'+scrollPct/10))) + string(rune('0'+scrollPct%10)) + "%)")
	}

	// Build footer
	footer := lipgloss.JoinVertical(lipgloss.Left,
		"",
		checkboxLine+scrollInfo,
		footerHint,
	)

	// Combine content and footer
	fullContent := lipgloss.JoinVertical(lipgloss.Left,
		contentArea,
		footer,
	)

	return lipgloss.Place(w.width, w.height, lipgloss.Center, lipgloss.Top, fullContent)
}

// buildContent generates the welcome page content
func (w *WelcomeView) buildContent() string {
	// Styles
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(DefaultTheme.Primary)

	subtitleStyle := lipgloss.NewStyle().
		Foreground(DefaultTheme.Text)

	sectionStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(DefaultTheme.Primary).
		MarginTop(1)

	keyStyle := lipgloss.NewStyle().
		Foreground(DefaultTheme.Accent).
		Width(12)

	descStyle := lipgloss.NewStyle().
		Foreground(DefaultTheme.Text)

	mutedStyle := lipgloss.NewStyle().
		Foreground(DefaultTheme.Muted)

	dividerStyle := lipgloss.NewStyle().
		Foreground(DefaultTheme.Muted)

	logoStyle := lipgloss.NewStyle().
		Foreground(DefaultTheme.Primary).
		Bold(true)

	// Calculate column width (half of available width minus gap)
	colWidth := (w.width - 8) / 2
	if colWidth < 30 {
		colWidth = 30
	}

	// === Header section (full width, centered) ===
	var header strings.Builder
	header.WriteString(logoStyle.Render(HelmXLogo) + "\n\n")
	header.WriteString(titleStyle.Render("Welcome to HelmX") + "\n")
	header.WriteString(subtitleStyle.Render("Helm Explorer TUI") + "\n\n")
	header.WriteString(mutedStyle.Render("A k9s-style terminal UI for Helm — manage releases, explore charts,") + "\n")
	header.WriteString(mutedStyle.Render("and handle deployments without leaving your terminal.") + "\n")

	divider := dividerStyle.Render(strings.Repeat("─", min(w.width-4, 90)))
	header.WriteString("\n" + divider + "\n")

	// Quick Start
	header.WriteString(sectionStyle.Render("Quick Start") + "\n")
	header.WriteString(descStyle.Render("  1. Press ") + keyStyle.Render("1") + descStyle.Render(" to view your Helm releases") + "\n")
	header.WriteString(descStyle.Render("  2. Press ") + keyStyle.Render("2") + descStyle.Render(" to explore and install new charts") + "\n")
	header.WriteString(descStyle.Render("  3. Press ") + keyStyle.Render("3") + descStyle.Render(" to manage Helm repositories") + "\n")
	header.WriteString(descStyle.Render("  4. Press ") + keyStyle.Render("4") + descStyle.Render(" to configure settings") + "\n")
	header.WriteString(descStyle.Render("  5. Press ") + keyStyle.Render("?") + descStyle.Render(" anytime for keyboard shortcuts") + "\n")

	header.WriteString("\n" + divider + "\n")

	// === Left column: Global, Releases, Release Details, Repos ===
	var left strings.Builder

	left.WriteString(sectionStyle.Render("Global Shortcuts") + "\n")
	left.WriteString(w.renderShortcut(keyStyle, descStyle, "1-4", "Switch views"))
	left.WriteString(w.renderShortcut(keyStyle, descStyle, "0 / w", "Welcome page"))
	left.WriteString(w.renderShortcut(keyStyle, descStyle, "?", "Help overlay"))
	left.WriteString(w.renderShortcut(keyStyle, descStyle, "q", "Quit / Back"))
	left.WriteString(w.renderShortcut(keyStyle, descStyle, "Esc", "Back / Cancel"))

	left.WriteString(sectionStyle.Render("Releases View") + "\n")
	left.WriteString(w.renderShortcut(keyStyle, descStyle, "Enter", "Open release details"))
	left.WriteString(w.renderShortcut(keyStyle, descStyle, "Space", "Toggle select (bulk)"))
	left.WriteString(w.renderShortcut(keyStyle, descStyle, "U", "Bulk upgrade"))
	left.WriteString(w.renderShortcut(keyStyle, descStyle, "c", "Clear selections"))
	left.WriteString(w.renderShortcut(keyStyle, descStyle, "d", "Delete release"))
	left.WriteString(w.renderShortcut(keyStyle, descStyle, "n", "Cycle namespaces"))
	left.WriteString(w.renderShortcut(keyStyle, descStyle, "r", "Refresh list"))
	left.WriteString(w.renderShortcut(keyStyle, descStyle, "/", "Filter releases"))
	left.WriteString(w.renderShortcut(keyStyle, descStyle, "j/k", "Navigate up/down"))

	left.WriteString(sectionStyle.Render("Release Details") + "\n")
	left.WriteString(w.renderShortcut(keyStyle, descStyle, "Tab", "Cycle panes"))
	left.WriteString(w.renderShortcut(keyStyle, descStyle, "u", "Upgrade (editor)"))
	left.WriteString(w.renderShortcut(keyStyle, descStyle, "d", "Diff preview"))
	left.WriteString(w.renderShortcut(keyStyle, descStyle, "r", "Rollback revision"))
	left.WriteString(w.renderShortcut(keyStyle, descStyle, "c", "Compare revisions"))
	left.WriteString(w.renderShortcut(keyStyle, descStyle, "p", "Refresh pods"))
	left.WriteString(w.renderShortcut(keyStyle, descStyle, "l", "Pod logs"))
	left.WriteString(w.renderShortcut(keyStyle, descStyle, "x", "Exec into pod"))
	left.WriteString(w.renderShortcut(keyStyle, descStyle, "f", "Port forward"))

	left.WriteString(sectionStyle.Render("Repos View") + "\n")
	left.WriteString(w.renderShortcut(keyStyle, descStyle, "a", "Add repository"))
	left.WriteString(w.renderShortcut(keyStyle, descStyle, "d", "Delete repository"))
	left.WriteString(w.renderShortcut(keyStyle, descStyle, "u", "Update selected"))
	left.WriteString(w.renderShortcut(keyStyle, descStyle, "U", "Update all repos"))
	left.WriteString(w.renderShortcut(keyStyle, descStyle, "r", "Refresh list"))

	// === Right column: Explore, Install, Settings, RBAC, Optional ===
	var right strings.Builder

	right.WriteString(sectionStyle.Render("Explore View") + "\n")
	right.WriteString(w.renderShortcut(keyStyle, descStyle, "Enter", "Search / Load chart"))
	right.WriteString(w.renderShortcut(keyStyle, descStyle, "Tab", "Switch panes"))
	right.WriteString(w.renderShortcut(keyStyle, descStyle, "i", "Install chart"))
	right.WriteString(w.renderShortcut(keyStyle, descStyle, "v", "Select version"))
	right.WriteString(w.renderShortcut(keyStyle, descStyle, "s", "Security scan (Trivy)"))
	right.WriteString(w.renderShortcut(keyStyle, descStyle, "S", "Values schema"))
	right.WriteString(w.renderShortcut(keyStyle, descStyle, "D", "Dependency tree"))
	right.WriteString(w.renderShortcut(keyStyle, descStyle, "r", "View README"))
	right.WriteString(w.renderShortcut(keyStyle, descStyle, "t", "Template to file"))
	right.WriteString(w.renderShortcut(keyStyle, descStyle, "a", "Add to multi-install"))
	right.WriteString(w.renderShortcut(keyStyle, descStyle, "M", "Multi-chart queue"))

	right.WriteString(sectionStyle.Render("Install Dialog") + "\n")
	right.WriteString(w.renderShortcut(keyStyle, descStyle, "Tab", "Next field"))
	right.WriteString(w.renderShortcut(keyStyle, descStyle, "e", "Edit values ($EDITOR)"))
	right.WriteString(w.renderShortcut(keyStyle, descStyle, "f", "Import values file"))
	right.WriteString(w.renderShortcut(keyStyle, descStyle, "d", "Dry-run preview"))
	right.WriteString(w.renderShortcut(keyStyle, descStyle, "T", "Save as template"))
	right.WriteString(w.renderShortcut(keyStyle, descStyle, "Enter", "Confirm / Install"))

	right.WriteString(sectionStyle.Render("Settings View") + "\n")
	right.WriteString(w.renderShortcut(keyStyle, descStyle, "t", "Change theme"))
	right.WriteString(w.renderShortcut(keyStyle, descStyle, "s", "Toggle security scan"))
	right.WriteString(w.renderShortcut(keyStyle, descStyle, "c", "Switch K8s context"))
	right.WriteString(w.renderShortcut(keyStyle, descStyle, "e", "Edit settings"))
	right.WriteString(w.renderShortcut(keyStyle, descStyle, "a", "Add registry"))
	right.WriteString(w.renderShortcut(keyStyle, descStyle, "d", "Delete registry"))

	right.WriteString(sectionStyle.Render("RBAC View") + "\n")
	right.WriteString(w.renderShortcut(keyStyle, descStyle, "a", "Add user"))
	right.WriteString(w.renderShortcut(keyStyle, descStyle, "e", "Edit user"))
	right.WriteString(w.renderShortcut(keyStyle, descStyle, "+", "Add namespace access"))
	right.WriteString(w.renderShortcut(keyStyle, descStyle, "d", "Delete user"))
	right.WriteString(w.renderShortcut(keyStyle, descStyle, "K", "Export kubeconfig"))

	right.WriteString(sectionStyle.Render("Optional Requirements") + "\n")
	right.WriteString(mutedStyle.Render("  • metrics-server — CPU/Memory display") + "\n")
	right.WriteString(mutedStyle.Render("  • Trivy — security scanning") + "\n")
	right.WriteString(mutedStyle.Render("  • helm-secrets — SOPS encryption") + "\n")
	right.WriteString(mutedStyle.Render("  Config: ~/.config/helmx/config.yaml") + "\n")

	// === Compose 2-column layout ===
	leftCol := lipgloss.NewStyle().Width(colWidth).Render(left.String())
	rightCol := lipgloss.NewStyle().Width(colWidth).Render(right.String())
	columns := lipgloss.JoinHorizontal(lipgloss.Top, leftCol, "  ", rightCol)

	return header.String() + "\n" + columns
}

// renderShortcut formats a single shortcut line
func (w *WelcomeView) renderShortcut(keyStyle, descStyle lipgloss.Style, key, desc string) string {
	return "  " + keyStyle.Render(key) + descStyle.Render(desc) + "\n"
}

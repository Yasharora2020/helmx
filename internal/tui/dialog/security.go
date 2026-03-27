package dialog

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// SecurityRescanMsg is sent when the user requests a rescan.
type SecurityRescanMsg struct{}

// Misconfiguration represents a security issue found by Trivy.
type Misconfiguration struct {
	ID          string
	Title       string
	Description string
	Message     string
	Severity    string
}

// ScanResult represents scan results for a single file/target.
type ScanResult struct {
	Target            string
	Misconfigurations []Misconfiguration
}

// SecurityScanResults holds all security scan results.
type SecurityScanResults struct {
	Results  []ScanResult
	Critical int
	High     int
	Medium   int
	Low      int
	Total    int
}

// SecurityDialog displays Trivy security scan results.
type SecurityDialog struct {
	BaseDialog
	viewport viewport.Model

	// Chart info
	chartName    string
	chartVersion string

	// Scan state
	results *SecurityScanResults
	loading bool
	lastErr error

	// Spinner view (passed from parent to avoid import cycles)
	spinnerView func() string

	// Styles (injected from parent)
	TitleStyle    lipgloss.Style
	SubtitleStyle lipgloss.Style
	MutedStyle    lipgloss.Style
	ValueStyle    lipgloss.Style
	ErrorStyle    lipgloss.Style
	SuccessStyle  lipgloss.Style
	WarningStyle  lipgloss.Style
	BorderColor   lipgloss.Color
	LockIcon      string
	CheckIcon     string
	CrossIcon     string
}

// NewSecurityDialog creates a new security scan dialog.
func NewSecurityDialog() *SecurityDialog {
	vp := viewport.New(70, 20)
	return &SecurityDialog{
		viewport: vp,
	}
}

// Open opens the dialog and resets state.
func (d *SecurityDialog) Open() {
	d.BaseDialog.Open()
	d.loading = true
	d.lastErr = nil
	d.results = nil
	d.viewport.GotoTop()
}

// OpenForChart opens the dialog for a specific chart.
func (d *SecurityDialog) OpenForChart(chartName, chartVersion string) {
	d.chartName = chartName
	d.chartVersion = chartVersion
	d.BaseDialog.Open()
	d.loading = true
	d.lastErr = nil
	d.results = nil
	d.viewport.GotoTop()
}

// SetResults sets the scan results.
func (d *SecurityDialog) SetResults(results *SecurityScanResults) {
	d.results = results
	d.loading = false
	d.lastErr = nil
	d.updateViewportContent()
}

// SetError sets a scan error.
func (d *SecurityDialog) SetError(err error) {
	d.lastErr = err
	d.loading = false
}

// SetLoading sets the loading state.
func (d *SecurityDialog) SetLoading(loading bool) {
	d.loading = loading
}

// SetSpinner sets the spinner view function.
func (d *SecurityDialog) SetSpinner(view func() string) {
	d.spinnerView = view
}

// Update handles messages for the security dialog.
func (d *SecurityDialog) Update(msg tea.Msg) (Dialog, tea.Cmd) {
	if !d.IsOpen() {
		return d, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		return d.handleKey(msg)
	}

	return d, nil
}

func (d *SecurityDialog) handleKey(msg tea.KeyMsg) (Dialog, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		d.Close()
		d.results = nil
		d.lastErr = nil
		return d, nil
	case "up", "k":
		d.viewport.LineUp(1)
	case "down", "j":
		d.viewport.LineDown(1)
	case "pgup", "ctrl+u":
		d.viewport.HalfViewUp()
	case "pgdown", "ctrl+d":
		d.viewport.HalfViewDown()
	case "home", "g":
		d.viewport.GotoTop()
	case "end", "G":
		d.viewport.GotoBottom()
	case "r":
		// Re-scan
		if !d.loading {
			d.loading = true
			d.lastErr = nil
			return d, func() tea.Msg {
				return SecurityRescanMsg{}
			}
		}
	}
	return d, nil
}

func (d *SecurityDialog) updateViewportContent() {
	if d.results == nil {
		d.viewport.SetContent(d.MutedStyle.Render("No scan results"))
		return
	}

	var content strings.Builder

	// Show results grouped by file
	for _, result := range d.results.Results {
		if len(result.Misconfigurations) == 0 {
			continue
		}
		content.WriteString(d.SubtitleStyle.Render(result.Target) + "\n")
		for _, m := range result.Misconfigurations {
			severityStyle := d.getSeverityStyle(m.Severity)
			content.WriteString(fmt.Sprintf("  %s %s\n",
				severityStyle.Render("["+m.Severity+"]"),
				d.ValueStyle.Render(m.Title),
			))
			msg := m.Message
			if msg == "" {
				msg = m.Description
			}
			if len(msg) > 70 {
				msg = msg[:67] + "..."
			}
			content.WriteString(d.MutedStyle.Render("    "+m.ID+": "+msg) + "\n")
		}
		content.WriteString("\n")
	}

	checkIcon := d.CheckIcon
	if checkIcon == "" {
		checkIcon = "✓"
	}

	if content.Len() == 0 {
		content.WriteString(d.SuccessStyle.Render(checkIcon + " No security issues found!"))
	}

	d.viewport.SetContent(content.String())
}

func (d *SecurityDialog) getSeverityStyle(severity string) lipgloss.Style {
	switch severity {
	case "CRITICAL":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#ff0000")).Bold(true)
	case "HIGH":
		return d.ErrorStyle
	case "MEDIUM":
		return d.WarningStyle
	case "LOW":
		return d.MutedStyle
	default:
		return d.ValueStyle
	}
}

// View renders the security dialog.
func (d *SecurityDialog) View() string {
	if !d.IsOpen() {
		return ""
	}

	// Calculate dialog dimensions
	dialogWidth := 80
	dialogHeight := d.Height - 8
	if dialogWidth > d.Width-4 {
		dialogWidth = d.Width - 4
	}
	if dialogHeight < 15 {
		dialogHeight = 15
	}

	dialogStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(d.BorderColor).
		Padding(1, 2).
		Width(dialogWidth)

	var content strings.Builder

	// Icons with defaults
	lockIcon := d.LockIcon
	if lockIcon == "" {
		lockIcon = "🔒"
	}
	crossIcon := d.CrossIcon
	if crossIcon == "" {
		crossIcon = "✗"
	}

	// Title
	content.WriteString(d.TitleStyle.Render(lockIcon+" Security Scan Results") + "\n")
	if d.chartName != "" {
		content.WriteString(d.MutedStyle.Render("Chart: "+d.chartName+" v"+d.chartVersion) + "\n")
	}
	content.WriteString("\n")

	if d.loading {
		if d.spinnerView != nil {
			content.WriteString(d.spinnerView() + " Scanning with Trivy...\n")
		} else {
			content.WriteString("Scanning with Trivy...\n")
		}
	} else if d.lastErr != nil {
		content.WriteString(d.ErrorStyle.Render(crossIcon+" Error: "+d.lastErr.Error()) + "\n")
	} else if d.results != nil {
		// Severity summary bar
		summary := d.results
		critStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#ff0000")).Bold(true)
		highStyle := d.ErrorStyle
		medStyle := d.WarningStyle
		lowStyle := d.MutedStyle

		summaryLine := fmt.Sprintf("%s  %s  %s  %s  Total: %d",
			critStyle.Render(fmt.Sprintf("Critical: %d", summary.Critical)),
			highStyle.Render(fmt.Sprintf("High: %d", summary.High)),
			medStyle.Render(fmt.Sprintf("Medium: %d", summary.Medium)),
			lowStyle.Render(fmt.Sprintf("Low: %d", summary.Low)),
			summary.Total,
		)
		content.WriteString(summaryLine + "\n")
		content.WriteString(strings.Repeat("─", dialogWidth-6) + "\n")

		// Update viewport dimensions
		viewportHeight := dialogHeight - 12
		if viewportHeight < 5 {
			viewportHeight = 5
		}
		d.viewport.Width = dialogWidth - 6
		d.viewport.Height = viewportHeight

		content.WriteString(d.viewport.View() + "\n")
	}

	// Footer hints
	content.WriteString("\n" + d.MutedStyle.Render("j/k:scroll  g/G:top/bottom  r:rescan  q/Esc:close"))

	dialog := dialogStyle.Render(content.String())
	return lipgloss.Place(d.Width, d.Height, lipgloss.Center, lipgloss.Center, dialog)
}

// SetStyles configures the dialog styles from the parent view.
func (d *SecurityDialog) SetStyles(title, subtitle, muted, value, errStyle, success, warning lipgloss.Style, borderColor lipgloss.Color, lockIcon, checkIcon, crossIcon string) {
	d.TitleStyle = title
	d.SubtitleStyle = subtitle
	d.MutedStyle = muted
	d.ValueStyle = value
	d.ErrorStyle = errStyle
	d.SuccessStyle = success
	d.WarningStyle = warning
	d.BorderColor = borderColor
	d.LockIcon = lockIcon
	d.CheckIcon = checkIcon
	d.CrossIcon = crossIcon
}

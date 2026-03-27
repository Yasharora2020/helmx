package dialog

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// VersionSelectedMsg is sent when a version is selected.
type VersionSelectedMsg struct {
	Version  string
	ChartRef string
}

// LocalVersion represents a chart version from local repos.
type LocalVersion struct {
	Version    string
	AppVersion string
}

// HubVersion represents a chart version from Artifact Hub.
type HubVersion struct {
	Version    string
	PreRelease bool
}

// VersionDialog allows selecting a specific chart version.
type VersionDialog struct {
	BaseDialog

	// Chart info
	chartName  string
	chartRef   string
	isHubChart bool

	// Version data
	localVersions []LocalVersion
	hubVersions   []HubVersion
	selectedIdx   int
	loading       bool
	lastError     error

	// Styles (injected from parent)
	TitleStyle   lipgloss.Style
	LabelStyle   lipgloss.Style
	MutedStyle   lipgloss.Style
	ValueStyle   lipgloss.Style
	ErrorStyle   lipgloss.Style
	WarningStyle lipgloss.Style
	BorderColor  lipgloss.Color
	ChartIcon    string
	ArrowIcon    string
	CrossIcon    string
}

// NewVersionDialog creates a new version selector dialog.
func NewVersionDialog() *VersionDialog {
	return &VersionDialog{}
}

// Open opens the dialog and resets state.
func (d *VersionDialog) Open() {
	d.BaseDialog.Open()
	d.selectedIdx = 0
	d.loading = true
	d.lastError = nil
	d.localVersions = nil
	d.hubVersions = nil
}

// OpenForChart opens the dialog for a specific chart.
func (d *VersionDialog) OpenForChart(chartName, chartRef string, isHub bool) {
	d.chartName = chartName
	d.chartRef = chartRef
	d.isHubChart = isHub
	d.BaseDialog.Open()
	d.selectedIdx = 0
	d.loading = true
	d.lastError = nil
	d.localVersions = nil
	d.hubVersions = nil
}

// SetLocalVersions sets the available local versions.
func (d *VersionDialog) SetLocalVersions(versions []LocalVersion) {
	d.localVersions = versions
	d.loading = false
	d.lastError = nil
}

// SetHubVersions sets the available Artifact Hub versions.
func (d *VersionDialog) SetHubVersions(versions []HubVersion) {
	d.hubVersions = versions
	d.loading = false
	d.lastError = nil
}

// SetError sets a loading error.
func (d *VersionDialog) SetError(err error) {
	d.lastError = err
	d.loading = false
}

// SetLoading sets the loading state.
func (d *VersionDialog) SetLoading(loading bool) {
	d.loading = loading
}

// GetChartRef returns the chart reference.
func (d *VersionDialog) GetChartRef() string {
	return d.chartRef
}

// IsHubChart returns true if this is an Artifact Hub chart.
func (d *VersionDialog) IsHubChart() bool {
	return d.isHubChart
}

// getVersionCount returns the total number of versions.
func (d *VersionDialog) getVersionCount() int {
	if d.isHubChart {
		return len(d.hubVersions)
	}
	return len(d.localVersions)
}

// getSelectedVersion returns the currently selected version string.
func (d *VersionDialog) getSelectedVersion() string {
	if d.isHubChart {
		if d.selectedIdx < len(d.hubVersions) {
			return d.hubVersions[d.selectedIdx].Version
		}
	} else {
		if d.selectedIdx < len(d.localVersions) {
			return d.localVersions[d.selectedIdx].Version
		}
	}
	return ""
}

// Update handles messages for the version dialog.
func (d *VersionDialog) Update(msg tea.Msg) (Dialog, tea.Cmd) {
	if !d.IsOpen() {
		return d, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		return d.handleKey(msg)
	}

	return d, nil
}

func (d *VersionDialog) handleKey(msg tea.KeyMsg) (Dialog, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		d.Close()
		return d, nil
	case "up", "k":
		if d.selectedIdx > 0 {
			d.selectedIdx--
		}
	case "down", "j":
		maxIdx := d.getVersionCount() - 1
		if d.selectedIdx < maxIdx {
			d.selectedIdx++
		}
	case "enter":
		// Select version
		version := d.getSelectedVersion()
		if version != "" {
			d.Close()
			return d, func() tea.Msg {
				return VersionSelectedMsg{
					Version:  version,
					ChartRef: d.chartRef,
				}
			}
		}
	}
	return d, nil
}

// View renders the version dialog.
func (d *VersionDialog) View() string {
	if !d.IsOpen() {
		return ""
	}

	width := 60
	height := 20

	var content strings.Builder

	// Icons with defaults
	chartIcon := d.ChartIcon
	if chartIcon == "" {
		chartIcon = "📦"
	}
	arrowIcon := d.ArrowIcon
	if arrowIcon == "" {
		arrowIcon = "→"
	}
	crossIcon := d.CrossIcon
	if crossIcon == "" {
		crossIcon = "✗"
	}

	// Title
	title := " " + chartIcon + " Select Version "
	content.WriteString(d.TitleStyle.Render(title) + "\n")
	content.WriteString(d.MutedStyle.Render("Chart: ") + d.ValueStyle.Render(d.chartName) + "\n\n")

	if d.loading {
		content.WriteString(d.MutedStyle.Render("Loading versions...") + "\n")
	} else if d.lastError != nil {
		content.WriteString(d.ErrorStyle.Render(crossIcon+" "+d.lastError.Error()) + "\n")
	} else if d.getVersionCount() == 0 {
		content.WriteString(d.MutedStyle.Render("No versions found.") + "\n")
	} else {
		// Show versions
		content.WriteString(d.MutedStyle.Render("Available versions:") + "\n\n")

		// Limit display to avoid overflow
		maxDisplay := 10
		startIdx := 0
		if d.selectedIdx >= maxDisplay {
			startIdx = d.selectedIdx - maxDisplay + 1
		}

		if d.isHubChart {
			for i := startIdx; i < len(d.hubVersions) && i < startIdx+maxDisplay; i++ {
				v := d.hubVersions[i]
				prefix := "  "
				if i == d.selectedIdx {
					prefix = arrowIcon + " "
				}
				line := prefix + v.Version
				if i == 0 {
					line += d.MutedStyle.Render(" (latest)")
				}
				if v.PreRelease {
					line += d.WarningStyle.Render(" [pre-release]")
				}
				content.WriteString(line + "\n")
			}
		} else {
			for i := startIdx; i < len(d.localVersions) && i < startIdx+maxDisplay; i++ {
				v := d.localVersions[i]
				prefix := "  "
				if i == d.selectedIdx {
					prefix = arrowIcon + " "
				}
				line := prefix + v.Version
				if i == 0 {
					line += d.MutedStyle.Render(" (latest)")
				}
				if v.AppVersion != "" {
					line += d.MutedStyle.Render(" (app: " + v.AppVersion + ")")
				}
				content.WriteString(line + "\n")
			}
		}

		// Scroll indicator
		if d.getVersionCount() > maxDisplay {
			content.WriteString("\n" + d.MutedStyle.Render(fmt.Sprintf("Showing %d-%d of %d",
				startIdx+1, min(startIdx+maxDisplay, d.getVersionCount()), d.getVersionCount())) + "\n")
		}
	}

	content.WriteString("\n")
	content.WriteString(d.MutedStyle.Render("j/k:navigate  Enter:select  Esc:cancel"))

	// Build dialog box
	dialogStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(d.BorderColor).
		Padding(1, 2).
		Width(width).
		Height(height)

	dialog := dialogStyle.Render(content.String())

	// Center the dialog
	return lipgloss.Place(d.Width, d.Height, lipgloss.Center, lipgloss.Center, dialog)
}

// SetStyles configures the dialog styles from the parent view.
func (d *VersionDialog) SetStyles(title, label, muted, value, errStyle, warning lipgloss.Style, borderColor lipgloss.Color, chartIcon, arrowIcon, crossIcon string) {
	d.TitleStyle = title
	d.LabelStyle = label
	d.MutedStyle = muted
	d.ValueStyle = value
	d.ErrorStyle = errStyle
	d.WarningStyle = warning
	d.BorderColor = borderColor
	d.ChartIcon = chartIcon
	d.ArrowIcon = arrowIcon
	d.CrossIcon = crossIcon
}

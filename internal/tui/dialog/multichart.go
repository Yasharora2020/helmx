package dialog

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// MultiChartStatus represents the status of a chart in the queue.
type MultiChartStatus int

const (
	MultiChartPending MultiChartStatus = iota
	MultiChartInstalling
	MultiChartReady
	MultiChartFailed
)

// MultiChartQueueItem represents a chart in the installation queue.
type MultiChartQueueItem struct {
	ChartRef        string           // Chart reference (e.g., "bitnami/postgresql")
	ChartName       string           // Display name
	ChartVersion    string           // Chart version
	ReleaseName     string           // Release name
	Namespace       string           // Target namespace
	Values          string           // YAML values content
	CreateNamespace bool             // Create namespace if not exists
	WaitForReady    bool             // Wait for pods to be ready before next
	Status          MultiChartStatus // Installation status
	Error           error            // Error if installation failed
}

// MultiChartField represents the focused field in the dialog.
type MultiChartField int

const (
	MultiChartFieldQueue MultiChartField = iota
	MultiChartFieldInstall
	MultiChartFieldSave
	MultiChartFieldLoad
	MultiChartFieldCancel
)

// Message types for MultiChartDialog

// MultiChartEditValuesMsg is sent when the user wants to edit chart values.
type MultiChartEditValuesMsg struct {
	Index int
}

// MultiChartInstallMsg is sent when installation should start.
type MultiChartInstallMsg struct{}

// MultiChartSaveStackMsg is sent when the user wants to save the queue as a stack.
type MultiChartSaveStackMsg struct{}

// MultiChartLoadStackMsg is sent when the user wants to load a stack.
type MultiChartLoadStackMsg struct{}

// MultiChartDialog displays the multi-chart installation dialog.
type MultiChartDialog struct {
	BaseDialog
	viewport viewport.Model

	// Queue state
	queue       []MultiChartQueueItem
	selectedIdx int
	field       MultiChartField

	// Installation state
	installing bool
	currentIdx int
	lastError  error

	// Spinner view (passed from parent)
	spinnerView func() string

	// Styles (injected from parent)
	TitleStyle    lipgloss.Style
	LabelStyle    lipgloss.Style
	MutedStyle    lipgloss.Style
	ValueStyle    lipgloss.Style
	ErrorStyle    lipgloss.Style
	SuccessStyle  lipgloss.Style
	WarningStyle  lipgloss.Style
	SelectedStyle lipgloss.Style
	ButtonStyle   lipgloss.Style
	ButtonFocus   lipgloss.Style
	BorderColor   lipgloss.Color
	ChartIcon     string
	CheckIcon     string
	CrossIcon     string
	ArrowIcon     string
}

// NewMultiChartDialog creates a new multi-chart install dialog.
func NewMultiChartDialog() *MultiChartDialog {
	vp := viewport.New(70, 15)
	return &MultiChartDialog{
		viewport: vp,
		queue:    make([]MultiChartQueueItem, 0),
	}
}

// Open opens the dialog.
func (d *MultiChartDialog) Open() {
	d.BaseDialog.Open()
	d.field = MultiChartFieldQueue
	d.selectedIdx = 0
	d.installing = false
	d.lastError = nil
}

// Close closes the dialog.
func (d *MultiChartDialog) Close() {
	d.BaseDialog.Close()
}

// GetQueue returns the current queue.
func (d *MultiChartDialog) GetQueue() []MultiChartQueueItem {
	return d.queue
}

// SetQueue sets the queue.
func (d *MultiChartDialog) SetQueue(queue []MultiChartQueueItem) {
	d.queue = queue
	if d.selectedIdx >= len(queue) && d.selectedIdx > 0 {
		d.selectedIdx = len(queue) - 1
	}
}

// AddToQueue adds a chart to the queue.
func (d *MultiChartDialog) AddToQueue(item MultiChartQueueItem) {
	// Check if chart is already in queue
	for _, existing := range d.queue {
		if existing.ChartRef == item.ChartRef {
			return // Already in queue
		}
	}
	d.queue = append(d.queue, item)
}

// GetSelectedIndex returns the currently selected index.
func (d *MultiChartDialog) GetSelectedIndex() int {
	return d.selectedIdx
}

// IsInstalling returns whether installation is in progress.
func (d *MultiChartDialog) IsInstalling() bool {
	return d.installing
}

// SetInstalling sets the installing state.
func (d *MultiChartDialog) SetInstalling(installing bool) {
	d.installing = installing
}

// GetCurrentIdx returns the index of the chart being installed.
func (d *MultiChartDialog) GetCurrentIdx() int {
	return d.currentIdx
}

// SetCurrentIdx sets the index of the chart being installed.
func (d *MultiChartDialog) SetCurrentIdx(idx int) {
	d.currentIdx = idx
}

// SetError sets the last error.
func (d *MultiChartDialog) SetError(err error) {
	d.lastError = err
}

// UpdateItemStatus updates the status of a queue item.
func (d *MultiChartDialog) UpdateItemStatus(idx int, status MultiChartStatus, err error) {
	if idx >= 0 && idx < len(d.queue) {
		d.queue[idx].Status = status
		d.queue[idx].Error = err
	}
}

// UpdateItemValues updates the values of a queue item.
func (d *MultiChartDialog) UpdateItemValues(idx int, values string) {
	if idx >= 0 && idx < len(d.queue) {
		d.queue[idx].Values = values
	}
}

// ResetAllStatuses resets all queue items to pending.
func (d *MultiChartDialog) ResetAllStatuses() {
	for i := range d.queue {
		d.queue[i].Status = MultiChartPending
		d.queue[i].Error = nil
	}
}

// ClearQueue clears the queue.
func (d *MultiChartDialog) ClearQueue() {
	d.queue = make([]MultiChartQueueItem, 0)
	d.selectedIdx = 0
}

// SetSpinner sets the spinner view function.
func (d *MultiChartDialog) SetSpinner(view func() string) {
	d.spinnerView = view
}

// Update handles messages for the multi-chart dialog.
func (d *MultiChartDialog) Update(msg tea.Msg) (Dialog, tea.Cmd) {
	if !d.IsOpen() {
		return d, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		return d.handleKey(msg)
	}

	return d, nil
}

func (d *MultiChartDialog) handleKey(msg tea.KeyMsg) (Dialog, tea.Cmd) {
	// Don't allow navigation during installation
	if d.installing {
		if msg.String() == "esc" {
			// Cancel installation
			d.installing = false
			// Reset installing items to pending
			for i := range d.queue {
				if d.queue[i].Status == MultiChartInstalling {
					d.queue[i].Status = MultiChartPending
				}
			}
		}
		return d, nil
	}

	switch msg.String() {
	case "esc", "q":
		d.Close()
		return d, nil

	case "j", "down":
		if d.field == MultiChartFieldQueue && len(d.queue) > 0 {
			if d.selectedIdx < len(d.queue)-1 {
				d.selectedIdx++
			}
		}

	case "k", "up":
		if d.field == MultiChartFieldQueue && d.selectedIdx > 0 {
			d.selectedIdx--
		}

	case "J": // Move item down in queue
		if d.field == MultiChartFieldQueue && len(d.queue) > 1 {
			if d.selectedIdx < len(d.queue)-1 {
				d.queue[d.selectedIdx], d.queue[d.selectedIdx+1] =
					d.queue[d.selectedIdx+1], d.queue[d.selectedIdx]
				d.selectedIdx++
			}
		}

	case "K": // Move item up in queue
		if d.field == MultiChartFieldQueue && len(d.queue) > 1 {
			if d.selectedIdx > 0 {
				d.queue[d.selectedIdx], d.queue[d.selectedIdx-1] =
					d.queue[d.selectedIdx-1], d.queue[d.selectedIdx]
				d.selectedIdx--
			}
		}

	case "d", "delete": // Remove from queue
		if d.field == MultiChartFieldQueue && len(d.queue) > 0 {
			d.queue = append(d.queue[:d.selectedIdx], d.queue[d.selectedIdx+1:]...)
			if d.selectedIdx >= len(d.queue) && d.selectedIdx > 0 {
				d.selectedIdx--
			}
		}

	case "e": // Edit selected item values
		if d.field == MultiChartFieldQueue && len(d.queue) > 0 {
			return d, func() tea.Msg {
				return MultiChartEditValuesMsg{Index: d.selectedIdx}
			}
		}

	case "w": // Toggle wait for ready
		if d.field == MultiChartFieldQueue && len(d.queue) > 0 {
			d.queue[d.selectedIdx].WaitForReady = !d.queue[d.selectedIdx].WaitForReady
		}

	case "tab":
		d.field = MultiChartField((int(d.field) + 1) % 5)

	case "shift+tab":
		d.field = MultiChartField((int(d.field) + 4) % 5)

	case "enter":
		switch d.field {
		case MultiChartFieldInstall:
			if len(d.queue) > 0 {
				return d, func() tea.Msg {
					return MultiChartInstallMsg{}
				}
			}
		case MultiChartFieldSave:
			if len(d.queue) > 0 {
				return d, func() tea.Msg {
					return MultiChartSaveStackMsg{}
				}
			}
		case MultiChartFieldLoad:
			return d, func() tea.Msg {
				return MultiChartLoadStackMsg{}
			}
		case MultiChartFieldCancel:
			d.Close()
			return d, nil
		}

	case "c": // Clear queue
		d.ClearQueue()
	}

	return d, nil
}

// View renders the multi-chart dialog.
func (d *MultiChartDialog) View() string {
	if !d.IsOpen() {
		return ""
	}

	dialogWidth := 75
	if d.Width < 80 {
		dialogWidth = d.Width - 6
	}

	dialogStyle := lipgloss.NewStyle().
		Border(lipgloss.DoubleBorder()).
		BorderForeground(d.BorderColor).
		Padding(1, 2).
		Width(dialogWidth)

	var content strings.Builder

	// Title
	chartIcon := d.ChartIcon
	if chartIcon == "" {
		chartIcon = "📦"
	}
	content.WriteString(d.TitleStyle.Render(chartIcon+" Multi-Chart Install") + "\n\n")

	// Queue section
	if len(d.queue) == 0 {
		content.WriteString(d.MutedStyle.Render("No charts in queue.") + "\n")
		content.WriteString(d.MutedStyle.Render("Press 'a' on a loaded chart to add it to the queue.") + "\n\n")
	} else {
		content.WriteString(d.LabelStyle.Render(fmt.Sprintf("Installation Queue (%d charts):", len(d.queue))) + "\n\n")

		// Column headers
		header := fmt.Sprintf("  %-3s %-25s %-15s %-8s %s", "#", "CHART", "RELEASE", "WAIT", "STATUS")
		content.WriteString(d.MutedStyle.Render(header) + "\n")

		for i, item := range d.queue {
			// Status icon and text
			var statusIcon, statusText string
			switch item.Status {
			case MultiChartPending:
				statusIcon = "○"
				statusText = d.MutedStyle.Render("Pending")
			case MultiChartInstalling:
				statusIcon = "◐"
				statusText = d.WarningStyle.Render("Installing...")
			case MultiChartReady:
				checkIcon := d.CheckIcon
				if checkIcon == "" {
					checkIcon = "✓"
				}
				statusIcon = checkIcon
				statusText = d.SuccessStyle.Render("Ready")
			case MultiChartFailed:
				crossIcon := d.CrossIcon
				if crossIcon == "" {
					crossIcon = "✗"
				}
				statusIcon = crossIcon
				errMsg := "Failed"
				if item.Error != nil {
					errMsg = item.Error.Error()
					if len(errMsg) > 20 {
						errMsg = errMsg[:20] + "..."
					}
				}
				statusText = d.ErrorStyle.Render(errMsg)
			}

			// Wait indicator
			waitStr := "Yes"
			if !item.WaitForReady {
				waitStr = "No"
			}

			// Truncate names if needed
			chartName := item.ChartName
			if len(chartName) > 25 {
				chartName = chartName[:22] + "..."
			}
			releaseName := item.ReleaseName
			if len(releaseName) > 15 {
				releaseName = releaseName[:12] + "..."
			}

			line := fmt.Sprintf("%s %-3d %-25s %-15s %-8s %s",
				statusIcon, i+1, chartName, releaseName, waitStr, statusText)

			arrowIcon := d.ArrowIcon
			if arrowIcon == "" {
				arrowIcon = ">"
			}
			if i == d.selectedIdx && d.field == MultiChartFieldQueue {
				content.WriteString(d.SelectedStyle.Render(arrowIcon+" "+line) + "\n")
			} else {
				content.WriteString("  " + line + "\n")
			}
		}
		content.WriteString("\n")
	}

	// Error display
	if d.lastError != nil {
		crossIcon := d.CrossIcon
		if crossIcon == "" {
			crossIcon = "✗"
		}
		content.WriteString(d.ErrorStyle.Render(crossIcon+" Error: "+d.lastError.Error()) + "\n\n")
	}

	// Buttons
	if !d.installing {
		installBtn := d.ButtonStyle.Render(" Install All ")
		saveBtn := d.ButtonStyle.Render(" Save Stack ")
		loadBtn := d.ButtonStyle.Render(" Load Stack ")
		cancelBtn := d.ButtonStyle.Render(" Cancel ")

		if d.field == MultiChartFieldInstall {
			installBtn = d.ButtonFocus.Render(" Install All ")
		}
		if d.field == MultiChartFieldSave {
			saveBtn = d.ButtonFocus.Render(" Save Stack ")
		}
		if d.field == MultiChartFieldLoad {
			loadBtn = d.ButtonFocus.Render(" Load Stack ")
		}
		if d.field == MultiChartFieldCancel {
			cancelBtn = d.ButtonFocus.Render(" Cancel ")
		}

		content.WriteString(installBtn + " " + saveBtn + " " + loadBtn + " " + cancelBtn + "\n")
	} else {
		if d.spinnerView != nil {
			content.WriteString(d.spinnerView() + " Installing...\n")
		} else {
			content.WriteString("Installing...\n")
		}
	}

	// Key hints
	content.WriteString("\n" + d.MutedStyle.Render("j/k:navigate  J/K:reorder  d:remove  e:edit  w:toggle wait  c:clear  Tab:buttons"))

	dialog := dialogStyle.Render(content.String())
	return lipgloss.Place(d.Width, d.Height, lipgloss.Center, lipgloss.Center, dialog)
}

// SetStyles configures the dialog styles from the parent view.
func (d *MultiChartDialog) SetStyles(
	title, label, muted, value, errStyle, success, warning, selected, button, buttonFocus lipgloss.Style,
	borderColor lipgloss.Color,
	chartIcon, checkIcon, crossIcon, arrowIcon string,
) {
	d.TitleStyle = title
	d.LabelStyle = label
	d.MutedStyle = muted
	d.ValueStyle = value
	d.ErrorStyle = errStyle
	d.SuccessStyle = success
	d.WarningStyle = warning
	d.SelectedStyle = selected
	d.ButtonStyle = button
	d.ButtonFocus = buttonFocus
	d.BorderColor = borderColor
	d.ChartIcon = chartIcon
	d.CheckIcon = checkIcon
	d.CrossIcon = crossIcon
	d.ArrowIcon = arrowIcon
}

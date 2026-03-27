package dialog

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ProgressFetchMsg is sent when the dialog needs fresh pod/resource data.
type ProgressFetchMsg struct{}

// ProgressTickMsg triggers the next auto-refresh cycle.
type ProgressTickMsg struct{}

// PodStatus holds the status of a single pod.
type PodStatus struct {
	Name            string
	Status          string
	Ready           string
	Restarts        int
	ContainerStatus []ContainerStatus
}

// ContainerStatus holds the status of a container within a pod.
type ContainerStatus struct {
	Name   string
	State  string
	Reason string
}

// ResourceStatus holds the status of a Kubernetes resource.
type ResourceStatus struct {
	Kind   string
	Name   string
	Ready  bool
	Status string
}

// EventInfo holds Kubernetes event information.
type EventInfo struct {
	Type      string
	Reason    string
	Object    string
	Message   string
	Timestamp time.Time
}

// ReleaseInfo holds basic release information for the progress dialog.
type ReleaseInfo struct {
	Name      string
	Namespace string
}

// ProgressDialog displays installation progress with pod/resource monitoring.
type ProgressDialog struct {
	BaseDialog
	viewport viewport.Model

	// Release being monitored
	release ReleaseInfo

	// Data from parent
	pods      []PodStatus
	resources []ResourceStatus
	events    []EventInfo
	allReady  bool
	lastError error

	// UI state
	activeTab int // 0=Pods, 1=Resources, 2=Events

	// Styles (injected from parent)
	TitleStyle       lipgloss.Style
	LabelStyle       lipgloss.Style
	MutedStyle       lipgloss.Style
	ValueStyle       lipgloss.Style
	ErrorStyle       lipgloss.Style
	SuccessStyle     lipgloss.Style
	WarningStyle     lipgloss.Style
	ButtonStyle      lipgloss.Style
	ButtonFocusStyle lipgloss.Style
	BorderColor      lipgloss.Color
	CheckIcon        string
	CrossIcon        string
	PendingIcon      string
	ArrowIcon        string
}

// NewProgressDialog creates a new installation progress dialog.
func NewProgressDialog() *ProgressDialog {
	vp := viewport.New(80, 20)
	return &ProgressDialog{
		viewport: vp,
	}
}

// Open opens the dialog and resets state.
func (d *ProgressDialog) Open() {
	d.BaseDialog.Open()
	d.activeTab = 0
	d.pods = nil
	d.resources = nil
	d.events = nil
	d.allReady = false
	d.lastError = nil
	d.viewport.GotoTop()
}

// OpenWithRelease opens the dialog for a specific release.
func (d *ProgressDialog) OpenWithRelease(name, namespace string) tea.Cmd {
	d.release = ReleaseInfo{
		Name:      name,
		Namespace: namespace,
	}
	d.BaseDialog.Open()
	d.activeTab = 0
	d.pods = nil
	d.resources = nil
	d.events = nil
	d.allReady = false
	d.lastError = nil
	d.viewport.GotoTop()

	// Return commands for initial fetch and start tick
	return tea.Batch(
		func() tea.Msg { return ProgressFetchMsg{} },
		d.scheduleNextTick(),
	)
}

// scheduleNextTick schedules the next auto-refresh.
func (d *ProgressDialog) scheduleNextTick() tea.Cmd {
	return tea.Tick(2500*time.Millisecond, func(t time.Time) tea.Msg {
		return ProgressTickMsg{}
	})
}

// GetRelease returns the release info being monitored.
func (d *ProgressDialog) GetRelease() ReleaseInfo {
	return d.release
}

// SetData updates the dialog with fresh data from the parent.
func (d *ProgressDialog) SetData(pods []PodStatus, resources []ResourceStatus, events []EventInfo, allReady bool, err error) {
	d.pods = pods
	d.resources = resources
	d.events = events
	d.allReady = allReady
	d.lastError = err
	d.updateViewportContent()
}

// IsAllReady returns true if all pods are ready.
func (d *ProgressDialog) IsAllReady() bool {
	return d.allReady
}

// Update handles messages for the progress dialog.
func (d *ProgressDialog) Update(msg tea.Msg) (Dialog, tea.Cmd) {
	if !d.IsOpen() {
		return d, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		return d.handleKey(msg)
	case ProgressTickMsg:
		// Request fresh data and schedule next tick
		if !d.allReady {
			return d, tea.Batch(
				func() tea.Msg { return ProgressFetchMsg{} },
				d.scheduleNextTick(),
			)
		}
	}

	return d, nil
}

func (d *ProgressDialog) handleKey(msg tea.KeyMsg) (Dialog, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		d.Close()
		return d, nil
	case "tab":
		d.activeTab = (d.activeTab + 1) % 3
		d.updateViewportContent()
	case "1":
		d.activeTab = 0
		d.updateViewportContent()
	case "2":
		d.activeTab = 1
		d.updateViewportContent()
	case "3":
		d.activeTab = 2
		d.updateViewportContent()
	case "r":
		// Manual refresh
		return d, func() tea.Msg { return ProgressFetchMsg{} }
	case "j", "down":
		d.viewport.LineDown(1)
	case "k", "up":
		d.viewport.LineUp(1)
	case "g":
		d.viewport.GotoTop()
	case "G":
		d.viewport.GotoBottom()
	}

	var cmd tea.Cmd
	d.viewport, cmd = d.viewport.Update(msg)
	return d, cmd
}

func (d *ProgressDialog) updateViewportContent() {
	var content string

	switch d.activeTab {
	case 0:
		content = d.formatPods()
	case 1:
		content = d.formatResources()
	case 2:
		content = d.formatEvents()
	}

	d.viewport.SetContent(content)
}

func (d *ProgressDialog) formatPods() string {
	if len(d.pods) == 0 {
		return d.MutedStyle.Render("No pods found for this release.\nThis may be normal for Jobs or if pods haven't been created yet.")
	}

	var lines []string
	for _, pod := range d.pods {
		statusStyle := d.getPodStatusStyle(pod.Status)
		restartInfo := ""
		if pod.Restarts > 0 {
			restartInfo = d.WarningStyle.Render(fmt.Sprintf(" (%d restarts)", pod.Restarts))
		}

		line := fmt.Sprintf("%s  %s  %s%s",
			statusStyle.Render(fmt.Sprintf("%-18s", pod.Status)),
			d.MutedStyle.Render(pod.Ready),
			d.ValueStyle.Render(pod.Name),
			restartInfo,
		)
		lines = append(lines, line)

		// Show container status if there are issues
		arrow := d.ArrowIcon
		if arrow == "" {
			arrow = "→"
		}
		for _, cs := range pod.ContainerStatus {
			if cs.State == "Waiting" && cs.Reason != "" {
				lines = append(lines, d.MutedStyle.Render("  "+arrow+" "+cs.Name+": "+cs.Reason))
			}
		}
	}

	return strings.Join(lines, "\n")
}

func (d *ProgressDialog) formatResources() string {
	if len(d.resources) == 0 {
		return d.MutedStyle.Render("No resources found for this release.\nResources will appear here once the chart is processed.")
	}

	var lines []string
	for _, res := range d.resources {
		// Icon based on ready status
		check := d.CheckIcon
		if check == "" {
			check = "✓"
		}
		pending := d.PendingIcon
		if pending == "" {
			pending = "○"
		}

		icon := check
		style := d.SuccessStyle
		if !res.Ready {
			icon = pending
			style = d.WarningStyle
		}

		// Format: [Icon] Kind/Name (status)
		line := fmt.Sprintf("%s %s/%s",
			style.Render(icon),
			d.LabelStyle.Render(res.Kind),
			d.ValueStyle.Render(res.Name),
		)

		// Add status text for resources that have it
		if res.Status != "" && res.Status != "Exists" {
			line += d.MutedStyle.Render("  (" + res.Status + ")")
		}

		lines = append(lines, line)
	}

	return strings.Join(lines, "\n")
}

func (d *ProgressDialog) formatEvents() string {
	if len(d.events) == 0 {
		return d.MutedStyle.Render("No events found yet.\nEvents typically appear when pods start initializing.")
	}

	var lines []string
	for _, event := range d.events {
		// Style based on event type
		typeStyle := d.SuccessStyle
		check := d.CheckIcon
		if check == "" {
			check = "✓"
		}
		cross := d.CrossIcon
		if cross == "" {
			cross = "✗"
		}
		icon := check
		if event.Type == "Warning" {
			typeStyle = d.WarningStyle
			icon = cross
		}

		// Format timestamp
		age := time.Since(event.Timestamp)
		ageStr := formatAge(age)

		// Format: [Icon] Reason (age)
		header := fmt.Sprintf("%s %s %s",
			typeStyle.Render(icon),
			d.LabelStyle.Render(event.Reason),
			d.MutedStyle.Render("("+ageStr+")"),
		)

		// Object and message on next line
		msg := event.Message
		if len(msg) > 60 {
			msg = msg[:57] + "..."
		}
		detail := fmt.Sprintf("  %s: %s",
			d.ValueStyle.Render(event.Object),
			d.MutedStyle.Render(msg),
		)

		lines = append(lines, header)
		lines = append(lines, detail)
		lines = append(lines, "") // Empty line between events
	}

	return strings.Join(lines, "\n")
}

func (d *ProgressDialog) getPodStatusStyle(status string) lipgloss.Style {
	switch status {
	case "Running":
		return d.SuccessStyle
	case "Succeeded", "Completed":
		return d.SuccessStyle
	case "Pending", "ContainerCreating":
		return d.WarningStyle
	case "CrashLoopBackOff", "ImagePullBackOff", "Error", "Failed":
		return d.ErrorStyle
	default:
		return d.MutedStyle
	}
}

// formatAge formats a duration nicely.
func formatAge(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	return fmt.Sprintf("%dh", int(d.Hours()))
}

// View renders the progress dialog.
func (d *ProgressDialog) View() string {
	if !d.IsOpen() {
		return ""
	}

	dialogWidth := d.Width - 10
	if dialogWidth > 100 {
		dialogWidth = 100
	}
	if dialogWidth < 50 {
		dialogWidth = 50
	}
	dialogHeight := d.Height - 8
	if dialogHeight < 15 {
		dialogHeight = 15
	}

	dialogStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(d.BorderColor).
		Padding(1, 2).
		Width(dialogWidth)

	// Icons with defaults
	checkIcon := d.CheckIcon
	if checkIcon == "" {
		checkIcon = "✓"
	}
	crossIcon := d.CrossIcon
	if crossIcon == "" {
		crossIcon = "✗"
	}
	pendingIcon := d.PendingIcon
	if pendingIcon == "" {
		pendingIcon = "○"
	}

	// Status indicator
	statusIcon := pendingIcon
	statusStyle := d.WarningStyle
	statusText := "Waiting for pods..."
	if d.allReady {
		statusIcon = checkIcon
		statusStyle = d.SuccessStyle
		statusText = "All pods ready!"
	} else if d.lastError != nil {
		statusIcon = crossIcon
		statusStyle = d.ErrorStyle
		statusText = "Error"
	}

	title := statusStyle.Bold(true).Render(statusIcon + " Installation Progress: " + d.release.Name)

	// Tab bar
	tabs := []string{"Pods", "Resources", "Events"}
	var tabParts []string
	for i, tab := range tabs {
		var count int
		switch i {
		case 0:
			count = len(d.pods)
		case 1:
			count = len(d.resources)
		case 2:
			count = len(d.events)
		}
		label := fmt.Sprintf("%s (%d)", tab, count)

		if i == d.activeTab {
			tabParts = append(tabParts, d.ButtonFocusStyle.Render(" "+label+" "))
		} else {
			tabParts = append(tabParts, d.ButtonStyle.Render(" "+label+" "))
		}
	}
	tabBar := strings.Join(tabParts, " ")

	// Update viewport size
	viewportHeight := dialogHeight - 12
	if viewportHeight < 5 {
		viewportHeight = 5
	}
	d.viewport.Width = dialogWidth - 6
	d.viewport.Height = viewportHeight

	// Make sure content is updated
	d.updateViewportContent()

	// Build content
	var content strings.Builder
	content.WriteString(title + "\n")
	content.WriteString(d.MutedStyle.Render("Namespace: "+d.release.Namespace) + "\n\n")
	content.WriteString(tabBar + "\n")
	content.WriteString(strings.Repeat("─", dialogWidth-6) + "\n")
	content.WriteString(d.viewport.View() + "\n")
	content.WriteString(strings.Repeat("─", dialogWidth-6) + "\n")

	// Status message
	if d.allReady {
		content.WriteString(d.SuccessStyle.Render(checkIcon+" "+statusText) + "\n")
	} else if d.lastError != nil {
		content.WriteString(d.ErrorStyle.Render(crossIcon+" "+d.lastError.Error()) + "\n")
	} else {
		content.WriteString(d.WarningStyle.Render(pendingIcon+" "+statusText+" (auto-refreshing)") + "\n")
	}

	// Footer
	content.WriteString("\n" + d.MutedStyle.Render("Tab/1/2/3:switch  r:refresh  j/k:scroll  Esc:close"))

	dialog := dialogStyle.Render(content.String())
	return lipgloss.Place(d.Width, d.Height, lipgloss.Center, lipgloss.Center, dialog)
}

// SetStyles configures the dialog styles from the parent view.
func (d *ProgressDialog) SetStyles(title, label, muted, value, errStyle, success, warning, button, buttonFocus lipgloss.Style, borderColor lipgloss.Color, checkIcon, crossIcon, pendingIcon, arrowIcon string) {
	d.TitleStyle = title
	d.LabelStyle = label
	d.MutedStyle = muted
	d.ValueStyle = value
	d.ErrorStyle = errStyle
	d.SuccessStyle = success
	d.WarningStyle = warning
	d.ButtonStyle = button
	d.ButtonFocusStyle = buttonFocus
	d.BorderColor = borderColor
	d.CheckIcon = checkIcon
	d.CrossIcon = crossIcon
	d.PendingIcon = pendingIcon
	d.ArrowIcon = arrowIcon
}

package tui

import (
	"fmt"
	"strings"

	"github.com/yasharora2020/helmx/internal/helm"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Log viewer message types
type podListLoadedMsg struct {
	pods []helm.PodInfo
	err  error
}

type podLogsLoadedMsg struct {
	logs string
	err  error
}

// fetchPodList fetches the list of pods for the release
func (r *ReleaseDetail) fetchPodList() tea.Cmd {
	releaseName := r.release.Name
	namespace := r.release.Namespace

	return func() tea.Msg {
		pods, err := r.clusterInspector.GetReleasePods(releaseName, namespace)
		return podListLoadedMsg{pods: pods, err: err}
	}
}

// fetchPodLogs fetches logs from the selected pod/container
func (r *ReleaseDetail) fetchPodLogs() tea.Cmd {
	pod := r.logSelectedPod
	if pod == nil {
		return func() tea.Msg {
			return podLogsLoadedMsg{err: fmt.Errorf("no pod selected")}
		}
	}

	containerName := ""
	if r.logContainerIdx < len(pod.Containers) {
		containerName = pod.Containers[r.logContainerIdx]
	}

	opts := helm.PodLogOptions{
		Container:  containerName,
		TailLines:  100, // Default: last 100 lines
		Timestamps: r.logShowTimestamps,
		Previous:   r.logShowPrevious,
	}

	podName := pod.Name
	namespace := pod.Namespace

	return func() tea.Msg {
		logs, err := r.clusterInspector.GetPodLogs(podName, namespace, opts)
		return podLogsLoadedMsg{logs: logs, err: err}
	}
}

// updateLogDialog routes key events for the log viewer dialog
func (r *ReleaseDetail) updateLogDialog(msg tea.KeyMsg) (*ReleaseDetail, tea.Cmd) {
	switch r.logDialogStep {
	case LogStepSelectPod:
		return r.updateLogPodSelection(msg)
	case LogStepViewLogs:
		return r.updateLogViewer(msg)
	}
	return r, nil
}

func (r *ReleaseDetail) updateLogPodSelection(msg tea.KeyMsg) (*ReleaseDetail, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		r.showLogDialog = false
		return r, nil
	case "up", "k":
		if r.logSelectedPodIdx > 0 {
			r.logSelectedPodIdx--
			r.logContainerIdx = 0 // Reset container selection
		}
	case "down", "j":
		if r.logSelectedPodIdx < len(r.logPodList)-1 {
			r.logSelectedPodIdx++
			r.logContainerIdx = 0
		}
	case "left", "h":
		// Cycle through containers (if multiple)
		if len(r.logPodList) > 0 && r.logContainerIdx > 0 {
			r.logContainerIdx--
		}
	case "right", "l":
		if len(r.logPodList) > 0 {
			pod := r.logPodList[r.logSelectedPodIdx]
			if r.logContainerIdx < len(pod.Containers)-1 {
				r.logContainerIdx++
			}
		}
	case "t":
		r.logShowTimestamps = !r.logShowTimestamps
	case "p":
		r.logShowPrevious = !r.logShowPrevious
	case "enter":
		// Select pod and fetch logs
		if len(r.logPodList) > 0 {
			r.logSelectedPod = &r.logPodList[r.logSelectedPodIdx]
			r.logDialogStep = LogStepViewLogs
			r.logLoading = true
			r.logError = nil
			return r, r.fetchPodLogs()
		}
	}
	return r, nil
}

func (r *ReleaseDetail) updateLogViewer(msg tea.KeyMsg) (*ReleaseDetail, tea.Cmd) {
	switch msg.String() {
	case "esc":
		// Go back to pod selection
		r.logDialogStep = LogStepSelectPod
		r.logContent = ""
		r.logError = nil
		return r, nil
	case "q":
		// Close entire dialog
		r.showLogDialog = false
		return r, nil
	case "r":
		// Refresh logs
		r.logLoading = true
		r.logError = nil
		return r, r.fetchPodLogs()
	case "j", "down":
		r.logViewport.LineDown(1)
	case "k", "up":
		r.logViewport.LineUp(1)
	case "ctrl+d":
		r.logViewport.HalfViewDown()
	case "ctrl+u":
		r.logViewport.HalfViewUp()
	case "g":
		r.logViewport.GotoTop()
	case "G":
		r.logViewport.GotoBottom()
	case "t":
		// Toggle timestamps and refresh
		r.logShowTimestamps = !r.logShowTimestamps
		r.logLoading = true
		return r, r.fetchPodLogs()
	case "p":
		// Toggle previous container logs
		r.logShowPrevious = !r.logShowPrevious
		r.logLoading = true
		return r, r.fetchPodLogs()
	}

	var cmd tea.Cmd
	r.logViewport, cmd = r.logViewport.Update(msg)
	return r, cmd
}

// renderLogDialog renders the log viewer dialog
func (r *ReleaseDetail) renderLogDialog() string {
	dialogWidth := r.width - 10
	if dialogWidth > 120 {
		dialogWidth = 120
	}
	if dialogWidth < 60 {
		dialogWidth = 60
	}

	dialogHeight := r.height - 8
	if dialogHeight < 20 {
		dialogHeight = 20
	}

	dialogStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(DefaultTheme.Primary).
		Padding(1, 2).
		Width(dialogWidth)

	var content strings.Builder

	if r.logDialogStep == LogStepSelectPod {
		content.WriteString(r.renderLogPodSelection(dialogWidth, dialogHeight))
	} else {
		content.WriteString(r.renderLogViewer(dialogWidth, dialogHeight))
	}

	dialog := dialogStyle.Render(content.String())
	return lipgloss.Place(r.width, r.height, lipgloss.Center, lipgloss.Center, dialog)
}

func (r *ReleaseDetail) renderLogPodSelection(width, height int) string {
	var content strings.Builder

	// Title
	content.WriteString(S.Title.Render(Icons.Deployed+" Select Pod for Logs") + "\n\n")

	// Error state
	if r.logError != nil {
		content.WriteString(S.Error.Render(Icons.Cross+" "+r.logError.Error()) + "\n\n")
	}

	// Loading state
	if r.logLoading {
		content.WriteString(S.Muted.Render(Icons.Helm+" Loading pods...") + "\n\n")
	} else if len(r.logPodList) == 0 {
		content.WriteString(S.Warning.Render("No pods found for this release.") + "\n")
	} else {
		// Pod list
		for i, pod := range r.logPodList {
			statusStyle := r.getPodStatusStyle(pod.Status)

			prefix := "  "
			if i == r.logSelectedPodIdx {
				prefix = Icons.Arrow + " "
			}

			// Pod name and status
			line := fmt.Sprintf("%s%s  %s",
				prefix,
				S.Value.Render(pod.Name),
				statusStyle.Render(pod.Status),
			)

			if i == r.logSelectedPodIdx {
				line = S.Selected.Render(line)
			}
			content.WriteString(line + "\n")

			// Show containers if selected pod has multiple
			if i == r.logSelectedPodIdx && len(pod.Containers) > 1 {
				content.WriteString(S.Label.Render("    Containers: "))
				for ci, c := range pod.Containers {
					if ci == r.logContainerIdx {
						content.WriteString(S.Highlighted.Render("[" + c + "]"))
					} else {
						content.WriteString(S.Muted.Render(" " + c + " "))
					}
				}
				content.WriteString("\n")
			}
		}
	}

	content.WriteString("\n")

	// Options toggles
	tsIcon := "[ ]"
	if r.logShowTimestamps {
		tsIcon = "[" + Icons.Check + "]"
	}
	prevIcon := "[ ]"
	if r.logShowPrevious {
		prevIcon = "[" + Icons.Check + "]"
	}

	content.WriteString(S.Muted.Render("Options:") + "\n")
	content.WriteString(fmt.Sprintf("  t: %s Timestamps\n", tsIcon))
	content.WriteString(fmt.Sprintf("  p: %s Previous container\n", prevIcon))

	content.WriteString("\n")

	// Footer hints
	hints := "j/k:select  "
	if len(r.logPodList) > 0 && len(r.logPodList[r.logSelectedPodIdx].Containers) > 1 {
		hints += "h/l:container  "
	}
	hints += "Enter:view logs  Esc:close"
	content.WriteString(S.Muted.Render(hints))

	return content.String()
}

func (r *ReleaseDetail) renderLogViewer(width, height int) string {
	var content strings.Builder

	// Title with pod/container info
	containerInfo := ""
	if r.logSelectedPod != nil && len(r.logSelectedPod.Containers) > 1 {
		if r.logContainerIdx < len(r.logSelectedPod.Containers) {
			containerInfo = " / " + r.logSelectedPod.Containers[r.logContainerIdx]
		}
	}
	podName := ""
	if r.logSelectedPod != nil {
		podName = r.logSelectedPod.Name
	}
	title := fmt.Sprintf("%s Logs: %s%s",
		Icons.Deployed,
		podName,
		containerInfo,
	)
	content.WriteString(S.Title.Render(title) + "\n")

	// Options status bar
	var optionParts []string
	if r.logShowTimestamps {
		optionParts = append(optionParts, "timestamps:on")
	}
	if r.logShowPrevious {
		optionParts = append(optionParts, "previous:on")
	}
	if len(optionParts) > 0 {
		content.WriteString(S.Muted.Render("  ["+strings.Join(optionParts, ", ")+"]") + "\n")
	}
	content.WriteString("\n")

	// Separator
	content.WriteString(strings.Repeat("─", width-6) + "\n")

	// Loading/Error/Content
	if r.logLoading {
		content.WriteString(S.Muted.Render(Icons.Helm+" Fetching logs...") + "\n")
	} else if r.logError != nil {
		content.WriteString(S.Error.Render(Icons.Cross+" "+r.logError.Error()) + "\n")
	} else if r.logContent == "" {
		content.WriteString(S.Muted.Render("No logs available.") + "\n")
	} else {
		// Update viewport dimensions
		viewportHeight := height - 12
		if viewportHeight < 5 {
			viewportHeight = 5
		}
		r.logViewport.Width = width - 6
		r.logViewport.Height = viewportHeight

		content.WriteString(r.logViewport.View() + "\n")
	}

	// Separator
	content.WriteString(strings.Repeat("─", width-6) + "\n")

	// Scroll hint
	if r.logViewport.TotalLineCount() > r.logViewport.Height {
		scrollHint := RenderScrollHint(r.logViewport.YOffset,
			r.logViewport.TotalLineCount()-r.logViewport.Height)
		content.WriteString(scrollHint + "\n")
	}

	// Footer hints
	content.WriteString(S.Muted.Render("r:refresh  t:timestamps  p:previous  j/k:scroll  g/G:top/bottom  Esc:back  q:close"))

	return content.String()
}

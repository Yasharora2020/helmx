package tui

import (
	"fmt"
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/yasharora2020/helmx/internal/helm"
)

// Exec (shell access) message types
type execPodListLoadedMsg struct {
	pods []helm.PodInfo
	err  error
}

type execCompletedMsg struct {
	err error
}

func (r *ReleaseDetail) fetchPodListForExec() tea.Cmd {
	releaseName := r.release.Name
	namespace := r.release.Namespace

	return func() tea.Msg {
		pods, err := r.clusterInspector.GetReleasePods(releaseName, namespace)
		return execPodListLoadedMsg{pods: pods, err: err}
	}
}

// updateExecDialog routes key events for the exec/shell dialog
func (r *ReleaseDetail) updateExecDialog(msg tea.KeyMsg) (*ReleaseDetail, tea.Cmd) {
	switch r.execDialogStep {
	case ExecStepSelectPod:
		return r.updateExecPodSelection(msg)
	case ExecStepSelectShell:
		return r.updateExecShellSelection(msg)
	}
	return r, nil
}

// updateExecPodSelection handles pod selection in exec dialog
func (r *ReleaseDetail) updateExecPodSelection(msg tea.KeyMsg) (*ReleaseDetail, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		r.showExecDialog = false
		return r, nil
	case "up", "k":
		if r.execSelectedPodIdx > 0 {
			r.execSelectedPodIdx--
		}
	case "down", "j":
		if r.execSelectedPodIdx < len(r.execPodList)-1 {
			r.execSelectedPodIdx++
		}
	case "tab":
		// Cycle through containers of selected pod
		if r.execSelectedPodIdx < len(r.execPodList) {
			pod := r.execPodList[r.execSelectedPodIdx]
			if len(pod.Containers) > 1 {
				r.execContainerIdx = (r.execContainerIdx + 1) % len(pod.Containers)
			}
		}
	case "enter":
		if len(r.execPodList) > 0 && r.execSelectedPodIdx < len(r.execPodList) {
			pod := r.execPodList[r.execSelectedPodIdx]
			// Only allow exec into running pods
			if pod.Status == "Running" {
				r.execSelectedPod = &pod
				r.execDialogStep = ExecStepSelectShell
				r.execShellIdx = 0
			}
		}
	}
	return r, nil
}

// updateExecShellSelection handles shell selection in exec dialog
func (r *ReleaseDetail) updateExecShellSelection(msg tea.KeyMsg) (*ReleaseDetail, tea.Cmd) {
	switch msg.String() {
	case "esc":
		// Go back to pod selection
		r.execDialogStep = ExecStepSelectPod
		return r, nil
	case "q":
		// Close entire dialog
		r.showExecDialog = false
		return r, nil
	case "up", "k":
		if r.execShellIdx > 0 {
			r.execShellIdx--
		}
	case "down", "j":
		if r.execShellIdx < len(supportedShells)-1 {
			r.execShellIdx++
		}
	case "enter":
		// Execute shell
		r.showExecDialog = false
		return r, r.executeShell()
	}
	return r, nil
}

// executeShell runs kubectl exec to open a shell in the selected pod
func (r *ReleaseDetail) executeShell() tea.Cmd {
	pod := r.execSelectedPod
	if pod == nil {
		return func() tea.Msg {
			return execCompletedMsg{err: fmt.Errorf("no pod selected")}
		}
	}

	containerName := ""
	if r.execContainerIdx < len(pod.Containers) {
		containerName = pod.Containers[r.execContainerIdx]
	}
	shell := supportedShells[r.execShellIdx]

	args := []string{"exec", "-it", pod.Name, "-n", pod.Namespace}
	if containerName != "" {
		args = append(args, "-c", containerName)
	}
	args = append(args, "--", shell)

	cmd := exec.Command("kubectl", args...)
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		return execCompletedMsg{err: err}
	})
}

// renderExecDialog renders the exec/shell dialog
func (r *ReleaseDetail) renderExecDialog() string {
	width := 70
	height := 20

	var content strings.Builder

	// Title
	title := " ⌨ Shell Access "
	content.WriteString(S.Title.Render(title) + "\n\n")

	if r.execLoading {
		content.WriteString(S.Muted.Render("Loading pods...") + "\n")
	} else if r.execError != nil {
		content.WriteString(S.Error.Render(Icons.Cross+" "+r.execError.Error()) + "\n")
	} else if r.execDialogStep == ExecStepSelectPod {
		content.WriteString(r.renderExecPodSelection())
	} else if r.execDialogStep == ExecStepSelectShell {
		content.WriteString(r.renderExecShellSelection())
	}

	// Build dialog box
	dialogStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(DefaultTheme.BorderFocus).
		Padding(1, 2).
		Width(width).
		Height(height)

	dialog := dialogStyle.Render(content.String())

	// Center the dialog
	return lipgloss.Place(r.width, r.height, lipgloss.Center, lipgloss.Center, dialog)
}

// renderExecPodSelection renders the pod selection step
func (r *ReleaseDetail) renderExecPodSelection() string {
	var content strings.Builder

	if len(r.execPodList) == 0 {
		content.WriteString(S.Muted.Render("No pods found for this release.") + "\n")
		content.WriteString("\n")
		content.WriteString(S.Muted.Render("Esc:close"))
		return content.String()
	}

	content.WriteString(S.Muted.Render("Select a pod to exec into:") + "\n\n")

	for i, pod := range r.execPodList {
		prefix := "  "
		if i == r.execSelectedPodIdx {
			prefix = Icons.Arrow + " "
		}

		// Pod name and status
		statusStyle := r.getPodStatusStyle(pod.Status)
		podLine := prefix + pod.Name + " " + statusStyle.Render("["+pod.Status+"]")

		// Show selected container if multiple
		if i == r.execSelectedPodIdx && len(pod.Containers) > 1 {
			containerName := pod.Containers[r.execContainerIdx]
			podLine += S.Muted.Render(" (container: " + containerName + ")")
		}

		content.WriteString(podLine + "\n")

		// Show container info for selected pod
		if i == r.execSelectedPodIdx && len(pod.Containers) > 1 {
			content.WriteString("   " + S.Muted.Render(fmt.Sprintf("Tab to cycle containers (%d available)", len(pod.Containers))) + "\n")
		}
	}

	content.WriteString("\n")
	content.WriteString(S.Muted.Render("j/k:navigate  Tab:container  Enter:select  Esc:cancel"))

	return content.String()
}

// renderExecShellSelection renders the shell selection step
func (r *ReleaseDetail) renderExecShellSelection() string {
	var content strings.Builder

	pod := r.execSelectedPod
	containerName := ""
	if r.execContainerIdx < len(pod.Containers) {
		containerName = pod.Containers[r.execContainerIdx]
	}

	content.WriteString(S.Muted.Render("Pod: ") + S.Value.Render(pod.Name) + "\n")
	if containerName != "" {
		content.WriteString(S.Muted.Render("Container: ") + S.Value.Render(containerName) + "\n")
	}
	content.WriteString("\n")
	content.WriteString(S.Muted.Render("Select shell:") + "\n\n")

	for i, shell := range supportedShells {
		prefix := "  "
		if i == r.execShellIdx {
			prefix = Icons.Arrow + " "
		}
		content.WriteString(prefix + shell + "\n")
	}

	content.WriteString("\n")
	content.WriteString(S.Muted.Render("j/k:navigate  Enter:exec  Esc:back  q:close"))

	return content.String()
}

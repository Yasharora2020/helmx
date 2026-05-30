package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/yasharora2020/helmx/internal/helm"
)

// Port forward message types
type pfPodListLoadedMsg struct {
	pods []helm.PodInfo
	err  error
}

type pfPodPortsLoadedMsg struct {
	ports []helm.PodPort
	err   error
}

type pfStartedMsg struct {
	pf  *helm.PortForward
	err error
}

type pfStoppedMsg struct {
	err error
}

// fetchPodListForPortForward fetches pods for port forwarding
func (r *ReleaseDetail) fetchPodListForPortForward() tea.Cmd {
	releaseName := r.release.Name
	namespace := r.release.Namespace

	return func() tea.Msg {
		pods, err := r.clusterInspector.GetReleasePods(releaseName, namespace)
		return pfPodListLoadedMsg{pods: pods, err: err}
	}
}

// fetchPodPorts fetches available ports from a pod
func (r *ReleaseDetail) fetchPodPorts(podName, namespace string) tea.Cmd {
	return func() tea.Msg {
		ports, err := r.clusterInspector.GetPodPorts(podName, namespace)
		return pfPodPortsLoadedMsg{ports: ports, err: err}
	}
}

// startPortForward starts a new port forward
func (r *ReleaseDetail) startPortForward(podName, namespace string, localPort, remotePort int) tea.Cmd {
	manager := r.pfManager
	releaseName := r.release.Name

	return func() tea.Msg {
		pf, err := manager.StartPortForward(podName, namespace, localPort, remotePort, releaseName)
		return pfStartedMsg{pf: pf, err: err}
	}
}

// stopPortForward stops a port forward by ID
func (r *ReleaseDetail) stopPortForward(id string) tea.Cmd {
	manager := r.pfManager

	return func() tea.Msg {
		err := manager.StopPortForward(id)
		return pfStoppedMsg{err: err}
	}
}

// updatePortForwardDialog routes key events for the port forward dialog
func (r *ReleaseDetail) updatePortForwardDialog(msg tea.KeyMsg) (*ReleaseDetail, tea.Cmd) {
	switch r.pfDialogStep {
	case PFStepManage:
		return r.updatePFManage(msg)
	case PFStepSelectPod:
		return r.updatePFPodSelection(msg)
	case PFStepSelectPort:
		return r.updatePFPortSelection(msg)
	}
	return r, nil
}

// updatePFManage handles the manage view (list active forwards, option to add new)
func (r *ReleaseDetail) updatePFManage(msg tea.KeyMsg) (*ReleaseDetail, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		r.showPortForwardDialog = false
		return r, nil
	case "n", "a":
		// New port forward
		r.pfDialogStep = PFStepSelectPod
		r.pfSelectedPodIdx = 0
		r.pfError = nil
		return r, nil
	case "d", "x":
		// Delete/stop selected port forward
		if len(r.pfActiveForwards) > 0 && r.pfSelectedPortIdx < len(r.pfActiveForwards) {
			pf := r.pfActiveForwards[r.pfSelectedPortIdx]
			return r, r.stopPortForward(pf.ID)
		}
	case "up", "k":
		if r.pfSelectedPortIdx > 0 {
			r.pfSelectedPortIdx--
		}
	case "down", "j":
		if r.pfSelectedPortIdx < len(r.pfActiveForwards)-1 {
			r.pfSelectedPortIdx++
		}
	}
	return r, nil
}

// updatePFPodSelection handles pod selection for new port forward
func (r *ReleaseDetail) updatePFPodSelection(msg tea.KeyMsg) (*ReleaseDetail, tea.Cmd) {
	switch msg.String() {
	case "esc":
		r.pfDialogStep = PFStepManage
		return r, nil
	case "q":
		r.showPortForwardDialog = false
		return r, nil
	case "up", "k":
		if r.pfSelectedPodIdx > 0 {
			r.pfSelectedPodIdx--
		}
	case "down", "j":
		if r.pfSelectedPodIdx < len(r.pfPodList)-1 {
			r.pfSelectedPodIdx++
		}
	case "enter":
		if len(r.pfPodList) > 0 && r.pfSelectedPodIdx < len(r.pfPodList) {
			pod := r.pfPodList[r.pfSelectedPodIdx]
			if pod.Status == "Running" {
				r.pfDialogStep = PFStepSelectPort
				r.pfLoading = true
				r.pfError = nil
				return r, r.fetchPodPorts(pod.Name, pod.Namespace)
			}
		}
	}
	return r, nil
}

// updatePFPortSelection handles port selection and starting the port forward
func (r *ReleaseDetail) updatePFPortSelection(msg tea.KeyMsg) (*ReleaseDetail, tea.Cmd) {
	switch msg.String() {
	case "esc":
		r.pfDialogStep = PFStepSelectPod
		return r, nil
	case "q":
		r.showPortForwardDialog = false
		return r, nil
	case "up", "k":
		if r.pfSelectedPortIdx > 0 {
			r.pfSelectedPortIdx--
			if len(r.pfPodPorts) > 0 {
				r.pfLocalPort = fmt.Sprint(r.pfPodPorts[r.pfSelectedPortIdx].ContainerPort)
			}
		}
	case "down", "j":
		if r.pfSelectedPortIdx < len(r.pfPodPorts)-1 {
			r.pfSelectedPortIdx++
			if len(r.pfPodPorts) > 0 {
				r.pfLocalPort = fmt.Sprint(r.pfPodPorts[r.pfSelectedPortIdx].ContainerPort)
			}
		}
	case "enter":
		if len(r.pfPodPorts) > 0 && r.pfSelectedPortIdx < len(r.pfPodPorts) {
			pod := r.pfPodList[r.pfSelectedPodIdx]
			port := r.pfPodPorts[r.pfSelectedPortIdx]

			// Parse local port
			localPort := port.ContainerPort
			if r.pfLocalPort != "" {
				if parsed, err := fmt.Sscanf(r.pfLocalPort, "%d", &localPort); err != nil || parsed != 1 {
					localPort = port.ContainerPort
				}
			}

			r.pfLoading = true
			r.pfDialogStep = PFStepManage
			return r, r.startPortForward(pod.Name, pod.Namespace, localPort, port.ContainerPort)
		}
	case "0", "1", "2", "3", "4", "5", "6", "7", "8", "9":
		// Allow typing local port
		if len(r.pfLocalPort) < 5 {
			r.pfLocalPort += msg.String()
		}
	case "backspace":
		if len(r.pfLocalPort) > 0 {
			r.pfLocalPort = r.pfLocalPort[:len(r.pfLocalPort)-1]
		}
	}
	return r, nil
}

// renderPortForwardDialog renders the port forward dialog
func (r *ReleaseDetail) renderPortForwardDialog() string {
	width := 70
	height := 22

	var content strings.Builder

	// Title
	title := " 🔌 Port Forward "
	content.WriteString(S.Title.Render(title) + "\n\n")

	switch r.pfDialogStep {
	case PFStepManage:
		content.WriteString(r.renderPFManage())
	case PFStepSelectPod:
		content.WriteString(r.renderPFPodSelection())
	case PFStepSelectPort:
		content.WriteString(r.renderPFPortSelection())
	}

	// Error display
	if r.pfError != nil {
		content.WriteString("\n" + S.Error.Render(Icons.Cross+" "+r.pfError.Error()) + "\n")
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

// renderPFManage renders the manage port forwards view
func (r *ReleaseDetail) renderPFManage() string {
	var content strings.Builder

	if r.pfLoading {
		content.WriteString(S.Muted.Render(Icons.Helm+" Loading...") + "\n")
		return content.String()
	}

	// Active port forwards
	content.WriteString(S.Label.Render("Active Port Forwards:") + "\n\n")

	if len(r.pfActiveForwards) == 0 {
		content.WriteString(S.Muted.Render("  No active port forwards for this release.") + "\n")
	} else {
		for i, pf := range r.pfActiveForwards {
			prefix := "  "
			if i == r.pfSelectedPortIdx {
				prefix = Icons.Arrow + " "
			}

			statusIcon := Icons.Check
			statusStyle := S.Success
			if pf.Status == "error" {
				statusIcon = Icons.Cross
				statusStyle = S.Error
			} else if pf.Status == "starting" {
				statusIcon = Icons.Helm
				statusStyle = S.Warning
			}

			line := fmt.Sprintf("%s%s localhost:%d → %s:%d",
				prefix,
				statusStyle.Render(statusIcon),
				pf.LocalPort,
				pf.PodName,
				pf.RemotePort,
			)

			if i == r.pfSelectedPortIdx {
				line = S.Selected.Render(line)
			}
			content.WriteString(line + "\n")
		}
	}

	content.WriteString("\n")
	content.WriteString(strings.Repeat("─", 60) + "\n\n")

	// Hints
	if len(r.pfActiveForwards) > 0 {
		content.WriteString(S.Muted.Render("n:new forward  d:stop selected  j/k:navigate  Esc:close"))
	} else {
		content.WriteString(S.Muted.Render("n:new forward  Esc:close"))
	}

	return content.String()
}

// renderPFPodSelection renders the pod selection view
func (r *ReleaseDetail) renderPFPodSelection() string {
	var content strings.Builder

	content.WriteString(S.Label.Render("Select Pod:") + "\n\n")

	if r.pfLoading {
		content.WriteString(S.Muted.Render(Icons.Helm+" Loading pods...") + "\n")
		return content.String()
	}

	if len(r.pfPodList) == 0 {
		content.WriteString(S.Warning.Render("No pods found for this release.") + "\n")
	} else {
		for i, pod := range r.pfPodList {
			prefix := "  "
			if i == r.pfSelectedPodIdx {
				prefix = Icons.Arrow + " "
			}

			statusStyle := r.getPodStatusStyle(pod.Status)
			line := fmt.Sprintf("%s%s  %s",
				prefix,
				pod.Name,
				statusStyle.Render("["+pod.Status+"]"),
			)

			if i == r.pfSelectedPodIdx {
				line = S.Selected.Render(line)
			}
			content.WriteString(line + "\n")
		}
	}

	content.WriteString("\n")
	content.WriteString(S.Muted.Render("j/k:navigate  Enter:select  Esc:back"))

	return content.String()
}

// renderPFPortSelection renders the port selection view
func (r *ReleaseDetail) renderPFPortSelection() string {
	var content strings.Builder

	pod := r.pfPodList[r.pfSelectedPodIdx]
	content.WriteString(S.Muted.Render("Pod: ") + S.Value.Render(pod.Name) + "\n\n")

	content.WriteString(S.Label.Render("Select Port:") + "\n\n")

	if r.pfLoading {
		content.WriteString(S.Muted.Render(Icons.Helm+" Loading ports...") + "\n")
		return content.String()
	}

	if len(r.pfPodPorts) == 0 {
		content.WriteString(S.Warning.Render("No exposed ports found in this pod.") + "\n")
		content.WriteString(S.Muted.Render("The pod may not have container ports defined.") + "\n")
	} else {
		for i, port := range r.pfPodPorts {
			prefix := "  "
			if i == r.pfSelectedPortIdx {
				prefix = Icons.Arrow + " "
			}

			portName := ""
			if port.Name != "" {
				portName = fmt.Sprintf(" (%s)", port.Name)
			}

			line := fmt.Sprintf("%s%d/%s%s  [%s]",
				prefix,
				port.ContainerPort,
				port.Protocol,
				portName,
				port.ContainerName,
			)

			if i == r.pfSelectedPortIdx {
				line = S.Selected.Render(line)
			}
			content.WriteString(line + "\n")
		}

		// Show local port input
		content.WriteString("\n")
		content.WriteString(S.Label.Render("Local port: ") + S.Highlighted.Render(r.pfLocalPort))
		if r.pfLocalPort == "" {
			content.WriteString(S.Muted.Render(" (same as remote)"))
		}
		content.WriteString("\n")
	}

	content.WriteString("\n")
	content.WriteString(S.Muted.Render("j/k:navigate  0-9:set local port  Enter:start  Esc:back"))

	return content.String()
}

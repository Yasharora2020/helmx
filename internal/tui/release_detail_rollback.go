package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type rollbackResultMsg struct {
	err error
}

func (r *ReleaseDetail) updateRollbackConfirm(msg tea.KeyMsg) (*ReleaseDetail, tea.Cmd) {
	switch msg.String() {
	case "y", "Y", "enter":
		return r, r.executeRollback()
	case "n", "N", "esc":
		r.showRollbackConfirm = false
	}
	return r, nil
}

func (r *ReleaseDetail) executeRollback() tea.Cmd {
	target := r.rollbackTarget
	name := r.release.Name
	ns := r.release.Namespace

	return func() tea.Msg {
		err := r.releaseManager.Rollback(name, ns, target)
		return rollbackResultMsg{err: err}
	}
}

func (r *ReleaseDetail) renderRollbackConfirm() string {
	dialogStyle := lipgloss.NewStyle().
		Border(lipgloss.DoubleBorder()).
		BorderForeground(DefaultTheme.Warning).
		Padding(2, 4).
		Width(50)

	content := S.Warning.Render(Icons.Rollback+" Confirm Rollback") + "\n\n"
	content += fmt.Sprintf("Rollback %s to revision %d?\n\n", r.release.Name, r.rollbackTarget)
	content += S.Muted.Render("This will restore the previous configuration.") + "\n\n"
	content += S.Muted.Render("[Y]es  [N]o")

	dialog := dialogStyle.Render(content)

	// Center it
	return lipgloss.Place(r.width, r.height, lipgloss.Center, lipgloss.Center, dialog)
}

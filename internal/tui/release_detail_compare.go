package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Revision comparison message types
type revisionDataMsg struct {
	values1   string
	values2   string
	manifest1 string
	manifest2 string
	err       error
}

// fetchRevisionData fetches values and manifests for both revisions being compared
func (r *ReleaseDetail) fetchRevisionData() tea.Cmd {
	name := r.release.Name
	namespace := r.release.Namespace
	rev1 := r.compareRev1
	rev2 := r.compareRev2

	return func() tea.Msg {
		// Fetch first revision
		rel1, err := r.releaseManager.GetRevision(name, namespace, rev1)
		if err != nil {
			return revisionDataMsg{err: fmt.Errorf("failed to get revision %d: %w", rev1, err)}
		}

		// Fetch second revision
		rel2, err := r.releaseManager.GetRevision(name, namespace, rev2)
		if err != nil {
			return revisionDataMsg{err: fmt.Errorf("failed to get revision %d: %w", rev2, err)}
		}

		// Format values as YAML
		values1 := GenerateValuesYAML(rel1.Config)
		values2 := GenerateValuesYAML(rel2.Config)

		return revisionDataMsg{
			values1:   values1,
			values2:   values2,
			manifest1: rel1.Manifest,
			manifest2: rel2.Manifest,
		}
	}
}

// updateRevisionCompare handles key events for the revision comparison dialog
func (r *ReleaseDetail) updateRevisionCompare(msg tea.KeyMsg) (*ReleaseDetail, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		r.showRevisionCompare = false
		return r, nil
	case "tab", "1", "2":
		// Toggle between values and manifest tabs
		if msg.String() == "tab" {
			r.compareActiveTab = (r.compareActiveTab + 1) % 2
		} else if msg.String() == "1" {
			r.compareActiveTab = 0
		} else {
			r.compareActiveTab = 1
		}
		r.updateCompareViewportContent()
	case "j", "down":
		r.compareViewport.LineDown(1)
	case "k", "up":
		r.compareViewport.LineUp(1)
	case "ctrl+d":
		r.compareViewport.HalfViewDown()
	case "ctrl+u":
		r.compareViewport.HalfViewUp()
	case "g":
		r.compareViewport.GotoTop()
	case "G":
		r.compareViewport.GotoBottom()
	}

	var cmd tea.Cmd
	r.compareViewport, cmd = r.compareViewport.Update(msg)
	return r, cmd
}

// updateCompareViewportContent updates the viewport content based on active tab
func (r *ReleaseDetail) updateCompareViewportContent() {
	var content string

	if r.compareActiveTab == 0 {
		// Values diff
		if !r.compareValuesDiff.HasChanges {
			content = S.Muted.Render("No value changes between revisions.")
		} else {
			content = RenderDiff(r.compareValuesDiff, 3)
		}
	} else {
		// Manifest diff
		if !r.compareManifestDiff.HasChanges {
			content = S.Muted.Render("No manifest changes between revisions.")
		} else {
			content = RenderDiff(r.compareManifestDiff, 3)
		}
	}

	r.compareViewport.SetContent(content)
}

// renderRevisionCompare renders the revision comparison dialog
func (r *ReleaseDetail) renderRevisionCompare() string {
	dialogWidth := r.width - 10
	if dialogWidth > 120 {
		dialogWidth = 120
	}
	dialogHeight := r.height - 8

	dialogStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(DefaultTheme.Primary).
		Padding(1, 2).
		Width(dialogWidth)

	// Title
	title := fmt.Sprintf("%s Compare Revisions: %s", Icons.Helm, r.release.Name)
	subtitle := fmt.Sprintf("Revision %d → Revision %d (current)", r.compareRev1, r.compareRev2)

	// Tab bar
	tabs := []string{"Values", "Manifest"}
	tabStyle := lipgloss.NewStyle().Padding(0, 2)
	activeTabStyle := tabStyle.Background(DefaultTheme.Primary).Foreground(lipgloss.Color("#000"))
	inactiveTabStyle := tabStyle.Foreground(DefaultTheme.Muted)

	var tabBar strings.Builder
	for i, tab := range tabs {
		indicator := ""
		if i == 0 && r.compareValuesDiff.HasChanges {
			indicator = " " + DiffSummary(r.compareValuesDiff)
		} else if i == 1 && r.compareManifestDiff.HasChanges {
			indicator = " " + DiffSummary(r.compareManifestDiff)
		}

		if i == r.compareActiveTab {
			tabBar.WriteString(activeTabStyle.Render(tab + indicator))
		} else {
			tabBar.WriteString(inactiveTabStyle.Render(tab + indicator))
		}
		tabBar.WriteString(" ")
	}

	// Update viewport size
	r.compareViewport.Width = dialogWidth - 6
	r.compareViewport.Height = dialogHeight - 12

	// Scroll indicator
	scrollInfo := ""
	if r.compareViewport.TotalLineCount() > r.compareViewport.Height {
		scrollInfo = RenderScrollHint(r.compareViewport.YOffset, r.compareViewport.TotalLineCount()-r.compareViewport.Height)
	}

	// Build content
	var content strings.Builder
	content.WriteString(S.Title.Render(title) + "\n")
	content.WriteString(S.Muted.Render(subtitle) + "\n\n")
	content.WriteString(tabBar.String() + "\n")
	content.WriteString(strings.Repeat("─", dialogWidth-6) + scrollInfo + "\n")

	if r.compareLoading {
		content.WriteString(S.Muted.Render(Icons.Helm+" Loading revision data...") + "\n")
	} else if r.compareError != nil {
		content.WriteString(S.Error.Render(Icons.Cross+" "+r.compareError.Error()) + "\n")
	} else {
		content.WriteString(r.compareViewport.View() + "\n")
	}

	content.WriteString(strings.Repeat("─", dialogWidth-6) + "\n")

	// Footer with keybindings
	content.WriteString(S.Muted.Render("Tab:switch tabs  j/k:scroll  g/G:top/bottom  Esc:close"))

	dialog := dialogStyle.Render(content.String())
	return lipgloss.Place(r.width, r.height, lipgloss.Center, lipgloss.Center, dialog)
}

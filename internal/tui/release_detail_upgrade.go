package tui

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/yasharora2020/helmx/internal/helm"
	"gopkg.in/yaml.v3"
)

type upgradeResultMsg struct {
	release *helm.Release
	err     error
}

type upgradeValuesEditedMsg struct {
	content string
	err     error
}

type manifestMsg struct {
	manifest string
	err      error
}

type diffComputedMsg struct {
	newManifest string
	err         error
}

func (r *ReleaseDetail) updateUpgradeConfirm(msg tea.KeyMsg) (*ReleaseDetail, tea.Cmd) {
	// Ignore keys while upgrade is in progress
	if r.upgrading {
		return r, nil
	}

	switch msg.String() {
	case "y", "Y", "enter":
		r.upgrading = true
		return r, r.executeUpgrade()
	case "n", "N", "esc":
		r.showUpgradeConfirm = false
		r.upgradeValuesContent = ""
		r.upgradeValueFiles = nil
		r.upgradeSetValues = nil
	}
	return r, nil
}

func (r *ReleaseDetail) updateDiffPreview(msg tea.KeyMsg) (*ReleaseDetail, tea.Cmd) {
	switch msg.String() {
	case "tab":
		// Toggle between values and manifest tabs
		r.diffActiveTab = (r.diffActiveTab + 1) % 2
		r.updateDiffViewportContent()
	case "1":
		r.diffActiveTab = 0
		r.updateDiffViewportContent()
	case "2":
		r.diffActiveTab = 1
		r.updateDiffViewportContent()
	case "f":
		// Add value file
		if r.upgradeFileImportDialog != nil {
			r.upgradeFileImportDialog.Open()
		}
		return r, nil
	case "s":
		// Add --set override
		if r.upgradeSetValueDialog != nil {
			r.upgradeSetValueDialog.Open()
		}
		return r, nil
	case "F":
		// Manage value sources
		if r.upgradeValueSourcesDialog != nil {
			r.upgradeValueSourcesDialog.OpenWith(r.upgradeValueFiles, r.upgradeSetValues)
		}
		return r, nil
	case "e":
		// Edit values in external editor
		r.showDiffPreview = false
		return r, r.openExternalEditor()
	case "y", "Y", "enter":
		// Proceed to upgrade confirmation
		if r.valuesDiff.HasChanges || r.manifestDiff.HasChanges {
			r.showDiffPreview = false
			r.showUpgradeConfirm = true
		}
	case "n", "N", "esc", "q":
		r.showDiffPreview = false
		r.upgradeValuesContent = ""
		r.upgradeValueFiles = nil
		r.upgradeSetValues = nil
	case "j", "down":
		r.diffViewport.LineDown(1)
	case "k", "up":
		r.diffViewport.LineUp(1)
	case "g":
		r.diffViewport.GotoTop()
	case "G":
		r.diffViewport.GotoBottom()
	case "ctrl+d":
		r.diffViewport.HalfViewDown()
	case "ctrl+u":
		r.diffViewport.HalfViewUp()
	}

	var cmd tea.Cmd
	r.diffViewport, cmd = r.diffViewport.Update(msg)
	return r, cmd
}

func (r *ReleaseDetail) updateDiffViewportContent() {
	var content string

	if r.diffActiveTab == 0 {
		// Values diff
		if !r.valuesDiff.HasChanges {
			content = S.Muted.Render("No value changes.\n\nPress 'e' to edit values in your editor.")
		} else {
			content = RenderDiff(r.valuesDiff, 3)
		}
	} else {
		// Manifest diff
		if r.loadingDiff {
			content = S.Muted.Render(Icons.Helm + " Computing manifest diff...")
		} else if !r.manifestDiff.HasChanges {
			content = S.Muted.Render("No manifest changes.")
		} else {
			content = RenderDiff(r.manifestDiff, 3)
		}
	}

	r.diffViewport.SetContent(content)
}

func (r *ReleaseDetail) fetchCurrentManifest() tea.Cmd {
	return func() tea.Msg {
		manifest, err := r.releaseManager.GetManifest(r.release.Name, r.release.Namespace)
		return manifestMsg{manifest: manifest, err: err}
	}
}

func (r *ReleaseDetail) computeManifestDiff() tea.Cmd {
	name := r.release.Name
	ns := r.release.Namespace
	content := r.upgradeValuesContent
	valueFiles := r.upgradeValueFiles
	setValues := r.upgradeSetValues
	mgr := r.releaseManager

	return func() tea.Msg {
		composer := &helm.ValuesComposer{
			ValueFiles: valueFiles,
			InlineYAML: content,
			SetValues:  setValues,
		}
		values, err := composer.Compose()
		if err != nil {
			return diffComputedMsg{err: fmt.Errorf("failed to merge values: %w", err)}
		}

		newManifest, err := mgr.DryRunUpgradeValues(name, ns, values)
		if err != nil {
			return diffComputedMsg{err: fmt.Errorf("failed to compute manifest diff: %w", err)}
		}

		return diffComputedMsg{newManifest: newManifest}
	}
}

func (r *ReleaseDetail) executeUpgrade() tea.Cmd {
	name := r.release.Name
	ns := r.release.Namespace
	content := r.upgradeValuesContent
	valueFiles := r.upgradeValueFiles
	setValues := r.upgradeSetValues
	mgr := r.releaseManager

	return func() tea.Msg {
		composer := &helm.ValuesComposer{
			ValueFiles: valueFiles,
			InlineYAML: content,
			SetValues:  setValues,
		}
		values, err := composer.Compose()
		if err != nil {
			return upgradeResultMsg{err: fmt.Errorf("failed to merge values: %w", err)}
		}

		release, err := mgr.UpgradeValues(name, ns, values)
		return upgradeResultMsg{release: release, err: err}
	}
}

// openExternalEditor opens the values in an external editor ($EDITOR or vim)
// Uses user-supplied values only, not computed values, so chart defaults can be updated
func (r *ReleaseDetail) openExternalEditor() tea.Cmd {
	content := r.userValues

	// Create temp file
	tmpFile, err := os.CreateTemp("", "helmx-upgrade-*.yaml")
	if err != nil {
		return func() tea.Msg {
			return upgradeValuesEditedMsg{err: err}
		}
	}
	tmpPath := tmpFile.Name()

	// Write content to temp file
	if _, err := tmpFile.WriteString(content); err != nil {
		_ = tmpFile.Close()
		_ = os.Remove(tmpPath)
		return func() tea.Msg {
			return upgradeValuesEditedMsg{err: err}
		}
	}
	_ = tmpFile.Close()

	// Get editor from $EDITOR, fallback to vim
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vim"
	}

	// Use tea.ExecProcess to suspend TUI and run editor
	cmd := exec.Command(editor, tmpPath)
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		defer os.Remove(tmpPath)

		if err != nil {
			return upgradeValuesEditedMsg{err: err}
		}

		// Read modified content
		data, err := os.ReadFile(tmpPath)
		if err != nil {
			return upgradeValuesEditedMsg{err: err}
		}

		return upgradeValuesEditedMsg{content: string(data)}
	})
}

// ParseYAMLValues parses YAML content into a map
func ParseYAMLValues(content string) (map[string]interface{}, error) {
	var values map[string]interface{}
	if err := yaml.Unmarshal([]byte(content), &values); err != nil {
		return nil, err
	}
	return values, nil
}

func (r *ReleaseDetail) renderUpgradeConfirm() string {
	dialogStyle := lipgloss.NewStyle().
		Border(lipgloss.DoubleBorder()).
		BorderForeground(DefaultTheme.Primary).
		Padding(1, 2).
		Width(60)

	content := S.Title.Render(Icons.Upgrade+" Confirm Upgrade: "+r.release.Name) + "\n\n"

	// Show summary
	content += S.Label.Render("Changes Summary:") + "\n"
	content += "  Values: " + DiffSummary(r.valuesDiff) + "\n"
	content += "  Manifest: " + DiffSummary(r.manifestDiff) + "\n\n"

	if r.upgrading {
		content += S.Muted.Render(Icons.Helm + " Upgrading...")
	} else {
		content += S.Muted.Render("[Y]es to upgrade  [N]o to cancel")
	}

	dialog := dialogStyle.Render(content)
	return lipgloss.Place(r.width, r.height, lipgloss.Center, lipgloss.Center, dialog)
}

func (r *ReleaseDetail) renderDiffPreview() string {
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
	title := S.Title.Render(Icons.Helm + " Diff Preview: " + r.release.Name)

	// Tab bar
	tabs := []string{"Values", "Manifest"}
	tabStyle := lipgloss.NewStyle().Padding(0, 2)
	activeTabStyle := tabStyle.Background(DefaultTheme.Primary).Foreground(lipgloss.Color("#000"))
	inactiveTabStyle := tabStyle.Foreground(DefaultTheme.Muted)

	var tabBar strings.Builder
	for i, tab := range tabs {
		indicator := ""
		if i == 0 && r.valuesDiff.HasChanges {
			indicator = " " + DiffSummary(r.valuesDiff)
		} else if i == 1 {
			if r.loadingDiff {
				indicator = " " + S.Muted.Render("...")
			} else if r.manifestDiff.HasChanges {
				indicator = " " + DiffSummary(r.manifestDiff)
			}
		}

		if i == r.diffActiveTab {
			tabBar.WriteString(activeTabStyle.Render(tab + indicator))
		} else {
			tabBar.WriteString(inactiveTabStyle.Render(tab + indicator))
		}
		tabBar.WriteString(" ")
	}

	// Update viewport size
	r.diffViewport.Width = dialogWidth - 6
	r.diffViewport.Height = dialogHeight - 10

	// Scroll indicator
	scrollInfo := ""
	if r.diffViewport.TotalLineCount() > r.diffViewport.Height {
		scrollInfo = RenderScrollHint(r.diffViewport.YOffset, r.diffViewport.TotalLineCount()-r.diffViewport.Height)
	}

	// Build content
	var content strings.Builder
	content.WriteString(title + "\n\n")
	content.WriteString(tabBar.String() + "\n")
	content.WriteString(strings.Repeat("─", dialogWidth-6) + scrollInfo + "\n")
	content.WriteString(r.diffViewport.View() + "\n")
	content.WriteString(strings.Repeat("─", dialogWidth-6) + "\n")

	// Value sources summary
	if len(r.upgradeValueFiles) > 0 || len(r.upgradeSetValues) > 0 {
		composer := &helm.ValuesComposer{
			ValueFiles: r.upgradeValueFiles,
			SetValues:  r.upgradeSetValues,
		}
		content.WriteString(S.Muted.Render("Sources: "+composer.Summary()) + "\n")
	}

	// Footer with keybindings
	hasChanges := r.valuesDiff.HasChanges || r.manifestDiff.HasChanges
	if hasChanges {
		content.WriteString(S.Muted.Render("Tab:switch  e:edit  f:file  s:set  F:sources  Enter:upgrade  Esc:cancel"))
	} else {
		content.WriteString(S.Muted.Render("Tab:switch  e:edit  f:file  s:set  F:sources  Esc:cancel"))
	}

	dialog := dialogStyle.Render(content.String())
	return lipgloss.Place(r.width, r.height, lipgloss.Center, lipgloss.Center, dialog)
}

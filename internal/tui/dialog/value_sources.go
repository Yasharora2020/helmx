package dialog

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ValueSourcesAddFileMsg is sent when the user wants to add a value file from within the sources dialog.
type ValueSourcesAddFileMsg struct{}

// ValueSourcesAddSetMsg is sent when the user wants to add a --set override from within the sources dialog.
type ValueSourcesAddSetMsg struct{}

// ValueSourcesClosedMsg is sent when the dialog is closed, returning the current state.
type ValueSourcesClosedMsg struct {
	ValueFiles []string
	SetValues  []string
}

// ValueSourcesDialog lets users view and remove accumulated value files and --set overrides.
type ValueSourcesDialog struct {
	BaseDialog
	valueFiles  []string
	setValues   []string
	selectedIdx int // index within combined list (files first, then set-values)

	// Styles (injected from parent)
	TitleStyle       lipgloss.Style
	LabelStyle       lipgloss.Style
	MutedStyle       lipgloss.Style
	ErrorStyle       lipgloss.Style
	HighlightedStyle lipgloss.Style
	ButtonStyle      lipgloss.Style
	ButtonFocusStyle lipgloss.Style
	BorderColor      lipgloss.Color
	CheckIcon        string
	CrossIcon        string
}

// NewValueSourcesDialog creates a new value sources manager dialog.
func NewValueSourcesDialog() *ValueSourcesDialog {
	return &ValueSourcesDialog{}
}

// OpenWith opens the dialog with the given value files and set overrides.
// Slices are copied to avoid sharing references with the caller.
func (d *ValueSourcesDialog) OpenWith(valueFiles []string, setValues []string) {
	d.BaseDialog.Open()
	d.valueFiles = make([]string, len(valueFiles))
	copy(d.valueFiles, valueFiles)
	d.setValues = make([]string, len(setValues))
	copy(d.setValues, setValues)
	d.selectedIdx = 0
	d.clampSelection()
}

// AddValueFile adds a file path to the value files list.
func (d *ValueSourcesDialog) AddValueFile(path string) {
	d.valueFiles = append(d.valueFiles, path)
}

// AddSetValue adds a --set override (key=value) to the set values list.
func (d *ValueSourcesDialog) AddSetValue(kv string) {
	d.setValues = append(d.setValues, kv)
}

// Update handles messages for the value sources dialog.
func (d *ValueSourcesDialog) Update(msg tea.Msg) (Dialog, tea.Cmd) {
	if !d.IsOpen() {
		return d, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		return d.handleKey(msg)
	}

	return d, nil
}

func (d *ValueSourcesDialog) handleKey(msg tea.KeyMsg) (Dialog, tea.Cmd) {
	total := d.totalItems()

	switch msg.String() {
	case "j", "down":
		if total > 0 {
			d.selectedIdx = (d.selectedIdx + 1) % total
		}
		return d, nil

	case "k", "up":
		if total > 0 {
			d.selectedIdx = (d.selectedIdx - 1 + total) % total
		}
		return d, nil

	case "d", "x":
		if total == 0 {
			return d, nil
		}
		if d.selectedIdx < len(d.valueFiles) {
			// Delete from value files
			idx := d.selectedIdx
			d.valueFiles = append(d.valueFiles[:idx], d.valueFiles[idx+1:]...)
		} else {
			// Delete from set values
			idx := d.selectedIdx - len(d.valueFiles)
			d.setValues = append(d.setValues[:idx], d.setValues[idx+1:]...)
		}
		d.clampSelection()
		return d, nil

	case "f":
		return d, func() tea.Msg {
			return ValueSourcesAddFileMsg{}
		}

	case "s":
		return d, func() tea.Msg {
			return ValueSourcesAddSetMsg{}
		}

	case "esc", "q":
		files := make([]string, len(d.valueFiles))
		copy(files, d.valueFiles)
		sets := make([]string, len(d.setValues))
		copy(sets, d.setValues)
		d.Close()
		return d, func() tea.Msg {
			return ValueSourcesClosedMsg{
				ValueFiles: files,
				SetValues:  sets,
			}
		}
	}

	return d, nil
}

// View renders the value sources dialog.
func (d *ValueSourcesDialog) View() string {
	if !d.IsOpen() {
		return ""
	}

	dialogWidth := 55

	dialogStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(d.BorderColor).
		Padding(1, 2).
		Width(dialogWidth)

	var content strings.Builder

	// Title
	content.WriteString(d.TitleStyle.Render("Value Sources") + "\n")
	content.WriteString(d.MutedStyle.Render(strings.Repeat("─", dialogWidth-6)) + "\n")

	// Value Files section
	content.WriteString(d.LabelStyle.Render("Value Files:") + "\n")
	if len(d.valueFiles) == 0 {
		content.WriteString("  " + d.MutedStyle.Render("(none)") + "\n")
	} else {
		for i, f := range d.valueFiles {
			prefix := "  "
			if d.selectedIdx == i {
				prefix = d.HighlightedStyle.Render("→ ")
			}
			line := fmt.Sprintf("%d. %s", i+1, f)
			if d.selectedIdx == i {
				content.WriteString(prefix + d.HighlightedStyle.Render(line) + "\n")
			} else {
				content.WriteString(prefix + line + "\n")
			}
		}
	}

	content.WriteString("\n")

	// --set Overrides section
	content.WriteString(d.LabelStyle.Render("--set Overrides:") + "\n")
	if len(d.setValues) == 0 {
		content.WriteString("  " + d.MutedStyle.Render("(none)") + "\n")
	} else {
		for i, sv := range d.setValues {
			globalIdx := len(d.valueFiles) + i
			prefix := "  "
			if d.selectedIdx == globalIdx {
				prefix = d.HighlightedStyle.Render("→ ")
			}
			line := fmt.Sprintf("%d. %s", i+1, sv)
			if d.selectedIdx == globalIdx {
				content.WriteString(prefix + d.HighlightedStyle.Render(line) + "\n")
			} else {
				content.WriteString(prefix + line + "\n")
			}
		}
	}

	// Footer
	content.WriteString("\n" + d.MutedStyle.Render("f:add file  s:add --set  d:delete  Esc:close"))

	dialog := dialogStyle.Render(content.String())
	return lipgloss.Place(d.Width, d.Height, lipgloss.Center, lipgloss.Center, dialog)
}

// SetStyles configures the dialog styles from the parent view.
func (d *ValueSourcesDialog) SetStyles(
	title, label, muted, errStyle, highlighted, button, buttonFocus lipgloss.Style,
	borderColor lipgloss.Color,
	checkIcon, crossIcon string,
) {
	d.TitleStyle = title
	d.LabelStyle = label
	d.MutedStyle = muted
	d.ErrorStyle = errStyle
	d.HighlightedStyle = highlighted
	d.ButtonStyle = button
	d.ButtonFocusStyle = buttonFocus
	d.BorderColor = borderColor
	d.CheckIcon = checkIcon
	d.CrossIcon = crossIcon
}

func (d *ValueSourcesDialog) totalItems() int {
	return len(d.valueFiles) + len(d.setValues)
}

func (d *ValueSourcesDialog) clampSelection() {
	total := d.totalItems()
	if total == 0 {
		d.selectedIdx = 0
		return
	}
	if d.selectedIdx >= total {
		d.selectedIdx = total - 1
	}
}

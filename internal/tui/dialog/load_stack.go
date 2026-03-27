package dialog

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// LoadStackItem represents one selectable stack in the picker.
type LoadStackItem struct {
	Name        string
	Description string
	ChartCount  int
}

// LoadStackResultMsg is sent when the user selects a stack to load.
type LoadStackResultMsg struct {
	Name string
}

// LoadStackDialog shows saved stacks and lets the user pick one.
type LoadStackDialog struct {
	BaseDialog

	stacks   []LoadStackItem
	cursor   int
	lastErr  string

	// Styles
	TitleStyle       lipgloss.Style
	LabelStyle       lipgloss.Style
	MutedStyle       lipgloss.Style
	SelectedStyle    lipgloss.Style
	ErrorStyle       lipgloss.Style
	BorderColor      lipgloss.Color
	ChartIcon        string
	ArrowIcon        string
}

// NewLoadStackDialog creates a new load stack dialog.
func NewLoadStackDialog() *LoadStackDialog {
	return &LoadStackDialog{}
}

// OpenWithStacks opens the dialog populated with the given stacks.
func (d *LoadStackDialog) OpenWithStacks(stacks []LoadStackItem) {
	d.stacks = stacks
	d.cursor = 0
	d.lastErr = ""
	d.BaseDialog.Open()
}

// Update handles messages.
func (d *LoadStackDialog) Update(msg tea.Msg) (Dialog, tea.Cmd) {
	if !d.IsOpen() {
		return d, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		return d.handleKey(msg)
	}

	return d, nil
}

func (d *LoadStackDialog) handleKey(msg tea.KeyMsg) (Dialog, tea.Cmd) {
	if len(d.stacks) == 0 {
		if msg.String() == "esc" || msg.String() == "q" {
			d.Close()
		}
		return d, nil
	}

	switch msg.String() {
	case "esc", "q":
		d.Close()
		return d, nil

	case "up", "k":
		if d.cursor > 0 {
			d.cursor--
		}

	case "down", "j":
		if d.cursor < len(d.stacks)-1 {
			d.cursor++
		}

	case "enter":
		selected := d.stacks[d.cursor]
		d.Close()
		return d, func() tea.Msg {
			return LoadStackResultMsg{Name: selected.Name}
		}
	}

	return d, nil
}

// View renders the dialog.
func (d *LoadStackDialog) View() string {
	if !d.IsOpen() {
		return ""
	}

	dialogWidth := 56

	dialogStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(d.BorderColor).
		Padding(1, 2).
		Width(dialogWidth)

	chartIcon := d.ChartIcon
	if chartIcon == "" {
		chartIcon = "⎈"
	}
	arrowIcon := d.ArrowIcon
	if arrowIcon == "" {
		arrowIcon = "→"
	}

	var content strings.Builder
	content.WriteString(d.TitleStyle.Render(chartIcon+" Load Stack") + "\n\n")

	if len(d.stacks) == 0 {
		content.WriteString(d.MutedStyle.Render("No saved stacks found.") + "\n\n")
		content.WriteString(d.MutedStyle.Render("Esc:close"))
	} else {
		for i, s := range d.stacks {
			charts := fmt.Sprintf("%d chart", s.ChartCount)
			if s.ChartCount != 1 {
				charts += "s"
			}
			if i == d.cursor {
				line := fmt.Sprintf("%s %-30s %s", arrowIcon, s.Name, charts)
				content.WriteString(d.SelectedStyle.Render(line))
				if s.Description != "" {
					content.WriteString("\n")
					content.WriteString(d.MutedStyle.Render("  " + s.Description))
				}
			} else {
				line := fmt.Sprintf("  %-30s %s", s.Name, charts)
				content.WriteString(d.LabelStyle.Render(line))
				if s.Description != "" {
					content.WriteString("\n")
					content.WriteString(d.MutedStyle.Render("  " + s.Description))
				}
			}
			content.WriteString("\n")
		}
		if d.lastErr != "" {
			content.WriteString("\n" + d.ErrorStyle.Render(d.lastErr) + "\n")
		}
		content.WriteString("\n" + d.MutedStyle.Render("↑↓/jk:navigate  Enter:load  Esc:cancel"))
	}

	dialog := dialogStyle.Render(content.String())
	return lipgloss.Place(d.Width, d.Height, lipgloss.Center, lipgloss.Center, dialog)
}

// SetStyles configures dialog styles from the parent view.
func (d *LoadStackDialog) SetStyles(title, label, muted, selected, errStyle lipgloss.Style, borderColor lipgloss.Color, chartIcon, arrowIcon string) {
	d.TitleStyle = title
	d.LabelStyle = label
	d.MutedStyle = muted
	d.SelectedStyle = selected
	d.ErrorStyle = errStyle
	d.BorderColor = borderColor
	d.ChartIcon = chartIcon
	d.ArrowIcon = arrowIcon
}

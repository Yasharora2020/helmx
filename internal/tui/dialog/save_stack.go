package dialog

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// SaveStackField represents which field is focused in the save stack dialog.
type SaveStackField int

const (
	SaveStackFieldName SaveStackField = iota
	SaveStackFieldDesc
	SaveStackFieldSave
	SaveStackFieldCancel
)

// SaveStackResultMsg is sent when the user confirms saving a stack.
type SaveStackResultMsg struct {
	Name        string
	Description string
}

// SaveStackDialog allows saving the multi-chart queue as a named stack.
type SaveStackDialog struct {
	BaseDialog

	// Queue info (for display)
	chartCount int

	// Input fields
	nameInput textinput.Model
	descInput textinput.Model
	field     SaveStackField
	lastError string

	// Styles (injected from parent)
	TitleStyle       lipgloss.Style
	LabelStyle       lipgloss.Style
	MutedStyle       lipgloss.Style
	HighlightedStyle lipgloss.Style
	ErrorStyle       lipgloss.Style
	ButtonStyle      lipgloss.Style
	ButtonFocusStyle lipgloss.Style
	BorderColor      lipgloss.Color
	AddIcon          string
	ArrowIcon        string
	CrossIcon        string
}

// NewSaveStackDialog creates a new save stack dialog.
func NewSaveStackDialog() *SaveStackDialog {
	nameInput := textinput.New()
	nameInput.Placeholder = "web-stack"
	nameInput.CharLimit = 63
	nameInput.Width = 30

	descInput := textinput.New()
	descInput.Placeholder = "Optional description"
	descInput.CharLimit = 256
	descInput.Width = 30

	return &SaveStackDialog{
		nameInput: nameInput,
		descInput: descInput,
	}
}

// Open opens the dialog and resets state.
func (d *SaveStackDialog) Open() {
	d.BaseDialog.Open()
	d.field = SaveStackFieldName
	d.lastError = ""
	d.nameInput.Reset()
	d.descInput.Reset()
	d.nameInput.Focus()
}

// OpenForQueue opens the dialog for a queue of the given size.
func (d *SaveStackDialog) OpenForQueue(chartCount int) {
	d.chartCount = chartCount
	d.Open()
}

// SetError sets an error message to display.
func (d *SaveStackDialog) SetError(err string) {
	d.lastError = err
}

// Update handles messages for the save stack dialog.
func (d *SaveStackDialog) Update(msg tea.Msg) (Dialog, tea.Cmd) {
	if !d.IsOpen() {
		return d, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		return d.handleKey(msg)
	}

	return d, nil
}

func (d *SaveStackDialog) handleKey(msg tea.KeyMsg) (Dialog, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg.String() {
	case "esc":
		d.Close()
		d.lastError = ""
		return d, nil

	case "tab":
		d.field = (d.field + 1) % 4
		d.updateFocus()
		return d, nil

	case "shift+tab":
		d.field = (d.field + 3) % 4
		d.updateFocus()
		return d, nil

	case "enter":
		switch d.field {
		case SaveStackFieldName, SaveStackFieldDesc:
			d.field = (d.field + 1) % 4
			d.updateFocus()
			return d, nil
		case SaveStackFieldSave:
			name := strings.TrimSpace(d.nameInput.Value())
			if name == "" {
				d.lastError = "Stack name is required"
				return d, nil
			}
			d.Close()
			return d, func() tea.Msg {
				return SaveStackResultMsg{
					Name:        name,
					Description: strings.TrimSpace(d.descInput.Value()),
				}
			}
		case SaveStackFieldCancel:
			d.Close()
			d.lastError = ""
			return d, nil
		}
	}

	// Pass keys to text inputs
	var cmd tea.Cmd
	switch d.field {
	case SaveStackFieldName:
		d.nameInput, cmd = d.nameInput.Update(msg)
		cmds = append(cmds, cmd)
	case SaveStackFieldDesc:
		d.descInput, cmd = d.descInput.Update(msg)
		cmds = append(cmds, cmd)
	}

	return d, tea.Batch(cmds...)
}

func (d *SaveStackDialog) updateFocus() {
	d.nameInput.Blur()
	d.descInput.Blur()

	switch d.field {
	case SaveStackFieldName:
		d.nameInput.Focus()
	case SaveStackFieldDesc:
		d.descInput.Focus()
	}
}

// View renders the save stack dialog.
func (d *SaveStackDialog) View() string {
	if !d.IsOpen() {
		return ""
	}

	dialogWidth := 50

	dialogStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(d.BorderColor).
		Padding(1, 2).
		Width(dialogWidth)

	var content strings.Builder

	addIcon := d.AddIcon
	if addIcon == "" {
		addIcon = "+"
	}
	arrowIcon := d.ArrowIcon
	if arrowIcon == "" {
		arrowIcon = "→"
	}
	crossIcon := d.CrossIcon
	if crossIcon == "" {
		crossIcon = "✗"
	}

	content.WriteString(d.TitleStyle.Render(addIcon+" Save Stack") + "\n\n")
	content.WriteString(d.MutedStyle.Render(fmt.Sprintf("  %d charts in queue", d.chartCount)) + "\n\n")

	if d.field == SaveStackFieldName {
		content.WriteString(d.HighlightedStyle.Render(arrowIcon+" Name:") + "\n")
	} else {
		content.WriteString(d.LabelStyle.Render("  Name:") + "\n")
	}
	content.WriteString("  " + d.nameInput.View() + "\n\n")

	if d.field == SaveStackFieldDesc {
		content.WriteString(d.HighlightedStyle.Render(arrowIcon+" Description:") + "\n")
	} else {
		content.WriteString(d.LabelStyle.Render("  Description:") + "\n")
	}
	content.WriteString("  " + d.descInput.View() + "\n\n")

	if d.lastError != "" {
		content.WriteString(d.ErrorStyle.Render(crossIcon+" "+d.lastError) + "\n\n")
	}

	saveBtn := d.ButtonStyle.Render(" Save ")
	cancelBtn := d.ButtonStyle.Render(" Cancel ")
	if d.field == SaveStackFieldSave {
		saveBtn = d.ButtonFocusStyle.Render(" Save ")
	}
	if d.field == SaveStackFieldCancel {
		cancelBtn = d.ButtonFocusStyle.Render(" Cancel ")
	}
	content.WriteString(saveBtn + "  " + cancelBtn + "\n")
	content.WriteString("\n" + d.MutedStyle.Render("Tab:navigate  Enter:confirm  Esc:cancel"))

	dialog := dialogStyle.Render(content.String())
	return lipgloss.Place(d.Width, d.Height, lipgloss.Center, lipgloss.Center, dialog)
}

// SetStyles configures the dialog styles from the parent view.
func (d *SaveStackDialog) SetStyles(title, label, muted, highlighted, errStyle, button, buttonFocus lipgloss.Style, borderColor lipgloss.Color, addIcon, arrowIcon, crossIcon string) {
	d.TitleStyle = title
	d.LabelStyle = label
	d.MutedStyle = muted
	d.HighlightedStyle = highlighted
	d.ErrorStyle = errStyle
	d.ButtonStyle = button
	d.ButtonFocusStyle = buttonFocus
	d.BorderColor = borderColor
	d.AddIcon = addIcon
	d.ArrowIcon = arrowIcon
	d.CrossIcon = crossIcon
}

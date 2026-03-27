package dialog

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// SaveTemplateField represents which field is focused in the save template dialog.
type SaveTemplateField int

const (
	SaveTemplateFieldName SaveTemplateField = iota
	SaveTemplateFieldDesc
	SaveTemplateFieldSave
	SaveTemplateFieldCancel
)

// SaveTemplateResultMsg is sent when the template is saved successfully.
type SaveTemplateResultMsg struct {
	Name        string
	ChartName   string
	Description string
	Values      string
}

// SaveTemplateDialog allows saving values as a reusable template.
type SaveTemplateDialog struct {
	BaseDialog

	// Chart info
	chartName     string
	valuesContent string
	valuesLines   int

	// Input fields
	nameInput textinput.Model
	descInput textinput.Model
	field     SaveTemplateField
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

// NewSaveTemplateDialog creates a new save template dialog.
func NewSaveTemplateDialog() *SaveTemplateDialog {
	nameInput := textinput.New()
	nameInput.Placeholder = "my-template"
	nameInput.CharLimit = 63
	nameInput.Width = 30

	descInput := textinput.New()
	descInput.Placeholder = "Optional description"
	descInput.CharLimit = 256
	descInput.Width = 30

	return &SaveTemplateDialog{
		nameInput: nameInput,
		descInput: descInput,
	}
}

// Open opens the dialog and resets state.
func (d *SaveTemplateDialog) Open() {
	d.BaseDialog.Open()
	d.field = SaveTemplateFieldName
	d.lastError = ""
	d.nameInput.Reset()
	d.descInput.Reset()
	d.nameInput.Focus()
}

// OpenForChart opens the dialog for a specific chart.
func (d *SaveTemplateDialog) OpenForChart(chartName, values string) {
	d.chartName = chartName
	d.valuesContent = values
	d.valuesLines = len(strings.Split(values, "\n"))
	d.Open()
}

// SetError sets an error message.
func (d *SaveTemplateDialog) SetError(err string) {
	d.lastError = err
}

// GetValues returns the template values for saving.
func (d *SaveTemplateDialog) GetValues() (name, chartName, description, values string) {
	return strings.TrimSpace(d.nameInput.Value()),
		d.chartName,
		strings.TrimSpace(d.descInput.Value()),
		d.valuesContent
}

// Update handles messages for the save template dialog.
func (d *SaveTemplateDialog) Update(msg tea.Msg) (Dialog, tea.Cmd) {
	if !d.IsOpen() {
		return d, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		return d.handleKey(msg)
	}

	return d, nil
}

func (d *SaveTemplateDialog) handleKey(msg tea.KeyMsg) (Dialog, tea.Cmd) {
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
		case SaveTemplateFieldName, SaveTemplateFieldDesc:
			d.field = (d.field + 1) % 4
			d.updateFocus()
			return d, nil
		case SaveTemplateFieldSave:
			name := strings.TrimSpace(d.nameInput.Value())
			if name == "" {
				d.lastError = "Template name is required"
				return d, nil
			}
			// Signal parent to save the template
			d.Close()
			return d, func() tea.Msg {
				return SaveTemplateResultMsg{
					Name:        name,
					ChartName:   d.chartName,
					Description: strings.TrimSpace(d.descInput.Value()),
					Values:      d.valuesContent,
				}
			}
		case SaveTemplateFieldCancel:
			d.Close()
			d.lastError = ""
			return d, nil
		}
	}

	// Pass keys to text inputs
	var cmd tea.Cmd
	switch d.field {
	case SaveTemplateFieldName:
		d.nameInput, cmd = d.nameInput.Update(msg)
		cmds = append(cmds, cmd)
	case SaveTemplateFieldDesc:
		d.descInput, cmd = d.descInput.Update(msg)
		cmds = append(cmds, cmd)
	}

	return d, tea.Batch(cmds...)
}

func (d *SaveTemplateDialog) updateFocus() {
	d.nameInput.Blur()
	d.descInput.Blur()

	switch d.field {
	case SaveTemplateFieldName:
		d.nameInput.Focus()
	case SaveTemplateFieldDesc:
		d.descInput.Focus()
	}
}

// View renders the save template dialog.
func (d *SaveTemplateDialog) View() string {
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

	// Icons with defaults
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

	// Title
	content.WriteString(d.TitleStyle.Render(addIcon+" Save as Template") + "\n\n")

	if d.chartName != "" {
		content.WriteString(d.LabelStyle.Render("Chart: ") + d.chartName + "\n\n")
	}

	// Name field
	if d.field == SaveTemplateFieldName {
		content.WriteString(d.HighlightedStyle.Render(arrowIcon+" Name:") + "\n")
	} else {
		content.WriteString(d.LabelStyle.Render("  Name:") + "\n")
	}
	content.WriteString("  " + d.nameInput.View() + "\n\n")

	// Description field
	if d.field == SaveTemplateFieldDesc {
		content.WriteString(d.HighlightedStyle.Render(arrowIcon+" Description:") + "\n")
	} else {
		content.WriteString(d.LabelStyle.Render("  Description:") + "\n")
	}
	content.WriteString("  " + d.descInput.View() + "\n\n")

	// Values info
	content.WriteString(d.MutedStyle.Render(fmt.Sprintf("  Values: %d lines will be saved", d.valuesLines)) + "\n\n")

	// Error message
	if d.lastError != "" {
		content.WriteString(d.ErrorStyle.Render(crossIcon+" "+d.lastError) + "\n\n")
	}

	// Buttons
	saveBtn := d.ButtonStyle.Render(" Save ")
	cancelBtn := d.ButtonStyle.Render(" Cancel ")
	if d.field == SaveTemplateFieldSave {
		saveBtn = d.ButtonFocusStyle.Render(" Save ")
	}
	if d.field == SaveTemplateFieldCancel {
		cancelBtn = d.ButtonFocusStyle.Render(" Cancel ")
	}
	content.WriteString(saveBtn + "  " + cancelBtn + "\n")

	// Footer
	content.WriteString("\n" + d.MutedStyle.Render("Tab:navigate  Enter:confirm  Esc:cancel"))

	dialog := dialogStyle.Render(content.String())
	return lipgloss.Place(d.Width, d.Height, lipgloss.Center, lipgloss.Center, dialog)
}

// SetStyles configures the dialog styles from the parent view.
func (d *SaveTemplateDialog) SetStyles(title, label, muted, highlighted, errStyle, button, buttonFocus lipgloss.Style, borderColor lipgloss.Color, addIcon, arrowIcon, crossIcon string) {
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

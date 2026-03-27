package dialog

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// TemplateField represents which field is focused in the template dialog
type TemplateField int

const (
	TemplateFieldOutputPath TemplateField = iota
	TemplateFieldValues
	TemplateFieldConfirm
	TemplateFieldCancel
)

// TemplateEditValuesMsg is sent when the user wants to edit values externally.
type TemplateEditValuesMsg struct{}

// TemplateImportFileMsg is sent when the user wants to import a value file.
type TemplateImportFileMsg struct{}

// TemplateAddSetValueMsg is sent when the user wants to add a --set override.
type TemplateAddSetValueMsg struct{}

// TemplateManageSourcesMsg is sent when the user wants to manage value files/overrides.
type TemplateManageSourcesMsg struct{}

// TemplateExecuteMsg is sent when the user confirms template execution.
type TemplateExecuteMsg struct {
	OutputPath string
	Values     string
	ValueFiles []string
	SetValues  []string
}

// TemplateDialog handles rendering chart templates to files.
type TemplateDialog struct {
	BaseDialog
	outputPathInput textinput.Model
	field           TemplateField
	values          string
	valuesErr       error
	templating      bool

	// Result from template execution
	resultPath string
	resultErr  error

	// Value sources
	valueFiles []string
	setValues  []string

	// Chart info
	chartName    string
	chartVersion string

	// Spinner (interface to avoid import cycles)
	spinnerView func() string

	// Styles (injected from parent)
	TitleStyle       lipgloss.Style
	LabelStyle       lipgloss.Style
	MutedStyle       lipgloss.Style
	ValueStyle       lipgloss.Style
	ErrorStyle       lipgloss.Style
	SuccessStyle     lipgloss.Style
	HighlightedStyle lipgloss.Style
	ButtonStyle      lipgloss.Style
	ButtonFocusStyle lipgloss.Style
	BorderColor      lipgloss.Color
	ChartIcon        string
	ArrowIcon        string
	CheckIcon        string
	CrossIcon        string
}

// NewTemplateDialog creates a new template dialog.
func NewTemplateDialog() *TemplateDialog {
	outputInput := textinput.New()
	outputInput.Placeholder = "./chart-rendered.yaml"
	outputInput.CharLimit = 256
	outputInput.Width = 45

	return &TemplateDialog{
		outputPathInput: outputInput,
	}
}

// Open opens the dialog with default settings.
func (d *TemplateDialog) Open() {
	d.BaseDialog.Open()
	d.field = TemplateFieldOutputPath
	d.templating = false
	d.resultPath = ""
	d.resultErr = nil
	d.valuesErr = nil
	d.valueFiles = nil
	d.setValues = nil
	d.outputPathInput.Focus()
}

// OpenWithChart opens the dialog for a specific chart.
func (d *TemplateDialog) OpenWithChart(chartName, chartVersion, defaultValues string) {
	d.chartName = chartName
	d.chartVersion = chartVersion
	d.values = defaultValues
	if d.values == "" {
		d.values = "# Custom values (YAML)\n"
	}
	d.BaseDialog.Open()
	d.field = TemplateFieldOutputPath
	d.templating = false
	d.resultPath = ""
	d.resultErr = nil
	d.valuesErr = nil
	d.valueFiles = nil
	d.setValues = nil
	d.outputPathInput.SetValue("./" + chartName + "-rendered.yaml")
	d.outputPathInput.Focus()
}

// SetTemplating sets the templating state.
func (d *TemplateDialog) SetTemplating(t bool) {
	d.templating = t
}

// SetResult sets the template execution result.
func (d *TemplateDialog) SetResult(path string, err error) {
	d.resultPath = path
	d.resultErr = err
	d.templating = false
}

// SetValues updates the values content.
func (d *TemplateDialog) SetValues(values string) {
	d.values = values
}

// GetValues returns the current values content.
func (d *TemplateDialog) GetValues() string {
	return d.values
}

// GetValueFiles returns the accumulated value file paths.
func (d *TemplateDialog) GetValueFiles() []string { return d.valueFiles }

// GetSetValues returns the accumulated --set overrides.
func (d *TemplateDialog) GetSetValues() []string { return d.setValues }

// AddValueFile adds a value file path.
func (d *TemplateDialog) AddValueFile(path string) { d.valueFiles = append(d.valueFiles, path) }

// AddSetValue adds a --set override.
func (d *TemplateDialog) AddSetValue(kv string) { d.setValues = append(d.setValues, kv) }

// SetValueFiles replaces the value files list.
func (d *TemplateDialog) SetValueFiles(files []string) { d.valueFiles = files }

// SetSetValues replaces the --set overrides list.
func (d *TemplateDialog) SetSetValues(values []string) { d.setValues = values }

// SetValuesError sets the values validation error.
func (d *TemplateDialog) SetValuesError(err error) {
	d.valuesErr = err
}

// SetSpinner sets the spinner view function.
func (d *TemplateDialog) SetSpinner(view func() string) {
	d.spinnerView = view
}

// Update handles messages for the template dialog.
func (d *TemplateDialog) Update(msg tea.Msg) (Dialog, tea.Cmd) {
	if !d.IsOpen() {
		return d, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		return d.handleKey(msg)
	}

	// Update text input
	var cmd tea.Cmd
	if d.field == TemplateFieldOutputPath {
		d.outputPathInput, cmd = d.outputPathInput.Update(msg)
		return d, cmd
	}

	return d, nil
}

func (d *TemplateDialog) handleKey(msg tea.KeyMsg) (Dialog, tea.Cmd) {
	switch msg.String() {
	case "esc":
		d.Close()
		d.resultPath = ""
		d.resultErr = nil
		return d, nil

	case "tab":
		// Cycle through fields (skip TemplateFieldValues since it's handled by 'e' key)
		nextField := (d.field + 1) % 4
		if nextField == TemplateFieldValues {
			nextField = TemplateFieldConfirm
		}
		d.field = nextField
		d.updateFocus()
		return d, nil

	case "shift+tab":
		// Cycle backwards
		prevField := (d.field + 3) % 4
		if prevField == TemplateFieldValues {
			prevField = TemplateFieldOutputPath
		}
		d.field = prevField
		d.updateFocus()
		return d, nil

	case "e":
		// Request external editor for values
		return d, func() tea.Msg {
			return TemplateEditValuesMsg{}
		}

	case "f":
		return d, func() tea.Msg { return TemplateImportFileMsg{} }

	case "s":
		return d, func() tea.Msg { return TemplateAddSetValueMsg{} }

	case "F":
		return d, func() tea.Msg { return TemplateManageSourcesMsg{} }

	case "enter":
		switch d.field {
		case TemplateFieldConfirm:
			if !d.templating {
				// Validate values before templating
				if d.valuesErr != nil {
					return d, nil
				}
				d.templating = true
				return d, func() tea.Msg {
					return TemplateExecuteMsg{
						OutputPath: d.outputPathInput.Value(),
						Values:     d.values,
						ValueFiles: d.valueFiles,
						SetValues:  d.setValues,
					}
				}
			}
		case TemplateFieldCancel:
			d.Close()
			d.resultPath = ""
			d.resultErr = nil
			return d, nil
		}
	}

	// Update the focused component
	var cmd tea.Cmd
	if d.field == TemplateFieldOutputPath {
		d.outputPathInput, cmd = d.outputPathInput.Update(msg)
		return d, cmd
	}

	return d, nil
}

func (d *TemplateDialog) updateFocus() {
	d.outputPathInput.Blur()
	if d.field == TemplateFieldOutputPath {
		d.outputPathInput.Focus()
	}
}

// View renders the template dialog.
func (d *TemplateDialog) View() string {
	if !d.IsOpen() {
		return ""
	}

	dialogWidth := 60

	dialogStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(d.BorderColor).
		Padding(1, 2).
		Width(dialogWidth)

	var content strings.Builder

	icon := d.ChartIcon
	if icon == "" {
		icon = "📦"
	}
	content.WriteString(d.TitleStyle.Render(icon+" Template to File") + "\n\n")

	// Chart name
	if d.chartName != "" {
		content.WriteString(d.LabelStyle.Render("Chart: "))
		content.WriteString(d.ValueStyle.Render(d.chartName+" v"+d.chartVersion) + "\n\n")
	}

	// Output path field
	arrow := d.ArrowIcon
	if arrow == "" {
		arrow = "→"
	}
	label := "  Output Path:"
	if d.field == TemplateFieldOutputPath {
		label = d.HighlightedStyle.Render(arrow + " Output Path:")
	} else {
		label = d.LabelStyle.Render(label)
	}
	content.WriteString(label + "\n  " + d.outputPathInput.View() + "\n\n")

	// Values section (edit with 'e')
	valuesLabel := d.LabelStyle.Render("  Values:")
	if d.valuesErr != nil {
		valuesLabel += " " + d.ErrorStyle.Render("(invalid YAML)")
	}
	content.WriteString(valuesLabel + "\n")
	content.WriteString(d.MutedStyle.Render("  Press 'e' to edit values in external editor") + "\n")

	// Show value sources count
	if len(d.valueFiles) > 0 || len(d.setValues) > 0 {
		var sources []string
		if len(d.valueFiles) > 0 {
			sources = append(sources, fmt.Sprintf("%d value file(s)", len(d.valueFiles)))
		}
		if len(d.setValues) > 0 {
			sources = append(sources, fmt.Sprintf("%d --set override(s)", len(d.setValues)))
		}
		content.WriteString(d.MutedStyle.Render("  "+strings.Join(sources, ", ")+" (F to manage)") + "\n")
	}
	content.WriteString("\n")

	// Buttons
	renderBtn := d.ButtonStyle.Render("Render")
	cancelBtn := d.ButtonStyle.Render("Cancel")
	if d.field == TemplateFieldConfirm {
		renderBtn = d.ButtonFocusStyle.Render("Render")
	}
	if d.field == TemplateFieldCancel {
		cancelBtn = d.ButtonFocusStyle.Render("Cancel")
	}

	// Show spinner or buttons
	if d.templating {
		if d.spinnerView != nil {
			content.WriteString("  " + d.spinnerView() + "\n\n")
		} else {
			content.WriteString("  Rendering...\n\n")
		}
	} else {
		content.WriteString("  " + renderBtn + "  " + cancelBtn + "\n\n")
	}

	// Show result
	if d.resultErr != nil {
		cross := d.CrossIcon
		if cross == "" {
			cross = "✗"
		}
		content.WriteString(d.ErrorStyle.Render(cross+" Error: "+d.resultErr.Error()) + "\n")
	} else if d.resultPath != "" {
		check := d.CheckIcon
		if check == "" {
			check = "✓"
		}
		content.WriteString(d.SuccessStyle.Render(check+" Saved to: "+d.resultPath) + "\n")
	}

	content.WriteString(d.MutedStyle.Render("Tab:next  e:edit  f:file  s:set  F:sources  Esc:cancel"))

	dialog := dialogStyle.Render(content.String())
	return lipgloss.Place(d.Width, d.Height, lipgloss.Center, lipgloss.Center, dialog)
}

// SetStyles configures the dialog styles from the parent view.
func (d *TemplateDialog) SetStyles(title, label, muted, value, errStyle, success, highlighted, button, buttonFocus lipgloss.Style, borderColor lipgloss.Color, chartIcon, arrowIcon, checkIcon, crossIcon string) {
	d.TitleStyle = title
	d.LabelStyle = label
	d.MutedStyle = muted
	d.ValueStyle = value
	d.ErrorStyle = errStyle
	d.SuccessStyle = success
	d.HighlightedStyle = highlighted
	d.ButtonStyle = button
	d.ButtonFocusStyle = buttonFocus
	d.BorderColor = borderColor
	d.ChartIcon = chartIcon
	d.ArrowIcon = arrowIcon
	d.CheckIcon = checkIcon
	d.CrossIcon = crossIcon
}

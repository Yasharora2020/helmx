package dialog

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// InstallField represents which field is focused in the install dialog.
type InstallField int

const (
	InstallFieldTemplate InstallField = iota
	InstallFieldReleaseName
	InstallFieldNamespace
	InstallFieldCreateNS
	InstallFieldValues // Handled via external editor
	InstallFieldConfirm
	InstallFieldCancel
)

// InstallEditValuesMsg is sent when the user wants to edit values.
type InstallEditValuesMsg struct{}

// InstallImportFileMsg is sent when the user wants to import values from file.
type InstallImportFileMsg struct{}

// InstallDryRunMsg is sent when the user wants a dry-run preview.
type InstallDryRunMsg struct {
	ReleaseName string
	Namespace   string
	Values      string
	ValueFiles  []string
	SetValues   []string
}

// InstallDecryptMsg is sent when the user wants to decrypt values.
type InstallDecryptMsg struct{}

// InstallSaveTemplateMsg is sent when the user wants to save values as template.
type InstallSaveTemplateMsg struct {
	Values string
}

// InstallAddSetValueMsg is sent when the user wants to add a --set override.
type InstallAddSetValueMsg struct{}

// InstallManageSourcesMsg is sent when the user wants to manage value files/overrides.
type InstallManageSourcesMsg struct{}

// InstallExecuteMsg is sent when the user confirms the install.
type InstallExecuteMsg struct {
	ReleaseName     string
	Namespace       string
	Values          string
	CreateNamespace bool
	ValueFiles      []string
	SetValues       []string
}

// InstallOpenReadmeMsg is sent when the user wants to view the README.
type InstallOpenReadmeMsg struct{}

// InstallTemplate represents a saved values template.
type InstallTemplate struct {
	Name           string
	Description    string
	Values         string // Merged: chart defaults + overrides
	VersionWarning string // Non-empty if chart version changed since template was saved
}

// InstallResult holds the result of an installation.
type InstallResult struct {
	Success     bool
	ReleaseName string
	Status      string
	Error       error
}

// InstallDialog handles chart installation.
type InstallDialog struct {
	BaseDialog
	releaseNameInput textinput.Model
	namespaceInput   textinput.Model
	field            InstallField

	// Chart info
	chartName    string
	chartVersion string
	chartRef     string

	// Values
	valuesContent   string
	valuesErr       error
	valuesEncrypted bool
	decryptionError string
	encryptOnSave   bool

	// Templates
	availableTemplates  []InstallTemplate
	selectedTemplateIdx int

	// Namespace option
	createNamespace bool

	// Value sources
	valueFiles []string // accumulated -f file paths
	setValues  []string // accumulated --set key=value pairs

	// Install state
	installing bool
	result     *InstallResult

	// Spinner (interface to avoid import cycles)
	spinnerView func() string

	// Styles (injected from parent)
	TitleStyle       lipgloss.Style
	LabelStyle       lipgloss.Style
	MutedStyle       lipgloss.Style
	ValueStyle       lipgloss.Style
	ErrorStyle       lipgloss.Style
	SuccessStyle     lipgloss.Style
	WarningStyle     lipgloss.Style
	HighlightedStyle lipgloss.Style
	SelectedStyle    lipgloss.Style
	ButtonStyle      lipgloss.Style
	ButtonFocusStyle lipgloss.Style
	BorderColor      lipgloss.Color
	AddIcon          string
	ArrowIcon        string
	CheckIcon        string
	CrossIcon        string
	LockIcon         string
}

// NewInstallDialog creates a new install dialog.
func NewInstallDialog() *InstallDialog {
	releaseInput := textinput.New()
	releaseInput.Placeholder = "my-release"
	releaseInput.CharLimit = 63 // K8s name limit
	releaseInput.Width = 30

	nsInput := textinput.New()
	nsInput.Placeholder = "default"
	nsInput.CharLimit = 63
	nsInput.Width = 30

	return &InstallDialog{
		releaseNameInput:    releaseInput,
		namespaceInput:      nsInput,
		selectedTemplateIdx: -1,
		createNamespace:     false,
	}
}

// Open opens the dialog with default settings.
func (d *InstallDialog) Open() {
	d.BaseDialog.Open()
	d.installing = false
	d.result = nil
	d.valuesErr = nil
	d.decryptionError = ""
	d.selectedTemplateIdx = -1
	d.valueFiles = nil
	d.setValues = nil

	if len(d.availableTemplates) > 0 {
		d.field = InstallFieldTemplate
	} else {
		d.field = InstallFieldReleaseName
	}
	d.updateFocus()
}

// OpenForChart opens the dialog for a specific chart.
func (d *InstallDialog) OpenForChart(chartName, chartVersion, chartRef, defaultValues string, templates []InstallTemplate, isEncrypted bool, decryptedValues string) {
	d.chartName = chartName
	d.chartVersion = chartVersion
	d.chartRef = chartRef
	d.availableTemplates = templates
	d.selectedTemplateIdx = -1

	d.BaseDialog.Open()
	d.installing = false
	d.result = nil
	d.valuesErr = nil
	d.valueFiles = nil
	d.setValues = nil

	// Handle values
	d.valuesEncrypted = isEncrypted
	if isEncrypted {
		if decryptedValues != "" {
			d.valuesContent = decryptedValues
			d.encryptOnSave = true
			d.decryptionError = ""
		} else {
			d.valuesContent = defaultValues
			d.decryptionError = "Values are encrypted. Press 'D' to decrypt (requires helm-secrets plugin)."
		}
	} else {
		if defaultValues != "" {
			d.valuesContent = defaultValues
		} else {
			d.valuesContent = "# Custom values (YAML)\n"
		}
		d.decryptionError = ""
		d.encryptOnSave = false
	}

	d.releaseNameInput.SetValue(chartName)
	d.namespaceInput.SetValue("default")
	d.createNamespace = true

	if len(templates) > 0 {
		d.field = InstallFieldTemplate
	} else {
		d.field = InstallFieldReleaseName
	}
	d.updateFocus()
}

// SetValues sets the values content.
func (d *InstallDialog) SetValues(values string) {
	d.valuesContent = values
}

// SetValuesError sets a values validation error.
func (d *InstallDialog) SetValuesError(err error) {
	d.valuesErr = err
}

// SetDecryptedValues sets decrypted values.
func (d *InstallDialog) SetDecryptedValues(values string) {
	d.valuesContent = values
	d.decryptionError = ""
	d.encryptOnSave = true
}

// SetDecryptionError sets a decryption error.
func (d *InstallDialog) SetDecryptionError(err string) {
	d.decryptionError = err
}

// SetInstalling sets the installing state.
func (d *InstallDialog) SetInstalling(installing bool) {
	d.installing = installing
}

// SetResult sets the installation result.
func (d *InstallDialog) SetResult(result *InstallResult) {
	d.result = result
	d.installing = false
}

// SetSpinner sets the spinner view function.
func (d *InstallDialog) SetSpinner(view func() string) {
	d.spinnerView = view
}

// GetValues returns the current values content.
func (d *InstallDialog) GetValues() string {
	return d.valuesContent
}

// GetReleaseName returns the release name.
func (d *InstallDialog) GetReleaseName() string {
	return d.releaseNameInput.Value()
}

// GetNamespace returns the namespace.
func (d *InstallDialog) GetNamespace() string {
	ns := d.namespaceInput.Value()
	if ns == "" {
		return "default"
	}
	return ns
}

// IsEncrypted returns whether values are encrypted.
func (d *InstallDialog) IsEncrypted() bool {
	return d.valuesEncrypted
}

// ShouldEncryptOnSave returns whether to encrypt values on save.
func (d *InstallDialog) ShouldEncryptOnSave() bool {
	return d.encryptOnSave
}

// GetValueFiles returns the accumulated value file paths.
func (d *InstallDialog) GetValueFiles() []string { return d.valueFiles }

// GetSetValues returns the accumulated --set key=value pairs.
func (d *InstallDialog) GetSetValues() []string { return d.setValues }

// AddValueFile appends a file path to the value files list.
func (d *InstallDialog) AddValueFile(path string) { d.valueFiles = append(d.valueFiles, path) }

// AddSetValue appends a --set key=value pair to the set values list.
func (d *InstallDialog) AddSetValue(kv string) { d.setValues = append(d.setValues, kv) }

// SetValueFiles replaces the value files list.
func (d *InstallDialog) SetValueFiles(files []string) { d.valueFiles = files }

// SetSetValues replaces the --set values list.
func (d *InstallDialog) SetSetValues(values []string) { d.setValues = values }

// Update handles messages for the install dialog.
func (d *InstallDialog) Update(msg tea.Msg) (Dialog, tea.Cmd) {
	if !d.IsOpen() {
		return d, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		return d.handleKey(msg)
	}

	// Update text inputs
	var cmd tea.Cmd
	switch d.field {
	case InstallFieldReleaseName:
		d.releaseNameInput, cmd = d.releaseNameInput.Update(msg)
		return d, cmd
	case InstallFieldNamespace:
		d.namespaceInput, cmd = d.namespaceInput.Update(msg)
		return d, cmd
	}

	return d, nil
}

func (d *InstallDialog) handleKey(msg tea.KeyMsg) (Dialog, tea.Cmd) {
	inTextInput := d.field == InstallFieldReleaseName || d.field == InstallFieldNamespace

	switch msg.String() {
	case "esc":
		d.Close()
		d.result = nil
		return d, nil

	case "tab":
		d.cycleFieldForward()
		d.updateFocus()
		return d, nil

	case "shift+tab":
		d.cycleFieldBackward()
		d.updateFocus()
		return d, nil

	case "e":
		if !inTextInput {
			return d, func() tea.Msg { return InstallEditValuesMsg{} }
		}

	case "r":
		if !inTextInput {
			return d, func() tea.Msg { return InstallOpenReadmeMsg{} }
		}

	case "d":
		if !inTextInput {
			return d, func() tea.Msg {
				return InstallDryRunMsg{
					ReleaseName: d.releaseNameInput.Value(),
					Namespace:   d.GetNamespace(),
					Values:      d.valuesContent,
					ValueFiles:  d.valueFiles,
					SetValues:   d.setValues,
				}
			}
		}

	case "D":
		if !inTextInput && d.valuesEncrypted && d.decryptionError != "" {
			return d, func() tea.Msg { return InstallDecryptMsg{} }
		}

	case "f":
		if !inTextInput {
			return d, func() tea.Msg { return InstallImportFileMsg{} }
		}

	case "s":
		if !inTextInput {
			return d, func() tea.Msg { return InstallAddSetValueMsg{} }
		}

	case "F":
		if !inTextInput {
			return d, func() tea.Msg { return InstallManageSourcesMsg{} }
		}

	case "j", "down":
		if d.field == InstallFieldTemplate && len(d.availableTemplates) > 0 {
			d.selectedTemplateIdx++
			if d.selectedTemplateIdx >= len(d.availableTemplates) {
				d.selectedTemplateIdx = -1
			}
			d.applySelectedTemplate()
			return d, nil
		}

	case "k", "up":
		if d.field == InstallFieldTemplate && len(d.availableTemplates) > 0 {
			d.selectedTemplateIdx--
			if d.selectedTemplateIdx < -1 {
				d.selectedTemplateIdx = len(d.availableTemplates) - 1
			}
			d.applySelectedTemplate()
			return d, nil
		}

	case "T":
		if !inTextInput && d.valuesErr == nil {
			return d, func() tea.Msg {
				return InstallSaveTemplateMsg{Values: d.valuesContent}
			}
		}

	case "enter":
		switch d.field {
		case InstallFieldCreateNS:
			d.createNamespace = !d.createNamespace
		case InstallFieldConfirm:
			if !d.installing && d.valuesErr == nil {
				d.installing = true
				return d, func() tea.Msg {
					return InstallExecuteMsg{
						ReleaseName:     d.releaseNameInput.Value(),
						Namespace:       d.GetNamespace(),
						Values:          d.valuesContent,
						CreateNamespace: d.createNamespace,
						ValueFiles:      d.valueFiles,
						SetValues:       d.setValues,
					}
				}
			}
		case InstallFieldCancel:
			d.Close()
			d.result = nil
			return d, nil
		}

	case " ":
		if d.field == InstallFieldCreateNS {
			d.createNamespace = !d.createNamespace
		}
	}

	// Update focused text input
	var cmd tea.Cmd
	switch d.field {
	case InstallFieldReleaseName:
		d.releaseNameInput, cmd = d.releaseNameInput.Update(msg)
		return d, cmd
	case InstallFieldNamespace:
		d.namespaceInput, cmd = d.namespaceInput.Update(msg)
		return d, cmd
	}

	return d, nil
}

func (d *InstallDialog) cycleFieldForward() {
	d.field = (d.field + 1) % 7
	if d.field == InstallFieldValues {
		d.field = InstallFieldConfirm
	}
	if d.field == InstallFieldTemplate && len(d.availableTemplates) == 0 {
		d.field = InstallFieldReleaseName
	}
}

func (d *InstallDialog) cycleFieldBackward() {
	d.field = (d.field + 6) % 7
	if d.field == InstallFieldValues {
		d.field = InstallFieldCreateNS
	}
	if d.field == InstallFieldTemplate && len(d.availableTemplates) == 0 {
		d.field = InstallFieldCancel
	}
}

func (d *InstallDialog) updateFocus() {
	d.releaseNameInput.Blur()
	d.namespaceInput.Blur()

	switch d.field {
	case InstallFieldReleaseName:
		d.releaseNameInput.Focus()
	case InstallFieldNamespace:
		d.namespaceInput.Focus()
	}
}

func (d *InstallDialog) applySelectedTemplate() {
	if d.selectedTemplateIdx >= 0 && d.selectedTemplateIdx < len(d.availableTemplates) {
		template := d.availableTemplates[d.selectedTemplateIdx]
		d.valuesContent = template.Values
		d.valuesErr = nil // Parent will re-validate
	}
	// When selectedTemplateIdx == -1, reset to default is handled by parent
}

// View renders the install dialog.
func (d *InstallDialog) View() string {
	if !d.IsOpen() {
		return ""
	}

	dialogWidth := 50

	dialogStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(d.BorderColor).
		Padding(1, 2).
		Width(dialogWidth)

	// Icons with defaults
	addIcon := d.AddIcon
	if addIcon == "" {
		addIcon = "+"
	}
	arrowIcon := d.ArrowIcon
	if arrowIcon == "" {
		arrowIcon = "→"
	}
	checkIcon := d.CheckIcon
	if checkIcon == "" {
		checkIcon = "✓"
	}
	crossIcon := d.CrossIcon
	if crossIcon == "" {
		crossIcon = "✗"
	}
	lockIcon := d.LockIcon
	if lockIcon == "" {
		lockIcon = "🔒"
	}

	var content strings.Builder
	content.WriteString(d.TitleStyle.Render(addIcon+" Install Chart") + "\n\n")

	// Chart info
	if d.chartName != "" {
		chartInfo := d.LabelStyle.Render("Chart: ") + d.chartName + " v" + d.chartVersion
		if d.valuesEncrypted {
			chartInfo += " " + d.WarningStyle.Render(lockIcon+" Encrypted")
		}
		content.WriteString(chartInfo + "\n\n")
	}

	// Template selection (only if templates exist)
	if len(d.availableTemplates) > 0 {
		if d.field == InstallFieldTemplate {
			content.WriteString(d.HighlightedStyle.Render(arrowIcon+" Template:") + "\n")
		} else {
			content.WriteString(d.LabelStyle.Render("  Template:") + "\n")
		}

		var templateDisplay string
		if d.selectedTemplateIdx == -1 {
			templateDisplay = "(No template - use defaults)"
		} else {
			t := d.availableTemplates[d.selectedTemplateIdx]
			templateDisplay = t.Name
			if t.Description != "" {
				templateDisplay += " - " + t.Description
			}
		}

		if d.field == InstallFieldTemplate {
			content.WriteString("  " + d.SelectedStyle.Render(templateDisplay) + "\n")
			content.WriteString(d.MutedStyle.Render("  j/k:select  "+fmt.Sprintf("%d templates available", len(d.availableTemplates))) + "\n")
		} else {
			content.WriteString("  " + templateDisplay + "\n")
		}

		// Show version warning if the selected template has a version mismatch
		if d.selectedTemplateIdx >= 0 && d.selectedTemplateIdx < len(d.availableTemplates) {
			if w := d.availableTemplates[d.selectedTemplateIdx].VersionWarning; w != "" {
				content.WriteString(d.WarningStyle.Render("  ⚠ "+w) + "\n")
			}
		}
		content.WriteString("\n")
	}

	// Release name field
	if d.field == InstallFieldReleaseName {
		content.WriteString(d.HighlightedStyle.Render(arrowIcon+" Release Name:") + "\n")
	} else {
		content.WriteString(d.LabelStyle.Render("  Release Name:") + "\n")
	}
	content.WriteString("  " + d.releaseNameInput.View() + "\n\n")

	// Namespace field
	if d.field == InstallFieldNamespace {
		content.WriteString(d.HighlightedStyle.Render(arrowIcon+" Namespace:") + "\n")
	} else {
		content.WriteString(d.LabelStyle.Render("  Namespace:") + "\n")
	}
	content.WriteString("  " + d.namespaceInput.View() + "\n\n")

	// Create namespace checkbox
	checkbox := "[ ]"
	if d.createNamespace {
		checkbox = "[" + checkIcon + "]"
	}
	if d.field == InstallFieldCreateNS {
		content.WriteString(d.HighlightedStyle.Render(arrowIcon+" "+checkbox+" Create namespace") + "\n\n")
	} else {
		content.WriteString(d.LabelStyle.Render("  "+checkbox+" Create namespace") + "\n\n")
	}

	// Values section
	valuesLines := len(strings.Split(d.valuesContent, "\n"))
	valuesLabel := fmt.Sprintf("  Values: %d lines loaded", valuesLines)
	if d.valuesEncrypted {
		valuesLabel += " " + lockIcon
	}
	content.WriteString(d.LabelStyle.Render(valuesLabel) + "\n")
	content.WriteString(d.MutedStyle.Render("  Press 'e' to edit in $EDITOR") + "\n")

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

	// Show encryption warnings or errors
	if d.decryptionError != "" {
		content.WriteString(d.WarningStyle.Render("  "+lockIcon+" "+d.decryptionError) + "\n")
	} else if d.encryptOnSave {
		content.WriteString(d.MutedStyle.Render("  "+lockIcon+" Values will be re-encrypted on save") + "\n")
	}

	if d.valuesErr != nil {
		content.WriteString(d.ErrorStyle.Render("  "+crossIcon+" YAML Error: "+d.valuesErr.Error()) + "\n")
	} else if d.decryptionError == "" {
		content.WriteString(d.SuccessStyle.Render("  "+checkIcon+" Valid YAML") + "\n")
	}
	content.WriteString("\n")

	// Buttons
	installBtn := d.ButtonStyle.Render(" Install ")
	cancelBtn := d.ButtonStyle.Render(" Cancel ")
	if d.field == InstallFieldConfirm {
		installBtn = d.ButtonFocusStyle.Render(" Install ")
	}
	if d.field == InstallFieldCancel {
		cancelBtn = d.ButtonFocusStyle.Render(" Cancel ")
	}

	if d.installing {
		if d.spinnerView != nil {
			content.WriteString(d.spinnerView() + "\n")
		} else {
			content.WriteString("Installing...\n")
		}
	} else {
		content.WriteString(installBtn + "  " + cancelBtn + "\n")
	}

	// Show result
	if d.result != nil {
		content.WriteString("\n")
		if d.result.Error != nil {
			errMsg := d.result.Error.Error()
			if len(errMsg) > dialogWidth-8 {
				errMsg = errMsg[:dialogWidth-11] + "..."
			}
			content.WriteString(d.ErrorStyle.Render(crossIcon + " " + errMsg))
		} else {
			content.WriteString(d.SuccessStyle.Render(checkIcon + " Installed successfully!"))
			content.WriteString("\n" + d.LabelStyle.Render("Release: "+d.result.ReleaseName))
			content.WriteString("\n" + d.LabelStyle.Render("Status: "+d.result.Status))
		}
	}

	// Footer hints
	content.WriteString("\n\n" + d.MutedStyle.Render("Tab:navigate  e:edit  f:file  s:set  F:sources  d:preview  Esc:cancel"))

	dialog := dialogStyle.Render(content.String())
	return lipgloss.Place(d.Width, d.Height, lipgloss.Center, lipgloss.Center, dialog)
}

// SetStyles configures the dialog styles from the parent view.
func (d *InstallDialog) SetStyles(title, label, muted, value, errStyle, success, warning, highlighted, selected, button, buttonFocus lipgloss.Style, borderColor lipgloss.Color, addIcon, arrowIcon, checkIcon, crossIcon, lockIcon string) {
	d.TitleStyle = title
	d.LabelStyle = label
	d.MutedStyle = muted
	d.ValueStyle = value
	d.ErrorStyle = errStyle
	d.SuccessStyle = success
	d.WarningStyle = warning
	d.HighlightedStyle = highlighted
	d.SelectedStyle = selected
	d.ButtonStyle = button
	d.ButtonFocusStyle = buttonFocus
	d.BorderColor = borderColor
	d.AddIcon = addIcon
	d.ArrowIcon = arrowIcon
	d.CheckIcon = checkIcon
	d.CrossIcon = crossIcon
	d.LockIcon = lockIcon
}

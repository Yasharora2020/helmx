package dialog

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/yasharora2020/helmx/internal/tui/validation"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"gopkg.in/yaml.v3"
)

// FileImportResultMsg is sent when values are successfully imported from a file.
type FileImportResultMsg struct {
	Content string
	Path    string // The original file path
}

// FileImportDialog handles importing values from a YAML file.
type FileImportDialog struct {
	BaseDialog
	input textinput.Model
	err   string

	// Styles (injected from parent)
	TitleStyle  lipgloss.Style
	LabelStyle  lipgloss.Style
	MutedStyle  lipgloss.Style
	ErrorStyle  lipgloss.Style
	BorderColor lipgloss.Color
	FileIcon    string
	CrossIcon   string
}

// NewFileImportDialog creates a new file import dialog.
func NewFileImportDialog() *FileImportDialog {
	input := textinput.New()
	input.Placeholder = "/path/to/values.yaml"
	input.CharLimit = 256
	input.Width = 45

	return &FileImportDialog{
		input: input,
	}
}

// Open opens the dialog and focuses the input.
func (d *FileImportDialog) Open() {
	d.BaseDialog.Open()
	d.input.SetValue("")
	d.input.Focus()
	d.err = ""
}

// Close closes the dialog and clears state.
func (d *FileImportDialog) Close() {
	d.BaseDialog.Close()
	d.err = ""
	d.input.Blur()
}

// Update handles messages for the file import dialog.
func (d *FileImportDialog) Update(msg tea.Msg) (Dialog, tea.Cmd) {
	if !d.IsOpen() {
		return d, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		return d.handleKey(msg)
	}

	// Update text input
	var cmd tea.Cmd
	d.input, cmd = d.input.Update(msg)
	return d, cmd
}

func (d *FileImportDialog) handleKey(msg tea.KeyMsg) (Dialog, tea.Cmd) {
	switch msg.String() {
	case "esc":
		d.Close()
		return d, nil

	case "enter":
		path := d.input.Value()
		if path == "" {
			d.err = "Please enter a file path"
			return d, nil
		}

		// Validate path format
		if err := validation.ValidatePath(path); err != nil {
			d.err = err.Error()
			return d, nil
		}

		// Expand ~ to home directory
		if strings.HasPrefix(path, "~/") {
			home, _ := os.UserHomeDir()
			path = filepath.Join(home, path[2:])
		}

		// Read file
		data, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				d.err = "File not found"
			} else if os.IsPermission(err) {
				d.err = "Permission denied"
			} else {
				d.err = err.Error()
			}
			return d, nil
		}

		// Validate YAML
		var values map[string]interface{}
		if err := yaml.Unmarshal(data, &values); err != nil {
			d.err = "Invalid YAML: " + err.Error()
			return d, nil
		}

		// Success - send result and close
		content := string(data)
		resolvedPath := path
		d.Close()
		return d, func() tea.Msg {
			return FileImportResultMsg{Content: content, Path: resolvedPath}
		}
	}

	// Pass other keys to text input
	var cmd tea.Cmd
	d.input, cmd = d.input.Update(msg)
	return d, cmd
}

// View renders the file import dialog.
func (d *FileImportDialog) View() string {
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
	icon := d.FileIcon
	if icon == "" {
		icon = "📄"
	}
	content.WriteString(d.TitleStyle.Render(icon+" Import Values File") + "\n\n")

	// Input field
	content.WriteString(d.LabelStyle.Render("Path: ") + d.input.View() + "\n\n")

	// Help text
	content.WriteString(d.MutedStyle.Render("Supports: ~/path, ./path, /absolute/path") + "\n")

	// Error message (if any)
	if d.err != "" {
		cross := d.CrossIcon
		if cross == "" {
			cross = "✗"
		}
		content.WriteString("\n" + d.ErrorStyle.Render(cross+" "+d.err) + "\n")
	}

	// Footer
	content.WriteString("\n" + d.MutedStyle.Render("Enter:confirm  Esc:cancel"))

	dialog := dialogStyle.Render(content.String())
	return lipgloss.Place(d.Width, d.Height, lipgloss.Center, lipgloss.Center, dialog)
}

// SetStyles configures the dialog styles from the parent view.
func (d *FileImportDialog) SetStyles(title, label, muted, errStyle lipgloss.Style, borderColor lipgloss.Color, fileIcon, crossIcon string) {
	d.TitleStyle = title
	d.LabelStyle = label
	d.MutedStyle = muted
	d.ErrorStyle = errStyle
	d.BorderColor = borderColor
	d.FileIcon = fileIcon
	d.CrossIcon = crossIcon
}

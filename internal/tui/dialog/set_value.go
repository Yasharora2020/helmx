package dialog

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// SetValueResultMsg is sent when the user enters a valid key=value pair.
type SetValueResultMsg struct {
	KeyValue string // e.g., "image.tag=v2.0"
}

// SetValueDialog handles entering --set key=value overrides.
type SetValueDialog struct {
	BaseDialog
	input textinput.Model
	err   string

	// Styles (injected from parent)
	TitleStyle       lipgloss.Style
	LabelStyle       lipgloss.Style
	MutedStyle       lipgloss.Style
	ErrorStyle       lipgloss.Style
	ButtonStyle      lipgloss.Style
	ButtonFocusStyle lipgloss.Style
	BorderColor      lipgloss.Color
}

// NewSetValueDialog creates a new set value dialog.
func NewSetValueDialog() *SetValueDialog {
	input := textinput.New()
	input.Placeholder = "image.tag=v2.0"
	input.CharLimit = 200
	input.Width = 40

	return &SetValueDialog{
		input: input,
	}
}

// Open opens the dialog and focuses the input.
func (d *SetValueDialog) Open() {
	d.BaseDialog.Open()
	d.input.SetValue("")
	d.input.Focus()
	d.err = ""
}

// Close closes the dialog and clears state.
func (d *SetValueDialog) Close() {
	d.BaseDialog.Close()
	d.err = ""
	d.input.Blur()
}

// Update handles messages for the set value dialog.
func (d *SetValueDialog) Update(msg tea.Msg) (Dialog, tea.Cmd) {
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

func (d *SetValueDialog) handleKey(msg tea.KeyMsg) (Dialog, tea.Cmd) {
	switch msg.String() {
	case "esc":
		d.Close()
		return d, nil

	case "enter":
		value := d.input.Value()
		if value == "" {
			d.err = "must be in key=value format"
			return d, nil
		}

		// Must contain '=' with a non-empty key before it
		eqIdx := strings.Index(value, "=")
		if eqIdx < 1 {
			d.err = "must be in key=value format"
			return d, nil
		}

		// Valid key=value pair
		d.Close()
		kv := value
		return d, func() tea.Msg {
			return SetValueResultMsg{KeyValue: kv}
		}
	}

	// Pass other keys to text input
	var cmd tea.Cmd
	d.input, cmd = d.input.Update(msg)
	return d, cmd
}

// View renders the set value dialog.
func (d *SetValueDialog) View() string {
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
	content.WriteString(d.TitleStyle.Render("Set Value Override") + "\n\n")

	// Input field
	content.WriteString(d.LabelStyle.Render("Value: ") + d.input.View() + "\n\n")

	// Help text
	content.WriteString(d.MutedStyle.Render("Enter key=value pair (e.g., image.tag=v2.0)") + "\n")

	// Error message (if any)
	if d.err != "" {
		content.WriteString("\n" + d.ErrorStyle.Render("✗ "+d.err) + "\n")
	}

	// Footer
	content.WriteString("\n" + d.MutedStyle.Render("Enter:confirm  Esc:cancel"))

	dialog := dialogStyle.Render(content.String())
	return lipgloss.Place(d.Width, d.Height, lipgloss.Center, lipgloss.Center, dialog)
}

// SetStyles configures the dialog styles from the parent view.
func (d *SetValueDialog) SetStyles(title, label, muted, errStyle, button, buttonFocus lipgloss.Style, borderColor lipgloss.Color) {
	d.TitleStyle = title
	d.LabelStyle = label
	d.MutedStyle = muted
	d.ErrorStyle = errStyle
	d.ButtonStyle = button
	d.ButtonFocusStyle = buttonFocus
	d.BorderColor = borderColor
}

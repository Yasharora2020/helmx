package dialog

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// SchemaDialog displays a values.schema.json browser for Helm charts.
type SchemaDialog struct {
	BaseDialog
	viewport viewport.Model
	data     map[string]interface{}
	path     []string
	expanded map[string]bool

	// Chart info
	chartName string

	// Styles (injected from parent)
	TitleStyle   lipgloss.Style
	MutedStyle   lipgloss.Style
	ValueStyle   lipgloss.Style
	ErrorStyle   lipgloss.Style
	SuccessStyle lipgloss.Style
	WarningStyle lipgloss.Style
	BorderColor  lipgloss.Color
	ValuesIcon   string
}

// NewSchemaDialog creates a new schema browser dialog.
func NewSchemaDialog() *SchemaDialog {
	vp := viewport.New(70, 20)
	return &SchemaDialog{
		viewport: vp,
		expanded: make(map[string]bool),
	}
}

// Open opens the dialog with the given schema data.
func (d *SchemaDialog) Open() {
	d.BaseDialog.Open()
	d.path = []string{}
	d.viewport.GotoTop()
}

// OpenWithSchema opens the dialog with parsed schema data.
func (d *SchemaDialog) OpenWithSchema(schemaBytes []byte, chartName string) {
	d.chartName = chartName
	d.BaseDialog.Open()
	d.path = []string{}
	d.viewport.GotoTop()

	// Parse the JSON schema
	if len(schemaBytes) > 0 {
		var schema map[string]interface{}
		if err := json.Unmarshal(schemaBytes, &schema); err == nil {
			d.data = schema
		} else {
			d.data = nil
		}
	} else {
		d.data = nil
	}

	// Render initial content
	d.updateContent()
}

// Update handles messages for the schema dialog.
func (d *SchemaDialog) Update(msg tea.Msg) (Dialog, tea.Cmd) {
	if !d.IsOpen() {
		return d, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		return d.handleKey(msg)
	}

	return d, nil
}

func (d *SchemaDialog) handleKey(msg tea.KeyMsg) (Dialog, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		d.Close()
		return d, nil
	case "up", "k":
		d.viewport.LineUp(1)
	case "down", "j":
		d.viewport.LineDown(1)
	case "pgup", "ctrl+u":
		d.viewport.HalfViewUp()
	case "pgdown", "ctrl+d":
		d.viewport.HalfViewDown()
	case "home", "g":
		d.viewport.GotoTop()
	case "end", "G":
		d.viewport.GotoBottom()
	}
	return d, nil
}

func (d *SchemaDialog) updateContent() {
	if d.data == nil {
		d.viewport.SetContent(d.MutedStyle.Render("No schema available or failed to parse."))
		return
	}

	content := d.renderSchemaTree(d.data, 0)
	d.viewport.SetContent(content)
}

func (d *SchemaDialog) renderSchemaTree(schema map[string]interface{}, depth int) string {
	var lines []string

	// Get schema title and description if at root level
	if depth == 0 {
		if title, ok := schema["title"].(string); ok && title != "" {
			lines = append(lines, d.TitleStyle.Render("📋 "+title))
		}
		if desc, ok := schema["description"].(string); ok && desc != "" {
			lines = append(lines, d.MutedStyle.Render(desc))
		}
		if len(lines) > 0 {
			lines = append(lines, "")
		}
	}

	// Get properties
	properties, hasProps := schema["properties"].(map[string]interface{})
	if !hasProps {
		return strings.Join(lines, "\n")
	}

	// Get required fields
	requiredSlice, _ := schema["required"].([]interface{})
	required := make(map[string]bool)
	for _, r := range requiredSlice {
		if rs, ok := r.(string); ok {
			required[rs] = true
		}
	}

	// Sort property names
	propNames := make([]string, 0, len(properties))
	for name := range properties {
		propNames = append(propNames, name)
	}
	sort.Strings(propNames)

	// Render each property
	for _, name := range propNames {
		prop, ok := properties[name].(map[string]interface{})
		if !ok {
			continue
		}

		indent := strings.Repeat("  ", depth)
		line := d.renderSchemaProperty(name, prop, required[name], indent)
		lines = append(lines, line)

		// Recursively render nested properties
		if nestedProps, hasNested := prop["properties"].(map[string]interface{}); hasNested {
			nestedSchema := map[string]interface{}{
				"properties": nestedProps,
			}
			if nestedRequired, ok := prop["required"]; ok {
				nestedSchema["required"] = nestedRequired
			}
			nested := d.renderSchemaTree(nestedSchema, depth+1)
			if nested != "" {
				lines = append(lines, nested)
			}
		}

		// Handle items for arrays
		if items, hasItems := prop["items"].(map[string]interface{}); hasItems {
			if itemProps, ok := items["properties"].(map[string]interface{}); ok {
				nestedSchema := map[string]interface{}{
					"properties": itemProps,
				}
				if nestedRequired, ok := items["required"]; ok {
					nestedSchema["required"] = nestedRequired
				}
				lines = append(lines, strings.Repeat("  ", depth+1)+d.MutedStyle.Render("└─ Array items:"))
				nested := d.renderSchemaTree(nestedSchema, depth+2)
				if nested != "" {
					lines = append(lines, nested)
				}
			}
		}
	}

	return strings.Join(lines, "\n")
}

func (d *SchemaDialog) renderSchemaProperty(name string, prop map[string]interface{}, isRequired bool, indent string) string {
	var parts []string

	// Property name
	if isRequired {
		parts = append(parts, indent+d.ValueStyle.Render(name)+" "+d.ErrorStyle.Render("*"))
	} else {
		parts = append(parts, indent+d.ValueStyle.Render(name))
	}

	// Type
	propType := ""
	if t, ok := prop["type"].(string); ok {
		propType = t
	} else if types, ok := prop["type"].([]interface{}); ok {
		// Handle union types like ["string", "null"]
		typeStrs := make([]string, len(types))
		for i, t := range types {
			if ts, ok := t.(string); ok {
				typeStrs[i] = ts
			}
		}
		propType = strings.Join(typeStrs, "|")
	}

	if propType != "" {
		parts = append(parts, d.MutedStyle.Render(" ("+propType+")"))
	}

	// Default value
	if defVal, hasDefault := prop["default"]; hasDefault {
		defStr := fmt.Sprintf("%v", defVal)
		if len(defStr) > 30 {
			defStr = defStr[:30] + "..."
		}
		parts = append(parts, d.SuccessStyle.Render(" = "+defStr))
	}

	// Description on the same line or next line
	if desc, ok := prop["description"].(string); ok && desc != "" {
		// Truncate long descriptions
		if len(desc) > 60 {
			desc = desc[:60] + "..."
		}
		parts = append(parts, "\n"+indent+"  "+d.MutedStyle.Render(desc))
	}

	// Enum values
	if enum, ok := prop["enum"].([]interface{}); ok && len(enum) > 0 {
		enumStrs := make([]string, len(enum))
		for i, e := range enum {
			enumStrs[i] = fmt.Sprintf("%v", e)
		}
		parts = append(parts, "\n"+indent+"  "+d.WarningStyle.Render("values: ["+strings.Join(enumStrs, ", ")+"]"))
	}

	return strings.Join(parts, "")
}

// View renders the schema dialog.
func (d *SchemaDialog) View() string {
	if !d.IsOpen() {
		return ""
	}

	dialogWidth := 80
	if d.Width < 90 {
		dialogWidth = d.Width - 10
	}
	dialogHeight := min(d.Height-10, 30)
	if dialogHeight < 15 {
		dialogHeight = 15
	}

	dialogStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(d.BorderColor).
		Padding(1, 2).
		Width(dialogWidth)

	var content strings.Builder

	// Header
	icon := d.ValuesIcon
	if icon == "" {
		icon = "📋"
	}
	content.WriteString(d.TitleStyle.Render(icon + " Values Schema: " + d.chartName))
	content.WriteString("\n\n")

	// Schema content in viewport
	d.viewport.Width = dialogWidth - 6
	d.viewport.Height = dialogHeight - 8
	content.WriteString(d.viewport.View())
	content.WriteString("\n\n")

	// Legend
	content.WriteString(d.MutedStyle.Render("* required"))
	content.WriteString("\n")

	// Footer
	content.WriteString(d.MutedStyle.Render("j/k:scroll  g/G:top/bottom  Esc:close"))

	dialog := dialogStyle.Render(content.String())
	return lipgloss.Place(d.Width, d.Height, lipgloss.Center, lipgloss.Center, dialog)
}

// SetStyles configures the dialog styles from the parent view.
func (d *SchemaDialog) SetStyles(title, muted, value, errStyle, success, warning lipgloss.Style, borderColor lipgloss.Color, valuesIcon string) {
	d.TitleStyle = title
	d.MutedStyle = muted
	d.ValueStyle = value
	d.ErrorStyle = errStyle
	d.SuccessStyle = success
	d.WarningStyle = warning
	d.BorderColor = borderColor
	d.ValuesIcon = valuesIcon
}

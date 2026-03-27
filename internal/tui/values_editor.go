package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"gopkg.in/yaml.v3"
)

// ValuesEditor provides YAML editing for chart values
type ValuesEditor struct {
	textarea    textarea.Model
	width       int
	height      int
	title       string
	err         error
	originalRaw string

	// File I/O
	filePath       string          // Current file path
	showFilePrompt bool            // Show file path input
	filePromptMode string          // "load" or "save"
	filePathInput  textinput.Model // Input for file path
	fileStatus     string          // Status message (e.g., "Saved to ...")

	// Search
	showSearchPrompt bool            // Show search input
	searchInput      textinput.Model // Search query input
	searchQuery      string          // Current search query
	searchMatches    []int           // Line numbers (0-indexed) with matches
	currentMatch     int             // Index into searchMatches
}

// NewValuesEditor creates a new values editor
func NewValuesEditor(title string) *ValuesEditor {
	ta := textarea.New()
	ta.Placeholder = "# Enter your values here in YAML format\n# Example:\n# replicaCount: 3\n# image:\n#   tag: latest"
	ta.ShowLineNumbers = true
	ta.SetWidth(60)
	ta.SetHeight(20)

	// File path input
	fp := textinput.New()
	fp.Placeholder = "./values.yaml"
	fp.CharLimit = 256
	fp.Width = 50

	// Search input
	si := textinput.New()
	si.Placeholder = "Search..."
	si.CharLimit = 100
	si.Width = 30

	return &ValuesEditor{
		textarea:      ta,
		title:         title,
		filePathInput: fp,
		searchInput:   si,
	}
}

// SetContent sets the editor content
func (v *ValuesEditor) SetContent(content string) {
	v.originalRaw = content
	v.textarea.SetValue(content)
}

// GetContent returns the current editor content
func (v *ValuesEditor) GetContent() string {
	return v.textarea.Value()
}

// GetValues parses the YAML and returns as map
func (v *ValuesEditor) GetValues() (map[string]interface{}, error) {
	content := v.textarea.Value()
	if strings.TrimSpace(content) == "" {
		return nil, nil
	}

	var values map[string]interface{}
	err := yaml.Unmarshal([]byte(content), &values)
	if err != nil {
		v.err = err
		return nil, err
	}
	v.err = nil
	return values, nil
}

// HasChanges returns true if content was modified
func (v *ValuesEditor) HasChanges() bool {
	return v.textarea.Value() != v.originalRaw
}

// LoadFromFile loads values from a file
func (v *ValuesEditor) LoadFromFile(path string) error {
	// Expand home directory
	if strings.HasPrefix(path, "~/") {
		home, _ := os.UserHomeDir()
		path = filepath.Join(home, path[2:])
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	v.SetContent(string(data))
	v.filePath = path
	v.fileStatus = "Loaded from " + filepath.Base(path)
	return nil
}

// SaveToFile saves values to a file
func (v *ValuesEditor) SaveToFile(path string) error {
	// Expand home directory
	if strings.HasPrefix(path, "~/") {
		home, _ := os.UserHomeDir()
		path = filepath.Join(home, path[2:])
	}

	// Create directory if needed
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	content := v.textarea.Value()
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return err
	}

	v.filePath = path
	v.fileStatus = "Saved to " + filepath.Base(path)
	return nil
}

// GetFilePath returns the current file path
func (v *ValuesEditor) GetFilePath() string {
	return v.filePath
}

// IsShowingFilePrompt returns true if file prompt is visible
func (v *ValuesEditor) IsShowingFilePrompt() bool {
	return v.showFilePrompt
}

// IsShowingSearchPrompt returns true if search prompt is visible
func (v *ValuesEditor) IsShowingSearchPrompt() bool {
	return v.showSearchPrompt
}

// Focus focuses the editor
func (v *ValuesEditor) Focus() {
	v.textarea.Focus()
}

// Blur blurs the editor
func (v *ValuesEditor) Blur() {
	v.textarea.Blur()
}

// SetSize sets the editor dimensions
func (v *ValuesEditor) SetSize(width, height int) {
	v.width = width
	v.height = height
	v.textarea.SetWidth(width - 4)
	v.textarea.SetHeight(height - 6)
}

// Update handles messages
func (v *ValuesEditor) Update(msg tea.Msg) (*ValuesEditor, tea.Cmd) {
	var cmd tea.Cmd

	// Handle file prompt mode
	if v.showFilePrompt {
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.String() {
			case "enter":
				path := v.filePathInput.Value()
				if path != "" {
					if v.filePromptMode == "load" {
						if err := v.LoadFromFile(path); err != nil {
							v.fileStatus = "Error: " + err.Error()
						}
					} else {
						if err := v.SaveToFile(path); err != nil {
							v.fileStatus = "Error: " + err.Error()
						}
					}
				}
				v.showFilePrompt = false
				v.textarea.Focus()
				return v, nil
			case "esc":
				v.showFilePrompt = false
				v.textarea.Focus()
				return v, nil
			}
		}
		v.filePathInput, cmd = v.filePathInput.Update(msg)
		return v, cmd
	}

	// Handle search prompt mode
	if v.showSearchPrompt {
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.String() {
			case "enter":
				// Save query, find matches, jump to current match, close prompt
				v.searchQuery = v.searchInput.Value()
				v.findMatches()
				if len(v.searchMatches) > 0 {
					v.jumpToLine(v.searchMatches[v.currentMatch])
				}
				v.showSearchPrompt = false
				v.textarea.Focus()
				return v, nil
			case "esc":
				// Close search and clear
				v.showSearchPrompt = false
				v.searchQuery = ""
				v.searchMatches = nil
				v.textarea.Focus()
				return v, nil
			case "down", "ctrl+n":
				// Next match
				if len(v.searchMatches) > 0 {
					v.currentMatch = (v.currentMatch + 1) % len(v.searchMatches)
					v.jumpToLine(v.searchMatches[v.currentMatch])
				}
				return v, nil
			case "up", "ctrl+p":
				// Previous match
				if len(v.searchMatches) > 0 {
					v.currentMatch = (v.currentMatch - 1 + len(v.searchMatches)) % len(v.searchMatches)
					v.jumpToLine(v.searchMatches[v.currentMatch])
				}
				return v, nil
			}
		}
		// Update search input and find matches as user types
		v.searchInput, cmd = v.searchInput.Update(msg)
		v.findMatches()
		return v, cmd
	}

	// Handle main editor
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+f", "f3":
			// Open search prompt (F3 = find next, reopens search)
			v.showSearchPrompt = true
			v.searchInput.SetValue(v.searchQuery) // Keep previous query
			v.searchInput.Focus()
			v.textarea.Blur()
			// If F3 and we have matches, jump to next
			if msg.String() == "f3" && len(v.searchMatches) > 0 {
				v.currentMatch = (v.currentMatch + 1) % len(v.searchMatches)
				v.jumpToLine(v.searchMatches[v.currentMatch])
			}
			return v, textinput.Blink
		case "ctrl+o":
			// Open file prompt for loading
			v.showFilePrompt = true
			v.filePromptMode = "load"
			v.filePathInput.SetValue("")
			v.filePathInput.Placeholder = "./values.yaml"
			v.filePathInput.Focus()
			v.textarea.Blur()
			v.fileStatus = ""
			return v, textinput.Blink
		case "ctrl+s":
			// Open file prompt for saving (or save to existing path)
			v.showFilePrompt = true
			v.filePromptMode = "save"
			if v.filePath != "" {
				v.filePathInput.SetValue(v.filePath)
			} else {
				v.filePathInput.SetValue("")
			}
			v.filePathInput.Placeholder = "./values.yaml"
			v.filePathInput.Focus()
			v.textarea.Blur()
			v.fileStatus = ""
			return v, textinput.Blink
		}
	}

	v.textarea, cmd = v.textarea.Update(msg)

	// Validate YAML on change
	if _, ok := msg.(tea.KeyMsg); ok {
		_, _ = v.GetValues() // This sets v.err if invalid
	}

	return v, cmd
}

// View renders the editor
func (v *ValuesEditor) View() string {
	titleStyle := S.Title
	hintStyle := S.Muted
	borderStyle := S.BorderFocus

	// Title with keybinding hints
	titleLine := titleStyle.Render(v.title)
	hints := hintStyle.Render("  Ctrl+F:Search  Ctrl+O:Load  Ctrl+S:Save")
	header := titleLine + hints

	content := header + "\n\n" + v.textarea.View()

	// Show file prompt if active
	if v.showFilePrompt {
		promptLabel := "Load from: "
		if v.filePromptMode == "save" {
			promptLabel = "Save to: "
		}
		promptStyle := S.Warning
		content += "\n" + promptStyle.Render(promptLabel) + v.filePathInput.View()
		content += "\n" + hintStyle.Render("Enter to confirm, Esc to cancel")
	} else if v.showSearchPrompt {
		// Show search prompt
		searchStyle := S.Warning
		content += "\n" + searchStyle.Render("Search: ") + v.searchInput.View()

		// Show match count
		if len(v.searchMatches) > 0 {
			matchInfo := fmt.Sprintf(" (%d/%d)", v.currentMatch+1, len(v.searchMatches))
			content += hintStyle.Render(matchInfo)
		} else if v.searchInput.Value() != "" {
			content += hintStyle.Render(" (no matches)")
		}
		content += "\n" + hintStyle.Render("Enter:jump  ↓/↑:next/prev  Esc:close")
	} else {
		// Show validation status
		if v.err != nil {
			errStyle := S.Error
			content += "\n" + errStyle.Render("✗ YAML Error: "+v.err.Error())
		} else {
			validStyle := S.Success
			content += "\n" + validStyle.Render("✓ Valid YAML")
		}

		// Show file status if any
		if v.fileStatus != "" {
			statusStyle := S.Muted
			content += "  " + statusStyle.Render(v.fileStatus)
		}
	}

	return borderStyle.Width(v.width).Height(v.height).Render(content)
}

// findMatches searches content for query and stores matching line numbers
func (v *ValuesEditor) findMatches() {
	v.searchMatches = nil
	v.currentMatch = 0

	query := strings.ToLower(v.searchInput.Value())
	if query == "" {
		return
	}

	lines := strings.Split(v.textarea.Value(), "\n")
	for i, line := range lines {
		if strings.Contains(strings.ToLower(line), query) {
			v.searchMatches = append(v.searchMatches, i)
		}
	}
}

// jumpToLine moves cursor to the start of a line
func (v *ValuesEditor) jumpToLine(lineNum int) {
	content := v.textarea.Value()
	lines := strings.Split(content, "\n")

	// Calculate character offset to line start
	offset := 0
	for i := 0; i < lineNum && i < len(lines); i++ {
		offset += len(lines[i]) + 1 // +1 for newline
	}

	// Set cursor to the calculated position
	v.textarea.SetCursor(offset)
}

// GenerateValuesYAML converts a map to YAML string
func GenerateValuesYAML(values map[string]interface{}) string {
	if values == nil {
		return ""
	}
	data, err := yaml.Marshal(values)
	if err != nil {
		return "# Error generating YAML: " + err.Error()
	}
	return string(data)
}

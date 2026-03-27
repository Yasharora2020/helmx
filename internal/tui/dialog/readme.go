package dialog

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ReadmeDialog displays a README file with search functionality.
type ReadmeDialog struct {
	BaseDialog
	viewport viewport.Model
	content  string // Original README content

	// Search state
	searchMode    bool
	searchQuery   string
	searchMatches []int // Line numbers containing matches
	searchIdx     int   // Current match index

	// Chart info
	chartName string

	// Styles (injected from parent)
	TitleStyle       lipgloss.Style
	LabelStyle       lipgloss.Style
	MutedStyle       lipgloss.Style
	SuccessStyle     lipgloss.Style
	WarningStyle     lipgloss.Style
	HighlightedStyle lipgloss.Style
	BorderColor      lipgloss.Color
	ChartIcon        string
}

// NewReadmeDialog creates a new README viewer dialog.
func NewReadmeDialog() *ReadmeDialog {
	vp := viewport.New(80, 30)
	return &ReadmeDialog{
		viewport: vp,
	}
}

// Open opens the dialog with the given README content.
func (d *ReadmeDialog) Open() {
	d.BaseDialog.Open()
	d.searchMode = false
	d.searchQuery = ""
	d.searchMatches = nil
	d.searchIdx = 0
}

// OpenWithContent opens the dialog with specific content.
func (d *ReadmeDialog) OpenWithContent(content, chartName string) {
	d.chartName = chartName
	d.content = content
	d.BaseDialog.Open()
	d.searchMode = false
	d.searchQuery = ""
	d.searchMatches = nil
	d.searchIdx = 0
	d.viewport.SetContent(content)
	d.viewport.GotoTop()
}

// Update handles messages for the readme dialog.
func (d *ReadmeDialog) Update(msg tea.Msg) (Dialog, tea.Cmd) {
	if !d.IsOpen() {
		return d, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		return d.handleKey(msg)
	}

	return d, nil
}

func (d *ReadmeDialog) handleKey(msg tea.KeyMsg) (Dialog, tea.Cmd) {
	// Handle search mode
	if d.searchMode {
		switch msg.String() {
		case "enter":
			// Execute search
			d.searchMode = false
			if d.searchQuery != "" {
				d.performSearch()
			}
			return d, nil
		case "esc":
			// Cancel search
			d.searchMode = false
			d.searchQuery = ""
			return d, nil
		case "backspace":
			if len(d.searchQuery) > 0 {
				d.searchQuery = d.searchQuery[:len(d.searchQuery)-1]
			}
			return d, nil
		default:
			// Add character to search query
			if len(msg.String()) == 1 {
				d.searchQuery += msg.String()
			}
			return d, nil
		}
	}

	// Normal mode keybindings
	switch msg.String() {
	case "esc", "q":
		d.Close()
		d.searchQuery = ""
		d.searchMatches = nil
		d.searchIdx = 0
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
	case "/":
		// Enter search mode
		d.searchMode = true
		d.searchQuery = ""
		return d, nil
	case "n":
		// Next match
		if len(d.searchMatches) > 0 {
			d.searchIdx = (d.searchIdx + 1) % len(d.searchMatches)
			d.scrollToMatch()
		}
	case "N":
		// Previous match
		if len(d.searchMatches) > 0 {
			d.searchIdx = (d.searchIdx - 1 + len(d.searchMatches)) % len(d.searchMatches)
			d.scrollToMatch()
		}
	}
	return d, nil
}

// performSearch searches for the query in README content
func (d *ReadmeDialog) performSearch() {
	if d.searchQuery == "" || d.content == "" {
		return
	}

	query := strings.ToLower(d.searchQuery)
	lines := strings.Split(d.content, "\n")
	d.searchMatches = nil
	d.searchIdx = 0

	// Find all lines containing the search query
	for i, line := range lines {
		if strings.Contains(strings.ToLower(line), query) {
			d.searchMatches = append(d.searchMatches, i)
		}
	}

	// Highlight matches in content
	d.highlightMatches()

	// Scroll to first match
	if len(d.searchMatches) > 0 {
		d.scrollToMatch()
	}
}

// highlightMatches highlights search matches in the README content
func (d *ReadmeDialog) highlightMatches() {
	if d.searchQuery == "" {
		d.viewport.SetContent(d.content)
		return
	}

	query := strings.ToLower(d.searchQuery)
	lines := strings.Split(d.content, "\n")
	var highlighted []string

	for _, line := range lines {
		lowerLine := strings.ToLower(line)
		if strings.Contains(lowerLine, query) {
			// Highlight all occurrences in this line
			highlightedLine := d.highlightOccurrences(line, d.searchQuery)
			highlighted = append(highlighted, highlightedLine)
		} else {
			highlighted = append(highlighted, line)
		}
	}

	d.viewport.SetContent(strings.Join(highlighted, "\n"))
}

// highlightOccurrences highlights all occurrences of query in text (case-insensitive)
func (d *ReadmeDialog) highlightOccurrences(text, query string) string {
	lowerText := strings.ToLower(text)
	lowerQuery := strings.ToLower(query)

	var result strings.Builder
	lastEnd := 0

	for {
		idx := strings.Index(lowerText[lastEnd:], lowerQuery)
		if idx == -1 {
			result.WriteString(text[lastEnd:])
			break
		}

		// Add text before match
		result.WriteString(text[lastEnd : lastEnd+idx])

		// Add highlighted match (preserve original case)
		matchText := text[lastEnd+idx : lastEnd+idx+len(query)]
		result.WriteString(d.HighlightedStyle.Render(matchText))

		lastEnd = lastEnd + idx + len(query)
	}

	return result.String()
}

// scrollToMatch scrolls the viewport to the current match
func (d *ReadmeDialog) scrollToMatch() {
	if len(d.searchMatches) == 0 {
		return
	}

	lineNum := d.searchMatches[d.searchIdx]
	// Position the match in the middle of the viewport if possible
	targetOffset := lineNum - d.viewport.Height/2
	if targetOffset < 0 {
		targetOffset = 0
	}
	d.viewport.SetYOffset(targetOffset)
}

// View renders the readme dialog.
func (d *ReadmeDialog) View() string {
	if !d.IsOpen() {
		return ""
	}

	// Update viewport size based on screen
	readmeWidth := d.Width - 10
	readmeHeight := d.Height - 8
	if readmeWidth < 40 {
		readmeWidth = 40
	}
	if readmeHeight < 10 {
		readmeHeight = 10
	}
	d.viewport.Width = readmeWidth - 4
	d.viewport.Height = readmeHeight - 6

	dialogStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(d.BorderColor).
		Padding(1, 2).
		Width(readmeWidth)

	var content strings.Builder

	// Title
	icon := d.ChartIcon
	if icon == "" {
		icon = "📦"
	}
	if d.chartName != "" {
		content.WriteString(d.TitleStyle.Render(icon+" README: "+d.chartName) + "\n")
	} else {
		content.WriteString(d.TitleStyle.Render(icon+" README") + "\n")
	}

	// Search bar or scroll info
	if d.searchMode {
		searchPrompt := d.LabelStyle.Render("/") + d.searchQuery + d.MutedStyle.Render("_")
		content.WriteString(searchPrompt + "\n")
	} else if d.searchQuery != "" && len(d.searchMatches) > 0 {
		matchInfo := fmt.Sprintf(" [%d/%d matches for '%s']", d.searchIdx+1, len(d.searchMatches), d.searchQuery)
		content.WriteString(d.SuccessStyle.Render(matchInfo) + "\n")
	} else if d.searchQuery != "" && len(d.searchMatches) == 0 {
		noMatch := fmt.Sprintf(" [No matches for '%s']", d.searchQuery)
		content.WriteString(d.WarningStyle.Render(noMatch) + "\n")
	} else {
		scrollInfo := fmt.Sprintf(" (%.0f%%)", d.viewport.ScrollPercent()*100)
		content.WriteString(d.MutedStyle.Render(scrollInfo) + "\n")
	}

	// Content
	content.WriteString(d.viewport.View() + "\n")

	// Footer hints
	if d.searchMode {
		content.WriteString(d.MutedStyle.Render("Enter:search  Esc:cancel"))
	} else {
		content.WriteString(d.MutedStyle.Render("/:search  n/N:next/prev  j/k:scroll  g/G:top/bottom  q:close"))
	}

	dialog := dialogStyle.Render(content.String())
	return lipgloss.Place(d.Width, d.Height, lipgloss.Center, lipgloss.Center, dialog)
}

// SetStyles configures the dialog styles from the parent view.
func (d *ReadmeDialog) SetStyles(title, label, muted, success, warning, highlighted lipgloss.Style, borderColor lipgloss.Color, chartIcon string) {
	d.TitleStyle = title
	d.LabelStyle = label
	d.MutedStyle = muted
	d.SuccessStyle = success
	d.WarningStyle = warning
	d.HighlightedStyle = highlighted
	d.BorderColor = borderColor
	d.ChartIcon = chartIcon
}

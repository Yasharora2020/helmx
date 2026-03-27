package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// README message type
type readmeMsg struct {
	content string
	err     error
}

// fetchReadme fetches the README from the installed release's chart
func (r *ReleaseDetail) fetchReadme() tea.Cmd {
	return func() tea.Msg {
		// Get the full release (includes the chart object)
		rel, err := r.releaseManager.GetRelease(r.release.Name, r.release.Namespace)
		if err != nil {
			return readmeMsg{err: fmt.Errorf("failed to get release: %w", err)}
		}

		if rel.Chart == nil {
			return readmeMsg{err: fmt.Errorf("no chart data available for this release")}
		}

		// Extract README from chart
		readme := r.chartExplorer.GetChartReadme(rel.Chart)
		if readme == "" {
			return readmeMsg{err: fmt.Errorf("no README found in chart")}
		}

		return readmeMsg{content: readme}
	}
}

// updateReadmeDialog handles input when README dialog is open
func (r *ReleaseDetail) updateReadmeDialog(msg tea.KeyMsg) (*ReleaseDetail, tea.Cmd) {
	// Handle search mode
	if r.readmeSearchMode {
		switch msg.String() {
		case "enter":
			// Execute search
			r.readmeSearchMode = false
			if r.readmeSearchQuery != "" {
				r.performReadmeSearch()
			}
			return r, nil
		case "esc":
			// Cancel search
			r.readmeSearchMode = false
			r.readmeSearchQuery = ""
			return r, nil
		case "backspace":
			if len(r.readmeSearchQuery) > 0 {
				r.readmeSearchQuery = r.readmeSearchQuery[:len(r.readmeSearchQuery)-1]
			}
			return r, nil
		default:
			// Add character to search query
			if len(msg.String()) == 1 {
				r.readmeSearchQuery += msg.String()
			}
			return r, nil
		}
	}

	// Normal mode keybindings
	switch msg.String() {
	case "q", "esc":
		r.showReadmeDialog = false
		r.readmeSearchQuery = ""
		r.readmeSearchMatches = nil
		r.readmeSearchIdx = 0
		r.readmeHighlighted = ""
		return r, nil
	case "j", "down":
		r.readmeViewport.LineDown(1)
	case "k", "up":
		r.readmeViewport.LineUp(1)
	case "ctrl+d":
		r.readmeViewport.HalfViewDown()
	case "ctrl+u":
		r.readmeViewport.HalfViewUp()
	case "g":
		r.readmeViewport.GotoTop()
	case "G":
		r.readmeViewport.GotoBottom()
	case "/":
		// Enter search mode
		r.readmeSearchMode = true
		r.readmeSearchQuery = ""
		return r, nil
	case "n":
		// Next match
		if len(r.readmeSearchMatches) > 0 {
			r.readmeSearchIdx = (r.readmeSearchIdx + 1) % len(r.readmeSearchMatches)
			r.scrollToMatch()
		}
	case "N":
		// Previous match
		if len(r.readmeSearchMatches) > 0 {
			r.readmeSearchIdx = (r.readmeSearchIdx - 1 + len(r.readmeSearchMatches)) % len(r.readmeSearchMatches)
			r.scrollToMatch()
		}
	}

	var cmd tea.Cmd
	r.readmeViewport, cmd = r.readmeViewport.Update(msg)
	return r, cmd
}

// performReadmeSearch searches for the query in README content
func (r *ReleaseDetail) performReadmeSearch() {
	if r.readmeSearchQuery == "" || r.readmeContent == "" {
		return
	}

	query := strings.ToLower(r.readmeSearchQuery)
	lines := strings.Split(r.readmeContent, "\n")
	r.readmeSearchMatches = nil
	r.readmeSearchIdx = 0

	// Find all lines containing the search query
	for i, line := range lines {
		if strings.Contains(strings.ToLower(line), query) {
			r.readmeSearchMatches = append(r.readmeSearchMatches, i)
		}
	}

	// Highlight matches in content
	r.highlightReadmeMatches()

	// Scroll to first match
	if len(r.readmeSearchMatches) > 0 {
		r.scrollToMatch()
	}
}

// highlightReadmeMatches highlights search matches in the README content
func (r *ReleaseDetail) highlightReadmeMatches() {
	if r.readmeSearchQuery == "" {
		r.readmeViewport.SetContent(r.readmeContent)
		return
	}

	query := strings.ToLower(r.readmeSearchQuery)
	lines := strings.Split(r.readmeContent, "\n")
	var highlighted []string

	for _, line := range lines {
		lowerLine := strings.ToLower(line)
		if strings.Contains(lowerLine, query) {
			// Highlight all occurrences in this line
			highlightedLine := r.highlightOccurrences(line, r.readmeSearchQuery)
			highlighted = append(highlighted, highlightedLine)
		} else {
			highlighted = append(highlighted, line)
		}
	}

	r.readmeHighlighted = strings.Join(highlighted, "\n")
	r.readmeViewport.SetContent(r.readmeHighlighted)
}

// highlightOccurrences highlights all occurrences of query in text (case-insensitive)
func (r *ReleaseDetail) highlightOccurrences(text, query string) string {
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
		result.WriteString(S.Highlighted.Render(matchText))

		lastEnd = lastEnd + idx + len(query)
	}

	return result.String()
}

// scrollToMatch scrolls the viewport to the current match
func (r *ReleaseDetail) scrollToMatch() {
	if len(r.readmeSearchMatches) == 0 {
		return
	}

	lineNum := r.readmeSearchMatches[r.readmeSearchIdx]
	// Position the match in the middle of the viewport if possible
	targetOffset := lineNum - r.readmeViewport.Height/2
	if targetOffset < 0 {
		targetOffset = 0
	}
	r.readmeViewport.SetYOffset(targetOffset)
}

// renderReadmeDialog renders the README viewer dialog
func (r *ReleaseDetail) renderReadmeDialog() string {
	// Calculate dialog dimensions
	dialogWidth := r.width - 10
	if dialogWidth > 120 {
		dialogWidth = 120
	}
	if dialogWidth < 60 {
		dialogWidth = 60
	}

	dialogHeight := r.height - 8
	if dialogHeight < 20 {
		dialogHeight = 20
	}

	// Update viewport dimensions
	r.readmeViewport.Width = dialogWidth - 6
	r.readmeViewport.Height = dialogHeight - 8

	dialogStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(DefaultTheme.Primary).
		Padding(1, 2).
		Width(dialogWidth)

	var content strings.Builder

	// Title
	title := Icons.Chart + " README: " + r.release.Name
	content.WriteString(S.Title.Render(title) + "\n")

	// Search bar or scroll info
	if r.readmeSearchMode {
		searchPrompt := S.Label.Render("/") + r.readmeSearchQuery + S.Muted.Render("_")
		content.WriteString(searchPrompt + "\n")
	} else if r.readmeSearchQuery != "" && len(r.readmeSearchMatches) > 0 {
		matchInfo := fmt.Sprintf(" [%d/%d matches for '%s']", r.readmeSearchIdx+1, len(r.readmeSearchMatches), r.readmeSearchQuery)
		content.WriteString(S.Success.Render(matchInfo) + "\n")
	} else if r.readmeSearchQuery != "" && len(r.readmeSearchMatches) == 0 {
		noMatch := fmt.Sprintf(" [No matches for '%s']", r.readmeSearchQuery)
		content.WriteString(S.Warning.Render(noMatch) + "\n")
	} else {
		scrollInfo := fmt.Sprintf(" (%.0f%%)", r.readmeViewport.ScrollPercent()*100)
		content.WriteString(S.Muted.Render(scrollInfo) + "\n")
	}

	content.WriteString(strings.Repeat("─", dialogWidth-6) + "\n")

	// Content
	if r.readmeLoading {
		content.WriteString(S.Muted.Render(Icons.Helm+" Loading README...") + "\n")
	} else if r.readmeError != nil {
		content.WriteString(S.Error.Render(Icons.Cross+" "+r.readmeError.Error()) + "\n")
	} else {
		content.WriteString(r.readmeViewport.View() + "\n")
	}

	content.WriteString(strings.Repeat("─", dialogWidth-6) + "\n")

	// Footer with keybindings
	if r.readmeSearchMode {
		content.WriteString(S.Muted.Render("Enter:search  Esc:cancel"))
	} else {
		content.WriteString(S.Muted.Render("/:search  n/N:next/prev  j/k:scroll  g/G:top/bottom  q:close"))
	}

	dialog := dialogStyle.Render(content.String())
	return lipgloss.Place(r.width, r.height, lipgloss.Center, lipgloss.Center, dialog)
}

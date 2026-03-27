package tui

import (
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// LoadingSpinner wraps the bubbles spinner with our styling
type LoadingSpinner struct {
	spinner spinner.Model
	message string
	active  bool
}

// NewLoadingSpinner creates a new loading spinner
func NewLoadingSpinner() LoadingSpinner {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(DefaultTheme.Primary)

	return LoadingSpinner{
		spinner: s,
		message: "Loading...",
	}
}

// Start activates the spinner with a message
func (l *LoadingSpinner) Start(message string) tea.Cmd {
	l.active = true
	l.message = message
	return l.spinner.Tick
}

// Stop deactivates the spinner
func (l *LoadingSpinner) Stop() {
	l.active = false
}

// IsActive returns whether the spinner is active
func (l *LoadingSpinner) IsActive() bool {
	return l.active
}

// Update handles spinner animation
func (l *LoadingSpinner) Update(msg tea.Msg) (LoadingSpinner, tea.Cmd) {
	if !l.active {
		return *l, nil
	}

	var cmd tea.Cmd
	l.spinner, cmd = l.spinner.Update(msg)
	return *l, cmd
}

// View renders the spinner
func (l *LoadingSpinner) View() string {
	if !l.active {
		return ""
	}
	return l.spinner.View() + " " + S.Muted.Render(l.message)
}

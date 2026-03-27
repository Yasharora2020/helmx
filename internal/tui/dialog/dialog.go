package dialog

import (
	tea "github.com/charmbracelet/bubbletea"
)

// Dialog is the interface that all dialogs must implement.
// Each dialog handles its own state, updates, and rendering.
type Dialog interface {
	// Update handles messages and returns updated state and commands.
	Update(msg tea.Msg) (Dialog, tea.Cmd)

	// View renders the dialog to a string.
	View() string

	// IsOpen returns true if the dialog is currently visible.
	IsOpen() bool

	// Open shows the dialog.
	Open()

	// Close hides the dialog.
	Close()
}

// BaseDialog provides common dialog functionality.
// Embed this in dialog implementations.
type BaseDialog struct {
	open   bool
	Width  int
	Height int
}

// IsOpen returns true if the dialog is open.
func (b *BaseDialog) IsOpen() bool {
	return b.open
}

// Open opens the dialog.
func (b *BaseDialog) Open() {
	b.open = true
}

// Close closes the dialog.
func (b *BaseDialog) Close() {
	b.open = false
}

// SetSize sets the dialog dimensions.
func (b *BaseDialog) SetSize(width, height int) {
	b.Width = width
	b.Height = height
}

// KeyHandler is a helper interface for dialogs that handle key events.
type KeyHandler interface {
	HandleKey(msg tea.KeyMsg) (Dialog, tea.Cmd)
}

// Focusable is an interface for dialogs with focusable inputs.
type Focusable interface {
	Focus()
	Blur()
}

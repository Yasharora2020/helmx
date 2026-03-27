// Package dialog provides dialog state management for the TUI.
package dialog

import (
	tea "github.com/charmbracelet/bubbletea"
)

// DialogID uniquely identifies a dialog type.
type DialogID string

// Common dialog IDs used across views.
const (
	DialogNone DialogID = ""

	// App-level dialogs
	DialogDelete         DialogID = "delete"
	DialogFeatureRequest DialogID = "feature_request"
	DialogBulkUpgrade    DialogID = "bulk_upgrade"

	// Release detail dialogs
	DialogUpgradeConfirm  DialogID = "upgrade_confirm"
	DialogRollbackConfirm DialogID = "rollback_confirm"
	DialogDiffPreview     DialogID = "diff_preview"
	DialogLogs            DialogID = "logs"
	DialogExec            DialogID = "exec"
	DialogRevisionCompare DialogID = "revision_compare"
	DialogPortForward     DialogID = "port_forward"

	// Explore view dialogs
	DialogInstall        DialogID = "install"
	DialogTemplate       DialogID = "template"
	DialogReadme         DialogID = "readme"
	DialogVersions       DialogID = "versions"
	DialogSecurity       DialogID = "security"
	DialogInstallPreview DialogID = "install_preview"
	DialogDepTree        DialogID = "dep_tree"
	DialogSchema         DialogID = "schema"
	DialogFileImport     DialogID = "file_import"
	DialogMultiChart     DialogID = "multi_chart"
	DialogSaveTemplate   DialogID = "save_template"
	DialogSaveStack      DialogID = "save_stack"
	DialogLoadStack      DialogID = "load_stack"
	DialogProgress       DialogID = "progress"

	// Repos view dialogs
	DialogAddRepo    DialogID = "add_repo"
	DialogDeleteRepo DialogID = "delete_repo"

	// Settings view dialogs
	DialogAddRegistry    DialogID = "add_registry"
	DialogDeleteRegistry DialogID = "delete_registry"
	DialogEditSettings   DialogID = "edit_settings"
	DialogTheme          DialogID = "theme"
	DialogContext        DialogID = "context"

	// RBAC view dialogs
	DialogAddUser      DialogID = "add_user"
	DialogEditUser     DialogID = "edit_user"
	DialogDeleteUser   DialogID = "delete_user"
	DialogAddNamespace DialogID = "add_namespace"
	DialogKubeconfig   DialogID = "kubeconfig"
)

// UpdateFunc is a function that handles dialog updates.
type UpdateFunc func(msg tea.Msg) (tea.Model, tea.Cmd)

// Manager manages dialog state for a view.
// Only one dialog can be open at a time.
type Manager struct {
	active DialogID
	stack  []DialogID // Stack for nested dialogs
}

// New creates a new dialog manager.
func New() *Manager {
	return &Manager{
		active: DialogNone,
		stack:  make([]DialogID, 0),
	}
}

// Open opens a dialog by ID. Closes any currently open dialog.
func (m *Manager) Open(id DialogID) {
	m.active = id
}

// OpenNested opens a nested dialog, pushing the current one onto the stack.
func (m *Manager) OpenNested(id DialogID) {
	if m.active != DialogNone {
		m.stack = append(m.stack, m.active)
	}
	m.active = id
}

// Close closes the currently open dialog.
// If there's a dialog on the stack, it becomes active.
func (m *Manager) Close() {
	if len(m.stack) > 0 {
		m.active = m.stack[len(m.stack)-1]
		m.stack = m.stack[:len(m.stack)-1]
	} else {
		m.active = DialogNone
	}
}

// CloseAll closes all dialogs including the stack.
func (m *Manager) CloseAll() {
	m.active = DialogNone
	m.stack = m.stack[:0]
}

// Active returns the currently active dialog ID.
func (m *Manager) Active() DialogID {
	return m.active
}

// IsOpen returns true if the specified dialog is currently open.
func (m *Manager) IsOpen(id DialogID) bool {
	return m.active == id
}

// IsAnyOpen returns true if any dialog is open.
func (m *Manager) IsAnyOpen() bool {
	return m.active != DialogNone
}

// HasOpenDialog is an alias for IsAnyOpen for API compatibility.
func (m *Manager) HasOpenDialog() bool {
	return m.IsAnyOpen()
}

// Toggle opens the dialog if closed, closes if open.
func (m *Manager) Toggle(id DialogID) {
	if m.active == id {
		m.Close()
	} else {
		m.Open(id)
	}
}

// IsOneOf returns true if the active dialog is one of the given IDs.
func (m *Manager) IsOneOf(ids ...DialogID) bool {
	for _, id := range ids {
		if m.active == id {
			return true
		}
	}
	return false
}

// StackDepth returns the number of dialogs on the stack.
func (m *Manager) StackDepth() int {
	return len(m.stack)
}

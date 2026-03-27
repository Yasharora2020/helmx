package dialog

import "testing"

func TestManagerNew(t *testing.T) {
	m := New()

	if m.IsAnyOpen() {
		t.Error("new Manager should have no open dialog")
	}
	if m.Active() != DialogNone {
		t.Errorf("Active() = %q, want %q", m.Active(), DialogNone)
	}
	if m.StackDepth() != 0 {
		t.Errorf("StackDepth() = %d, want 0", m.StackDepth())
	}
}

func TestManagerOpenClose(t *testing.T) {
	tests := []struct {
		name       string
		actions    func(m *Manager)
		wantActive DialogID
		wantOpen   bool
		wantDepth  int
	}{
		{
			name: "Open sets dialog active",
			actions: func(m *Manager) {
				m.Open(DialogDelete)
			},
			wantActive: DialogDelete,
			wantOpen:   true,
			wantDepth:  0,
		},
		{
			name: "Close after Open returns to none",
			actions: func(m *Manager) {
				m.Open(DialogDelete)
				m.Close()
			},
			wantActive: DialogNone,
			wantOpen:   false,
			wantDepth:  0,
		},
		{
			name: "Open replaces previously open dialog",
			actions: func(m *Manager) {
				m.Open(DialogDelete)
				m.Open(DialogInstall)
			},
			wantActive: DialogInstall,
			wantOpen:   true,
			wantDepth:  0,
		},
		{
			name: "Close when nothing is open is a no-op",
			actions: func(m *Manager) {
				m.Close()
			},
			wantActive: DialogNone,
			wantOpen:   false,
			wantDepth:  0,
		},
		{
			name: "multiple Close calls when nothing open",
			actions: func(m *Manager) {
				m.Close()
				m.Close()
				m.Close()
			},
			wantActive: DialogNone,
			wantOpen:   false,
			wantDepth:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := New()
			tt.actions(m)

			if got := m.Active(); got != tt.wantActive {
				t.Errorf("Active() = %q, want %q", got, tt.wantActive)
			}
			if got := m.IsAnyOpen(); got != tt.wantOpen {
				t.Errorf("IsAnyOpen() = %v, want %v", got, tt.wantOpen)
			}
			if got := m.StackDepth(); got != tt.wantDepth {
				t.Errorf("StackDepth() = %d, want %d", got, tt.wantDepth)
			}
		})
	}
}

func TestManagerOpenNested(t *testing.T) {
	tests := []struct {
		name       string
		actions    func(m *Manager)
		wantActive DialogID
		wantOpen   bool
		wantDepth  int
	}{
		{
			name: "OpenNested pushes current onto stack",
			actions: func(m *Manager) {
				m.Open(DialogInstall)
				m.OpenNested(DialogReadme)
			},
			wantActive: DialogReadme,
			wantOpen:   true,
			wantDepth:  1,
		},
		{
			name: "Close pops back to parent",
			actions: func(m *Manager) {
				m.Open(DialogInstall)
				m.OpenNested(DialogReadme)
				m.Close()
			},
			wantActive: DialogInstall,
			wantOpen:   true,
			wantDepth:  0,
		},
		{
			name: "double nested then close returns to middle",
			actions: func(m *Manager) {
				m.Open(DialogInstall)
				m.OpenNested(DialogReadme)
				m.OpenNested(DialogSchema)
			},
			wantActive: DialogSchema,
			wantOpen:   true,
			wantDepth:  2,
		},
		{
			name: "double nested close twice returns to root",
			actions: func(m *Manager) {
				m.Open(DialogInstall)
				m.OpenNested(DialogReadme)
				m.OpenNested(DialogSchema)
				m.Close()
				m.Close()
			},
			wantActive: DialogInstall,
			wantOpen:   true,
			wantDepth:  0,
		},
		{
			name: "double nested close three times closes all",
			actions: func(m *Manager) {
				m.Open(DialogInstall)
				m.OpenNested(DialogReadme)
				m.OpenNested(DialogSchema)
				m.Close()
				m.Close()
				m.Close()
			},
			wantActive: DialogNone,
			wantOpen:   false,
			wantDepth:  0,
		},
		{
			name: "OpenNested when nothing open just opens the dialog",
			actions: func(m *Manager) {
				m.OpenNested(DialogReadme)
			},
			wantActive: DialogReadme,
			wantOpen:   true,
			wantDepth:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := New()
			tt.actions(m)

			if got := m.Active(); got != tt.wantActive {
				t.Errorf("Active() = %q, want %q", got, tt.wantActive)
			}
			if got := m.IsAnyOpen(); got != tt.wantOpen {
				t.Errorf("IsAnyOpen() = %v, want %v", got, tt.wantOpen)
			}
			if got := m.StackDepth(); got != tt.wantDepth {
				t.Errorf("StackDepth() = %d, want %d", got, tt.wantDepth)
			}
		})
	}
}

func TestManagerCloseAll(t *testing.T) {
	tests := []struct {
		name    string
		actions func(m *Manager)
	}{
		{
			name: "CloseAll with single open dialog",
			actions: func(m *Manager) {
				m.Open(DialogInstall)
			},
		},
		{
			name: "CloseAll with nested dialogs",
			actions: func(m *Manager) {
				m.Open(DialogInstall)
				m.OpenNested(DialogReadme)
				m.OpenNested(DialogSchema)
			},
		},
		{
			name: "CloseAll when nothing is open",
			actions: func(m *Manager) {
				// no-op
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := New()
			tt.actions(m)
			m.CloseAll()

			if m.IsAnyOpen() {
				t.Error("IsAnyOpen() should be false after CloseAll()")
			}
			if got := m.Active(); got != DialogNone {
				t.Errorf("Active() = %q, want %q after CloseAll()", got, DialogNone)
			}
			if got := m.StackDepth(); got != 0 {
				t.Errorf("StackDepth() = %d, want 0 after CloseAll()", got)
			}
		})
	}
}

func TestManagerToggle(t *testing.T) {
	tests := []struct {
		name       string
		actions    func(m *Manager)
		wantActive DialogID
		wantOpen   bool
	}{
		{
			name: "Toggle opens closed dialog",
			actions: func(m *Manager) {
				m.Toggle(DialogDelete)
			},
			wantActive: DialogDelete,
			wantOpen:   true,
		},
		{
			name: "Toggle closes open dialog",
			actions: func(m *Manager) {
				m.Toggle(DialogDelete)
				m.Toggle(DialogDelete)
			},
			wantActive: DialogNone,
			wantOpen:   false,
		},
		{
			name: "Toggle different dialog replaces current",
			actions: func(m *Manager) {
				m.Toggle(DialogDelete)
				m.Toggle(DialogInstall)
			},
			wantActive: DialogInstall,
			wantOpen:   true,
		},
		{
			name: "Toggle three times ends open",
			actions: func(m *Manager) {
				m.Toggle(DialogDelete)
				m.Toggle(DialogDelete)
				m.Toggle(DialogDelete)
			},
			wantActive: DialogDelete,
			wantOpen:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := New()
			tt.actions(m)

			if got := m.Active(); got != tt.wantActive {
				t.Errorf("Active() = %q, want %q", got, tt.wantActive)
			}
			if got := m.IsAnyOpen(); got != tt.wantOpen {
				t.Errorf("IsAnyOpen() = %v, want %v", got, tt.wantOpen)
			}
		})
	}
}

func TestManagerIsOpen(t *testing.T) {
	m := New()
	m.Open(DialogDelete)

	if !m.IsOpen(DialogDelete) {
		t.Error("IsOpen(DialogDelete) should be true when delete dialog is active")
	}
	if m.IsOpen(DialogInstall) {
		t.Error("IsOpen(DialogInstall) should be false when delete dialog is active")
	}
	if m.IsOpen(DialogNone) {
		t.Error("IsOpen(DialogNone) should be false when a dialog is active")
	}
}

func TestManagerIsAnyOpen(t *testing.T) {
	m := New()

	if m.IsAnyOpen() {
		t.Error("IsAnyOpen() should be false on new Manager")
	}

	m.Open(DialogDelete)
	if !m.IsAnyOpen() {
		t.Error("IsAnyOpen() should be true when dialog is open")
	}

	m.Close()
	if m.IsAnyOpen() {
		t.Error("IsAnyOpen() should be false after closing")
	}
}

func TestManagerHasOpenDialog(t *testing.T) {
	m := New()

	if m.HasOpenDialog() {
		t.Error("HasOpenDialog() should be false on new Manager")
	}

	m.Open(DialogDelete)
	if !m.HasOpenDialog() {
		t.Error("HasOpenDialog() should be true when dialog is open")
	}

	// Verify it behaves identically to IsAnyOpen
	if m.HasOpenDialog() != m.IsAnyOpen() {
		t.Error("HasOpenDialog() and IsAnyOpen() should return the same value")
	}
}

func TestManagerIsOneOf(t *testing.T) {
	tests := []struct {
		name   string
		active DialogID
		check  []DialogID
		want   bool
	}{
		{
			name:   "matches first in list",
			active: DialogDelete,
			check:  []DialogID{DialogDelete, DialogInstall},
			want:   true,
		},
		{
			name:   "matches second in list",
			active: DialogInstall,
			check:  []DialogID{DialogDelete, DialogInstall},
			want:   true,
		},
		{
			name:   "matches single item",
			active: DialogDelete,
			check:  []DialogID{DialogDelete},
			want:   true,
		},
		{
			name:   "no match",
			active: DialogReadme,
			check:  []DialogID{DialogDelete, DialogInstall},
			want:   false,
		},
		{
			name:   "empty list returns false",
			active: DialogDelete,
			check:  []DialogID{},
			want:   false,
		},
		{
			name:   "no dialog open, checking DialogNone",
			active: DialogNone,
			check:  []DialogID{DialogNone},
			want:   true,
		},
		{
			name:   "no dialog open, checking non-empty list",
			active: DialogNone,
			check:  []DialogID{DialogDelete, DialogInstall},
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := New()
			if tt.active != DialogNone {
				m.Open(tt.active)
			}

			if got := m.IsOneOf(tt.check...); got != tt.want {
				t.Errorf("IsOneOf(%v) = %v, want %v", tt.check, got, tt.want)
			}
		})
	}
}

func TestManagerStackDepth(t *testing.T) {
	m := New()

	if got := m.StackDepth(); got != 0 {
		t.Errorf("StackDepth() = %d, want 0 on new Manager", got)
	}

	m.Open(DialogInstall)
	if got := m.StackDepth(); got != 0 {
		t.Errorf("StackDepth() = %d, want 0 after single Open", got)
	}

	m.OpenNested(DialogReadme)
	if got := m.StackDepth(); got != 1 {
		t.Errorf("StackDepth() = %d, want 1 after one nested open", got)
	}

	m.OpenNested(DialogSchema)
	if got := m.StackDepth(); got != 2 {
		t.Errorf("StackDepth() = %d, want 2 after two nested opens", got)
	}

	m.Close()
	if got := m.StackDepth(); got != 1 {
		t.Errorf("StackDepth() = %d, want 1 after one close from depth 2", got)
	}

	m.Close()
	if got := m.StackDepth(); got != 0 {
		t.Errorf("StackDepth() = %d, want 0 after closing back to root", got)
	}
}

func TestManagerActive(t *testing.T) {
	m := New()

	// Verify Active() at each nesting level
	if got := m.Active(); got != DialogNone {
		t.Errorf("Active() = %q, want %q on new Manager", got, DialogNone)
	}

	m.Open(DialogInstall)
	if got := m.Active(); got != DialogInstall {
		t.Errorf("Active() = %q, want %q", got, DialogInstall)
	}

	m.OpenNested(DialogReadme)
	if got := m.Active(); got != DialogReadme {
		t.Errorf("Active() = %q, want %q after OpenNested", got, DialogReadme)
	}

	m.OpenNested(DialogSchema)
	if got := m.Active(); got != DialogSchema {
		t.Errorf("Active() = %q, want %q after second OpenNested", got, DialogSchema)
	}

	m.Close()
	if got := m.Active(); got != DialogReadme {
		t.Errorf("Active() = %q, want %q after first Close", got, DialogReadme)
	}

	m.Close()
	if got := m.Active(); got != DialogInstall {
		t.Errorf("Active() = %q, want %q after second Close", got, DialogInstall)
	}

	m.Close()
	if got := m.Active(); got != DialogNone {
		t.Errorf("Active() = %q, want %q after final Close", got, DialogNone)
	}
}

func TestManagerEdgeCases(t *testing.T) {
	t.Run("nested then CloseAll then open again", func(t *testing.T) {
		m := New()
		m.Open(DialogInstall)
		m.OpenNested(DialogReadme)
		m.OpenNested(DialogSchema)

		m.CloseAll()

		// Should be able to open fresh dialogs after CloseAll
		m.Open(DialogDelete)
		if got := m.Active(); got != DialogDelete {
			t.Errorf("Active() = %q, want %q after CloseAll + Open", got, DialogDelete)
		}
		if got := m.StackDepth(); got != 0 {
			t.Errorf("StackDepth() = %d, want 0 after CloseAll + Open", got)
		}
	})

	t.Run("nested then close then open replaces without stacking", func(t *testing.T) {
		m := New()
		m.Open(DialogInstall)
		m.OpenNested(DialogReadme)
		m.Close() // back to DialogInstall

		// Open (not OpenNested) should replace, not stack
		m.Open(DialogDelete)
		if got := m.Active(); got != DialogDelete {
			t.Errorf("Active() = %q, want %q", got, DialogDelete)
		}
		// Stack should still be empty since Open doesn't push
		if got := m.StackDepth(); got != 0 {
			t.Errorf("StackDepth() = %d, want 0 after Open (not OpenNested)", got)
		}
	})

	t.Run("Toggle with nested stack uses Close which pops", func(t *testing.T) {
		m := New()
		m.Open(DialogInstall)
		m.OpenNested(DialogReadme)

		// Toggle the active dialog should Close (pop back to parent)
		m.Toggle(DialogReadme)
		if got := m.Active(); got != DialogInstall {
			t.Errorf("Active() = %q, want %q after Toggle pops nested", got, DialogInstall)
		}
		if got := m.StackDepth(); got != 0 {
			t.Errorf("StackDepth() = %d, want 0 after Toggle pops nested", got)
		}
	})

	t.Run("Open does not affect stack", func(t *testing.T) {
		m := New()
		m.Open(DialogInstall)
		m.OpenNested(DialogReadme)

		// Open replaces active but does NOT clear the stack
		m.Open(DialogDelete)
		if got := m.Active(); got != DialogDelete {
			t.Errorf("Active() = %q, want %q", got, DialogDelete)
		}
		// The stack still has DialogInstall from the earlier OpenNested
		if got := m.StackDepth(); got != 1 {
			t.Errorf("StackDepth() = %d, want 1 (stack preserved by Open)", got)
		}
	})

	t.Run("CloseAll is idempotent", func(t *testing.T) {
		m := New()
		m.CloseAll()
		m.CloseAll()
		m.CloseAll()

		if m.IsAnyOpen() {
			t.Error("IsAnyOpen() should be false after multiple CloseAll calls")
		}
		if got := m.StackDepth(); got != 0 {
			t.Errorf("StackDepth() = %d, want 0", got)
		}
	})
}

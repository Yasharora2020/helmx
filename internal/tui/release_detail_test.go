package tui

import (
	"testing"
)

func TestSupportedShellsContainsExpectedShells(t *testing.T) {
	// Validate that the supportedShells list contains the expected set of shells.
	// This catches accidental removals or reordering.
	expectedShells := []string{"/bin/bash", "/bin/sh", "/bin/zsh", "/bin/ash"}

	if len(supportedShells) != len(expectedShells) {
		t.Fatalf("expected %d shells, got %d: %v", len(expectedShells), len(supportedShells), supportedShells)
	}

	for i, shell := range expectedShells {
		if supportedShells[i] != shell {
			t.Errorf("supportedShells[%d] = %q, want %q", i, supportedShells[i], shell)
		}
	}
}

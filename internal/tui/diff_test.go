package tui

import (
	"fmt"
	"strings"
	"testing"
)

// Tests for the diff computation system (diff.go).
//
// The diff system uses a Longest Common Subsequence (LCS) algorithm to produce
// meaningful diffs between YAML values or Kubernetes manifests. This is used
// in the upgrade preview to show what will change.
//
// These tests validate:
// - ComputeDiff: Main diff function producing DiffResult with line-by-line changes
// - computeLCS: Core LCS algorithm finding common lines between two texts
// - trimTrailingEmpty: Utility to clean up trailing whitespace in line arrays
// - DiffSummary: Produces "+N -M" summary strings for display
//
// The LCS approach produces better diffs than simple line-by-line comparison
// by identifying moved or reordered content.

// TestComputeDiff validates the main diff computation for various scenarios:
// no changes, additions, removals, modifications, and edge cases (empty inputs).
func TestComputeDiff(t *testing.T) {
	tests := []struct {
		name         string
		old          string
		new          string
		hasChanges   bool
		addedCount   int
		removedCount int
	}{
		{
			name:       "no changes",
			old:        "line1\nline2\nline3",
			new:        "line1\nline2\nline3",
			hasChanges: false,
		},
		{
			name:         "single line added",
			old:          "line1\nline2",
			new:          "line1\nline2\nline3",
			hasChanges:   true,
			addedCount:   1,
			removedCount: 0,
		},
		{
			name:         "single line removed",
			old:          "line1\nline2\nline3",
			new:          "line1\nline2",
			hasChanges:   true,
			addedCount:   0,
			removedCount: 1,
		},
		{
			name:         "line modified",
			old:          "line1\nold-line\nline3",
			new:          "line1\nnew-line\nline3",
			hasChanges:   true,
			addedCount:   1,
			removedCount: 1,
		},
		{
			name:         "multiple changes",
			old:          "line1\nline2\nline3",
			new:          "line1\nmodified\nline3\nnew-line",
			hasChanges:   true,
			addedCount:   2,
			removedCount: 1,
		},
		{
			name:         "empty old",
			old:          "",
			new:          "line1\nline2",
			hasChanges:   true,
			addedCount:   2,
			removedCount: 0,
		},
		{
			name:         "empty new",
			old:          "line1\nline2",
			new:          "",
			hasChanges:   true,
			addedCount:   0,
			removedCount: 2,
		},
		{
			name:       "both empty",
			old:        "",
			new:        "",
			hasChanges: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ComputeDiff(tt.old, tt.new)

			if result.HasChanges != tt.hasChanges {
				t.Errorf("HasChanges = %v; want %v", result.HasChanges, tt.hasChanges)
			}

			if result.AddedCount != tt.addedCount {
				t.Errorf("AddedCount = %d; want %d", result.AddedCount, tt.addedCount)
			}

			if result.RemovedCount != tt.removedCount {
				t.Errorf("RemovedCount = %d; want %d", result.RemovedCount, tt.removedCount)
			}
		})
	}
}

// TestComputeDiff_LineTypes verifies that diff lines are correctly typed as
// DiffUnchanged, DiffAdded, or DiffRemoved with proper content preservation.
func TestComputeDiff_LineTypes(t *testing.T) {
	old := "unchanged\nremoved\nunchanged2"
	new := "unchanged\nadded\nunchanged2"

	result := ComputeDiff(old, new)

	// Check that we have the right types of lines
	hasUnchanged := false
	hasAdded := false
	hasRemoved := false

	for _, line := range result.Lines {
		switch line.Type {
		case DiffUnchanged:
			hasUnchanged = true
		case DiffAdded:
			hasAdded = true
			if line.Content != "added" {
				t.Errorf("Added line content = %q; want 'added'", line.Content)
			}
		case DiffRemoved:
			hasRemoved = true
			if line.Content != "removed" {
				t.Errorf("Removed line content = %q; want 'removed'", line.Content)
			}
		}
	}

	if !hasUnchanged {
		t.Error("Expected unchanged lines")
	}
	if !hasAdded {
		t.Error("Expected added lines")
	}
	if !hasRemoved {
		t.Error("Expected removed lines")
	}
}

// TestComputeLCS validates the Longest Common Subsequence algorithm.
// LCS finds the longest sequence of lines that appear in both inputs in order,
// which forms the basis for identifying unchanged lines in the diff.
func TestComputeLCS(t *testing.T) {
	tests := []struct {
		name     string
		a        []string
		b        []string
		expected []string
	}{
		{
			name:     "identical",
			a:        []string{"a", "b", "c"},
			b:        []string{"a", "b", "c"},
			expected: []string{"a", "b", "c"},
		},
		{
			name:     "one removed",
			a:        []string{"a", "b", "c"},
			b:        []string{"a", "c"},
			expected: []string{"a", "c"},
		},
		{
			name:     "one added",
			a:        []string{"a", "c"},
			b:        []string{"a", "b", "c"},
			expected: []string{"a", "c"},
		},
		{
			name:     "completely different",
			a:        []string{"a", "b"},
			b:        []string{"c", "d"},
			expected: []string{},
		},
		{
			name:     "empty first",
			a:        []string{},
			b:        []string{"a", "b"},
			expected: []string{},
		},
		{
			name:     "empty second",
			a:        []string{"a", "b"},
			b:        []string{},
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := computeLCS(tt.a, tt.b)

			if len(result) != len(tt.expected) {
				t.Errorf("LCS length = %d; want %d", len(result), len(tt.expected))
				return
			}

			for i, v := range result {
				if v != tt.expected[i] {
					t.Errorf("LCS[%d] = %q; want %q", i, v, tt.expected[i])
				}
			}
		})
	}
}

// TestTrimTrailingEmpty verifies cleanup of trailing empty/whitespace lines.
// This prevents spurious diff noise from trailing newlines in YAML content.
func TestTrimTrailingEmpty(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected []string
	}{
		{
			name:     "no trailing",
			input:    []string{"a", "b", "c"},
			expected: []string{"a", "b", "c"},
		},
		{
			name:     "one trailing empty",
			input:    []string{"a", "b", ""},
			expected: []string{"a", "b"},
		},
		{
			name:     "multiple trailing empty",
			input:    []string{"a", "b", "", "", ""},
			expected: []string{"a", "b"},
		},
		{
			name:     "trailing whitespace",
			input:    []string{"a", "b", "   ", "\t"},
			expected: []string{"a", "b"},
		},
		{
			name:     "all empty",
			input:    []string{"", "", ""},
			expected: []string{},
		},
		{
			name:     "empty input",
			input:    []string{},
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := trimTrailingEmpty(tt.input)

			if len(result) != len(tt.expected) {
				t.Errorf("length = %d; want %d", len(result), len(tt.expected))
				return
			}

			for i, v := range result {
				if v != tt.expected[i] {
					t.Errorf("[%d] = %q; want %q", i, v, tt.expected[i])
				}
			}
		})
	}
}

// stripAnsi removes ANSI escape codes from a string for content assertions.
// Lipgloss uses ANSI styling; stripping allows comparing plain text content.
func stripAnsi(s string) string {
	var result []byte
	i := 0
	for i < len(s) {
		if s[i] == '\x1b' {
			// Skip until we find the terminating letter
			for i < len(s) && !((s[i] >= 'A' && s[i] <= 'Z') || (s[i] >= 'a' && s[i] <= 'z')) {
				i++
			}
			if i < len(s) {
				i++ // skip the terminating letter
			}
		} else {
			result = append(result, s[i])
			i++
		}
	}
	return string(result)
}

// TestRenderDiff_NoChanges verifies that identical inputs produce a "No changes" message.
func TestRenderDiff_NoChanges(t *testing.T) {
	diff := ComputeDiff("line1\nline2\nline3", "line1\nline2\nline3")
	rendered := RenderDiff(diff, 3)
	if !strings.Contains(rendered, "No changes") {
		t.Errorf("expected 'No changes' for identical inputs, got: %q", rendered)
	}
}

// TestRenderDiff_SingleAddition verifies that a single added line shows a "+" prefix.
func TestRenderDiff_SingleAddition(t *testing.T) {
	diff := ComputeDiff("line1\nline2", "line1\nline2\nline3")
	rendered := RenderDiff(diff, 3)
	plain := stripAnsi(rendered)

	if !strings.Contains(plain, "+ line3") {
		t.Errorf("expected '+ line3' in output, got: %q", plain)
	}
	// Should not contain any "-" change lines (only the unchanged context uses " ")
	if strings.Contains(plain, "- ") {
		t.Errorf("unexpected removal line in output: %q", plain)
	}
}

// TestRenderDiff_SingleDeletion verifies that a single removed line shows a "-" prefix.
func TestRenderDiff_SingleDeletion(t *testing.T) {
	diff := ComputeDiff("line1\nline2\nline3", "line1\nline2")
	rendered := RenderDiff(diff, 3)
	plain := stripAnsi(rendered)

	if !strings.Contains(plain, "- line3") {
		t.Errorf("expected '- line3' in output, got: %q", plain)
	}
	if strings.Contains(plain, "+ ") {
		t.Errorf("unexpected addition line in output: %q", plain)
	}
}

// TestRenderDiff_MixedChanges verifies that modifications produce both "+" and "-" lines
// with correct context lines around them.
func TestRenderDiff_MixedChanges(t *testing.T) {
	old := "header\nold-value\nfooter"
	new := "header\nnew-value\nfooter"
	diff := ComputeDiff(old, new)
	rendered := RenderDiff(diff, 3)
	plain := stripAnsi(rendered)

	if !strings.Contains(plain, "- old-value") {
		t.Errorf("expected '- old-value' in output, got: %q", plain)
	}
	if !strings.Contains(plain, "+ new-value") {
		t.Errorf("expected '+ new-value' in output, got: %q", plain)
	}
	// Context lines: "header" and "footer" should appear as unchanged context
	if !strings.Contains(plain, "header") {
		t.Errorf("expected context line 'header' in output, got: %q", plain)
	}
	if !strings.Contains(plain, "footer") {
		t.Errorf("expected context line 'footer' in output, got: %q", plain)
	}
}

// TestRenderDiff_ZeroContext verifies that context=0 shows only changed lines
// (no surrounding unchanged lines).
func TestRenderDiff_ZeroContext(t *testing.T) {
	old := "before\nold-value\nafter"
	new := "before\nnew-value\nafter"
	diff := ComputeDiff(old, new)
	rendered := RenderDiff(diff, 0)
	plain := stripAnsi(rendered)

	// With 0 context, unchanged lines should not appear
	if strings.Contains(plain, "before") {
		t.Errorf("context=0 should not include unchanged 'before', got: %q", plain)
	}
	if strings.Contains(plain, "after") {
		t.Errorf("context=0 should not include unchanged 'after', got: %q", plain)
	}
	// Changed lines should still appear
	if !strings.Contains(plain, "- old-value") {
		t.Errorf("expected '- old-value' with context=0, got: %q", plain)
	}
	if !strings.Contains(plain, "+ new-value") {
		t.Errorf("expected '+ new-value' with context=0, got: %q", plain)
	}
}

// TestRenderDiff_ContextLines verifies that context parameter controls
// how many surrounding unchanged lines appear around changes.
func TestRenderDiff_ContextLines(t *testing.T) {
	// Build content with many lines, single change in the middle
	var oldLines, newLines []string
	for i := 0; i < 20; i++ {
		oldLines = append(oldLines, fmt.Sprintf("line%d", i))
		newLines = append(newLines, fmt.Sprintf("line%d", i))
	}
	// Change line 10
	oldLines[10] = "old-middle"
	newLines[10] = "new-middle"

	diff := ComputeDiff(strings.Join(oldLines, "\n"), strings.Join(newLines, "\n"))

	// With context=1, only line9 and line11 should appear as context
	rendered := RenderDiff(diff, 1)
	plain := stripAnsi(rendered)

	if !strings.Contains(plain, "line9") {
		t.Errorf("context=1 should include line9, got: %q", plain)
	}
	if !strings.Contains(plain, "line11") {
		t.Errorf("context=1 should include line11, got: %q", plain)
	}
	// line7 is 3 lines away from the change -- should not appear with context=1
	if strings.Contains(plain, "line7") {
		t.Errorf("context=1 should not include line7, got: %q", plain)
	}
}

// TestRenderDiff_GapSeparator verifies that non-adjacent changes produce a
// gap separator ("───") between the change hunks.
func TestRenderDiff_GapSeparator(t *testing.T) {
	// Two changes separated by many unchanged lines
	var oldLines, newLines []string
	for i := 0; i < 20; i++ {
		oldLines = append(oldLines, fmt.Sprintf("line%d", i))
		newLines = append(newLines, fmt.Sprintf("line%d", i))
	}
	oldLines[2] = "old-top"
	newLines[2] = "new-top"
	oldLines[17] = "old-bottom"
	newLines[17] = "new-bottom"

	diff := ComputeDiff(strings.Join(oldLines, "\n"), strings.Join(newLines, "\n"))
	rendered := RenderDiff(diff, 1)
	plain := stripAnsi(rendered)

	// Both changes should appear
	if !strings.Contains(plain, "- old-top") {
		t.Errorf("expected '- old-top', got: %q", plain)
	}
	if !strings.Contains(plain, "+ new-bottom") {
		t.Errorf("expected '+ new-bottom', got: %q", plain)
	}
	// The gap separator (rendered as "───") should appear between hunks
	if !strings.Contains(plain, "───") {
		t.Errorf("expected gap separator '───' between non-adjacent changes, got: %q", plain)
	}
}

// TestRenderDiff_LineNumbers verifies that rendered output contains line numbers.
func TestRenderDiff_LineNumbers(t *testing.T) {
	diff := ComputeDiff("line1\nold\nline3", "line1\nnew\nline3")
	rendered := RenderDiff(diff, 3)
	plain := stripAnsi(rendered)

	// Line numbers should be present (e.g. "  1", "  2", "  3")
	if !strings.Contains(plain, "1") {
		t.Errorf("expected line number 1 in output, got: %q", plain)
	}
	if !strings.Contains(plain, "2") {
		t.Errorf("expected line number 2 in output, got: %q", plain)
	}
}

// TestRenderCompactDiff_NoChanges verifies that compact diff with no changes shows "No changes".
func TestRenderCompactDiff_NoChanges(t *testing.T) {
	diff := ComputeDiff("same\ncontent", "same\ncontent")
	rendered := RenderCompactDiff(diff, 10)
	if !strings.Contains(rendered, "No changes") {
		t.Errorf("expected 'No changes', got: %q", rendered)
	}
}

// TestRenderCompactDiff_ShowsOnlyChanges verifies that compact diff shows only
// added/removed lines, not unchanged context lines.
func TestRenderCompactDiff_ShowsOnlyChanges(t *testing.T) {
	diff := ComputeDiff("ctx\nold\nctx2", "ctx\nnew\nctx2")
	rendered := RenderCompactDiff(diff, 10)
	plain := stripAnsi(rendered)

	if !strings.Contains(plain, "- old") {
		t.Errorf("expected '- old' in compact diff, got: %q", plain)
	}
	if !strings.Contains(plain, "+ new") {
		t.Errorf("expected '+ new' in compact diff, got: %q", plain)
	}
	// Unchanged lines should not appear
	if strings.Contains(plain, "ctx2") {
		t.Errorf("compact diff should not show unchanged 'ctx2', got: %q", plain)
	}
}

// TestRenderCompactDiff_Truncation verifies that compact diff truncates when
// there are more changes than maxLines, showing a "... and N more changes" message.
func TestRenderCompactDiff_Truncation(t *testing.T) {
	old := "a\nb\nc\nd\ne"
	new := "v\nw\nx\ny\nz"
	diff := ComputeDiff(old, new)

	// Allow only 2 lines -- there are 10 total changes (5 removed + 5 added)
	rendered := RenderCompactDiff(diff, 2)
	plain := stripAnsi(rendered)

	if !strings.Contains(plain, "more changes") {
		t.Errorf("expected truncation message with 'more changes', got: %q", plain)
	}

	// Count rendered change lines (before the truncation message)
	changeLines := 0
	for _, line := range strings.Split(plain, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "+") || strings.HasPrefix(trimmed, "-") {
			changeLines++
		}
	}
	if changeLines != 2 {
		t.Errorf("expected exactly 2 change lines before truncation, got %d", changeLines)
	}
}

// TestRenderCompactDiff_NoTruncation verifies that compact diff shows all changes
// when they fit within maxLines.
func TestRenderCompactDiff_NoTruncation(t *testing.T) {
	diff := ComputeDiff("old1\nold2", "new1\nnew2")
	// 4 changes total (2 removed + 2 added), maxLines=10 should fit all
	rendered := RenderCompactDiff(diff, 10)
	plain := stripAnsi(rendered)

	if strings.Contains(plain, "more changes") {
		t.Errorf("should not truncate when all changes fit, got: %q", plain)
	}
	if !strings.Contains(plain, "- old1") {
		t.Errorf("expected '- old1', got: %q", plain)
	}
	if !strings.Contains(plain, "+ new2") {
		t.Errorf("expected '+ new2', got: %q", plain)
	}
}

// TestDiffSummary validates the "+N -M" summary string generation for diff headers.
func TestDiffSummary(t *testing.T) {
	tests := []struct {
		name   string
		diff   DiffResult
		wantNz bool // whether result should be non-zero length
	}{
		{
			name:   "no changes",
			diff:   DiffResult{HasChanges: false},
			wantNz: true, // "No changes"
		},
		{
			name:   "only additions",
			diff:   DiffResult{HasChanges: true, AddedCount: 5, RemovedCount: 0},
			wantNz: true,
		},
		{
			name:   "only removals",
			diff:   DiffResult{HasChanges: true, AddedCount: 0, RemovedCount: 3},
			wantNz: true,
		},
		{
			name:   "both additions and removals",
			diff:   DiffResult{HasChanges: true, AddedCount: 2, RemovedCount: 4},
			wantNz: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DiffSummary(tt.diff)

			if tt.wantNz && len(result) == 0 {
				t.Error("expected non-empty result")
			}

			// Check that additions are represented with +
			if tt.diff.AddedCount > 0 && !strings.Contains(result, "+") {
				t.Error("expected + in result for additions")
			}

			// Check that removals are represented with -
			if tt.diff.RemovedCount > 0 && !strings.Contains(result, "-") {
				t.Error("expected - in result for removals")
			}
		})
	}
}

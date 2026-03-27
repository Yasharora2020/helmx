package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// DiffLine represents a line in the diff output
type DiffLine struct {
	Type    DiffType
	Content string
	OldNum  int // Line number in old content (0 if added)
	NewNum  int // Line number in new content (0 if removed)
}

// DiffType represents the type of diff line
type DiffType int

const (
	DiffUnchanged DiffType = iota
	DiffAdded
	DiffRemoved
	DiffHeader
)

// DiffResult contains the diff output
type DiffResult struct {
	Lines        []DiffLine
	AddedCount   int
	RemovedCount int
	HasChanges   bool
}

// ComputeDiff computes a diff between old and new content
// Uses a simple line-based diff algorithm
func ComputeDiff(old, new string) DiffResult {
	oldLines := strings.Split(old, "\n")
	newLines := strings.Split(new, "\n")

	// Trim trailing empty lines
	oldLines = trimTrailingEmpty(oldLines)
	newLines = trimTrailingEmpty(newLines)

	result := DiffResult{
		Lines: []DiffLine{},
	}

	// Use longest common subsequence approach for better diffs
	lcs := computeLCS(oldLines, newLines)

	oldIdx, newIdx := 0, 0
	lcsIdx := 0

	for oldIdx < len(oldLines) || newIdx < len(newLines) {
		// If we've matched all LCS elements, remaining are adds/removes
		if lcsIdx >= len(lcs) {
			// Add remaining old lines as removed
			for oldIdx < len(oldLines) {
				result.Lines = append(result.Lines, DiffLine{
					Type:    DiffRemoved,
					Content: oldLines[oldIdx],
					OldNum:  oldIdx + 1,
				})
				result.RemovedCount++
				oldIdx++
			}
			// Add remaining new lines as added
			for newIdx < len(newLines) {
				result.Lines = append(result.Lines, DiffLine{
					Type:    DiffAdded,
					Content: newLines[newIdx],
					NewNum:  newIdx + 1,
				})
				result.AddedCount++
				newIdx++
			}
			break
		}

		// Current LCS element
		lcsLine := lcs[lcsIdx]

		// Remove old lines until we hit the LCS line
		for oldIdx < len(oldLines) && oldLines[oldIdx] != lcsLine {
			result.Lines = append(result.Lines, DiffLine{
				Type:    DiffRemoved,
				Content: oldLines[oldIdx],
				OldNum:  oldIdx + 1,
			})
			result.RemovedCount++
			oldIdx++
		}

		// Add new lines until we hit the LCS line
		for newIdx < len(newLines) && newLines[newIdx] != lcsLine {
			result.Lines = append(result.Lines, DiffLine{
				Type:    DiffAdded,
				Content: newLines[newIdx],
				NewNum:  newIdx + 1,
			})
			result.AddedCount++
			newIdx++
		}

		// Add the unchanged LCS line
		if oldIdx < len(oldLines) && newIdx < len(newLines) {
			result.Lines = append(result.Lines, DiffLine{
				Type:    DiffUnchanged,
				Content: oldLines[oldIdx],
				OldNum:  oldIdx + 1,
				NewNum:  newIdx + 1,
			})
			oldIdx++
			newIdx++
			lcsIdx++
		}
	}

	result.HasChanges = result.AddedCount > 0 || result.RemovedCount > 0
	return result
}

// computeLCS computes the longest common subsequence of two string slices
func computeLCS(a, b []string) []string {
	m, n := len(a), len(b)

	// DP table
	dp := make([][]int, m+1)
	for i := range dp {
		dp[i] = make([]int, n+1)
	}

	// Fill the table
	for i := 1; i <= m; i++ {
		for j := 1; j <= n; j++ {
			if a[i-1] == b[j-1] {
				dp[i][j] = dp[i-1][j-1] + 1
			} else {
				dp[i][j] = max(dp[i-1][j], dp[i][j-1])
			}
		}
	}

	// Backtrack to find LCS (build in reverse, then flip)
	var lcsReverse []string
	i, j := m, n
	for i > 0 && j > 0 {
		if a[i-1] == b[j-1] {
			lcsReverse = append(lcsReverse, a[i-1])
			i--
			j--
		} else if dp[i-1][j] > dp[i][j-1] {
			i--
		} else {
			j--
		}
	}

	// Reverse to get correct order
	lcs := make([]string, len(lcsReverse))
	for k := range lcsReverse {
		lcs[len(lcsReverse)-1-k] = lcsReverse[k]
	}

	return lcs
}

func trimTrailingEmpty(lines []string) []string {
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

// RenderDiff renders a diff result with colors
func RenderDiff(diff DiffResult, contextLines int) string {
	if !diff.HasChanges {
		return S.Muted.Render("No changes")
	}

	addStyle := lipgloss.NewStyle().Foreground(DefaultTheme.Success)
	removeStyle := lipgloss.NewStyle().Foreground(DefaultTheme.Error)
	unchangedStyle := lipgloss.NewStyle().Foreground(DefaultTheme.Muted)
	lineNumStyle := lipgloss.NewStyle().Foreground(DefaultTheme.Muted).Width(4)

	var result []string
	var lastShownIdx = -1

	for i, line := range diff.Lines {
		// Determine if we should show this line
		showLine := false

		if line.Type != DiffUnchanged {
			showLine = true
		} else {
			// Check if within context of a change
			for j := max(0, i-contextLines); j <= min(len(diff.Lines)-1, i+contextLines); j++ {
				if diff.Lines[j].Type != DiffUnchanged {
					showLine = true
					break
				}
			}
		}

		if !showLine {
			continue
		}

		// Add separator if there's a gap
		if lastShownIdx >= 0 && i > lastShownIdx+1 {
			result = append(result, S.Muted.Render("───"))
		}
		lastShownIdx = i

		var prefix string
		var style lipgloss.Style
		lineNum := ""

		switch line.Type {
		case DiffAdded:
			prefix = "+"
			style = addStyle
			if line.NewNum > 0 {
				lineNum = fmt.Sprintf("%3d", line.NewNum)
			}
		case DiffRemoved:
			prefix = "-"
			style = removeStyle
			if line.OldNum > 0 {
				lineNum = fmt.Sprintf("%3d", line.OldNum)
			}
		case DiffUnchanged:
			prefix = " "
			style = unchangedStyle
			if line.NewNum > 0 {
				lineNum = fmt.Sprintf("%3d", line.NewNum)
			}
		}

		renderedLine := lineNumStyle.Render(lineNum) + " " + style.Render(prefix+" "+line.Content)
		result = append(result, renderedLine)
	}

	return strings.Join(result, "\n")
}

// RenderCompactDiff renders a compact diff showing only changes
func RenderCompactDiff(diff DiffResult, maxLines int) string {
	if !diff.HasChanges {
		return S.Muted.Render("No changes")
	}

	addStyle := lipgloss.NewStyle().Foreground(DefaultTheme.Success)
	removeStyle := lipgloss.NewStyle().Foreground(DefaultTheme.Error)

	var result []string
	lineCount := 0

	for _, line := range diff.Lines {
		if lineCount >= maxLines {
			remaining := diff.AddedCount + diff.RemovedCount - lineCount
			if remaining > 0 {
				result = append(result, S.Muted.Render(fmt.Sprintf("... and %d more changes", remaining)))
			}
			break
		}

		switch line.Type {
		case DiffAdded:
			result = append(result, addStyle.Render("+ "+line.Content))
			lineCount++
		case DiffRemoved:
			result = append(result, removeStyle.Render("- "+line.Content))
			lineCount++
		}
	}

	return strings.Join(result, "\n")
}

// DiffSummary returns a summary string for the diff
func DiffSummary(diff DiffResult) string {
	if !diff.HasChanges {
		return S.Muted.Render("No changes")
	}

	var parts []string
	if diff.AddedCount > 0 {
		parts = append(parts, S.Success.Render(fmt.Sprintf("+%d", diff.AddedCount)))
	}
	if diff.RemovedCount > 0 {
		parts = append(parts, S.Error.Render(fmt.Sprintf("-%d", diff.RemovedCount)))
	}
	return strings.Join(parts, " ")
}

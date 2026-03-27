package helm

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
)

// TrivyError represents errors from Trivy operations
type TrivyError struct {
	Op      string // Operation: "detect", "scan"
	Err     error
	Details string
}

func (e *TrivyError) Error() string {
	if e.Details != "" {
		return fmt.Sprintf("%s: %v - %s", e.Op, e.Err, e.Details)
	}
	return fmt.Sprintf("%s: %v", e.Op, e.Err)
}

func (e *TrivyError) Unwrap() error {
	return e.Err
}

// ErrTrivyNotInstalled is returned when Trivy is not available
var ErrTrivyNotInstalled = errors.New("trivy not installed")

// TrivySeverity represents vulnerability severity levels
type TrivySeverity string

const (
	SeverityCritical TrivySeverity = "CRITICAL"
	SeverityHigh     TrivySeverity = "HIGH"
	SeverityMedium   TrivySeverity = "MEDIUM"
	SeverityLow      TrivySeverity = "LOW"
)

// TrivyMisconfiguration represents a single security finding
type TrivyMisconfiguration struct {
	ID          string `json:"ID"`
	Title       string `json:"Title"`
	Severity    string `json:"Severity"`
	Description string `json:"Description"`
	Resolution  string `json:"Resolution"`
	Message     string `json:"Message"`
}

// TrivyResult represents results for a single target file
type TrivyResult struct {
	Target            string                  `json:"Target"`
	Misconfigurations []TrivyMisconfiguration `json:"Misconfigurations"`
}

// TrivyScanOutput represents the full Trivy JSON output
type TrivyScanOutput struct {
	Results []TrivyResult `json:"Results"`
}

// TrivyScanSummary provides a severity breakdown
type TrivyScanSummary struct {
	Critical int
	High     int
	Medium   int
	Low      int
	Total    int
	Results  []TrivyResult
}

// TrivyClient wraps Trivy CLI operations
type TrivyClient struct {
	version   string
	available bool
	checked   bool
	mu        sync.RWMutex
}

// NewTrivyClient creates a new Trivy client
func NewTrivyClient() *TrivyClient {
	return &TrivyClient{}
}

// IsAvailable checks if Trivy is installed (with double-checked locking)
func (t *TrivyClient) IsAvailable() bool {
	t.mu.RLock()
	if t.checked {
		available := t.available
		t.mu.RUnlock()
		return available
	}
	t.mu.RUnlock()

	t.mu.Lock()
	defer t.mu.Unlock()

	// Double-check after acquiring write lock
	if t.checked {
		return t.available
	}

	t.checked = true
	cmd := exec.Command("trivy", "version")
	output, err := cmd.Output()
	if err != nil {
		t.available = false
		return false
	}

	t.available = true
	// Parse version from output (first line usually contains version)
	lines := strings.Split(string(output), "\n")
	if len(lines) > 0 {
		t.version = strings.TrimSpace(lines[0])
	}
	return true
}

// GetVersion returns the Trivy version string
func (t *TrivyClient) GetVersion() string {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.version
}

// ScanRenderedManifest scans rendered YAML content for misconfigurations
func (t *TrivyClient) ScanRenderedManifest(yamlContent string) (*TrivyScanSummary, error) {
	if !t.IsAvailable() {
		return nil, &TrivyError{
			Op:      "scan",
			Err:     ErrTrivyNotInstalled,
			Details: "Install Trivy: https://trivy.dev/docs/latest/getting-started/installation/",
		}
	}

	// Write to temp file
	tmpFile, err := os.CreateTemp("", "helmx-trivy-scan-*.yaml")
	if err != nil {
		return nil, &TrivyError{Op: "scan", Err: err}
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	if _, err := tmpFile.WriteString(yamlContent); err != nil {
		_ = tmpFile.Close()
		return nil, &TrivyError{Op: "scan", Err: err}
	}
	_ = tmpFile.Close()

	cmd := exec.Command("trivy", "config", tmpPath,
		"--format", "json",
		"--severity", "CRITICAL,HIGH,MEDIUM,LOW",
	)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		// Trivy returns exit code 1 if findings exist, which is not an error
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			// Findings found - continue processing
		} else {
			return nil, &TrivyError{
				Op:      "scan",
				Err:     err,
				Details: parseTrivyError(stderr.String()),
			}
		}
	}

	return t.parseResults(stdout.Bytes())
}

// ScanChartDirectory scans a chart directory for misconfigurations
func (t *TrivyClient) ScanChartDirectory(chartPath string) (*TrivyScanSummary, error) {
	if !t.IsAvailable() {
		return nil, &TrivyError{
			Op:      "scan",
			Err:     ErrTrivyNotInstalled,
			Details: "Install Trivy: https://trivy.dev/docs/latest/getting-started/installation/",
		}
	}

	cmd := exec.Command("trivy", "config", chartPath,
		"--format", "json",
		"--severity", "CRITICAL,HIGH,MEDIUM,LOW",
	)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		// Trivy returns exit code 1 if findings exist, which is not an error
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			// Findings found - continue processing
		} else {
			return nil, &TrivyError{
				Op:      "scan",
				Err:     err,
				Details: parseTrivyError(stderr.String()),
			}
		}
	}

	return t.parseResults(stdout.Bytes())
}

// parseResults parses Trivy JSON output into a summary
func (t *TrivyClient) parseResults(data []byte) (*TrivyScanSummary, error) {
	// Handle empty output
	if len(data) == 0 {
		return &TrivyScanSummary{}, nil
	}

	var output TrivyScanOutput
	if err := json.Unmarshal(data, &output); err != nil {
		return nil, &TrivyError{Op: "scan", Err: err, Details: "failed to parse Trivy output"}
	}

	summary := &TrivyScanSummary{
		Results: output.Results,
	}

	for _, result := range output.Results {
		for _, misconfig := range result.Misconfigurations {
			summary.Total++
			switch TrivySeverity(misconfig.Severity) {
			case SeverityCritical:
				summary.Critical++
			case SeverityHigh:
				summary.High++
			case SeverityMedium:
				summary.Medium++
			case SeverityLow:
				summary.Low++
			}
		}
	}

	return summary, nil
}

// parseTrivyError extracts meaningful error messages from Trivy stderr
func parseTrivyError(stderr string) string {
	if strings.Contains(stderr, "no such file or directory") {
		return "Chart path not found"
	}
	if strings.Contains(stderr, "permission denied") {
		return "Permission denied accessing chart files"
	}
	if strings.Contains(stderr, "timeout") {
		return "Scan timed out - try again"
	}
	if stderr != "" {
		// Return first line of stderr, trimmed
		lines := strings.Split(stderr, "\n")
		if len(lines) > 0 && lines[0] != "" {
			return strings.TrimSpace(lines[0])
		}
		return stderr
	}
	return "Unknown error"
}

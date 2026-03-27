package helm

import (
	"testing"
)

func TestParseTrivyError(t *testing.T) {
	tests := []struct {
		name     string
		stderr   string
		expected string
	}{
		{
			name:     "file not found",
			stderr:   "no such file or directory",
			expected: "Chart path not found",
		},
		{
			name:     "permission denied",
			stderr:   "permission denied accessing file",
			expected: "Permission denied accessing chart files",
		},
		{
			name:     "timeout",
			stderr:   "timeout while scanning",
			expected: "Scan timed out - try again",
		},
		{
			name:     "empty stderr",
			stderr:   "",
			expected: "Unknown error",
		},
		{
			name:     "other error",
			stderr:   "some other error message\nwith multiple lines",
			expected: "some other error message",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseTrivyError(tt.stderr)
			if result != tt.expected {
				t.Errorf("parseTrivyError(%q) = %q, want %q", tt.stderr, result, tt.expected)
			}
		})
	}
}

func TestTrivyScanSummaryCalculation(t *testing.T) {
	client := NewTrivyClient()

	// Test with empty results
	summary, err := client.parseResults([]byte(`{"Results": []}`))
	if err != nil {
		t.Fatalf("parseResults failed: %v", err)
	}
	if summary.Total != 0 {
		t.Errorf("Expected Total=0 for empty results, got %d", summary.Total)
	}

	// Test with sample results
	sampleJSON := `{
		"Results": [
			{
				"Target": "deployment.yaml",
				"Misconfigurations": [
					{"ID": "KSV001", "Title": "Privileged", "Severity": "CRITICAL"},
					{"ID": "KSV002", "Title": "HostNetwork", "Severity": "HIGH"},
					{"ID": "KSV003", "Title": "Default SA", "Severity": "MEDIUM"},
					{"ID": "KSV004", "Title": "Labels", "Severity": "LOW"}
				]
			},
			{
				"Target": "service.yaml",
				"Misconfigurations": [
					{"ID": "KSV005", "Title": "Another Critical", "Severity": "CRITICAL"}
				]
			}
		]
	}`

	summary, err = client.parseResults([]byte(sampleJSON))
	if err != nil {
		t.Fatalf("parseResults failed: %v", err)
	}

	if summary.Total != 5 {
		t.Errorf("Expected Total=5, got %d", summary.Total)
	}
	if summary.Critical != 2 {
		t.Errorf("Expected Critical=2, got %d", summary.Critical)
	}
	if summary.High != 1 {
		t.Errorf("Expected High=1, got %d", summary.High)
	}
	if summary.Medium != 1 {
		t.Errorf("Expected Medium=1, got %d", summary.Medium)
	}
	if summary.Low != 1 {
		t.Errorf("Expected Low=1, got %d", summary.Low)
	}
	if len(summary.Results) != 2 {
		t.Errorf("Expected 2 result files, got %d", len(summary.Results))
	}
}

func TestTrivyClientNew(t *testing.T) {
	client := NewTrivyClient()
	if client == nil {
		t.Fatal("NewTrivyClient returned nil")
	}
	// Initial state should be unchecked
	if client.checked {
		t.Error("New client should not be checked yet")
	}
}

func TestTrivyErrorFormat(t *testing.T) {
	// Test error with details
	err := &TrivyError{
		Op:      "scan",
		Err:     ErrTrivyNotInstalled,
		Details: "Install Trivy first",
	}
	expected := "scan: trivy not installed - Install Trivy first"
	if err.Error() != expected {
		t.Errorf("Error() = %q, want %q", err.Error(), expected)
	}

	// Test error without details
	err2 := &TrivyError{
		Op:  "detect",
		Err: ErrTrivyNotInstalled,
	}
	expected2 := "detect: trivy not installed"
	if err2.Error() != expected2 {
		t.Errorf("Error() = %q, want %q", err2.Error(), expected2)
	}

	// Test Unwrap
	if err.Unwrap() != ErrTrivyNotInstalled {
		t.Errorf("Unwrap() did not return the wrapped error")
	}
}

func TestParseEmptyResults(t *testing.T) {
	client := NewTrivyClient()

	// Test with empty byte slice
	summary, err := client.parseResults([]byte{})
	if err != nil {
		t.Fatalf("parseResults failed on empty input: %v", err)
	}
	if summary.Total != 0 {
		t.Errorf("Expected Total=0 for empty input, got %d", summary.Total)
	}
}

func TestParseMalformedJSON(t *testing.T) {
	client := NewTrivyClient()

	// Test with malformed JSON
	_, err := client.parseResults([]byte(`{not valid json`))
	if err == nil {
		t.Error("Expected error for malformed JSON, got nil")
	}
	trivyErr, ok := err.(*TrivyError)
	if !ok {
		t.Error("Expected TrivyError type")
	}
	if trivyErr.Op != "scan" {
		t.Errorf("Expected Op='scan', got %q", trivyErr.Op)
	}
}

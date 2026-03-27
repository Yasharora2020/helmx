package helm

import (
	"testing"
)

// Tests for chart.go utility functions.
//
// These tests validate:
// - containsIgnoreCase: Case-insensitive substring matching for chart search
// - extractResourceKinds: Parsing Kubernetes manifests to extract resource types
//
// The extractResourceKinds function is essential for the Resources pane,
// parsing YAML manifests to display deployed Kubernetes resources.

// TestContainsIgnoreCase verifies case-insensitive search used in chart filtering.
// This ensures users can search for "nginx" and find "Nginx" or "NGINX" charts.
func TestContainsIgnoreCase(t *testing.T) {
	tests := []struct {
		s        string
		substr   string
		expected bool
	}{
		{"hello world", "world", true},
		{"Hello World", "world", true},
		{"HELLO WORLD", "world", true},
		{"hello world", "WORLD", true},
		{"nginx-ingress", "nginx", true},
		{"nginx-ingress", "NGINX", true},
		{"nginx-ingress", "redis", false},
		{"", "", true},
		{"hello", "", true},
		{"", "hello", false},
		{"abc", "abcd", false},
	}

	for _, tt := range tests {
		t.Run(tt.s+"_"+tt.substr, func(t *testing.T) {
			result := containsIgnoreCase(tt.s, tt.substr)
			if result != tt.expected {
				t.Errorf("containsIgnoreCase(%q, %q) = %v; want %v", tt.s, tt.substr, result, tt.expected)
			}
		})
	}
}

// TestExtractResourceKinds validates YAML manifest parsing for the Resources pane.
// Tests cover: empty manifests, single resources, multiple resources, resources
// without names, duplicate resources (deduplication), and invalid YAML.
func TestExtractResourceKinds(t *testing.T) {
	tests := []struct {
		name     string
		manifest string
		expected []string
	}{
		{
			name:     "empty manifest",
			manifest: "",
			expected: nil,
		},
		{
			name: "single deployment",
			manifest: `---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-app
spec:
  replicas: 1`,
			expected: []string{"Deployment/my-app"},
		},
		{
			name: "multiple resources",
			manifest: `---
apiVersion: v1
kind: Service
metadata:
  name: my-service
spec:
  ports:
    - port: 80
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-app
spec:
  replicas: 1
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: my-config
data:
  key: value`,
			expected: []string{"Service/my-service", "Deployment/my-app", "ConfigMap/my-config"},
		},
		{
			name: "resource without name",
			manifest: `---
apiVersion: v1
kind: Namespace
metadata:
  labels:
    app: test`,
			expected: []string{"Namespace"},
		},
		{
			name: "duplicate resources",
			manifest: `---
apiVersion: v1
kind: ConfigMap
metadata:
  name: config1
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: config1`,
			expected: []string{"ConfigMap/config1"},
		},
		{
			name:     "invalid yaml",
			manifest: "not: valid: yaml: content",
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractResourceKinds(tt.manifest)

			if len(result) != len(tt.expected) {
				t.Errorf("extractResourceKinds() returned %d items; want %d", len(result), len(tt.expected))
				t.Errorf("got: %v", result)
				t.Errorf("want: %v", tt.expected)
				return
			}

			for i, r := range result {
				if r != tt.expected[i] {
					t.Errorf("extractResourceKinds()[%d] = %s; want %s", i, r, tt.expected[i])
				}
			}
		})
	}
}

package helm

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

func writeTempYAML(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp("", "values-*.yaml")
	if err != nil {
		t.Fatalf("creating temp file: %v", err)
	}
	if err := os.WriteFile(f.Name(), []byte(content), 0644); err != nil {
		t.Fatalf("writing temp file: %v", err)
	}
	_ = f.Close()
	t.Cleanup(func() { _ = os.Remove(f.Name()) })
	return f.Name()
}

func TestValuesComposerEmpty(t *testing.T) {
	vc := &ValuesComposer{}

	vals, err := vc.Compose()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(vals) != 0 {
		t.Errorf("expected empty map, got %v", vals)
	}

	if !vc.IsEmpty() {
		t.Error("expected IsEmpty() to return true")
	}

	if s := vc.Summary(); s != "" {
		t.Errorf("expected empty summary, got %q", s)
	}
}

func TestValuesComposerInlineOnly(t *testing.T) {
	vc := &ValuesComposer{
		InlineYAML: "replicaCount: 3\nimage:\n  tag: latest\n",
	}

	vals, err := vc.Compose()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if vals["replicaCount"] != 3 {
		t.Errorf("expected replicaCount=3, got %v", vals["replicaCount"])
	}

	img, ok := vals["image"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected image to be a map, got %T", vals["image"])
	}
	if img["tag"] != "latest" {
		t.Errorf("expected image.tag=latest, got %v", img["tag"])
	}

	if vc.IsEmpty() {
		t.Error("expected IsEmpty() to return false")
	}

	if s := vc.Summary(); s != "inline values" {
		t.Errorf("expected summary 'inline values', got %q", s)
	}
}

func TestValuesComposerSetValuesOnly(t *testing.T) {
	vc := &ValuesComposer{
		SetValues: []string{"image.tag=v2", "replicaCount=5"},
	}

	vals, err := vc.Compose()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	img, ok := vals["image"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected image to be a map, got %T", vals["image"])
	}
	if img["tag"] != "v2" {
		t.Errorf("expected image.tag=v2, got %v", img["tag"])
	}

	if vals["replicaCount"] != "5" {
		t.Errorf("expected replicaCount=5, got %v (%T)", vals["replicaCount"], vals["replicaCount"])
	}
}

func TestValuesComposerMergeOrder(t *testing.T) {
	// File sets tag=v1, inline sets tag=v2, --set sets tag=v3
	// Final result should be v3 (--set wins)
	file := writeTempYAML(t, "image:\n  tag: v1\n  repo: nginx\n")

	vc := &ValuesComposer{
		ValueFiles: []string{file},
		InlineYAML: "image:\n  tag: v2\n",
		SetValues:  []string{"image.tag=v3"},
	}

	vals, err := vc.Compose()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	img, ok := vals["image"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected image to be a map, got %T", vals["image"])
	}

	if img["tag"] != "v3" {
		t.Errorf("expected image.tag=v3 (--set wins), got %v", img["tag"])
	}

	// repo should still be present from file (not overridden)
	if img["repo"] != "nginx" {
		t.Errorf("expected image.repo=nginx preserved from file, got %v", img["repo"])
	}
}

func TestValuesComposerMultipleFiles(t *testing.T) {
	file1 := writeTempYAML(t, "name: first\nshared: from-file1\n")
	file2 := writeTempYAML(t, "name: second\nextra: value\n")

	vc := &ValuesComposer{
		ValueFiles: []string{file1, file2},
	}

	vals, err := vc.Compose()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Second file overrides first
	if vals["name"] != "second" {
		t.Errorf("expected name=second (file2 wins), got %v", vals["name"])
	}

	// First file's unique key preserved
	if vals["shared"] != "from-file1" {
		t.Errorf("expected shared=from-file1, got %v", vals["shared"])
	}

	// Second file's unique key present
	if vals["extra"] != "value" {
		t.Errorf("expected extra=value, got %v", vals["extra"])
	}
}

func TestValuesComposerDeepMerge(t *testing.T) {
	file := writeTempYAML(t, `
server:
  port: 8080
  host: localhost
  tls:
    enabled: true
    cert: /path/to/cert
`)

	vc := &ValuesComposer{
		ValueFiles: []string{file},
		InlineYAML: `
server:
  port: 9090
  tls:
    key: /path/to/key
`,
	}

	vals, err := vc.Compose()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	server, ok := vals["server"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected server to be a map")
	}

	// port overridden by inline
	if server["port"] != 9090 {
		t.Errorf("expected server.port=9090, got %v", server["port"])
	}

	// host preserved from file
	if server["host"] != "localhost" {
		t.Errorf("expected server.host=localhost, got %v", server["host"])
	}

	tls, ok := server["tls"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected server.tls to be a map")
	}

	// tls.enabled preserved from file (deep merge, not replace)
	if tls["enabled"] != true {
		t.Errorf("expected server.tls.enabled=true (preserved), got %v", tls["enabled"])
	}

	// tls.cert preserved from file
	if tls["cert"] != "/path/to/cert" {
		t.Errorf("expected server.tls.cert preserved, got %v", tls["cert"])
	}

	// tls.key added from inline
	if tls["key"] != "/path/to/key" {
		t.Errorf("expected server.tls.key=/path/to/key, got %v", tls["key"])
	}
}

func TestValuesComposerInvalidYAML(t *testing.T) {
	vc := &ValuesComposer{
		InlineYAML: "not: valid: yaml: [[[",
	}

	_, err := vc.Compose()
	if err == nil {
		t.Error("expected error for invalid YAML, got nil")
	}
}

func TestValuesComposerInvalidSetValue(t *testing.T) {
	vc := &ValuesComposer{
		SetValues: []string{"invalid[=broken"},
	}

	_, err := vc.Compose()
	if err == nil {
		t.Error("expected error for invalid --set value, got nil")
	}
}

func TestValuesComposerMissingFile(t *testing.T) {
	vc := &ValuesComposer{
		ValueFiles: []string{filepath.Join(os.TempDir(), "nonexistent-values-12345.yaml")},
	}

	_, err := vc.Compose()
	if err == nil {
		t.Error("expected error for missing file, got nil")
	}
}

func TestValuesComposerSummary(t *testing.T) {
	tests := []struct {
		name     string
		vc       ValuesComposer
		expected string
	}{
		{
			name:     "empty",
			vc:       ValuesComposer{},
			expected: "",
		},
		{
			name:     "one file",
			vc:       ValuesComposer{ValueFiles: []string{"/a.yaml"}},
			expected: "1 file",
		},
		{
			name:     "two files",
			vc:       ValuesComposer{ValueFiles: []string{"/a.yaml", "/b.yaml"}},
			expected: "2 files",
		},
		{
			name:     "inline only",
			vc:       ValuesComposer{InlineYAML: "key: val"},
			expected: "inline values",
		},
		{
			name:     "one override",
			vc:       ValuesComposer{SetValues: []string{"a=b"}},
			expected: "1 override",
		},
		{
			name:     "three overrides",
			vc:       ValuesComposer{SetValues: []string{"a=b", "c=d", "e=f"}},
			expected: "3 overrides",
		},
		{
			name: "all sources",
			vc: ValuesComposer{
				ValueFiles: []string{"/a.yaml", "/b.yaml"},
				InlineYAML: "key: val",
				SetValues:  []string{"a=b", "c=d", "e=f"},
			},
			expected: "2 files, inline values, 3 overrides",
		},
		{
			name:     "whitespace-only inline ignored",
			vc:       ValuesComposer{InlineYAML: "   \n\t  "},
			expected: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.vc.Summary()
			if got != tc.expected {
				t.Errorf("expected %q, got %q", tc.expected, got)
			}
		})
	}
}

func TestValuesComposerComposeToYAML(t *testing.T) {
	vc := &ValuesComposer{
		InlineYAML: "name: test\ncount: 3\n",
	}

	yamlStr, err := vc.ComposeToYAML()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if yamlStr == "" {
		t.Error("expected non-empty YAML output")
	}

	// Verify it's valid YAML by unmarshaling
	var parsed map[string]interface{}
	if err := yaml.Unmarshal([]byte(yamlStr), &parsed); err != nil {
		t.Fatalf("ComposeToYAML produced invalid YAML: %v", err)
	}

	if parsed["name"] != "test" {
		t.Errorf("expected name=test, got %v", parsed["name"])
	}

	// Empty composer returns empty string
	empty := &ValuesComposer{}
	emptyYAML, err := empty.ComposeToYAML()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if emptyYAML != "" {
		t.Errorf("expected empty string for empty composer, got %q", emptyYAML)
	}
}

func TestMergeMapsImmutability(t *testing.T) {
	base := map[string]interface{}{
		"key1": "original",
		"nested": map[string]interface{}{
			"a": "1",
			"b": "2",
		},
	}

	override := map[string]interface{}{
		"key1": "changed",
		"nested": map[string]interface{}{
			"b": "overridden",
			"c": "3",
		},
	}

	// Snapshot original values
	baseKey1 := base["key1"]
	baseNested := base["nested"].(map[string]interface{})
	baseNestedB := baseNested["b"]
	overrideKey1 := override["key1"]
	overrideNested := override["nested"].(map[string]interface{})
	overrideNestedB := overrideNested["b"]

	result := mergeMaps(base, override)

	// Verify result is correct
	if result["key1"] != "changed" {
		t.Errorf("expected key1=changed, got %v", result["key1"])
	}

	resultNested, ok := result["nested"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected nested to be a map")
	}
	if resultNested["a"] != "1" {
		t.Errorf("expected nested.a=1, got %v", resultNested["a"])
	}
	if resultNested["b"] != "overridden" {
		t.Errorf("expected nested.b=overridden, got %v", resultNested["b"])
	}
	if resultNested["c"] != "3" {
		t.Errorf("expected nested.c=3, got %v", resultNested["c"])
	}

	// Verify base was NOT mutated
	if base["key1"] != baseKey1 {
		t.Errorf("base[key1] was mutated: expected %v, got %v", baseKey1, base["key1"])
	}
	if baseNested["b"] != baseNestedB {
		t.Errorf("base[nested][b] was mutated: expected %v, got %v", baseNestedB, baseNested["b"])
	}
	if _, exists := baseNested["c"]; exists {
		t.Error("base[nested][c] should not exist - base was mutated")
	}

	// Verify override was NOT mutated
	if override["key1"] != overrideKey1 {
		t.Errorf("override[key1] was mutated: expected %v, got %v", overrideKey1, override["key1"])
	}
	if overrideNested["b"] != overrideNestedB {
		t.Errorf("override[nested][b] was mutated: expected %v, got %v", overrideNestedB, overrideNested["b"])
	}
	if _, exists := overrideNested["a"]; exists {
		t.Error("override[nested][a] should not exist - override was mutated")
	}
}

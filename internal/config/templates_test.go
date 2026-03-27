package config

import (
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestTemplatesDir_ReturnsPathUnderConfigDir(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)

	dir, err := TemplatesDir()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := filepath.Join(tmp, "helmx", "templates")
	if dir != expected {
		t.Errorf("expected %q, got %q", expected, dir)
	}
}

func TestChartDirName_ReplacesSlash(t *testing.T) {
	got := chartDirName("bitnami/nginx")
	if got != "bitnami-nginx" {
		t.Errorf("expected bitnami-nginx, got %q", got)
	}
}

func TestChartDirName_ReplacesColon(t *testing.T) {
	got := chartDirName("oci://registry:5000/chart")
	// Verify no / or : remain in the sanitized directory name
	if strings.ContainsAny(got, "/:") {
		t.Errorf("expected no / or : in %q", got)
	}
}

func TestExtractOverrides_SingleChange(t *testing.T) {
	defaults := "replicaCount: 1\nimage:\n  tag: latest\n"
	user := "replicaCount: 5\nimage:\n  tag: latest\n"

	got, err := ExtractOverrides(user, defaults)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 override, got %d: %v", len(got), got)
	}
	if got["replicaCount"] != 5 {
		t.Errorf("expected replicaCount=5, got %v", got["replicaCount"])
	}
}

func TestExtractOverrides_NoChanges(t *testing.T) {
	y := "replicaCount: 1\n"
	got, err := ExtractOverrides(y, y)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty overrides, got %v", got)
	}
}

func TestExtractOverrides_SliceValue_NoChange(t *testing.T) {
	// YAML arrays unmarshal to []interface{} — comparing with != panics in Go
	y := "imagePullSecrets: []\ntolerations: []\n"
	got, err := ExtractOverrides(y, y)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty overrides for unchanged slices, got %v", got)
	}
}

func TestExtractOverrides_SliceValue_Changed(t *testing.T) {
	defaults := "tolerations: []\n"
	user := "tolerations:\n  - key: dedicated\n    value: gpu\n"
	got, err := ExtractOverrides(user, defaults)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := got["tolerations"]; !ok {
		t.Error("expected tolerations override to be present")
	}
}

func TestExtractOverrides_NestedChange(t *testing.T) {
	defaults := "image:\n  tag: latest\n  pullPolicy: IfNotPresent\n"
	user := "image:\n  tag: 1.2.3\n  pullPolicy: IfNotPresent\n"

	got, err := ExtractOverrides(user, defaults)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	imageMap, ok := got["image"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected image to be a map, got %T", got["image"])
	}
	if imageMap["tag"] != "1.2.3" {
		t.Errorf("expected tag=1.2.3, got %v", imageMap["tag"])
	}
	if _, exists := imageMap["pullPolicy"]; exists {
		t.Error("expected pullPolicy to be absent (not changed)")
	}
}

func TestApplyTemplate_MergesOverrides(t *testing.T) {
	defaults := "replicaCount: 1\nimage:\n  tag: latest\n"
	overrides := map[string]interface{}{"replicaCount": 5}

	result, err := ApplyTemplate(overrides, defaults)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var out map[string]interface{}
	if err := yaml.Unmarshal([]byte(result), &out); err != nil {
		t.Fatalf("result is not valid YAML: %v", err)
	}
	if out["replicaCount"] != 5 {
		t.Errorf("expected replicaCount=5, got %v", out["replicaCount"])
	}
	img, ok := out["image"].(map[string]interface{})
	if !ok || img["tag"] != "latest" {
		t.Errorf("expected image.tag=latest to be preserved, got %v", out["image"])
	}
}

func TestApplyTemplate_EmptyOverrides(t *testing.T) {
	defaults := "replicaCount: 1\n"
	result, err := ApplyTemplate(nil, defaults)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != defaults {
		t.Errorf("expected defaults unchanged, got %q", result)
	}
}

func TestSaveAndLoadTemplate(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)

	defaults := "replicaCount: 1\nimage:\n  tag: latest\n"
	user := "replicaCount: 5\nimage:\n  tag: latest\n"

	err := SaveTemplate("ha", "bitnami/nginx", "18.1.0", "HA setup", user, defaults)
	if err != nil {
		t.Fatalf("SaveTemplate: %v", err)
	}

	tf, err := LoadTemplate("bitnami/nginx", "ha")
	if err != nil {
		t.Fatalf("LoadTemplate: %v", err)
	}
	if tf.Name != "ha" {
		t.Errorf("expected name=ha, got %q", tf.Name)
	}
	if tf.ChartVersion != "18.1.0" {
		t.Errorf("expected version=18.1.0, got %q", tf.ChartVersion)
	}
	if tf.Values["replicaCount"] != 5 {
		t.Errorf("expected replicaCount=5, got %v", tf.Values["replicaCount"])
	}
	// Full defaults must NOT be stored (image.tag was unchanged)
	if _, ok := tf.Values["image"]; ok {
		t.Error("expected image key to be absent (not overridden)")
	}
}

func TestSaveTemplate_DuplicateReturnsError(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)

	defaults := "replicaCount: 1\n"
	err := SaveTemplate("ha", "bitnami/nginx", "18.1.0", "", defaults, defaults)
	if err != nil {
		t.Fatalf("first save: %v", err)
	}
	err = SaveTemplate("ha", "bitnami/nginx", "18.1.0", "", defaults, defaults)
	if err != ErrTemplateExists {
		t.Errorf("expected ErrTemplateExists, got %v", err)
	}
}

func TestListTemplates(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)

	defaults := "replicaCount: 1\n"
	_ = SaveTemplate("t1", "bitnami/nginx", "18.1.0", "", "replicaCount: 2\n", defaults)
	_ = SaveTemplate("t2", "bitnami/nginx", "18.1.0", "", "replicaCount: 3\n", defaults)

	templates, err := ListTemplates("bitnami/nginx")
	if err != nil {
		t.Fatalf("ListTemplates: %v", err)
	}
	if len(templates) != 2 {
		t.Errorf("expected 2 templates, got %d", len(templates))
	}
}

func TestDeleteTemplate(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)

	defaults := "replicaCount: 1\n"
	_ = SaveTemplate("ha", "bitnami/nginx", "18.1.0", "", "replicaCount: 5\n", defaults)

	err := DeleteTemplate("bitnami/nginx", "ha")
	if err != nil {
		t.Fatalf("DeleteTemplate: %v", err)
	}
	templates, _ := ListTemplates("bitnami/nginx")
	if len(templates) != 0 {
		t.Errorf("expected 0 templates after delete, got %d", len(templates))
	}
}

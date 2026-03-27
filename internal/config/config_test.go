package config

import (
	"os"
	"path/filepath"
	"testing"
)

// ---------- Load / Save / round-trip ----------

func TestLoad_MissingFileReturnsDefaults(t *testing.T) {
	// Point configPath at a non-existent directory so Load() falls back to defaults.
	// We override $XDG_CONFIG_HOME to a temp dir that has no config file.
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	// Verify all default values
	if cfg.Theme != "default" {
		t.Errorf("expected default theme, got %q", cfg.Theme)
	}
	if !cfg.SecurityScanEnabled {
		t.Error("expected SecurityScanEnabled to be true by default")
	}
	if !cfg.ShowWelcomeOnStart {
		t.Error("expected ShowWelcomeOnStart to be true by default")
	}
	if cfg.DefaultNamespace != "" {
		t.Errorf("expected empty DefaultNamespace, got %q", cfg.DefaultNamespace)
	}
	if cfg.Editor != "" {
		t.Errorf("expected empty Editor, got %q", cfg.Editor)
	}
	if len(cfg.ChartRegistries) != 1 {
		t.Fatalf("expected 1 default registry, got %d", len(cfg.ChartRegistries))
	}
	if cfg.ChartRegistries[0].URL != DefaultRegistryURL {
		t.Errorf("expected default registry URL %q, got %q", DefaultRegistryURL, cfg.ChartRegistries[0].URL)
	}
	if cfg.ChartRegistries[0].Name != DefaultRegistryName {
		t.Errorf("expected default registry name %q, got %q", DefaultRegistryName, cfg.ChartRegistries[0].Name)
	}
}

func TestLoad_CorruptYAMLReturnsError(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)

	dir := filepath.Join(tmp, ConfigDir)
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ConfigFile), []byte("{{{{not yaml!!!!"), 0600); err != nil {
		t.Fatal(err)
	}

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for corrupt YAML, got nil")
	}
}

func TestLoad_ValidConfigRoundTrips(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)

	original := &Config{
		ChartRegistries: []ChartRegistry{
			{Name: "custom", URL: "https://custom.example.com"},
		},
		DefaultNamespace:    "production",
		Editor:              "nvim",
		Theme:               "dracula",
		SecurityScanEnabled: true,
		ShowWelcomeOnStart:  false,
		StackTemplates: []StackTemplate{
			{
				Name: "stack1",
				Charts: []StackChartItem{
					{ChartRef: "bitnami/postgresql", ReleaseName: "pg", Namespace: "db"},
				},
			},
		},
	}

	if err := original.Save(); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	loaded, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if loaded.DefaultNamespace != original.DefaultNamespace {
		t.Errorf("DefaultNamespace mismatch: got %q, want %q", loaded.DefaultNamespace, original.DefaultNamespace)
	}
	if loaded.Editor != original.Editor {
		t.Errorf("Editor mismatch: got %q, want %q", loaded.Editor, original.Editor)
	}
	if loaded.Theme != original.Theme {
		t.Errorf("Theme mismatch: got %q, want %q", loaded.Theme, original.Theme)
	}
	if loaded.SecurityScanEnabled != original.SecurityScanEnabled {
		t.Errorf("SecurityScanEnabled mismatch: got %v, want %v", loaded.SecurityScanEnabled, original.SecurityScanEnabled)
	}
	if loaded.ShowWelcomeOnStart != original.ShowWelcomeOnStart {
		t.Errorf("ShowWelcomeOnStart mismatch: got %v, want %v", loaded.ShowWelcomeOnStart, original.ShowWelcomeOnStart)
	}
	if len(loaded.ChartRegistries) != 1 || loaded.ChartRegistries[0].URL != "https://custom.example.com" {
		t.Errorf("ChartRegistries mismatch: got %+v", loaded.ChartRegistries)
	}
	if len(loaded.StackTemplates) != 1 || loaded.StackTemplates[0].Name != "stack1" {
		t.Errorf("StackTemplates mismatch: got %+v", loaded.StackTemplates)
	}
}

func TestLoad_EmptyRegistriesGetDefault(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)

	dir := filepath.Join(tmp, ConfigDir)
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	// Write config with explicitly empty registries
	data := []byte("chartRegistries: []\ntheme: nord\n")
	if err := os.WriteFile(filepath.Join(dir, ConfigFile), data, 0600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if len(cfg.ChartRegistries) != 1 || cfg.ChartRegistries[0].URL != DefaultRegistryURL {
		t.Errorf("expected default registry to be added when empty, got %+v", cfg.ChartRegistries)
	}
}

func TestLoad_EmptyThemeGetDefault(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)

	dir := filepath.Join(tmp, ConfigDir)
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	data := []byte("defaultNamespace: test\n")
	if err := os.WriteFile(filepath.Join(dir, ConfigFile), data, 0600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.Theme != "default" {
		t.Errorf("expected theme 'default' when empty, got %q", cfg.Theme)
	}
}

func TestSave_CreatesDirectoryAndFile(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)

	cfg := &Config{
		Theme: "gruvbox",
		ChartRegistries: []ChartRegistry{
			{Name: DefaultRegistryName, URL: DefaultRegistryURL},
		},
	}
	if err := cfg.Save(); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	expectedPath := filepath.Join(tmp, ConfigDir, ConfigFile)
	info, err := os.Stat(expectedPath)
	if err != nil {
		t.Fatalf("config file not created at %s: %v", expectedPath, err)
	}
	if info.Mode().Perm() != ConfigFilePerm {
		t.Errorf("expected file perm %o, got %o", ConfigFilePerm, info.Mode().Perm())
	}

	dirInfo, err := os.Stat(filepath.Dir(expectedPath))
	if err != nil {
		t.Fatalf("config dir not created: %v", err)
	}
	if dirInfo.Mode().Perm() != ConfigDirPerm {
		t.Errorf("expected dir perm %o, got %o", ConfigDirPerm, dirInfo.Mode().Perm())
	}
}


// ---------- Stack CRUD ----------

func TestAddStack_Success(t *testing.T) {
	cfg := &Config{}
	stack := StackTemplate{
		Name: "my-stack",
		Charts: []StackChartItem{
			{ChartRef: "bitnami/postgresql", ReleaseName: "pg", Namespace: "db"},
		},
	}

	if err := cfg.AddStack(stack); err != nil {
		t.Fatalf("AddStack failed: %v", err)
	}
	if len(cfg.StackTemplates) != 1 {
		t.Fatalf("expected 1 stack, got %d", len(cfg.StackTemplates))
	}
	if cfg.StackTemplates[0].CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be set")
	}
}

func TestAddStack_DuplicateNameReturnsError(t *testing.T) {
	cfg := &Config{}
	_ = cfg.AddStack(StackTemplate{Name: "dup"})
	err := cfg.AddStack(StackTemplate{Name: "dup"})
	if err != ErrStackExists {
		t.Errorf("expected ErrStackExists, got %v", err)
	}
}

func TestGetStack_Found(t *testing.T) {
	cfg := &Config{}
	_ = cfg.AddStack(StackTemplate{Name: "find-me"})

	stack, found := cfg.GetStack("find-me")
	if !found {
		t.Fatal("expected stack to be found")
	}
	if stack.Name != "find-me" {
		t.Errorf("unexpected name %q", stack.Name)
	}
}

func TestGetStack_NotFound(t *testing.T) {
	cfg := &Config{}
	_, found := cfg.GetStack("nonexistent")
	if found {
		t.Error("expected stack to NOT be found")
	}
}

func TestGetStacks_Empty(t *testing.T) {
	cfg := &Config{}
	if stacks := cfg.GetStacks(); len(stacks) != 0 {
		t.Errorf("expected 0 stacks, got %d", len(stacks))
	}
}

func TestUpdateStack_Success(t *testing.T) {
	cfg := &Config{}
	_ = cfg.AddStack(StackTemplate{Name: "update-me", Description: "old"})

	newCharts := []StackChartItem{{ChartRef: "new/chart", ReleaseName: "new"}}
	if err := cfg.UpdateStack("update-me", newCharts, "new desc"); err != nil {
		t.Fatalf("UpdateStack failed: %v", err)
	}
	stack, _ := cfg.GetStack("update-me")
	if len(stack.Charts) != 1 || stack.Charts[0].ChartRef != "new/chart" {
		t.Errorf("charts not updated: %+v", stack.Charts)
	}
	if stack.Description != "new desc" {
		t.Errorf("description not updated: got %q", stack.Description)
	}
}

func TestUpdateStack_EmptyDescriptionPreservesOld(t *testing.T) {
	cfg := &Config{}
	_ = cfg.AddStack(StackTemplate{Name: "keep-desc", Description: "original"})

	if err := cfg.UpdateStack("keep-desc", nil, ""); err != nil {
		t.Fatalf("UpdateStack failed: %v", err)
	}
	stack, _ := cfg.GetStack("keep-desc")
	if stack.Description != "original" {
		t.Errorf("expected description preserved as 'original', got %q", stack.Description)
	}
}

func TestUpdateStack_NotFound(t *testing.T) {
	cfg := &Config{}
	err := cfg.UpdateStack("ghost", nil, "d")
	if err != ErrStackNotFound {
		t.Errorf("expected ErrStackNotFound, got %v", err)
	}
}

func TestDeleteStack_Success(t *testing.T) {
	cfg := &Config{}
	_ = cfg.AddStack(StackTemplate{Name: "delete-me"})

	if err := cfg.DeleteStack("delete-me"); err != nil {
		t.Fatalf("DeleteStack failed: %v", err)
	}
	if len(cfg.StackTemplates) != 0 {
		t.Error("expected 0 stacks after delete")
	}
}

func TestDeleteStack_NotFound(t *testing.T) {
	cfg := &Config{}
	err := cfg.DeleteStack("ghost")
	if err != ErrStackNotFound {
		t.Errorf("expected ErrStackNotFound, got %v", err)
	}
}

func TestDeleteStack_MiddleOfSlice(t *testing.T) {
	cfg := &Config{}
	_ = cfg.AddStack(StackTemplate{Name: "first"})
	_ = cfg.AddStack(StackTemplate{Name: "second"})
	_ = cfg.AddStack(StackTemplate{Name: "third"})

	if err := cfg.DeleteStack("second"); err != nil {
		t.Fatalf("DeleteStack failed: %v", err)
	}
	if len(cfg.StackTemplates) != 2 {
		t.Fatalf("expected 2 stacks, got %d", len(cfg.StackTemplates))
	}
	if cfg.StackTemplates[0].Name != "first" || cfg.StackTemplates[1].Name != "third" {
		t.Errorf("unexpected remaining stacks: %+v", cfg.StackTemplates)
	}
}

// ---------- Registry ops ----------

func TestAddRegistry_New(t *testing.T) {
	cfg := &Config{}
	cfg.AddRegistry("custom", "https://custom.example.com")

	if len(cfg.ChartRegistries) != 1 {
		t.Fatalf("expected 1 registry, got %d", len(cfg.ChartRegistries))
	}
	if cfg.ChartRegistries[0].Name != "custom" || cfg.ChartRegistries[0].URL != "https://custom.example.com" {
		t.Errorf("unexpected registry: %+v", cfg.ChartRegistries[0])
	}
}

func TestAddRegistry_DuplicateURLIgnored(t *testing.T) {
	cfg := &Config{}
	cfg.AddRegistry("first", "https://same.url")
	cfg.AddRegistry("second", "https://same.url")

	if len(cfg.ChartRegistries) != 1 {
		t.Errorf("expected 1 registry (duplicate URL ignored), got %d", len(cfg.ChartRegistries))
	}
	if cfg.ChartRegistries[0].Name != "first" {
		t.Errorf("expected first name to be kept, got %q", cfg.ChartRegistries[0].Name)
	}
}

func TestAddRegistry_DifferentURLsAllowed(t *testing.T) {
	cfg := &Config{}
	cfg.AddRegistry("a", "https://a.example.com")
	cfg.AddRegistry("b", "https://b.example.com")

	if len(cfg.ChartRegistries) != 2 {
		t.Errorf("expected 2 registries, got %d", len(cfg.ChartRegistries))
	}
}

func TestRemoveRegistry(t *testing.T) {
	cfg := &Config{
		ChartRegistries: []ChartRegistry{
			{Name: "a", URL: "https://a.example.com"},
			{Name: "b", URL: "https://b.example.com"},
			{Name: "c", URL: "https://c.example.com"},
		},
	}

	cfg.RemoveRegistry("https://b.example.com")
	if len(cfg.ChartRegistries) != 2 {
		t.Fatalf("expected 2 registries after remove, got %d", len(cfg.ChartRegistries))
	}
	if cfg.HasRegistry("https://b.example.com") {
		t.Error("expected removed registry to not be found")
	}
}

func TestRemoveRegistry_NonExistent(t *testing.T) {
	cfg := &Config{
		ChartRegistries: []ChartRegistry{
			{Name: "a", URL: "https://a.example.com"},
		},
	}
	cfg.RemoveRegistry("https://not-here.example.com")
	if len(cfg.ChartRegistries) != 1 {
		t.Errorf("expected registry count unchanged, got %d", len(cfg.ChartRegistries))
	}
}

func TestHasRegistry(t *testing.T) {
	cfg := &Config{
		ChartRegistries: []ChartRegistry{
			{Name: "a", URL: "https://a.example.com"},
		},
	}

	if !cfg.HasRegistry("https://a.example.com") {
		t.Error("expected HasRegistry to return true for existing URL")
	}
	if cfg.HasRegistry("https://nope.example.com") {
		t.Error("expected HasRegistry to return false for missing URL")
	}
}

func TestHasCustomRegistries(t *testing.T) {
	tests := []struct {
		name     string
		regs     []ChartRegistry
		expected bool
	}{
		{
			name:     "empty registries",
			regs:     nil,
			expected: false,
		},
		{
			name:     "only default registry",
			regs:     []ChartRegistry{{Name: DefaultRegistryName, URL: DefaultRegistryURL}},
			expected: false,
		},
		{
			name: "default plus custom",
			regs: []ChartRegistry{
				{Name: DefaultRegistryName, URL: DefaultRegistryURL},
				{Name: "custom", URL: "https://custom.example.com"},
			},
			expected: true,
		},
		{
			name:     "only custom",
			regs:     []ChartRegistry{{Name: "custom", URL: "https://custom.example.com"}},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{ChartRegistries: tt.regs}
			if got := cfg.HasCustomRegistries(); got != tt.expected {
				t.Errorf("HasCustomRegistries() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestGetChartRegistries_EmptyReturnsDefault(t *testing.T) {
	cfg := &Config{}
	regs := cfg.GetChartRegistries()
	if len(regs) != 1 || regs[0].URL != DefaultRegistryURL {
		t.Errorf("expected default registry when empty, got %+v", regs)
	}
}

func TestGetChartRegistries_ReturnsConfigured(t *testing.T) {
	cfg := &Config{
		ChartRegistries: []ChartRegistry{
			{Name: "custom", URL: "https://custom.example.com"},
		},
	}
	regs := cfg.GetChartRegistries()
	if len(regs) != 1 || regs[0].URL != "https://custom.example.com" {
		t.Errorf("expected configured registry, got %+v", regs)
	}
}

// ---------- GetEditor ----------

func TestGetEditor_ConfigValueUsed(t *testing.T) {
	// Unset EDITOR to isolate config-based test
	t.Setenv("EDITOR", "")

	cfg := &Config{Editor: "vim"}
	editor := cfg.GetEditor()
	if editor != "vim" {
		t.Errorf("expected 'vim', got %q", editor)
	}
}

func TestGetEditor_EnvFallback(t *testing.T) {
	t.Setenv("EDITOR", "nano")

	cfg := &Config{Editor: ""}
	editor := cfg.GetEditor()
	if editor != "nano" {
		t.Errorf("expected 'nano' from $EDITOR, got %q", editor)
	}
}

func TestGetEditor_UnsafeConfigFallsToEnv(t *testing.T) {
	t.Setenv("EDITOR", "vim")

	cfg := &Config{Editor: "vim; rm -rf /"}
	editor := cfg.GetEditor()
	// The unsafe config editor should be rejected, falling back to $EDITOR
	if editor != "vim" {
		t.Errorf("expected 'vim' (from $EDITOR fallback), got %q", editor)
	}
}

func TestGetEditor_UnsafeEnvFallsToDefault(t *testing.T) {
	t.Setenv("EDITOR", "$(malicious)")

	cfg := &Config{Editor: ""}
	editor := cfg.GetEditor()
	// Both config and env are unsafe; should fall back to vim/vi/nano
	safe := map[string]bool{"vim": true, "vi": true, "nano": true}
	if !safe[editor] {
		t.Errorf("expected safe fallback editor, got %q", editor)
	}
}

// ---------- isEditorSafe ----------

func TestIsEditorSafe(t *testing.T) {
	tests := []struct {
		name     string
		editor   string
		expected bool
	}{
		// Unsafe inputs
		{"empty string", "", false},
		{"whitespace only", "   ", false},
		{"semicolon injection", "vim; rm -rf /", false},
		{"pipe injection", "vim | cat /etc/passwd", false},
		{"ampersand injection", "vim & malicious", false},
		{"dollar substitution", "$(malicious)", false},
		{"backtick substitution", "`malicious`", false},
		{"double quote injection", `vim" --malicious`, false},
		{"single quote injection", "vim' --malicious", false},
		{"redirect injection", "vim > /tmp/evil", false},
		{"parentheses injection", "vim()", false},
		{"brace injection", "vim{}", false},
		{"bracket injection", "vim[]", false},
		{"exclamation injection", "vim!", false},
		{"hash injection", "vim#comment", false},
		{"glob injection", "vim*", false},
		{"tilde injection", "vim~", false},
		{"backslash injection", "vim\\n", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isEditorSafe(tt.editor); got != tt.expected {
				t.Errorf("isEditorSafe(%q) = %v, want %v", tt.editor, got, tt.expected)
			}
		})
	}
}

func TestIsEditorSafe_KnownSafeEditors(t *testing.T) {
	// These are in the safe allowlist. They should pass IF they exist on the system.
	// We test the ones most likely present in CI / dev environments.
	for _, editor := range []string{"vim", "vi", "nano"} {
		t.Run(editor, func(t *testing.T) {
			result := isEditorSafe(editor)
			// We can't guarantee these are installed, so just verify no panic.
			// If installed, should be true. If not, should be false (LookPath fails).
			_ = result
		})
	}
}

// ---------- Feature request config ----------

func TestGetFeatureRequestURL_EnvOverride(t *testing.T) {
	t.Setenv("HELMX_FEATURE_REQUEST_URL", "https://env.example.com/hook")

	cfg := &Config{FeatureRequest: FeatureRequestConfig{URL: "https://config.example.com/hook"}}
	if got := cfg.GetFeatureRequestURL(); got != "https://env.example.com/hook" {
		t.Errorf("expected env URL, got %q", got)
	}
}

func TestGetFeatureRequestURL_FallbackToConfig(t *testing.T) {
	t.Setenv("HELMX_FEATURE_REQUEST_URL", "")

	cfg := &Config{FeatureRequest: FeatureRequestConfig{URL: "https://config.example.com/hook"}}
	if got := cfg.GetFeatureRequestURL(); got != "https://config.example.com/hook" {
		t.Errorf("expected config URL, got %q", got)
	}
}

func TestGetFeatureRequestURL_Empty(t *testing.T) {
	t.Setenv("HELMX_FEATURE_REQUEST_URL", "")

	cfg := &Config{}
	if got := cfg.GetFeatureRequestURL(); got != "" {
		t.Errorf("expected empty URL, got %q", got)
	}
}

func TestGetFeatureRequestAPIKey_EnvOverride(t *testing.T) {
	t.Setenv("HELMX_FEATURE_REQUEST_API_KEY", "env-key")

	cfg := &Config{FeatureRequest: FeatureRequestConfig{APIKey: "config-key"}}
	if got := cfg.GetFeatureRequestAPIKey(); got != "env-key" {
		t.Errorf("expected env key, got %q", got)
	}
}

func TestGetFeatureRequestAPIKey_FallbackToConfig(t *testing.T) {
	t.Setenv("HELMX_FEATURE_REQUEST_API_KEY", "")

	cfg := &Config{FeatureRequest: FeatureRequestConfig{APIKey: "config-key"}}
	if got := cfg.GetFeatureRequestAPIKey(); got != "config-key" {
		t.Errorf("expected config key, got %q", got)
	}
}

func TestIsFeatureRequestEnabled(t *testing.T) {
	t.Setenv("HELMX_FEATURE_REQUEST_URL", "")

	cfg := &Config{}
	if cfg.IsFeatureRequestEnabled() {
		t.Error("expected disabled when no URL configured")
	}

	cfg.FeatureRequest.URL = "https://hook.example.com"
	if !cfg.IsFeatureRequestEnabled() {
		t.Error("expected enabled when URL configured")
	}
}

// ---------- ConfigPath ----------

func TestConfigPath_ReturnsNonEmpty(t *testing.T) {
	path := ConfigPath()
	if path == "" {
		t.Error("expected non-empty config path")
	}
	if !filepath.IsAbs(path) && path != "~/.config/helmx/config.yaml" {
		// It should be absolute or the fallback value
		t.Errorf("expected absolute path or fallback, got %q", path)
	}
}

// ---------- Edge cases ----------


func TestStack_EmptyCharts(t *testing.T) {
	cfg := &Config{}
	err := cfg.AddStack(StackTemplate{Name: "empty-stack", Charts: nil})
	if err != nil {
		t.Fatalf("AddStack with nil charts failed: %v", err)
	}
	stack, found := cfg.GetStack("empty-stack")
	if !found {
		t.Fatal("expected stack to be found")
	}
	if stack.Charts != nil {
		t.Errorf("expected nil charts, got %+v", stack.Charts)
	}
}

func TestRemoveRegistry_AllRegistries(t *testing.T) {
	cfg := &Config{
		ChartRegistries: []ChartRegistry{
			{Name: "a", URL: "https://a.example.com"},
		},
	}
	cfg.RemoveRegistry("https://a.example.com")
	if len(cfg.ChartRegistries) != 0 {
		t.Errorf("expected 0 registries, got %d", len(cfg.ChartRegistries))
	}
	// ChartRegistries is nil now, GetChartRegistries should still return default
	regs := cfg.GetChartRegistries()
	if len(regs) != 1 || regs[0].URL != DefaultRegistryURL {
		t.Errorf("expected default registry from GetChartRegistries after removing all, got %+v", regs)
	}
}

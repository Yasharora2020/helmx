package config

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

var (
	// ErrStackExists is returned when adding a stack with a name that already exists
	ErrStackExists = errors.New("stack with this name already exists")

	// ErrStackNotFound is returned when a stack is not found
	ErrStackNotFound = errors.New("stack not found")
)

// ChartRegistry represents a chart registry configuration
type ChartRegistry struct {
	Name string `yaml:"name"`
	URL  string `yaml:"url"`
}

// StackChartItem represents a single chart in a multi-chart installation stack.
// Charts are installed in order, with each waiting for the previous to be ready.
type StackChartItem struct {
	ChartRef        string `yaml:"chartRef"`        // Chart reference (e.g., "bitnami/postgresql", "oci://...")
	ReleaseName     string `yaml:"releaseName"`     // Release name for this chart
	Namespace       string `yaml:"namespace"`       // Target namespace
	Values          string `yaml:"values"`          // YAML values content
	CreateNamespace bool   `yaml:"createNamespace"` // Whether to create the namespace
	WaitForReady    bool   `yaml:"waitForReady"`    // Wait for pods to be ready before next chart
}

// StackTemplate represents a saved multi-chart installation configuration.
// Stacks allow users to deploy multiple related charts in sequence.
type StackTemplate struct {
	Name        string           `yaml:"name"`        // User-defined stack name
	Description string           `yaml:"description"` // Optional description
	Charts      []StackChartItem `yaml:"charts"`      // Charts to install in order
	CreatedAt   time.Time        `yaml:"createdAt"`   // When the stack was created
}

// FeatureRequestConfig holds settings for the feature request webhook.
// These can be overridden via environment variables:
// - HELMX_FEATURE_REQUEST_URL
// - HELMX_FEATURE_REQUEST_API_KEY
type FeatureRequestConfig struct {
	// URL is the webhook URL for feature requests (default: disabled)
	URL string `yaml:"url,omitempty"`
	// APIKey is the API key for authentication (optional)
	APIKey string `yaml:"apiKey,omitempty"`
}

// Config holds application configuration
type Config struct {
	// ChartRegistries is a list of chart registry URLs for searching
	ChartRegistries []ChartRegistry `yaml:"chartRegistries,omitempty"`

	// DefaultNamespace is the default namespace for operations
	DefaultNamespace string `yaml:"defaultNamespace,omitempty"`

	// Editor is the preferred editor for values editing
	Editor string `yaml:"editor,omitempty"`

	// Theme is the color theme (future use)
	Theme string `yaml:"theme,omitempty"`

	// LastFeatureRequestTime tracks when the last feature request was sent (for rate limiting)
	LastFeatureRequestTime time.Time `yaml:"lastFeatureRequestTime,omitempty"`

	// SecurityScanEnabled controls whether Trivy security scanning is enabled
	SecurityScanEnabled bool `yaml:"securityScanEnabled,omitempty"`

	// ShowWelcomeOnStart controls whether the welcome page shows on app launch
	ShowWelcomeOnStart bool `yaml:"showWelcomeOnStart"`

	// StackTemplates stores saved multi-chart installation configurations
	StackTemplates []StackTemplate `yaml:"stackTemplates,omitempty"`

	// FeatureRequest holds webhook settings for feature requests
	FeatureRequest FeatureRequestConfig `yaml:"featureRequest,omitempty"`
}

const (
	// DefaultRegistryName is the name for the default Artifact Hub
	DefaultRegistryName = "Artifact Hub"

	// DefaultRegistryURL is the public Artifact Hub API
	DefaultRegistryURL = "https://artifacthub.io/api/v1"

	// ConfigDir is the directory name for config files
	ConfigDir = "helmx"

	// ConfigFile is the config file name
	ConfigFile = "config.yaml"
)

// configPath returns the full path to the config file
func configPath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		// Fallback to home directory
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		configDir = filepath.Join(home, ".config")
	}

	return filepath.Join(configDir, ConfigDir, ConfigFile), nil
}

// Load reads the config file or returns defaults
func Load() (*Config, error) {
	cfg := &Config{
		ChartRegistries: []ChartRegistry{
			{Name: DefaultRegistryName, URL: DefaultRegistryURL},
		},
		DefaultNamespace:    "",
		Editor:              "",
		Theme:               "default",
		SecurityScanEnabled: true,
		ShowWelcomeOnStart:  true,
	}

	path, err := configPath()
	if err != nil {
		return cfg, nil // Return defaults on error
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil // Return defaults if no config file
		}
		return cfg, err
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return cfg, err
	}

	// If no registries configured, add default
	if len(cfg.ChartRegistries) == 0 {
		cfg.ChartRegistries = []ChartRegistry{
			{Name: DefaultRegistryName, URL: DefaultRegistryURL},
		}
	}

	if cfg.Theme == "" {
		cfg.Theme = "default"
	}

	return cfg, nil
}

// Config file permission constants
const (
	// ConfigDirPerm restricts config directory to owner only
	ConfigDirPerm = 0700
	// ConfigFilePerm restricts config file to owner read/write only
	// This is important as config may contain API keys or sensitive settings
	ConfigFilePerm = 0600
)

// Save writes the config to file with secure permissions
func (c *Config) Save() error {
	path, err := configPath()
	if err != nil {
		return err
	}

	// Create directory if needed with restricted permissions
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, ConfigDirPerm); err != nil {
		return err
	}

	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, ConfigFilePerm)
}

// GetChartRegistries returns the list of configured registries
func (c *Config) GetChartRegistries() []ChartRegistry {
	if len(c.ChartRegistries) == 0 {
		return []ChartRegistry{{Name: DefaultRegistryName, URL: DefaultRegistryURL}}
	}
	return c.ChartRegistries
}

// AddRegistry adds a new chart registry
func (c *Config) AddRegistry(name, url string) {
	// Check if already exists
	for _, r := range c.ChartRegistries {
		if r.URL == url {
			return
		}
	}
	c.ChartRegistries = append(c.ChartRegistries, ChartRegistry{Name: name, URL: url})
}

// RemoveRegistry removes a chart registry by URL
func (c *Config) RemoveRegistry(url string) {
	var filtered []ChartRegistry
	for _, r := range c.ChartRegistries {
		if r.URL != url {
			filtered = append(filtered, r)
		}
	}
	c.ChartRegistries = filtered
}

// HasRegistry checks if a registry URL is configured
func (c *Config) HasRegistry(url string) bool {
	for _, r := range c.ChartRegistries {
		if r.URL == url {
			return true
		}
	}
	return false
}

// safeEditors is an allowlist of known safe editor commands
var safeEditors = map[string]bool{
	"vim": true, "vi": true, "nvim": true, "nano": true, "emacs": true,
	"code": true, "subl": true, "micro": true, "helix": true, "hx": true,
	"gedit": true, "kate": true, "kwrite": true, "pluma": true, "xed": true,
	"ne": true, "joe": true, "pico": true, "zed": true,
}

// shellMetachars contains characters that could enable command injection
var shellMetachars = regexp.MustCompile("[;&|$`\"'\\\\<>(){}\\[\\]!#*?~]")

// isEditorSafe validates an editor command for security
func isEditorSafe(editor string) bool {
	if editor == "" {
		return false
	}

	trimmed := strings.TrimSpace(editor)
	if trimmed == "" {
		return false
	}

	// Reject shell metacharacters
	if shellMetachars.MatchString(editor) {
		return false
	}

	parts := strings.Fields(trimmed)
	baseCmd := parts[0]
	baseName := filepath.Base(baseCmd)

	// Check allowlist
	if safeEditors[baseName] {
		if _, err := exec.LookPath(baseCmd); err == nil {
			return true
		}
	}

	// For non-allowlisted, reject multi-word commands and verify existence
	if len(parts) > 1 {
		return false
	}
	if _, err := exec.LookPath(baseCmd); err != nil {
		return false
	}

	return true
}

// GetEditor returns the configured editor or falls back to $EDITOR/vim
// Validates the editor command for security to prevent command injection
func (c *Config) GetEditor() string {
	// Try config editor first
	if c.Editor != "" && isEditorSafe(c.Editor) {
		return c.Editor
	}

	// Try $EDITOR
	if editor := os.Getenv("EDITOR"); editor != "" && isEditorSafe(editor) {
		return editor
	}

	// Safe fallback chain
	for _, fallback := range []string{"vim", "vi", "nano"} {
		if _, err := exec.LookPath(fallback); err == nil {
			return fallback
		}
	}

	return "vi" // Ultimate fallback
}

// HasCustomRegistries returns true if using non-default registries
func (c *Config) HasCustomRegistries() bool {
	if len(c.ChartRegistries) == 0 {
		return false
	}
	if len(c.ChartRegistries) == 1 && c.ChartRegistries[0].URL == DefaultRegistryURL {
		return false
	}
	return true
}

// ConfigPath returns the path to the config file (for display)
func ConfigPath() string {
	path, err := configPath()
	if err != nil {
		return "~/.config/helmx/config.yaml"
	}
	return path
}

// AddStack adds a new stack template. Returns error if name already exists.
func (c *Config) AddStack(stack StackTemplate) error {
	// Check for duplicate name
	for _, s := range c.StackTemplates {
		if s.Name == stack.Name {
			return ErrStackExists
		}
	}
	stack.CreatedAt = time.Now()
	c.StackTemplates = append(c.StackTemplates, stack)
	return nil
}

// GetStacks returns all stack templates
func (c *Config) GetStacks() []StackTemplate {
	return c.StackTemplates
}

// GetStack returns a stack by name
func (c *Config) GetStack(name string) (StackTemplate, bool) {
	for _, s := range c.StackTemplates {
		if s.Name == name {
			return s, true
		}
	}
	return StackTemplate{}, false
}

// UpdateStack updates an existing stack by name
func (c *Config) UpdateStack(name string, charts []StackChartItem, description string) error {
	for i, s := range c.StackTemplates {
		if s.Name == name {
			c.StackTemplates[i].Charts = charts
			if description != "" {
				c.StackTemplates[i].Description = description
			}
			return nil
		}
	}
	return ErrStackNotFound
}

// DeleteStack removes a stack by name
func (c *Config) DeleteStack(name string) error {
	for i, s := range c.StackTemplates {
		if s.Name == name {
			c.StackTemplates = append(c.StackTemplates[:i], c.StackTemplates[i+1:]...)
			return nil
		}
	}
	return ErrStackNotFound
}

// GetFeatureRequestURL returns the webhook URL for feature requests.
// Priority: environment variable > config file > empty (disabled)
func (c *Config) GetFeatureRequestURL() string {
	if url := os.Getenv("HELMX_FEATURE_REQUEST_URL"); url != "" {
		return url
	}
	return c.FeatureRequest.URL
}

// GetFeatureRequestAPIKey returns the API key for feature requests.
// Priority: environment variable > config file > empty
func (c *Config) GetFeatureRequestAPIKey() string {
	if key := os.Getenv("HELMX_FEATURE_REQUEST_API_KEY"); key != "" {
		return key
	}
	return c.FeatureRequest.APIKey
}

// IsFeatureRequestEnabled returns true if feature request webhook is configured.
func (c *Config) IsFeatureRequestEnabled() bool {
	return c.GetFeatureRequestURL() != ""
}

// Package validation provides input validation functions for the TUI.
package validation

import (
	"fmt"
	"net/url"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// Kubernetes naming constraints
const (
	// MaxNameLength is the maximum length for Kubernetes resource names
	MaxNameLength = 63
	// MaxNamespaceLength is the maximum length for Kubernetes namespaces
	MaxNamespaceLength = 63
)

// DNS1123 regex for valid Kubernetes names
var dns1123Regex = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`)

// ValidationError represents a validation failure with context.
type ValidationError struct {
	Field   string
	Message string
}

func (e ValidationError) Error() string {
	if e.Field != "" {
		return fmt.Sprintf("%s: %s", e.Field, e.Message)
	}
	return e.Message
}

// ValidateReleaseName validates a Helm release name.
// Returns nil if valid, error otherwise.
func ValidateReleaseName(name string) error {
	if name == "" {
		return ValidationError{Field: "release name", Message: "cannot be empty"}
	}
	if len(name) > MaxNameLength {
		return ValidationError{
			Field:   "release name",
			Message: fmt.Sprintf("cannot exceed %d characters (got %d)", MaxNameLength, len(name)),
		}
	}
	if !dns1123Regex.MatchString(name) {
		return ValidationError{
			Field:   "release name",
			Message: "must be lowercase alphanumeric, may contain dashes, must start/end with alphanumeric",
		}
	}
	return nil
}

// ValidateNamespace validates a Kubernetes namespace name.
// Returns nil if valid, error otherwise.
func ValidateNamespace(ns string) error {
	if ns == "" {
		return ValidationError{Field: "namespace", Message: "cannot be empty"}
	}
	if len(ns) > MaxNamespaceLength {
		return ValidationError{
			Field:   "namespace",
			Message: fmt.Sprintf("cannot exceed %d characters (got %d)", MaxNamespaceLength, len(ns)),
		}
	}
	if !dns1123Regex.MatchString(ns) {
		return ValidationError{
			Field:   "namespace",
			Message: "must be lowercase alphanumeric, may contain dashes, must start/end with alphanumeric",
		}
	}
	return nil
}

// ValidateTargetNamespace validates a Kubernetes namespace name used as an RBAC target.
// Allows "*" as a special value meaning cluster-wide (all namespaces).
func ValidateTargetNamespace(ns string) error {
	if ns == "*" {
		return nil // cluster-wide wildcard
	}
	return ValidateNamespace(ns)
}

// IsValidTargetNamespace returns true if the namespace is valid as an RBAC target (including "*").
func IsValidTargetNamespace(ns string) bool {
	return ValidateTargetNamespace(ns) == nil
}

// ValidateURL validates a URL string.
// Returns nil if valid, error otherwise.
func ValidateURL(rawURL string) error {
	if rawURL == "" {
		return ValidationError{Field: "URL", Message: "cannot be empty"}
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return ValidationError{Field: "URL", Message: err.Error()}
	}
	if parsed.Scheme == "" {
		return ValidationError{Field: "URL", Message: "must include scheme (http:// or https://)"}
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" && parsed.Scheme != "oci" {
		return ValidationError{
			Field:   "URL",
			Message: fmt.Sprintf("scheme must be http, https, or oci (got %s)", parsed.Scheme),
		}
	}
	if parsed.Host == "" {
		return ValidationError{Field: "URL", Message: "must include host"}
	}
	return nil
}

// ValidateYAML validates YAML content.
// Returns nil if valid, error with details otherwise.
func ValidateYAML(content string) error {
	if strings.TrimSpace(content) == "" {
		return nil // Empty YAML is valid
	}
	var parsed interface{}
	if err := yaml.Unmarshal([]byte(content), &parsed); err != nil {
		return ValidationError{Field: "YAML", Message: err.Error()}
	}
	return nil
}

// ValidateYAMLValues validates YAML content and ensures it's a map.
// Helm values must be key-value pairs at the top level.
func ValidateYAMLValues(content string) error {
	if strings.TrimSpace(content) == "" {
		return nil // Empty values are valid
	}
	var parsed map[string]interface{}
	if err := yaml.Unmarshal([]byte(content), &parsed); err != nil {
		return ValidationError{
			Field:   "values",
			Message: fmt.Sprintf("must be valid YAML key-value pairs: %s", err.Error()),
		}
	}
	return nil
}

// ValidatePath validates a file path.
// Returns nil if valid format, error otherwise.
// Note: Does not check if file exists.
func ValidatePath(path string) error {
	if path == "" {
		return ValidationError{Field: "path", Message: "cannot be empty"}
	}
	// Check for obviously invalid characters
	if strings.ContainsAny(path, "\x00") {
		return ValidationError{Field: "path", Message: "contains invalid characters"}
	}
	return nil
}

// ValidatePortNumber validates a port number.
// Returns nil if valid (1-65535), error otherwise.
func ValidatePortNumber(port int) error {
	if port < 1 || port > 65535 {
		return ValidationError{
			Field:   "port",
			Message: fmt.Sprintf("must be between 1 and 65535 (got %d)", port),
		}
	}
	return nil
}

// ValidateNotEmpty validates that a string is not empty or whitespace only.
func ValidateNotEmpty(value, fieldName string) error {
	if strings.TrimSpace(value) == "" {
		return ValidationError{Field: fieldName, Message: "cannot be empty"}
	}
	return nil
}

// ValidateMaxLength validates that a string does not exceed a maximum length.
func ValidateMaxLength(value, fieldName string, maxLen int) error {
	if len(value) > maxLen {
		return ValidationError{
			Field:   fieldName,
			Message: fmt.Sprintf("cannot exceed %d characters (got %d)", maxLen, len(value)),
		}
	}
	return nil
}

// IsValidReleaseName returns true if the name is a valid Helm release name.
func IsValidReleaseName(name string) bool {
	return ValidateReleaseName(name) == nil
}

// IsValidNamespace returns true if the namespace is a valid Kubernetes namespace.
func IsValidNamespace(ns string) bool {
	return ValidateNamespace(ns) == nil
}

// IsValidURL returns true if the URL is valid.
func IsValidURL(rawURL string) bool {
	return ValidateURL(rawURL) == nil
}

// IsValidYAML returns true if the content is valid YAML.
func IsValidYAML(content string) bool {
	return ValidateYAML(content) == nil
}

// ValidateOutputPath validates an output file path for security.
// Prevents path traversal attacks by:
// - Rejecting paths with ".." components
// - Rejecting absolute paths (must be relative to current directory)
// - Rejecting paths starting with ~/
func ValidateOutputPath(path string) error {
	if path == "" {
		return ValidationError{Field: "output path", Message: "cannot be empty"}
	}

	// Clean the path to normalize it
	cleaned := filepath.Clean(path)

	// Reject absolute paths
	if filepath.IsAbs(cleaned) {
		return ValidationError{
			Field:   "output path",
			Message: "must be a relative path, not an absolute path",
		}
	}

	// Reject paths that escape current directory
	if strings.HasPrefix(cleaned, "..") || strings.Contains(cleaned, string(filepath.Separator)+"..") {
		return ValidationError{
			Field:   "output path",
			Message: "path traversal not allowed (cannot use '..')",
		}
	}

	// Reject home directory references
	if strings.HasPrefix(path, "~/") || strings.HasPrefix(path, "~\\") {
		return ValidationError{
			Field:   "output path",
			Message: "home directory paths not allowed (use relative path)",
		}
	}

	// Check for null bytes
	if strings.ContainsAny(path, "\x00") {
		return ValidationError{Field: "output path", Message: "contains invalid characters"}
	}

	return nil
}

// safeEditors is an allowlist of known safe editor commands
var safeEditors = map[string]bool{
	"vim":    true,
	"vi":     true,
	"nvim":   true,
	"nano":   true,
	"emacs":  true,
	"code":   true, // VS Code
	"subl":   true, // Sublime Text
	"micro":  true,
	"helix":  true,
	"hx":     true,
	"gedit":  true,
	"kate":   true,
	"kwrite": true,
	"pluma":  true,
	"xed":    true,
	"ne":     true,
	"joe":    true,
	"pico":   true,
	"zed":    true,
}

// shellMetachars contains characters that could enable command injection
var shellMetachars = regexp.MustCompile("[;&|$`\"'\\\\<>(){}\\[\\]!#*?~]")

// ValidateEditor validates an editor command for security.
// Prevents command injection by:
// - Rejecting shell metacharacters in any part of the command
// - Verifying the editor exists via exec.LookPath
// - Using an allowlist of known safe editors
func ValidateEditor(editor string) error {
	if editor == "" {
		return ValidationError{Field: "editor", Message: "cannot be empty"}
	}

	// Reject whitespace-only
	trimmed := strings.TrimSpace(editor)
	if trimmed == "" {
		return ValidationError{Field: "editor", Message: "cannot be empty"}
	}

	// ALWAYS check for shell metacharacters first - this is critical for security
	if shellMetachars.MatchString(editor) {
		return ValidationError{
			Field:   "editor",
			Message: "contains shell metacharacters which are not allowed",
		}
	}

	// Extract base command (first word)
	parts := strings.Fields(trimmed)
	baseCmd := parts[0]

	// Get the base name for allowlist check
	baseName := filepath.Base(baseCmd)

	// Check if it's in the safe editors list
	if safeEditors[baseName] {
		// For safe editors, verify it exists
		if _, err := exec.LookPath(baseCmd); err != nil {
			return ValidationError{
				Field:   "editor",
				Message: fmt.Sprintf("editor '%s' not found in PATH", baseCmd),
			}
		}
		return nil
	}

	// For non-allowlisted editors, reject multi-word commands
	if len(parts) > 1 {
		return ValidationError{
			Field:   "editor",
			Message: "editor command with arguments not allowed for non-standard editors",
		}
	}

	// Verify the editor exists
	if _, err := exec.LookPath(baseCmd); err != nil {
		return ValidationError{
			Field:   "editor",
			Message: fmt.Sprintf("editor '%s' not found in PATH", baseCmd),
		}
	}

	return nil
}

// GetSafeEditor returns a validated editor command.
// It tries the provided editor first, then falls back through safe defaults.
// Returns the first valid editor found, or "vi" as ultimate fallback.
func GetSafeEditor(preferred string) string {
	// Try preferred editor if provided and valid
	if preferred != "" {
		if err := ValidateEditor(preferred); err == nil {
			return preferred
		}
	}

	// Fallback chain of safe editors
	fallbacks := []string{"vim", "vi", "nano"}
	for _, editor := range fallbacks {
		if _, err := exec.LookPath(editor); err == nil {
			return editor
		}
	}

	// Ultimate fallback (may not exist, but is safe)
	return "vi"
}

// IsValidOutputPath returns true if the path is a valid output path.
func IsValidOutputPath(path string) bool {
	return ValidateOutputPath(path) == nil
}

// IsValidEditor returns true if the editor is valid.
func IsValidEditor(editor string) bool {
	return ValidateEditor(editor) == nil
}

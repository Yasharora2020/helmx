package helm

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
)

// SecretsError represents errors from helm-secrets operations
type SecretsError struct {
	Op      string // Operation: "detect", "decrypt", "encrypt", "view"
	Err     error
	Details string
}

func (e *SecretsError) Error() string {
	if e.Details != "" {
		return fmt.Sprintf("%s: %v - %s", e.Op, e.Err, e.Details)
	}
	return fmt.Sprintf("%s: %v", e.Op, e.Err)
}

func (e *SecretsError) Unwrap() error {
	return e.Err
}

// ErrPluginNotInstalled is returned when helm-secrets plugin is not available
var ErrPluginNotInstalled = errors.New("helm-secrets plugin not installed")

// SecretsClient wraps helm-secrets plugin operations
type SecretsClient struct {
	pluginVersion string
	available     bool
	checked       bool
	mu            sync.RWMutex
}

// NewSecretsClient creates a new secrets client
func NewSecretsClient() *SecretsClient {
	return &SecretsClient{}
}

// IsAvailable checks if helm-secrets plugin is installed
func (s *SecretsClient) IsAvailable() bool {
	s.mu.RLock()
	if s.checked {
		available := s.available
		s.mu.RUnlock()
		return available
	}
	s.mu.RUnlock()

	s.mu.Lock()
	defer s.mu.Unlock()

	// Double-check after acquiring write lock
	if s.checked {
		return s.available
	}

	s.checked = true
	cmd := exec.Command("helm", "secrets", "--help")
	if err := cmd.Run(); err != nil {
		s.available = false
		return false
	}

	s.available = true
	s.detectVersion()
	return true
}

// detectVersion extracts the helm-secrets version
func (s *SecretsClient) detectVersion() {
	cmd := exec.Command("helm", "secrets", "version")
	out, err := cmd.Output()
	if err == nil {
		s.pluginVersion = strings.TrimSpace(string(out))
	}
}

// GetVersion returns the helm-secrets plugin version
func (s *SecretsClient) GetVersion() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.pluginVersion
}

// IsEncrypted checks if content is SOPS-encrypted
func (s *SecretsClient) IsEncrypted(content string) bool {
	return IsSOPSEncrypted(content)
}

// IsEncryptedFile checks if a file appears to be encrypted
func (s *SecretsClient) IsEncryptedFile(path string) (bool, error) {
	// Check filename conventions first (fast path)
	if IsEncryptedFilename(path) {
		return true, nil
	}

	// Read file and check content
	data, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}

	return IsSOPSEncrypted(string(data)), nil
}

// Decrypt decrypts SOPS-encrypted content
// Returns decrypted content and any error
func (s *SecretsClient) Decrypt(content string) (string, error) {
	if !s.IsAvailable() {
		return "", &SecretsError{
			Op:      "decrypt",
			Err:     ErrPluginNotInstalled,
			Details: "Install with: helm plugin install https://github.com/jkroepke/helm-secrets",
		}
	}

	// Write content to temp file (helm secrets needs a file)
	tmpFile, err := os.CreateTemp("", "helmx-decrypt-*.yaml")
	if err != nil {
		return "", &SecretsError{Op: "decrypt", Err: err}
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath) // Clean up immediately after use

	if _, err := tmpFile.WriteString(content); err != nil {
		_ = tmpFile.Close()
		return "", &SecretsError{Op: "decrypt", Err: err}
	}
	_ = tmpFile.Close()

	// Run helm secrets decrypt
	cmd := exec.Command("helm", "secrets", "decrypt", tmpPath)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		details := strings.TrimSpace(stderr.String())
		return "", &SecretsError{
			Op:      "decrypt",
			Err:     err,
			Details: parseSecretsError(details),
		}
	}

	return stdout.String(), nil
}

// DecryptFile decrypts a SOPS-encrypted file
func (s *SecretsClient) DecryptFile(path string) (string, error) {
	if !s.IsAvailable() {
		return "", &SecretsError{
			Op:      "decrypt",
			Err:     ErrPluginNotInstalled,
			Details: "Install with: helm plugin install https://github.com/jkroepke/helm-secrets",
		}
	}

	cmd := exec.Command("helm", "secrets", "decrypt", path)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		details := strings.TrimSpace(stderr.String())
		return "", &SecretsError{
			Op:      "decrypt",
			Err:     err,
			Details: parseSecretsError(details),
		}
	}

	return stdout.String(), nil
}

// View decrypts and returns content without modifying the file
// Uses "helm secrets view" which is safer than decrypt
func (s *SecretsClient) View(path string) (string, error) {
	if !s.IsAvailable() {
		return "", &SecretsError{
			Op:  "view",
			Err: ErrPluginNotInstalled,
		}
	}

	cmd := exec.Command("helm", "secrets", "view", path)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		details := strings.TrimSpace(stderr.String())
		return "", &SecretsError{
			Op:      "view",
			Err:     err,
			Details: parseSecretsError(details),
		}
	}

	return stdout.String(), nil
}

// Encrypt encrypts content using SOPS
// The content must be plain YAML, and the result will be SOPS-encrypted
func (s *SecretsClient) Encrypt(content string) (string, error) {
	if !s.IsAvailable() {
		return "", &SecretsError{
			Op:  "encrypt",
			Err: ErrPluginNotInstalled,
		}
	}

	// Create temp file with plain content
	tmpFile, err := os.CreateTemp("", "helmx-encrypt-*.yaml")
	if err != nil {
		return "", &SecretsError{Op: "encrypt", Err: err}
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	if _, err := tmpFile.WriteString(content); err != nil {
		_ = tmpFile.Close()
		return "", &SecretsError{Op: "encrypt", Err: err}
	}
	_ = tmpFile.Close()

	// Run helm secrets encrypt (encrypts in place)
	cmd := exec.Command("helm", "secrets", "encrypt", tmpPath)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		details := strings.TrimSpace(stderr.String())
		return "", &SecretsError{
			Op:      "encrypt",
			Err:     err,
			Details: parseSecretsError(details),
		}
	}

	// Read encrypted content
	encrypted, err := os.ReadFile(tmpPath)
	if err != nil {
		return "", &SecretsError{Op: "encrypt", Err: err}
	}

	return string(encrypted), nil
}

// EncryptInPlace encrypts a file in place
func (s *SecretsClient) EncryptInPlace(path string) error {
	if !s.IsAvailable() {
		return &SecretsError{
			Op:  "encrypt",
			Err: ErrPluginNotInstalled,
		}
	}

	cmd := exec.Command("helm", "secrets", "encrypt", path)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		details := strings.TrimSpace(stderr.String())
		return &SecretsError{
			Op:      "encrypt",
			Err:     err,
			Details: parseSecretsError(details),
		}
	}

	return nil
}

// parseSecretsError extracts meaningful error messages from helm-secrets output
func parseSecretsError(stderr string) string {
	// Common error patterns
	if strings.Contains(stderr, "could not find key") || strings.Contains(stderr, "key not found") {
		return "Encryption key not found. Ensure your GPG/age key is available."
	}
	if strings.Contains(stderr, "permission denied") {
		return "Permission denied accessing encryption keys."
	}
	if strings.Contains(stderr, "could not read config") || strings.Contains(stderr, ".sops.yaml") {
		return "SOPS configuration not found. Create a .sops.yaml file."
	}
	if strings.Contains(stderr, "no matching keys") {
		return "No matching encryption keys found for this file."
	}
	if strings.Contains(stderr, "kms") && strings.Contains(stderr, "error") {
		return "AWS KMS error. Check your AWS credentials and permissions."
	}
	if strings.Contains(stderr, "age") && strings.Contains(stderr, "identity") {
		return "Age identity not found. Set SOPS_AGE_KEY_FILE or add to ~/.config/sops/age/keys.txt"
	}

	// Return original if no pattern matched
	if stderr != "" {
		return stderr
	}
	return "Unknown error"
}

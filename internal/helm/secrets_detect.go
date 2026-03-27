package helm

import (
	"regexp"
	"strings"
)

// SOPS encryption patterns
var (
	// Pattern for SOPS metadata key at root level
	sopsKeyPattern = regexp.MustCompile(`(?m)^sops:\s*$`)

	// Pattern for inline SOPS encrypted values
	// Example: ENC[AES256_GCM,data:...,iv:...,tag:...,type:str]
	sopsInlinePattern = regexp.MustCompile(`ENC\[AES256_GCM,data:[^,]+,`)

	// Pattern for age-encrypted values
	agePattern = regexp.MustCompile(`ENC\[age,data:[^,]+,`)

	// Common encrypted filename patterns
	encryptedFilePatterns = []string{
		".enc.yaml",
		".enc.yml",
		"secrets.yaml",
		"secrets.yml",
		".sops.yaml",
		".sops.yml",
	}
)

// IsSOPSEncrypted checks if content contains SOPS encryption markers
func IsSOPSEncrypted(content string) bool {
	// Check for sops: key (most reliable indicator)
	if sopsKeyPattern.MatchString(content) {
		return true
	}

	// Check for inline encrypted values
	if sopsInlinePattern.MatchString(content) {
		return true
	}

	// Check for age encryption
	if agePattern.MatchString(content) {
		return true
	}

	return false
}

// IsEncryptedFilename checks if filename follows encrypted file conventions
func IsEncryptedFilename(path string) bool {
	lowerPath := strings.ToLower(path)
	for _, pattern := range encryptedFilePatterns {
		if strings.HasSuffix(lowerPath, pattern) {
			return true
		}
	}
	return false
}

// EncryptionType represents the type of encryption used
type EncryptionType string

const (
	EncryptionNone    EncryptionType = "none"
	EncryptionPGP     EncryptionType = "pgp"
	EncryptionAge     EncryptionType = "age"
	EncryptionAWSKMS  EncryptionType = "aws-kms"
	EncryptionGCPKMS  EncryptionType = "gcp-kms"
	EncryptionAzureKV EncryptionType = "azure-kv"
	EncryptionVault   EncryptionType = "vault"
	EncryptionSOPS    EncryptionType = "sops" // Generic SOPS when type unknown
)

// DetectEncryptionType returns the type of encryption detected in content
func DetectEncryptionType(content string) EncryptionType {
	if !IsSOPSEncrypted(content) {
		return EncryptionNone
	}

	// Check the sops block for specific encryption types
	if sopsKeyPattern.MatchString(content) {
		// Check for age encryption
		if strings.Contains(content, "age:") && strings.Contains(content, "recipient:") {
			return EncryptionAge
		}
		// Check for PGP encryption
		if strings.Contains(content, "pgp:") || strings.Contains(content, "fp:") {
			return EncryptionPGP
		}
		// Check for AWS KMS
		if strings.Contains(content, "kms:") && strings.Contains(content, "arn:") {
			return EncryptionAWSKMS
		}
		// Check for GCP KMS
		if strings.Contains(content, "gcp_kms:") {
			return EncryptionGCPKMS
		}
		// Check for Azure Key Vault
		if strings.Contains(content, "azure_kv:") {
			return EncryptionAzureKV
		}
		// Check for HashiCorp Vault
		if strings.Contains(content, "hc_vault:") {
			return EncryptionVault
		}
		return EncryptionSOPS
	}

	// Check inline patterns
	if sopsInlinePattern.MatchString(content) {
		return EncryptionSOPS
	}

	if agePattern.MatchString(content) {
		return EncryptionAge
	}

	return EncryptionSOPS
}

// GetEncryptedValuePaths extracts paths to encrypted values in YAML
// Returns a list of dot-notation paths (e.g., "database.password")
func GetEncryptedValuePaths(content string) []string {
	var paths []string

	lines := strings.Split(content, "\n")
	currentPath := []string{}
	indentStack := []int{-1}

	for _, line := range lines {
		// Skip empty lines and comments
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		// Skip the sops metadata block
		if trimmed == "sops:" {
			break
		}

		// Calculate indentation
		indent := len(line) - len(strings.TrimLeft(line, " \t"))

		// Pop from stack if we've dedented
		for len(indentStack) > 1 && indent <= indentStack[len(indentStack)-1] {
			indentStack = indentStack[:len(indentStack)-1]
			if len(currentPath) > 0 {
				currentPath = currentPath[:len(currentPath)-1]
			}
		}

		// Check if this line has a key
		colonIdx := strings.Index(trimmed, ":")
		if colonIdx > 0 {
			key := trimmed[:colonIdx]
			value := strings.TrimSpace(trimmed[colonIdx+1:])

			// Update path
			if indent > indentStack[len(indentStack)-1] {
				indentStack = append(indentStack, indent)
				currentPath = append(currentPath, key)
			} else {
				if len(currentPath) > 0 {
					currentPath[len(currentPath)-1] = key
				} else {
					currentPath = append(currentPath, key)
				}
			}

			// Check if value is encrypted
			if sopsInlinePattern.MatchString(value) || agePattern.MatchString(value) {
				paths = append(paths, strings.Join(currentPath, "."))
			}
		}
	}

	return paths
}

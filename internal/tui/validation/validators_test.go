package validation

import (
	"strings"
	"testing"
)

func TestValidationError(t *testing.T) {
	tests := []struct {
		name     string
		err      ValidationError
		expected string
	}{
		{
			name:     "with field",
			err:      ValidationError{Field: "release name", Message: "cannot be empty"},
			expected: "release name: cannot be empty",
		},
		{
			name:     "without field",
			err:      ValidationError{Field: "", Message: "something went wrong"},
			expected: "something went wrong",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.err.Error()
			if got != tt.expected {
				t.Errorf("ValidationError.Error() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestValidateReleaseName(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		// Valid names
		{"simple name", "my-release", false},
		{"single char", "a", false},
		{"numbers only", "123", false},
		{"mixed alphanumeric", "my-app-v2", false},
		{"long valid name", "a" + strings.Repeat("b", 61) + "c", false}, // exactly 63 chars
		{"single digit", "1", false},

		// Invalid - empty
		{"empty string", "", true},

		// Invalid - too long
		{"too long 64 chars", strings.Repeat("a", 64), true},
		{"too long 100 chars", strings.Repeat("a", 100), true},

		// Invalid - uppercase
		{"uppercase letters", "MyRelease", true},
		{"all uppercase", "RELEASE", true},
		{"mixed case", "my-Release", true},

		// Invalid - dashes at start/end
		{"dash at start", "-my-release", true},
		{"dash at end", "my-release-", true},
		{"only dashes", "---", true},
		{"single dash", "-", true},

		// Invalid - special characters
		{"underscore", "my_release", true},
		{"dot", "my.release", true},
		{"space", "my release", true},
		{"at sign", "my@release", true},
		{"exclamation", "release!", true},
		{"slash", "my/release", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateReleaseName(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateReleaseName(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestValidateNamespace(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		// Valid namespaces
		{"simple namespace", "default", false},
		{"with dashes", "kube-system", false},
		{"numbers", "ns-123", false},
		{"single char", "a", false},
		{"max length 63", strings.Repeat("a", 63), false},

		// Invalid - empty
		{"empty string", "", true},

		// Invalid - too long
		{"too long 64 chars", strings.Repeat("a", 64), true},

		// Invalid - uppercase
		{"uppercase", "Default", true},
		{"all uppercase", "NAMESPACE", true},

		// Invalid - dashes at start/end
		{"dash at start", "-namespace", true},
		{"dash at end", "namespace-", true},

		// Invalid - special characters
		{"underscore", "my_namespace", true},
		{"dot", "my.namespace", true},
		{"space", "my namespace", true},
		{"colon", "ns:test", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateNamespace(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateNamespace(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestValidateURL(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		// Valid URLs
		{"valid http", "http://example.com", false},
		{"valid https", "https://example.com", false},
		{"https with path", "https://charts.helm.sh/stable", false},
		{"https with port", "https://example.com:8080/path", false},
		{"oci scheme", "oci://registry.example.com/charts", false},

		// Invalid - empty
		{"empty string", "", true},

		// Invalid - missing scheme
		{"no scheme", "example.com", true},
		{"missing scheme with path", "example.com/charts", true},

		// Invalid - wrong scheme
		{"ftp scheme", "ftp://example.com", true},
		{"ssh scheme", "ssh://example.com", true},

		// Invalid - missing host
		{"http no host", "http://", true},
		{"https no host", "https://", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateURL(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateURL(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestValidateYAML(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		// Valid YAML
		{"simple key-value", "key: value", false},
		{"nested map", "parent:\n  child: value", false},
		{"list", "- item1\n- item2", false},
		{"number value", "replicas: 3", false},
		{"boolean value", "enabled: true", false},
		{"empty string", "", false},
		{"whitespace only", "   \n  \t  ", false},
		{"multiline string", "key: |\n  line1\n  line2", false},

		// Invalid YAML
		{"invalid yaml tabs", "key:\n\t\tbad: indent", true},
		{"unmatched bracket", "key: [unclosed", true},
		{"bad indentation", "parent:\n child: value\n  bad: indent", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateYAML(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateYAML(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestValidateYAMLValues(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		// Valid values (maps)
		{"simple key-value", "key: value", false},
		{"nested map", "parent:\n  child: value", false},
		{"multiple keys", "key1: val1\nkey2: val2", false},
		{"empty string", "", false},
		{"whitespace only", "   ", false},
		{"map with array value", "items:\n  - one\n  - two", false},
		{"nested objects", "outer:\n  inner:\n    deep: value", false},

		// Invalid - not a map
		{"plain array", "- item1\n- item2", true},
		{"scalar string", "just a string", true},
		{"scalar number", "42", true},

		// Invalid YAML entirely
		{"invalid yaml", "key: [unclosed", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateYAMLValues(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateYAMLValues(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestValidatePortNumber(t *testing.T) {
	tests := []struct {
		name    string
		port    int
		wantErr bool
	}{
		// Valid ports
		{"minimum port", 1, false},
		{"http port", 80, false},
		{"https port", 443, false},
		{"common alt port", 8080, false},
		{"maximum port", 65535, false},

		// Invalid ports
		{"zero", 0, true},
		{"negative", -1, true},
		{"large negative", -100, true},
		{"over max", 65536, true},
		{"way over max", 100000, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePortNumber(tt.port)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidatePortNumber(%d) error = %v, wantErr %v", tt.port, err, tt.wantErr)
			}
		})
	}
}

func TestValidateNotEmpty(t *testing.T) {
	tests := []struct {
		name      string
		value     string
		fieldName string
		wantErr   bool
	}{
		// Valid - not empty
		{"non-empty string", "hello", "field", false},
		{"string with spaces", "hello world", "field", false},
		{"single char", "x", "field", false},

		// Invalid - empty or whitespace
		{"empty string", "", "field", true},
		{"spaces only", "   ", "field", true},
		{"tab only", "\t", "field", true},
		{"newline only", "\n", "field", true},
		{"mixed whitespace", " \t\n ", "field", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateNotEmpty(tt.value, tt.fieldName)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateNotEmpty(%q, %q) error = %v, wantErr %v", tt.value, tt.fieldName, err, tt.wantErr)
			}
			// Verify the field name is in the error message
			if err != nil {
				ve, ok := err.(ValidationError)
				if !ok {
					t.Errorf("expected ValidationError, got %T", err)
				} else if ve.Field != tt.fieldName {
					t.Errorf("ValidationError.Field = %q, want %q", ve.Field, tt.fieldName)
				}
			}
		})
	}
}

func TestValidateMaxLength(t *testing.T) {
	tests := []struct {
		name      string
		value     string
		fieldName string
		maxLen    int
		wantErr   bool
	}{
		// Valid - under or at limit
		{"under limit", "hello", "field", 10, false},
		{"at limit", "hello", "field", 5, false},
		{"empty string", "", "field", 5, false},
		{"single char at limit 1", "a", "field", 1, false},

		// Invalid - over limit
		{"over limit by 1", "hello!", "field", 5, true},
		{"way over limit", "hello world", "field", 3, true},
		{"single char over zero limit", "a", "field", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateMaxLength(tt.value, tt.fieldName, tt.maxLen)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateMaxLength(%q, %q, %d) error = %v, wantErr %v", tt.value, tt.fieldName, tt.maxLen, err, tt.wantErr)
			}
		})
	}
}

func TestValidatePath(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		// Valid paths
		{"simple file", "file.yaml", false},
		{"with directory", "dir/file.yaml", false},
		{"absolute path", "/tmp/file.yaml", false},
		{"dot prefix", "./file.yaml", false},

		// Invalid paths
		{"empty path", "", true},
		{"null byte", "file\x00.yaml", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePath(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidatePath(%q) error = %v, wantErr %v", tt.path, err, tt.wantErr)
			}
		})
	}
}

func TestIsValidReleaseName(t *testing.T) {
	if IsValidReleaseName("") {
		t.Error("IsValidReleaseName accepted empty string")
	}
	if !IsValidReleaseName("valid-name") {
		t.Error("IsValidReleaseName rejected valid name")
	}
	if IsValidReleaseName("Invalid-Name") {
		t.Error("IsValidReleaseName accepted uppercase")
	}
}

func TestIsValidNamespace(t *testing.T) {
	if IsValidNamespace("") {
		t.Error("IsValidNamespace accepted empty string")
	}
	if !IsValidNamespace("default") {
		t.Error("IsValidNamespace rejected valid namespace")
	}
	if IsValidNamespace("Invalid") {
		t.Error("IsValidNamespace accepted uppercase")
	}
}

func TestIsValidURL(t *testing.T) {
	if IsValidURL("") {
		t.Error("IsValidURL accepted empty string")
	}
	if !IsValidURL("https://example.com") {
		t.Error("IsValidURL rejected valid URL")
	}
	if IsValidURL("not-a-url") {
		t.Error("IsValidURL accepted invalid URL")
	}
}

func TestIsValidYAML(t *testing.T) {
	if !IsValidYAML("key: value") {
		t.Error("IsValidYAML rejected valid YAML")
	}
	if !IsValidYAML("") {
		t.Error("IsValidYAML rejected empty string")
	}
	if IsValidYAML("key: [unclosed") {
		t.Error("IsValidYAML accepted invalid YAML")
	}
}

func TestValidateOutputPath(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		// Valid paths
		{"valid relative path", "output.yaml", false},
		{"valid subdir path", "manifests/output.yaml", false},
		{"valid nested path", "deploy/k8s/manifests.yaml", false},
		{"valid current dir", "./output.yaml", false},

		// Invalid paths - empty
		{"empty path", "", true},

		// Invalid paths - absolute
		{"absolute unix path", "/etc/passwd", true},
		{"absolute unix deep", "/tmp/output.yaml", true},

		// Invalid paths - traversal
		{"path traversal up", "../output.yaml", true},
		{"path traversal up twice", "../../etc/passwd", true},
		{"path traversal in middle", "foo/../../../etc/passwd", true},
		{"hidden traversal", "foo/bar/../../../output.yaml", true},

		// Invalid paths - home directory
		{"home directory", "~/output.yaml", true},
		{"home subdir", "~/.config/output.yaml", true},

		// Invalid paths - special chars
		{"null byte", "output\x00.yaml", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateOutputPath(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateOutputPath(%q) error = %v, wantErr %v", tt.path, err, tt.wantErr)
			}
		})
	}
}

func TestValidateEditor(t *testing.T) {
	tests := []struct {
		name    string
		editor  string
		wantErr bool
	}{
		// Empty
		{"empty", "", true},
		{"whitespace only", "   ", true},

		// Command injection attempts
		{"semicolon injection", "vim; rm -rf /", true},
		{"ampersand injection", "vim & malicious", true},
		{"pipe injection", "cat | malicious", true},
		{"backtick injection", "vim `whoami`", true},
		{"dollar injection", "vim $HOME", true},
		{"subshell injection", "vim $(whoami)", true},
		{"redirect injection", "vim > /etc/passwd", true},

		// Multi-word commands for non-allowlisted
		{"unknown with args", "myeditor -v", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateEditor(tt.editor)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateEditor(%q) error = %v, wantErr %v", tt.editor, err, tt.wantErr)
			}
		})
	}
}

func TestGetSafeEditor(t *testing.T) {
	// GetSafeEditor should always return a non-empty string
	editor := GetSafeEditor("")
	if editor == "" {
		t.Error("GetSafeEditor(\"\") returned empty string, want non-empty fallback")
	}

	// With malicious input, should fall back to safe editor
	editor = GetSafeEditor("vim; rm -rf /")
	if editor == "vim; rm -rf /" {
		t.Error("GetSafeEditor accepted malicious input")
	}
}

func TestIsValidOutputPath(t *testing.T) {
	if IsValidOutputPath("../etc/passwd") {
		t.Error("IsValidOutputPath accepted path traversal")
	}
	if !IsValidOutputPath("output.yaml") {
		t.Error("IsValidOutputPath rejected valid path")
	}
}

func TestIsValidEditor(t *testing.T) {
	if IsValidEditor("vim; rm -rf /") {
		t.Error("IsValidEditor accepted command injection")
	}
}

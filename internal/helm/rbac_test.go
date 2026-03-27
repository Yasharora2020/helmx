package helm

import (
	"testing"
)

func TestDeterminePermissionLevel(t *testing.T) {
	tests := []struct {
		name     string
		roleName string
		want     string
	}{
		{
			name:     "helmx managed read-only",
			roleName: "helmx-read-only",
			want:     PermissionReadOnly,
		},
		{
			name:     "helmx managed developer",
			roleName: "helmx-developer",
			want:     PermissionDeveloper,
		},
		{
			name:     "helmx managed namespace-admin",
			roleName: "helmx-namespace-admin",
			want:     PermissionNamespaceAdmin,
		},
		{
			name:     "cluster-admin role",
			roleName: "cluster-admin",
			want:     PermissionNamespaceAdmin,
		},
		{
			name:     "admin role",
			roleName: "admin",
			want:     PermissionNamespaceAdmin,
		},
		{
			name:     "edit role",
			roleName: "edit",
			want:     PermissionDeveloper,
		},
		{
			name:     "developer role",
			roleName: "my-developer",
			want:     PermissionDeveloper,
		},
		{
			name:     "view role",
			roleName: "view",
			want:     PermissionReadOnly,
		},
		{
			name:     "readonly role",
			roleName: "my-read-viewer",
			want:     PermissionReadOnly,
		},
		{
			name:     "custom role",
			roleName: "my-special-role",
			want:     PermissionCustom,
		},
		{
			name:     "empty role name",
			roleName: "",
			want:     PermissionCustom,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := determinePermissionLevel(tt.roleName)
			if got != tt.want {
				t.Errorf("determinePermissionLevel(%q) = %q, want %q", tt.roleName, got, tt.want)
			}
		})
	}
}

func TestFormatPermissionsSummary(t *testing.T) {
	tests := []struct {
		name        string
		permissions []NamespaceAccess
		wantEmpty   bool
		wantContain string
	}{
		{
			name:        "empty permissions",
			permissions: nil,
			wantEmpty:   false,
			wantContain: "no access",
		},
		{
			name: "single namespace",
			permissions: []NamespaceAccess{
				{Namespace: "dev", Permission: PermissionDeveloper},
			},
			wantEmpty:   false,
			wantContain: "dev",
		},
		{
			name: "cluster-wide access",
			permissions: []NamespaceAccess{
				{Namespace: "*", Permission: PermissionNamespaceAdmin},
			},
			wantEmpty:   false,
			wantContain: "all namespaces",
		},
		{
			name: "multiple namespaces same permission",
			permissions: []NamespaceAccess{
				{Namespace: "dev", Permission: PermissionReadOnly},
				{Namespace: "staging", Permission: PermissionReadOnly},
			},
			wantEmpty:   false,
			wantContain: "read-only",
		},
		{
			name: "many namespaces",
			permissions: []NamespaceAccess{
				{Namespace: "ns1", Permission: PermissionDeveloper},
				{Namespace: "ns2", Permission: PermissionDeveloper},
				{Namespace: "ns3", Permission: PermissionDeveloper},
				{Namespace: "ns4", Permission: PermissionDeveloper},
			},
			wantEmpty:   false,
			wantContain: "4 namespaces",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatPermissionsSummary(tt.permissions)
			if tt.wantEmpty && got != "" {
				t.Errorf("FormatPermissionsSummary() = %q, want empty", got)
			}
			if !tt.wantEmpty && got == "" {
				t.Errorf("FormatPermissionsSummary() returned empty, want non-empty")
			}
			if tt.wantContain != "" && got != "" {
				if !contains(got, tt.wantContain) {
					t.Errorf("FormatPermissionsSummary() = %q, want to contain %q", got, tt.wantContain)
				}
			}
		})
	}
}

func TestGetPermissionPresets(t *testing.T) {
	presets := GetPermissionPresets()

	if len(presets) != 3 {
		t.Errorf("GetPermissionPresets() returned %d presets, want 3", len(presets))
	}

	expected := []string{PermissionReadOnly, PermissionDeveloper, PermissionNamespaceAdmin}
	for i, p := range presets {
		if p != expected[i] {
			t.Errorf("GetPermissionPresets()[%d] = %q, want %q", i, p, expected[i])
		}
	}
}

func TestCreateRoleForPermission(t *testing.T) {
	tests := []struct {
		name       string
		permission string
		wantRules  int
	}{
		{
			name:       "read-only has multiple rules",
			permission: PermissionReadOnly,
			wantRules:  4,
		},
		{
			name:       "developer has multiple rules",
			permission: PermissionDeveloper,
			wantRules:  4,
		},
		{
			name:       "namespace-admin has single wildcard rule",
			permission: PermissionNamespaceAdmin,
			wantRules:  1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			role := createRoleForPermission("test-"+tt.permission, "test-ns", tt.permission)
			if len(role.Rules) != tt.wantRules {
				t.Errorf("createRoleForPermission(%q) has %d rules, want %d", tt.permission, len(role.Rules), tt.wantRules)
			}

			// Verify label
			if role.Labels["app.kubernetes.io/managed-by"] != "helmx" {
				t.Errorf("Role should have managed-by label set to 'helmx'")
			}
		})
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

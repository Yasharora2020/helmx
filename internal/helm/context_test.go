package helm

import (
	"os"
	"path/filepath"
	"testing"

	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
)

// createTestKubeconfig creates a temporary kubeconfig file for testing
func createTestKubeconfig(t *testing.T) (string, func()) {
	t.Helper()

	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "helmx-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	kubeconfigPath := filepath.Join(tmpDir, "config")

	// Create a test kubeconfig
	config := api.NewConfig()

	// Add clusters
	config.Clusters["cluster1"] = &api.Cluster{
		Server:                "https://cluster1.example.com:6443",
		InsecureSkipTLSVerify: true,
	}
	config.Clusters["cluster2"] = &api.Cluster{
		Server:                "https://cluster2.example.com:6443",
		InsecureSkipTLSVerify: true,
	}
	config.Clusters["production"] = &api.Cluster{
		Server:                "https://prod.example.com:6443",
		InsecureSkipTLSVerify: true,
	}

	// Add users
	config.AuthInfos["user1"] = &api.AuthInfo{
		Token: "test-token-1",
	}
	config.AuthInfos["user2"] = &api.AuthInfo{
		Token: "test-token-2",
	}
	config.AuthInfos["admin"] = &api.AuthInfo{
		Token: "admin-token",
	}

	// Add contexts
	config.Contexts["dev"] = &api.Context{
		Cluster:   "cluster1",
		AuthInfo:  "user1",
		Namespace: "development",
	}
	config.Contexts["staging"] = &api.Context{
		Cluster:   "cluster2",
		AuthInfo:  "user2",
		Namespace: "staging",
	}
	config.Contexts["prod"] = &api.Context{
		Cluster:   "production",
		AuthInfo:  "admin",
		Namespace: "production",
	}

	// Set current context
	config.CurrentContext = "dev"

	// Write the config
	if err := clientcmd.WriteToFile(*config, kubeconfigPath); err != nil {
		t.Fatalf("failed to write kubeconfig: %v", err)
	}

	// Register cleanup with testing.T
	t.Cleanup(func() {
		_ = os.RemoveAll(tmpDir)
	})

	return kubeconfigPath, func() {}
}

func TestListContexts(t *testing.T) {
	kubeconfigPath, cleanup := createTestKubeconfig(t)
	defer cleanup()

	client, err := NewClient(kubeconfigPath)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	contexts, err := client.ListContexts()
	if err != nil {
		t.Fatalf("ListContexts() error = %v", err)
	}

	// Should have 3 contexts
	if len(contexts) != 3 {
		t.Errorf("ListContexts() returned %d contexts; want 3", len(contexts))
	}

	// Verify contexts are sorted by name
	expectedNames := []string{"dev", "prod", "staging"}
	for i, expected := range expectedNames {
		if contexts[i].Name != expected {
			t.Errorf("contexts[%d].Name = %s; want %s", i, contexts[i].Name, expected)
		}
	}

	// Verify current context is marked
	foundCurrent := false
	for _, ctx := range contexts {
		if ctx.Name == "dev" {
			if !ctx.IsCurrent {
				t.Error("dev context should be marked as current")
			}
			foundCurrent = true
		} else {
			if ctx.IsCurrent {
				t.Errorf("context %s should not be marked as current", ctx.Name)
			}
		}
	}
	if !foundCurrent {
		t.Error("dev context not found")
	}
}

func TestListContextsDetails(t *testing.T) {
	kubeconfigPath, cleanup := createTestKubeconfig(t)
	defer cleanup()

	client, err := NewClient(kubeconfigPath)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	contexts, err := client.ListContexts()
	if err != nil {
		t.Fatalf("ListContexts() error = %v", err)
	}

	// Find dev context and verify details
	for _, ctx := range contexts {
		if ctx.Name == "dev" {
			if ctx.Cluster != "cluster1" {
				t.Errorf("dev context Cluster = %s; want cluster1", ctx.Cluster)
			}
			if ctx.User != "user1" {
				t.Errorf("dev context User = %s; want user1", ctx.User)
			}
			if ctx.Namespace != "development" {
				t.Errorf("dev context Namespace = %s; want development", ctx.Namespace)
			}
		}
	}
}

func TestSwitchContext(t *testing.T) {
	kubeconfigPath, cleanup := createTestKubeconfig(t)
	defer cleanup()

	client, err := NewClient(kubeconfigPath)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	// Initially current context is "dev"
	current := client.GetCurrentContext()
	if current != "dev" {
		t.Errorf("initial context = %s; want dev", current)
	}

	// Switch to staging
	if err := client.SwitchContext("staging"); err != nil {
		t.Fatalf("SwitchContext(staging) error = %v", err)
	}

	// Verify the switch
	current = client.GetCurrentContext()
	if current != "staging" {
		t.Errorf("after switch context = %s; want staging", current)
	}

	// Verify ListContexts shows the new current
	contexts, err := client.ListContexts()
	if err != nil {
		t.Fatalf("ListContexts() error = %v", err)
	}

	for _, ctx := range contexts {
		if ctx.Name == "staging" && !ctx.IsCurrent {
			t.Error("staging context should be marked as current after switch")
		}
		if ctx.Name == "dev" && ctx.IsCurrent {
			t.Error("dev context should not be marked as current after switch")
		}
	}
}

func TestSwitchContextNonExistent(t *testing.T) {
	kubeconfigPath, cleanup := createTestKubeconfig(t)
	defer cleanup()

	client, err := NewClient(kubeconfigPath)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	err = client.SwitchContext("nonexistent")
	if err == nil {
		t.Error("SwitchContext(nonexistent) should return an error")
	}
}

func TestGetContextInfo(t *testing.T) {
	kubeconfigPath, cleanup := createTestKubeconfig(t)
	defer cleanup()

	client, err := NewClient(kubeconfigPath)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	tests := []struct {
		name          string
		contextName   string
		wantErr       bool
		wantCluster   string
		wantUser      string
		wantNamespace string
	}{
		{
			name:          "dev context",
			contextName:   "dev",
			wantCluster:   "cluster1",
			wantUser:      "user1",
			wantNamespace: "development",
		},
		{
			name:          "prod context",
			contextName:   "prod",
			wantCluster:   "production",
			wantUser:      "admin",
			wantNamespace: "production",
		},
		{
			name:        "nonexistent context",
			contextName: "nonexistent",
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info, err := client.GetContextInfo(tt.contextName)
			if tt.wantErr {
				if err == nil {
					t.Error("GetContextInfo() should return an error")
				}
				return
			}
			if err != nil {
				t.Fatalf("GetContextInfo() error = %v", err)
			}
			if info.Cluster != tt.wantCluster {
				t.Errorf("Cluster = %s; want %s", info.Cluster, tt.wantCluster)
			}
			if info.User != tt.wantUser {
				t.Errorf("User = %s; want %s", info.User, tt.wantUser)
			}
			if info.Namespace != tt.wantNamespace {
				t.Errorf("Namespace = %s; want %s", info.Namespace, tt.wantNamespace)
			}
		})
	}
}

func TestValidateContext(t *testing.T) {
	kubeconfigPath, cleanup := createTestKubeconfig(t)
	defer cleanup()

	client, err := NewClient(kubeconfigPath)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	tests := []struct {
		name        string
		contextName string
		wantErr     bool
	}{
		{
			name:        "valid context",
			contextName: "dev",
			wantErr:     false,
		},
		{
			name:        "nonexistent context",
			contextName: "nonexistent",
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := client.ValidateContext(tt.contextName)
			if tt.wantErr && err == nil {
				t.Error("ValidateContext() should return an error")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("ValidateContext() error = %v", err)
			}
		})
	}
}

func TestRenameContext(t *testing.T) {
	kubeconfigPath, cleanup := createTestKubeconfig(t)
	defer cleanup()

	client, err := NewClient(kubeconfigPath)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	// Rename dev to development
	if err := client.RenameContext("dev", "development"); err != nil {
		t.Fatalf("RenameContext() error = %v", err)
	}

	// Verify old context is gone
	_, err = client.GetContextInfo("dev")
	if err == nil {
		t.Error("old context 'dev' should not exist")
	}

	// Verify new context exists
	info, err := client.GetContextInfo("development")
	if err != nil {
		t.Fatalf("new context 'development' not found: %v", err)
	}
	if info.Cluster != "cluster1" {
		t.Errorf("renamed context Cluster = %s; want cluster1", info.Cluster)
	}

	// Current context should be updated since dev was current
	current := client.GetCurrentContext()
	if current != "development" {
		t.Errorf("current context = %s; want development", current)
	}
}

func TestRenameContextToExisting(t *testing.T) {
	kubeconfigPath, cleanup := createTestKubeconfig(t)
	defer cleanup()

	client, err := NewClient(kubeconfigPath)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	// Try to rename dev to staging (which already exists)
	err = client.RenameContext("dev", "staging")
	if err == nil {
		t.Error("RenameContext() should return error when target exists")
	}
}

func TestDeleteContext(t *testing.T) {
	kubeconfigPath, cleanup := createTestKubeconfig(t)
	defer cleanup()

	client, err := NewClient(kubeconfigPath)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	// Delete staging (not current)
	if err := client.DeleteContext("staging"); err != nil {
		t.Fatalf("DeleteContext() error = %v", err)
	}

	// Verify staging is gone
	contexts, err := client.ListContexts()
	if err != nil {
		t.Fatalf("ListContexts() error = %v", err)
	}
	if len(contexts) != 2 {
		t.Errorf("after delete, got %d contexts; want 2", len(contexts))
	}
	for _, ctx := range contexts {
		if ctx.Name == "staging" {
			t.Error("staging context should have been deleted")
		}
	}
}

func TestDeleteCurrentContext(t *testing.T) {
	kubeconfigPath, cleanup := createTestKubeconfig(t)
	defer cleanup()

	client, err := NewClient(kubeconfigPath)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	// Try to delete current context (dev)
	err = client.DeleteContext("dev")
	if err == nil {
		t.Error("DeleteContext() should return error when deleting current context")
	}
}

func TestSetContextNamespace(t *testing.T) {
	kubeconfigPath, cleanup := createTestKubeconfig(t)
	defer cleanup()

	client, err := NewClient(kubeconfigPath)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	// Change namespace for dev context
	if err := client.SetContextNamespace("dev", "new-namespace"); err != nil {
		t.Fatalf("SetContextNamespace() error = %v", err)
	}

	// Verify the change
	info, err := client.GetContextInfo("dev")
	if err != nil {
		t.Fatalf("GetContextInfo() error = %v", err)
	}
	if info.Namespace != "new-namespace" {
		t.Errorf("Namespace = %s; want new-namespace", info.Namespace)
	}
}

func TestCreateContext(t *testing.T) {
	kubeconfigPath, cleanup := createTestKubeconfig(t)
	defer cleanup()

	client, err := NewClient(kubeconfigPath)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	// Create a new context
	if err := client.CreateContext("test-context", "cluster1", "user1", "test-ns"); err != nil {
		t.Fatalf("CreateContext() error = %v", err)
	}

	// Verify the new context
	info, err := client.GetContextInfo("test-context")
	if err != nil {
		t.Fatalf("GetContextInfo() error = %v", err)
	}
	if info.Cluster != "cluster1" {
		t.Errorf("Cluster = %s; want cluster1", info.Cluster)
	}
	if info.User != "user1" {
		t.Errorf("User = %s; want user1", info.User)
	}
	if info.Namespace != "test-ns" {
		t.Errorf("Namespace = %s; want test-ns", info.Namespace)
	}
}

func TestCreateContextAlreadyExists(t *testing.T) {
	kubeconfigPath, cleanup := createTestKubeconfig(t)
	defer cleanup()

	client, err := NewClient(kubeconfigPath)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	// Try to create a context that already exists
	err = client.CreateContext("dev", "cluster1", "user1", "test-ns")
	if err == nil {
		t.Error("CreateContext() should return error when context exists")
	}
}

func TestCreateContextInvalidCluster(t *testing.T) {
	kubeconfigPath, cleanup := createTestKubeconfig(t)
	defer cleanup()

	client, err := NewClient(kubeconfigPath)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	// Try to create a context with non-existent cluster
	err = client.CreateContext("test", "nonexistent-cluster", "user1", "test-ns")
	if err == nil {
		t.Error("CreateContext() should return error when cluster doesn't exist")
	}
}

func TestGetCurrentContextDetails(t *testing.T) {
	kubeconfigPath, cleanup := createTestKubeconfig(t)
	defer cleanup()

	client, err := NewClient(kubeconfigPath)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	details, err := client.GetCurrentContextDetails()
	if err != nil {
		t.Fatalf("GetCurrentContextDetails() error = %v", err)
	}

	if details.Name != "dev" {
		t.Errorf("Name = %s; want dev", details.Name)
	}
	if details.Cluster != "cluster1" {
		t.Errorf("Cluster = %s; want cluster1", details.Cluster)
	}
	if !details.IsCurrent {
		t.Error("IsCurrent should be true")
	}
}

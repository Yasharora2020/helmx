package tui

import (
	"testing"

	"github.com/yasharora2020/helmx/internal/helm"
)

func TestNewRBACView(t *testing.T) {
	view := NewRBACView(nil)

	if view == nil {
		t.Fatal("NewRBACView() returned nil")
	}

	// Verify initial state
	if view.selectedIdx != 0 {
		t.Errorf("Initial selectedIdx should be 0, got %d", view.selectedIdx)
	}

	if view.showAddDialog {
		t.Error("showAddDialog should be false initially")
	}

	if view.showEditDialog {
		t.Error("showEditDialog should be false initially")
	}

	if view.showDeleteConfirm {
		t.Error("showDeleteConfirm should be false initially")
	}

	if view.showKubeconfig {
		t.Error("showKubeconfig should be false initially")
	}
}

func TestRBACViewSetSize(t *testing.T) {
	view := NewRBACView(nil)
	view.SetSize(100, 50)

	if view.width != 100 {
		t.Errorf("SetSize() width = %d, want 100", view.width)
	}

	if view.height != 50 {
		t.Errorf("SetSize() height = %d, want 50", view.height)
	}
}

func TestRBACViewOpenAddDialog(t *testing.T) {
	view := NewRBACView(nil)
	view.openAddDialog()

	if !view.showAddDialog {
		t.Error("openAddDialog() should set showAddDialog to true")
	}

	if view.addField != 0 {
		t.Errorf("addField should be 0 after opening dialog, got %d", view.addField)
	}

	if view.addUserType != 0 {
		t.Errorf("addUserType should be 0 (ServiceAccount) by default, got %d", view.addUserType)
	}

	if view.addPermission != 0 {
		t.Errorf("addPermission should be 0 (read-only) by default, got %d", view.addPermission)
	}
}

func TestRBACViewOpenEditDialog(t *testing.T) {
	view := NewRBACView(nil)

	user := &helm.ManagedUser{
		Name: "test-user",
		Kind: "ServiceAccount",
		Permissions: []helm.NamespaceAccess{
			{Namespace: "dev", Permission: "developer"},
		},
	}

	view.openEditDialog(user)

	if !view.showEditDialog {
		t.Error("openEditDialog() should set showEditDialog to true")
	}

	if view.editUser != user {
		t.Error("openEditDialog() should set editUser")
	}

	if view.editSelectedPerm != 0 {
		t.Errorf("editSelectedPerm should be 0 initially, got %d", view.editSelectedPerm)
	}
}

func TestRBACViewOpenAddNSDialog(t *testing.T) {
	view := NewRBACView(nil)
	view.users = []helm.ManagedUser{
		{Name: "test-user", Kind: "User"},
	}

	view.openAddNSDialog(0)

	if !view.showAddNSDialog {
		t.Error("openAddNSDialog() should set showAddNSDialog to true")
	}

	if view.addNSUserIdx != 0 {
		t.Errorf("addNSUserIdx should be 0, got %d", view.addNSUserIdx)
	}

	if view.addNSField != 0 {
		t.Errorf("addNSField should be 0 initially, got %d", view.addNSField)
	}
}

func TestRBACViewOpenKubeconfigDialog(t *testing.T) {
	view := NewRBACView(nil)

	user := &helm.ManagedUser{
		Name:      "test-sa",
		Kind:      "ServiceAccount",
		Namespace: "default",
	}

	view.openKubeconfigDialog(user)

	if !view.showKubeconfig {
		t.Error("openKubeconfigDialog() should set showKubeconfig to true")
	}

	if view.kubeconfigUser != user {
		t.Error("openKubeconfigDialog() should set kubeconfigUser")
	}

	if view.kubeconfigContent != "" {
		t.Error("kubeconfigContent should be empty initially")
	}

	if view.kubeconfigSaved {
		t.Error("kubeconfigSaved should be false initially")
	}

	if view.kubeconfigPath != "" {
		t.Error("kubeconfigPath should be empty initially")
	}
}

func TestRBACViewKubeconfigInstructions(t *testing.T) {
	view := NewRBACView(nil)
	view.SetSize(100, 50)

	user := &helm.ManagedUser{
		Name:      "test-sa",
		Kind:      "ServiceAccount",
		Namespace: "default",
	}

	view.openKubeconfigDialog(user)
	view.kubeconfigContent = "apiVersion: v1\nkind: Config"
	view.kubeconfigSaved = true
	view.kubeconfigPath = "/home/user/kubeconfig-test-sa-default.yaml"

	output := view.renderKubeconfigDialog()

	// Should show success message
	if !containsSubstring(output, "saved successfully") {
		t.Error("renderKubeconfigDialog() should show success message after save")
	}

	// Should show usage instructions
	if !containsSubstring(output, "Usage Instructions") {
		t.Error("renderKubeconfigDialog() should show usage instructions after save")
	}

	// Should show export KUBECONFIG command
	if !containsSubstring(output, "export KUBECONFIG=") {
		t.Error("renderKubeconfigDialog() should show export KUBECONFIG command")
	}

	// Should show --kubeconfig flag example
	if !containsSubstring(output, "--kubeconfig=") {
		t.Error("renderKubeconfigDialog() should show --kubeconfig flag example")
	}

	// Should show the saved path
	if !containsSubstring(output, view.kubeconfigPath) {
		t.Error("renderKubeconfigDialog() should show the saved file path")
	}

	// Should show merge instructions
	if !containsSubstring(output, "Merge with existing") {
		t.Error("renderKubeconfigDialog() should show merge instructions")
	}
}

func TestRBACViewGetAddFieldCount(t *testing.T) {
	view := NewRBACView(nil)

	// ServiceAccount type (0) should have 7 fields
	view.addUserType = 0
	if got := view.getAddFieldCount(); got != 7 {
		t.Errorf("getAddFieldCount() for ServiceAccount = %d, want 7", got)
	}

	// User type (1) should have 6 fields
	view.addUserType = 1
	if got := view.getAddFieldCount(); got != 6 {
		t.Errorf("getAddFieldCount() for User = %d, want 6", got)
	}
}

func TestRBACViewGetPermissionFieldIndex(t *testing.T) {
	view := NewRBACView(nil)

	// ServiceAccount type (0) should have permission at index 4
	view.addUserType = 0
	if got := view.getPermissionFieldIndex(); got != 4 {
		t.Errorf("getPermissionFieldIndex() for ServiceAccount = %d, want 4", got)
	}

	// User type (1) should have permission at index 3
	view.addUserType = 1
	if got := view.getPermissionFieldIndex(); got != 3 {
		t.Errorf("getPermissionFieldIndex() for User = %d, want 3", got)
	}
}

func TestRBACViewView(t *testing.T) {
	view := NewRBACView(nil)
	view.SetSize(80, 24)

	// Main view
	output := view.View()
	if output == "" {
		t.Error("View() should return non-empty string")
	}

	// Should contain RBAC title
	if !containsSubstring(output, "RBAC") {
		t.Error("View() should contain 'RBAC'")
	}
}

func TestRBACViewRenderAddDialog(t *testing.T) {
	view := NewRBACView(nil)
	view.SetSize(80, 40)
	view.showAddDialog = true

	output := view.renderAddDialog()
	if output == "" {
		t.Error("renderAddDialog() should return non-empty string")
	}

	// Should contain form elements
	if !containsSubstring(output, "User type") {
		t.Error("renderAddDialog() should contain 'User type'")
	}
	if !containsSubstring(output, "Name") {
		t.Error("renderAddDialog() should contain 'Name'")
	}
	if !containsSubstring(output, "Permission") {
		t.Error("renderAddDialog() should contain 'Permission'")
	}
}

func TestRBACViewRenderEditDialog(t *testing.T) {
	view := NewRBACView(nil)
	view.SetSize(80, 40)
	view.showEditDialog = true
	view.editUser = &helm.ManagedUser{
		Name: "test-user",
		Kind: "ServiceAccount",
		Permissions: []helm.NamespaceAccess{
			{Namespace: "dev", Permission: "developer"},
			{Namespace: "prod", Permission: "read-only"},
		},
	}

	output := view.renderEditDialog()
	if output == "" {
		t.Error("renderEditDialog() should return non-empty string")
	}

	// Should contain user name
	if !containsSubstring(output, "test-user") {
		t.Error("renderEditDialog() should contain user name")
	}
	// Should contain permissions
	if !containsSubstring(output, "dev") {
		t.Error("renderEditDialog() should contain 'dev' namespace")
	}
}

func TestRBACViewRenderDeleteConfirm(t *testing.T) {
	view := NewRBACView(nil)
	view.SetSize(80, 40)
	view.showDeleteConfirm = true
	view.deleteUser = &helm.ManagedUser{
		Name: "doomed-user",
		Kind: "User",
	}

	output := view.renderDeleteConfirm()
	if output == "" {
		t.Error("renderDeleteConfirm() should return non-empty string")
	}

	if !containsSubstring(output, "doomed-user") {
		t.Error("renderDeleteConfirm() should contain user name")
	}
	if !containsSubstring(output, "Delete") {
		t.Error("renderDeleteConfirm() should contain 'Delete'")
	}
}

func TestRBACViewLoadingState(t *testing.T) {
	view := NewRBACView(nil)
	view.SetSize(80, 24)
	view.loading = true

	output := view.renderMainView()

	if !containsSubstring(output, "Loading") {
		t.Error("renderMainView() should show loading state")
	}
}

func TestRBACViewEmptyUserList(t *testing.T) {
	view := NewRBACView(nil)
	view.SetSize(80, 24)
	view.loading = false
	view.users = []helm.ManagedUser{}

	output := view.renderMainView()

	if !containsSubstring(output, "No users") {
		t.Error("renderMainView() should show empty state message")
	}
}

func TestRBACViewUserListNavigation(t *testing.T) {
	view := NewRBACView(nil)
	view.users = []helm.ManagedUser{
		{Name: "user1", Kind: "User"},
		{Name: "user2", Kind: "ServiceAccount"},
		{Name: "user3", Kind: "User"},
	}
	view.selectedIdx = 0

	// Navigate down
	view.selectedIdx++
	if view.selectedIdx != 1 {
		t.Errorf("Navigation down: selectedIdx = %d, want 1", view.selectedIdx)
	}

	// Navigate up
	view.selectedIdx--
	if view.selectedIdx != 0 {
		t.Errorf("Navigation up: selectedIdx = %d, want 0", view.selectedIdx)
	}

	// Bounds checking - can't go below 0
	if view.selectedIdx > 0 {
		view.selectedIdx--
	}
	if view.selectedIdx != 0 {
		t.Errorf("Should not go below 0: selectedIdx = %d", view.selectedIdx)
	}

	// Navigate to end
	view.selectedIdx = len(view.users) - 1
	if view.selectedIdx != 2 {
		t.Errorf("Navigate to end: selectedIdx = %d, want 2", view.selectedIdx)
	}

	// Can't go beyond end
	if view.selectedIdx < len(view.users)-1 {
		view.selectedIdx++
	}
	if view.selectedIdx != 2 {
		t.Errorf("Should not go beyond end: selectedIdx = %d", view.selectedIdx)
	}
}

// Helper function to check if a string contains a substring
func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// --- Filter Tests ---

func TestRBACViewApplyFilters_TypeFilter(t *testing.T) {
	view := NewRBACView(nil)
	view.users = []helm.ManagedUser{
		{Name: "user1", Kind: "User"},
		{Name: "sa1", Kind: "ServiceAccount", Namespace: "default"},
		{Name: "user2", Kind: "User"},
		{Name: "sa2", Kind: "ServiceAccount", Namespace: "kube-system"},
	}

	// Filter All (index 0) - should include all users
	view.filterTypeIdx = 0
	view.applyFilters()
	if len(view.filteredUsers) != 4 {
		t.Errorf("Type filter All: got %d users, want 4", len(view.filteredUsers))
	}

	// Filter ServiceAccount (index 1)
	view.filterTypeIdx = 1
	view.applyFilters()
	if len(view.filteredUsers) != 2 {
		t.Errorf("Type filter ServiceAccount: got %d users, want 2", len(view.filteredUsers))
	}
	for _, u := range view.filteredUsers {
		if u.Kind != "ServiceAccount" {
			t.Errorf("Type filter ServiceAccount: got user of kind %s", u.Kind)
		}
	}

	// Filter User (index 2)
	view.filterTypeIdx = 2
	view.applyFilters()
	if len(view.filteredUsers) != 2 {
		t.Errorf("Type filter User: got %d users, want 2", len(view.filteredUsers))
	}
	for _, u := range view.filteredUsers {
		if u.Kind != "User" {
			t.Errorf("Type filter User: got user of kind %s", u.Kind)
		}
	}
}

func TestRBACViewApplyFilters_NamespaceFilter(t *testing.T) {
	view := NewRBACView(nil)
	view.users = []helm.ManagedUser{
		{Name: "sa1", Kind: "ServiceAccount", Namespace: "default"},
		{Name: "sa2", Kind: "ServiceAccount", Namespace: "kube-system"},
		{Name: "sa3", Kind: "ServiceAccount", Namespace: "default"},
		{Name: "user1", Kind: "User"}, // Users should not be affected by namespace filter
	}

	// Filter by namespace "default"
	view.filterNamespace = "default"
	view.applyFilters()
	if len(view.filteredUsers) != 3 { // 2 SAs in default + 1 User (not affected)
		t.Errorf("Namespace filter 'default': got %d users, want 3", len(view.filteredUsers))
	}

	// Filter by namespace "kube-system"
	view.filterNamespace = "kube-system"
	view.applyFilters()
	if len(view.filteredUsers) != 2 { // 1 SA in kube-system + 1 User
		t.Errorf("Namespace filter 'kube-system': got %d users, want 2", len(view.filteredUsers))
	}
}

func TestRBACViewApplyFilters_QueryFilter(t *testing.T) {
	view := NewRBACView(nil)
	view.users = []helm.ManagedUser{
		{Name: "deploy-bot", Kind: "ServiceAccount"},
		{Name: "deploy-pipeline", Kind: "ServiceAccount"},
		{Name: "admin-user", Kind: "User"},
		{Name: "developer", Kind: "User"},
	}

	// Search for "deploy"
	view.filterQuery = "deploy"
	view.applyFilters()
	if len(view.filteredUsers) != 2 {
		t.Errorf("Query filter 'deploy': got %d users, want 2", len(view.filteredUsers))
	}

	// Search for "DEPLOY" (case insensitive)
	view.filterQuery = "DEPLOY"
	view.applyFilters()
	if len(view.filteredUsers) != 2 {
		t.Errorf("Query filter 'DEPLOY' (case insensitive): got %d users, want 2", len(view.filteredUsers))
	}

	// Search for "admin"
	view.filterQuery = "admin"
	view.applyFilters()
	if len(view.filteredUsers) != 1 {
		t.Errorf("Query filter 'admin': got %d users, want 1", len(view.filteredUsers))
	}
}

func TestRBACViewApplyFilters_Combined(t *testing.T) {
	view := NewRBACView(nil)
	view.users = []helm.ManagedUser{
		{Name: "deploy-bot", Kind: "ServiceAccount", Namespace: "production"},
		{Name: "deploy-pipeline", Kind: "ServiceAccount", Namespace: "staging"},
		{Name: "deploy-admin", Kind: "User"},
		{Name: "monitor-bot", Kind: "ServiceAccount", Namespace: "production"},
	}

	// Combine type + namespace + query
	view.filterTypeIdx = 1 // ServiceAccount
	view.filterNamespace = "production"
	view.filterQuery = "deploy"
	view.applyFilters()
	if len(view.filteredUsers) != 1 {
		t.Errorf("Combined filters: got %d users, want 1", len(view.filteredUsers))
	}
	if len(view.filteredUsers) > 0 && view.filteredUsers[0].Name != "deploy-bot" {
		t.Errorf("Combined filters: got %s, want 'deploy-bot'", view.filteredUsers[0].Name)
	}
}

func TestRBACViewApplyFilters_ResetSelection(t *testing.T) {
	view := NewRBACView(nil)
	view.users = []helm.ManagedUser{
		{Name: "user1", Kind: "User"},
		{Name: "user2", Kind: "User"},
		{Name: "user3", Kind: "User"},
	}
	view.selectedIdx = 2

	// Apply filter that results in fewer users
	view.filterQuery = "user1"
	view.applyFilters()

	if view.selectedIdx != 0 {
		t.Errorf("Selection should reset to 0 when out of bounds, got %d", view.selectedIdx)
	}
}

func TestRBACViewHasActiveFilters(t *testing.T) {
	view := NewRBACView(nil)

	// No filters active
	if view.hasActiveFilters() {
		t.Error("hasActiveFilters() should return false when no filters active")
	}

	// Type filter active
	view.filterTypeIdx = 1
	if !view.hasActiveFilters() {
		t.Error("hasActiveFilters() should return true with type filter active")
	}
	view.filterTypeIdx = 0

	// Query filter active
	view.filterQuery = "test"
	if !view.hasActiveFilters() {
		t.Error("hasActiveFilters() should return true with query filter active")
	}
	view.filterQuery = ""

	// Namespace filter active
	view.filterNamespace = "default"
	if !view.hasActiveFilters() {
		t.Error("hasActiveFilters() should return true with namespace filter active")
	}
}

func TestRBACViewClearFilters(t *testing.T) {
	view := NewRBACView(nil)
	view.users = []helm.ManagedUser{
		{Name: "user1", Kind: "User"},
		{Name: "user2", Kind: "ServiceAccount"},
	}

	// Set all filters
	view.filterMode = true
	view.filterInput.SetValue("test")
	view.filterQuery = "test"
	view.filterTypeIdx = 1
	view.filterNamespace = "default"

	view.clearFilters()

	if view.filterMode {
		t.Error("clearFilters() should set filterMode to false")
	}
	if view.filterQuery != "" {
		t.Error("clearFilters() should clear filterQuery")
	}
	if view.filterTypeIdx != 0 {
		t.Error("clearFilters() should reset filterTypeIdx to 0")
	}
	if view.filterNamespace != "" {
		t.Error("clearFilters() should clear filterNamespace")
	}
	if len(view.filteredUsers) != 2 {
		t.Errorf("clearFilters() should show all users, got %d", len(view.filteredUsers))
	}
}

func TestRBACViewExtractNamespaces(t *testing.T) {
	view := NewRBACView(nil)
	view.users = []helm.ManagedUser{
		{Name: "sa1", Kind: "ServiceAccount", Namespace: "production"},
		{Name: "sa2", Kind: "ServiceAccount", Namespace: "staging"},
		{Name: "sa3", Kind: "ServiceAccount", Namespace: "production"},
		{Name: "user1", Kind: "User"}, // Users have no namespace
	}

	view.extractNamespaces()

	if len(view.namespaces) != 2 {
		t.Errorf("extractNamespaces(): got %d namespaces, want 2", len(view.namespaces))
	}

	// Check namespaces are sorted
	expected := []string{"production", "staging"}
	for i, ns := range view.namespaces {
		if ns != expected[i] {
			t.Errorf("extractNamespaces(): namespaces[%d] = %s, want %s", i, ns, expected[i])
		}
	}
}

func TestRBACViewRenderMainView_FilterIndicators(t *testing.T) {
	view := NewRBACView(nil)
	view.SetSize(100, 50)
	view.users = []helm.ManagedUser{
		{Name: "user1", Kind: "User"},
		{Name: "user2", Kind: "ServiceAccount", Namespace: "default"},
	}
	view.applyFilters()

	// With filter active
	view.filterTypeIdx = 1 // ServiceAccount
	view.filterQuery = "user"
	view.applyFilters()

	output := view.renderMainView()

	// Should show filtered count
	if !containsSubstring(output, "/") {
		t.Error("renderMainView() should show filtered count (x/y) when filters active")
	}

	// Should show type filter pills
	if !containsSubstring(output, "ServiceAccount") {
		t.Error("renderMainView() should show type filter options")
	}
}

func TestRBACViewRenderMainView_EmptyFilteredResults(t *testing.T) {
	view := NewRBACView(nil)
	view.SetSize(100, 50)
	view.users = []helm.ManagedUser{
		{Name: "user1", Kind: "User"},
	}

	// Apply filter that matches nothing
	view.filterQuery = "nonexistent"
	view.applyFilters()

	output := view.renderMainView()

	if !containsSubstring(output, "No users match") {
		t.Error("renderMainView() should show 'No users match' when filters return empty")
	}
}

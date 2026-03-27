package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/yasharora2020/helmx/internal/helm"
	"github.com/yasharora2020/helmx/internal/tui/validation"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// RBACView handles RBAC management
type RBACView struct {
	rbacManager helm.RBACManager
	users       []helm.ManagedUser
	width       int
	height      int

	// User list navigation
	selectedIdx int

	// Loading state
	loading    bool
	loadingErr error

	// Filter state
	filterMode      bool            // True when search input is active
	filterInput     textinput.Model // Text input for search
	filterTypeIdx   int             // 0=All, 1=ServiceAccount, 2=User
	filterQuery     string          // Applied search query
	filterNamespace string          // Namespace filter (empty = all)
	filteredUsers   []helm.ManagedUser
	namespaces      []string // Unique namespaces from users

	// Add user dialog
	showAddDialog     bool
	addUserType       int // 0=ServiceAccount, 1=User
	addNameInput      textinput.Model
	addNamespaceInput textinput.Model
	addTargetNSInput  textinput.Model
	addPermission     int // 0=read-only, 1=developer, 2=namespace-admin
	addField          int // 0=name, 1=namespace(SA only), 2=targetNS, 3=permission, 4=create, 5=cancel
	addError          error

	// Edit user dialog
	showEditDialog   bool
	editUser         *helm.ManagedUser
	editSelectedPerm int

	// Add namespace to user dialog
	showAddNSDialog     bool
	addNSUserIdx        int
	addNSNamespaceInput textinput.Model
	addNSPermission     int
	addNSField          int // 0=namespace, 1=permission, 2=add, 3=cancel
	addNSError          error

	// Delete confirmation
	showDeleteConfirm bool
	deleteUser        *helm.ManagedUser

	// Kubeconfig export
	showKubeconfig    bool
	kubeconfigUser    *helm.ManagedUser
	kubeconfigContent string
	kubeconfigErr     error
	kubeconfigSaved   bool   // True after file is saved
	kubeconfigPath    string // Full path to saved file

	// Status
	status string
	err    error
}

// NewRBACView creates a new RBAC view
func NewRBACView(rbac helm.RBACManager) *RBACView {
	// Add name input
	addName := textinput.New()
	addName.Placeholder = "deploy-bot"
	addName.CharLimit = 63
	addName.Width = 40

	// Add namespace input (for SA)
	addNamespace := textinput.New()
	addNamespace.Placeholder = "default"
	addNamespace.CharLimit = 63
	addNamespace.Width = 30

	// Add target namespace input
	addTargetNS := textinput.New()
	addTargetNS.Placeholder = "production"
	addTargetNS.CharLimit = 63
	addTargetNS.Width = 30

	// Add namespace to user input
	addNSNamespace := textinput.New()
	addNSNamespace.Placeholder = "staging"
	addNSNamespace.CharLimit = 63
	addNSNamespace.Width = 30

	// Filter input
	filterInput := textinput.New()
	filterInput.Placeholder = "search by name..."
	filterInput.CharLimit = 50
	filterInput.Width = 30

	return &RBACView{
		rbacManager:         rbac,
		addNameInput:        addName,
		addNamespaceInput:   addNamespace,
		addTargetNSInput:    addTargetNS,
		addNSNamespaceInput: addNSNamespace,
		filterInput:         filterInput,
	}
}

// SetSize updates dimensions
func (r *RBACView) SetSize(width, height int) {
	r.width = width
	r.height = height
}

// HasOpenDialog returns true if any dialog is currently open.
func (r *RBACView) HasOpenDialog() bool {
	return r.showAddDialog ||
		r.showEditDialog ||
		r.showDeleteConfirm ||
		r.showAddNSDialog ||
		r.showKubeconfig
}

// applyFilters filters the user list based on current filter state
func (r *RBACView) applyFilters() {
	r.filteredUsers = nil

	// Filter types
	typeFilters := []string{"All", "ServiceAccount", "User"}

	for _, user := range r.users {
		// Type filter
		if r.filterTypeIdx != 0 {
			if user.Kind != typeFilters[r.filterTypeIdx] {
				continue
			}
		}

		// Namespace filter (only applies to ServiceAccounts)
		if r.filterNamespace != "" && user.Kind == "ServiceAccount" {
			if user.Namespace != r.filterNamespace {
				continue
			}
		}

		// Name search (case-insensitive substring match)
		if r.filterQuery != "" {
			if !strings.Contains(strings.ToLower(user.Name), strings.ToLower(r.filterQuery)) {
				continue
			}
		}

		r.filteredUsers = append(r.filteredUsers, user)
	}

	// Reset selection if out of bounds
	if r.selectedIdx >= len(r.filteredUsers) {
		r.selectedIdx = len(r.filteredUsers) - 1
	}
	if r.selectedIdx < 0 {
		r.selectedIdx = 0
	}
}

// extractNamespaces extracts unique namespaces from ServiceAccounts
func (r *RBACView) extractNamespaces() {
	nsMap := make(map[string]bool)
	for _, user := range r.users {
		if user.Kind == "ServiceAccount" && user.Namespace != "" {
			nsMap[user.Namespace] = true
		}
	}

	r.namespaces = nil
	for ns := range nsMap {
		r.namespaces = append(r.namespaces, ns)
	}
	// Sort for consistent ordering
	sort.Strings(r.namespaces)
}

// clearFilters resets all filter state
func (r *RBACView) clearFilters() {
	r.filterMode = false
	r.filterInput.Blur()
	r.filterInput.SetValue("")
	r.filterQuery = ""
	r.filterTypeIdx = 0
	r.filterNamespace = ""
	r.applyFilters()
}

// hasActiveFilters returns true if any filter is active
func (r *RBACView) hasActiveFilters() bool {
	return r.filterTypeIdx != 0 || r.filterQuery != "" || r.filterNamespace != ""
}

// Init loads the initial data
func (r *RBACView) Init() tea.Cmd {
	r.loading = true
	return r.loadUsers()
}

// Update handles input
func (r *RBACView) Update(msg tea.Msg) (*RBACView, tea.Cmd) {
	// Handle loaded users message
	if loadedMsg, ok := msg.(rbacUsersLoadedMsg); ok {
		r.loading = false
		if loadedMsg.err != nil {
			r.loadingErr = loadedMsg.err
			return r, nil
		}
		r.users = loadedMsg.users
		r.loadingErr = nil
		r.extractNamespaces()
		r.applyFilters()
		return r, nil
	}

	// Handle grant access result
	if resultMsg, ok := msg.(rbacGrantResultMsg); ok {
		if resultMsg.err != nil {
			if r.showAddDialog {
				r.addError = resultMsg.err
			} else if r.showAddNSDialog {
				r.addNSError = resultMsg.err
			} else {
				r.err = resultMsg.err
			}
		} else {
			r.showAddDialog = false
			r.showAddNSDialog = false
			r.status = "Access granted successfully"
		}
		return r, r.loadUsers()
	}

	// Handle delete result
	if resultMsg, ok := msg.(rbacDeleteResultMsg); ok {
		r.showDeleteConfirm = false
		if resultMsg.err != nil {
			r.err = resultMsg.err
		} else {
			r.status = "User deleted successfully"
		}
		return r, r.loadUsers()
	}

	// Handle revoke result
	if resultMsg, ok := msg.(rbacRevokeResultMsg); ok {
		r.showEditDialog = false
		if resultMsg.err != nil {
			r.err = resultMsg.err
		} else {
			r.status = "Access revoked successfully"
		}
		return r, r.loadUsers()
	}

	// Handle kubeconfig result
	if resultMsg, ok := msg.(rbacKubeconfigMsg); ok {
		if resultMsg.err != nil {
			r.kubeconfigErr = resultMsg.err
		} else {
			r.kubeconfigContent = resultMsg.content
		}
		return r, nil
	}

	// Route to dialog handlers
	if r.showAddDialog {
		return r.updateAddDialog(msg)
	}
	if r.showEditDialog {
		return r.updateEditDialog(msg)
	}
	if r.showAddNSDialog {
		return r.updateAddNSDialog(msg)
	}
	if r.showDeleteConfirm {
		return r.updateDeleteConfirm(msg)
	}
	if r.showKubeconfig {
		return r.updateKubeconfigDialog(msg)
	}

	// Handle filter mode input
	if r.filterMode {
		return r.updateFilterMode(msg)
	}

	// Main view input handling
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		r.SetSize(msg.Width, msg.Height)

	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if r.selectedIdx > 0 {
				r.selectedIdx--
			}
		case "down", "j":
			if r.selectedIdx < len(r.filteredUsers)-1 {
				r.selectedIdx++
			}
		case "/":
			// Enter search mode
			r.filterMode = true
			r.filterInput.Focus()
			return r, textinput.Blink
		case "f":
			// Cycle type filter: All → ServiceAccount → User → All
			r.filterTypeIdx = (r.filterTypeIdx + 1) % 3
			r.applyFilters()
		case "n":
			// Cycle namespace filter
			if len(r.namespaces) > 0 {
				if r.filterNamespace == "" {
					r.filterNamespace = r.namespaces[0]
				} else {
					// Find current index and move to next
					found := false
					for i, ns := range r.namespaces {
						if ns == r.filterNamespace {
							if i+1 < len(r.namespaces) {
								r.filterNamespace = r.namespaces[i+1]
							} else {
								r.filterNamespace = "" // Clear filter
							}
							found = true
							break
						}
					}
					if !found {
						r.filterNamespace = r.namespaces[0]
					}
				}
				r.applyFilters()
			}
		case "esc":
			// Clear filters if any are active
			if r.hasActiveFilters() {
				r.clearFilters()
			}
		case "a":
			r.openAddDialog()
			return r, textinput.Blink
		case "e", "enter":
			if len(r.filteredUsers) > 0 && r.selectedIdx < len(r.filteredUsers) {
				r.openEditDialog(&r.filteredUsers[r.selectedIdx])
			}
		case "+":
			if len(r.filteredUsers) > 0 && r.selectedIdx < len(r.filteredUsers) {
				// Find the index in the original users slice
				selectedUser := r.filteredUsers[r.selectedIdx]
				for i, user := range r.users {
					if user.Name == selectedUser.Name && user.Kind == selectedUser.Kind && user.Namespace == selectedUser.Namespace {
						r.openAddNSDialog(i)
						return r, textinput.Blink
					}
				}
			}
		case "d":
			if len(r.filteredUsers) > 0 && r.selectedIdx < len(r.filteredUsers) {
				r.showDeleteConfirm = true
				r.deleteUser = &r.filteredUsers[r.selectedIdx]
			}
		case "K":
			if len(r.filteredUsers) > 0 && r.selectedIdx < len(r.filteredUsers) {
				user := r.filteredUsers[r.selectedIdx]
				if user.Kind == "ServiceAccount" {
					r.openKubeconfigDialog(&user)
					return r, r.generateKubeconfig(&user)
				} else {
					r.status = "Kubeconfig export only available for ServiceAccounts"
				}
			}
		case "r":
			r.loading = true
			return r, r.loadUsers()
		}
	}

	return r, nil
}

// updateFilterMode handles input while in filter/search mode
func (r *RBACView) updateFilterMode(msg tea.Msg) (*RBACView, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			// Exit filter mode, keep current query if any
			r.filterMode = false
			r.filterInput.Blur()
			return r, nil
		case "enter":
			// Apply search query and exit filter mode
			r.filterQuery = strings.TrimSpace(r.filterInput.Value())
			r.filterMode = false
			r.filterInput.Blur()
			r.applyFilters()
			return r, nil
		}
	}

	// Update text input
	var cmd tea.Cmd
	r.filterInput, cmd = r.filterInput.Update(msg)
	return r, cmd
}

// openAddDialog opens the add user dialog
func (r *RBACView) openAddDialog() {
	r.showAddDialog = true
	r.addField = 0
	r.addUserType = 0
	r.addPermission = 0
	r.addNameInput.SetValue("")
	r.addNamespaceInput.SetValue("")
	r.addTargetNSInput.SetValue("")
	r.addNameInput.Focus()
	r.addError = nil
	r.status = ""
	r.err = nil
}

func (r *RBACView) updateAddDialog(msg tea.Msg) (*RBACView, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			r.showAddDialog = false
			return r, nil
		case "tab", "down":
			r.addField = (r.addField + 1) % r.getAddFieldCount()
			r.updateAddFocus()
		case "shift+tab", "up":
			r.addField = (r.addField + r.getAddFieldCount() - 1) % r.getAddFieldCount()
			r.updateAddFocus()
		case "left", "right":
			// Toggle user type or permission
			if r.addField == 0 {
				r.addUserType = 1 - r.addUserType
			} else if r.addField == r.getPermissionFieldIndex() {
				if msg.String() == "left" && r.addPermission > 0 {
					r.addPermission--
				} else if msg.String() == "right" && r.addPermission < 2 {
					r.addPermission++
				}
			}
		case "enter":
			if r.addField == r.getAddFieldCount()-2 { // Create button
				return r.submitAddUser()
			} else if r.addField == r.getAddFieldCount()-1 { // Cancel button
				r.showAddDialog = false
			}
			return r, nil
		}
	}

	// Update focused input
	var cmd tea.Cmd
	if r.addUserType == 0 { // ServiceAccount
		switch r.addField {
		case 1:
			r.addNameInput, cmd = r.addNameInput.Update(msg)
		case 2:
			r.addNamespaceInput, cmd = r.addNamespaceInput.Update(msg)
		case 3:
			r.addTargetNSInput, cmd = r.addTargetNSInput.Update(msg)
		}
	} else { // User
		switch r.addField {
		case 1:
			r.addNameInput, cmd = r.addNameInput.Update(msg)
		case 2:
			r.addTargetNSInput, cmd = r.addTargetNSInput.Update(msg)
		}
	}
	return r, cmd
}

func (r *RBACView) getAddFieldCount() int {
	if r.addUserType == 0 { // ServiceAccount has extra namespace field
		return 7 // type, name, saNamespace, targetNS, permission, create, cancel
	}
	return 6 // type, name, targetNS, permission, create, cancel
}

func (r *RBACView) getPermissionFieldIndex() int {
	if r.addUserType == 0 {
		return 4
	}
	return 3
}

func (r *RBACView) updateAddFocus() {
	r.addNameInput.Blur()
	r.addNamespaceInput.Blur()
	r.addTargetNSInput.Blur()

	if r.addUserType == 0 { // ServiceAccount
		switch r.addField {
		case 1:
			r.addNameInput.Focus()
		case 2:
			r.addNamespaceInput.Focus()
		case 3:
			r.addTargetNSInput.Focus()
		}
	} else { // User
		switch r.addField {
		case 1:
			r.addNameInput.Focus()
		case 2:
			r.addTargetNSInput.Focus()
		}
	}
}

func (r *RBACView) submitAddUser() (*RBACView, tea.Cmd) {
	name := strings.TrimSpace(r.addNameInput.Value())
	targetNS := strings.TrimSpace(r.addTargetNSInput.Value())

	if name == "" {
		r.addError = fmt.Errorf("name is required")
		return r, nil
	}
	if targetNS == "" {
		r.addError = fmt.Errorf("target namespace is required")
		return r, nil
	}

	// Validate user name (uses same rules as release names for K8s resources)
	if err := validation.ValidateReleaseName(name); err != nil {
		r.addError = fmt.Errorf("invalid user name: %v", err)
		return r, nil
	}

	// Validate target namespace (* = cluster-wide)
	if err := validation.ValidateTargetNamespace(targetNS); err != nil {
		r.addError = fmt.Errorf("invalid target namespace: %v", err)
		return r, nil
	}

	presets := helm.GetPermissionPresets()
	permission := presets[r.addPermission]

	user := helm.ManagedUser{
		Name: name,
	}

	if r.addUserType == 0 { // ServiceAccount
		user.Kind = "ServiceAccount"
		user.Namespace = strings.TrimSpace(r.addNamespaceInput.Value())
		if user.Namespace == "" {
			user.Namespace = targetNS
		} else {
			// Validate SA namespace if provided
			if err := validation.ValidateNamespace(user.Namespace); err != nil {
				r.addError = fmt.Errorf("invalid ServiceAccount namespace: %v", err)
				return r, nil
			}
		}
	} else {
		user.Kind = "User"
	}

	return r, r.grantAccess(user, targetNS, permission)
}

// openEditDialog opens the edit user dialog
func (r *RBACView) openEditDialog(user *helm.ManagedUser) {
	r.showEditDialog = true
	r.editUser = user
	r.editSelectedPerm = 0
	r.status = ""
	r.err = nil
}

func (r *RBACView) updateEditDialog(msg tea.Msg) (*RBACView, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "q":
			r.showEditDialog = false
			return r, nil
		case "up", "k":
			if r.editSelectedPerm > 0 {
				r.editSelectedPerm--
			}
		case "down", "j":
			if r.editSelectedPerm < len(r.editUser.Permissions)-1 {
				r.editSelectedPerm++
			}
		case "x", "d":
			// Revoke access for selected permission
			if len(r.editUser.Permissions) > 0 && r.editSelectedPerm < len(r.editUser.Permissions) {
				perm := r.editUser.Permissions[r.editSelectedPerm]
				return r, r.revokeAccess(*r.editUser, perm.Namespace)
			}
		}
	}
	return r, nil
}

// openAddNSDialog opens dialog to add namespace access to existing user
func (r *RBACView) openAddNSDialog(userIdx int) {
	r.showAddNSDialog = true
	r.addNSUserIdx = userIdx
	r.addNSField = 0
	r.addNSPermission = 0
	r.addNSNamespaceInput.SetValue("")
	r.addNSNamespaceInput.Focus()
	r.addNSError = nil
}

func (r *RBACView) updateAddNSDialog(msg tea.Msg) (*RBACView, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			r.showAddNSDialog = false
			return r, nil
		case "tab", "down":
			r.addNSField = (r.addNSField + 1) % 4
			r.updateAddNSFocus()
		case "shift+tab", "up":
			r.addNSField = (r.addNSField + 3) % 4
			r.updateAddNSFocus()
		case "left":
			if r.addNSField == 1 && r.addNSPermission > 0 {
				r.addNSPermission--
			}
		case "right":
			if r.addNSField == 1 && r.addNSPermission < 2 {
				r.addNSPermission++
			}
		case "enter":
			if r.addNSField == 2 { // Add button
				return r.submitAddNS()
			} else if r.addNSField == 3 { // Cancel button
				r.showAddNSDialog = false
			}
			return r, nil
		}
	}

	// Update focused input
	var cmd tea.Cmd
	if r.addNSField == 0 {
		r.addNSNamespaceInput, cmd = r.addNSNamespaceInput.Update(msg)
	}
	return r, cmd
}

func (r *RBACView) updateAddNSFocus() {
	r.addNSNamespaceInput.Blur()
	if r.addNSField == 0 {
		r.addNSNamespaceInput.Focus()
	}
}

func (r *RBACView) submitAddNS() (*RBACView, tea.Cmd) {
	namespace := strings.TrimSpace(r.addNSNamespaceInput.Value())
	if namespace == "" {
		r.addNSError = fmt.Errorf("namespace is required")
		return r, nil
	}

	// Validate namespace (* = cluster-wide)
	if err := validation.ValidateTargetNamespace(namespace); err != nil {
		r.addNSError = fmt.Errorf("invalid namespace: %v", err)
		return r, nil
	}

	presets := helm.GetPermissionPresets()
	permission := presets[r.addNSPermission]
	user := r.users[r.addNSUserIdx]

	return r, r.grantAccess(user, namespace, permission)
}

func (r *RBACView) updateDeleteConfirm(msg tea.Msg) (*RBACView, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "y", "Y", "enter":
			r.showDeleteConfirm = false
			if r.deleteUser != nil {
				return r, r.deleteUserCmd(*r.deleteUser)
			}
		case "n", "N", "esc":
			r.showDeleteConfirm = false
			r.deleteUser = nil
		}
	}
	return r, nil
}

// openKubeconfigDialog opens the kubeconfig export dialog
func (r *RBACView) openKubeconfigDialog(user *helm.ManagedUser) {
	r.showKubeconfig = true
	r.kubeconfigUser = user
	r.kubeconfigContent = ""
	r.kubeconfigErr = nil
	r.kubeconfigSaved = false
	r.kubeconfigPath = ""
}

func (r *RBACView) updateKubeconfigDialog(msg tea.Msg) (*RBACView, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "q":
			r.showKubeconfig = false
			r.kubeconfigUser = nil
			r.kubeconfigSaved = false
			r.kubeconfigPath = ""
			return r, nil
		case "s":
			// Save to ~/.kube/ directory (standard location for kubeconfigs)
			if r.kubeconfigContent != "" && r.kubeconfigUser != nil && !r.kubeconfigSaved {
				filename := fmt.Sprintf("kubeconfig-%s-%s.yaml", r.kubeconfigUser.Name, r.kubeconfigUser.Namespace)
				homeDir, _ := os.UserHomeDir()
				kubeDir := filepath.Join(homeDir, ".kube")

				// Create ~/.kube/ with secure permissions if it doesn't exist
				if err := os.MkdirAll(kubeDir, 0700); err != nil {
					r.kubeconfigErr = fmt.Errorf("failed to create ~/.kube: %w", err)
					return r, nil
				}

				savePath := filepath.Join(kubeDir, filename)
				if err := os.WriteFile(savePath, []byte(r.kubeconfigContent), 0600); err != nil {
					r.kubeconfigErr = fmt.Errorf("failed to save: %w", err)
				} else {
					r.kubeconfigSaved = true
					r.kubeconfigPath = savePath
					r.status = fmt.Sprintf("Saved to ~/.kube/%s", filename)
				}
			}
		}
	}
	return r, nil
}

// View renders the RBAC view
func (r *RBACView) View() string {
	if r.showAddDialog {
		return r.renderAddDialog()
	}
	if r.showEditDialog {
		return r.renderEditDialog()
	}
	if r.showAddNSDialog {
		return r.renderAddNSDialog()
	}
	if r.showDeleteConfirm {
		return r.renderDeleteConfirm()
	}
	if r.showKubeconfig {
		return r.renderKubeconfigDialog()
	}
	return r.renderMainView()
}

func (r *RBACView) renderMainView() string {
	var b strings.Builder

	// Header
	b.WriteString(S.Title.Render("👥 RBAC Management") + "\n\n")

	if r.loading {
		b.WriteString(S.Muted.Render("Loading users...") + "\n")
	} else if r.loadingErr != nil {
		b.WriteString(S.Error.Render(Icons.Cross+" Error: "+r.loadingErr.Error()) + "\n")
	} else if len(r.users) == 0 {
		b.WriteString(S.Muted.Render("No users with RBAC permissions found.") + "\n")
		b.WriteString(S.Muted.Render("Press 'a' to add a new user.") + "\n")
	} else {
		// User count with filter indicator
		countStr := fmt.Sprintf("Users (%d)", len(r.users))
		if r.hasActiveFilters() {
			countStr = fmt.Sprintf("Users (%d/%d)", len(r.filteredUsers), len(r.users))
		}
		b.WriteString(S.Highlighted.Render(countStr))

		// Active filter indicators
		typeFilters := []string{"All", "ServiceAccount", "User"}
		var filterParts []string

		// Type filter pills
		for i, t := range typeFilters {
			if i == r.filterTypeIdx {
				filterParts = append(filterParts, S.Selected.Render("["+t+"]"))
			} else {
				filterParts = append(filterParts, S.Muted.Render("["+t+"]"))
			}
		}
		b.WriteString("  " + strings.Join(filterParts, " ") + "\n")

		// Search and namespace filter line
		var filterLine []string
		if r.filterMode {
			filterLine = append(filterLine, Icons.Search+" "+r.filterInput.View())
		} else if r.filterQuery != "" {
			filterLine = append(filterLine, S.Value.Render(Icons.Search+" \""+r.filterQuery+"\""))
		}
		if r.filterNamespace != "" {
			filterLine = append(filterLine, S.Value.Render("ns:"+r.filterNamespace))
		}
		if len(filterLine) > 0 {
			b.WriteString(strings.Join(filterLine, "  ") + "\n")
		}
		b.WriteString("\n")

		// Check for empty filtered results
		if len(r.filteredUsers) == 0 && r.hasActiveFilters() {
			b.WriteString(S.Muted.Render("No users match the current filters.") + "\n")
			b.WriteString(S.Muted.Render("Press Esc to clear filters.") + "\n")
		} else {
			// Calculate visible range for scrolling
			visibleRows := r.height - 17 // Adjusted for filter row
			if visibleRows < 5 {
				visibleRows = 5
			}
			startIdx := 0
			if r.selectedIdx >= visibleRows {
				startIdx = r.selectedIdx - visibleRows + 1
			}
			endIdx := startIdx + visibleRows
			if endIdx > len(r.filteredUsers) {
				endIdx = len(r.filteredUsers)
			}

			for i := startIdx; i < endIdx; i++ {
				user := r.filteredUsers[i]
				var line string

				// User icon based on type
				icon := "👤"
				if user.Kind == "ServiceAccount" {
					icon = "🤖"
				}

				// Name with selection
				nameStr := fmt.Sprintf("%s %s", icon, user.Name)
				if i == r.selectedIdx {
					nameStr = S.Selected.Render(Icons.Arrow + " " + nameStr)
				} else {
					nameStr = S.Value.Render("  " + nameStr)
				}

				// Kind and namespace
				kindStr := S.Muted.Render(user.Kind)
				if user.Kind == "ServiceAccount" && user.Namespace != "" {
					kindStr += S.Muted.Render(" (" + user.Namespace + ")")
				}

				// Permissions summary
				permSummary := helm.FormatPermissionsSummary(user.Permissions)
				permStr := S.Muted.Render("    " + permSummary)

				line = nameStr + "  " + kindStr + "\n" + permStr
				b.WriteString(line + "\n\n")
			}

			// Scroll indicator
			if len(r.filteredUsers) > visibleRows {
				b.WriteString(S.Muted.Render(fmt.Sprintf("  [%d/%d]", r.selectedIdx+1, len(r.filteredUsers))) + "\n")
			}
		}
	}

	content := S.BorderFocus.Width(r.width - 4).Height(r.height - 10).Render(b.String())

	// Status bar
	var statusLeft string
	if r.status != "" {
		statusLeft = S.Success.Render(Icons.Check + " " + r.status)
	}
	if r.err != nil {
		statusLeft = S.Error.Render(Icons.Cross + " " + r.err.Error())
	}

	// Add filter hints to status bar
	hints := [][2]string{{"a", "add"}, {"e", "edit"}, {"/", "search"}, {"f", "type"}, {"n", "ns"}}
	if r.hasActiveFilters() {
		hints = append(hints, [2]string{"Esc", "clear"})
	}
	hints = append(hints, [2]string{"r", "refresh"})
	statusRight := KeyHints(hints)
	statusBar := StatusBarView(statusLeft, statusRight, r.width)

	return content + "\n" + statusBar
}

func (r *RBACView) renderAddDialog() string {
	dialogStyle := lipgloss.NewStyle().
		Border(lipgloss.DoubleBorder()).
		BorderForeground(DefaultTheme.Primary).
		Padding(1, 2).
		Width(60)

	var b strings.Builder
	b.WriteString(S.Title.Render(Icons.Add+" Add User Access") + "\n\n")

	// User type selector
	saIcon := "○"
	userIcon := "○"
	saStyle := S.Value
	userStyle := S.Value
	if r.addUserType == 0 {
		saIcon = "◉"
		saStyle = S.Selected
	} else {
		userIcon = "◉"
		userStyle = S.Selected
	}

	typeLabel := "  User type:"
	if r.addField == 0 {
		typeLabel = S.Highlighted.Render(Icons.Arrow + " User type:")
	} else {
		typeLabel = S.Label.Render(typeLabel)
	}
	b.WriteString(typeLabel + "\n")
	b.WriteString("  " + saStyle.Render(saIcon+" ServiceAccount") + "   " + userStyle.Render(userIcon+" Human User") + "\n\n")

	// Name field
	nameLabel := "  Name:"
	if r.addField == 1 {
		nameLabel = S.Highlighted.Render(Icons.Arrow + " Name:")
	} else {
		nameLabel = S.Label.Render(nameLabel)
	}
	b.WriteString(nameLabel + "\n  " + r.addNameInput.View() + "\n\n")

	// SA namespace field (only for ServiceAccount)
	if r.addUserType == 0 {
		nsLabel := "  SA Namespace:"
		if r.addField == 2 {
			nsLabel = S.Highlighted.Render(Icons.Arrow + " SA Namespace:")
		} else {
			nsLabel = S.Label.Render(nsLabel)
		}
		b.WriteString(nsLabel + "\n  " + r.addNamespaceInput.View() + "\n")
		b.WriteString(S.Muted.Render("  Where to create the ServiceAccount") + "\n\n")
	}

	// Target namespace field
	targetNSFieldIdx := 3
	if r.addUserType == 1 {
		targetNSFieldIdx = 2
	}
	targetLabel := "  Target Namespace:"
	if r.addField == targetNSFieldIdx {
		targetLabel = S.Highlighted.Render(Icons.Arrow + " Target Namespace:")
	} else {
		targetLabel = S.Label.Render(targetLabel)
	}
	b.WriteString(targetLabel + "\n  " + r.addTargetNSInput.View() + "\n")
	b.WriteString(S.Muted.Render("  Namespace to grant access to") + "\n\n")

	// Permission selector
	permFieldIdx := r.getPermissionFieldIndex()
	permLabel := "  Permission:"
	if r.addField == permFieldIdx {
		permLabel = S.Highlighted.Render(Icons.Arrow + " Permission:")
	} else {
		permLabel = S.Label.Render(permLabel)
	}
	b.WriteString(permLabel + "\n  ")

	presets := helm.GetPermissionPresets()
	for i, p := range presets {
		icon := "○"
		style := S.Value
		if i == r.addPermission {
			icon = "◉"
			style = S.Selected
		}
		b.WriteString(style.Render(icon+" "+p) + "  ")
	}
	b.WriteString("\n\n")

	// Error message
	if r.addError != nil {
		b.WriteString(S.Error.Render(Icons.Cross+" "+r.addError.Error()) + "\n\n")
	}

	// Buttons
	createFieldIdx := r.getAddFieldCount() - 2
	cancelFieldIdx := r.getAddFieldCount() - 1
	createBtn := S.Button.Render("Create")
	cancelBtn := S.Button.Render("Cancel")
	if r.addField == createFieldIdx {
		createBtn = S.ButtonFocus.Render("Create")
	}
	if r.addField == cancelFieldIdx {
		cancelBtn = S.ButtonFocus.Render("Cancel")
	}
	b.WriteString("  " + createBtn + "  " + cancelBtn + "\n\n")
	b.WriteString(S.Muted.Render("  Tab: next  ←/→: select  Enter: confirm  Esc: cancel"))

	dialog := dialogStyle.Render(b.String())
	return lipgloss.Place(r.width, r.height, lipgloss.Center, lipgloss.Center, dialog)
}

func (r *RBACView) renderEditDialog() string {
	dialogStyle := lipgloss.NewStyle().
		Border(lipgloss.DoubleBorder()).
		BorderForeground(DefaultTheme.Primary).
		Padding(1, 2).
		Width(65)

	var b strings.Builder
	b.WriteString(S.Title.Render(Icons.Edit+" Edit: "+r.editUser.Name) + "\n\n")

	// User info
	kindStr := r.editUser.Kind
	if r.editUser.Kind == "ServiceAccount" && r.editUser.Namespace != "" {
		kindStr += " (" + r.editUser.Namespace + ")"
	}
	b.WriteString(S.Label.Render("Type: ") + S.Value.Render(kindStr) + "\n\n")

	// Permissions table
	b.WriteString(S.Highlighted.Render("Current Access:") + "\n\n")

	if len(r.editUser.Permissions) == 0 {
		b.WriteString(S.Muted.Render("  No permissions configured") + "\n")
	} else {
		// Header
		header := fmt.Sprintf("  %-20s %-18s %s", "Namespace", "Permission", "Actions")
		b.WriteString(S.Muted.Render(header) + "\n")
		b.WriteString(S.Muted.Render("  "+strings.Repeat("─", 55)) + "\n")

		for i, perm := range r.editUser.Permissions {
			ns := perm.Namespace
			if len(ns) > 18 {
				ns = ns[:15] + "..."
			}
			permStr := perm.Permission
			if len(permStr) > 16 {
				permStr = permStr[:13] + "..."
			}

			line := fmt.Sprintf("  %-20s %-18s", ns, permStr)
			if i == r.editSelectedPerm {
				line = S.Selected.Render(Icons.Arrow + line[1:])
				line += " " + S.Error.Render("[x:remove]")
			} else {
				line = S.Value.Render(line)
			}
			b.WriteString(line + "\n")
		}
	}

	b.WriteString("\n" + S.Muted.Render("  j/k: navigate  x: remove access  Esc: done"))

	dialog := dialogStyle.Render(b.String())
	return lipgloss.Place(r.width, r.height, lipgloss.Center, lipgloss.Center, dialog)
}

func (r *RBACView) renderAddNSDialog() string {
	dialogStyle := lipgloss.NewStyle().
		Border(lipgloss.DoubleBorder()).
		BorderForeground(DefaultTheme.Primary).
		Padding(1, 2).
		Width(55)

	user := r.users[r.addNSUserIdx]

	var b strings.Builder
	b.WriteString(S.Title.Render(Icons.Add+" Add Namespace Access") + "\n\n")
	b.WriteString(S.Label.Render("User: ") + S.Value.Render(user.Name) + "\n\n")

	// Namespace field
	nsLabel := "  Namespace:"
	if r.addNSField == 0 {
		nsLabel = S.Highlighted.Render(Icons.Arrow + " Namespace:")
	} else {
		nsLabel = S.Label.Render(nsLabel)
	}
	b.WriteString(nsLabel + "\n  " + r.addNSNamespaceInput.View() + "\n\n")

	// Permission selector
	permLabel := "  Permission:"
	if r.addNSField == 1 {
		permLabel = S.Highlighted.Render(Icons.Arrow + " Permission:")
	} else {
		permLabel = S.Label.Render(permLabel)
	}
	b.WriteString(permLabel + "\n  ")

	presets := helm.GetPermissionPresets()
	for i, p := range presets {
		icon := "○"
		style := S.Value
		if i == r.addNSPermission {
			icon = "◉"
			style = S.Selected
		}
		b.WriteString(style.Render(icon+" "+p) + "  ")
	}
	b.WriteString("\n\n")

	// Error message
	if r.addNSError != nil {
		b.WriteString(S.Error.Render(Icons.Cross+" "+r.addNSError.Error()) + "\n\n")
	}

	// Buttons
	addBtn := S.Button.Render("Add")
	cancelBtn := S.Button.Render("Cancel")
	if r.addNSField == 2 {
		addBtn = S.ButtonFocus.Render("Add")
	}
	if r.addNSField == 3 {
		cancelBtn = S.ButtonFocus.Render("Cancel")
	}
	b.WriteString("  " + addBtn + "  " + cancelBtn + "\n\n")
	b.WriteString(S.Muted.Render("  Tab: next  ←/→: select  Enter: confirm  Esc: cancel"))

	dialog := dialogStyle.Render(b.String())
	return lipgloss.Place(r.width, r.height, lipgloss.Center, lipgloss.Center, dialog)
}

func (r *RBACView) renderDeleteConfirm() string {
	dialogStyle := lipgloss.NewStyle().
		Border(lipgloss.DoubleBorder()).
		BorderForeground(DefaultTheme.Error).
		Padding(2, 4).
		Width(50)

	content := S.Error.Render(Icons.Delete+" Delete User") + "\n\n"
	if r.deleteUser != nil {
		content += fmt.Sprintf("Remove \"%s\" (%s)\n", r.deleteUser.Name, r.deleteUser.Kind)
		content += "and all their access?\n\n"
	}
	content += S.Muted.Render("[Y]es  [N]o")

	dialog := dialogStyle.Render(content)
	return lipgloss.Place(r.width, r.height, lipgloss.Center, lipgloss.Center, dialog)
}

func (r *RBACView) renderKubeconfigDialog() string {
	dialogStyle := lipgloss.NewStyle().
		Border(lipgloss.DoubleBorder()).
		BorderForeground(DefaultTheme.Primary).
		Padding(1, 2).
		Width(75).
		Height(30)

	var b strings.Builder
	b.WriteString(S.Title.Render("📋 Kubeconfig Export") + "\n\n")

	if r.kubeconfigUser != nil {
		b.WriteString(S.Label.Render("ServiceAccount: ") + S.Value.Render(r.kubeconfigUser.Name) + "\n")
		b.WriteString(S.Label.Render("Namespace: ") + S.Value.Render(r.kubeconfigUser.Namespace) + "\n\n")
	}

	if r.kubeconfigErr != nil {
		b.WriteString(S.Error.Render(Icons.Cross+" Error: "+r.kubeconfigErr.Error()) + "\n")
	} else if r.kubeconfigContent == "" {
		b.WriteString(S.Muted.Render("Generating kubeconfig...") + "\n")
	} else if r.kubeconfigSaved {
		// Show usage instructions after save
		b.WriteString(S.Success.Render(Icons.Check+" Kubeconfig saved successfully!") + "\n\n")
		b.WriteString(S.Highlighted.Render("Usage Instructions:") + "\n\n")

		// Option 1: Export variable
		b.WriteString(S.Label.Render("Option 1: Set KUBECONFIG environment variable") + "\n")
		b.WriteString(S.Value.Render(fmt.Sprintf("  export KUBECONFIG=%s", r.kubeconfigPath)) + "\n\n")

		// Option 2: Use --kubeconfig flag
		b.WriteString(S.Label.Render("Option 2: Use --kubeconfig flag") + "\n")
		b.WriteString(S.Value.Render(fmt.Sprintf("  kubectl --kubeconfig=%s get pods", r.kubeconfigPath)) + "\n\n")

		// Option 3: Merge with existing
		b.WriteString(S.Label.Render("Option 3: Merge with existing kubeconfig") + "\n")
		b.WriteString(S.Value.Render(fmt.Sprintf("  export KUBECONFIG=~/.kube/config:%s", r.kubeconfigPath)) + "\n")
		b.WriteString(S.Value.Render("  kubectl config view --flatten > ~/.kube/config.merged") + "\n")
		b.WriteString(S.Value.Render("  mv ~/.kube/config.merged ~/.kube/config") + "\n\n")

		// Test connectivity
		b.WriteString(S.Label.Render("Test connectivity:") + "\n")
		b.WriteString(S.Value.Render(fmt.Sprintf("  kubectl --kubeconfig=%s auth whoami", r.kubeconfigPath)) + "\n")
	} else {
		// Show truncated kubeconfig preview
		lines := strings.Split(r.kubeconfigContent, "\n")
		maxLines := 12
		if len(lines) > maxLines {
			for i := 0; i < maxLines; i++ {
				b.WriteString(S.Muted.Render(lines[i]) + "\n")
			}
			b.WriteString(S.Muted.Render(fmt.Sprintf("... (%d more lines)", len(lines)-maxLines)) + "\n")
		} else {
			for _, line := range lines {
				b.WriteString(S.Muted.Render(line) + "\n")
			}
		}
		b.WriteString("\n" + S.Muted.Render("s: save to file  Esc: close"))
	}

	if !r.kubeconfigSaved && r.status != "" {
		b.WriteString("\n" + S.Success.Render(Icons.Check+" "+r.status))
	}

	if r.kubeconfigSaved {
		b.WriteString("\n" + S.Muted.Render("Esc: close"))
	}

	dialog := dialogStyle.Render(b.String())
	return lipgloss.Place(r.width, r.height, lipgloss.Center, lipgloss.Center, dialog)
}

// --- Commands ---

type rbacUsersLoadedMsg struct {
	users []helm.ManagedUser
	err   error
}

type rbacGrantResultMsg struct {
	err error
}

type rbacDeleteResultMsg struct {
	err error
}

type rbacRevokeResultMsg struct {
	err error
}

type rbacKubeconfigMsg struct {
	content string
	err     error
}

func (r *RBACView) loadUsers() tea.Cmd {
	return func() tea.Msg {
		if r.rbacManager == nil {
			return rbacUsersLoadedMsg{err: fmt.Errorf("helm client not initialized")}
		}
		users, err := r.rbacManager.ListManagedUsers()
		return rbacUsersLoadedMsg{users: users, err: err}
	}
}

func (r *RBACView) grantAccess(user helm.ManagedUser, namespace, permission string) tea.Cmd {
	return func() tea.Msg {
		if r.rbacManager == nil {
			return rbacGrantResultMsg{err: fmt.Errorf("helm client not initialized")}
		}
		err := r.rbacManager.GrantAccess(user, namespace, permission)
		return rbacGrantResultMsg{err: err}
	}
}

func (r *RBACView) deleteUserCmd(user helm.ManagedUser) tea.Cmd {
	return func() tea.Msg {
		if r.rbacManager == nil {
			return rbacDeleteResultMsg{err: fmt.Errorf("helm client not initialized")}
		}
		err := r.rbacManager.DeleteUser(user)
		return rbacDeleteResultMsg{err: err}
	}
}

func (r *RBACView) revokeAccess(user helm.ManagedUser, namespace string) tea.Cmd {
	return func() tea.Msg {
		if r.rbacManager == nil {
			return rbacRevokeResultMsg{err: fmt.Errorf("helm client not initialized")}
		}
		err := r.rbacManager.RevokeAccess(user, namespace)
		return rbacRevokeResultMsg{err: err}
	}
}

func (r *RBACView) generateKubeconfig(user *helm.ManagedUser) tea.Cmd {
	return func() tea.Msg {
		if r.rbacManager == nil {
			return rbacKubeconfigMsg{err: fmt.Errorf("helm client not initialized")}
		}
		content, err := r.rbacManager.GenerateKubeconfig(*user)
		return rbacKubeconfigMsg{content: content, err: err}
	}
}

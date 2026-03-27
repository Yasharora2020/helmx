package helm

import (
	"context"
	"encoding/base64"
	"fmt"
	"sort"
	"strings"

	authv1 "k8s.io/api/authentication/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

// ManagedUser represents a user (ServiceAccount or User) with their permissions
type ManagedUser struct {
	Name        string            // Username or SA name
	Kind        string            // "ServiceAccount" or "User"
	Namespace   string            // SA namespace (empty for User)
	Permissions []NamespaceAccess // Aggregated from RoleBindings
}

// NamespaceAccess represents access granted to a namespace
type NamespaceAccess struct {
	Namespace   string // Target namespace ("*" for cluster-wide)
	Permission  string // "read-only", "developer", "namespace-admin", "custom"
	RoleName    string // Actual K8s Role/ClusterRole name
	BindingName string // RoleBinding that grants this
}

// Permission preset names
const (
	PermissionReadOnly       = "read-only"
	PermissionDeveloper      = "developer"
	PermissionNamespaceAdmin = "namespace-admin"
	PermissionCustom         = "custom"
)

// helmx- prefix for managed resources
const managedPrefix = "helmx-"

// getKubernetesClient returns a cached Kubernetes clientset.
// It delegates to getClientset() which uses sync.Once for thread-safe caching.
func (c *Client) getKubernetesClient() (*kubernetes.Clientset, error) {
	clientset, _, err := c.getClientset()
	return clientset, err
}

// ListManagedUsers returns all users with RBAC permissions in the cluster
// It aggregates ServiceAccounts and Users from RoleBindings
func (c *Client) ListManagedUsers() ([]ManagedUser, error) {
	clientset, err := c.getKubernetesClient()
	if err != nil {
		return nil, err
	}

	ctx := context.Background()

	// Map to aggregate permissions by user
	userMap := make(map[string]*ManagedUser)

	// List all RoleBindings across all namespaces
	roleBindings, err := clientset.RbacV1().RoleBindings("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list role bindings: %w", err)
	}

	for _, rb := range roleBindings.Items {
		// Only process helmx-managed role bindings
		if !strings.HasPrefix(rb.Name, managedPrefix) {
			continue
		}
		for _, subject := range rb.Subjects {
			if subject.Kind != "ServiceAccount" && subject.Kind != "User" {
				continue
			}

			// Create unique key for user
			var key string
			if subject.Kind == "ServiceAccount" {
				ns := subject.Namespace
				if ns == "" {
					ns = rb.Namespace
				}
				key = fmt.Sprintf("ServiceAccount/%s/%s", ns, subject.Name)
			} else {
				key = fmt.Sprintf("User/%s", subject.Name)
			}

			// Get or create user entry
			user, exists := userMap[key]
			if !exists {
				user = &ManagedUser{
					Name:      subject.Name,
					Kind:      subject.Kind,
					Namespace: subject.Namespace,
				}
				if subject.Kind == "ServiceAccount" && user.Namespace == "" {
					user.Namespace = rb.Namespace
				}
				userMap[key] = user
			}

			// Determine permission level from role name
			permission := determinePermissionLevel(rb.RoleRef.Name)

			user.Permissions = append(user.Permissions, NamespaceAccess{
				Namespace:   rb.Namespace,
				Permission:  permission,
				RoleName:    rb.RoleRef.Name,
				BindingName: rb.Name,
			})
		}
	}

	// List ClusterRoleBindings for cluster-wide access
	clusterBindings, err := clientset.RbacV1().ClusterRoleBindings().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list cluster role bindings: %w", err)
	}

	for _, crb := range clusterBindings.Items {
		// Only process helmx-managed cluster role bindings
		if !strings.HasPrefix(crb.Name, managedPrefix) {
			continue
		}
		for _, subject := range crb.Subjects {
			if subject.Kind != "ServiceAccount" && subject.Kind != "User" {
				continue
			}

			var key string
			if subject.Kind == "ServiceAccount" {
				key = fmt.Sprintf("ServiceAccount/%s/%s", subject.Namespace, subject.Name)
			} else {
				key = fmt.Sprintf("User/%s", subject.Name)
			}

			user, exists := userMap[key]
			if !exists {
				user = &ManagedUser{
					Name:      subject.Name,
					Kind:      subject.Kind,
					Namespace: subject.Namespace,
				}
				userMap[key] = user
			}

			permission := determinePermissionLevel(crb.RoleRef.Name)

			user.Permissions = append(user.Permissions, NamespaceAccess{
				Namespace:   "*", // Cluster-wide
				Permission:  permission,
				RoleName:    crb.RoleRef.Name,
				BindingName: crb.Name,
			})
		}
	}

	// Convert map to slice
	users := make([]ManagedUser, 0, len(userMap))
	for _, user := range userMap {
		users = append(users, *user)
	}

	// Sort by name
	sort.Slice(users, func(i, j int) bool {
		return users[i].Name < users[j].Name
	})

	return users, nil
}

// determinePermissionLevel tries to identify the permission level from a role name
func determinePermissionLevel(roleName string) string {
	lowerName := strings.ToLower(roleName)

	// Check for our managed roles first
	switch roleName {
	case managedPrefix + "read-only":
		return PermissionReadOnly
	case managedPrefix + "developer":
		return PermissionDeveloper
	case managedPrefix + "namespace-admin":
		return PermissionNamespaceAdmin
	}

	// Check common patterns
	if strings.Contains(lowerName, "admin") || strings.Contains(lowerName, "cluster-admin") {
		return PermissionNamespaceAdmin
	}
	if strings.Contains(lowerName, "edit") || strings.Contains(lowerName, "developer") {
		return PermissionDeveloper
	}
	if strings.Contains(lowerName, "view") || strings.Contains(lowerName, "read") {
		return PermissionReadOnly
	}

	return PermissionCustom
}

// GrantAccess grants a user access to a namespace with a specific permission level.
// Pass namespace="*" for cluster-wide access (uses ClusterRole + ClusterRoleBinding).
func (c *Client) GrantAccess(user ManagedUser, namespace, permission string) error {
	clientset, err := c.getKubernetesClient()
	if err != nil {
		return err
	}

	ctx := context.Background()

	// For ServiceAccounts, create the SA in its own namespace if it doesn't exist
	if user.Kind == "ServiceAccount" {
		saNamespace := user.Namespace
		if saNamespace == "" {
			// Fall back to target namespace only when it's a real namespace
			if namespace != "*" {
				saNamespace = namespace
			} else {
				saNamespace = "default"
			}
		}

		_, err := clientset.CoreV1().ServiceAccounts(saNamespace).Get(ctx, user.Name, metav1.GetOptions{})
		if err != nil {
			sa := &corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name:      user.Name,
					Namespace: saNamespace,
					Labels: map[string]string{
						"app.kubernetes.io/managed-by": "helmx",
					},
				},
			}
			_, err = clientset.CoreV1().ServiceAccounts(saNamespace).Create(ctx, sa, metav1.CreateOptions{})
			if err != nil {
				return fmt.Errorf("failed to create service account: %w", err)
			}
		}
	}

	bindingName := fmt.Sprintf("%s%s-%s", managedPrefix, user.Name, permission)
	roleName := managedPrefix + permission

	subject := rbacv1.Subject{
		Kind: user.Kind,
		Name: user.Name,
	}
	if user.Kind == "ServiceAccount" {
		subject.Namespace = user.Namespace
		if subject.Namespace == "" {
			if namespace != "*" {
				subject.Namespace = namespace
			} else {
				subject.Namespace = "default"
			}
		}
	}

	if namespace == "*" {
		// Cluster-wide: use ClusterRole + ClusterRoleBinding
		_, err = clientset.RbacV1().ClusterRoles().Get(ctx, roleName, metav1.GetOptions{})
		if err != nil {
			cr := createClusterRoleForPermission(roleName, permission)
			_, err = clientset.RbacV1().ClusterRoles().Create(ctx, cr, metav1.CreateOptions{})
			if err != nil {
				return fmt.Errorf("failed to create cluster role: %w", err)
			}
		}

		crb := &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: bindingName,
				Labels: map[string]string{
					"app.kubernetes.io/managed-by": "helmx",
				},
			},
			Subjects: []rbacv1.Subject{subject},
			RoleRef: rbacv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "ClusterRole",
				Name:     roleName,
			},
		}
		_, err = clientset.RbacV1().ClusterRoleBindings().Create(ctx, crb, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("failed to create cluster role binding: %w", err)
		}
		return nil
	}

	// Namespace-scoped: use Role + RoleBinding
	role, err := clientset.RbacV1().Roles(namespace).Get(ctx, roleName, metav1.GetOptions{})
	if err != nil {
		role = createRoleForPermission(roleName, namespace, permission)
		_, err = clientset.RbacV1().Roles(namespace).Create(ctx, role, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("failed to create role: %w", err)
		}
	}

	rb := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      bindingName,
			Namespace: namespace,
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "helmx",
			},
		},
		Subjects: []rbacv1.Subject{subject},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "Role",
			Name:     role.Name,
		},
	}

	_, err = clientset.RbacV1().RoleBindings(namespace).Create(ctx, rb, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create role binding: %w", err)
	}

	return nil
}

// createRoleForPermission creates a Role with the appropriate rules for the permission level
func createRoleForPermission(name, namespace, permission string) *rbacv1.Role {
	role := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "helmx",
			},
		},
	}

	switch permission {
	case PermissionReadOnly:
		role.Rules = []rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{"pods", "services", "configmaps", "endpoints", "persistentvolumeclaims"},
				Verbs:     []string{"get", "list", "watch"},
			},
			{
				APIGroups: []string{"apps"},
				Resources: []string{"deployments", "replicasets", "statefulsets", "daemonsets"},
				Verbs:     []string{"get", "list", "watch"},
			},
			{
				APIGroups: []string{"batch"},
				Resources: []string{"jobs", "cronjobs"},
				Verbs:     []string{"get", "list", "watch"},
			},
			{
				APIGroups: []string{"networking.k8s.io"},
				Resources: []string{"ingresses", "networkpolicies"},
				Verbs:     []string{"get", "list", "watch"},
			},
		}

	case PermissionDeveloper:
		role.Rules = []rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{"pods", "pods/log", "pods/exec", "services", "configmaps", "secrets", "endpoints", "persistentvolumeclaims"},
				Verbs:     []string{"get", "list", "watch", "create", "update", "patch", "delete"},
			},
			{
				APIGroups: []string{"apps"},
				Resources: []string{"deployments", "replicasets", "statefulsets"},
				Verbs:     []string{"get", "list", "watch", "create", "update", "patch", "delete"},
			},
			{
				APIGroups: []string{"batch"},
				Resources: []string{"jobs", "cronjobs"},
				Verbs:     []string{"get", "list", "watch", "create", "update", "patch", "delete"},
			},
			{
				APIGroups: []string{"networking.k8s.io"},
				Resources: []string{"ingresses"},
				Verbs:     []string{"get", "list", "watch", "create", "update", "patch", "delete"},
			},
		}

	case PermissionNamespaceAdmin:
		role.Rules = []rbacv1.PolicyRule{
			{
				APIGroups: []string{"*"},
				Resources: []string{"*"},
				Verbs:     []string{"*"},
			},
		}
	}

	return role
}

// createClusterRoleForPermission creates a ClusterRole with rules matching the permission level.
// Used when granting cluster-wide access (namespace="*").
func createClusterRoleForPermission(name, permission string) *rbacv1.ClusterRole {
	cr := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "helmx",
			},
		},
	}

	// Reuse the same rule sets as namespace-scoped roles
	tmp := createRoleForPermission(name, "", permission)
	cr.Rules = tmp.Rules
	return cr
}

// RevokeAccess removes a user's access to a namespace
func (c *Client) RevokeAccess(user ManagedUser, namespace string) error {
	clientset, err := c.getKubernetesClient()
	if err != nil {
		return err
	}

	ctx := context.Background()

	// Find the RoleBinding for this user in this namespace
	roleBindings, err := clientset.RbacV1().RoleBindings(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: "app.kubernetes.io/managed-by=helmx",
	})
	if err != nil {
		return fmt.Errorf("failed to list role bindings: %w", err)
	}

	for _, rb := range roleBindings.Items {
		for _, subject := range rb.Subjects {
			if subject.Name == user.Name && subject.Kind == user.Kind {
				err = clientset.RbacV1().RoleBindings(namespace).Delete(ctx, rb.Name, metav1.DeleteOptions{})
				if err != nil {
					return fmt.Errorf("failed to delete role binding: %w", err)
				}
			}
		}
	}

	// Clean up orphaned helmx- roles if no other bindings reference them
	// Errors are intentionally ignored here because:
	// 1. The main revoke operation already succeeded
	// 2. Orphaned role cleanup is a best-effort optimization
	// 3. The roles will be cleaned up on subsequent operations
	_ = c.cleanupOrphanedRoles(clientset, namespace)

	return nil
}

// cleanupOrphanedRoles removes helmx- prefixed roles that have no bindings
func (c *Client) cleanupOrphanedRoles(clientset *kubernetes.Clientset, namespace string) error {
	ctx := context.Background()

	// Get all helmx- managed roles
	roles, err := clientset.RbacV1().Roles(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: "app.kubernetes.io/managed-by=helmx",
	})
	if err != nil {
		return err
	}

	// Get all role bindings
	bindings, err := clientset.RbacV1().RoleBindings(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}

	// Build a set of referenced roles
	referencedRoles := make(map[string]bool)
	for _, rb := range bindings.Items {
		referencedRoles[rb.RoleRef.Name] = true
	}

	// Delete orphaned roles
	for _, role := range roles.Items {
		if !referencedRoles[role.Name] {
			_ = clientset.RbacV1().Roles(namespace).Delete(ctx, role.Name, metav1.DeleteOptions{})
		}
	}

	return nil
}

// UpdatePermission changes a user's permission level in a namespace
func (c *Client) UpdatePermission(user ManagedUser, namespace, newPermission string) error {
	// Remove existing access and grant new
	if err := c.RevokeAccess(user, namespace); err != nil {
		return fmt.Errorf("failed to revoke existing access: %w", err)
	}

	if err := c.GrantAccess(user, namespace, newPermission); err != nil {
		return fmt.Errorf("failed to grant new access: %w", err)
	}

	return nil
}

// DeleteUser removes a user and all their access
func (c *Client) DeleteUser(user ManagedUser) error {
	clientset, err := c.getKubernetesClient()
	if err != nil {
		return err
	}

	ctx := context.Background()

	// Delete all RoleBindings for this user
	namespaces, err := clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list namespaces: %w", err)
	}

	for _, ns := range namespaces.Items {
		roleBindings, err := clientset.RbacV1().RoleBindings(ns.Name).List(ctx, metav1.ListOptions{
			LabelSelector: "app.kubernetes.io/managed-by=helmx",
		})
		if err != nil {
			continue
		}

		for _, rb := range roleBindings.Items {
			for _, subject := range rb.Subjects {
				if subject.Name == user.Name && subject.Kind == user.Kind {
					_ = clientset.RbacV1().RoleBindings(ns.Name).Delete(ctx, rb.Name, metav1.DeleteOptions{})
				}
			}
		}
	}

	// Delete ClusterRoleBindings
	clusterBindings, err := clientset.RbacV1().ClusterRoleBindings().List(ctx, metav1.ListOptions{
		LabelSelector: "app.kubernetes.io/managed-by=helmx",
	})
	if err == nil {
		for _, crb := range clusterBindings.Items {
			for _, subject := range crb.Subjects {
				if subject.Name == user.Name && subject.Kind == user.Kind {
					_ = clientset.RbacV1().ClusterRoleBindings().Delete(ctx, crb.Name, metav1.DeleteOptions{})
				}
			}
		}
	}

	// For ServiceAccounts managed by helmx, delete the SA
	if user.Kind == "ServiceAccount" && user.Namespace != "" {
		sa, err := clientset.CoreV1().ServiceAccounts(user.Namespace).Get(ctx, user.Name, metav1.GetOptions{})
		if err == nil && sa.Labels["app.kubernetes.io/managed-by"] == "helmx" {
			_ = clientset.CoreV1().ServiceAccounts(user.Namespace).Delete(ctx, user.Name, metav1.DeleteOptions{})
		}
	}

	return nil
}

// GenerateKubeconfig generates a kubeconfig for a ServiceAccount
func (c *Client) GenerateKubeconfig(user ManagedUser) (string, error) {
	if user.Kind != "ServiceAccount" {
		return "", fmt.Errorf("kubeconfig generation only supported for ServiceAccounts")
	}

	clientset, err := c.getKubernetesClient()
	if err != nil {
		return "", err
	}

	ctx := context.Background()

	// Verify the service account exists
	_, err = clientset.CoreV1().ServiceAccounts(user.Namespace).Get(ctx, user.Name, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to get service account: %w", err)
	}

	// Get current context for cluster info
	config, err := clientcmd.LoadFromFile(c.kubeconfig)
	if err != nil {
		return "", fmt.Errorf("failed to load kubeconfig: %w", err)
	}

	currentContext := config.CurrentContext
	ctxConfig, ok := config.Contexts[currentContext]
	if !ok {
		return "", fmt.Errorf("current context not found")
	}

	cluster, ok := config.Clusters[ctxConfig.Cluster]
	if !ok {
		return "", fmt.Errorf("cluster not found")
	}

	// Try to create a token using TokenRequest API (K8s 1.22+)
	expirationSeconds := int64(3600 * 24 * 7) // 7 days
	tokenRequest := &authv1.TokenRequest{
		Spec: authv1.TokenRequestSpec{
			ExpirationSeconds: &expirationSeconds,
		},
	}

	token, err := clientset.CoreV1().ServiceAccounts(user.Namespace).CreateToken(
		ctx,
		user.Name,
		tokenRequest,
		metav1.CreateOptions{},
	)

	var saToken string
	if err != nil {
		// Fall back to reading from secret (for older clusters)
		secrets, err := clientset.CoreV1().Secrets(user.Namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			return "", fmt.Errorf("failed to list secrets: %w", err)
		}

		for _, secret := range secrets.Items {
			if secret.Type == corev1.SecretTypeServiceAccountToken {
				if secret.Annotations["kubernetes.io/service-account.name"] == user.Name {
					saToken = string(secret.Data["token"])
					break
				}
			}
		}

		if saToken == "" {
			return "", fmt.Errorf("no token found for service account (TokenRequest API failed: %v)", err)
		}
	} else {
		saToken = token.Status.Token
	}

	// Encode CA data
	caData := base64.StdEncoding.EncodeToString(cluster.CertificateAuthorityData)

	// Generate kubeconfig
	kubeconfigContent := fmt.Sprintf(`apiVersion: v1
kind: Config
clusters:
- name: %s
  cluster:
    server: %s
    certificate-authority-data: %s
contexts:
- name: %s-%s
  context:
    cluster: %s
    namespace: %s
    user: %s
current-context: %s-%s
users:
- name: %s
  user:
    token: %s
`,
		ctxConfig.Cluster,
		cluster.Server,
		caData,
		user.Name, user.Namespace,
		ctxConfig.Cluster,
		user.Namespace,
		user.Name,
		user.Name, user.Namespace,
		user.Name,
		saToken,
	)

	return kubeconfigContent, nil
}

// ListAllNamespaces returns all namespaces in the cluster
func (c *Client) ListAllNamespaces() ([]string, error) {
	clientset, err := c.getKubernetesClient()
	if err != nil {
		return nil, err
	}

	namespaces, err := clientset.CoreV1().Namespaces().List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list namespaces: %w", err)
	}

	names := make([]string, len(namespaces.Items))
	for i, ns := range namespaces.Items {
		names[i] = ns.Name
	}

	sort.Strings(names)
	return names, nil
}

// GetPermissionPresets returns available permission presets
func GetPermissionPresets() []string {
	return []string{PermissionReadOnly, PermissionDeveloper, PermissionNamespaceAdmin}
}

// FormatPermissionsSummary creates a summary string of a user's permissions
func FormatPermissionsSummary(permissions []NamespaceAccess) string {
	if len(permissions) == 0 {
		return "no access"
	}

	// Group by permission level
	nsGroups := make(map[string][]string)
	for _, p := range permissions {
		nsGroups[p.Permission] = append(nsGroups[p.Permission], p.Namespace)
	}

	var parts []string
	for perm, namespaces := range nsGroups {
		if len(namespaces) == 1 && namespaces[0] == "*" {
			parts = append(parts, fmt.Sprintf("all namespaces (%s)", perm))
		} else if len(namespaces) <= 3 {
			parts = append(parts, fmt.Sprintf("%s (%s)", strings.Join(namespaces, ", "), perm))
		} else {
			parts = append(parts, fmt.Sprintf("%d namespaces (%s)", len(namespaces), perm))
		}
	}

	return strings.Join(parts, "; ")
}

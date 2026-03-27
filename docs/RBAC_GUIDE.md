# RBAC Management Guide for helmx

Complete reference for managing Kubernetes RBAC through helmx, including both ServiceAccounts and Corporate Identity users.

## Table of Contents
1. [ServiceAccounts vs Manual Users](#serviceaccounts-vs-manual-users)
2. [Corporate Identity Flow](#corporate-identity-flow)
3. [How helmx Fits In](#how-helmx-fits-in)
4. [Practical Workflows](#practical-workflows)
5. [Exporting Kubeconfigs](#exporting-kubeconfigs)

---

## ServiceAccounts vs Manual Users

### ServiceAccounts ✅ (Can Export Kubeconfig)

**What they are:**
- Real Kubernetes objects that live in a namespace
- Auto-generated credentials (certificate + key stored in secrets)
- Built-in authentication via certificates
- Perfect for app-to-app, automation, CI/CD pipelines

**How they work:**
```
ServiceAccount created in Kubernetes
  ↓
Secret auto-generated with cert/key
  ↓
helmx extracts cert/key data
  ↓
Builds kubeconfig file
  ↓
Export to file for use by apps/scripts
```

**Example Use Cases:**
- CI/CD pipelines (GitHub Actions, GitLab CI)
- Helm deployments from automated systems
- Microservices authenticating to Kubernetes
- Testing and demos

---

### Manual Users ❌ (Cannot Export Kubeconfig)

**What they are:**
- NOT real Kubernetes objects
- Just tracking entries in helmx for documentation
- No auto-generated credentials
- Meant for real people using external identity providers

**How they work:**
```
Manual User created in helmx
  ↓
helmx creates RoleBinding in Kubernetes
  ↓
User authenticates via corporate SSO (Azure AD, Okta, OIDC, etc.)
  ↓
Kubernetes checks RoleBinding
  ↓
Grants permissions if found
```

**Example Use Cases:**
- Team members with corporate credentials
- External auditors using company identity
- Contractors with SSO access
- Tracking "who should have access" without kubeconfig files

---

## Corporate Identity Flow

### Architecture

```
john.doe@company.com (or any corporate identity)
    ↓
Attempts kubectl command
    ↓
Kubernetes redirects to OIDC/SSO provider
    ↓
User logs in to Azure AD / Okta / LDAP / Google Workspace
    ↓
Provider: "Yes, that's john.doe"
    ↓
Kubernetes: "Let me check what john.doe can do..."
    ↓
Finds RoleBinding for "john.doe" (created by helmx)
    ↓
Grants permissions (read-only, developer, namespace-admin)
    ↓
User can run kubectl commands
```

### How Credentials Work

**ServiceAccount Flow:**
- Credentials are certificate files stored in kubeconfig
- File contains: cert, key, cluster info
- User runs: `kubectl --kubeconfig=file.yaml get pods`

**Corporate Identity Flow:**
- No static credentials
- User's identity verified by corporate SSO
- User runs: `kubectl get pods` (credentials come from browser/SSO login)

---

## How helmx Fits In

### Two-Part System

#### **Part 1: Cluster Admin (One-Time Setup)**
This is **NOT** handled by helmx - done by cluster administrator:

```bash
# Cluster admin configures OIDC/Azure AD at cluster level
# Example: Azure AD Integration
apiVersion: v1
kind: Config
clusters:
- cluster:
    server: https://your-cluster:6443
    certificate-authority: /path/to/ca.crt
  name: your-cluster
users:
- name: azure-user
  user:
    exec:
      apiVersion: client.authentication.k8s.io/v1beta1
      command: kubelogin
      args:
      - get-token
      - --server-id=your-server-id
      - --client-id=your-client-id
```

Once this is set up, Kubernetes trusts the corporate SSO provider.

#### **Part 2: DevOps/Security Admin (Via helmx - Ongoing)**
This is what you do with helmx:

```bash
# In helmx RBAC view (Press 5):
1. Press a → Add New User
2. For ServiceAccount:
   - Type: ServiceAccount
   - Namespace: default
   - Target NS: test-app
   - Permission: developer
   - Export kubeconfig later (Press K)

3. For Manual User (corporate identity):
   - Type: User
   - Name: john.doe@company.com
   - Target NS: test-app
   - Permission: developer
   - NO kubeconfig export (they use corporate creds)
```

### helmx's Responsibilities

✅ Create permission bindings (RoleBindings, Roles)
✅ Assign permission levels (read-only, developer, namespace-admin)
✅ Export kubeconfigs for ServiceAccounts
✅ Track user access across namespaces
✅ Delete users and clean up resources

❌ Set up identity providers (OIDC, Azure AD, etc.)
❌ Manage corporate authentication
❌ Handle SSO configuration

---

## Practical Workflows

### Workflow 1: Automation/CI-CD (ServiceAccount)

**Scenario:** GitHub Actions needs to deploy releases

```bash
# In helmx RBAC view:
1. Press a → Add New User
2. Enter name: github-actions
3. Type: ServiceAccount
4. Namespace: kube-ci
5. Target NS: production
6. Permission: developer
7. Press Enter

# Export kubeconfig:
1. Navigate to github-actions
2. Press K (Shift+k)
3. File saved: ~/helmx-kubeconfig-github-actions.yaml

# In GitHub repo (secrets):
1. Add secret: KUBECONFIG_PROD
2. Value: contents of ~/helmx-kubeconfig-github-actions.yaml

# In GitHub workflow:
name: Deploy to Production
jobs:
  deploy:
    env:
      KUBECONFIG: /tmp/kubeconfig
    steps:
      - name: Setup kubeconfig
        run: echo "${{ secrets.KUBECONFIG_PROD }}" > /tmp/kubeconfig
      - name: Deploy with Helm
        run: helm upgrade --kubeconfig=/tmp/kubeconfig my-app ./chart
```

---

### Workflow 2: Team Access (Manual User + Corporate Identity)

**Scenario:** John from DevOps team needs access

```bash
# Prerequisite: Cluster admin has configured Azure AD OIDC

# In helmx RBAC view:
1. Press a → Add New User
2. Enter name: john.doe
3. Type: User (Manual)
4. Target NS: test-app
5. Permission: developer
6. Press Enter

# John at his desk:
$ kubectl get pods -n test-app
# Redirects to Azure AD login
# John logs in with: john.doe@company.com + password
# Success! Can now see pods in test-app

# helmx verified he should have developer access
# Azure AD verified he's really john.doe
```

---

### Workflow 3: Read-Only Access (Auditor)

**Scenario:** External auditor needs to view logs and metrics (no modifications)

```bash
# In helmx RBAC view:
1. Press a → Add New User
2. Enter name: auditor@external-company.com
3. Type: User (Manual)
4. Target NS: production (or "*" for cluster-wide)
5. Permission: read-only
6. Press Enter

# Auditor with corporate identity:
$ kubectl get pods -n production          # Works ✓
$ kubectl logs pod/my-app -n production   # Works ✓
$ kubectl delete pod/my-app -n production # Fails ✗ (read-only)
```

---

### Workflow 4: Namespace Admin (Team Lead)

**Scenario:** Platform team lead needs full control of one namespace

```bash
# In helmx RBAC view:
1. Press a → Add New User
2. Enter name: alice
3. Type: User (Manual)
4. Target NS: platform
5. Permission: namespace-admin
6. Press Enter

# Alice can:
$ kubectl apply -f deployment.yaml -n platform    # ✓
$ kubectl delete deployment my-app -n platform    # ✓
$ kubectl auth can-i '*' '*' -n platform         # ✓ (all verbs)

# Alice cannot:
$ kubectl delete pod -n other-namespace           # ✗ (scoped to platform only)
$ kubectl create clusterrole my-role              # ✗ (namespace-scoped only)
```

---

## Exporting Kubeconfigs

### When You Can Export

✅ **ServiceAccounts only**
- Kubernetes auto-generates certificate and key
- helmx can extract them into kubeconfig format
- File contains all auth info needed

❌ **Manual Users cannot export**
- No auto-generated credentials
- They use corporate SSO credentials instead
- No file to extract/export

### How to Export (ServiceAccount)

**In helmx RBAC view:**
```
1. Navigate to ServiceAccount user with j/k
2. Press K (Shift+k)
3. "Export Kubeconfig" dialog opens
4. Verify file path (default: ~/helmx-kubeconfig-<username>.yaml)
5. Press Enter to export
```

**File location:**
```bash
~/helmx-kubeconfig-<username>.yaml
# Example: ~/helmx-kubeconfig-dev-user-1.yaml
```

### Verify the Kubeconfig Works

```bash
# Test read-only ServiceAccount:
kubectl --kubeconfig=~/helmx-kubeconfig-dev-user-1.yaml get pods -n test-app
# Output: List of pods ✓

# Test permission boundary:
kubectl --kubeconfig=~/helmx-kubeconfig-dev-user-1.yaml delete pod <name> -n test-app
# Output: Depends on permission level (developer can delete, read-only cannot)

# Test namespace boundary:
kubectl --kubeconfig=~/helmx-kubeconfig-dev-user-1.yaml get pods -n kube-system
# Output: Error (not authorized for kube-system, only test-app)
```

### Contents of Exported Kubeconfig

```yaml
# Your exported kubeconfig contains:

apiVersion: v1
kind: Config
clusters:
  - name: your-cluster
    cluster:
      server: https://127.0.0.1:xxxxx
      certificate-authority-data: LS0tLS1...  # CA cert
contexts:
  - name: your-cluster
    context:
      cluster: your-cluster
      user: dev-user-1
      namespace: test-app
current-context: your-cluster
users:
  - name: dev-user-1
    user:
      client-certificate-data: LS0tLS1...  # ServiceAccount cert
      client-key-data: LS0tLS1...          # ServiceAccount key
```

### Using the Kubeconfig

```bash
# Option 1: Explicit flag
kubectl --kubeconfig=~/helmx-kubeconfig-dev-user-1.yaml get pods

# Option 2: Environment variable
export KUBECONFIG=~/helmx-kubeconfig-dev-user-1.yaml
kubectl get pods  # Uses exported kubeconfig

# Option 3: Merge with default kubeconfig
cp ~/helmx-kubeconfig-dev-user-1.yaml ~/.kube/dev-user-1.yaml
# Then use with --context flag or set in kubeconfig
```

---

## Permission Levels Explained

### read-only
```yaml
verbs: ["get", "list", "watch"]
resources:
  - pods
  - services
  - deployments
  - configmaps
  - secrets (viewing only)
```
**Can:** View everything, read logs
**Cannot:** Create, modify, delete anything

### developer
```yaml
verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
scope: namespace only
resources: Most Kubernetes resources in the namespace
```
**Can:** Deploy, update, delete resources in namespace
**Cannot:** Access other namespaces, create ClusterRoles, modify RBAC

### namespace-admin
```yaml
verbs: ["*"]  # All verbs
scope: namespace only
resources: ["*"]  # All resource types in namespace
```
**Can:** Full control of namespace
**Cannot:** Delete namespace itself, create cluster-wide resources

---

## Common Questions

### Q: Can I add corporate user without OIDC setup?
**A:** No. helmx creates the RoleBinding, but Kubernetes needs to know HOW to authenticate the user. Without OIDC/corporate identity setup at cluster level, Kubernetes won't recognize the user.

### Q: What if I want both ServiceAccount AND Manual User for one person?
**A:** Not recommended. Pick one:
- ServiceAccount: If they're automation/bot
- Manual User: If they're a real person with corporate SSO

### Q: Can I export kubeconfig for Manual Users?
**A:** No. Manual users don't have auto-generated credentials in Kubernetes. They authenticate via corporate SSO.

### Q: What happens if corporate identity user has no RoleBinding?
**A:** They can authenticate successfully but get "Forbidden" errors (no permissions). helmx must create the RoleBinding first.

### Q: Can I change permission level after creating user?
**A:** Yes! In helmx RBAC view, press `e` to edit, then change permission level.

### Q: How do I revoke access?
**A:** Press `d` to delete user. helmx removes all RoleBindings and ServiceAccount resources.

---

## Security Best Practices

1. **Use namespace admin sparingly** - Prefer developer level
2. **Audit access regularly** - Press `r` to refresh and review users
3. **Use read-only for external users** - Auditors, contractors
4. **Prefix test users** - Use `test-` prefix to identify disposable accounts
5. **Clean up unused accounts** - Delete unused users to reduce attack surface
6. **Never share kubeconfigs** - Each user/service should have their own
7. **Rotate credentials periodically** - Re-export kubeconfigs if compromised

---

## Related Documentation

- [CLAUDE.md - RBAC Management Section](../CLAUDE.md#rbac-management)
- [HELMX_TEST_PLAN.md - RBAC Tests](../HELMX_TEST_PLAN.md#8-rbac-management-view-tab-5)
- [Kubernetes RBAC Documentation](https://kubernetes.io/docs/reference/access-authn-authz/rbac/)
- [OIDC Authentication in Kubernetes](https://kubernetes.io/docs/reference/access-authn-authz/authentication/#openid-connect-tokens)


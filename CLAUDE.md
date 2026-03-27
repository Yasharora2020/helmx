# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build & Development Commands

```bash
# Build
make build              # Build to bin/helmx
go build -o bin/helmx ./cmd/helmx

# Run
make run                # Run without building
./bin/helmx             # Run built binary

# Test
make test               # Run all tests with race detection
go test -v ./internal/helm/...  # Run tests for specific package

# Lint
make lint               # Run golangci-lint
make lint-fix           # Auto-fix lint issues

# Format
make fmt                # Format code
make fmt-check          # Check formatting (CI)

# Full CI check
make check              # fmt-check + vet + lint + test
```

## Architecture Overview

**helmx** is a k9s-style terminal UI for Helm built with Go and the Bubble Tea framework.

### Core Stack
- **Bubble Tea** (Elm architecture): Model → Update → View loop
- **Helm SDK v3**: Direct API calls, no shell commands
- **Lipgloss**: Terminal styling
- **ORAS**: OCI registry support for charts

### Package Structure

```
cmd/helmx/main.go         # Entry point, creates tea.Program with App model

internal/
├── config/              # Configuration management
│   └── config.go        # Load/Save config, chart registries, theme settings
│
├── helm/                # Helm SDK wrapper (no TUI logic)
│   ├── client.go        # Core: ListReleases, Install, Upgrade, Rollback, GetManifest, GetReleasePodStatus, DryRunUpgrade
│   ├── chart.go         # LoadChart, GetChartInfo, GetChartValues, GetChartReadme, SearchCharts
│   ├── artifacthub.go   # SearchArtifactHub, GetArtifactHubReadme (public chart registry API)
│   ├── repo.go          # ListRepos, AddRepo, RemoveRepo, UpdateRepo
│   ├── registry.go      # OCI registry support (oras-go)
│   ├── context.go       # Kubeconfig context management: ListContexts, SwitchContext, ValidateContext
│   ├── rbac.go          # RBAC management: ListManagedUsers, GrantAccess, RevokeAccess, GenerateKubeconfig
│   ├── secrets.go       # SecretsClient: Decrypt, Encrypt, View (helm-secrets plugin wrapper)
│   └── secrets_detect.go # SOPS encryption detection: IsSOPSEncrypted, DetectEncryptionType
│
└── tui/                 # Bubble Tea UI components
    ├── app.go           # Root model, mode switching, global keybindings
    ├── styles.go        # Themes, icons, layout helpers (S = global styles)
    ├── release_detail.go # 4-pane detail view with pod status, diff preview, upgrade/rollback
    ├── explore.go       # Chart search/install with preview, README viewer, template to file
    ├── repos.go         # Repository management
    ├── settings.go      # Settings view (registries, theme, editor, context switcher)
    ├── rbac.go          # RBAC management view (user list, add/edit/delete users, kubeconfig export)
    ├── diff.go          # Diff computation (LCS algorithm) and rendering
    ├── values_editor.go # YAML editor with live validation
    ├── help.go          # Help overlay
    └── spinner.go       # Loading indicator
```

### Key Patterns

**Mode-based navigation**: App.mode determines which view renders. Modes: `ModeReleases`, `ModeReleaseDetail`, `ModeExplore`, `ModeRepos`, `ModeSettings`, `ModeRBAC`

**Message pattern**: Async operations return typed messages (e.g., `releasesLoadedMsg`, `historyLoadedMsg`) that Update() handlers process

**Viewport scrolling**: Long content (values, resources, history) uses `bubbles/viewport` for scrolling

**Pane focus**: Multi-pane views track `activePane` enum to route keypresses to correct component

### TUI Styling & Themes

Global styles in `styles.go`:
- `S` = global Styles instance with pre-configured lipgloss styles
- `DefaultTheme` = current active theme color palette
- `Themes` = map of available themes (default, dracula, nord, catppuccin, gruvbox, tokyo-night)
- `SetTheme(name)` = switch theme and update global styles
- `Icons` = unicode icons for visual elements
- Helper functions: `TabBarView()`, `KeyBar()`, `RenderLogo()`, `StatusBarView()`

### Configuration

Config file: `~/.config/helmx/config.yaml`
```go
cfg, _ := config.Load()           // Load or create default config
cfg.Theme = "dracula"             // Set theme
cfg.ChartRegistries               // Multiple chart registry URLs
cfg.DefaultNamespace              // Default namespace filter
cfg.Editor                        // Preferred editor ($EDITOR fallback)
cfg.Save()                        // Persist to file
```

### Helm Client Usage

```go
client, _ := helm.NewClient("")  // Uses default kubeconfig
releases, _ := client.ListReleases("")  // All namespaces
client.Install(chartRef, name, ns, values, createNS)
client.Upgrade(chartRef, name, ns, values)
client.Rollback(name, ns, revision)
```

### Chart Exploration & Install

The Explore view (`explore.go`) provides chart discovery and installation:

**Chart Search**: Searches both local repos and Artifact Hub (`artifacthub.go`)
```go
results, _ := client.SearchRepos("nginx")           // Local repos
hubResults, _ := helm.SearchArtifactHub("nginx", 10) // Public Artifact Hub
```

**Install Dialog**: Simple centered popup with external editor for values
- Press `i` on loaded chart to open install dialog
- Press `e` to edit values in `$EDITOR` (vim fallback) - kubectl edit style
- Uses `tea.ExecProcess` to suspend TUI and run external editor
- Returns with YAML validation on save

**External Editor Pattern** (like kubectl edit):
```go
// Write values to temp file, open $EDITOR, read back on exit
cmd := exec.Command(editor, tmpPath)
return tea.ExecProcess(cmd, func(err error) tea.Msg {
    data, _ := os.ReadFile(tmpPath)
    return valuesEditedMsg{content: string(data)}
})
```

### Diff Preview System

The diff system (`diff.go`) uses LCS (Longest Common Subsequence) algorithm:

```go
// Compute diff between old and new content
diff := ComputeDiff(oldContent, newContent)

// Render with context lines (3 lines around changes)
rendered := RenderDiff(diff, 3)

// Get summary like "+5 -3"
summary := DiffSummary(diff)
```

**Upgrade Flow with Diff**:
1. User presses `u` → opens external editor
2. On save, `upgradeValuesEditedMsg` triggers
3. Values diff computed immediately: `ComputeDiff(original, new)`
4. Manifest diff computed via dry-run: `client.DryRunUpgrade()`
5. Both diffs shown in tabbed preview dialog
6. User confirms → actual upgrade executed

### Pod Status Display

Pod status is fetched from Kubernetes API using label selector:
```go
// Get pod status for a release (uses app.kubernetes.io/instance label)
pods, _ := client.GetReleasePodStatus(releaseName, namespace)

// Each pod has: Name, Status, Ready ("1/1"), Restarts, ContainerStatus
// Status values: Running, Pending, CrashLoopBackOff, ImagePullBackOff, Terminating, etc.
```

Pod status is displayed in the Resources pane and auto-refreshes after upgrade/rollback.

### Context Management

The context system (`context.go`) manages kubeconfig contexts:

```go
type KubeContext struct {
    Name      string
    Cluster   string
    User      string
    Namespace string
    IsCurrent bool
}

// List all available contexts
contexts, _ := client.ListContexts()

// Switch to a different context
client.SwitchContext("production")

// Get details about a specific context
info, _ := client.GetContextInfo("staging")

// Validate a context before switching
err := client.ValidateContext("production")
```

Press `c` in Settings view to open the context switcher dialog. Switching context refreshes the Releases view automatically.

### Template to File (GitOps)

The template feature renders charts to YAML files without installing:

```go
// Template a chart to a file (uses helm template under the hood)
// Output can be used for GitOps tools like ArgoCD or Flux
```

Press `t` in Explore view (with a chart loaded) to open the template dialog:
- Enter output path (default: `./<chart>-rendered.yaml`)
- Press `e` to edit values in external editor
- Tab to Render → Enter to generate YAML file

The rendered YAML can be committed to Git for ArgoCD/Flux to sync.

### Helm Secrets Integration

The secrets system integrates with the helm-secrets plugin for SOPS encryption:

```go
// Check if helm-secrets plugin is available
if client.HasSecretsSupport() {
    secretsClient := client.GetSecretsClient()

    // Decrypt SOPS-encrypted content
    decrypted, err := secretsClient.Decrypt(encryptedYAML)

    // Encrypt plain YAML
    encrypted, err := secretsClient.Encrypt(plainYAML)

    // Check if content is encrypted
    isEncrypted := helm.IsSOPSEncrypted(content)

    // Detect encryption type (pgp, age, kms, etc.)
    encType := helm.DetectEncryptionType(content)
}
```

**Features:**
- 🔒 Lock icon displayed when values are encrypted
- Automatic detection of SOPS-encrypted content
- Press `D` in install dialog to decrypt values
- Re-encryption on save when editing encrypted values
- Shows helm-secrets plugin status in Settings view

**Requirements:**
- helm-secrets plugin: `helm plugin install https://github.com/jkroepke/helm-secrets`
- SOPS configuration (`.sops.yaml`) for encryption keys
- Encryption keys available (GPG, age, AWS KMS, etc.)

### RBAC Management

The RBAC view (`rbac.go`) provides simplified Kubernetes access control with a user-centric model:

```go
// Core types
type ManagedUser struct {
    Name        string            // Username or SA name
    Kind        string            // "ServiceAccount" or "User"
    Namespace   string            // SA namespace (empty for User)
    Permissions []NamespaceAccess // Aggregated from RoleBindings
}

type NamespaceAccess struct {
    Namespace   string // Target namespace ("*" for cluster-wide)
    Permission  string // "read-only", "developer", "namespace-admin", "custom"
    RoleName    string // Actual K8s Role/ClusterRole name
    BindingName string // RoleBinding that grants this
}

// Key functions
users, _ := client.ListManagedUsers()                    // Aggregates from ServiceAccounts + RoleBindings
client.GrantAccess(user, namespace, permission)          // Creates SA + Role + RoleBinding
client.RevokeAccess(user, namespace)                     // Deletes RoleBinding
kubeconfig, _ := client.GenerateKubeconfig(user)         // Export kubeconfig for SA
client.DeleteUser(user)                                  // Remove user and all access
```

**Permission Presets:**
| Preset | Description | K8s Resources |
|--------|-------------|---------------|
| `read-only` | View pods, services, deployments, configmaps | get, list, watch |
| `developer` | Create/edit deployments, services, exec into pods, view logs | get, list, watch, create, update, patch, delete |
| `namespace-admin` | Full control within namespace | * on * in namespace |

**Keyboard shortcuts (RBAC view):**
- `a` - Add new user
- `e` / `Enter` - Edit selected user
- `+` - Add namespace access to selected user
- `d` - Delete user
- `K` - Export kubeconfig (ServiceAccount only)
- `r` - Refresh user list

**helmx- prefix for managed resources:** Resources created by helmx are prefixed with `helmx-` (e.g., `helmx-read-only`, `helmx-developer`) so we only modify roles/bindings we created.

## Key Files for Common Tasks

| Task | Files |
|------|-------|
| Add keyboard shortcut | `internal/tui/app.go` (keyMap, Update) |
| Change colors/styling | `internal/tui/styles.go` (Themes, SetTheme) |
| Add new theme | `internal/tui/styles.go` (add to Themes map and ThemeNames) |
| Add Helm operation | `internal/helm/client.go` |
| New UI view | Create in `internal/tui/`, add Mode in `app.go` |
| Release detail panes | `internal/tui/release_detail.go` |
| Diff preview | `internal/tui/diff.go`, `internal/tui/release_detail.go` |
| Pod status | `internal/helm/client.go` (GetReleasePodStatus), `internal/tui/release_detail.go` |
| Chart README | `internal/helm/chart.go`, `internal/helm/artifacthub.go`, `internal/tui/explore.go` |
| Settings/config | `internal/tui/settings.go`, `internal/config/config.go` |
| Chart registries | `internal/config/config.go` (ChartRegistries) |
| Context switcher | `internal/helm/context.go`, `internal/tui/settings.go` |
| Template to file | `internal/tui/explore.go` (template dialog) |
| Helm secrets | `internal/helm/secrets.go`, `internal/helm/secrets_detect.go` |
| RBAC management | `internal/helm/rbac.go`, `internal/tui/rbac.go` |
| Help documentation | `internal/tui/help.go` |

## Testing

```bash
# Run all tests
go test -v ./internal/helm/... ./internal/tui/...

# Run with race detection
go test -race ./...

# Run specific test
go test -v -run TestComputeDiff ./internal/tui/...
```

Test files:
- `internal/helm/client_test.go` - Pod status, formatting utilities
- `internal/helm/chart_test.go` - Resource extraction, case-insensitive search
- `internal/helm/artifacthub_test.go` - HTTP mock tests for Artifact Hub API
- `internal/helm/context_test.go` - Context management with temp kubeconfig
- `internal/helm/secrets_test.go` - SOPS encryption detection, SecretsClient
- `internal/helm/rbac_test.go` - Permission level detection, preset creation, formatting
- `internal/tui/diff_test.go` - LCS algorithm, diff computation
- `internal/tui/explore_test.go` - Template dialog, install field cycling
- `internal/tui/rbac_test.go` - RBAC view dialogs, navigation, rendering

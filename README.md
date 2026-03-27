# helmx

A k9s-style terminal UI for Helm. Manage releases, explore and install charts, handle rollbacks, and inspect deployments — all from the keyboard without memorizing flags.

![CI](https://github.com/yasharora2020/helmx/actions/workflows/ci.yml/badge.svg)

> **Alpha** — This project is in active development. Features may change, break, or be removed. Bug reports are welcome via [GitHub Issues](https://github.com/yasharora2020/helmx/issues).

> Built with Go 1.24, [Bubble Tea](https://github.com/charmbracelet/bubbletea), [Lipgloss](https://github.com/charmbracelet/lipgloss), and the [Helm SDK v3](https://helm.sh/docs/topics/advanced/) — entirely through AI pair programming with [Claude Code](https://docs.anthropic.com/en/docs/claude-code/overview).

## Why

Helm's CLI is powerful but verbose. Commands like `helm upgrade --install my-release bitnami/postgresql --namespace prod --create-namespace --set auth.password=secret` are hard to get right without copy-pasting from docs. Common tasks — checking what's deployed, diffing values before an upgrade, rolling back a bad revision — require multiple commands with flags that are easy to forget.

helmx wraps the Helm SDK directly (no shelling out) in a keyboard-driven TUI so those workflows become a few keystrokes.

```
┌─────────────────────────────────────────────────────────────────────────┐
│ ⎈ helmx  [1]RELEASES  [2]EXPLORE  [3]REPOS  [4]SETTINGS  [5]RBAC       │
├─────────────────────────────────────────────────────────────────────────┤
│  ⎈ Releases (default)                                                   │
│                                                                         │
│  ▸ nginx          default    nginx-15.0.2      ● deployed               │
│    postgresql     prod       postgresql-12.1   ● deployed               │
│    redis          cache      redis-17.3.1      ● deployed               │
│    my-app         staging    my-app-0.1.0      ✗ failed                 │
│                                                                         │
├─────────────────────────────────────────────────────────────────────────┤
│ ⏎ details │ ␣ select │ U bulk upgrade │ d delete │ n namespace │ r refresh │
└─────────────────────────────────────────────────────────────────────────┘
```

## Features

### Releases View

The home screen. Shows all releases across namespaces with live status.

- Filter by namespace with `n` — cycles All → namespace1 → namespace2 → All
- Filter by name with `/` (fuzzy search)
- Multi-select releases with `Space`, then `U` to bulk upgrade all selected to latest
- `c` clears the selection
- Delete a release with `d` (confirmation required)
- `r` refreshes the list

### Release Details

4-pane view: **Info** | **History** | **Values** | **Resources**. Tab cycles between panes.

- **Info pane**: chart version, namespace, last deployed, status
- **History pane**: full revision history; navigate with j/k, press `r` to rollback to the selected revision, press `c` to compare a selected revision's values against the current one
- **Values pane**: current user-supplied values in YAML tree format
- **Resources pane**: all K8s objects created by the release, plus live pod status (Running/Pending/CrashLoopBackOff/ImagePullBackOff with restart counts); press `p` to refresh

From any pane:
- `u` — upgrade: opens `$EDITOR` (falls back to vim), shows a diff preview of both values and rendered manifests before confirming
- `d` — open standalone diff preview without upgrading
- `l` — pod log viewer: select a pod, view streaming logs
- `x` — exec into a pod: select pod, select shell (/bin/bash, /bin/sh, etc.)
- `f` — port-forward manager: select pod/port, manages active forwards
- `R` — view the chart's README

### Explore View

Chart discovery and installation. Searches local repos and Artifact Hub simultaneously.

**Search**: type a chart name (or `repo/chart`, `oci://registry/chart`, or a local `./path`) and press Enter. Results from local repos and Artifact Hub appear in the list pane.

**Once a chart is loaded**:
- `v` — version selector: pick any published version
- `r` — README viewer with search (`/`), next/prev match (`n`/`N`), scroll (`j/k`), jump to top/bottom (`g`/`G`)
- `s` — Trivy security scan (requires Trivy installed)
- `D` — dependency tree
- `S` — values schema browser
- `t` — template to file: render the chart to a YAML manifest for GitOps (ArgoCD, Flux)
- `i` — install dialog
- `a` — add to multi-chart install queue; `M` opens the queue dialog

**Install dialog**:
- Fill release name, namespace, create-namespace toggle
- `e` — edit values in `$EDITOR` (kubectl-edit style)
- `f` — import values from an existing file
- `d` — dry-run preview: shows the full rendered manifest before installing
- `T` — save current values as a reusable install template
- `j/k` — select from saved install templates
- Enter on the Install button to deploy

**Multi-chart install (stacks)**:
- Add multiple charts to a queue with `a`
- Reorder with `J`/`K`, remove with `d`, edit per-chart values with `e`
- Toggle "wait for ready" per chart with `w`
- Install all in order with Enter

### Repos View

- List all configured Helm repositories
- `a` — add a new repository (name + URL, with optional auth)
- `d` — delete repository
- `u` — update selected repo's index
- `U` — update all repos

Supports standard HTTP repos and OCI registries (`oci://`). Authentication uses `~/.docker/config.json` or `helm registry login`.

### Settings

- `t` — cycle through 6 built-in themes: default, dracula, nord, catppuccin, gruvbox, tokyo-night
- `c` — context switcher: list all kubeconfig contexts, switch without leaving the TUI; triggers automatic release list refresh
- `s` — toggle security scanning (Trivy integration)
- `a` / `d` — add or remove additional Artifact Hub registry endpoints
- `T` — manage saved install templates
- `e` — edit raw config file
- `r` — reset settings to defaults

Config is saved to `~/.config/helmx/config.yaml`.

### RBAC Management

Simplified Kubernetes access control with a user-centric model. helmx manages its own resources prefixed with `helmx-` so it never touches roles/bindings created by other tools.

- `a` — add a new service account or user
- `e` / Enter — edit selected user's access
- `+` — grant namespace access to a user (read-only, developer, or namespace-admin presets)
- `d` — delete user and all associated bindings
- `K` — export a kubeconfig for the selected service account
- `r` — refresh user list

Permission presets:
| Preset | Access |
|--------|--------|
| `read-only` | get, list, watch on pods/services/deployments/configmaps |
| `developer` | create/update/delete deployments and services, exec into pods, view logs |
| `namespace-admin` | full control within namespace |

## Installation

### From Source

```bash
git clone https://github.com/yasharora2020/helmx.git
cd helmx
make build        # builds to bin/helmx
./bin/helmx
```

### Go Install

```bash
go install github.com/yasharora2020/helmx/cmd/helmx@latest
```

### Pre-built Binaries

Download from [GitHub Releases](https://github.com/yasharora2020/helmx/releases):

```bash
# Linux AMD64
curl -LO https://github.com/yasharora2020/helmx/releases/latest/download/helmx_Linux_x86_64.tar.gz
tar xzf helmx_Linux_x86_64.tar.gz
sudo mv helmx /usr/local/bin/

# macOS ARM64 (Apple Silicon)
curl -LO https://github.com/yasharora2020/helmx/releases/latest/download/helmx_Darwin_arm64.tar.gz
tar xzf helmx_Darwin_arm64.tar.gz
sudo mv helmx /usr/local/bin/

# macOS AMD64 (Intel)
curl -LO https://github.com/yasharora2020/helmx/releases/latest/download/helmx_Darwin_x86_64.tar.gz
tar xzf helmx_Darwin_x86_64.tar.gz
sudo mv helmx /usr/local/bin/
```

## Requirements

**Required**:
- Go 1.24+ (build from source only)
- A valid kubeconfig with cluster access
- Helm 3 repos configured (or add them via the TUI)

**Optional**:
- [Trivy](https://trivy.dev/) — security scanning in Explore view
- metrics-server — CPU/memory display in pod status

## Usage

```bash
helmx
# Uses current kubeconfig context — same as: kubectl config current-context
```

Press `?` at any time to open the full keyboard shortcut overlay. Press `0` or `w` to open the welcome/about page.

## Configuration

Config file: `~/.config/helmx/config.yaml`

```yaml
theme: dracula                          # default, dracula, nord, catppuccin, gruvbox, tokyo-night
defaultNamespace: ""                    # pre-filter namespace on startup (empty = all)
editor: ""                              # preferred editor ($EDITOR env var is the fallback, then vim)
showWelcomeOnStart: false               # show the welcome page on startup
chartRegistries:                        # additional Artifact Hub endpoints
  - https://artifacthub.io
securityScanEnabled: true               # enable Trivy integration
```

## Key Bindings

Press `?` for the full in-app overlay. Quick reference:

### Global
| Key | Action |
|-----|--------|
| `0` / `w` | Welcome / About page |
| `1` | Releases view |
| `2` | Explore view |
| `3` | Repos view |
| `4` | Settings view |
| `5` | RBAC view |
| `?` | Toggle help overlay |
| `q` | Quit / Back |
| `Esc` | Back / Cancel |

### Releases View
| Key | Action |
|-----|--------|
| `Enter` | Open release details |
| `Space` | Toggle select (multi-select) |
| `U` | Bulk upgrade selected releases |
| `c` | Clear all selections |
| `d` | Delete release |
| `n` | Cycle namespaces |
| `r` | Refresh list |
| `/` | Filter releases |
| `j/k` | Navigate |

### Release Details
| Key | Action |
|-----|--------|
| `Tab` | Cycle: Info → History → Values → Resources |
| `u` | Upgrade (opens editor, then diff preview) |
| `d` | Diff preview (values + manifest) |
| `r` | Rollback selected revision |
| `c` | Compare selected revision with current |
| `R` | View chart README |
| `l` | View pod logs |
| `x` | Exec into pod (shell) |
| `f` | Port-forward manager |
| `p` | Refresh pod status |

### Explore View
| Key | Action |
|-----|--------|
| `Enter` | Search / Load chart |
| `Tab` | Switch panes |
| `i` | Install chart |
| `a` | Add to multi-chart queue |
| `M` | Open multi-chart install dialog |
| `v` | Select chart version |
| `s` | Security scan (Trivy) |
| `S` | Browse values schema |
| `D` | View dependency tree |
| `r` | View README |
| `t` | Template to file (GitOps) |
| `/` / `Esc` | Return to search input |

### Install Dialog
| Key | Action |
|-----|--------|
| `Tab` | Next field |
| `e` | Edit values in `$EDITOR` |
| `f` | Import values from file |
| `d` | Dry-run preview |
| `T` | Save values as install template |
| `j/k` | Select saved template |
| `Enter` | Confirm / Install |

### Multi-Chart Install
| Key | Action |
|-----|--------|
| `j/k` | Navigate queue |
| `J/K` | Reorder charts |
| `d` | Remove from queue |
| `e` | Edit chart values |
| `w` | Toggle wait-for-ready |
| `c` | Clear queue |

### Repos View
| Key | Action |
|-----|--------|
| `a` | Add repository |
| `d` | Delete repository |
| `u` | Update selected repo |
| `U` | Update all repos |

### Settings
| Key | Action |
|-----|--------|
| `t` | Change theme |
| `c` | Switch kubectl context |
| `s` | Toggle security scanning |
| `a` / `d` | Add / delete registry |
| `T` | Manage install templates |
| `e` | Edit settings |
| `r` | Reset to defaults |

### RBAC View
| Key | Action |
|-----|--------|
| `a` | Add user |
| `e` / Enter | Edit selected user |
| `+` | Add namespace access |
| `d` | Delete user |
| `K` | Export kubeconfig (ServiceAccount only) |
| `r` | Refresh |

## Development

```bash
git clone https://github.com/yasharora2020/helmx.git
cd helmx
make build        # Build to bin/helmx
make test         # Run tests with race detection
make lint         # Run golangci-lint
make check        # fmt + vet + lint + test
```

Test files cover: Helm client operations, chart loading, Artifact Hub API, context management, RBAC permission logic, diff algorithm, explore view dialogs, and RBAC view rendering.

## Built With

- **Go 1.24** — single static binary, no runtime dependencies
- **[Bubble Tea](https://github.com/charmbracelet/bubbletea)** — TUI framework (Elm architecture)
- **Helm SDK v3** — direct API calls, no subprocess shelling
- **[Lip Gloss](https://github.com/charmbracelet/lipgloss)** — terminal styling
- **ORAS** — OCI registry support
- **[Claude Code](https://docs.anthropic.com/en/docs/claude-code/overview)** — AI pair programming throughout development

## License

Apache-2.0

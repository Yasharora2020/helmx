package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/yasharora2020/helmx/internal/helm"
	"github.com/yasharora2020/helmx/internal/tui/dialog"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ReleaseDetailPane represents which pane is focused
type ReleaseDetailPane int

const (
	DetailPaneInfo ReleaseDetailPane = iota
	DetailPaneHistory
	DetailPaneValues
	DetailPaneResources
)

// LogDialogStep represents the current step in the log viewer flow
type LogDialogStep int

const (
	LogStepSelectPod LogDialogStep = iota
	LogStepViewLogs
)

// ExecDialogStep represents the current step in the exec/shell flow
type ExecDialogStep int

const (
	ExecStepSelectPod ExecDialogStep = iota
	ExecStepSelectShell
)

// PortForwardDialogStep represents the current step in the port forward flow
type PortForwardDialogStep int

const (
	PFStepSelectPod PortForwardDialogStep = iota
	PFStepSelectPort
	PFStepManage
)

// supportedShells is the list of shells to try when exec'ing into a pod
var supportedShells = []string{"/bin/bash", "/bin/sh", "/bin/zsh", "/bin/ash"}

// ReleaseDetail shows detailed info about a release with history.
//
// This is a complex multi-pane view with 4 main panes (Info, History, Values, Resources)
// plus several modal dialogs (rollback, upgrade, diff preview, logs, exec, port-forward).
//
// # Pane Navigation
//
// Tab/Shift+Tab cycles through the 4 main panes. Each pane has its own viewport
// for scrolling content. The activePane field determines which pane receives input.
//
// # Async Operations
//
// Most data loading uses the tea.Cmd pattern for async operations:
//
//  1. SetRelease() triggers parallel fetches: history, values, resources, manifest, pod status
//  2. Each fetch returns a typed message (historyLoadedMsg, valuesLoadedMsg, etc.)
//  3. Update() handles these messages and updates state
//
// Loading states are tracked per-operation (e.g., loadingDiff, logLoading, pfLoading)
// to show appropriate spinners without blocking the UI.
//
// # Dialog State Machine
//
// Dialogs are shown via boolean flags (showUpgradeConfirm, showLogDialog, etc.).
// Some dialogs have multi-step flows tracked by step enums:
//   - LogDialogStep: LogStepSelectPod → LogStepViewLogs
//   - ExecDialogStep: ExecStepSelectPod → ExecStepSelectShell
//   - PortForwardDialogStep: PFStepSelectPod → PFStepSelectPort → PFStepManage
//
// # Port Forwarding
//
// Port forwarding runs in background goroutines managed by PortForwardManager.
// Active forwards are tracked in pfActiveForwards and persist across dialog opens.
// The manager handles cleanup when the view is closed.
type ReleaseDetail struct {
	releaseManager   helm.ReleaseManager
	clusterInspector helm.ClusterInspector
	chartExplorer    helm.ChartExplorer
	width            int
	height           int

	// Current release
	release   *helm.Release
	resources []string         // Deployed resources
	podStatus []helm.PodStatus // Pod status for the release

	// History
	history       []HistoryEntry
	selectedRev   int
	historyScroll int

	// UI
	activePane    ReleaseDetailPane
	valuesView    viewport.Model
	resourcesView viewport.Model
	historyView   viewport.Model

	// Rollback dialog
	showRollbackConfirm bool
	rollbackTarget      int

	// Upgrade dialog
	showUpgradeConfirm   bool
	upgradeValuesContent string
	originalValues       string // Computed values (for display)
	userValues           string // User-supplied values only (for editing)
	upgrading            bool   // True while upgrade is in progress

	// Upgrade value sources (multiple files + --set overrides)
	upgradeValueFiles  []string
	upgradeSetValues   []string

	// Dialogs for upgrade value sources
	upgradeFileImportDialog   *dialog.FileImportDialog
	upgradeSetValueDialog     *dialog.SetValueDialog
	upgradeValueSourcesDialog *dialog.ValueSourcesDialog

	// Diff preview
	showDiffPreview bool
	valuesDiff      DiffResult
	manifestDiff    DiffResult
	currentManifest string
	previewManifest string
	diffActiveTab   int // 0 = values, 1 = manifest
	diffViewport    viewport.Model
	loadingDiff     bool

	// Secrets/encryption handling
	valuesEncrypted bool   // Whether release values are encrypted
	decryptionError string // Error message if decryption failed

	// Pod log viewer dialog
	showLogDialog     bool           // Show log dialog
	logDialogStep     LogDialogStep  // Current step: pod selection or log view
	logPodList        []helm.PodInfo // Pods available for selection
	logSelectedPodIdx int            // Currently selected pod index
	logSelectedPod    *helm.PodInfo  // Selected pod for viewing logs
	logContainerIdx   int            // Selected container index
	logContent        string         // Fetched log content
	logViewport       viewport.Model // Scrollable log viewport
	logLoading        bool           // Loading indicator
	logError          error          // Error from log fetch
	logShowTimestamps bool           // Toggle: show timestamps
	logShowPrevious   bool           // Toggle: show previous container logs

	// Pod exec (shell access) dialog
	showExecDialog     bool           // Show exec dialog
	execDialogStep     ExecDialogStep // Current step: pod selection or shell selection
	execPodList        []helm.PodInfo // Pods available for selection
	execSelectedPodIdx int            // Currently selected pod index
	execSelectedPod    *helm.PodInfo  // Selected pod for exec
	execContainerIdx   int            // Selected container index
	execShellIdx       int            // Selected shell index
	execLoading        bool           // Loading indicator
	execError          error          // Error from exec

	// Port forwarding dialog
	showPortForwardDialog bool                  // Show port forward dialog
	pfDialogStep          PortForwardDialogStep // Current step in port forward flow
	pfManager             *helm.PortForwardManager
	pfPodList             []helm.PodInfo // Pods available for port forward
	pfSelectedPodIdx      int            // Currently selected pod index
	pfPodPorts            []helm.PodPort // Ports from selected pod
	pfSelectedPortIdx     int            // Currently selected port index
	pfLocalPort           string         // Local port input
	pfLoading             bool           // Loading indicator
	pfError               error          // Error from port forward
	pfActiveForwards      []*helm.PortForward

	// Revision comparison dialog
	showRevisionCompare bool           // Show revision comparison dialog
	compareRev1         int            // First revision number
	compareRev2         int            // Second revision number (current is 0)
	compareValues1      string         // Values from first revision
	compareValues2      string         // Values from second revision
	compareManifest1    string         // Manifest from first revision
	compareManifest2    string         // Manifest from second revision
	compareValuesDiff   DiffResult     // Values diff result
	compareManifestDiff DiffResult     // Manifest diff result
	compareActiveTab    int            // 0 = values, 1 = manifest
	compareViewport     viewport.Model // Viewport for comparison
	compareLoading      bool           // Loading indicator
	compareError        error          // Error from comparison

	// README dialog
	showReadmeDialog bool           // Show README dialog
	readmeViewport   viewport.Model // Viewport for README content
	readmeContent    string         // README content
	readmeLoading    bool           // Loading indicator
	readmeError      error          // Error from README fetch

	// Search within README
	readmeSearchMode    bool   // Whether search input is active
	readmeSearchQuery   string // Current search query
	readmeSearchMatches []int  // Line numbers containing matches
	readmeSearchIdx     int    // Current match index
	readmeHighlighted   string // Content with highlighted matches

	err    error
	status string
}

// HistoryEntry represents a release revision
type HistoryEntry struct {
	Revision    int
	Updated     time.Time
	Status      string
	Chart       string
	AppVersion  string
	Description string
}

// NewReleaseDetail creates a new release detail view
func NewReleaseDetail(rm helm.ReleaseManager, ci helm.ClusterInspector, ce helm.ChartExplorer) *ReleaseDetail {
	vv := viewport.New(40, 15)
	rv := viewport.New(40, 15)
	hv := viewport.New(30, 10)
	dv := viewport.New(60, 20)
	lv := viewport.New(80, 20)     // Log viewport
	cv := viewport.New(100, 25)    // Comparison viewport
	readme := viewport.New(80, 20) // README viewport

	rd := &ReleaseDetail{
		releaseManager:    rm,
		clusterInspector:  ci,
		chartExplorer:     ce,
		valuesView:        vv,
		resourcesView:     rv,
		historyView:       hv,
		diffViewport:      dv,
		logViewport:       lv,
		compareViewport:   cv,
		readmeViewport:    readme,
		activePane:        DetailPaneInfo,
		logShowTimestamps: true, // Default timestamps on
	}

	// Initialize upgrade value source dialogs
	rd.upgradeFileImportDialog = dialog.NewFileImportDialog()
	rd.upgradeSetValueDialog = dialog.NewSetValueDialog()
	rd.upgradeValueSourcesDialog = dialog.NewValueSourcesDialog()

	// Set styles on upgrade dialogs
	rd.upgradeFileImportDialog.SetStyles(
		S.Title, S.Label, S.Muted, S.Error,
		DefaultTheme.BorderFocus, Icons.File, Icons.Cross,
	)
	rd.upgradeSetValueDialog.SetStyles(
		S.Title, S.Label, S.Muted, S.Error,
		S.Button, S.ButtonFocus, DefaultTheme.BorderFocus,
	)
	rd.upgradeValueSourcesDialog.SetStyles(
		S.Title, S.Label, S.Muted, S.Error, S.Highlighted,
		S.Button, S.ButtonFocus, DefaultTheme.BorderFocus,
		Icons.Check, Icons.Cross,
	)

	return rd
}

// SetRelease sets the release to display
func (r *ReleaseDetail) SetRelease(rel *helm.Release) tea.Cmd {
	r.release = rel
	r.selectedRev = 0
	r.historyScroll = 0
	r.showRollbackConfirm = false
	r.showUpgradeConfirm = false
	r.showDiffPreview = false
	r.upgradeValuesContent = ""
	r.upgradeValueFiles = nil
	r.upgradeSetValues = nil
	r.originalValues = ""
	r.userValues = ""
	r.upgrading = false
	r.currentManifest = ""
	r.previewManifest = ""
	r.loadingDiff = false
	r.diffActiveTab = 0
	r.err = nil
	r.status = ""
	r.resources = nil

	// Reset all dialog states
	r.showLogDialog = false
	r.logDialogStep = 0
	r.logPodList = nil
	r.logSelectedPodIdx = 0
	r.logSelectedPod = nil
	r.logContainerIdx = 0
	r.logContent = ""
	r.logLoading = false
	r.logError = nil

	r.showExecDialog = false
	r.execDialogStep = 0
	r.execPodList = nil
	r.execSelectedPodIdx = 0
	r.execSelectedPod = nil
	r.execContainerIdx = 0
	r.execShellIdx = 0
	r.execLoading = false
	r.execError = nil

	r.showPortForwardDialog = false
	r.pfDialogStep = 0
	r.pfPodList = nil
	r.pfSelectedPodIdx = 0
	r.pfSelectedPortIdx = 0
	r.pfLocalPort = ""
	r.pfLoading = false
	r.pfError = nil

	r.showRevisionCompare = false
	r.compareLoading = false
	r.compareError = nil

	r.showReadmeDialog = false
	r.readmeContent = ""
	r.readmeLoading = false
	r.readmeError = nil
	r.readmeSearchMode = false
	r.readmeSearchQuery = ""

	// Fetch history, values, resources, manifest, and pod status
	return tea.Batch(
		r.fetchHistory(),
		r.fetchValues(),
		r.fetchResources(),
		r.fetchCurrentManifest(),
		r.fetchPodStatus(),
	)
}

// SetSize updates the dimensions
func (r *ReleaseDetail) SetSize(width, height int) {
	r.width = width
	r.height = height

	// Account for nav bar (1 line) and status bar (1 line)
	contentHeight := height - 3
	// Use 0.25 ratio for left column (Info/History) - makes right column wider
	layout := CalculatePaneLayout(width, contentHeight, 0.25)

	// Values viewport (top right) - account for border and header
	r.valuesView.Width = layout.RightWidth - 4
	r.valuesView.Height = layout.TopHeight - 5

	// Resources viewport (bottom right)
	r.resourcesView.Width = layout.RightWidth - 4
	r.resourcesView.Height = layout.BottomHeight - 5

	// History viewport (bottom left)
	r.historyView.Width = layout.LeftWidth - 4
	r.historyView.Height = layout.BottomHeight - 5

	// Update upgrade dialog sizes
	if r.upgradeFileImportDialog != nil {
		r.upgradeFileImportDialog.SetSize(width, height)
	}
	if r.upgradeSetValueDialog != nil {
		r.upgradeSetValueDialog.SetSize(width, height)
	}
	if r.upgradeValueSourcesDialog != nil {
		r.upgradeValueSourcesDialog.SetSize(width, height)
	}
}

// HasOpenDialog returns true if any dialog is currently open.
// This consolidates the dialog state checks for external callers.
func (r *ReleaseDetail) HasOpenDialog() bool {
	return r.showUpgradeConfirm ||
		r.showRollbackConfirm ||
		r.showDiffPreview ||
		r.showLogDialog ||
		r.showExecDialog ||
		r.showRevisionCompare ||
		r.showPortForwardDialog ||
		r.showReadmeDialog ||
		(r.upgradeFileImportDialog != nil && r.upgradeFileImportDialog.IsOpen()) ||
		(r.upgradeSetValueDialog != nil && r.upgradeSetValueDialog.IsOpen()) ||
		(r.upgradeValueSourcesDialog != nil && r.upgradeValueSourcesDialog.IsOpen())
}

// Update handles input
func (r *ReleaseDetail) Update(msg tea.Msg) (*ReleaseDetail, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		r.SetSize(msg.Width, msg.Height)

	case tea.KeyMsg:
		// Route to upgrade value source dialogs if open
		if r.upgradeFileImportDialog != nil && r.upgradeFileImportDialog.IsOpen() {
			updated, cmd := r.upgradeFileImportDialog.Update(msg)
			r.upgradeFileImportDialog = updated.(*dialog.FileImportDialog)
			return r, cmd
		}
		if r.upgradeSetValueDialog != nil && r.upgradeSetValueDialog.IsOpen() {
			updated, cmd := r.upgradeSetValueDialog.Update(msg)
			r.upgradeSetValueDialog = updated.(*dialog.SetValueDialog)
			return r, cmd
		}
		if r.upgradeValueSourcesDialog != nil && r.upgradeValueSourcesDialog.IsOpen() {
			updated, cmd := r.upgradeValueSourcesDialog.Update(msg)
			r.upgradeValueSourcesDialog = updated.(*dialog.ValueSourcesDialog)
			return r, cmd
		}

		// Handle log dialog
		if r.showLogDialog {
			return r.updateLogDialog(msg)
		}

		// Handle exec dialog
		if r.showExecDialog {
			return r.updateExecDialog(msg)
		}

		// Handle port forward dialog
		if r.showPortForwardDialog {
			return r.updatePortForwardDialog(msg)
		}

		// Handle revision comparison dialog
		if r.showRevisionCompare {
			return r.updateRevisionCompare(msg)
		}

		// Handle README dialog
		if r.showReadmeDialog {
			return r.updateReadmeDialog(msg)
		}

		// Handle rollback confirmation
		if r.showRollbackConfirm {
			return r.updateRollbackConfirm(msg)
		}

		// Handle diff preview
		if r.showDiffPreview {
			return r.updateDiffPreview(msg)
		}

		// Handle upgrade confirmation
		if r.showUpgradeConfirm {
			return r.updateUpgradeConfirm(msg)
		}

		switch msg.String() {
		case "tab":
			r.activePane = (r.activePane + 1) % 4

		case "shift+tab":
			r.activePane = (r.activePane + 3) % 4

		case "up", "k":
			if r.activePane == DetailPaneHistory && r.selectedRev > 0 {
				r.selectedRev--
				r.historyView.SetContent(r.formatHistory())
			}

		case "down", "j":
			if r.activePane == DetailPaneHistory && r.selectedRev < len(r.history)-1 {
				r.selectedRev++
				r.historyView.SetContent(r.formatHistory())
			}

		case "r":
			// Open rollback dialog
			if len(r.history) > 1 && r.selectedRev > 0 {
				r.showRollbackConfirm = true
				r.rollbackTarget = r.history[r.selectedRev].Revision
			}

		case "u":
			// Open external editor for upgrade
			return r, r.openExternalEditor()

		case "d", "ctrl+d":
			// Show diff preview with current values
			if r.userValues != "" {
				r.showDiffPreview = true
				r.upgradeValuesContent = r.userValues
				r.valuesDiff = ComputeDiff(r.userValues, r.userValues)
				r.diffViewport.SetContent(S.Muted.Render("No value changes. Press 'e' to edit values."))
				r.diffViewport.GotoTop()
			}

		case "p":
			// Refresh pod status
			cmds = append(cmds, r.fetchPodStatus())

		case "l":
			// Open log viewer dialog
			r.showLogDialog = true
			r.logDialogStep = LogStepSelectPod
			r.logSelectedPodIdx = 0
			r.logContainerIdx = 0
			r.logSelectedPod = nil
			r.logContent = ""
			r.logError = nil
			r.logLoading = true
			cmds = append(cmds, r.fetchPodList())

		case "x":
			// Open exec (shell access) dialog
			r.showExecDialog = true
			r.execDialogStep = ExecStepSelectPod
			r.execSelectedPodIdx = 0
			r.execContainerIdx = 0
			r.execShellIdx = 0
			r.execSelectedPod = nil
			r.execError = nil
			r.execLoading = true
			cmds = append(cmds, r.fetchPodListForExec())

		case "f":
			// Open port forward dialog
			r.showPortForwardDialog = true
			r.pfDialogStep = PFStepManage // Start with manage view to show active forwards
			r.pfSelectedPodIdx = 0
			r.pfSelectedPortIdx = 0
			r.pfLocalPort = ""
			r.pfError = nil
			r.pfLoading = true
			// Initialize port forward manager if needed
			if r.pfManager == nil {
				r.pfManager = helm.NewPortForwardManager("")
			}
			// Refresh active forwards for this release
			r.pfActiveForwards = r.pfManager.ListPortForwardsByRelease(r.release.Name, r.release.Namespace)
			cmds = append(cmds, r.fetchPodListForPortForward())

		case "c":
			// Compare selected revision with current (only if in history pane and not current revision)
			if r.activePane == DetailPaneHistory && len(r.history) > 1 && r.selectedRev > 0 {
				r.showRevisionCompare = true
				r.compareRev1 = r.history[r.selectedRev].Revision // Selected (older)
				r.compareRev2 = r.history[0].Revision             // Current (latest)
				r.compareActiveTab = 0
				r.compareLoading = true
				r.compareError = nil
				cmds = append(cmds, r.fetchRevisionData())
			}

		case "R":
			// Open README dialog
			r.showReadmeDialog = true
			r.readmeLoading = true
			r.readmeError = nil
			r.readmeSearchMode = false
			r.readmeSearchQuery = ""
			r.readmeSearchMatches = nil
			r.readmeSearchIdx = 0
			cmds = append(cmds, r.fetchReadme())
		}

	case historyMsg:
		r.history = msg.entries
		r.err = msg.err
		r.historyView.SetContent(r.formatHistory())

	case valuesMsg:
		if msg.err == nil {
			// Store computed values for display
			content := GenerateValuesYAML(msg.values)
			r.valuesView.SetContent(content)
			r.originalValues = content

			// Store user values for editing during upgrade
			r.userValues = GenerateValuesYAML(msg.userValues)

			// Check if values are encrypted
			r.valuesEncrypted = helm.IsSOPSEncrypted(content)
			if r.valuesEncrypted {
				r.decryptionError = "Values contain encrypted data"
			} else {
				r.decryptionError = ""
			}
		} else {
			r.err = msg.err
		}

	case resourcesMsg:
		if msg.err == nil {
			r.resources = msg.resources
			r.resourcesView.SetContent(r.formatResources())
		} else {
			r.err = msg.err
		}

	case podStatusMsg:
		if msg.err == nil {
			r.podStatus = msg.pods
			// Update resources view to include pod status
			r.resourcesView.SetContent(r.formatResources())
		}

	case readmeMsg:
		r.readmeLoading = false
		if msg.err != nil {
			r.readmeError = msg.err
			r.readmeContent = ""
		} else {
			r.readmeContent = msg.content
			r.readmeViewport.SetContent(msg.content)
			r.readmeViewport.GotoTop()
		}

	case podListLoadedMsg:
		r.logLoading = false
		if msg.err != nil {
			r.logError = msg.err
		} else {
			r.logPodList = msg.pods
			r.logSelectedPodIdx = 0
			r.logContainerIdx = 0
		}

	case podLogsLoadedMsg:
		r.logLoading = false
		if msg.err != nil {
			r.logError = msg.err
			r.logContent = ""
		} else {
			r.logContent = msg.logs
			r.logViewport.SetContent(msg.logs)
			r.logViewport.GotoBottom() // Most recent logs at bottom
		}

	case execPodListLoadedMsg:
		r.execLoading = false
		if msg.err != nil {
			r.execError = msg.err
		} else {
			r.execPodList = msg.pods
			r.execSelectedPodIdx = 0
			r.execContainerIdx = 0
		}

	case execCompletedMsg:
		// Shell exited, dialog is already closed
		if msg.err != nil {
			r.execError = msg.err
		}

	case pfPodListLoadedMsg:
		r.pfLoading = false
		if msg.err != nil {
			r.pfError = msg.err
		} else {
			r.pfPodList = msg.pods
			r.pfSelectedPodIdx = 0
		}

	case pfPodPortsLoadedMsg:
		r.pfLoading = false
		if msg.err != nil {
			r.pfError = msg.err
		} else {
			r.pfPodPorts = msg.ports
			r.pfSelectedPortIdx = 0
			if len(msg.ports) > 0 {
				r.pfLocalPort = fmt.Sprint(msg.ports[0].ContainerPort)
			}
		}

	case pfStartedMsg:
		r.pfLoading = false
		if msg.err != nil {
			r.pfError = msg.err
		} else {
			// Refresh the list of active forwards
			r.pfActiveForwards = r.pfManager.ListPortForwardsByRelease(r.release.Name, r.release.Namespace)
			r.status = fmt.Sprintf("%s Port forward started: localhost:%d → %d", Icons.Check, msg.pf.LocalPort, msg.pf.RemotePort)
		}

	case pfStoppedMsg:
		if msg.err != nil {
			r.pfError = msg.err
		} else {
			// Refresh the list of active forwards
			r.pfActiveForwards = r.pfManager.ListPortForwardsByRelease(r.release.Name, r.release.Namespace)
			r.status = Icons.Check + " Port forward stopped"
		}

	case revisionDataMsg:
		r.compareLoading = false
		if msg.err != nil {
			r.compareError = msg.err
		} else {
			r.compareValues1 = msg.values1
			r.compareValues2 = msg.values2
			r.compareManifest1 = msg.manifest1
			r.compareManifest2 = msg.manifest2

			// Compute diffs
			r.compareValuesDiff = ComputeDiff(r.compareValues1, r.compareValues2)
			r.compareManifestDiff = ComputeDiff(r.compareManifest1, r.compareManifest2)

			// Update viewport content
			r.updateCompareViewportContent()
		}

	case rollbackResultMsg:
		r.showRollbackConfirm = false
		if msg.err != nil {
			r.err = msg.err
			r.status = ""
		} else {
			r.status = Icons.Check + " Rolled back successfully to revision " + fmt.Sprint(r.rollbackTarget)
			// Refresh history and pod status
			cmds = append(cmds, r.fetchHistory(), r.fetchPodStatus())
		}

	case upgradeResultMsg:
		r.upgrading = false
		r.showUpgradeConfirm = false
		if msg.err != nil {
			r.err = msg.err
			r.status = ""
		} else {
			r.status = Icons.Check + " Upgraded successfully!"
			// Refresh history, values, and pod status
			cmds = append(cmds, r.fetchHistory(), r.fetchValues(), r.fetchPodStatus())
		}

	case upgradeValuesEditedMsg:
		if msg.err != nil {
			r.err = msg.err
		} else {
			r.upgradeValuesContent = msg.content
			// Compute values diff (compare user values, not computed values)
			r.valuesDiff = ComputeDiff(r.userValues, msg.content)
			// Show diff preview with loading state for manifest
			r.showDiffPreview = true
			r.loadingDiff = true
			r.diffActiveTab = 0
			r.updateDiffViewportContent()
			// Compute manifest diff in background
			cmds = append(cmds, r.computeManifestDiff())
		}

	case manifestMsg:
		r.currentManifest = msg.manifest
		if msg.err != nil {
			r.err = msg.err
		}

	case dialog.FileImportResultMsg:
		if msg.Path != "" {
			r.upgradeValueFiles = append(r.upgradeValueFiles, msg.Path)
			// Sync to value sources dialog if open
			if r.upgradeValueSourcesDialog != nil && r.upgradeValueSourcesDialog.IsOpen() {
				r.upgradeValueSourcesDialog.AddValueFile(msg.Path)
			}
		}

	case dialog.SetValueResultMsg:
		r.upgradeSetValues = append(r.upgradeSetValues, msg.KeyValue)
		// Sync to value sources dialog if open
		if r.upgradeValueSourcesDialog != nil && r.upgradeValueSourcesDialog.IsOpen() {
			r.upgradeValueSourcesDialog.AddSetValue(msg.KeyValue)
		}

	case dialog.ValueSourcesClosedMsg:
		r.upgradeValueFiles = msg.ValueFiles
		r.upgradeSetValues = msg.SetValues

	case dialog.ValueSourcesAddFileMsg:
		if r.upgradeFileImportDialog != nil {
			r.upgradeFileImportDialog.Open()
		}

	case dialog.ValueSourcesAddSetMsg:
		if r.upgradeSetValueDialog != nil {
			r.upgradeSetValueDialog.Open()
		}

	case diffComputedMsg:
		r.loadingDiff = false
		if msg.err != nil {
			r.err = msg.err
			r.manifestDiff = DiffResult{HasChanges: false}
		} else {
			r.previewManifest = msg.newManifest
			r.manifestDiff = ComputeDiff(r.currentManifest, msg.newManifest)
		}
		r.updateDiffViewportContent()
	}

	// Update focused component
	var cmd tea.Cmd
	switch r.activePane {
	case DetailPaneHistory:
		r.historyView, cmd = r.historyView.Update(msg)
		cmds = append(cmds, cmd)
	case DetailPaneValues:
		r.valuesView, cmd = r.valuesView.Update(msg)
		cmds = append(cmds, cmd)
	case DetailPaneResources:
		r.resourcesView, cmd = r.resourcesView.Update(msg)
		cmds = append(cmds, cmd)
	}

	return r, tea.Batch(cmds...)
}

// View renders the release detail
func (r *ReleaseDetail) View() string {
	if r.release == nil {
		return "No release selected"
	}

	// Upgrade value source dialogs (render on top of everything)
	if r.upgradeFileImportDialog != nil && r.upgradeFileImportDialog.IsOpen() {
		return r.upgradeFileImportDialog.View()
	}
	if r.upgradeSetValueDialog != nil && r.upgradeSetValueDialog.IsOpen() {
		return r.upgradeSetValueDialog.View()
	}
	if r.upgradeValueSourcesDialog != nil && r.upgradeValueSourcesDialog.IsOpen() {
		return r.upgradeValueSourcesDialog.View()
	}

	if r.showLogDialog {
		return r.renderLogDialog()
	}

	if r.showExecDialog {
		return r.renderExecDialog()
	}

	if r.showPortForwardDialog {
		return r.renderPortForwardDialog()
	}

	if r.showRevisionCompare {
		return r.renderRevisionCompare()
	}

	if r.showReadmeDialog {
		return r.renderReadmeDialog()
	}

	if r.showRollbackConfirm {
		return r.renderRollbackConfirm()
	}

	if r.showDiffPreview {
		return r.renderDiffPreview()
	}

	if r.showUpgradeConfirm {
		return r.renderUpgradeConfirm()
	}

	return r.renderMainView()
}

func (r *ReleaseDetail) renderMainView() string {
	// Account for nav bar and status bar
	contentHeight := r.height - 3
	// Use 0.25 ratio for left column (Info/History) - makes right column wider
	layout := CalculatePaneLayout(r.width, contentHeight, 0.25)

	// Pane navigation bar
	paneNames := []string{"Info", "History", "Values", "Resources"}
	navBar := PaneNavBar(paneNames, int(r.activePane), r.width)

	// Info panel (top left - compact)
	infoActive := r.activePane == DetailPaneInfo
	infoStyle := S.BorderNormal
	if infoActive {
		infoStyle = S.BorderFocus
	}

	infoHeader := RenderPaneHeader(Icons.Release, "Info", -1, infoActive)
	infoContent := infoHeader + "\n"
	infoContent += S.Label.Render("Name: ") + S.Value.Render(r.release.Name) + "\n"
	infoContent += S.Label.Render("NS: ") + S.Value.Render(r.release.Namespace) + "\n"
	infoContent += S.Label.Render("Chart: ") + S.Value.Render(r.release.Chart) + "\n"
	infoContent += S.Label.Render("Status: ") + RenderStatus(r.release.Status) + "\n"
	infoContent += S.Label.Render("Rev: ") + S.Value.Render(fmt.Sprint(r.release.Revision))

	infoBox := infoStyle.Width(layout.LeftWidth).Height(layout.TopHeight).Render(infoContent)

	// History panel (bottom left - with viewport)
	histActive := r.activePane == DetailPaneHistory
	historyStyle := S.BorderNormal
	if histActive {
		historyStyle = S.BorderFocus
	}

	histHeader := RenderPaneHeader(Icons.History, "History", len(r.history), histActive)
	scrollHint := ""
	if r.historyView.TotalLineCount() > r.historyView.Height {
		scrollHint = RenderScrollHint(r.historyView.YOffset, r.historyView.TotalLineCount()-r.historyView.Height)
	}
	historyContent := histHeader + scrollHint + "\n" + r.historyView.View()

	historyBox := historyStyle.Width(layout.LeftWidth).Height(layout.BottomHeight).Render(historyContent)

	// Values panel (top right - with viewport)
	valActive := r.activePane == DetailPaneValues
	valuesStyle := S.BorderNormal
	if valActive {
		valuesStyle = S.BorderFocus
	}

	valTitle := "Values"
	if r.valuesEncrypted {
		valTitle = "Values " + Icons.Lock
	}
	valHeader := RenderPaneHeader(Icons.Values, valTitle, -1, valActive)
	valScrollHint := ""
	if r.valuesView.TotalLineCount() > r.valuesView.Height {
		valScrollHint = RenderScrollHint(r.valuesView.YOffset, r.valuesView.TotalLineCount()-r.valuesView.Height)
	}
	valuesContent := valHeader + valScrollHint + "\n" + r.valuesView.View()

	valuesBox := valuesStyle.Width(layout.RightWidth).Height(layout.TopHeight).Render(valuesContent)

	// Resources panel (bottom right - with viewport)
	resActive := r.activePane == DetailPaneResources
	resourcesStyle := S.BorderNormal
	if resActive {
		resourcesStyle = S.BorderFocus
	}

	resHeader := RenderPaneHeader(Icons.Chart, "Resources", len(r.resources), resActive)
	resScrollHint := ""
	if r.resourcesView.TotalLineCount() > r.resourcesView.Height {
		resScrollHint = RenderScrollHint(r.resourcesView.YOffset, r.resourcesView.TotalLineCount()-r.resourcesView.Height)
	}
	resourcesContent := resHeader + resScrollHint + "\n" + r.resourcesView.View()

	resourcesBox := resourcesStyle.Width(layout.RightWidth).Height(layout.BottomHeight).Render(resourcesContent)

	// Layout: 2x2 grid
	leftColumn := lipgloss.JoinVertical(lipgloss.Left, infoBox, historyBox)
	rightColumn := lipgloss.JoinVertical(lipgloss.Left, valuesBox, resourcesBox)
	main := lipgloss.JoinHorizontal(lipgloss.Top, leftColumn, rightColumn)

	// Status bar with hints
	var statusLeft string
	if r.status != "" {
		statusLeft = S.Success.Render(r.status)
	}
	if r.err != nil {
		statusLeft = S.Error.Render(Icons.Cross + " " + r.err.Error())
	}

	hints := S.Muted.Render("u:upgrade  d:diff  r:rollback  R:readme  l:logs  x:shell  f:port-forward  p:pods  q:back")

	return navBar + "\n" + main + "\n" + StatusBarView(statusLeft, hints, r.width)
}

// formatHistory returns formatted history content for viewport
func (r *ReleaseDetail) formatHistory() string {
	if len(r.history) == 0 {
		return S.Muted.Render("No history available")
	}

	var lines []string

	for i, entry := range r.history {
		marker := "  "
		if i == 0 {
			marker = Icons.Deployed + " " // Current
		}

		line := fmt.Sprintf("%sRev %d: %s (%s)",
			marker,
			entry.Revision,
			entry.Status,
			entry.Updated.Format("Jan 02 15:04"),
		)

		if i == r.selectedRev {
			line = S.Selected.Render(Icons.Arrow + " " + line[2:])
		} else if i == 0 {
			line = S.Success.Render(line)
		} else {
			line = S.Value.Render(line)
		}

		lines = append(lines, line)
	}

	return strings.Join(lines, "\n")
}

// formatResources returns formatted resources content for viewport
func (r *ReleaseDetail) formatResources() string {
	if len(r.resources) == 0 && len(r.podStatus) == 0 {
		return S.Muted.Render("No resources found")
	}

	var lines []string

	// Show pod status section first if we have pods
	if len(r.podStatus) > 0 {
		lines = append(lines, S.Label.Render(Icons.Deployed+" Pods:"))
		// Column header
		lines = append(lines, S.Muted.Render("  STATUS           NAME                                     READY   CPU      MEM      RESTARTS"))

		for _, pod := range r.podStatus {
			statusStyle := r.getPodStatusStyle(pod.Status)

			// Truncate pod name if too long
			podName := pod.Name
			if len(podName) > 40 {
				podName = podName[:37] + "..."
			}

			// Format CPU/Memory (show "-" if not available)
			cpu := pod.CPUUsage
			if cpu == "" {
				cpu = "-"
			}
			mem := pod.MemoryUsage
			if mem == "" {
				mem = "-"
			}

			// Pad values for alignment (pad BEFORE styling)
			statusPadded := fmt.Sprintf("%-16s", pod.Status)
			namePadded := fmt.Sprintf("%-40s", podName)
			readyPadded := fmt.Sprintf("%-7s", pod.Ready)
			cpuPadded := fmt.Sprintf("%-8s", cpu)
			memPadded := fmt.Sprintf("%-8s", mem)
			restartsPadded := fmt.Sprintf("%d", pod.Restarts)

			// Build line with styled padded values
			line := fmt.Sprintf("  %s %s %s %s %s %s",
				statusStyle.Render(statusPadded),
				S.Value.Render(namePadded),
				S.Muted.Render(readyPadded),
				S.Value.Render(cpuPadded),
				S.Value.Render(memPadded),
				S.Muted.Render(restartsPadded),
			)
			lines = append(lines, line)
		}
		lines = append(lines, "") // Empty line separator
	}

	// Show resources section
	if len(r.resources) > 0 {
		lines = append(lines, S.Label.Render(Icons.Chart+" Resources:"))
		for _, res := range r.resources {
			// Parse Kind/Name format
			parts := strings.SplitN(res, "/", 2)
			kind := parts[0]
			name := ""
			if len(parts) > 1 {
				name = parts[1]
			}

			// Add icon based on kind
			icon := Icons.Bullet
			switch kind {
			case "Deployment":
				icon = Icons.Deployed
			case "Service":
				icon = "◆"
			case "ConfigMap":
				icon = "◇"
			case "Secret":
				icon = "◈"
			case "Ingress":
				icon = Icons.ArrowRight
			case "ServiceAccount":
				icon = "◎"
			case "PersistentVolumeClaim", "PVC":
				icon = "◉"
			case "StatefulSet":
				icon = "▣"
			case "DaemonSet":
				icon = "▤"
			case "Job", "CronJob":
				icon = "▶"
			}

			if name != "" {
				lines = append(lines, fmt.Sprintf("  %s %s/%s", icon, S.Muted.Render(kind), S.Value.Render(name)))
			} else {
				lines = append(lines, fmt.Sprintf("  %s %s", icon, S.Value.Render(kind)))
			}
		}
	}

	return strings.Join(lines, "\n")
}

// getPodStatusStyle returns the appropriate style for a pod status
func (r *ReleaseDetail) getPodStatusStyle(status string) lipgloss.Style {
	switch status {
	case "Running":
		return S.Success
	case "Pending", "ContainerCreating":
		return S.Warning
	case "Succeeded", "Completed":
		return S.Success
	case "Failed", "Error", "CrashLoopBackOff", "ImagePullBackOff":
		return S.Error
	case "Terminating":
		return S.Warning
	default:
		if strings.HasPrefix(status, "Init:") {
			return S.Warning
		}
		return S.Muted
	}
}

// --- Async Message Types ---
//
// These message types are returned by tea.Cmd functions to communicate
// results of async operations back to Update(). Each message carries
// either success data or an error.
//
// Flow: User action → tea.Cmd (goroutine) → Message → Update() → State change
//
// Message categories:
//   - Data loading: historyMsg, valuesMsg, resourcesMsg, podStatusMsg
//   - Operations: rollbackResultMsg, upgradeResultMsg, upgradeValuesEditedMsg (in sibling files)
//   - Diff preview: manifestMsg, diffComputedMsg (in release_detail_upgrade.go)
//   - Log viewer: podListLoadedMsg, podLogsLoadedMsg (in release_detail_logs.go)
//   - Shell exec: execPodListLoadedMsg, execCompletedMsg (in release_detail_exec.go)
//   - Port forward: pfPodListLoadedMsg, pfPodPortsLoadedMsg, pfStartedMsg, pfStoppedMsg (in release_detail_portforward.go)
//   - Revision compare: revisionDataMsg (in release_detail_compare.go)
//   - README: readmeMsg (in release_detail_readme.go)

type historyMsg struct {
	entries []HistoryEntry
	err     error
}

type valuesMsg struct {
	values     map[string]interface{} // Computed values (for display)
	userValues map[string]interface{} // User-supplied values only (for editing)
	err        error
}

type resourcesMsg struct {
	resources []string
	err       error
}

type podStatusMsg struct {
	pods []helm.PodStatus
	err  error
}

func (r *ReleaseDetail) fetchHistory() tea.Cmd {
	return func() tea.Msg {
		history, err := r.releaseManager.GetHistory(r.release.Name, r.release.Namespace)
		if err != nil {
			return historyMsg{err: err}
		}

		entries := make([]HistoryEntry, len(history))
		for i, h := range history {
			entries[i] = HistoryEntry{
				Revision:    h.Version,
				Updated:     h.Info.LastDeployed.Time,
				Status:      h.Info.Status.String(),
				Chart:       h.Chart.Metadata.Name + "-" + h.Chart.Metadata.Version,
				AppVersion:  h.Chart.Metadata.AppVersion,
				Description: h.Info.Description,
			}
		}

		// Sort by revision descending (newest first)
		for i := 0; i < len(entries)/2; i++ {
			j := len(entries) - 1 - i
			entries[i], entries[j] = entries[j], entries[i]
		}

		return historyMsg{entries: entries}
	}
}

func (r *ReleaseDetail) fetchValues() tea.Cmd {
	return func() tea.Msg {
		// Fetch computed values (for display)
		values, err := r.releaseManager.GetValues(r.release.Name, r.release.Namespace)
		if err != nil {
			return valuesMsg{err: err}
		}
		// Fetch user-supplied values only (for editing during upgrade)
		userValues, err := r.releaseManager.GetUserValues(r.release.Name, r.release.Namespace)
		return valuesMsg{values: values, userValues: userValues, err: err}
	}
}

func (r *ReleaseDetail) fetchResources() tea.Cmd {
	return func() tea.Msg {
		resources, err := r.releaseManager.GetDeployedResources(r.release.Name, r.release.Namespace)
		return resourcesMsg{resources: resources, err: err}
	}
}

func (r *ReleaseDetail) fetchPodStatus() tea.Cmd {
	return func() tea.Msg {
		// Use the metrics-enabled version to get CPU/memory usage
		pods, err := r.clusterInspector.GetReleasePodStatusWithMetrics(r.release.Name, r.release.Namespace)
		return podStatusMsg{pods: pods, err: err}
	}
}

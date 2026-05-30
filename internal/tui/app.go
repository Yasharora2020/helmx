package tui

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/yasharora2020/helmx/internal/config"
	"github.com/yasharora2020/helmx/internal/helm"
)

// Feature request rate limiting (client-side)
// Webhook URL and API key are configured via config file or environment variables:
// - Config: ~/.config/helmx/config.yaml (featureRequest.url, featureRequest.apiKey)
// - Env: HELMX_FEATURE_REQUEST_URL, HELMX_FEATURE_REQUEST_API_KEY
const featureRequestCooldown = 24 * time.Hour

// Mode represents which view we're in
type Mode int

const (
	ModeReleases      Mode = iota // Manage installed releases
	ModeReleaseDetail             // View release details, history, upgrade
	ModeExplore                   // Explore charts before installing
	ModeRepos                     // Manage repositories
	ModeSettings                  // Application settings
	ModeRBAC                      // RBAC management
	ModeWelcome                   // Welcome/about page
)

// BulkUpgradeStatus represents the status of a release in bulk upgrade
type BulkUpgradeStatus int

const (
	BulkUpgradePending BulkUpgradeStatus = iota
	BulkUpgradeInProgress
	BulkUpgradeSuccess
	BulkUpgradeFailed
)

// BulkUpgradeItem represents a release in the bulk upgrade queue
type BulkUpgradeItem struct {
	Name      string
	Namespace string
	Chart     string
	Status    BulkUpgradeStatus
	Error     error
}

// Tab names for the tab bar
var TabNames = []string{
	"Releases",
	"Explore",
	"Repos",
	"Settings",
	"RBAC",
}

// App is the root model for the TUI
// In Bubble Tea, the "model" holds ALL your application state
type App struct {
	mode          Mode
	releases      list.Model     // List component for releases
	releaseDetail *ReleaseDetail // Release detail view
	exploreView   *ExploreView   // Chart exploration view
	reposView     *ReposView     // Repository management view
	settingsView  *SettingsView  // Settings view
	rbacView      *RBACView      // RBAC management view
	welcomeView   *WelcomeView   // Welcome/about page view
	helmClient    *helm.Client   // Our Helm SDK wrapper
	config        *config.Config // Application config
	width         int            // Terminal width
	height        int            // Terminal height
	err           error          // Last error (if any)
	keys          keyMap         // Keybindings
	breadcrumb    []string       // Current navigation path
	helpOverlay   *HelpOverlay   // Help overlay

	// Namespace filtering
	namespaces        []string // Available namespaces
	selectedNamespace string   // Current filter ("" = all)
	namespaceIndex    int      // Index in namespaces list (-1 = all)

	// Delete confirmation
	showDeleteConfirm bool   // Show delete confirmation dialog
	deleteReleaseName string // Name of release to delete
	deleteReleaseNS   string // Namespace of release to delete

	// Feature request dialog
	showFeatureRequest    bool            // Show feature request dialog
	featureRequestInput   textinput.Model // Text input for feature request
	featureRequestSending bool            // True while sending request
	featureRequestError   error           // Error from submission
	featureRequestSuccess bool            // True if successfully submitted

	// Multi-select for bulk operations
	selectedReleases map[string]bool // Map of "namespace/name" -> selected

	// Bulk upgrade dialog
	showBulkUpgrade       bool              // Show bulk upgrade dialog
	bulkUpgradeQueue      []BulkUpgradeItem // Queue of releases to upgrade
	bulkUpgradeIdx        int               // Current index in upgrade queue
	bulkUpgradeInProgress bool              // True while bulk upgrade is running
	bulkUpgradeError      error             // Last error during bulk upgrade
}

// keyMap defines all keyboard shortcuts
type keyMap struct {
	Quit     key.Binding
	Help     key.Binding
	Switch   key.Binding
	Enter    key.Binding
	Refresh  key.Binding
	Delete   key.Binding
	Rollback key.Binding
	Back     key.Binding
}

func defaultKeyMap() keyMap {
	return keyMap{
		Quit:     key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
		Help:     key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
		Switch:   key.NewBinding(key.WithKeys("1", "2", "3", "4"), key.WithHelp("1/2/3/4", "releases/explore/repos/settings")),
		Enter:    key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "select")),
		Refresh:  key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh")),
		Delete:   key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "delete")),
		Rollback: key.NewBinding(key.WithKeys("b"), key.WithHelp("b", "rollback")),
		Back:     key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back")),
	}
}

// NewApp creates the initial application state
func NewApp() *App {
	// Load configuration
	cfg, _ := config.Load()

	// Apply saved theme
	if cfg.Theme != "" {
		SetTheme(cfg.Theme)
	}

	// Create the Helm client
	client, err := helm.NewClient("")

	// Create selected releases map first so we can pass it to delegate
	selectedReleases := make(map[string]bool)

	// Create an empty list with custom delegate for checkboxes
	delegate := newReleaseDelegate(&selectedReleases)
	releaseList := list.New([]list.Item{}, delegate, 0, 0)
	releaseList.Title = Icons.Release + " Releases"
	releaseList.SetShowStatusBar(true)
	releaseList.SetFilteringEnabled(true)
	releaseList.Styles.Title = S.Title

	// Create the explore view with config
	exploreView := NewExploreView(client, client, client, client, client, cfg)

	// Create the release detail view
	releaseDetail := NewReleaseDetail(client, client, client)

	// Create the repos view
	reposView := NewReposView(client)

	// Create the settings view
	settingsView := NewSettingsView(cfg, client, client)

	// Create the RBAC view
	rbacView := NewRBACView(client)

	// Create feature request text input
	featureInput := textinput.New()
	featureInput.Placeholder = "Describe the feature you'd like..."
	featureInput.CharLimit = 500
	featureInput.Width = 60

	// Create the welcome view
	welcomeView := NewWelcomeView(cfg)

	// Determine initial mode based on config
	initialMode := ModeReleases
	initialBreadcrumb := []string{"Releases"}
	if cfg.ShowWelcomeOnStart {
		initialMode = ModeWelcome
		initialBreadcrumb = []string{"Welcome"}
	}

	return &App{
		mode:                initialMode,
		releases:            releaseList,
		releaseDetail:       releaseDetail,
		exploreView:         exploreView,
		reposView:           reposView,
		settingsView:        settingsView,
		rbacView:            rbacView,
		welcomeView:         welcomeView,
		helmClient:          client,
		config:              cfg,
		keys:                defaultKeyMap(),
		err:                 err,
		breadcrumb:          initialBreadcrumb,
		helpOverlay:         NewHelpOverlay(),
		namespaceIndex:      -1, // -1 = all namespaces
		featureRequestInput: featureInput,
		selectedReleases:    selectedReleases,
	}
}

// Init is called once when the program starts
// Return a Cmd (command) to do I/O - here we fetch releases
func (a *App) Init() tea.Cmd {
	return tea.Batch(
		a.fetchReleases(), // Fetch helm releases on startup
	)
}

// Update handles all events (key presses, mouse, window resize, etc).
// This is THE place where state changes happen in the Bubble Tea architecture.
//
// # Message Flow
//
// Messages arrive from three sources:
//  1. User input (tea.KeyMsg, tea.MouseMsg)
//  2. System events (tea.WindowSizeMsg)
//  3. Async operation results (custom typed messages like releasesMsg, errMsg)
//
// # State Machine
//
// The Update function routes messages based on the current mode:
//
//	┌─────────────────────────────────────────────────────────────────┐
//	│                        Global Handlers                          │
//	│  (WindowSizeMsg, HelpOverlay toggle, async result messages)     │
//	└─────────────────────────────────────────────────────────────────┘
//	                              │
//	                              ▼
//	┌─────────────────────────────────────────────────────────────────┐
//	│                     Mode-Specific Routing                       │
//	│  KeyMsg → updateReleasesMode / updateDetailMode / etc.          │
//	└─────────────────────────────────────────────────────────────────┘
//	                              │
//	                              ▼
//	┌─────────────────────────────────────────────────────────────────┐
//	│                    View Delegation                              │
//	│  Non-key messages → delegate to active view's Update()          │
//	└─────────────────────────────────────────────────────────────────┘
//
// # Mode Transitions
//
//   - Tab 1-4: Switch between main modes (Releases, Explore, Repos, Settings)
//   - Enter on release: ModeReleases → ModeReleaseDetail
//   - Escape: Return to previous mode or close dialog
//   - Context switch: Triggers ContextSwitchedMsg → refreshes releases
//
// # Dialog Handling
//
// When a dialog is shown (showDeleteConfirm, showFeatureRequest, etc.),
// keyboard input is handled by the dialog first. The dialog sets flags
// to indicate completion, and the next Update cycle processes the result.
func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {

	// Window resized
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		// Account for tab bar and status bar
		contentHeight := msg.Height - 4
		a.releases.SetSize(msg.Width-4, contentHeight-2)
		// Update explore view
		var cmd tea.Cmd
		a.exploreView.width = msg.Width
		a.exploreView.height = contentHeight
		a.exploreView, cmd = a.exploreView.Update(msg)
		cmds = append(cmds, cmd)
		// Update release detail view
		a.releaseDetail.SetSize(msg.Width, contentHeight)
		// Update repos view
		a.reposView.SetSize(msg.Width, contentHeight)
		// Update settings view
		a.settingsView.SetSize(msg.Width, contentHeight)
		// Update RBAC view
		a.rbacView.SetSize(msg.Width, contentHeight)
		// Update help overlay
		a.helpOverlay.SetSize(msg.Width, msg.Height)
		// Update welcome view
		a.welcomeView.SetSize(msg.Width, contentHeight)

	// Keyboard input
	case tea.KeyMsg:
		// Handle help overlay first
		if a.helpOverlay.IsVisible() {
			if msg.String() == "?" || msg.String() == "esc" || msg.String() == "q" {
				a.helpOverlay.Hide()
			}
			return a, nil
		}

		// Toggle help with ?
		if msg.String() == "?" {
			a.helpOverlay.Toggle()
			return a, nil
		}

		// Handle based on current mode
		switch a.mode {
		case ModeReleases:
			return a.updateReleasesMode(msg)
		case ModeReleaseDetail:
			return a.updateDetailMode(msg)
		case ModeExplore:
			return a.updateExploreMode(msg)
		case ModeRepos:
			return a.updateReposMode(msg)
		case ModeSettings:
			return a.updateSettingsMode(msg)
		case ModeRBAC:
			return a.updateRBACMode(msg)
		case ModeWelcome:
			return a.updateWelcomeMode(msg)
		}

	// Our custom message when releases are fetched
	case releasesMsg:
		a.releases.SetItems(msg.items)
		a.namespaces = msg.namespaces

	// Error message
	case errMsg:
		a.err = msg.err

	// Context switched - refresh releases
	case ContextSwitchedMsg:
		return a, a.fetchReleases()

	// Feature request result
	case featureRequestResultMsg:
		a.featureRequestSending = false
		if msg.err != nil {
			a.featureRequestError = msg.err
		} else {
			a.featureRequestSuccess = true
			// Save timestamp for client-side rate limiting
			a.config.LastFeatureRequestTime = time.Now()
			_ = a.config.Save() // Ignore error - best effort save
		}
		return a, nil

	// Bulk upgrade progress
	case bulkUpgradeProgressMsg:
		if msg.idx < len(a.bulkUpgradeQueue) {
			a.bulkUpgradeQueue[msg.idx].Status = msg.status
			a.bulkUpgradeQueue[msg.idx].Error = msg.err

			// If failed, stop the bulk upgrade
			if msg.status == BulkUpgradeFailed {
				a.bulkUpgradeInProgress = false
				a.bulkUpgradeError = msg.err
				return a, nil
			}

			// Move to next release
			a.bulkUpgradeIdx++
			if a.bulkUpgradeIdx < len(a.bulkUpgradeQueue) {
				return a, a.upgradeNextRelease()
			} else {
				// All done
				a.bulkUpgradeInProgress = false
			}
		}
		return a, nil
	}

	// Delegate to the active view for non-key messages
	var cmd tea.Cmd
	switch a.mode {
	case ModeReleases:
		a.releases, cmd = a.releases.Update(msg)
	case ModeReleaseDetail:
		a.releaseDetail, cmd = a.releaseDetail.Update(msg)
	case ModeExplore:
		a.exploreView, cmd = a.exploreView.Update(msg)
	case ModeRepos:
		a.reposView, cmd = a.reposView.Update(msg)
	case ModeSettings:
		a.settingsView, cmd = a.settingsView.Update(msg)
	case ModeRBAC:
		a.rbacView, cmd = a.rbacView.Update(msg)
	case ModeWelcome:
		a.welcomeView, cmd = a.welcomeView.Update(msg)
	}
	cmds = append(cmds, cmd)

	return a, tea.Batch(cmds...)
}

// handleGlobalNavigation handles mode-switching keys shared across all views.
// Returns (handled bool, model tea.Model, cmd tea.Cmd).
// If handled is true, the caller should return the model and cmd immediately.
func (a *App) handleGlobalNavigation(key string, dialogOpen bool) (bool, tea.Model, tea.Cmd) {
	if dialogOpen {
		return false, a, nil
	}

	switch key {
	case "0", "w":
		a.mode = ModeWelcome
		a.breadcrumb = []string{"Welcome"}
		return true, a, nil
	case "1":
		a.mode = ModeReleases
		a.breadcrumb = []string{"Releases"}
		return true, a, a.fetchReleases()
	case "2":
		a.mode = ModeExplore
		a.breadcrumb = []string{"Explore"}
		return true, a, nil
	case "3":
		a.mode = ModeRepos
		a.breadcrumb = []string{"Repos"}
		return true, a, a.reposView.Init()
	case "4":
		a.mode = ModeSettings
		a.breadcrumb = []string{"Settings"}
		return true, a, nil
	case "5":
		a.mode = ModeRBAC
		a.breadcrumb = []string{"RBAC"}
		return true, a, a.rbacView.Init()
	}

	return false, a, nil
}

func (a *App) updateReleasesMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Handle bulk upgrade dialog
	if a.showBulkUpgrade {
		return a.updateBulkUpgradeDialog(msg)
	}

	// Handle feature request dialog
	if a.showFeatureRequest {
		return a.updateFeatureRequestDialog(msg)
	}

	// Handle delete confirmation dialog
	if a.showDeleteConfirm {
		return a.updateDeleteConfirm(msg)
	}

	// When filtering is active, only handle quit/escape/number keys
	// Let all other keys pass through to the list's filter input
	filtering := a.releases.FilterState() == list.Filtering

	// Use shared navigation helper (filtering acts as dialog guard)
	if !filtering {
		if handled, model, cmd := a.handleGlobalNavigation(msg.String(), false); handled {
			return model, cmd
		}
	}

	switch msg.String() {
	case "q", "ctrl+c":
		if !filtering {
			return a, tea.Quit
		}
	case "enter":
		if !filtering {
			// Open detail view for selected release
			selected := a.releases.SelectedItem()
			if selected != nil {
				item := selected.(releaseItem)
				a.mode = ModeReleaseDetail
				a.breadcrumb = []string{"Releases", item.release.Name}
				return a, a.releaseDetail.SetRelease(&item.release)
			}
		}
	case "r":
		if !filtering {
			return a, a.fetchReleases()
		}
	case "d":
		if !filtering {
			// Show delete confirmation dialog
			selected := a.releases.SelectedItem()
			if selected != nil {
				item := selected.(releaseItem)
				a.showDeleteConfirm = true
				a.deleteReleaseName = item.release.Name
				a.deleteReleaseNS = item.release.Namespace
			}
			return a, nil
		}
	case "n":
		if !filtering {
			// Cycle through namespaces
			return a, a.cycleNamespace()
		}
	case " ":
		if !filtering {
			// Toggle selection for multi-select
			selected := a.releases.SelectedItem()
			if selected != nil {
				item := selected.(releaseItem)
				key := item.release.Namespace + "/" + item.release.Name
				if a.selectedReleases[key] {
					delete(a.selectedReleases, key)
				} else {
					a.selectedReleases[key] = true
				}
			}
			return a, nil
		}
	case "U":
		if !filtering {
			// Open bulk upgrade dialog if releases are selected
			if len(a.selectedReleases) > 0 {
				a.openBulkUpgradeDialog()
			}
			return a, nil
		}
	case "c":
		if !filtering {
			// Clear all selections
			clear(a.selectedReleases)
			return a, nil
		}
	case "f":
		if !filtering {
			// Open feature request dialog with rate limiting
			// Check if feature is enabled
			if !a.config.IsFeatureRequestEnabled() {
				a.showFeatureRequest = true
				a.featureRequestError = fmt.Errorf("feature request not configured - set HELMX_FEATURE_REQUEST_URL env var or config")
				a.featureRequestSuccess = false
				a.featureRequestSending = false
				return a, nil
			}
			cooldownRemaining := featureRequestCooldown - time.Since(a.config.LastFeatureRequestTime)
			if cooldownRemaining > 0 {
				// Show rate limit message
				a.showFeatureRequest = true
				a.featureRequestError = fmt.Errorf("please wait %s before submitting another request",
					cooldownRemaining.Round(time.Minute))
				a.featureRequestSuccess = false
				a.featureRequestSending = false
				return a, nil
			}
			a.showFeatureRequest = true
			a.featureRequestInput.Reset()
			a.featureRequestInput.Focus()
			a.featureRequestError = nil
			a.featureRequestSuccess = false
			a.featureRequestSending = false
			return a, textinput.Blink
		}
	}

	// Let list handle other keys
	var cmd tea.Cmd
	a.releases, cmd = a.releases.Update(msg)
	return a, cmd
}

func (a *App) updateDeleteConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y", "enter":
		a.showDeleteConfirm = false
		return a, a.deleteRelease()
	case "n", "N", "esc":
		a.showDeleteConfirm = false
	}
	return a, nil
}

func (a *App) updateFeatureRequestDialog(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// If showing success message, any key closes
	if a.featureRequestSuccess {
		a.showFeatureRequest = false
		a.featureRequestSuccess = false
		return a, nil
	}

	// Don't process input while sending
	if a.featureRequestSending {
		return a, nil
	}

	switch msg.String() {
	case "esc":
		a.showFeatureRequest = false
		a.featureRequestError = nil
		return a, nil
	case "enter":
		// Submit the feature request
		text := strings.TrimSpace(a.featureRequestInput.Value())
		if text != "" {
			a.featureRequestSending = true
			a.featureRequestError = nil
			return a, a.submitFeatureRequest(text)
		}
		return a, nil
	}

	// Let text input handle other keys
	var cmd tea.Cmd
	a.featureRequestInput, cmd = a.featureRequestInput.Update(msg)
	return a, cmd
}

func (a *App) updateDetailMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Check if any dialog is open
	dialogOpen := a.releaseDetail.HasOpenDialog()

	if handled, model, cmd := a.handleGlobalNavigation(msg.String(), dialogOpen); handled {
		return model, cmd
	}

	switch msg.String() {
	case "q", "ctrl+c":
		if !dialogOpen {
			a.mode = ModeReleases
			a.breadcrumb = []string{"Releases"}
			return a, nil
		}
	case "esc":
		if !dialogOpen {
			a.mode = ModeReleases
			a.breadcrumb = []string{"Releases"}
			return a, a.fetchReleases()
		}
	}

	var cmd tea.Cmd
	a.releaseDetail, cmd = a.releaseDetail.Update(msg)
	return a, cmd
}

func (a *App) updateExploreMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// If search input is focused, let explore view handle all keys (except help toggle)
	if a.exploreView.IsSearchFocused() && msg.String() != "?" {
		var cmd tea.Cmd
		a.exploreView, cmd = a.exploreView.Update(msg)
		return a, cmd
	}

	// Check if any dialog is open
	dialogOpen := a.exploreView.HasOpenDialog()

	// "/" and "esc" jump back to search input from chart list
	if !dialogOpen {
		switch msg.String() {
		case "/", "esc":
			a.exploreView.FocusSearch()
			return a, nil
		}
	}

	if handled, model, cmd := a.handleGlobalNavigation(msg.String(), dialogOpen); handled {
		return model, cmd
	}

	switch msg.String() {
	case "q", "ctrl+c":
		if !dialogOpen {
			return a, tea.Quit
		}
	}

	var cmd tea.Cmd
	a.exploreView, cmd = a.exploreView.Update(msg)
	return a, cmd
}

func (a *App) updateReposMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Check if any dialog is open
	dialogOpen := a.reposView.HasOpenDialog()

	if handled, model, cmd := a.handleGlobalNavigation(msg.String(), dialogOpen); handled {
		return model, cmd
	}

	switch msg.String() {
	case "q", "ctrl+c":
		if !dialogOpen {
			return a, tea.Quit
		}
	}

	var cmd tea.Cmd
	a.reposView, cmd = a.reposView.Update(msg)
	return a, cmd
}

func (a *App) updateSettingsMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Check if any dialog is open
	dialogOpen := a.settingsView.HasOpenDialog()

	if handled, model, cmd := a.handleGlobalNavigation(msg.String(), dialogOpen); handled {
		return model, cmd
	}

	switch msg.String() {
	case "q", "ctrl+c":
		if !dialogOpen {
			return a, tea.Quit
		}
	case "esc":
		if !dialogOpen {
			a.mode = ModeReleases
			a.breadcrumb = []string{"Releases"}
			return a, nil
		}
	}

	var cmd tea.Cmd
	a.settingsView, cmd = a.settingsView.Update(msg)
	return a, cmd
}

func (a *App) updateRBACMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Check if any dialog is open
	dialogOpen := a.rbacView.HasOpenDialog()

	if handled, model, cmd := a.handleGlobalNavigation(msg.String(), dialogOpen); handled {
		return model, cmd
	}

	switch msg.String() {
	case "q", "ctrl+c":
		if !dialogOpen {
			return a, tea.Quit
		}
	case "esc":
		if !dialogOpen {
			a.mode = ModeReleases
			a.breadcrumb = []string{"Releases"}
			return a, nil
		}
	}

	var cmd tea.Cmd
	a.rbacView, cmd = a.rbacView.Update(msg)
	return a, cmd
}

func (a *App) updateWelcomeMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if handled, model, cmd := a.handleGlobalNavigation(msg.String(), false); handled {
		return model, cmd
	}

	switch msg.String() {
	case "q", "ctrl+c":
		return a, tea.Quit
	case "enter", "esc":
		// Go to Releases view
		a.mode = ModeReleases
		a.breadcrumb = []string{"Releases"}
		return a, nil
	}

	// Let welcome view handle scrolling and checkbox toggle
	var cmd tea.Cmd
	a.welcomeView, cmd = a.welcomeView.Update(msg)
	return a, cmd
}

// View renders the UI - called after every Update
// Returns a string that gets printed to the terminal
func (a *App) View() string {
	if a.width == 0 {
		return "Loading..."
	}

	// Show help overlay if visible
	if a.helpOverlay.IsVisible() {
		return a.helpOverlay.View()
	}

	// Special k9s-style layout for Releases view (home screen)
	if a.mode == ModeReleases {
		return a.renderReleasesHome()
	}

	// Full-screen welcome view
	if a.mode == ModeWelcome {
		return a.welcomeView.View()
	}

	// Determine active tab index
	activeTab := 0
	switch a.mode {
	case ModeReleases, ModeReleaseDetail:
		activeTab = 0
	case ModeExplore:
		activeTab = 1
	case ModeRepos:
		activeTab = 2
	case ModeSettings:
		activeTab = 3
	case ModeRBAC:
		activeTab = 4
	}

	// Tab bar
	tabBar := TabBarView(TabNames, activeTab, a.width)

	// Breadcrumb
	breadcrumb := BreadcrumbView(a.breadcrumb)

	// Main content
	var content string
	var hints [][2]string

	switch a.mode {
	case ModeReleaseDetail:
		content = a.releaseDetail.View()
		hints = [][2]string{{"u", "upgrade"}, {"r", "rollback"}, {"c", "compare"}, {"f", "forward"}, {"d", "diff"}, {"?", "help"}}

	case ModeExplore:
		content = a.exploreView.View()
		hints = [][2]string{{"⏎", "search"}, {"i", "install"}, {"r", "readme"}, {"v", "versions"}, {"t", "template"}, {"D", "deps"}, {"?", "help"}}

	case ModeRepos:
		content = a.reposView.View()
		hints = [][2]string{{"a", "add"}, {"d", "delete"}, {"u", "update"}, {"?", "help"}}

	case ModeSettings:
		content = a.settingsView.View()
		hints = [][2]string{{"e", "edit"}, {"r", "reset"}, {"?", "help"}}

	case ModeRBAC:
		content = a.rbacView.View()
		hints = [][2]string{{"a", "add"}, {"e", "edit"}, {"+", "add ns"}, {"d", "delete"}, {"K", "kubeconfig"}, {"?", "help"}}
	}

	// Error handling
	var statusLeft string
	if a.err != nil {
		statusLeft = S.Error.Render(Icons.Cross + " " + a.err.Error())
	}

	// Status bar
	statusRight := KeyHints(hints)
	statusBar := StatusBarView(statusLeft, statusRight, a.width)

	// Combine all parts
	return lipgloss.JoinVertical(lipgloss.Left,
		tabBar,
		breadcrumb,
		content,
		statusBar,
	)
}

// renderReleasesHome renders the k9s-style home screen with logo
func (a *App) renderReleasesHome() string {
	// Show bulk upgrade dialog if active
	if a.showBulkUpgrade {
		return a.renderBulkUpgradeDialog()
	}

	// Show feature request dialog if active
	if a.showFeatureRequest {
		return a.renderFeatureRequestDialog()
	}

	// Show delete confirmation dialog if active
	if a.showDeleteConfirm {
		return a.renderDeleteConfirm()
	}

	// Key bar at top
	keyBar := KeyBar(a.width)

	// Logo section with context info
	context := a.helmClient.GetCurrentContext()
	logo := RenderLogo(a.width, context, a.selectedNamespace)

	// Releases list (adjust height for logo)
	// Logo takes ~12 lines, key bar 1, status bar 1
	listHeight := a.height - 14
	if listHeight < 5 {
		listHeight = 5
	}
	a.releases.SetHeight(listHeight)

	releasesContent := "\n" + a.releases.View()

	// Error handling
	var statusLeft string
	if a.err != nil {
		statusLeft = S.Error.Render(Icons.Cross + " " + a.err.Error())
	}

	// Status bar with hints
	hints := [][2]string{{"⏎", "details"}, {"␣", "select"}, {"U", "bulk upgrade"}, {"d", "delete"}, {"n", "namespace"}, {"r", "refresh"}}
	// Show selection count if any releases are selected
	if len(a.selectedReleases) > 0 {
		hints = append([][2]string{{fmt.Sprintf("%d", len(a.selectedReleases)), "selected"}}, hints...)
	}
	statusRight := KeyHints(hints)
	statusBar := StatusBarView(statusLeft, statusRight, a.width)

	return lipgloss.JoinVertical(lipgloss.Left,
		keyBar,
		logo,
		releasesContent,
		statusBar,
	)
}

// renderDeleteConfirm renders the delete confirmation dialog
func (a *App) renderDeleteConfirm() string {
	dialogStyle := lipgloss.NewStyle().
		Border(lipgloss.DoubleBorder()).
		BorderForeground(DefaultTheme.Error).
		Padding(2, 4).
		Width(50)

	content := S.Error.Render(Icons.Delete+" Delete Release") + "\n\n"
	content += "Uninstall release \"" + a.deleteReleaseName + "\"\n"
	content += "from namespace \"" + a.deleteReleaseNS + "\"?\n\n"
	content += S.Muted.Render("[Y]es  [N]o")

	dialog := dialogStyle.Render(content)

	return lipgloss.Place(a.width, a.height, lipgloss.Center, lipgloss.Center, dialog)
}

// --- Commands (async operations) ---

type releasesMsg struct {
	items      []list.Item
	namespaces []string
}

type errMsg struct {
	err error
}

type featureRequestResultMsg struct {
	err error
}

// bulkUpgradeProgressMsg is sent when a single release upgrade completes
type bulkUpgradeProgressMsg struct {
	idx    int
	status BulkUpgradeStatus
	err    error
}

func (a *App) fetchReleases() tea.Cmd {
	namespace := a.selectedNamespace
	return func() tea.Msg {
		releases, err := a.helmClient.ListReleases(namespace)
		if err != nil {
			return errMsg{err: err}
		}

		// Collect unique namespaces
		nsMap := make(map[string]bool)
		for _, r := range releases {
			nsMap[r.Namespace] = true
		}
		namespaces := make([]string, 0, len(nsMap))
		for ns := range nsMap {
			namespaces = append(namespaces, ns)
		}

		items := make([]list.Item, len(releases))
		for i, r := range releases {
			items[i] = releaseItem{release: r}
		}
		return releasesMsg{items: items, namespaces: namespaces}
	}
}

func (a *App) cycleNamespace() tea.Cmd {
	// Cycle: All -> ns1 -> ns2 -> ... -> All
	if len(a.namespaces) == 0 {
		return a.fetchReleases()
	}

	a.namespaceIndex++
	if a.namespaceIndex >= len(a.namespaces) {
		a.namespaceIndex = -1 // Back to "all"
		a.selectedNamespace = ""
	} else {
		a.selectedNamespace = a.namespaces[a.namespaceIndex]
	}

	// Update breadcrumb
	if a.selectedNamespace == "" {
		a.breadcrumb = []string{"Releases", "All Namespaces"}
	} else {
		a.breadcrumb = []string{"Releases", a.selectedNamespace}
	}

	return a.fetchReleases()
}

func (a *App) deleteRelease() tea.Cmd {
	// Use the stored release name and namespace from the confirmation dialog
	releaseName := a.deleteReleaseName
	releaseNS := a.deleteReleaseNS
	namespace := a.selectedNamespace

	if releaseName == "" {
		return nil
	}

	return func() tea.Msg {
		err := a.helmClient.Uninstall(releaseName, releaseNS)
		if err != nil {
			return errMsg{err: err}
		}
		releases, err := a.helmClient.ListReleases(namespace)
		if err != nil {
			return errMsg{err: err}
		}

		// Collect unique namespaces
		nsMap := make(map[string]bool)
		for _, r := range releases {
			nsMap[r.Namespace] = true
		}
		namespaces := make([]string, 0, len(nsMap))
		for ns := range nsMap {
			namespaces = append(namespaces, ns)
		}

		items := make([]list.Item, len(releases))
		for i, r := range releases {
			items[i] = releaseItem{release: r}
		}
		return releasesMsg{items: items, namespaces: namespaces}
	}
}

// releaseItem implements list.Item interface for the bubbles list component
type releaseItem struct {
	release helm.Release
}

func (r releaseItem) Title() string {
	return r.release.Name
}

func (r releaseItem) Description() string {
	status := RenderStatus(r.release.Status)
	return r.release.Namespace + " " + Icons.Tab + " " + r.release.Chart + " " + Icons.Tab + " " + status
}

func (r releaseItem) FilterValue() string {
	return r.release.Name
}

// releaseDelegate is a custom delegate that shows checkboxes for selected releases
type releaseDelegate struct {
	list.DefaultDelegate
	selectedReleases *map[string]bool
}

func newReleaseDelegate(selectedReleases *map[string]bool) releaseDelegate {
	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.
		Foreground(lipgloss.Color("0")).
		Background(DefaultTheme.Primary).
		Bold(true).
		Padding(0, 1)
	delegate.Styles.SelectedDesc = delegate.Styles.SelectedDesc.
		Foreground(lipgloss.Color("0")).
		Background(DefaultTheme.Primary).
		Padding(0, 1)

	return releaseDelegate{
		DefaultDelegate:  delegate,
		selectedReleases: selectedReleases,
	}
}

func (d releaseDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	ri, ok := item.(releaseItem)
	if !ok {
		return
	}

	// Check if this release is selected
	key := ri.release.Namespace + "/" + ri.release.Name
	isSelected := (*d.selectedReleases)[key]

	// Build title with checkbox prefix
	var title string
	if isSelected {
		title = lipgloss.NewStyle().Foreground(DefaultTheme.Success).Bold(true).Render("✓ ") + ri.Title()
	} else {
		title = "  " + ri.Title()
	}
	desc := ri.Description()

	// Check if this is the cursor position
	isCursor := index == m.Index()

	// Apply styles
	if isCursor {
		title = d.Styles.SelectedTitle.Render(title)
		desc = d.Styles.SelectedDesc.Render(desc)
	} else {
		title = d.Styles.NormalTitle.Render(title)
		desc = d.Styles.NormalDesc.Render(desc)
	}

	fmt.Fprintf(w, "%s\n%s", title, desc)
}

// submitFeatureRequest sends the feature request to the configured webhook.
// Webhook URL and API key are read from config (supports env var override).
func (a *App) submitFeatureRequest(text string) tea.Cmd {
	context := a.helmClient.GetCurrentContext()
	webhookURL := a.config.GetFeatureRequestURL()
	apiKey := a.config.GetFeatureRequestAPIKey()

	return func() tea.Msg {
		// Check if feature request is enabled
		if webhookURL == "" {
			return featureRequestResultMsg{err: fmt.Errorf("feature request webhook not configured")}
		}

		// Generate machine fingerprint for rate limiting
		hostname, _ := os.Hostname()
		username := os.Getenv("USER")
		if username == "" {
			username = os.Getenv("USERNAME") // Windows fallback
		}
		fingerprint := sha256.Sum256([]byte(hostname + ":" + username))
		fingerprintHex := hex.EncodeToString(fingerprint[:16]) // First 16 bytes

		// Build the payload
		payload := map[string]interface{}{
			"feature_request": text,
			"timestamp":       time.Now().UTC().Format(time.RFC3339),
			"context":         context,
			"os":              runtime.GOOS,
			"arch":            runtime.GOARCH,
			"fingerprint":     fingerprintHex,
		}

		jsonData, err := json.Marshal(payload)
		if err != nil {
			return featureRequestResultMsg{err: err}
		}

		// Create HTTP request with configured URL
		req, err := http.NewRequest("POST", webhookURL, bytes.NewBuffer(jsonData))
		if err != nil {
			return featureRequestResultMsg{err: err}
		}
		req.Header.Set("Content-Type", "application/json")
		if apiKey != "" {
			req.Header.Set("Authorization", "Bearer "+apiKey)
		}
		req.Header.Set("X-HelmX-Fingerprint", fingerprintHex)

		// Send with timeout
		client := &http.Client{Timeout: HTTPRequestTimeout}
		resp, err := client.Do(req)
		if err != nil {
			return featureRequestResultMsg{err: err}
		}
		defer resp.Body.Close()

		// Handle rate limiting from server
		if resp.StatusCode == 429 {
			return featureRequestResultMsg{err: fmt.Errorf("rate limited - please try again later")}
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return featureRequestResultMsg{err: fmt.Errorf("server error: %d", resp.StatusCode)}
		}

		return featureRequestResultMsg{err: nil}
	}
}

// renderFeatureRequestDialog renders the feature request dialog
func (a *App) renderFeatureRequestDialog() string {
	dialogStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(DefaultTheme.Primary).
		Padding(1, 2).
		Width(70)

	var content strings.Builder

	content.WriteString(S.Title.Render("💡 Request a Feature") + "\n\n")

	if a.featureRequestSuccess {
		content.WriteString(S.Success.Render(Icons.Check+" Feature request submitted successfully!") + "\n\n")
		content.WriteString(S.Muted.Render("Press any key to close"))
	} else if a.featureRequestSending {
		content.WriteString(S.Muted.Render("Sending request...") + "\n")
	} else {
		content.WriteString(S.Muted.Render("Describe the feature you'd like to see in helmx:") + "\n\n")
		content.WriteString(a.featureRequestInput.View() + "\n\n")

		if a.featureRequestError != nil {
			content.WriteString(S.Error.Render(Icons.Cross+" Failed to submit: "+a.featureRequestError.Error()) + "\n\n")
		}

		content.WriteString(S.Muted.Render("Enter:submit  Esc:cancel"))
	}

	dialog := dialogStyle.Render(content.String())

	return lipgloss.Place(a.width, a.height, lipgloss.Center, lipgloss.Center, dialog)
}

// openBulkUpgradeDialog initializes the bulk upgrade dialog with selected releases
func (a *App) openBulkUpgradeDialog() {
	a.showBulkUpgrade = true
	a.bulkUpgradeQueue = make([]BulkUpgradeItem, 0, len(a.selectedReleases))
	a.bulkUpgradeIdx = 0
	a.bulkUpgradeInProgress = false
	a.bulkUpgradeError = nil

	// Build queue from selected releases
	for _, item := range a.releases.Items() {
		ri := item.(releaseItem)
		key := ri.release.Namespace + "/" + ri.release.Name
		if a.selectedReleases[key] {
			a.bulkUpgradeQueue = append(a.bulkUpgradeQueue, BulkUpgradeItem{
				Name:      ri.release.Name,
				Namespace: ri.release.Namespace,
				Chart:     ri.release.Chart,
				Status:    BulkUpgradePending,
			})
		}
	}
}

// updateBulkUpgradeDialog handles keyboard input in the bulk upgrade dialog
func (a *App) updateBulkUpgradeDialog(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Don't allow closing while upgrade is in progress
	if a.bulkUpgradeInProgress {
		return a, nil
	}

	switch msg.String() {
	case "esc", "q":
		a.showBulkUpgrade = false
		// Always clear selections when closing bulk upgrade dialog
		clear(a.selectedReleases)
		return a, a.fetchReleases()
	case "enter", "y", "Y":
		if !a.bulkUpgradeInProgress && a.bulkUpgradeError == nil {
			// Check if all are done
			allDone := true
			for _, item := range a.bulkUpgradeQueue {
				if item.Status == BulkUpgradePending {
					allDone = false
					break
				}
			}
			if allDone {
				// Close dialog
				a.showBulkUpgrade = false
				clear(a.selectedReleases)
				return a, a.fetchReleases()
			}
			// Start bulk upgrade
			a.bulkUpgradeInProgress = true
			a.bulkUpgradeIdx = 0
			return a, a.upgradeNextRelease()
		}
		return a, nil
	}
	return a, nil
}

// upgradeNextRelease upgrades the next release in the queue
func (a *App) upgradeNextRelease() tea.Cmd {
	if a.bulkUpgradeIdx >= len(a.bulkUpgradeQueue) {
		return nil
	}

	item := &a.bulkUpgradeQueue[a.bulkUpgradeIdx]
	item.Status = BulkUpgradeInProgress
	idx := a.bulkUpgradeIdx

	name := item.Name
	ns := item.Namespace

	return func() tea.Msg {
		// Get current values for this release (user-supplied values only to avoid conflicts)
		values, err := a.helmClient.GetUserValues(name, ns)
		if err != nil {
			return bulkUpgradeProgressMsg{idx: idx, status: BulkUpgradeFailed, err: err}
		}

		// Upgrade with same values (this will use latest chart version if repo was updated)
		_, err = a.helmClient.UpgradeValues(name, ns, values)
		if err != nil {
			return bulkUpgradeProgressMsg{idx: idx, status: BulkUpgradeFailed, err: err}
		}

		return bulkUpgradeProgressMsg{idx: idx, status: BulkUpgradeSuccess, err: nil}
	}
}

// renderBulkUpgradeDialog renders the bulk upgrade dialog
func (a *App) renderBulkUpgradeDialog() string {
	dialogWidth := 70

	dialogStyle := lipgloss.NewStyle().
		Border(lipgloss.DoubleBorder()).
		BorderForeground(DefaultTheme.Primary).
		Padding(1, 2).
		Width(dialogWidth)

	var content strings.Builder

	content.WriteString(S.Title.Render(Icons.Upgrade+" Bulk Upgrade Releases") + "\n\n")

	// Show queue with status
	for i, item := range a.bulkUpgradeQueue {
		var statusIcon string
		var style lipgloss.Style

		switch item.Status {
		case BulkUpgradePending:
			statusIcon = Icons.Pending
			style = S.Muted
		case BulkUpgradeInProgress:
			statusIcon = "⏳"
			style = S.Subtitle
		case BulkUpgradeSuccess:
			statusIcon = Icons.Check
			style = S.Success
		case BulkUpgradeFailed:
			statusIcon = Icons.Cross
			style = S.Error
		}

		line := fmt.Sprintf("%s %s (%s)", statusIcon, item.Name, item.Namespace)
		content.WriteString(style.Render(line))

		if item.Error != nil {
			content.WriteString("\n   " + S.Error.Render(item.Error.Error()))
		}

		if i < len(a.bulkUpgradeQueue)-1 {
			content.WriteString("\n")
		}
	}

	content.WriteString("\n\n")

	// Show progress or prompts
	if a.bulkUpgradeInProgress {
		progress := fmt.Sprintf("Upgrading %d of %d...", a.bulkUpgradeIdx+1, len(a.bulkUpgradeQueue))
		content.WriteString(S.Subtitle.Render(progress))
	} else if a.bulkUpgradeError != nil {
		content.WriteString(S.Error.Render("Upgrade stopped due to error. Press Esc to close."))
	} else {
		// Check if all done
		allDone := true
		for _, item := range a.bulkUpgradeQueue {
			if item.Status == BulkUpgradePending {
				allDone = false
				break
			}
		}
		if allDone && len(a.bulkUpgradeQueue) > 0 {
			content.WriteString(S.Success.Render(fmt.Sprintf("All %d releases upgraded successfully!", len(a.bulkUpgradeQueue))))
			content.WriteString("\n" + S.Muted.Render("Press Enter or Esc to close"))
		} else {
			content.WriteString(S.Muted.Render(fmt.Sprintf("Upgrade %d releases with current values?\n", len(a.bulkUpgradeQueue))))
			content.WriteString(S.Muted.Render("[Enter] Start  [Esc] Cancel"))
		}
	}

	dialog := dialogStyle.Render(content.String())
	return lipgloss.Place(a.width, a.height, lipgloss.Center, lipgloss.Center, dialog)
}

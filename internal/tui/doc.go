// Package tui implements the terminal user interface using the Bubble Tea framework.
//
// This package follows the Elm Architecture pattern: Model → Update → View.
// The TUI is organized around "modes" that represent different views, each with
// its own state and update logic.
//
// # Architecture Overview
//
// App is the root model that orchestrates all views. It owns:
//   - The current Mode (which view is active)
//   - Sub-views for each mode (ReleaseDetail, ExploreView, ReposView, etc.)
//   - Global state (terminal dimensions, error handling, keybindings)
//
// # Mode-Based Navigation
//
// The application uses modes to switch between different views:
//
//	ModeReleases      - Main list view of installed Helm releases
//	ModeReleaseDetail - Detailed view of a single release (4 panes)
//	ModeExplore       - Chart discovery and installation
//	ModeRepos         - Repository management
//	ModeSettings      - Application settings and context switching
//	ModeWelcome       - Welcome/about page
//
// Tab keys (1-4) switch between main modes. Enter typically drills down into
// detail views. Escape returns to the previous mode.
//
// # Message Pattern
//
// Async operations (Helm API calls, file I/O) use the tea.Cmd pattern:
//
//  1. User action triggers a tea.Cmd that performs the async work
//  2. The Cmd returns a typed message (e.g., releasesLoadedMsg, historyLoadedMsg)
//  3. Update() handles the message and updates state accordingly
//
// This pattern keeps the UI responsive during long-running operations.
//
// # Pane Focus System
//
// Multi-pane views (like ReleaseDetail) track an activePane enum to route
// keyboard input to the correct component. Tab/Shift+Tab cycle between panes.
// Each pane may have its own viewport for scrolling content.
//
// # Dialog Stacking
//
// Views can show modal dialogs by setting boolean flags (e.g., showUpgradeConfirm,
// showDeleteConfirm). When a dialog is visible, keyboard input is routed to
// the dialog handler first. Dialogs are dismissed with Escape or completion.
//
// # Styling
//
// All styling is centralized in styles.go:
//   - S: Global Styles instance with pre-configured lipgloss styles
//   - DefaultTheme: Current active color palette
//   - Themes: Map of available themes
//   - SetTheme(): Switches theme and updates global styles
//
// # Key Files
//
//	app.go           - Root model, mode switching, global keybindings
//	release_detail.go - 4-pane detail view with upgrade/rollback workflows
//	explore.go       - Chart search, preview, and installation
//	repos.go         - Repository management
//	settings.go      - Settings and context switcher
//	styles.go        - Themes, icons, and styling utilities
//	diff.go          - LCS-based diff computation and rendering
//	values_editor.go - YAML editor with validation
//	help.go          - Help overlay
//	spinner.go       - Loading indicator
package tui

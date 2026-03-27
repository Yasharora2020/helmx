// Package config manages application configuration persistence.
//
// Configuration is stored in a YAML file at ~/.config/helmx/config.yaml (or the
// platform-specific config directory). The package handles loading, saving,
// and providing sensible defaults when no config file exists.
//
// # Configuration Options
//
// The Config struct contains all user-configurable settings:
//
//	ChartRegistries      - List of chart registry URLs for searching (default: Artifact Hub)
//	DefaultNamespace     - Default namespace filter for release listing
//	Editor               - Preferred editor for values editing (falls back to $EDITOR, then vim)
//	Theme                - Color theme name (default, dracula, nord, etc.)
//	SecurityScanEnabled  - Whether Trivy security scanning is enabled
//	ShowWelcomeOnStart   - Whether to show welcome page on startup
//
// # Usage
//
//	// Load configuration (creates defaults if file doesn't exist)
//	cfg, err := config.Load()
//
//	// Modify settings
//	cfg.Theme = "dracula"
//	cfg.DefaultNamespace = "production"
//
//	// Persist changes
//	err = cfg.Save()
//
// # File Location
//
// The config file path is determined by os.UserConfigDir():
//   - Linux: ~/.config/helmx/config.yaml
//   - macOS: ~/Library/Application Support/helmx/config.yaml
//   - Windows: %AppData%/helmx/config.yaml
//
// If UserConfigDir() fails, falls back to ~/.config/helmx/config.yaml.
//
// # Thread Safety
//
// Config operations are NOT thread-safe. The TUI typically loads config once
// at startup and saves only in response to explicit user actions.
package config

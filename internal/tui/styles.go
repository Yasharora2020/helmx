package tui

import (
	"fmt"
	"sync"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// Theme defines the color palette
type Theme struct {
	Primary     lipgloss.Color
	Secondary   lipgloss.Color
	Accent      lipgloss.Color
	Success     lipgloss.Color
	Warning     lipgloss.Color
	Error       lipgloss.Color
	Muted       lipgloss.Color
	Text        lipgloss.Color
	TextDim     lipgloss.Color
	Background  lipgloss.Color
	Border      lipgloss.Color
	BorderFocus lipgloss.Color
}

// Available themes
var Themes = map[string]Theme{
	"default": {
		Primary:     lipgloss.Color("205"), // Pink/magenta
		Secondary:   lipgloss.Color("39"),  // Blue
		Accent:      lipgloss.Color("214"), // Orange
		Success:     lipgloss.Color("82"),  // Green
		Warning:     lipgloss.Color("214"), // Orange
		Error:       lipgloss.Color("196"), // Red
		Muted:       lipgloss.Color("241"), // Gray
		Text:        lipgloss.Color("252"), // Light gray
		TextDim:     lipgloss.Color("245"), // Dimmer gray
		Background:  lipgloss.Color("236"), // Dark gray
		Border:      lipgloss.Color("240"), // Border gray
		BorderFocus: lipgloss.Color("205"), // Focused border (pink)
	},
	"dracula": {
		Primary:     lipgloss.Color("#bd93f9"), // Purple
		Secondary:   lipgloss.Color("#8be9fd"), // Cyan
		Accent:      lipgloss.Color("#ffb86c"), // Orange
		Success:     lipgloss.Color("#50fa7b"), // Green
		Warning:     lipgloss.Color("#ffb86c"), // Orange
		Error:       lipgloss.Color("#ff5555"), // Red
		Muted:       lipgloss.Color("#6272a4"), // Comment
		Text:        lipgloss.Color("#f8f8f2"), // Foreground
		TextDim:     lipgloss.Color("#bfbfbf"), // Dimmer
		Background:  lipgloss.Color("#282a36"), // Background
		Border:      lipgloss.Color("#44475a"), // Current line
		BorderFocus: lipgloss.Color("#bd93f9"), // Purple
	},
	"nord": {
		Primary:     lipgloss.Color("#88c0d0"), // Frost
		Secondary:   lipgloss.Color("#81a1c1"), // Frost 2
		Accent:      lipgloss.Color("#ebcb8b"), // Yellow
		Success:     lipgloss.Color("#a3be8c"), // Green
		Warning:     lipgloss.Color("#ebcb8b"), // Yellow
		Error:       lipgloss.Color("#bf616a"), // Red
		Muted:       lipgloss.Color("#4c566a"), // Polar night
		Text:        lipgloss.Color("#eceff4"), // Snow storm
		TextDim:     lipgloss.Color("#d8dee9"), // Snow storm 2
		Background:  lipgloss.Color("#2e3440"), // Polar night
		Border:      lipgloss.Color("#3b4252"), // Polar night 2
		BorderFocus: lipgloss.Color("#88c0d0"), // Frost
	},
	"catppuccin": {
		Primary:     lipgloss.Color("#cba6f7"), // Mauve
		Secondary:   lipgloss.Color("#89b4fa"), // Blue
		Accent:      lipgloss.Color("#fab387"), // Peach
		Success:     lipgloss.Color("#a6e3a1"), // Green
		Warning:     lipgloss.Color("#f9e2af"), // Yellow
		Error:       lipgloss.Color("#f38ba8"), // Red
		Muted:       lipgloss.Color("#6c7086"), // Overlay0
		Text:        lipgloss.Color("#cdd6f4"), // Text
		TextDim:     lipgloss.Color("#a6adc8"), // Subtext0
		Background:  lipgloss.Color("#1e1e2e"), // Base
		Border:      lipgloss.Color("#313244"), // Surface0
		BorderFocus: lipgloss.Color("#cba6f7"), // Mauve
	},
	"gruvbox": {
		Primary:     lipgloss.Color("#d79921"), // Yellow
		Secondary:   lipgloss.Color("#458588"), // Blue
		Accent:      lipgloss.Color("#d65d0e"), // Orange
		Success:     lipgloss.Color("#98971a"), // Green
		Warning:     lipgloss.Color("#d79921"), // Yellow
		Error:       lipgloss.Color("#cc241d"), // Red
		Muted:       lipgloss.Color("#928374"), // Gray
		Text:        lipgloss.Color("#ebdbb2"), // Foreground
		TextDim:     lipgloss.Color("#a89984"), // Gray
		Background:  lipgloss.Color("#282828"), // Background
		Border:      lipgloss.Color("#3c3836"), // Dark1
		BorderFocus: lipgloss.Color("#d79921"), // Yellow
	},
	"tokyo-night": {
		Primary:     lipgloss.Color("#7aa2f7"), // Blue
		Secondary:   lipgloss.Color("#bb9af7"), // Purple
		Accent:      lipgloss.Color("#ff9e64"), // Orange
		Success:     lipgloss.Color("#9ece6a"), // Green
		Warning:     lipgloss.Color("#e0af68"), // Yellow
		Error:       lipgloss.Color("#f7768e"), // Red
		Muted:       lipgloss.Color("#565f89"), // Comment
		Text:        lipgloss.Color("#c0caf5"), // Foreground
		TextDim:     lipgloss.Color("#a9b1d6"), // Dimmer
		Background:  lipgloss.Color("#1a1b26"), // Background
		Border:      lipgloss.Color("#292e42"), // Dark
		BorderFocus: lipgloss.Color("#7aa2f7"), // Blue
	},
}

// ThemeNames returns all available theme names
var ThemeNames = []string{"default", "dracula", "nord", "catppuccin", "gruvbox", "tokyo-night"}

// DefaultTheme is the current active theme
var DefaultTheme = Themes["default"]

// CurrentThemeName tracks the active theme name
var CurrentThemeName = "default"

// themeMu guards DefaultTheme, CurrentThemeName, and S from concurrent access
var themeMu sync.RWMutex

// SetTheme changes the active theme and updates global styles
func SetTheme(name string) bool {
	theme, ok := Themes[name]
	if !ok {
		return false
	}
	themeMu.Lock()
	defer themeMu.Unlock()
	DefaultTheme = theme
	CurrentThemeName = name
	S = NewStyles(theme)
	return true
}

// GetTheme returns a copy of the current active theme under a read lock
func GetTheme() Theme {
	themeMu.RLock()
	defer themeMu.RUnlock()
	return DefaultTheme
}

// GetStyles returns a copy of the current global styles under a read lock
func GetStyles() Styles {
	themeMu.RLock()
	defer themeMu.RUnlock()
	return S
}

// GetThemeName returns the current active theme name under a read lock
func GetThemeName() string {
	themeMu.RLock()
	defer themeMu.RUnlock()
	return CurrentThemeName
}

// Icons for visual elements
var Icons = struct {
	Helm       string
	Release    string
	Deployed   string
	Failed     string
	Pending    string
	Chart      string
	Repo       string
	History    string
	Values     string
	Search     string
	Add        string
	Delete     string
	Upgrade    string
	Rollback   string
	Refresh    string
	Check      string
	Cross      string
	Arrow      string
	ArrowRight string
	Bullet     string
	Tab        string
	Settings   string
	Edit       string
	Lock       string
	Unlock     string
	File       string
}{
	Helm:       "⎈",
	Release:    "📦",
	Deployed:   "●",
	Failed:     "✗",
	Pending:    "◐",
	Chart:      "📋",
	Repo:       "🗄️",
	History:    "📜",
	Values:     "⚙️",
	Search:     "🔍",
	Add:        "+",
	Delete:     "×",
	Upgrade:    "↑",
	Rollback:   "↩",
	Refresh:    "↻",
	Check:      "✓",
	Cross:      "✗",
	Arrow:      "▸",
	ArrowRight: "→",
	Bullet:     "•",
	Tab:        "│",
	Settings:   "⚙",
	Edit:       "✎",
	Lock:       "🔒",
	Unlock:     "🔓",
	File:       "📄",
}

// Operation timeouts and intervals
const (
	HTTPRequestTimeout = 10 * time.Second
)

// HelmXLogo is the ASCII art logo for the app
const HelmXLogo = `
██╗  ██╗███████╗██╗     ███╗   ███╗██╗  ██╗
██║  ██║██╔════╝██║     ████╗ ████║╚██╗██╔╝
███████║█████╗  ██║     ██╔████╔██║ ╚███╔╝
██╔══██║██╔══╝  ██║     ██║╚██╔╝██║ ██╔██╗
██║  ██║███████╗███████╗██║ ╚═╝ ██║██╔╝ ██╗
╚═╝  ╚═╝╚══════╝╚══════╝╚═╝     ╚═╝╚═╝  ╚═╝`

// Styles holds reusable style definitions
type Styles struct {
	// Layout
	TabBar      lipgloss.Style
	TabActive   lipgloss.Style
	TabInactive lipgloss.Style
	Breadcrumb  lipgloss.Style
	MainContent lipgloss.Style
	StatusBar   lipgloss.Style

	// Borders
	BorderNormal lipgloss.Style
	BorderFocus  lipgloss.Style
	BorderDim    lipgloss.Style

	// Pane headers
	PaneHeaderActive   lipgloss.Style
	PaneHeaderInactive lipgloss.Style
	PaneNav            lipgloss.Style
	PaneNavActive      lipgloss.Style
	PaneNavInactive    lipgloss.Style

	// Text
	Title    lipgloss.Style
	Subtitle lipgloss.Style
	Label    lipgloss.Style
	Value    lipgloss.Style
	Muted    lipgloss.Style
	Dimmed   lipgloss.Style
	Success  lipgloss.Style
	Warning  lipgloss.Style
	Error    lipgloss.Style

	// Interactive
	Selected    lipgloss.Style
	Highlighted lipgloss.Style
	Button      lipgloss.Style
	ButtonFocus lipgloss.Style

	// Status indicators
	StatusDeployed lipgloss.Style
	StatusFailed   lipgloss.Style
	StatusPending  lipgloss.Style

	// Scrollbar
	ScrollIndicator lipgloss.Style
}

// NewStyles creates styles based on a theme
func NewStyles(theme Theme) Styles {
	return Styles{
		// Layout
		TabBar: lipgloss.NewStyle().
			Background(theme.Background).
			Padding(0, 1),

		TabActive: lipgloss.NewStyle().
			Bold(true).
			Foreground(theme.Primary).
			Background(lipgloss.Color("238")).
			Padding(0, 2).
			Border(lipgloss.Border{Bottom: "▀"}, false, false, true, false).
			BorderForeground(theme.Primary),

		TabInactive: lipgloss.NewStyle().
			Foreground(theme.TextDim).
			Padding(0, 2),

		Breadcrumb: lipgloss.NewStyle().
			Foreground(theme.Muted).
			Padding(0, 1),

		MainContent: lipgloss.NewStyle().
			Padding(1, 0),

		StatusBar: lipgloss.NewStyle().
			Foreground(theme.Muted).
			Padding(0, 1),

		// Borders
		BorderNormal: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(theme.Border).
			Padding(0, 1),

		BorderFocus: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(theme.BorderFocus).
			Padding(0, 1),

		BorderDim: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("237")).
			Padding(0, 1),

		// Pane headers
		PaneHeaderActive: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("0")).
			Background(theme.Primary).
			Padding(0, 1),

		PaneHeaderInactive: lipgloss.NewStyle().
			Foreground(theme.TextDim).
			Padding(0, 1),

		PaneNav: lipgloss.NewStyle().
			Foreground(theme.Muted).
			Padding(0, 1),

		PaneNavActive: lipgloss.NewStyle().
			Bold(true).
			Foreground(theme.Primary),

		PaneNavInactive: lipgloss.NewStyle().
			Foreground(theme.Muted),

		// Text
		Title: lipgloss.NewStyle().
			Bold(true).
			Foreground(theme.Primary),

		Subtitle: lipgloss.NewStyle().
			Bold(true).
			Foreground(theme.Text),

		Label: lipgloss.NewStyle().
			Foreground(theme.TextDim),

		Value: lipgloss.NewStyle().
			Foreground(theme.Text),

		Muted: lipgloss.NewStyle().
			Foreground(theme.Muted),

		Dimmed: lipgloss.NewStyle().
			Foreground(lipgloss.Color("239")),

		Success: lipgloss.NewStyle().
			Foreground(theme.Success),

		Warning: lipgloss.NewStyle().
			Foreground(theme.Warning),

		Error: lipgloss.NewStyle().
			Foreground(theme.Error),

		// Interactive
		Selected: lipgloss.NewStyle().
			Background(theme.Primary).
			Foreground(lipgloss.Color("0")),

		Highlighted: lipgloss.NewStyle().
			Foreground(theme.Primary).
			Bold(true),

		Button: lipgloss.NewStyle().
			Padding(0, 2).
			Background(theme.Border).
			Foreground(theme.Text),

		ButtonFocus: lipgloss.NewStyle().
			Padding(0, 2).
			Background(theme.Primary).
			Foreground(lipgloss.Color("0")).
			Bold(true),

		// Status indicators
		StatusDeployed: lipgloss.NewStyle().
			Foreground(theme.Success),

		StatusFailed: lipgloss.NewStyle().
			Foreground(theme.Error),

		StatusPending: lipgloss.NewStyle().
			Foreground(theme.Warning),

		// Scrollbar
		ScrollIndicator: lipgloss.NewStyle().
			Foreground(theme.Muted),
	}
}

// Global styles instance
var S = NewStyles(DefaultTheme)

// Layout helpers

// TabBarView renders the tab bar
func TabBarView(tabs []string, activeIndex int, width int) string {
	theme := GetTheme()
	styles := GetStyles()

	var renderedTabs []string

	for i, tab := range tabs {
		if i == activeIndex {
			renderedTabs = append(renderedTabs, styles.TabActive.Render(tab))
		} else {
			renderedTabs = append(renderedTabs, styles.TabInactive.Render(tab))
		}
	}

	tabContent := lipgloss.JoinHorizontal(lipgloss.Bottom, renderedTabs...)

	// Add the helm icon and fill the bar
	header := lipgloss.NewStyle().
		Bold(true).
		Foreground(theme.Primary).
		Padding(0, 1).
		Render(Icons.Helm + " helmx")

	spacer := lipgloss.NewStyle().
		Width(width - lipgloss.Width(header) - lipgloss.Width(tabContent) - 2).
		Render("")

	bar := lipgloss.JoinHorizontal(lipgloss.Bottom, header, spacer, tabContent)

	return lipgloss.NewStyle().
		Background(theme.Background).
		Width(width).
		Render(bar)
}

// BreadcrumbView renders breadcrumb navigation
func BreadcrumbView(parts []string) string {
	if len(parts) == 0 {
		return ""
	}

	var rendered []string
	for i, part := range parts {
		if i == len(parts)-1 {
			// Last part is highlighted
			rendered = append(rendered, S.Highlighted.Render(part))
		} else {
			rendered = append(rendered, S.Muted.Render(part))
		}
	}

	return S.Breadcrumb.Render(
		lipgloss.JoinHorizontal(lipgloss.Center,
			joinWithSeparator(rendered, S.Muted.Render(" "+Icons.ArrowRight+" "))...),
	)
}

func joinWithSeparator(items []string, sep string) []string {
	if len(items) == 0 {
		return items
	}
	result := make([]string, 0, len(items)*2-1)
	for i, item := range items {
		result = append(result, item)
		if i < len(items)-1 {
			result = append(result, sep)
		}
	}
	return result
}

// StatusBarView renders a consistent status bar
func StatusBarView(left, right string, width int) string {
	leftContent := S.StatusBar.Render(left)
	rightContent := S.StatusBar.Render(right)

	spacerWidth := width - lipgloss.Width(leftContent) - lipgloss.Width(rightContent)
	if spacerWidth < 0 {
		spacerWidth = 0
	}
	spacer := lipgloss.NewStyle().Width(spacerWidth).Render("")

	return lipgloss.JoinHorizontal(lipgloss.Center, leftContent, spacer, rightContent)
}

// PaneLayout calculates adaptive pane sizes
type PaneLayout struct {
	LeftWidth    int
	RightWidth   int
	TopHeight    int
	BottomHeight int
}

// CalculatePaneLayout calculates pane sizes based on terminal dimensions
func CalculatePaneLayout(width, height int, preferredLeftRatio float64) PaneLayout {
	// Minimum widths
	minLeft := 30
	minRight := 40

	// Calculate based on ratio, but respect minimums
	leftWidth := int(float64(width) * preferredLeftRatio)
	if leftWidth < minLeft {
		leftWidth = minLeft
	}
	rightWidth := width - leftWidth - 4 // 4 for borders/padding
	if rightWidth < minRight {
		rightWidth = minRight
		leftWidth = width - rightWidth - 4
	}

	// Height calculations (accounting for tab bar, status bar)
	availableHeight := height - 4 // Tab bar + status bar
	topHeight := availableHeight / 2
	bottomHeight := availableHeight - topHeight

	return PaneLayout{
		LeftWidth:    leftWidth,
		RightWidth:   rightWidth,
		TopHeight:    topHeight,
		BottomHeight: bottomHeight,
	}
}

// RenderStatus returns styled status based on status string
func RenderStatus(status string) string {
	switch status {
	case "deployed":
		return S.StatusDeployed.Render(Icons.Deployed + " deployed")
	case "failed":
		return S.StatusFailed.Render(Icons.Failed + " failed")
	case "pending-install", "pending-upgrade", "pending-rollback":
		return S.StatusPending.Render(Icons.Pending + " " + status)
	default:
		return S.Muted.Render(status)
	}
}

// KeyHint formats a key hint
func KeyHint(key, desc string) string {
	return S.Highlighted.Render(key) + S.Muted.Render(":"+desc)
}

// KeyHints formats multiple key hints
func KeyHints(hints [][2]string) string {
	var parts []string
	for _, h := range hints {
		parts = append(parts, KeyHint(h[0], h[1]))
	}
	return lipgloss.JoinHorizontal(lipgloss.Center,
		joinWithSeparator(parts, S.Muted.Render("  "))...)
}

// PaneNavBar renders a navigation bar showing pane positions
// panes is a slice of pane names, activeIdx is the currently focused pane
func PaneNavBar(panes []string, activeIdx int, width int) string {
	var parts []string

	for i, name := range panes {
		indicator := "  "
		if i == activeIdx {
			indicator = Icons.Arrow + " "
			parts = append(parts, S.PaneNavActive.Render(indicator+name))
		} else {
			parts = append(parts, S.PaneNavInactive.Render(indicator+name))
		}
	}

	nav := lipgloss.JoinHorizontal(lipgloss.Center,
		joinWithSeparator(parts, S.Muted.Render("  "))...)

	return S.PaneNav.Width(width).Render("[Tab] " + nav)
}

// RenderPaneHeader renders a header with active/inactive styling
func RenderPaneHeader(icon, title string, count int, active bool) string {
	countStr := ""
	if count >= 0 {
		countStr = " (" + lipgloss.NewStyle().Render(fmt.Sprintf("%d", count)) + ")"
	}

	if active {
		return S.PaneHeaderActive.Render(icon+" "+title) + S.Muted.Render(countStr)
	}
	return S.PaneHeaderInactive.Render(icon+" "+title) + S.Muted.Render(countStr)
}

// RenderScrollHint shows scroll position indicator
func RenderScrollHint(current, total int) string {
	if total <= 0 {
		return ""
	}
	return S.ScrollIndicator.Render(fmt.Sprintf(" [%d/%d]", current+1, total))
}

// KeyBar renders the top keybindings bar (k9s style)
func KeyBar(width int) string {
	theme := GetTheme()

	keyStyle := lipgloss.NewStyle().
		Foreground(theme.Primary).
		Bold(true)

	descStyle := lipgloss.NewStyle().
		Foreground(theme.Text)

	keys := []struct {
		key  string
		desc string
	}{
		{"0", "Welcome"},
		{"1", "Releases"},
		{"2", "Explore"},
		{"3", "Repos"},
		{"4", "Settings"},
		{"5", "RBAC"},
		{"?", "Help"},
		{"q", "Quit"},
	}

	var parts []string
	for _, k := range keys {
		parts = append(parts, keyStyle.Render("<"+k.key+">")+descStyle.Render(k.desc))
	}

	content := lipgloss.JoinHorizontal(lipgloss.Center,
		joinWithSeparator(parts, "  ")...)

	return lipgloss.NewStyle().
		Background(theme.Background).
		Width(width).
		Padding(0, 1).
		Render(content)
}

// RenderLogo renders the HelmX ASCII art logo centered
func RenderLogo(width int, context, namespace string) string {
	theme := GetTheme()

	logoStyle := lipgloss.NewStyle().
		Foreground(theme.Primary).
		Bold(true)

	subtitleStyle := lipgloss.NewStyle().
		Foreground(theme.Text).
		Bold(true)

	contextStyle := lipgloss.NewStyle().
		Foreground(theme.Muted)

	logo := logoStyle.Render(HelmXLogo)
	subtitle := subtitleStyle.Render("Helm Explorer")

	// Context info
	nsDisplay := namespace
	if nsDisplay == "" {
		nsDisplay = "All"
	}
	contextInfo := contextStyle.Render(fmt.Sprintf("Context: %s │ NS: %s", context, nsDisplay))

	// Stack vertically and center
	content := lipgloss.JoinVertical(lipgloss.Center,
		logo,
		"",
		subtitle,
		contextInfo,
		"",
	)

	return lipgloss.Place(width, 12, lipgloss.Center, lipgloss.Center, content)
}

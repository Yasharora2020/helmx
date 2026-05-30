package tui

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/yasharora2020/helmx/internal/config"
	"github.com/yasharora2020/helmx/internal/helm"
)

// SettingsView handles application settings
type SettingsView struct {
	config           *config.Config
	contextManager   helm.ContextManager
	securityProvider helm.SecurityProvider
	width            int
	height           int

	// Registry list
	selectedRegistry int

	// Add registry dialog
	showAddDialog bool
	addNameInput  textinput.Model
	addURLInput   textinput.Model
	addField      int // 0=name, 1=url, 2=add, 3=cancel

	// Delete confirmation
	showDeleteConfirm bool

	// Edit other settings dialog
	showEditDialog bool
	editField      int // 0=namespace, 1=editor, 2=save, 3=cancel
	nsInput        textinput.Model
	editorInput    textinput.Model

	// Theme dialog
	showThemeDialog bool
	selectedTheme   int

	// Context dialog
	showContextDialog bool
	contexts          []helm.KubeContext
	selectedContext   int

	// Templates dialog
	showTemplatesDialog bool
	selectedTemplate    int
	templatesDeleteConf bool // Show delete confirmation for template

	// Status
	status string
	err    error
}

// NewSettingsView creates a new settings view
func NewSettingsView(cfg *config.Config, cm helm.ContextManager, sp helm.SecurityProvider) *SettingsView {
	// Add registry inputs
	addName := textinput.New()
	addName.Placeholder = "My Registry"
	addName.CharLimit = 50
	addName.Width = 40

	addURL := textinput.New()
	addURL.Placeholder = "https://charts.example.com/api/v1"
	addURL.CharLimit = 200
	addURL.Width = 50

	// Edit settings inputs
	nsInput := textinput.New()
	nsInput.Placeholder = "default"
	nsInput.CharLimit = 63
	nsInput.Width = 30

	editorInput := textinput.New()
	editorInput.Placeholder = "vim"
	editorInput.CharLimit = 50
	editorInput.Width = 30

	// Find current theme index
	themeIdx := 0
	for i, name := range ThemeNames {
		if name == cfg.Theme {
			themeIdx = i
			break
		}
	}

	return &SettingsView{
		config:           cfg,
		contextManager:   cm,
		securityProvider: sp,
		addNameInput:     addName,
		addURLInput:      addURL,
		nsInput:          nsInput,
		editorInput:      editorInput,
		selectedTheme:    themeIdx,
	}
}

// SetHelmClient updates the helm client references (context manager and security provider)
func (s *SettingsView) SetHelmClient(cm helm.ContextManager, sp helm.SecurityProvider) {
	s.contextManager = cm
	s.securityProvider = sp
}

// SetConfig updates the config reference
func (s *SettingsView) SetConfig(cfg *config.Config) {
	s.config = cfg
}

// SetSize updates dimensions
func (s *SettingsView) SetSize(width, height int) {
	s.width = width
	s.height = height
}

// HasOpenDialog returns true if any dialog is currently open.
func (s *SettingsView) HasOpenDialog() bool {
	return s.showAddDialog ||
		s.showDeleteConfirm ||
		s.showEditDialog ||
		s.showThemeDialog ||
		s.showContextDialog ||
		s.showTemplatesDialog
}

// Update handles input
func (s *SettingsView) Update(msg tea.Msg) (*SettingsView, tea.Cmd) {
	// Handle contextsLoadedMsg regardless of dialog state (needed to show dialog initially)
	if loadedMsg, ok := msg.(contextsLoadedMsg); ok {
		if loadedMsg.err != nil {
			s.err = loadedMsg.err
			return s, nil
		}
		s.contexts = loadedMsg.contexts
		s.showContextDialog = true
		// Find current context index
		for i, ctx := range s.contexts {
			if ctx.IsCurrent {
				s.selectedContext = i
				break
			}
		}
		return s, nil
	}

	// Handle add dialog
	if s.showAddDialog {
		return s.updateAddDialog(msg)
	}

	// Handle delete confirmation
	if s.showDeleteConfirm {
		return s.updateDeleteConfirm(msg)
	}

	// Handle edit dialog
	if s.showEditDialog {
		return s.updateEditDialog(msg)
	}

	// Handle theme dialog
	if s.showThemeDialog {
		return s.updateThemeDialog(msg)
	}

	// Handle context dialog
	if s.showContextDialog {
		return s.updateContextDialog(msg)
	}

	// Handle templates dialog
	if s.showTemplatesDialog {
		return s.updateTemplatesDialog(msg)
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		s.SetSize(msg.Width, msg.Height)

	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if s.selectedRegistry > 0 {
				s.selectedRegistry--
			}
		case "down", "j":
			registries := s.config.GetChartRegistries()
			if s.selectedRegistry < len(registries)-1 {
				s.selectedRegistry++
			}
		case "a":
			s.openAddDialog()
			return s, textinput.Blink
		case "d", "delete":
			registries := s.config.GetChartRegistries()
			if len(registries) > 1 { // Don't allow deleting if only one registry
				s.showDeleteConfirm = true
			} else {
				s.status = "Cannot delete the only registry"
			}
		case "e":
			s.openEditDialog()
			return s, textinput.Blink
		case "t":
			s.openThemeDialog()
		case "c":
			return s, s.openContextDialog()
		case "s":
			// Toggle security scanning
			s.config.SecurityScanEnabled = !s.config.SecurityScanEnabled
			if err := s.config.Save(); err != nil {
				s.err = err
			} else {
				if s.config.SecurityScanEnabled {
					s.status = "Security scanning enabled"
				} else {
					s.status = "Security scanning disabled"
				}
			}
		case "T":
			// Open templates management dialog
			s.openTemplatesDialog()
		case "r":
			// Reset to defaults
			s.config.ChartRegistries = []config.ChartRegistry{
				{Name: config.DefaultRegistryName, URL: config.DefaultRegistryURL},
			}
			s.config.DefaultNamespace = ""
			s.config.Editor = ""
			s.config.Theme = "default"
			s.config.SecurityScanEnabled = true
			s.selectedRegistry = 0
			s.selectedTheme = 0
			SetTheme("default")
			if err := s.config.Save(); err != nil {
				s.err = err
			} else {
				s.status = "Reset to defaults"
			}
		}

	case settingsSavedMsg:
		s.err = msg.err
		if msg.err == nil {
			s.status = "Settings saved"
		}
	}

	return s, nil
}

func (s *SettingsView) openAddDialog() {
	s.showAddDialog = true
	s.addField = 0
	s.addNameInput.SetValue("")
	s.addURLInput.SetValue("")
	s.addNameInput.Focus()
	s.status = ""
	s.err = nil
}

func (s *SettingsView) updateAddDialog(msg tea.Msg) (*SettingsView, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			s.showAddDialog = false
			return s, nil
		case "tab", "down":
			s.addField = (s.addField + 1) % 4
			s.updateAddFocus()
		case "shift+tab", "up":
			s.addField = (s.addField + 3) % 4
			s.updateAddFocus()
		case "enter":
			switch s.addField {
			case 2: // Add
				name := s.addNameInput.Value()
				url := s.addURLInput.Value()
				if name != "" && url != "" {
					s.config.AddRegistry(name, url)
					if err := s.config.Save(); err != nil {
						s.err = err
					} else {
						s.status = "Added " + name
					}
					s.showAddDialog = false
				}
			case 3: // Cancel
				s.showAddDialog = false
			}
			return s, nil
		}
	}

	// Update focused input
	var cmd tea.Cmd
	switch s.addField {
	case 0:
		s.addNameInput, cmd = s.addNameInput.Update(msg)
	case 1:
		s.addURLInput, cmd = s.addURLInput.Update(msg)
	}
	return s, cmd
}

func (s *SettingsView) updateAddFocus() {
	s.addNameInput.Blur()
	s.addURLInput.Blur()
	switch s.addField {
	case 0:
		s.addNameInput.Focus()
	case 1:
		s.addURLInput.Focus()
	}
}

func (s *SettingsView) updateDeleteConfirm(msg tea.Msg) (*SettingsView, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "y", "Y", "enter":
			s.showDeleteConfirm = false
			registries := s.config.GetChartRegistries()
			if s.selectedRegistry < len(registries) {
				reg := registries[s.selectedRegistry]
				s.config.RemoveRegistry(reg.URL)
				if err := s.config.Save(); err != nil {
					s.err = err
				} else {
					s.status = "Removed " + reg.Name
				}
				if s.selectedRegistry >= len(s.config.ChartRegistries) {
					s.selectedRegistry = len(s.config.ChartRegistries) - 1
				}
			}
		case "n", "N", "esc":
			s.showDeleteConfirm = false
		}
	}
	return s, nil
}

func (s *SettingsView) openEditDialog() {
	s.showEditDialog = true
	s.editField = 0
	s.nsInput.SetValue(s.config.DefaultNamespace)
	s.editorInput.SetValue(s.config.Editor)
	s.nsInput.Focus()
	s.status = ""
	s.err = nil
}

func (s *SettingsView) updateEditDialog(msg tea.Msg) (*SettingsView, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			s.showEditDialog = false
			return s, nil
		case "tab", "down":
			s.editField = (s.editField + 1) % 4
			s.updateEditFocus()
		case "shift+tab", "up":
			s.editField = (s.editField + 3) % 4
			s.updateEditFocus()
		case "enter":
			switch s.editField {
			case 2: // Save
				s.config.DefaultNamespace = s.nsInput.Value()
				s.config.Editor = s.editorInput.Value()
				if err := s.config.Save(); err != nil {
					s.err = err
				} else {
					s.status = "Settings saved"
				}
				s.showEditDialog = false
			case 3: // Cancel
				s.showEditDialog = false
			}
			return s, nil
		}
	}

	// Update focused input
	var cmd tea.Cmd
	switch s.editField {
	case 0:
		s.nsInput, cmd = s.nsInput.Update(msg)
	case 1:
		s.editorInput, cmd = s.editorInput.Update(msg)
	}
	return s, cmd
}

func (s *SettingsView) updateEditFocus() {
	s.nsInput.Blur()
	s.editorInput.Blur()
	switch s.editField {
	case 0:
		s.nsInput.Focus()
	case 1:
		s.editorInput.Focus()
	}
}

func (s *SettingsView) openThemeDialog() {
	s.showThemeDialog = true
	// Find current theme index
	for i, name := range ThemeNames {
		if name == s.config.Theme {
			s.selectedTheme = i
			break
		}
	}
	s.status = ""
	s.err = nil
}

func (s *SettingsView) updateThemeDialog(msg tea.Msg) (*SettingsView, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			s.showThemeDialog = false
			return s, nil
		case "up", "k":
			if s.selectedTheme > 0 {
				s.selectedTheme--
			}
		case "down", "j":
			if s.selectedTheme < len(ThemeNames)-1 {
				s.selectedTheme++
			}
		case "enter":
			themeName := ThemeNames[s.selectedTheme]
			s.config.Theme = themeName
			SetTheme(themeName)
			if err := s.config.Save(); err != nil {
				s.err = err
			} else {
				s.status = "Theme changed to " + themeName
			}
			s.showThemeDialog = false
			return s, nil
		}
	}
	return s, nil
}

// openContextDialog opens the context switching dialog
func (s *SettingsView) openContextDialog() tea.Cmd {
	return func() tea.Msg {
		if s.contextManager == nil {
			return contextsLoadedMsg{err: fmt.Errorf("context manager not initialized")}
		}
		contexts, err := s.contextManager.ListContexts()
		return contextsLoadedMsg{contexts: contexts, err: err}
	}
}

func (s *SettingsView) updateContextDialog(msg tea.Msg) (*SettingsView, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			s.showContextDialog = false
			return s, nil
		case "up", "k":
			if s.selectedContext > 0 {
				s.selectedContext--
			}
		case "down", "j":
			if s.selectedContext < len(s.contexts)-1 {
				s.selectedContext++
			}
		case "enter":
			if s.selectedContext < len(s.contexts) {
				ctx := s.contexts[s.selectedContext]
				if err := s.contextManager.SwitchContext(ctx.Name); err != nil {
					s.err = err
				} else {
					s.status = "Switched to context: " + ctx.Name
				}
				s.showContextDialog = false
				return s, contextSwitchedCmd(ctx.Name)
			}
		}
	}
	return s, nil
}

// View renders the settings view
func (s *SettingsView) View() string {
	if s.showAddDialog {
		return s.renderAddDialog()
	}
	if s.showDeleteConfirm {
		return s.renderDeleteConfirm()
	}
	if s.showEditDialog {
		return s.renderEditDialog()
	}
	if s.showThemeDialog {
		return s.renderThemeDialog()
	}
	if s.showContextDialog {
		return s.renderContextDialog()
	}
	if s.showTemplatesDialog {
		return s.renderTemplatesDialog()
	}
	return s.renderMainView()
}

func (s *SettingsView) renderMainView() string {
	var b strings.Builder

	b.WriteString(S.Title.Render(Icons.Settings+" Settings") + "\n\n")

	// Chart Registries section
	b.WriteString(S.Highlighted.Render("Chart Registries") + "\n")
	b.WriteString(S.Muted.Render("Search these registries when exploring charts") + "\n\n")

	registries := s.config.GetChartRegistries()
	for i, reg := range registries {
		name := reg.Name
		url := reg.URL

		// Truncate URL if too long
		maxURLLen := s.width - 30
		if maxURLLen > 60 {
			maxURLLen = 60
		}
		if len(url) > maxURLLen {
			url = url[:maxURLLen-3] + "..."
		}

		var line string
		if i == s.selectedRegistry {
			line = S.Selected.Render(Icons.Arrow + " " + name)
		} else {
			line = S.Value.Render("  " + name)
		}
		line += "\n" + S.Muted.Render("    "+url)
		b.WriteString(line + "\n")
	}

	// Other settings
	b.WriteString("\n" + S.Highlighted.Render("Other Settings") + "\n\n")

	labelStyle := lipgloss.NewStyle().Foreground(DefaultTheme.Muted).Width(20)
	valueStyle := lipgloss.NewStyle().Foreground(DefaultTheme.Text)

	// Default Namespace
	ns := s.config.DefaultNamespace
	if ns == "" {
		ns = "(all namespaces)"
	}
	b.WriteString(labelStyle.Render("Default Namespace:"))
	b.WriteString(valueStyle.Render(ns) + "\n")

	// Editor
	editor := s.config.GetEditor()
	editorSource := ""
	if s.config.Editor != "" {
		editorSource = " (configured)"
	} else if os.Getenv("EDITOR") != "" {
		editorSource = " ($EDITOR)"
	} else {
		editorSource = " (fallback)"
	}
	b.WriteString(labelStyle.Render("Editor:"))
	b.WriteString(valueStyle.Render(editor+editorSource) + "\n")

	// Theme
	themeName := s.config.Theme
	if themeName == "" {
		themeName = "default"
	}
	b.WriteString(labelStyle.Render("Theme:"))
	b.WriteString(valueStyle.Render(themeName) + "\n")

	// Helm Secrets status
	secretsStatus := Icons.Cross + " Not installed"
	if s.securityProvider != nil && s.securityProvider.HasSecretsSupport() {
		secretsClient := s.securityProvider.GetSecretsClient()
		version := secretsClient.GetVersion()
		if version != "" {
			secretsStatus = Icons.Check + " v" + version
		} else {
			secretsStatus = Icons.Check + " Available"
		}
	}
	b.WriteString(labelStyle.Render("Helm Secrets:"))
	b.WriteString(valueStyle.Render(secretsStatus) + "\n")

	// Current Context
	currentContext := "unknown"
	if s.contextManager != nil {
		currentContext = s.contextManager.GetCurrentContext()
	}
	b.WriteString(labelStyle.Render("K8s Context:"))
	b.WriteString(valueStyle.Render(currentContext) + "\n")

	// Security Scanning status
	secScanStatus := Icons.Check + " Enabled"
	if !s.config.SecurityScanEnabled {
		secScanStatus = Icons.Cross + " Disabled"
	}
	// Check if Trivy is available
	trivyAvailable := ""
	if s.securityProvider != nil && s.securityProvider.HasTrivySupport() {
		trivyClient := s.securityProvider.GetTrivyClient()
		if version := trivyClient.GetVersion(); version != "" {
			trivyAvailable = " (Trivy: " + version + ")"
		} else {
			trivyAvailable = " (Trivy available)"
		}
	} else {
		trivyAvailable = " (Trivy not installed)"
	}
	b.WriteString(labelStyle.Render("Security Scan:"))
	b.WriteString(valueStyle.Render(secScanStatus+trivyAvailable) + "\n")

	// Install Templates section
	b.WriteString("\n" + S.Highlighted.Render("Install Templates") + "\n")
	templates, _ := config.ListAllTemplates()
	if len(templates) == 0 {
		b.WriteString(S.Muted.Render("No saved templates. Use 'T' in install dialog to save.") + "\n")
	} else {
		b.WriteString(S.Muted.Render(fmt.Sprintf("%d saved templates. Press 'T' to manage.", len(templates))) + "\n")
	}

	// Config file path
	b.WriteString("\n")
	b.WriteString(S.Muted.Render("Config: "+config.ConfigPath()) + "\n")

	content := S.BorderFocus.Width(s.width - 4).Height(s.height - 10).Render(b.String())

	// Status bar
	var statusLeft string
	if s.status != "" {
		statusLeft = S.Success.Render(Icons.Check + " " + s.status)
	}
	if s.err != nil {
		statusLeft = S.Error.Render(Icons.Cross + " " + s.err.Error())
	}

	hints := [][2]string{{"a", "add"}, {"d", "delete"}, {"e", "edit"}, {"t", "theme"}, {"c", "context"}, {"s", "scan"}, {"T", "templates"}, {"r", "reset"}}
	statusRight := KeyHints(hints)
	statusBar := StatusBarView(statusLeft, statusRight, s.width)

	return content + "\n" + statusBar
}

func (s *SettingsView) renderAddDialog() string {
	dialogStyle := lipgloss.NewStyle().
		Border(lipgloss.DoubleBorder()).
		BorderForeground(DefaultTheme.Primary).
		Padding(1, 2).
		Width(65)

	var b strings.Builder
	b.WriteString(S.Title.Render(Icons.Add+" Add Chart Registry") + "\n\n")

	// Name field
	label := "  Name:"
	if s.addField == 0 {
		label = S.Highlighted.Render(Icons.Arrow + " Name:")
	} else {
		label = S.Label.Render(label)
	}
	b.WriteString(label + "\n  " + s.addNameInput.View() + "\n\n")

	// URL field
	label = "  URL:"
	if s.addField == 1 {
		label = S.Highlighted.Render(Icons.Arrow + " URL:")
	} else {
		label = S.Label.Render(label)
	}
	b.WriteString(label + "\n  " + s.addURLInput.View() + "\n")
	b.WriteString(S.Muted.Render("  e.g., https://charts.example.com/api/v1") + "\n\n")

	// Buttons
	addBtn := S.Button.Render("Add")
	cancelBtn := S.Button.Render("Cancel")
	if s.addField == 2 {
		addBtn = S.ButtonFocus.Render("Add")
	}
	if s.addField == 3 {
		cancelBtn = S.ButtonFocus.Render("Cancel")
	}
	b.WriteString("  " + addBtn + "  " + cancelBtn + "\n\n")
	b.WriteString(S.Muted.Render("  Tab: next  Enter: confirm  Esc: cancel"))

	dialog := dialogStyle.Render(b.String())
	return lipgloss.Place(s.width, s.height, lipgloss.Center, lipgloss.Center, dialog)
}

func (s *SettingsView) renderDeleteConfirm() string {
	dialogStyle := lipgloss.NewStyle().
		Border(lipgloss.DoubleBorder()).
		BorderForeground(DefaultTheme.Error).
		Padding(2, 4).
		Width(50)

	registries := s.config.GetChartRegistries()
	var regName string
	if s.selectedRegistry < len(registries) {
		regName = registries[s.selectedRegistry].Name
	}

	content := S.Error.Render(Icons.Delete+" Remove Registry") + "\n\n"
	content += fmt.Sprintf("Remove \"%s\"?\n\n", regName)
	content += S.Muted.Render("[Y]es  [N]o")

	dialog := dialogStyle.Render(content)
	return lipgloss.Place(s.width, s.height, lipgloss.Center, lipgloss.Center, dialog)
}

func (s *SettingsView) renderEditDialog() string {
	dialogStyle := lipgloss.NewStyle().
		Border(lipgloss.DoubleBorder()).
		BorderForeground(DefaultTheme.Primary).
		Padding(1, 2).
		Width(60)

	var b strings.Builder
	b.WriteString(S.Title.Render(Icons.Edit+" Edit Settings") + "\n\n")

	// Default Namespace
	label := "  Default Namespace:"
	if s.editField == 0 {
		label = S.Highlighted.Render(Icons.Arrow + " Default Namespace:")
	} else {
		label = S.Label.Render(label)
	}
	b.WriteString(label + "\n  " + s.nsInput.View() + "\n")
	b.WriteString(S.Muted.Render("  Leave empty for all namespaces") + "\n\n")

	// Editor
	label = "  Editor:"
	if s.editField == 1 {
		label = S.Highlighted.Render(Icons.Arrow + " Editor:")
	} else {
		label = S.Label.Render(label)
	}
	b.WriteString(label + "\n  " + s.editorInput.View() + "\n")
	b.WriteString(S.Muted.Render("  Leave empty to use $EDITOR or vim") + "\n\n")

	// Buttons
	saveBtn := S.Button.Render("Save")
	cancelBtn := S.Button.Render("Cancel")
	if s.editField == 2 {
		saveBtn = S.ButtonFocus.Render("Save")
	}
	if s.editField == 3 {
		cancelBtn = S.ButtonFocus.Render("Cancel")
	}
	b.WriteString("  " + saveBtn + "  " + cancelBtn + "\n\n")
	b.WriteString(S.Muted.Render("  Tab: next  Enter: confirm  Esc: cancel"))

	dialog := dialogStyle.Render(b.String())
	return lipgloss.Place(s.width, s.height, lipgloss.Center, lipgloss.Center, dialog)
}

func (s *SettingsView) renderThemeDialog() string {
	dialogStyle := lipgloss.NewStyle().
		Border(lipgloss.DoubleBorder()).
		BorderForeground(DefaultTheme.Primary).
		Padding(1, 2).
		Width(50)

	var b strings.Builder
	b.WriteString(S.Title.Render(Icons.Settings+" Select Theme") + "\n\n")

	for i, name := range ThemeNames {
		theme := Themes[name]
		// Show preview swatch
		swatch := lipgloss.NewStyle().
			Foreground(theme.Primary).
			Render("●")

		var line string
		if i == s.selectedTheme {
			line = S.Selected.Render(Icons.Arrow + " " + name)
		} else {
			line = S.Value.Render("  " + name)
		}
		b.WriteString(swatch + " " + line + "\n")
	}

	b.WriteString("\n" + S.Muted.Render("j/k: navigate  Enter: select  Esc: cancel"))

	dialog := dialogStyle.Render(b.String())
	return lipgloss.Place(s.width, s.height, lipgloss.Center, lipgloss.Center, dialog)
}

func (s *SettingsView) renderContextDialog() string {
	dialogStyle := lipgloss.NewStyle().
		Border(lipgloss.DoubleBorder()).
		BorderForeground(DefaultTheme.Primary).
		Padding(1, 2).
		Width(70)

	var b strings.Builder
	b.WriteString(S.Title.Render(Icons.Helm+" Switch Kubernetes Context") + "\n\n")

	if len(s.contexts) == 0 {
		b.WriteString(S.Muted.Render("No contexts found") + "\n")
	} else {
		for i, ctx := range s.contexts {
			// Show current marker
			var marker string
			if ctx.IsCurrent {
				marker = S.Success.Render("*")
			} else {
				marker = " "
			}

			var line string
			if i == s.selectedContext {
				line = S.Selected.Render(Icons.Arrow + " " + ctx.Name)
			} else {
				line = S.Value.Render("  " + ctx.Name)
			}

			// Show cluster info
			clusterInfo := S.Muted.Render(fmt.Sprintf(" (%s)", ctx.Cluster))
			if ctx.Namespace != "" {
				clusterInfo = S.Muted.Render(fmt.Sprintf(" (%s, ns: %s)", ctx.Cluster, ctx.Namespace))
			}

			b.WriteString(marker + line + clusterInfo + "\n")
		}
	}

	b.WriteString("\n" + S.Muted.Render("j/k: navigate  Enter: switch  Esc: cancel"))

	dialog := dialogStyle.Render(b.String())
	return lipgloss.Place(s.width, s.height, lipgloss.Center, lipgloss.Center, dialog)
}

func (s *SettingsView) openTemplatesDialog() {
	s.showTemplatesDialog = true
	s.selectedTemplate = 0
	s.templatesDeleteConf = false
}

func (s *SettingsView) updateTemplatesDialog(msg tea.Msg) (*SettingsView, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		templates, _ := config.ListAllTemplates()

		// Handle delete confirmation
		if s.templatesDeleteConf {
			switch msg.String() {
			case "y", "Y":
				// Delete the selected template
				if s.selectedTemplate < len(templates) {
					t := templates[s.selectedTemplate]
					if err := config.DeleteTemplate(t.Chart, t.Name); err != nil {
						s.err = err
					} else {
						s.status = fmt.Sprintf("Template '%s' deleted", t.Name)
					}
				}
				s.templatesDeleteConf = false
				// Adjust selection if needed
				remaining, _ := config.ListAllTemplates()
				if s.selectedTemplate >= len(remaining) && s.selectedTemplate > 0 {
					s.selectedTemplate--
				}
				return s, nil
			case "n", "N", "esc":
				s.templatesDeleteConf = false
				return s, nil
			}
			return s, nil
		}

		switch msg.String() {
		case "esc", "q":
			s.showTemplatesDialog = false
			return s, nil
		case "up", "k":
			if s.selectedTemplate > 0 {
				s.selectedTemplate--
			}
		case "down", "j":
			if s.selectedTemplate < len(templates)-1 {
				s.selectedTemplate++
			}
		case "d", "delete":
			if len(templates) > 0 {
				s.templatesDeleteConf = true
			}
		}
	}
	return s, nil
}

func (s *SettingsView) renderTemplatesDialog() string {
	dialogWidth := 70
	dialogHeight := 20

	dialogStyle := lipgloss.NewStyle().
		Border(lipgloss.DoubleBorder()).
		BorderForeground(DefaultTheme.Primary).
		Padding(1, 2).
		Width(dialogWidth).
		Height(dialogHeight)

	var b strings.Builder
	b.WriteString(S.Title.Render(Icons.Values+" Install Templates") + "\n\n")

	templates, _ := config.ListAllTemplates()

	if len(templates) == 0 {
		b.WriteString(S.Muted.Render("No templates saved yet.") + "\n\n")
		b.WriteString(S.Muted.Render("To save a template:") + "\n")
		b.WriteString(S.Muted.Render("1. Go to Explore tab and search for a chart") + "\n")
		b.WriteString(S.Muted.Render("2. Press 'i' to open install dialog") + "\n")
		b.WriteString(S.Muted.Render("3. Edit values with 'e'") + "\n")
		b.WriteString(S.Muted.Render("4. Press 'T' to save as template") + "\n")
	} else {
		// Delete confirmation
		if s.templatesDeleteConf {
			t := templates[s.selectedTemplate]
			b.WriteString(S.Warning.Render("Delete template '"+t.Name+"'? (y/n)") + "\n\n")
		}

		// Template list
		for i, t := range templates {
			var line string
			if i == s.selectedTemplate {
				line = S.Selected.Render(Icons.Arrow + " " + t.Name)
			} else {
				line = S.Value.Render("  " + t.Name)
			}

			// Chart and description info
			details := S.Muted.Render("    Chart: " + t.Chart)
			if t.Description != "" {
				details += " - " + t.Description
			}

			// Created time
			created := S.Muted.Render(fmt.Sprintf("    Created: %s", t.CreatedAt.Format("2006-01-02 15:04")))

			// Value lines count
			valuesInfo := S.Muted.Render(fmt.Sprintf("    Values: %d keys", len(t.Values)))

			b.WriteString(line + "\n" + details + "\n" + created + "  " + valuesInfo + "\n\n")
		}
	}

	b.WriteString("\n" + S.Muted.Render("j/k: navigate  d: delete  Esc: close"))

	dialog := dialogStyle.Render(b.String())
	return lipgloss.Place(s.width, s.height, lipgloss.Center, lipgloss.Center, dialog)
}

// --- Commands ---

type settingsSavedMsg struct {
	err error
}

type contextsLoadedMsg struct {
	contexts []helm.KubeContext
	err      error
}

// ContextSwitchedMsg is sent when the context is switched
// This message is exported so the App can handle it
type ContextSwitchedMsg struct {
	Context string
}

func contextSwitchedCmd(context string) tea.Cmd {
	return func() tea.Msg {
		return ContextSwitchedMsg{Context: context}
	}
}

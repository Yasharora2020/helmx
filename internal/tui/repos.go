package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/yasharora2020/helmx/internal/helm"
	"github.com/yasharora2020/helmx/internal/tui/validation"
)

// ReposView handles repository management
type ReposView struct {
	repoManager helm.RepoManager
	width       int
	height      int

	// Data
	repos         []helm.Repository
	selectedIndex int
	err           error
	status        string

	// Add dialog
	showAddDialog      bool
	addNameInput       textinput.Model
	addURLInput        textinput.Model
	addUsernameInput   textinput.Model // Username for basic auth
	addPasswordInput   textinput.Model // Password for basic auth
	addRequiresAuth    bool            // Toggle: show auth fields
	addInsecureSkipTLS bool            // Toggle: skip TLS verification
	addField           int             // 0=name, 1=url, 2=auth toggle, 3=username, 4=password, 5=insecure, 6=confirm, 7=cancel
	addError           string          // Validation error message

	// Delete confirmation
	showDeleteConfirm bool

	// Loading state
	spinner LoadingSpinner
}

// NewReposView creates a new repos view
func NewReposView(repo helm.RepoManager) *ReposView {
	nameInput := textinput.New()
	nameInput.Placeholder = "bitnami"
	nameInput.CharLimit = 50
	nameInput.Width = 40

	urlInput := textinput.New()
	urlInput.Placeholder = "https://charts.bitnami.com/bitnami"
	urlInput.CharLimit = 200
	urlInput.Width = 40

	usernameInput := textinput.New()
	usernameInput.Placeholder = "username"
	usernameInput.CharLimit = 100
	usernameInput.Width = 40

	passwordInput := textinput.New()
	passwordInput.Placeholder = "password"
	passwordInput.CharLimit = 100
	passwordInput.Width = 40
	passwordInput.EchoMode = textinput.EchoPassword // Mask password input

	return &ReposView{
		repoManager:      repo,
		addNameInput:     nameInput,
		addURLInput:      urlInput,
		addUsernameInput: usernameInput,
		addPasswordInput: passwordInput,
		spinner:          NewLoadingSpinner(),
	}
}

// Init initializes the repos view
func (r *ReposView) Init() tea.Cmd {
	return r.fetchRepos()
}

// SetSize updates dimensions
func (r *ReposView) SetSize(width, height int) {
	r.width = width
	r.height = height
}

// HasOpenDialog returns true if any dialog is currently open.
func (r *ReposView) HasOpenDialog() bool {
	return r.showAddDialog || r.showDeleteConfirm
}

// Update handles input
func (r *ReposView) Update(msg tea.Msg) (*ReposView, tea.Cmd) {
	var cmds []tea.Cmd

	// Update spinner
	var spinnerCmd tea.Cmd
	r.spinner, spinnerCmd = r.spinner.Update(msg)
	if spinnerCmd != nil {
		cmds = append(cmds, spinnerCmd)
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		r.SetSize(msg.Width, msg.Height)

	case tea.KeyMsg:
		if r.showAddDialog {
			return r.updateAddDialog(msg)
		}
		if r.showDeleteConfirm {
			return r.updateDeleteConfirm(msg)
		}

		switch msg.String() {
		case "up", "k":
			if r.selectedIndex > 0 {
				r.selectedIndex--
			}
		case "down", "j":
			if r.selectedIndex < len(r.repos)-1 {
				r.selectedIndex++
			}
		case "a":
			r.openAddDialog()
		case "d", "delete":
			if len(r.repos) > 0 {
				r.showDeleteConfirm = true
			}
		case "u":
			// Update selected repo
			if len(r.repos) > 0 {
				cmds = append(cmds, r.spinner.Start("Updating "+r.repos[r.selectedIndex].Name+"..."))
				return r, tea.Batch(append(cmds, r.updateRepo(r.repos[r.selectedIndex].Name))...)
			}
		case "U":
			// Update all repos
			cmds = append(cmds, r.spinner.Start("Updating all repositories..."))
			return r, tea.Batch(append(cmds, r.updateAllRepos())...)
		case "r":
			cmds = append(cmds, r.spinner.Start("Refreshing..."))
			return r, tea.Batch(append(cmds, r.fetchRepos())...)
		}

	case reposListMsg:
		r.repos = msg.repos
		r.err = msg.err
		r.spinner.Stop()
		if r.selectedIndex >= len(r.repos) {
			r.selectedIndex = len(r.repos) - 1
		}
		if r.selectedIndex < 0 {
			r.selectedIndex = 0
		}

	case repoActionMsg:
		r.spinner.Stop()
		if msg.err != nil {
			r.err = msg.err
			r.status = ""
		} else {
			r.status = msg.status
			r.err = nil
			// Refresh list
			cmds = append(cmds, r.spinner.Start("Refreshing..."))
			cmds = append(cmds, r.fetchRepos())
		}
	}

	return r, tea.Batch(cmds...)
}

func (r *ReposView) openAddDialog() {
	r.showAddDialog = true
	r.addField = 0
	r.addNameInput.SetValue("")
	r.addURLInput.SetValue("")
	r.addUsernameInput.SetValue("")
	r.addPasswordInput.SetValue("")
	r.addRequiresAuth = false
	r.addInsecureSkipTLS = false
	r.addNameInput.Focus()
}

func (r *ReposView) updateAddDialog(msg tea.KeyMsg) (*ReposView, tea.Cmd) {
	// Calculate max field based on whether auth is shown
	// Fields: 0=name, 1=url, 2=auth toggle, 3=username, 4=password, 5=insecure, 6=confirm, 7=cancel
	// When auth is hidden: 0=name, 1=url, 2=auth toggle, 3=confirm, 4=cancel
	maxField := 4
	confirmField := 3
	cancelField := 4
	if r.addRequiresAuth {
		maxField = 7
		confirmField = 6
		cancelField = 7
	}

	switch msg.String() {
	case "esc":
		r.showAddDialog = false
		return r, nil

	case "tab", "down":
		r.addField = (r.addField + 1) % (maxField + 1)
		// Skip auth fields if auth is not enabled
		if !r.addRequiresAuth && r.addField >= 3 && r.addField <= 5 {
			r.addField = confirmField
		}
		r.updateAddFocus()

	case "shift+tab", "up":
		r.addField = (r.addField + maxField) % (maxField + 1)
		// Skip auth fields if auth is not enabled
		if !r.addRequiresAuth && r.addField >= 3 && r.addField <= 5 {
			r.addField = 2 // Go back to auth toggle
		}
		r.updateAddFocus()

	case "enter", " ":
		switch r.addField {
		case 2: // Auth toggle
			r.addRequiresAuth = !r.addRequiresAuth
			return r, nil
		case 5: // Insecure toggle (only when auth shown)
			if r.addRequiresAuth {
				r.addInsecureSkipTLS = !r.addInsecureSkipTLS
				return r, nil
			}
		}

		// Handle confirm/cancel
		if r.addField == confirmField {
			name := r.addNameInput.Value()
			url := r.addURLInput.Value()
			if name == "" || url == "" {
				r.addError = "Name and URL are required"
				return r, nil
			}

			// Validate repo name (uses same rules as K8s names)
			if err := validation.ValidateReleaseName(name); err != nil {
				r.addError = "Invalid repo name: " + err.Error()
				return r, nil
			}

			// Validate URL
			if err := validation.ValidateURL(url); err != nil {
				r.addError = "Invalid URL: " + err.Error()
				return r, nil
			}

			r.addError = "" // Clear any previous error
			r.showAddDialog = false
			spinnerCmd := r.spinner.Start("Adding " + name + "...")

			// Build auth options if authentication is enabled
			var auth *helm.RepoAuthOptions
			if r.addRequiresAuth {
				auth = &helm.RepoAuthOptions{
					Username:              r.addUsernameInput.Value(),
					Password:              r.addPasswordInput.Value(),
					InsecureSkipTLSVerify: r.addInsecureSkipTLS,
				}
			}
			return r, tea.Batch(spinnerCmd, r.addRepo(name, url, auth))
		} else if r.addField == cancelField {
			r.addError = ""
			r.showAddDialog = false
			return r, nil
		}
	}

	// Update focused input
	var cmd tea.Cmd
	switch r.addField {
	case 0:
		r.addNameInput, cmd = r.addNameInput.Update(msg)
	case 1:
		r.addURLInput, cmd = r.addURLInput.Update(msg)
	case 3:
		if r.addRequiresAuth {
			r.addUsernameInput, cmd = r.addUsernameInput.Update(msg)
		}
	case 4:
		if r.addRequiresAuth {
			r.addPasswordInput, cmd = r.addPasswordInput.Update(msg)
		}
	}

	return r, cmd
}

func (r *ReposView) updateAddFocus() {
	r.addNameInput.Blur()
	r.addURLInput.Blur()
	r.addUsernameInput.Blur()
	r.addPasswordInput.Blur()
	switch r.addField {
	case 0:
		r.addNameInput.Focus()
	case 1:
		r.addURLInput.Focus()
	case 3:
		if r.addRequiresAuth {
			r.addUsernameInput.Focus()
		}
	case 4:
		if r.addRequiresAuth {
			r.addPasswordInput.Focus()
		}
	}
}

func (r *ReposView) updateDeleteConfirm(msg tea.KeyMsg) (*ReposView, tea.Cmd) {
	switch msg.String() {
	case "y", "Y", "enter":
		r.showDeleteConfirm = false
		if len(r.repos) > 0 {
			return r, r.removeRepo(r.repos[r.selectedIndex].Name)
		}
	case "n", "N", "esc":
		r.showDeleteConfirm = false
	}
	return r, nil
}

// View renders the repos view
func (r *ReposView) View() string {
	if r.showAddDialog {
		return r.renderAddDialog()
	}
	if r.showDeleteConfirm {
		return r.renderDeleteConfirm()
	}
	return r.renderMainView()
}

func (r *ReposView) renderMainView() string {
	// Header
	content := S.Title.Render(Icons.Repo+" Helm Repositories") + "\n\n"

	if r.spinner.IsActive() {
		content += r.spinner.View() + "\n\n"
	}

	if len(r.repos) == 0 && !r.spinner.IsActive() {
		content += S.Muted.Render("No repositories configured.\nPress 'a' to add one.")
	} else {
		content += r.renderRepoList()
	}

	main := S.BorderFocus.Width(r.width - 4).Height(r.height - 8).Render(content)

	// Status bar
	var statusLeft string
	if r.status != "" {
		statusLeft = S.Success.Render(r.status)
	}
	if r.err != nil {
		statusLeft = S.Error.Render(Icons.Cross + " " + r.err.Error())
	}

	return main + "\n" + statusLeft
}

func (r *ReposView) renderRepoList() string {
	var lines []string

	for i, repo := range r.repos {
		name := repo.Name
		url := repo.URL

		// Add auth indicator
		authIndicator := ""
		if repo.HasAuth {
			authIndicator = " " + Icons.Lock
		}

		// Format last updated time
		lastUpdated := ""
		if !repo.LastUpdated.IsZero() {
			lastUpdated = " · " + formatTimeAgo(repo.LastUpdated)
		}

		// Truncate URL if too long
		maxURLLen := r.width - 20
		if len(url) > maxURLLen {
			url = url[:maxURLLen-3] + "..."
		}

		var line string
		if i == r.selectedIndex {
			line = S.Selected.Render(Icons.Arrow + " " + name + authIndicator)
		} else {
			line = S.Value.Render("  " + name + authIndicator)
		}
		line += "\n" + S.Muted.Render("    "+url+lastUpdated)
		lines = append(lines, line)
	}

	return strings.Join(lines, "\n")
}

// formatTimeAgo returns a human-readable relative time string
func formatTimeAgo(t time.Time) string {
	duration := time.Since(t)

	switch {
	case duration < time.Minute:
		return "just now"
	case duration < time.Hour:
		mins := int(duration.Minutes())
		if mins == 1 {
			return "1 min ago"
		}
		return fmt.Sprintf("%d mins ago", mins)
	case duration < 24*time.Hour:
		hours := int(duration.Hours())
		if hours == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", hours)
	case duration < 7*24*time.Hour:
		days := int(duration.Hours() / 24)
		if days == 1 {
			return "1 day ago"
		}
		return fmt.Sprintf("%d days ago", days)
	default:
		weeks := int(duration.Hours() / 24 / 7)
		if weeks == 1 {
			return "1 week ago"
		}
		return fmt.Sprintf("%d weeks ago", weeks)
	}
}

func (r *ReposView) renderAddDialog() string {
	dialogStyle := lipgloss.NewStyle().
		Border(lipgloss.DoubleBorder()).
		BorderForeground(DefaultTheme.Primary).
		Padding(1, 2).
		Width(60)

	var content strings.Builder
	content.WriteString(S.Title.Render(Icons.Add+" Add Repository") + "\n\n")

	// Name field
	nameLabel := "  Name:"
	if r.addField == 0 {
		nameLabel = S.Highlighted.Render(Icons.Arrow + " Name:")
	} else {
		nameLabel = S.Label.Render(nameLabel)
	}
	content.WriteString(nameLabel + "\n  " + r.addNameInput.View() + "\n\n")

	// URL field
	urlLabel := "  URL:"
	if r.addField == 1 {
		urlLabel = S.Highlighted.Render(Icons.Arrow + " URL:")
	} else {
		urlLabel = S.Label.Render(urlLabel)
	}
	content.WriteString(urlLabel + "\n  " + r.addURLInput.View() + "\n\n")

	// Display validation error if any
	if r.addError != "" {
		content.WriteString(S.Error.Render(Icons.Cross+" "+r.addError) + "\n\n")
	}

	// Auth toggle checkbox
	authCheckbox := "[ ]"
	if r.addRequiresAuth {
		authCheckbox = "[x]"
	}
	authLabel := "  " + authCheckbox + " Requires authentication"
	if r.addField == 2 {
		authLabel = S.Highlighted.Render(Icons.Arrow + " " + authCheckbox + " Requires authentication")
	} else {
		authLabel = S.Label.Render(authLabel)
	}
	content.WriteString(authLabel + "\n")

	// Show auth fields if enabled
	if r.addRequiresAuth {
		content.WriteString("\n")

		// Username field
		usernameLabel := "  Username:"
		if r.addField == 3 {
			usernameLabel = S.Highlighted.Render(Icons.Arrow + " Username:")
		} else {
			usernameLabel = S.Label.Render(usernameLabel)
		}
		content.WriteString(usernameLabel + "\n  " + r.addUsernameInput.View() + "\n\n")

		// Password field
		passwordLabel := "  Password:"
		if r.addField == 4 {
			passwordLabel = S.Highlighted.Render(Icons.Arrow + " Password:")
		} else {
			passwordLabel = S.Label.Render(passwordLabel)
		}
		content.WriteString(passwordLabel + "\n  " + r.addPasswordInput.View() + "\n\n")

		// Insecure TLS toggle
		insecureCheckbox := "[ ]"
		if r.addInsecureSkipTLS {
			insecureCheckbox = "[x]"
		}
		insecureLabel := "  " + insecureCheckbox + " Skip TLS verification"
		if r.addField == 5 {
			insecureLabel = S.Highlighted.Render(Icons.Arrow + " " + insecureCheckbox + " Skip TLS verification")
		} else {
			insecureLabel = S.Muted.Render(insecureLabel)
		}
		content.WriteString(insecureLabel + "\n")
	}

	content.WriteString("\n")

	// Calculate button field indices based on auth state
	confirmField := 3
	cancelField := 4
	if r.addRequiresAuth {
		confirmField = 6
		cancelField = 7
	}

	// Buttons
	addBtn := S.Button.Render("Add")
	cancelBtn := S.Button.Render("Cancel")
	if r.addField == confirmField {
		addBtn = S.ButtonFocus.Render("Add")
	}
	if r.addField == cancelField {
		cancelBtn = S.ButtonFocus.Render("Cancel")
	}
	content.WriteString(addBtn + "  " + cancelBtn)

	dialog := dialogStyle.Render(content.String())

	return lipgloss.Place(r.width, r.height, lipgloss.Center, lipgloss.Center, dialog)
}

func (r *ReposView) renderDeleteConfirm() string {
	dialogStyle := lipgloss.NewStyle().
		Border(lipgloss.DoubleBorder()).
		BorderForeground(DefaultTheme.Error).
		Padding(2, 4).
		Width(50)

	var repoName string
	if len(r.repos) > 0 && r.selectedIndex < len(r.repos) {
		repoName = r.repos[r.selectedIndex].Name
	}

	content := S.Error.Render(Icons.Delete+" Delete Repository") + "\n\n"
	content += "Remove repository \"" + repoName + "\"?\n\n"
	content += S.Muted.Render("[Y]es  [N]o")

	dialog := dialogStyle.Render(content)

	return lipgloss.Place(r.width, r.height, lipgloss.Center, lipgloss.Center, dialog)
}

// --- Commands ---

type reposListMsg struct {
	repos []helm.Repository
	err   error
}

type repoActionMsg struct {
	status string
	err    error
}

func (r *ReposView) fetchRepos() tea.Cmd {
	return func() tea.Msg {
		repos, err := r.repoManager.ListRepos()
		return reposListMsg{repos: repos, err: err}
	}
}

func (r *ReposView) addRepo(name, url string, auth *helm.RepoAuthOptions) tea.Cmd {
	return func() tea.Msg {
		err := r.repoManager.AddRepo(name, url, auth)
		if err != nil {
			return repoActionMsg{err: err}
		}
		return repoActionMsg{status: Icons.Check + " Added " + name}
	}
}

func (r *ReposView) removeRepo(name string) tea.Cmd {
	return func() tea.Msg {
		err := r.repoManager.RemoveRepo(name)
		if err != nil {
			return repoActionMsg{err: err}
		}
		return repoActionMsg{status: Icons.Check + " Removed " + name}
	}
}

func (r *ReposView) updateRepo(name string) tea.Cmd {
	return func() tea.Msg {
		err := r.repoManager.UpdateRepo(name)
		if err != nil {
			return repoActionMsg{err: err}
		}
		return repoActionMsg{status: Icons.Check + " Updated " + name}
	}
}

func (r *ReposView) updateAllRepos() tea.Cmd {
	return func() tea.Msg {
		err := r.repoManager.UpdateAllRepos()
		if err != nil {
			return repoActionMsg{err: err}
		}
		return repoActionMsg{status: Icons.Check + " Updated all repositories"}
	}
}

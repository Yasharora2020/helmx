package tui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/yasharora2020/helmx/internal/config"
	"github.com/yasharora2020/helmx/internal/helm"
	"github.com/yasharora2020/helmx/internal/tui/dialog"
	"github.com/yasharora2020/helmx/internal/tui/validation"
	"gopkg.in/yaml.v3"
	"helm.sh/helm/v3/pkg/chart"
)

// ExploreView handles chart exploration before installation
type ExploreView struct {
	chartExplorer    helm.ChartExplorer
	releaseManager   helm.ReleaseManager
	clusterInspector helm.ClusterInspector
	repoManager      helm.RepoManager
	securityProvider helm.SecurityProvider
	config           *config.Config
	width            int
	height           int

	// UI Components
	searchInput textinput.Model
	valuesView  viewport.Model
	previewView viewport.Model
	activePane  ExplorePane
	spinner     LoadingSpinner

	// Install dialog
	releaseNameInput textinput.Model
	namespaceInput   textinput.Model
	valuesContent    string // YAML values content (edited externally)
	valuesErr        error  // YAML validation error
	createNamespace  bool
	installing       bool
	installResult    *installResultMsg

	// Version state (business logic, not dialog state)
	selectedVersion string // Selected version string (empty = latest)
	isHubChart      bool   // True if current chart is from Artifact Hub

	// Install dry-run preview dialog
	showInstallPreview      bool           // Show dry-run preview
	installPreviewManifest  string         // Rendered manifest
	installPreviewResources []string       // Resources that will be created
	installPreviewViewport  viewport.Model // Scrollable manifest view
	installPreviewLoading   bool           // Loading indicator
	installPreviewError     error          // Error from dry-run

	// Dependency tree dialog
	showDepTree     bool           // Show dependency tree dialog
	depTreeViewport viewport.Model // Scrollable tree view

	// Data
	searchResults  []helm.ChartInfo         // Local repo results
	hubResults     []helm.ArtifactHubResult // Artifact Hub results
	selectedIndex  int
	loadedChart    *LoadedChartData
	loadedChartRaw *chart.Chart // Raw chart object for dependency tree
	chartRef       string       // The chart reference used to load
	err            error

	// Extracted dialog components (Phase 4 refactoring)
	// These are being migrated from inline state to modular components
	fileImportDialog   *dialog.FileImportDialog
	setValueDialog     *dialog.SetValueDialog
	valueSourcesDialog *dialog.ValueSourcesDialog

	// Track which context triggered sub-dialogs (install vs template)
	templateFileImportActive bool
	templateSetValueActive   bool
	templateSourcesActive    bool
	versionDialog            *dialog.VersionDialog
	schemaDialog             *dialog.SchemaDialog
	readmeDialog             *dialog.ReadmeDialog
	securityDialog           *dialog.SecurityDialog
	templateDialog           *dialog.TemplateDialog
	progressDialog           *dialog.ProgressDialog
	installDialog            *dialog.InstallDialog
	multiChartDialog         *dialog.MultiChartDialog
	saveTemplateDialog       *dialog.SaveTemplateDialog
	saveStackDialog          *dialog.SaveStackDialog
	loadStackDialog          *dialog.LoadStackDialog
}

// NewExploreView creates a new explore view
func NewExploreView(ce helm.ChartExplorer, rm helm.ReleaseManager, ci helm.ClusterInspector, repo helm.RepoManager, sp helm.SecurityProvider, cfg *config.Config) *ExploreView {
	// Search input
	ti := textinput.New()
	ti.Placeholder = "Search charts (e.g., bitnami/postgresql, nginx)..."
	ti.Focus()
	ti.CharLimit = 100
	ti.Width = 50

	// Values viewport
	vv := viewport.New(40, 20)
	vv.SetContent("Select a chart to view its values")

	// Preview viewport
	pv := viewport.New(40, 20)
	pv.SetContent("Select a chart to preview resources")

	// Release name input for install dialog
	rn := textinput.New()
	rn.Placeholder = "my-release"
	rn.CharLimit = 63 // K8s name limit
	rn.Width = 30

	// Namespace input for install dialog
	ns := textinput.New()
	ns.Placeholder = "default"
	ns.CharLimit = 63
	ns.Width = 30

	// Install preview viewport
	installPrevVp := viewport.New(70, 20)
	installPrevVp.SetContent("")

	// Dependency tree viewport
	depVp := viewport.New(50, 15)
	depVp.SetContent("")

	e := &ExploreView{
		chartExplorer:          ce,
		releaseManager:         rm,
		clusterInspector:       ci,
		repoManager:            repo,
		securityProvider:       sp,
		config:                 cfg,
		searchInput:            ti,
		valuesView:             vv,
		previewView:            pv,
		activePane:             PaneSearch,
		releaseNameInput:       rn,
		namespaceInput:         ns,
		createNamespace:        true,
		installPreviewViewport: installPrevVp,
		depTreeViewport:        depVp,
		spinner:                NewLoadingSpinner(),
	}

	// Initialize extracted dialog components
	e.initDialogs()

	return e
}

// Init initializes the explore view
func (e *ExploreView) Init() tea.Cmd {
	return textinput.Blink
}

// HasOpenDialog returns true if any dialog is currently open.
func (e *ExploreView) HasOpenDialog() bool {
	// Check extracted dialogs
	if e.hasAnyDialogOpen() {
		return true
	}
	// Check remaining inline dialogs (not yet extracted)
	return e.showInstallPreview ||
		e.showDepTree
}

// IsSearchFocused returns true when the search input is focused and should receive keypresses
func (e *ExploreView) IsSearchFocused() bool {
	// Don't intercept if any dialog is open
	if e.HasOpenDialog() {
		return false
	}
	return e.activePane == PaneSearch
}

// Update handles messages for the explore view
func (e *ExploreView) Update(msg tea.Msg) (*ExploreView, tea.Cmd) {
	var cmds []tea.Cmd

	// Update spinner
	var spinnerCmd tea.Cmd
	e.spinner, spinnerCmd = e.spinner.Update(msg)
	if spinnerCmd != nil {
		cmds = append(cmds, spinnerCmd)
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		e.width = msg.Width
		e.height = msg.Height
		e.updateLayout()
		// Update dialog sizes for extracted components
		e.updateDialogSizes(msg.Width, msg.Height)
		// Initialize dialog styles (safe to call multiple times)
		e.initDialogStyles()

	case tea.KeyMsg:
		// If load stack dialog is open, handle it separately
		if e.loadStackDialog != nil && e.loadStackDialog.IsOpen() {
			_, cmd := e.loadStackDialog.Update(msg)
			return e, cmd
		}

		// If save stack dialog is open, handle it separately
		if e.saveStackDialog != nil && e.saveStackDialog.IsOpen() {
			_, cmd := e.saveStackDialog.Update(msg)
			return e, cmd
		}

		// If save template dialog is open, handle it separately
		if e.saveTemplateDialog != nil && e.saveTemplateDialog.IsOpen() {
			_, cmd := e.saveTemplateDialog.Update(msg)
			return e, cmd
		}

		// If file import dialog is open, handle it separately
		if e.fileImportDialog != nil && e.fileImportDialog.IsOpen() {
			_, cmd := e.fileImportDialog.Update(msg)
			return e, cmd
		}

		// If set value dialog is open, handle it separately
		if e.setValueDialog != nil && e.setValueDialog.IsOpen() {
			_, cmd := e.setValueDialog.Update(msg)
			return e, cmd
		}

		// If value sources dialog is open, handle it separately
		if e.valueSourcesDialog != nil && e.valueSourcesDialog.IsOpen() {
			_, cmd := e.valueSourcesDialog.Update(msg)
			return e, cmd
		}

		// If install preview dialog is open, handle it separately
		if e.showInstallPreview {
			return e.updateInstallPreview(msg)
		}

		// If install dialog is open, handle it separately
		if e.installDialog != nil && e.installDialog.IsOpen() {
			_, cmd := e.installDialog.Update(msg)
			return e, cmd
		}

		// If multi-chart dialog is open, handle it separately
		if e.multiChartDialog != nil && e.multiChartDialog.IsOpen() {
			_, cmd := e.multiChartDialog.Update(msg)
			return e, cmd
		}

		// If progress dialog is open, handle it separately
		if e.progressDialog != nil && e.progressDialog.IsOpen() {
			_, cmd := e.progressDialog.Update(msg)
			return e, cmd
		}

		// If README dialog is open, handle it separately
		if e.readmeDialog != nil && e.readmeDialog.IsOpen() {
			_, cmd := e.readmeDialog.Update(msg)
			return e, cmd
		}

		// If template dialog is open, handle it separately
		if e.templateDialog != nil && e.templateDialog.IsOpen() {
			_, cmd := e.templateDialog.Update(msg)
			return e, cmd
		}

		// If version dialog is open, handle it separately
		if e.versionDialog != nil && e.versionDialog.IsOpen() {
			_, cmd := e.versionDialog.Update(msg)
			return e, cmd
		}

		// If security dialog is open, handle it separately
		if e.securityDialog != nil && e.securityDialog.IsOpen() {
			_, cmd := e.securityDialog.Update(msg)
			return e, cmd
		}

		// If dependency tree dialog is open, handle it separately
		if e.showDepTree {
			return e.updateDepTreeDialog(msg)
		}

		// If schema browser dialog is open, handle it separately
		if e.schemaDialog != nil && e.schemaDialog.IsOpen() {
			_, cmd := e.schemaDialog.Update(msg)
			return e, cmd
		}

		// When in search pane, let text input handle most keys (except navigation)
		key := msg.String()
		if e.activePane == PaneSearch {
			// Only handle special keys, let everything else go to text input
			switch key {
			case "tab", "shift+tab", "enter", "esc", "ctrl+c", "?", "1", "2", "3", "4":
				// Fall through to handle these
			default:
				// Let the text input handle all other keys
				var cmd tea.Cmd
				e.searchInput, cmd = e.searchInput.Update(msg)
				return e, cmd
			}
		}

		switch key {
		case "tab":
			// Cycle through panes
			e.activePane = (e.activePane + 1) % 4
			e.updateFocus()

		case "shift+tab":
			// Cycle backwards
			e.activePane = (e.activePane + 3) % 4
			e.updateFocus()

		case "enter":
			if e.activePane == PaneSearch {
				// Trigger search
				query := e.searchInput.Value()
				if query != "" {
					cmds = append(cmds, e.spinner.Start("Searching..."))
					cmds = append(cmds, e.searchCharts(query))
					return e, tea.Batch(cmds...)
				}
			} else if e.activePane == PaneChartList && e.totalResults() > 0 {
				// Load selected chart
				e.chartRef = e.getSelectedChartRef()
				if e.chartRef != "" {
					cmds = append(cmds, e.spinner.Start("Loading chart..."))
					cmds = append(cmds, e.loadChart(e.chartRef))
					return e, tea.Batch(cmds...)
				}
			}

		case "i":
			// Open install dialog if a chart is loaded (not while typing in search)
			if e.activePane != PaneSearch && e.loadedChart != nil {
				e.openInstallDialog()
			}

		case "M":
			// Open multi-chart install dialog (not while typing in search)
			if e.activePane != PaneSearch {
				e.openMultiChartDialog()
			}

		case "a":
			// Add current chart to multi-chart queue (not while typing in search)
			if e.activePane != PaneSearch && e.loadedChart != nil {
				e.addChartToQueue()
			}

		case "r":
			// Open README dialog if a chart is loaded (not while typing in search)
			if e.activePane != PaneSearch && e.loadedChart != nil && e.loadedChart.Readme != "" {
				e.openReadmeDialog()
			}

		case "t":
			// Open template dialog if a chart is loaded (not while typing in search)
			if e.activePane != PaneSearch && e.loadedChart != nil {
				e.openTemplateDialog()
			}

		case "v":
			// Open version selector if a chart is loaded or chart list has results
			if e.activePane != PaneSearch && e.totalResults() > 0 {
				e.openVersionDialog()
				return e, e.fetchChartVersions()
			}

		case "s":
			// Open security scan dialog if a chart is loaded (not while typing in search)
			if e.activePane != PaneSearch && e.loadedChart != nil {
				// Check if security scanning is enabled
				if !e.config.SecurityScanEnabled {
					chartName := e.loadedChart.Info.Name
					chartVersion := e.loadedChart.Info.Version
					e.securityDialog.OpenForChart(chartName, chartVersion)
					e.securityDialog.SetError(fmt.Errorf("security scanning is disabled - enable in Settings (press 4, then s)"))
					return e, nil
				}
				e.openSecurityDialog()
				cmds = append(cmds, e.spinner.Start("Scanning with Trivy..."))
				cmds = append(cmds, e.startSecurityScan())
				return e, tea.Batch(cmds...)
			}

		case "D": // Shift+D for dependency tree
			// Show dependency tree dialog if a chart is loaded
			if e.activePane != PaneSearch && e.loadedChartRaw != nil {
				e.openDepTreeDialog()
			}

		case "S": // Shift+S for schema browser
			// Show schema browser if a chart is loaded and has schema
			if e.activePane != PaneSearch && e.loadedChart != nil && len(e.loadedChart.Values.Schema) > 0 {
				e.openSchemaDialog()
			}

		case "up", "k":
			if e.activePane == PaneChartList && e.selectedIndex > 0 {
				e.selectedIndex--
			}

		case "down", "j":
			if e.activePane == PaneChartList && e.selectedIndex < e.totalResults()-1 {
				e.selectedIndex++
			}
		}

	case searchResultsMsg:
		e.spinner.Stop()
		e.searchResults = msg.localResults
		e.hubResults = msg.hubResults
		e.selectedIndex = 0
		e.err = msg.err
		// Auto-switch to chart list so user can browse results
		if e.totalResults() > 0 {
			e.activePane = PaneChartList
			e.updateFocus()
		}

	case chartLoadedMsg:
		e.spinner.Stop()
		e.loadedChart = msg.data
		e.loadedChartRaw = msg.rawChart
		e.err = msg.err
		e.updateContent()

	case chartVersionsMsg:
		if e.versionDialog != nil {
			e.versionDialog.SetLoading(false)
			if msg.err != nil {
				e.versionDialog.SetError(msg.err)
			} else if msg.isHub {
				// Convert helm versions to dialog format
				hubVersions := make([]dialog.HubVersion, len(msg.hubVersions))
				for i, v := range msg.hubVersions {
					hubVersions[i] = dialog.HubVersion{
						Version:    v.Version,
						PreRelease: v.PreRelease,
					}
				}
				e.versionDialog.SetHubVersions(hubVersions)
			} else {
				// Convert local versions to dialog format
				localVersions := make([]dialog.LocalVersion, len(msg.versions))
				for i, v := range msg.versions {
					localVersions[i] = dialog.LocalVersion{
						Version:    v.Version,
						AppVersion: v.AppVersion,
					}
				}
				e.versionDialog.SetLocalVersions(localVersions)
			}
		}

	case installResultMsg:
		e.spinner.Stop()
		e.installing = false
		e.installResult = &msg
		if msg.err == nil {
			// Success - transition to progress monitoring
			if e.installDialog != nil {
				e.installDialog.Close()
			}
			cmds = append(cmds, e.startProgressMonitoring(msg.release))
		}

	case installProgressMsg:
		if e.progressDialog != nil && e.progressDialog.IsOpen() {
			// Update extracted dialog with pod/resource status
			e.progressDialog.SetData(
				convertPodsToDialogFormat(msg.pods),
				convertResourcesToDialogFormat(msg.resources),
				convertEventsToDialogFormat(msg.events),
				msg.allReady,
				msg.err,
			)
		}

	case multiChartValuesEditedMsg:
		// Handle return from external editor for multi-chart item
		if e.multiChartDialog != nil {
			queue := e.multiChartDialog.GetQueue()
			if msg.idx < len(queue) {
				if msg.err != nil {
					e.multiChartDialog.SetError(msg.err)
				} else {
					e.multiChartDialog.UpdateItemValues(msg.idx, msg.values)
					e.multiChartDialog.SetError(nil)
				}
			}
		}

	case multiChartInstallProgressMsg:
		// Handle multi-chart installation progress
		if e.multiChartDialog != nil {
			e.multiChartDialog.UpdateItemStatus(msg.idx, dialog.MultiChartStatus(msg.status), msg.err)

			if msg.status == MultiChartFailed {
				// Continue to next chart instead of stopping
				e.multiChartDialog.SetCurrentIdx(msg.idx + 1)
			} else if msg.status == MultiChartReady {
				// Move to next chart
				nextIdx := msg.idx + 1
				e.multiChartDialog.SetCurrentIdx(nextIdx)
				queue := e.multiChartDialog.GetQueue()
				if nextIdx >= len(queue) {
					// All done
					e.multiChartDialog.SetInstalling(false)
					e.spinner.Stop()
				}
			}
		}

	case multiChartInstallCompleteMsg:
		// Multi-chart installation completed
		e.spinner.Stop()
		if e.multiChartDialog != nil {
			e.multiChartDialog.SetInstalling(false)
			if msg.err != nil {
				e.multiChartDialog.SetError(msg.err)
			}
		}

	case multiChartStackSavedMsg:
		// Stack saved to config
		if msg.err != nil {
			if e.saveStackDialog != nil {
				e.saveStackDialog.SetError(msg.err.Error())
				e.saveStackDialog.Open()
			}
		}

	case multiChartStackLoadedMsg:
		// Stack loaded from config
		if msg.err != nil && e.multiChartDialog != nil {
			e.multiChartDialog.SetError(msg.err)
		}

	case valuesEditedMsg:
		// Handle return from external editor
		if msg.err != nil {
			e.valuesErr = msg.err
		} else {
			e.valuesContent = msg.content
			// Validate YAML
			var values map[string]interface{}
			if err := yaml.Unmarshal([]byte(msg.content), &values); err != nil {
				e.valuesErr = err
			} else {
				e.valuesErr = nil
			}
			// Also update install dialog if open
			if e.installDialog != nil && e.installDialog.IsOpen() {
				e.installDialog.SetValues(msg.content)
				if e.valuesErr != nil {
					e.installDialog.SetValuesError(e.valuesErr)
				} else {
					e.installDialog.SetValuesError(nil)
				}
			}
		}

	case valuesDecryptedMsg:
		// Handle decryption result - update values content
		if msg.err == nil {
			e.valuesContent = msg.content
			// Validate the decrypted YAML
			var values map[string]interface{}
			if err := yaml.Unmarshal([]byte(msg.content), &values); err != nil {
				e.valuesErr = err
			} else {
				e.valuesErr = nil
			}
			// Also update install dialog if open
			if e.installDialog != nil && e.installDialog.IsOpen() {
				e.installDialog.SetValues(msg.content)
			}
		}

	case templateValuesEditedMsg:
		// Handle return from external editor for template values
		if e.templateDialog == nil {
			break
		}
		if msg.err != nil {
			e.templateDialog.SetValuesError(msg.err)
		} else {
			e.templateDialog.SetValues(msg.content)
			// Validate YAML
			var values map[string]interface{}
			if err := yaml.Unmarshal([]byte(msg.content), &values); err != nil {
				e.templateDialog.SetValuesError(err)
			} else {
				e.templateDialog.SetValuesError(nil)
			}
		}

	case templateResultMsg:
		e.spinner.Stop()
		if e.templateDialog != nil {
			e.templateDialog.SetTemplating(false)
			e.templateDialog.SetResult(msg.outputPath, msg.err)
		}

	case securityScanResultMsg:
		e.spinner.Stop()
		if e.securityDialog == nil {
			break
		}
		if msg.err != nil {
			e.securityDialog.SetError(msg.err)
		} else if msg.results != nil {
			// Convert helm results to dialog format
			dialogResults := &dialog.SecurityScanResults{
				Critical: msg.results.Critical,
				High:     msg.results.High,
				Medium:   msg.results.Medium,
				Low:      msg.results.Low,
				Total:    msg.results.Total,
			}
			for _, r := range msg.results.Results {
				scanResult := dialog.ScanResult{Target: r.Target}
				for _, m := range r.Misconfigurations {
					scanResult.Misconfigurations = append(scanResult.Misconfigurations, dialog.Misconfiguration{
						ID:          m.ID,
						Title:       m.Title,
						Description: m.Description,
						Message:     m.Message,
						Severity:    m.Severity,
					})
				}
				dialogResults.Results = append(dialogResults.Results, scanResult)
			}
			e.securityDialog.SetResults(dialogResults)
		}

	case installPreviewMsg:
		e.spinner.Stop()
		e.installPreviewLoading = false
		if msg.err != nil {
			e.installPreviewError = msg.err
		} else {
			e.installPreviewManifest = msg.manifest
			e.installPreviewResources = msg.resources
			e.installPreviewError = nil
			e.showInstallPreview = true
			e.updateInstallPreviewViewport()
		}

	// Handle extracted dialog messages
	case dialog.InstallExecuteMsg:
		// User confirmed install from install dialog
		e.installing = true
		chartRef := e.chartRef
		if e.selectedVersion != "" {
			chartRef = chartRef + ":" + e.selectedVersion
		}
		releaseName := msg.ReleaseName
		namespace := msg.Namespace
		if namespace == "" {
			namespace = "default"
		}
		createNS := msg.CreateNamespace
		valueFiles := msg.ValueFiles
		setValues := msg.SetValues
		valuesContent := msg.Values

		cmds = append(cmds, e.spinner.Start("Installing..."))
		cmds = append(cmds, func() tea.Msg {
			composer := &helm.ValuesComposer{
				ValueFiles: valueFiles,
				InlineYAML: valuesContent,
				SetValues:  setValues,
			}
			values, err := composer.Compose()
			if err != nil {
				return installResultMsg{err: fmt.Errorf("failed to merge values: %v", err)}
			}
			release, err := e.releaseManager.Install(chartRef, releaseName, namespace, values, createNS)
			return installResultMsg{release: release, err: err}
		})
		return e, tea.Batch(cmds...)

	case dialog.InstallEditValuesMsg:
		// User wants to edit values in external editor from install dialog
		if e.installDialog != nil && e.installDialog.IsOpen() {
			return e, e.openInstallDialogEditor()
		}

	case dialog.InstallImportFileMsg:
		// User wants to import values from file
		if e.fileImportDialog != nil {
			e.fileImportDialog.Open()
		}

	case dialog.InstallDryRunMsg:
		// User wants a dry-run preview from install dialog
		e.installPreviewLoading = true
		cmds = append(cmds, e.spinner.Start("Running dry-run..."))
		chartRef := e.chartRef
		version := e.selectedVersion
		releaseName := msg.ReleaseName
		namespace := msg.Namespace
		if namespace == "" {
			namespace = "default"
		}
		valuesContent := msg.Values
		valueFiles := msg.ValueFiles
		setValues := msg.SetValues
		cmds = append(cmds, func() tea.Msg {
			composer := &helm.ValuesComposer{
				ValueFiles: valueFiles,
				InlineYAML: valuesContent,
				SetValues:  setValues,
			}
			values, err := composer.Compose()
			if err != nil {
				return installPreviewMsg{err: fmt.Errorf("failed to merge values: %v", err)}
			}
			var manifest string
			var resources []string
			if version != "" {
				manifest, resources, err = e.chartExplorer.PreviewInstallWithVersion(chartRef, version, releaseName, namespace, values)
			} else {
				manifest, resources, err = e.chartExplorer.PreviewInstall(chartRef, releaseName, namespace, values)
			}
			return installPreviewMsg{manifest: manifest, resources: resources, err: err}
		})
		return e, tea.Batch(cmds...)

	case dialog.InstallDecryptMsg:
		// User wants to decrypt SOPS-encrypted values
		if e.securityProvider.HasSecretsSupport() && e.installDialog != nil && e.installDialog.IsOpen() {
			secretsClient := e.securityProvider.GetSecretsClient()
			valuesContent := e.installDialog.GetValues()
			cmds = append(cmds, func() tea.Msg {
				decrypted, err := secretsClient.Decrypt(valuesContent)
				return valuesDecryptedMsg{content: decrypted, err: err}
			})
			return e, tea.Batch(cmds...)
		}

	case dialog.InstallOpenReadmeMsg:
		// User wants to view the README from install dialog
		e.openReadmeDialog()

	case dialog.FileImportResultMsg:
		if e.templateFileImportActive {
			// File import was triggered from template dialog
			e.templateFileImportActive = false
			if e.templateDialog != nil && e.templateDialog.IsOpen() && msg.Path != "" {
				e.templateDialog.AddValueFile(msg.Path)
				if e.valueSourcesDialog != nil && e.valueSourcesDialog.IsOpen() {
					e.valueSourcesDialog.AddValueFile(msg.Path)
				}
			}
		} else if e.installDialog != nil && e.installDialog.IsOpen() {
			// Add file path to value files list (content is merged at execution time)
			if msg.Path != "" {
				e.installDialog.AddValueFile(msg.Path)
				// Sync to value sources dialog if open
				if e.valueSourcesDialog != nil && e.valueSourcesDialog.IsOpen() {
					e.valueSourcesDialog.AddValueFile(msg.Path)
				}
			} else {
				// Fallback: if no path, set values directly (backwards compat)
				e.installDialog.SetValues(msg.Content)
			}
		}
		// Also update explore view state for non-dialog contexts
		if msg.Path == "" && !e.templateFileImportActive {
			e.valuesContent = msg.Content
			e.valuesErr = nil
		}

	case dialog.InstallAddSetValueMsg:
		if e.setValueDialog != nil {
			e.setValueDialog.Open()
		}

	case dialog.InstallManageSourcesMsg:
		if e.valueSourcesDialog != nil && e.installDialog != nil {
			e.valueSourcesDialog.OpenWith(e.installDialog.GetValueFiles(), e.installDialog.GetSetValues())
		}

	case dialog.SetValueResultMsg:
		if e.templateSetValueActive {
			// Set value was triggered from template dialog
			e.templateSetValueActive = false
			if e.templateDialog != nil && e.templateDialog.IsOpen() {
				e.templateDialog.AddSetValue(msg.KeyValue)
				if e.valueSourcesDialog != nil && e.valueSourcesDialog.IsOpen() {
					e.valueSourcesDialog.AddSetValue(msg.KeyValue)
				}
			}
		} else if e.installDialog != nil && e.installDialog.IsOpen() {
			e.installDialog.AddSetValue(msg.KeyValue)
			// Sync to value sources dialog if open
			if e.valueSourcesDialog != nil && e.valueSourcesDialog.IsOpen() {
				e.valueSourcesDialog.AddSetValue(msg.KeyValue)
			}
		}

	case dialog.ValueSourcesClosedMsg:
		if e.templateSourcesActive {
			e.templateSourcesActive = false
			if e.templateDialog != nil && e.templateDialog.IsOpen() {
				e.templateDialog.SetValueFiles(msg.ValueFiles)
				e.templateDialog.SetSetValues(msg.SetValues)
			}
		} else if e.installDialog != nil && e.installDialog.IsOpen() {
			e.installDialog.SetValueFiles(msg.ValueFiles)
			e.installDialog.SetSetValues(msg.SetValues)
		}

	case dialog.ValueSourcesAddFileMsg:
		if e.fileImportDialog != nil {
			e.fileImportDialog.Open()
		}

	case dialog.ValueSourcesAddSetMsg:
		if e.setValueDialog != nil {
			e.setValueDialog.Open()
		}

	case dialog.VersionSelectedMsg:
		// Version was selected - load chart with that version
		if msg.Version != "" {
			e.selectedVersion = msg.Version
			chartRef := msg.ChartRef
			if chartRef != "" {
				e.chartRef = chartRef
				cmds = append(cmds, e.spinner.Start("Loading chart..."))
				cmds = append(cmds, e.loadChartWithVersion(chartRef, msg.Version))
			}
		}

	case dialog.TemplateEditValuesMsg:
		// User wants to edit template values externally
		cmds = append(cmds, e.openTemplateExternalEditor())

	case dialog.TemplateImportFileMsg:
		if e.fileImportDialog != nil {
			e.fileImportDialog.Open()
			e.templateFileImportActive = true
		}

	case dialog.TemplateAddSetValueMsg:
		if e.setValueDialog != nil {
			e.setValueDialog.Open()
			e.templateSetValueActive = true
		}

	case dialog.TemplateManageSourcesMsg:
		if e.valueSourcesDialog != nil && e.templateDialog != nil {
			e.valueSourcesDialog.OpenWith(e.templateDialog.GetValueFiles(), e.templateDialog.GetSetValues())
			e.templateSourcesActive = true
		}

	case dialog.TemplateExecuteMsg:
		// Execute template to file
		if e.templateDialog != nil {
			e.templateDialog.SetTemplating(true)
		}
		cmds = append(cmds, e.spinner.Start("Rendering template..."))
		cmds = append(cmds, e.executeTemplateToFile(msg.OutputPath, msg.Values, msg.ValueFiles, msg.SetValues))

	case dialog.ProgressFetchMsg:
		// Fetch progress data for the progress dialog
		if e.progressDialog != nil && e.progressDialog.IsOpen() {
			cmds = append(cmds, e.fetchProgress())
		}

	case dialog.ProgressTickMsg:
		// Progress tick - handled by the dialog, which will send ProgressFetchMsg

	case dialog.SecurityRescanMsg:
		// User requested a security rescan
		if e.securityDialog != nil {
			e.securityDialog.SetLoading(true)
		}
		cmds = append(cmds, e.spinner.Start("Scanning..."))
		cmds = append(cmds, e.startSecurityScan())

	case dialog.MultiChartEditValuesMsg:
		// User wants to edit values for a chart in the queue
		cmds = append(cmds, e.openMultiChartValuesEditor(msg.Index))

	case dialog.MultiChartInstallMsg:
		// Start multi-chart installation
		if e.multiChartDialog != nil {
			e.multiChartDialog.SetInstalling(true)
			e.multiChartDialog.SetCurrentIdx(0)
		}
		cmds = append(cmds, e.spinner.Start("Installing charts..."))
		cmds = append(cmds, e.startMultiChartInstall())

	case dialog.MultiChartSaveStackMsg:
		// Open save stack dialog for user to enter name/description
		if e.saveStackDialog != nil && e.multiChartDialog != nil {
			e.saveStackDialog.OpenForQueue(len(e.multiChartDialog.GetQueue()))
		}

	case dialog.MultiChartLoadStackMsg:
		// Open stack picker dialog
		if e.loadStackDialog != nil {
			stacks := e.config.GetStacks()
			items := make([]dialog.LoadStackItem, len(stacks))
			for i, s := range stacks {
				items[i] = dialog.LoadStackItem{
					Name:        s.Name,
					Description: s.Description,
					ChartCount:  len(s.Charts),
				}
			}
			e.loadStackDialog.OpenWithStacks(items)
		}

	case dialog.LoadStackResultMsg:
		cmds = append(cmds, e.loadMultiChartStackByName(msg.Name))

	case dialog.InstallSaveTemplateMsg:
		// User wants to save current values as a template
		if e.saveTemplateDialog != nil && e.loadedChart != nil {
			chartName := e.chartRef
			if chartName == "" {
				chartName = e.loadedChart.Info.Name
			}
			e.saveTemplateDialog.OpenForChart(chartName, msg.Values)
		}

	case dialog.SaveTemplateResultMsg:
		if e.loadedChart == nil {
			break
		}
		err := config.SaveTemplate(
			msg.Name,
			msg.ChartName,
			e.loadedChart.Info.Version,
			msg.Description,
			msg.Values,
			e.loadedChart.Values.Raw,
		)
		if err != nil {
			if e.saveTemplateDialog != nil {
				e.saveTemplateDialog.SetError(err.Error())
				e.saveTemplateDialog.Open()
			}
		}

	case dialog.SaveStackResultMsg:
		cmds = append(cmds, e.saveMultiChartStackWithName(msg.Name, msg.Description))
	}

	// Update the focused component
	var cmd tea.Cmd
	switch e.activePane {
	case PaneSearch:
		e.searchInput, cmd = e.searchInput.Update(msg)
		cmds = append(cmds, cmd)
	case PaneValues:
		e.valuesView, cmd = e.valuesView.Update(msg)
		cmds = append(cmds, cmd)
	case PanePreview:
		e.previewView, cmd = e.previewView.Update(msg)
		cmds = append(cmds, cmd)
	}

	return e, tea.Batch(cmds...)
}

func (e *ExploreView) openInstallDialog() {
	if e.installDialog == nil {
		return
	}

	// Collect chart info
	chartName := ""
	chartVersion := ""
	defaultValues := "# Custom values (YAML)\n"
	isEncrypted := false
	decryptedValues := ""

	if e.loadedChart != nil {
		chartName = e.loadedChart.Info.Name
		chartVersion = e.loadedChart.Info.Version
		if e.loadedChart.Values.Raw != "" {
			isEncrypted = e.loadedChart.Values.IsEncrypted
			if isEncrypted && e.loadedChart.Values.Decrypted != "" {
				decryptedValues = e.loadedChart.Values.Decrypted
			}
			defaultValues = e.loadedChart.Values.Raw
		}
	}

	// Load templates for this chart
	templates := e.loadTemplatesForChart(chartName)

	e.installDialog.OpenForChart(chartName, chartVersion, e.chartRef, defaultValues, templates, isEncrypted, decryptedValues)
}

// openMultiChartDialog opens the multi-chart installation dialog
func (e *ExploreView) openMultiChartDialog() {
	if e.multiChartDialog == nil {
		return
	}
	e.multiChartDialog.Open()
}

// loadTemplatesForChart loads templates for a chart from config
func (e *ExploreView) loadTemplatesForChart(chartName string) []dialog.InstallTemplate {
	key := e.chartRef
	if key == "" {
		key = chartName
	}
	fileTemplates, err := config.ListTemplates(key)
	if err != nil || len(fileTemplates) == 0 {
		fileTemplates, _ = config.ListTemplates(chartName)
	}

	templates := make([]dialog.InstallTemplate, 0, len(fileTemplates))
	for _, ft := range fileTemplates {
		var mergedValues string
		if e.loadedChart != nil {
			var mergeErr error
			mergedValues, mergeErr = config.ApplyTemplate(ft.Values, e.loadedChart.Values.Raw)
			if mergeErr != nil {
				mergedValues = e.loadedChart.Values.Raw
			}
		}

		var versionWarning string
		if e.loadedChart != nil && ft.ChartVersion != e.loadedChart.Info.Version {
			versionWarning = fmt.Sprintf("⚠ Saved for v%s, current chart is v%s — review values",
				ft.ChartVersion, e.loadedChart.Info.Version)
		}

		templates = append(templates, dialog.InstallTemplate{
			Name:           ft.Name,
			Description:    ft.Description,
			Values:         mergedValues,
			VersionWarning: versionWarning,
		})
	}
	return templates
}

// openMultiChartValuesEditor opens external editor for editing values of a chart in the queue
func (e *ExploreView) openMultiChartValuesEditor(index int) tea.Cmd {
	if e.multiChartDialog == nil {
		return nil
	}

	queue := e.multiChartDialog.GetQueue()
	if index < 0 || index >= len(queue) {
		return nil
	}

	item := queue[index]
	content := item.Values

	// Create temp file
	tmpFile, err := os.CreateTemp("", "helmx-multi-values-*.yaml")
	if err != nil {
		return func() tea.Msg {
			return multiChartValuesEditedMsg{idx: index, err: err}
		}
	}
	tmpPath := tmpFile.Name()

	// Write content to temp file
	if _, err := tmpFile.WriteString(content); err != nil {
		_ = tmpFile.Close()
		_ = os.Remove(tmpPath)
		return func() tea.Msg {
			return multiChartValuesEditedMsg{idx: index, err: err}
		}
	}
	_ = tmpFile.Close()

	// Get editor from $EDITOR, fallback to vim
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vim"
	}

	// Use tea.ExecProcess to suspend TUI and run editor
	cmd := exec.Command(editor, tmpPath)
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		defer os.Remove(tmpPath)

		if err != nil {
			return multiChartValuesEditedMsg{idx: index, err: err}
		}

		// Read modified content
		data, err := os.ReadFile(tmpPath)
		if err != nil {
			return multiChartValuesEditedMsg{idx: index, err: err}
		}

		return multiChartValuesEditedMsg{idx: index, values: string(data)}
	})
}

// startMultiChartInstall starts installing all charts in the queue sequentially
func (e *ExploreView) startMultiChartInstall() tea.Cmd {
	return func() tea.Msg {
		if e.multiChartDialog == nil {
			return multiChartInstallCompleteMsg{err: fmt.Errorf("dialog not initialized")}
		}

		queue := e.multiChartDialog.GetQueue()
		if len(queue) == 0 {
			return multiChartInstallCompleteMsg{err: fmt.Errorf("no charts in queue")}
		}

		for i, item := range queue {
			// Update status to installing
			e.multiChartDialog.UpdateItemStatus(i, dialog.MultiChartInstalling, nil)

			// Parse values string to map
			var valuesMap map[string]interface{}
			if item.Values != "" {
				if err := yaml.Unmarshal([]byte(item.Values), &valuesMap); err != nil {
					e.multiChartDialog.UpdateItemStatus(i, dialog.MultiChartFailed, fmt.Errorf("invalid YAML: %w", err))
					continue
				}
			}

			// Install the chart
			_, err := e.releaseManager.Install(
				item.ChartRef,
				item.ReleaseName,
				item.Namespace,
				valuesMap,
				item.CreateNamespace,
			)

			if err != nil {
				e.multiChartDialog.UpdateItemStatus(i, dialog.MultiChartFailed, err)
				// Continue to next chart instead of stopping
				continue
			}

			e.multiChartDialog.UpdateItemStatus(i, dialog.MultiChartReady, nil)
		}

		return multiChartInstallCompleteMsg{}
	}
}

// saveMultiChartStackWithName saves the current queue as a named stack.
func (e *ExploreView) saveMultiChartStackWithName(name, description string) tea.Cmd {
	return func() tea.Msg {
		if e.multiChartDialog == nil {
			return multiChartStackSavedMsg{err: fmt.Errorf("dialog not initialized")}
		}

		queue := e.multiChartDialog.GetQueue()
		if len(queue) == 0 {
			return multiChartStackSavedMsg{err: fmt.Errorf("no charts in queue")}
		}

		stack := config.StackTemplate{
			Name:        name,
			Description: description,
			Charts:      make([]config.StackChartItem, len(queue)),
		}

		for i, item := range queue {
			stack.Charts[i] = config.StackChartItem{
				ChartRef:        item.ChartRef,
				ReleaseName:     item.ReleaseName,
				Namespace:       item.Namespace,
				Values:          item.Values,
				CreateNamespace: item.CreateNamespace,
				WaitForReady:    item.WaitForReady,
			}
		}

		if err := e.config.AddStack(stack); err != nil {
			return multiChartStackSavedMsg{err: err}
		}

		return multiChartStackSavedMsg{name: stack.Name}
	}
}

// loadMultiChartStackByName loads a named stack from config into the queue.
func (e *ExploreView) loadMultiChartStackByName(name string) tea.Cmd {
	return func() tea.Msg {
		if e.multiChartDialog == nil {
			return multiChartStackLoadedMsg{err: fmt.Errorf("dialog not initialized")}
		}

		stack, ok := e.config.GetStack(name)
		if !ok {
			return multiChartStackLoadedMsg{err: fmt.Errorf("stack %q not found", name)}
		}

		e.multiChartDialog.ClearQueue()
		for _, sc := range stack.Charts {
			item := dialog.MultiChartQueueItem{
				ChartRef:        sc.ChartRef,
				ChartName:       sc.ChartRef,
				ReleaseName:     sc.ReleaseName,
				Namespace:       sc.Namespace,
				Values:          sc.Values,
				CreateNamespace: sc.CreateNamespace,
				WaitForReady:    sc.WaitForReady,
				Status:          dialog.MultiChartPending,
			}
			e.multiChartDialog.AddToQueue(item)
		}

		return multiChartStackLoadedMsg{name: stack.Name}
	}
}

func (e *ExploreView) openReadmeDialog() {
	if e.readmeDialog == nil {
		return
	}

	chartName := ""
	content := ""
	if e.loadedChart != nil {
		chartName = e.loadedChart.Info.Name
		content = e.loadedChart.Readme
	}

	e.readmeDialog.OpenWithContent(content, chartName)
}

// --- Security Scan Dialog Functions ---

func (e *ExploreView) openSecurityDialog() {
	if e.securityDialog == nil {
		return
	}
	chartName := ""
	chartVersion := ""
	if e.loadedChart != nil {
		chartName = e.loadedChart.Info.Name
		chartVersion = e.loadedChart.Info.Version
	}
	e.securityDialog.OpenForChart(chartName, chartVersion)
}

func (e *ExploreView) startSecurityScan() tea.Cmd {
	return func() tea.Msg {
		trivyClient := e.securityProvider.GetTrivyClient()
		if trivyClient == nil || !trivyClient.IsAvailable() {
			return securityScanResultMsg{
				err: fmt.Errorf("trivy not installed. Install: https://trivy.dev/docs/latest/getting-started/installation/"),
			}
		}

		// Try to scan rendered manifest for more accurate scanning
		if e.loadedChart != nil && e.loadedChart.Preview != "" {
			results, err := trivyClient.ScanRenderedManifest(e.loadedChart.Preview)
			return securityScanResultMsg{results: results, err: err}
		}

		// Fallback: scan chart directory if available
		if e.chartRef != "" {
			results, err := trivyClient.ScanChartDirectory(e.chartRef)
			return securityScanResultMsg{results: results, err: err}
		}

		return securityScanResultMsg{
			err: fmt.Errorf("no chart loaded to scan"),
		}
	}
}

// --- Dependency Tree Dialog Functions ---

func (e *ExploreView) openDepTreeDialog() {
	e.showDepTree = true
	e.depTreeViewport.GotoTop()

	// Build dependency tree content
	if e.loadedChartRaw != nil {
		tree := helm.BuildDependencyTree(e.loadedChartRaw)
		content := renderDependencyTree(tree, 0, true)
		e.depTreeViewport.SetContent(content)
	}
}

func (e *ExploreView) updateDepTreeDialog(msg tea.KeyMsg) (*ExploreView, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		e.showDepTree = false
		return e, nil
	case "up", "k":
		e.depTreeViewport.LineUp(1)
	case "down", "j":
		e.depTreeViewport.LineDown(1)
	case "pgup", "ctrl+u":
		e.depTreeViewport.HalfViewUp()
	case "pgdown", "ctrl+d":
		e.depTreeViewport.HalfViewDown()
	case "home", "g":
		e.depTreeViewport.GotoTop()
	case "end", "G":
		e.depTreeViewport.GotoBottom()
	}
	return e, nil
}

// renderDependencyTree renders a dependency tree recursively
func renderDependencyTree(dep *helm.ChartDependency, depth int, isLast bool) string {
	if dep == nil {
		return ""
	}

	var lines []string

	// Build prefix for tree structure
	prefix := ""
	if depth > 0 {
		prefix = strings.Repeat("│   ", depth-1)
		if isLast {
			prefix += "└── "
		} else {
			prefix += "├── "
		}
	}

	// Icon based on status
	icon := "✓"
	style := S.Success
	if !dep.IsLoaded {
		if dep.IsOptional {
			icon = "○"
			style = S.Muted
		} else {
			icon = "✗"
			style = S.Error
		}
	}

	// Build line: icon name version (condition)
	line := prefix + icon + " " + style.Render(dep.Name)
	if dep.Version != "" {
		line += S.Muted.Render(" v" + dep.Version)
	}
	if dep.Condition != "" {
		line += S.Muted.Render(" (" + dep.Condition + ")")
	}
	lines = append(lines, line)

	// Recurse children
	for i, child := range dep.Children {
		isChildLast := i == len(dep.Children)-1
		lines = append(lines, renderDependencyTree(child, depth+1, isChildLast))
	}

	return strings.Join(lines, "\n")
}

func (e *ExploreView) renderDepTreeDialog() string {
	dialogWidth := 55
	dialogHeight := min(e.height-10, 25)
	if dialogHeight < 10 {
		dialogHeight = 10
	}

	dialogStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(DefaultTheme.Primary).
		Padding(1, 2).
		Width(dialogWidth)

	var content strings.Builder

	// Header
	chartName := ""
	chartVersion := ""
	if e.loadedChart != nil {
		chartName = e.loadedChart.Info.Name
		chartVersion = e.loadedChart.Info.Version
	}
	content.WriteString(S.Title.Render(Icons.Helm + " Dependencies: " + chartName + " v" + chartVersion))
	content.WriteString("\n\n")

	// Tree content in viewport
	e.depTreeViewport.Width = dialogWidth - 6
	e.depTreeViewport.Height = dialogHeight - 8
	content.WriteString(e.depTreeViewport.View())
	content.WriteString("\n\n")

	// Legend
	content.WriteString(S.Muted.Render("✓ loaded  ○ optional  ✗ missing"))
	content.WriteString("\n")

	// Footer
	content.WriteString(S.Muted.Render("j/k:scroll  Esc:close"))

	dialog := dialogStyle.Render(content.String())
	return lipgloss.Place(e.width, e.height, lipgloss.Center, lipgloss.Center, dialog)
}

// --- Schema Browser Dialog Functions ---

func (e *ExploreView) openSchemaDialog() {
	if e.schemaDialog == nil {
		return
	}
	chartName := ""
	var schemaBytes []byte
	if e.loadedChart != nil {
		chartName = e.loadedChart.Info.Name
		schemaBytes = e.loadedChart.Values.Schema
	}
	e.schemaDialog.OpenWithSchema(schemaBytes, chartName)
}

// --- Template Dialog Functions ---

func (e *ExploreView) openTemplateDialog() {
	if e.templateDialog == nil {
		return
	}
	chartName := "chart"
	chartVersion := ""
	defaultValues := "# Custom values (YAML)\n"
	if e.loadedChart != nil {
		chartName = e.loadedChart.Info.Name
		chartVersion = e.loadedChart.Info.Version
		if e.loadedChart.Values.Raw != "" {
			defaultValues = e.loadedChart.Values.Raw
		}
	}
	e.templateDialog.OpenWithChart(chartName, chartVersion, defaultValues)
}

// updateInstallPreview handles key events in the install preview dialog
func (e *ExploreView) updateInstallPreview(msg tea.KeyMsg) (*ExploreView, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg.String() {
	case "esc", "q":
		// Close preview, go back to install dialog
		e.showInstallPreview = false
		return e, nil

	case "enter":
		// Confirm install from preview
		e.showInstallPreview = false
		if !e.installing {
			e.installing = true
			cmds = append(cmds, e.spinner.Start("Installing..."))
			cmds = append(cmds, e.executeInstall())
			return e, tea.Batch(cmds...)
		}

	case "e":
		// Edit values and go back to install dialog
		e.showInstallPreview = false
		return e, e.openExternalEditor()

	case "j", "down":
		e.installPreviewViewport.LineDown(1)
	case "k", "up":
		e.installPreviewViewport.LineUp(1)
	case "ctrl+d":
		e.installPreviewViewport.HalfViewDown()
	case "ctrl+u":
		e.installPreviewViewport.HalfViewUp()
	case "g":
		e.installPreviewViewport.GotoTop()
	case "G":
		e.installPreviewViewport.GotoBottom()
	}

	return e, tea.Batch(cmds...)
}

// View renders the explore view
func (e *ExploreView) View() string {
	if e.width == 0 {
		return "Loading..."
	}

	// If load stack dialog is open, render it on top
	if e.loadStackDialog != nil && e.loadStackDialog.IsOpen() {
		return e.loadStackDialog.View()
	}

	// If save stack dialog is open, render it on top
	if e.saveStackDialog != nil && e.saveStackDialog.IsOpen() {
		return e.saveStackDialog.View()
	}

	// If save template dialog is open, render it on top
	if e.saveTemplateDialog != nil && e.saveTemplateDialog.IsOpen() {
		return e.saveTemplateDialog.View()
	}

	// If file import dialog is open, render it on top
	if e.fileImportDialog != nil && e.fileImportDialog.IsOpen() {
		return e.fileImportDialog.View()
	}

	// If set value dialog is open, render it on top
	if e.setValueDialog != nil && e.setValueDialog.IsOpen() {
		return e.setValueDialog.View()
	}

	// If value sources dialog is open, render it on top
	if e.valueSourcesDialog != nil && e.valueSourcesDialog.IsOpen() {
		return e.valueSourcesDialog.View()
	}

	// If install preview dialog is open, render it on top
	if e.showInstallPreview {
		return e.renderInstallPreview()
	}

	// If install dialog is open, render it on top
	if e.installDialog != nil && e.installDialog.IsOpen() {
		return e.installDialog.View()
	}

	// If multi-chart dialog is open, render it on top
	if e.multiChartDialog != nil && e.multiChartDialog.IsOpen() {
		return e.multiChartDialog.View()
	}

	// If progress dialog is open, render it on top
	if e.progressDialog != nil && e.progressDialog.IsOpen() {
		return e.progressDialog.View()
	}

	// If README dialog is open, render it on top
	if e.readmeDialog != nil && e.readmeDialog.IsOpen() {
		return e.readmeDialog.View()
	}

	// If template dialog is open, render it on top
	if e.templateDialog != nil && e.templateDialog.IsOpen() {
		return e.templateDialog.View()
	}

	// If version dialog is open, render it on top
	if e.versionDialog != nil && e.versionDialog.IsOpen() {
		return e.versionDialog.View()
	}

	// If security dialog is open, render it on top
	if e.securityDialog != nil && e.securityDialog.IsOpen() {
		return e.securityDialog.View()
	}

	// If dependency tree dialog is open, render it on top
	if e.showDepTree {
		return e.renderDepTreeDialog()
	}

	// If schema browser dialog is open, render it on top
	if e.schemaDialog != nil && e.schemaDialog.IsOpen() {
		return e.schemaDialog.View()
	}

	return e.renderMainView()
}

func (e *ExploreView) renderMainView() string {
	// Use adaptive pane layout
	layout := CalculatePaneLayout(e.width, e.height, 0.35)

	// Show any errors prominently at the top
	var errorBanner string
	if e.err != nil {
		errorBanner = S.Error.Render(Icons.Cross+" Error: "+e.err.Error()) + "\n"
	}

	// Search box
	searchStyle := S.BorderNormal
	if e.activePane == PaneSearch {
		searchStyle = S.BorderFocus
	}
	searchContent := S.Title.Render(Icons.Search+" Search") + "\n" + e.searchInput.View()
	if e.spinner.IsActive() {
		searchContent += "\n" + e.spinner.View()
	}
	searchBox := searchStyle.Width(layout.LeftWidth).Render(searchContent)

	// Chart list
	listStyle := S.BorderNormal
	if e.activePane == PaneChartList {
		listStyle = S.BorderFocus
	}
	chartList := e.renderChartList(layout.LeftWidth)
	chartListBox := listStyle.Width(layout.LeftWidth).Height(layout.TopHeight).Render(
		S.Title.Render(Icons.Chart+" Charts") + "\n" + chartList,
	)

	// Values pane
	valuesStyle := S.BorderNormal
	if e.activePane == PaneValues {
		valuesStyle = S.BorderFocus
	}
	valuesBox := valuesStyle.Width(layout.RightWidth).Height(layout.TopHeight).Render(
		S.Title.Render(Icons.Values+" Values") + "\n" + e.valuesView.View(),
	)

	// Preview pane
	previewStyle := S.BorderNormal
	if e.activePane == PanePreview {
		previewStyle = S.BorderFocus
	}
	previewBox := previewStyle.Width(layout.RightWidth).Height(layout.BottomHeight).Render(
		S.Title.Render(Icons.Release+" Preview") + "\n" + e.previewView.View(),
	)

	// Layout: left column (search + list) | right column (values + preview)
	leftColumn := lipgloss.JoinVertical(lipgloss.Left, searchBox, chartListBox)
	rightColumn := lipgloss.JoinVertical(lipgloss.Left, valuesBox, previewBox)

	main := lipgloss.JoinHorizontal(lipgloss.Top, leftColumn, " ", rightColumn)

	// Add top spacing and error banner if any
	return "\n" + errorBanner + main
}

// renderInstallPreview renders the dry-run preview dialog
func (e *ExploreView) renderInstallPreview() string {
	// Calculate dialog size
	dialogWidth := e.width - 10
	dialogHeight := e.height - 6
	if dialogWidth < 60 {
		dialogWidth = 60
	}
	if dialogHeight < 20 {
		dialogHeight = 20
	}

	// Update viewport size
	e.installPreviewViewport.Width = dialogWidth - 6
	e.installPreviewViewport.Height = dialogHeight - 14 // Reserve space for header, resources, and footer

	dialogStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(DefaultTheme.Primary).
		Padding(1, 2).
		Width(dialogWidth)

	var content strings.Builder
	content.WriteString(S.Title.Render(Icons.Release+" Dry-Run Preview") + "\n\n")

	// Show chart info
	if e.loadedChart != nil {
		content.WriteString(S.Label.Render("Chart: ") + e.loadedChart.Info.Name + " v" + e.loadedChart.Info.Version + "\n")
	}
	content.WriteString(S.Label.Render("Release: ") + e.releaseNameInput.Value() + "\n")
	ns := e.namespaceInput.Value()
	if ns == "" {
		ns = "default"
	}
	content.WriteString(S.Label.Render("Namespace: ") + ns + "\n\n")

	// Show error if any
	if e.installPreviewError != nil {
		content.WriteString(S.Error.Render(Icons.Cross+" Error: "+e.installPreviewError.Error()) + "\n\n")
		content.WriteString(S.Muted.Render("Esc:back  e:edit values"))
		dialog := dialogStyle.Render(content.String())
		return lipgloss.Place(e.width, e.height, lipgloss.Center, lipgloss.Center, dialog)
	}

	// Show resources summary
	if len(e.installPreviewResources) > 0 {
		content.WriteString(S.Highlighted.Render("Will create:") + "\n")
		// Group by kind and count
		kindCounts := make(map[string]int)
		for _, res := range e.installPreviewResources {
			kindCounts[res]++
		}
		for kind, count := range kindCounts {
			if count > 1 {
				content.WriteString(fmt.Sprintf("  %s %s (%d)\n", Icons.Check, kind, count))
			} else {
				content.WriteString(fmt.Sprintf("  %s %s\n", Icons.Check, kind))
			}
		}
		content.WriteString("\n")
	}

	// Divider
	divider := S.Muted.Render(strings.Repeat("─", dialogWidth-6))
	content.WriteString(divider + "\n")

	// Manifest viewport
	content.WriteString(e.installPreviewViewport.View() + "\n")

	// Scroll indicator
	scrollPct := int(e.installPreviewViewport.ScrollPercent() * 100)
	scrollInfo := fmt.Sprintf(" %d%% ", scrollPct)
	content.WriteString(divider + "\n")

	// Footer
	content.WriteString(S.Muted.Render(fmt.Sprintf("j/k:scroll %s  Enter:install  e:edit  Esc:back", scrollInfo)))

	dialog := dialogStyle.Render(content.String())
	return lipgloss.Place(e.width, e.height, lipgloss.Center, lipgloss.Center, dialog)
}

func (e *ExploreView) renderChartList(width int) string {
	if e.totalResults() == 0 {
		return S.Muted.Render("No charts found. Try searching above.")
	}

	var lines []string
	idx := 0

	// Render local results first
	for _, chart := range e.searchResults {
		name := chart.Name
		if len(name) > width-4 {
			name = name[:width-7] + "..."
		}

		desc := chart.Description
		if len(desc) > width-6 {
			desc = desc[:width-9] + "..."
		}

		var line string
		if idx == e.selectedIndex {
			line = S.Selected.Render(Icons.Arrow + " " + name)
		} else {
			line = S.Value.Render("  " + name)
		}
		line += "\n" + S.Muted.Render("    "+desc)
		lines = append(lines, line)
		idx++
	}

	// Add separator and hub results if any
	if len(e.hubResults) > 0 {
		if len(e.searchResults) > 0 {
			lines = append(lines, S.Muted.Render("─── Artifact Hub ───"))
		}
		for _, hub := range e.hubResults {
			name := fmt.Sprintf("%s/%s", hub.RepoName, hub.Name)
			if len(name) > width-8 {
				name = name[:width-11] + "..."
			}

			// Add star count and cloud icon
			starStr := ""
			if hub.Stars > 0 {
				starStr = fmt.Sprintf(" (%d★)", hub.Stars)
			}

			desc := hub.Description
			if len(desc) > width-6 {
				desc = desc[:width-9] + "..."
			}

			var line string
			if idx == e.selectedIndex {
				line = S.Selected.Render(Icons.Arrow + " ☁ " + name + starStr)
			} else {
				line = S.Value.Render("  ☁ " + name + starStr)
			}
			line += "\n" + S.Muted.Render("    "+desc)
			lines = append(lines, line)
			idx++
		}
	}

	return strings.Join(lines, "\n")
}

// totalResults returns the total number of results (local + hub)
func (e *ExploreView) totalResults() int {
	return len(e.searchResults) + len(e.hubResults)
}

// getSelectedChartRef returns the chart reference for the selected item
func (e *ExploreView) getSelectedChartRef() string {
	localCount := len(e.searchResults)

	if e.selectedIndex < localCount {
		// Local result
		return e.searchResults[e.selectedIndex].Name
	}

	// Hub result - use repo/name format
	hubIdx := e.selectedIndex - localCount
	if hubIdx < len(e.hubResults) {
		hub := e.hubResults[hubIdx]
		return fmt.Sprintf("%s/%s", hub.RepoName, hub.Name)
	}

	return ""
}

// getSelectedHubResult returns the Hub result if selected, nil otherwise
func (e *ExploreView) getSelectedHubResult() *helm.ArtifactHubResult {
	localCount := len(e.searchResults)

	if e.selectedIndex < localCount {
		return nil // Local result
	}

	hubIdx := e.selectedIndex - localCount
	if hubIdx < len(e.hubResults) {
		return &e.hubResults[hubIdx]
	}

	return nil
}

func (e *ExploreView) updateLayout() {
	layout := CalculatePaneLayout(e.width, e.height, 0.35)

	e.valuesView.Width = layout.RightWidth - 4
	e.valuesView.Height = layout.TopHeight - 4
	e.previewView.Width = layout.RightWidth - 4
	e.previewView.Height = layout.BottomHeight - 4
	e.searchInput.Width = layout.LeftWidth - 4
}

// FocusSearch switches to the search pane and resets for a new query.
func (e *ExploreView) FocusSearch() {
	e.activePane = PaneSearch
	e.searchInput.SetValue("")
	e.searchInput.Focus()
	e.searchResults = nil
	e.hubResults = nil
	e.selectedIndex = 0
	e.loadedChart = nil
	e.loadedChartRaw = nil
	e.chartRef = ""
	e.err = nil
}

func (e *ExploreView) updateFocus() {
	// Only the search input can be "focused" in the textinput sense
	if e.activePane == PaneSearch {
		e.searchInput.Focus()
	} else {
		e.searchInput.Blur()
	}
}

func (e *ExploreView) updateContent() {
	if e.loadedChart == nil {
		return
	}

	// Update values view with parsed values as tree
	valuesContent := renderValuesTree(e.loadedChart.Values.Parsed, "", 0)
	if valuesContent == "" {
		valuesContent = "No values defined"
	}
	e.valuesView.SetContent(valuesContent)

	// Update preview with resources
	var previewLines []string
	previewLines = append(previewLines, fmt.Sprintf("Chart: %s v%s", e.loadedChart.Info.Name, e.loadedChart.Info.Version))
	previewLines = append(previewLines, "")
	previewLines = append(previewLines, "Resources to be created:")
	for _, res := range e.loadedChart.Resources {
		previewLines = append(previewLines, "  • "+res)
	}
	if e.loadedChart.Preview != "" {
		previewLines = append(previewLines, "")
		previewLines = append(previewLines, "--- Manifest Preview ---")
		// Show first 50 lines of manifest
		manifestLines := strings.Split(e.loadedChart.Preview, "\n")
		if len(manifestLines) > 50 {
			manifestLines = manifestLines[:50]
			manifestLines = append(manifestLines, "... (truncated)")
		}
		previewLines = append(previewLines, manifestLines...)
	}
	e.previewView.SetContent(strings.Join(previewLines, "\n"))
}

// renderValuesTree renders a map as an indented tree
func renderValuesTree(values map[string]interface{}, prefix string, depth int) string {
	if depth > 10 { // Prevent infinite recursion
		return prefix + "..."
	}

	var lines []string
	indent := strings.Repeat("  ", depth)

	keyStyle := lipgloss.NewStyle().Foreground(DefaultTheme.Secondary) // Blue
	valStyle := lipgloss.NewStyle().Foreground(DefaultTheme.Success)   // Green

	for key, val := range values {
		switch v := val.(type) {
		case map[string]interface{}:
			lines = append(lines, indent+keyStyle.Render(key)+":")
			lines = append(lines, renderValuesTree(v, prefix, depth+1))
		case []interface{}:
			lines = append(lines, indent+keyStyle.Render(key)+": "+valStyle.Render(fmt.Sprintf("[%d items]", len(v))))
		default:
			lines = append(lines, indent+keyStyle.Render(key)+": "+valStyle.Render(fmt.Sprintf("%v", v)))
		}
	}

	return strings.Join(lines, "\n")
}

// --- Progress Dialog Functions ---

// startProgressMonitoring begins the progress dialog after successful install
func (e *ExploreView) startProgressMonitoring(release *helm.Release) tea.Cmd {
	if e.progressDialog == nil {
		return nil
	}
	// OpenWithRelease returns commands for initial fetch and tick scheduling
	return e.progressDialog.OpenWithRelease(release.Name, release.Namespace)
}

// fetchProgress fetches current pod status, resources, and events
func (e *ExploreView) fetchProgress() tea.Cmd {
	if e.progressDialog == nil || !e.progressDialog.IsOpen() {
		return nil
	}

	release := e.progressDialog.GetRelease()
	releaseName := release.Name
	namespace := release.Namespace

	return func() tea.Msg {
		// Get pod status
		pods, podErr := e.clusterInspector.GetReleasePodStatus(releaseName, namespace)

		// Get all resource statuses
		resources, resErr := e.clusterInspector.GetReleaseResourceStatus(releaseName, namespace)

		// Get events
		events, eventErr := e.clusterInspector.GetReleaseEvents(releaseName, namespace)

		// Check if all ready
		allReady, readyErr := e.clusterInspector.AllPodsReady(releaseName, namespace)

		// Combine errors
		var err error
		if podErr != nil {
			err = podErr
		} else if resErr != nil {
			err = resErr
		} else if eventErr != nil {
			err = eventErr
		} else if readyErr != nil {
			err = readyErr
		}

		return installProgressMsg{
			pods:      pods,
			resources: resources,
			events:    events,
			allReady:  allReady,
			err:       err,
		}
	}
}

// openInstallDialogEditor opens the install dialog's values in an external editor
func (e *ExploreView) openInstallDialogEditor() tea.Cmd {
	content := ""
	if e.installDialog != nil {
		content = e.installDialog.GetValues()
	}

	tmpFile, err := os.CreateTemp("", "helmx-install-values-*.yaml")
	if err != nil {
		return func() tea.Msg {
			return valuesEditedMsg{err: err}
		}
	}
	tmpPath := tmpFile.Name()

	if _, err := tmpFile.WriteString(content); err != nil {
		_ = tmpFile.Close()
		_ = os.Remove(tmpPath)
		return func() tea.Msg {
			return valuesEditedMsg{err: err}
		}
	}
	_ = tmpFile.Close()

	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vim"
	}

	cmd := exec.Command(editor, tmpPath)
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		defer os.Remove(tmpPath)
		if err != nil {
			return valuesEditedMsg{err: err}
		}
		data, err := os.ReadFile(tmpPath)
		if err != nil {
			return valuesEditedMsg{err: err}
		}
		return valuesEditedMsg{content: string(data)}
	})
}

// openExternalEditor opens the values in an external editor ($EDITOR or vim)
func (e *ExploreView) openExternalEditor() tea.Cmd {
	content := e.valuesContent

	// Create temp file
	tmpFile, err := os.CreateTemp("", "helmx-values-*.yaml")
	if err != nil {
		return func() tea.Msg {
			return valuesEditedMsg{err: err}
		}
	}
	tmpPath := tmpFile.Name()

	// Write content to temp file
	if _, err := tmpFile.WriteString(content); err != nil {
		_ = tmpFile.Close()
		_ = os.Remove(tmpPath)
		return func() tea.Msg {
			return valuesEditedMsg{err: err}
		}
	}
	_ = tmpFile.Close()

	// Get editor from $EDITOR, fallback to vim, then nano
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vim"
	}

	// Use tea.ExecProcess to suspend TUI and run editor
	cmd := exec.Command(editor, tmpPath)
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		defer os.Remove(tmpPath)

		if err != nil {
			return valuesEditedMsg{err: err}
		}

		// Read modified content
		data, err := os.ReadFile(tmpPath)
		if err != nil {
			return valuesEditedMsg{err: err}
		}

		return valuesEditedMsg{content: string(data)}
	})
}

// openTemplateExternalEditor opens an external editor for template values
func (e *ExploreView) openTemplateExternalEditor() tea.Cmd {
	content := ""
	if e.templateDialog != nil {
		content = e.templateDialog.GetValues()
	}

	// Create temp file
	tmpFile, err := os.CreateTemp("", "helmx-template-values-*.yaml")
	if err != nil {
		return func() tea.Msg {
			return templateValuesEditedMsg{err: err}
		}
	}
	tmpPath := tmpFile.Name()

	// Write content to temp file
	if _, err := tmpFile.WriteString(content); err != nil {
		_ = tmpFile.Close()
		_ = os.Remove(tmpPath)
		return func() tea.Msg {
			return templateValuesEditedMsg{err: err}
		}
	}
	_ = tmpFile.Close()

	// Get editor from $EDITOR, fallback to vim
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vim"
	}

	// Use tea.ExecProcess to suspend TUI and run editor
	cmd := exec.Command(editor, tmpPath)
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		defer os.Remove(tmpPath)

		if err != nil {
			return templateValuesEditedMsg{err: err}
		}

		// Read modified content
		data, err := os.ReadFile(tmpPath)
		if err != nil {
			return templateValuesEditedMsg{err: err}
		}

		return templateValuesEditedMsg{content: string(data)}
	})
}

// executeTemplateToFile renders the chart to a file using provided values (dialog version)
func (e *ExploreView) executeTemplateToFile(outputPath, values string, valueFiles, setValues []string) tea.Cmd {
	chartRef := e.chartRef

	return func() tea.Msg {
		// Validate output path for security (prevent path traversal)
		if err := validation.ValidateOutputPath(outputPath); err != nil {
			return templateResultMsg{err: fmt.Errorf("invalid output path: %v", err)}
		}

		// Merge values from all sources
		composer := &helm.ValuesComposer{
			ValueFiles: valueFiles,
			InlineYAML: values,
			SetValues:  setValues,
		}
		valuesMap, err := composer.Compose()
		if err != nil {
			return templateResultMsg{err: fmt.Errorf("failed to merge values: %v", err)}
		}

		// Generate a release name for templating
		releaseName := "release"
		if e.loadedChart != nil {
			releaseName = e.loadedChart.Info.Name
		}

		// Render the chart
		manifest, err := e.releaseManager.Template(chartRef, releaseName, "default", valuesMap)
		if err != nil {
			return templateResultMsg{err: fmt.Errorf("template failed: %v", err)}
		}

		// Create parent directory if needed
		dir := filepath.Dir(outputPath)
		if dir != "" && dir != "." {
			if err := os.MkdirAll(dir, 0755); err != nil {
				return templateResultMsg{err: fmt.Errorf("failed to create directory: %v", err)}
			}
		}

		// Write to file
		if err := os.WriteFile(outputPath, []byte(manifest), 0644); err != nil {
			return templateResultMsg{err: fmt.Errorf("failed to write file: %v", err)}
		}

		return templateResultMsg{outputPath: outputPath}
	}
}

// convertPodsToDialogFormat converts helm.PodStatus to dialog.PodStatus
func convertPodsToDialogFormat(pods []helm.PodStatus) []dialog.PodStatus {
	result := make([]dialog.PodStatus, len(pods))
	for i, p := range pods {
		containers := make([]dialog.ContainerStatus, len(p.ContainerStatus))
		for j, c := range p.ContainerStatus {
			containers[j] = dialog.ContainerStatus{
				Name:   c.Name,
				State:  c.State,
				Reason: c.Reason,
			}
		}
		result[i] = dialog.PodStatus{
			Name:            p.Name,
			Status:          p.Status,
			Ready:           p.Ready,
			Restarts:        int(p.Restarts),
			ContainerStatus: containers,
		}
	}
	return result
}

// convertResourcesToDialogFormat converts helm.ResourceStatus to dialog.ResourceStatus
func convertResourcesToDialogFormat(resources []helm.ResourceStatus) []dialog.ResourceStatus {
	result := make([]dialog.ResourceStatus, len(resources))
	for i, r := range resources {
		result[i] = dialog.ResourceStatus{
			Kind:   r.Kind,
			Name:   r.Name,
			Ready:  r.Ready,
			Status: r.Status,
		}
	}
	return result
}

// convertEventsToDialogFormat converts helm.K8sEvent to dialog.EventInfo
func convertEventsToDialogFormat(events []helm.K8sEvent) []dialog.EventInfo {
	result := make([]dialog.EventInfo, len(events))
	for i, e := range events {
		result[i] = dialog.EventInfo{
			Type:      e.Type,
			Reason:    e.Reason,
			Object:    e.Object,
			Message:   e.Message,
			Timestamp: e.Timestamp,
		}
	}
	return result
}

// isLocalPath checks if the query looks like a local filesystem path
func isLocalPath(query string) bool {
	// Absolute paths (Unix)
	if strings.HasPrefix(query, "/") {
		return true
	}
	// Relative paths
	if strings.HasPrefix(query, "./") || strings.HasPrefix(query, "../") {
		return true
	}
	// Home directory paths
	if strings.HasPrefix(query, "~/") {
		return true
	}
	// Windows absolute paths (C:\, D:\, etc.)
	if len(query) >= 2 && query[1] == ':' {
		return true
	}
	return false
}

func (e *ExploreView) searchCharts(query string) tea.Cmd {
	return func() tea.Msg {
		if query == "" {
			return searchResultsMsg{localResults: nil, hubResults: nil}
		}

		// Check if it's a local filesystem path
		if isLocalPath(query) {
			return searchResultsMsg{
				localResults: []helm.ChartInfo{{
					Name:        query,
					Description: "Local chart - press Enter to load",
				}},
			}
		}

		// Check if it's an OCI reference
		if helm.IsOCIReference(query) {
			return searchResultsMsg{
				localResults: []helm.ChartInfo{{
					Name:        query,
					Description: "OCI registry chart - press Enter to load",
				}},
			}
		}

		// If it looks like a direct chart reference (contains /), try to load it directly
		if strings.Contains(query, "/") {
			return searchResultsMsg{
				localResults: []helm.ChartInfo{{
					Name:        query,
					Description: "Direct chart reference - press Enter to load",
				}},
			}
		}

		// Search local repos first
		localResults, err := e.chartExplorer.SearchRepos(query)
		if err != nil {
			// Continue even if local search fails
			localResults = nil
		}

		// Search all configured chart registries
		var hubResults []helm.ArtifactHubResult
		if e.config != nil {
			registries := e.config.GetChartRegistries()
			for _, reg := range registries {
				results, regErr := helm.SearchArtifactHub(query, 10, reg.URL)
				if regErr != nil {
					continue // Skip failed registries
				}
				// Tag results with registry name for display
				for i := range results {
					if results[i].RepoName == "" {
						results[i].RepoName = reg.Name
					}
				}
				hubResults = append(hubResults, results...)
			}
		} else {
			// Fallback to default registry
			results, _ := helm.SearchArtifactHub(query, 10, "")
			hubResults = results
		}

		// Filter out hub results that are already in local results
		if len(localResults) > 0 && len(hubResults) > 0 {
			localNames := make(map[string]bool)
			for _, r := range localResults {
				localNames[r.Name] = true
			}
			var filteredHub []helm.ArtifactHubResult
			for _, h := range hubResults {
				hubName := fmt.Sprintf("%s/%s", h.RepoName, h.Name)
				if !localNames[hubName] {
					filteredHub = append(filteredHub, h)
				}
			}
			hubResults = filteredHub
		}

		return searchResultsMsg{localResults: localResults, hubResults: hubResults, err: err}
	}
}

func (e *ExploreView) loadChart(chartRef string) tea.Cmd {
	// Capture hub result info before returning the command
	hubResult := e.getSelectedHubResult()

	return func() tea.Msg {
		// If this is a Hub result, check if repo exists and add if needed
		if hubResult != nil && hubResult.RepoURL != "" {
			// Check if repo already exists
			repos, reposErr := e.repoManager.ListRepos()
			if reposErr != nil {
				return chartLoadedMsg{err: fmt.Errorf("failed to list repos: %w", reposErr)}
			}

			repoExists := false
			for _, r := range repos {
				if r.Name == hubResult.RepoName {
					repoExists = true
					break
				}
			}

			// Add repo if it doesn't exist (public repos from Artifact Hub, no auth needed)
			if !repoExists {
				if err := e.repoManager.AddRepo(hubResult.RepoName, hubResult.RepoURL, nil); err != nil {
					return chartLoadedMsg{err: fmt.Errorf("failed to add repo %s (%s): %w", hubResult.RepoName, hubResult.RepoURL, err)}
				}
			} else {
				// Repo exists but might need update - try updating
				// This helps when the local index is stale
				_ = e.repoManager.UpdateRepo(hubResult.RepoName)
			}
		}

		chart, err := e.chartExplorer.LoadChart(chartRef)
		if err != nil {
			// Provide detailed error context for debugging
			errMsg := fmt.Sprintf("failed to load chart %q", chartRef)
			if hubResult != nil {
				errMsg += fmt.Sprintf(" (repo: %s, url: %s)", hubResult.RepoName, hubResult.RepoURL)
			}
			return chartLoadedMsg{err: fmt.Errorf("%s: %w", errMsg, err)}
		}

		info := e.chartExplorer.GetChartInfo(chart)
		values := e.chartExplorer.GetChartValues(chart)

		// Get README from chart
		readme := e.chartExplorer.GetChartReadme(chart)

		// If no README in chart and this is an Artifact Hub result, try fetching from API
		if readme == "" && hubResult != nil {
			hubReadme, _ := helm.GetArtifactHubReadme(hubResult.RepoName, hubResult.Name, "")
			if hubReadme != "" {
				readme = hubReadme
			}
		}

		// Try to get a preview (dry-run install)
		preview, resources, _ := e.chartExplorer.PreviewInstall(chartRef, "preview", "default", nil)

		return chartLoadedMsg{
			data: &LoadedChartData{
				Info:      info,
				Values:    values,
				Preview:   preview,
				Resources: resources,
				Readme:    readme,
			},
			rawChart: chart,
		}
	}
}

func (e *ExploreView) executeInstall() tea.Cmd {
	e.installing = true

	releaseName := e.releaseNameInput.Value()
	namespace := e.namespaceInput.Value()
	if namespace == "" {
		namespace = "default"
	}

	chartRef := e.chartRef
	createNS := e.createNamespace
	valuesContent := e.valuesContent

	return func() tea.Msg {
		// Validate release name
		if err := validation.ValidateReleaseName(releaseName); err != nil {
			return installResultMsg{err: fmt.Errorf("invalid release name: %v", err)}
		}

		// Validate namespace
		if err := validation.ValidateNamespace(namespace); err != nil {
			return installResultMsg{err: fmt.Errorf("invalid namespace: %v", err)}
		}

		// Parse values from content
		var values map[string]interface{}
		if strings.TrimSpace(valuesContent) != "" {
			if err := yaml.Unmarshal([]byte(valuesContent), &values); err != nil {
				return installResultMsg{err: fmt.Errorf("invalid YAML: %v", err)}
			}
		}

		release, err := e.releaseManager.Install(chartRef, releaseName, namespace, values, createNS)
		return installResultMsg{release: release, err: err}
	}
}

// updateInstallPreviewViewport updates the viewport content with the manifest
func (e *ExploreView) updateInstallPreviewViewport() {
	e.installPreviewViewport.SetContent(e.installPreviewManifest)
	e.installPreviewViewport.GotoTop()
}

// --- Version Dialog ---

// openVersionDialog initializes the version selector dialog
func (e *ExploreView) openVersionDialog() {
	if e.versionDialog == nil {
		return
	}

	// Get chart name and ref
	chartName := ""
	chartRef := ""
	isHub := false

	totalLocalResults := len(e.searchResults)
	if e.selectedIndex < totalLocalResults {
		chartName = e.searchResults[e.selectedIndex].Name
		chartRef = chartName
	} else {
		hubIdx := e.selectedIndex - totalLocalResults
		if hubIdx < len(e.hubResults) {
			chartName = e.hubResults[hubIdx].RepoName + "/" + e.hubResults[hubIdx].Name
			chartRef = chartName
			isHub = true
		}
	}

	e.versionDialog.OpenForChart(chartName, chartRef, isHub)
	e.isHubChart = isHub
	e.selectedVersion = ""
}

// fetchChartVersions fetches all versions for the selected chart
func (e *ExploreView) fetchChartVersions() tea.Cmd {
	// Check if selected chart is from local repo or Artifact Hub
	totalLocalResults := len(e.searchResults)

	if e.selectedIndex < totalLocalResults {
		// Local repo chart
		chart := e.searchResults[e.selectedIndex]
		// Parse repo/chartName from chart.Name
		parts := strings.SplitN(chart.Name, "/", 2)
		if len(parts) != 2 {
			return func() tea.Msg {
				return chartVersionsMsg{err: fmt.Errorf("invalid chart reference: %s", chart.Name)}
			}
		}
		repoName, chartName := parts[0], parts[1]

		return func() tea.Msg {
			versions, err := e.chartExplorer.ListChartVersions(repoName, chartName)
			return chartVersionsMsg{versions: versions, isHub: false, err: err}
		}
	}

	// Artifact Hub chart
	hubIdx := e.selectedIndex - totalLocalResults
	if hubIdx >= 0 && hubIdx < len(e.hubResults) {
		hubResult := e.hubResults[hubIdx]
		repoName := hubResult.RepoName
		packageName := hubResult.Name
		registryURL := ""
		// Get registry URL if available
		if e.config != nil {
			registries := e.config.GetChartRegistries()
			for _, reg := range registries {
				if reg.Name == repoName || strings.Contains(hubResult.RepoURL, reg.URL) {
					registryURL = reg.URL
					break
				}
			}
		}

		return func() tea.Msg {
			versions, err := helm.GetArtifactHubVersions(repoName, packageName, registryURL)
			return chartVersionsMsg{hubVersions: versions, isHub: true, err: err}
		}
	}

	return func() tea.Msg {
		return chartVersionsMsg{err: fmt.Errorf("no chart selected")}
	}
}

// loadChartWithVersion loads a chart with a specific version
func (e *ExploreView) loadChartWithVersion(chartRef, version string) tea.Cmd {
	hubResult := e.getSelectedHubResult()

	return func() tea.Msg {
		// If this is a Hub result, check if repo exists and add if needed
		if hubResult != nil && hubResult.RepoURL != "" {
			repos, reposErr := e.repoManager.ListRepos()
			if reposErr != nil {
				return chartLoadedMsg{err: fmt.Errorf("failed to list repos: %w", reposErr)}
			}

			repoExists := false
			for _, r := range repos {
				if r.Name == hubResult.RepoName {
					repoExists = true
					break
				}
			}

			if !repoExists {
				// Public repo from Artifact Hub, no auth needed
				if err := e.repoManager.AddRepo(hubResult.RepoName, hubResult.RepoURL, nil); err != nil {
					return chartLoadedMsg{err: fmt.Errorf("failed to add repo %s (%s): %w", hubResult.RepoName, hubResult.RepoURL, err)}
				}
			} else {
				// Repo exists but might need update
				_ = e.repoManager.UpdateRepo(hubResult.RepoName)
			}
		}

		// Load chart with specific version using ChartPathOptions
		chart, err := e.chartExplorer.LoadChartWithVersion(chartRef, version)
		if err != nil {
			errMsg := fmt.Sprintf("failed to load chart %q version %q", chartRef, version)
			if hubResult != nil {
				errMsg += fmt.Sprintf(" (repo: %s, url: %s)", hubResult.RepoName, hubResult.RepoURL)
			}
			return chartLoadedMsg{err: fmt.Errorf("%s: %w", errMsg, err)}
		}

		info := e.chartExplorer.GetChartInfo(chart)
		values := e.chartExplorer.GetChartValues(chart)

		readme := e.chartExplorer.GetChartReadme(chart)
		if readme == "" && hubResult != nil {
			hubReadme, _ := helm.GetArtifactHubReadme(hubResult.RepoName, hubResult.Name, "")
			if hubReadme != "" {
				readme = hubReadme
			}
		}

		preview, resources, _ := e.chartExplorer.PreviewInstallWithVersion(chartRef, version, "preview", "default", nil)

		return chartLoadedMsg{
			data: &LoadedChartData{
				Info:      info,
				Values:    values,
				Preview:   preview,
				Resources: resources,
				Readme:    readme,
			},
			rawChart: chart,
		}
	}
}

// ============================================================================
// Multi-Chart Install (Stack) Functions
// ============================================================================

// addChartToQueue adds the currently loaded chart to the multi-chart queue
func (e *ExploreView) addChartToQueue() {
	if e.loadedChart == nil || e.multiChartDialog == nil {
		return
	}

	// Create queue item using dialog types
	item := dialog.MultiChartQueueItem{
		ChartRef:        e.chartRef,
		ChartName:       e.loadedChart.Info.Name,
		ChartVersion:    e.loadedChart.Info.Version,
		ReleaseName:     e.loadedChart.Info.Name,
		Namespace:       "default",
		Values:          e.loadedChart.Values.Raw,
		CreateNamespace: true,
		WaitForReady:    true,
		Status:          dialog.MultiChartPending,
	}

	// Add to dialog queue (dialog handles duplicate check)
	e.multiChartDialog.AddToQueue(item)

	// If dialog is not open, show it with the queue
	if !e.multiChartDialog.IsOpen() {
		e.openMultiChartDialog()
	}
}

package tui

import (
	"github.com/yasharora2020/helmx/internal/helm"
	"helm.sh/helm/v3/pkg/chart"
)

// ExplorePane represents which pane is focused in explore view
type ExplorePane int

const (
	PaneSearch ExplorePane = iota
	PaneChartList
	PaneValues
	PanePreview
)

// InstallField represents which field is focused in the install dialog
type InstallField int

const (
	FieldTemplate InstallField = iota // Template selection (first field)
	FieldReleaseName
	FieldNamespace
	FieldCreateNS
	FieldValues
	FieldConfirm
	FieldCancel
)

// TemplateField represents which field is focused in the template dialog
type TemplateField int

const (
	TemplateFieldOutputPath TemplateField = iota
	TemplateFieldValues
	TemplateFieldConfirm
	TemplateFieldCancel
)

// MultiChartQueueItem represents a chart in the multi-chart install queue
type MultiChartQueueItem struct {
	ChartRef        string           // Chart reference (e.g., "bitnami/postgresql")
	ChartName       string           // Display name
	ChartVersion    string           // Chart version
	ReleaseName     string           // Release name
	Namespace       string           // Target namespace
	Values          string           // YAML values content
	CreateNamespace bool             // Create namespace if not exists
	WaitForReady    bool             // Wait for pods to be ready before next
	Status          MultiChartStatus // Installation status
	Error           error            // Error if installation failed
}

// MultiChartStatus represents the status of a chart in the queue
type MultiChartStatus int

const (
	MultiChartPending MultiChartStatus = iota
	MultiChartInstalling
	MultiChartReady
	MultiChartFailed
)

// LoadedChartData holds the currently loaded chart's data
type LoadedChartData struct {
	Info      helm.ChartInfo
	Values    helm.ChartValues
	Preview   string   // Rendered manifest preview
	Resources []string // List of resources that will be created
	Readme    string   // README content
}

// --- Message Types ---
// These are messages used in the tea.Cmd pattern for async operations

// searchResultsMsg contains chart search results from local repos and Artifact Hub
type searchResultsMsg struct {
	localResults []helm.ChartInfo
	hubResults   []helm.ArtifactHubResult
	err          error
}

// chartLoadedMsg is sent when a chart is loaded with its values and metadata
type chartLoadedMsg struct {
	data     *LoadedChartData
	rawChart *chart.Chart // Raw chart for dependency tree
	err      error
}

// chartVersionsMsg contains available versions for a chart
type chartVersionsMsg struct {
	versions    []helm.ChartVersion
	hubVersions []helm.ArtifactHubVersion
	isHub       bool
	err         error
}

// installResultMsg is sent when chart installation completes
type installResultMsg struct {
	release *helm.Release
	err     error
}

// valuesEditedMsg is sent when values are edited in external editor
type valuesEditedMsg struct {
	content string
	err     error
}

// valuesDecryptedMsg is sent when SOPS-encrypted values are decrypted
type valuesDecryptedMsg struct {
	content string
	err     error
}

// templateResultMsg is sent when chart templating completes
type templateResultMsg struct {
	outputPath string
	err        error
}

// templateValuesEditedMsg is sent when template values are edited
type templateValuesEditedMsg struct {
	content string
	err     error
}

// installProgressMsg contains pod status and events during install monitoring
type installProgressMsg struct {
	pods      []helm.PodStatus
	resources []helm.ResourceStatus
	events    []helm.K8sEvent
	allReady  bool
	err       error
}

// securityScanResultMsg contains Trivy scan results
type securityScanResultMsg struct {
	results *helm.TrivyScanSummary
	err     error
}

// installPreviewMsg contains dry-run preview results
type installPreviewMsg struct {
	manifest  string
	resources []string
	err       error
}

// multiChartValuesEditedMsg is sent when values are edited for a queue item
type multiChartValuesEditedMsg struct {
	idx    int
	values string
	err    error
}

// multiChartInstallProgressMsg is sent during multi-chart installation
type multiChartInstallProgressMsg struct {
	idx    int
	status MultiChartStatus
	err    error
}

// multiChartInstallCompleteMsg is sent when multi-chart installation completes
type multiChartInstallCompleteMsg struct {
	err error
}

// multiChartStackSavedMsg is sent when a stack is saved to config
type multiChartStackSavedMsg struct {
	name string
	err  error
}

// multiChartStackLoadedMsg is sent when a stack is loaded from config
type multiChartStackLoadedMsg struct {
	name string
	err  error
}

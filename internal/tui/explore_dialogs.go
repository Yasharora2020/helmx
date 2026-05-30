package tui

// This file provides dialog orchestration for the ExploreView.
// It bridges the extracted dialogs in internal/tui/dialog/ with the ExploreView.
//
// Dialog Architecture:
// - Each dialog is a self-contained component in internal/tui/dialog/
// - Dialogs communicate with the parent via messages (tea.Msg)
// - The parent (ExploreView) handles business logic and data fetching
// - Dialogs only handle UI state and user input
//
// Available Dialogs:
// - dialog.FileImportDialog - Import values from YAML files
// - dialog.VersionDialog - Select chart versions
// - dialog.SchemaDialog - Browse values.schema.json
// - dialog.ReadmeDialog - View chart README with search
// - dialog.SecurityDialog - Trivy security scan results
// - dialog.TemplateDialog - Template chart to file
// - dialog.ProgressDialog - Installation progress monitoring
// - dialog.InstallDialog - Chart installation
//
// Message Flow:
// 1. User action triggers dialog opening via ExploreView
// 2. Dialog handles key events and returns messages
// 3. ExploreView receives messages and performs business logic
// 4. ExploreView updates dialog state with results

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/yasharora2020/helmx/internal/tui/dialog"
)

// initDialogStyles initializes styles for all dialogs.
// Call this after theme changes to update dialog appearance.
func (e *ExploreView) initDialogStyles() {
	// File Import Dialog
	if e.fileImportDialog != nil {
		e.fileImportDialog.SetStyles(
			S.Title,
			S.Label,
			S.Muted,
			S.Error,
			DefaultTheme.BorderFocus,
			Icons.File,
			Icons.Cross,
		)
	}

	// Version Dialog
	if e.versionDialog != nil {
		e.versionDialog.SetStyles(
			S.Title,
			S.Label,
			S.Muted,
			S.Value,
			S.Error,
			S.Warning,
			DefaultTheme.BorderFocus,
			Icons.Chart,
			Icons.Arrow,
			Icons.Cross,
		)
	}

	// Schema Dialog
	if e.schemaDialog != nil {
		e.schemaDialog.SetStyles(
			S.Title,
			S.Muted,
			S.Value,
			S.Error,
			S.Success,
			S.Warning,
			DefaultTheme.BorderFocus,
			Icons.Values,
		)
	}

	// Readme Dialog
	if e.readmeDialog != nil {
		e.readmeDialog.SetStyles(
			S.Title,
			S.Label,
			S.Muted,
			S.Success,
			S.Warning,
			S.Highlighted,
			DefaultTheme.BorderFocus,
			Icons.Chart,
		)
	}

	// Security Dialog
	if e.securityDialog != nil {
		e.securityDialog.SetStyles(
			S.Title,
			S.Subtitle,
			S.Muted,
			S.Value,
			S.Error,
			S.Success,
			S.Warning,
			DefaultTheme.BorderFocus,
			Icons.Lock,
			Icons.Check,
			Icons.Cross,
		)
		e.securityDialog.SetSpinner(e.spinner.View)
	}

	// Template Dialog
	if e.templateDialog != nil {
		e.templateDialog.SetStyles(
			S.Title,
			S.Label,
			S.Muted,
			S.Value,
			S.Error,
			S.Success,
			S.Highlighted,
			S.Button,
			S.ButtonFocus,
			DefaultTheme.BorderFocus,
			Icons.Chart,
			Icons.Arrow,
			Icons.Check,
			Icons.Cross,
		)
		e.templateDialog.SetSpinner(e.spinner.View)
	}

	// Progress Dialog
	if e.progressDialog != nil {
		e.progressDialog.SetStyles(
			S.Title,
			S.Label,
			S.Muted,
			S.Value,
			S.Error,
			S.Success,
			S.Warning,
			S.Button,
			S.ButtonFocus,
			DefaultTheme.BorderFocus,
			Icons.Check,
			Icons.Cross,
			Icons.Pending,
			Icons.Arrow,
		)
	}

	// Install Dialog
	if e.installDialog != nil {
		e.installDialog.SetStyles(
			S.Title,
			S.Label,
			S.Muted,
			S.Value,
			S.Error,
			S.Success,
			S.Warning,
			S.Highlighted,
			S.Selected,
			S.Button,
			S.ButtonFocus,
			DefaultTheme.BorderFocus,
			Icons.Add,
			Icons.Arrow,
			Icons.Check,
			Icons.Cross,
			Icons.Lock,
		)
		e.installDialog.SetSpinner(e.spinner.View)
	}

	// Multi-Chart Dialog
	if e.multiChartDialog != nil {
		e.multiChartDialog.SetStyles(
			S.Title,
			S.Label,
			S.Muted,
			S.Value,
			S.Error,
			S.Success,
			S.Warning,
			S.Selected,
			S.Button,
			S.ButtonFocus,
			DefaultTheme.BorderFocus,
			Icons.Chart,
			Icons.Check,
			Icons.Cross,
			Icons.Arrow,
		)
		e.multiChartDialog.SetSpinner(e.spinner.View)
	}

	// Save Template Dialog
	if e.saveTemplateDialog != nil {
		e.saveTemplateDialog.SetStyles(
			S.Title,
			S.Label,
			S.Muted,
			S.Highlighted,
			S.Error,
			S.Button,
			S.ButtonFocus,
			DefaultTheme.BorderFocus,
			Icons.Add,
			Icons.Arrow,
			Icons.Cross,
		)
	}
	if e.saveStackDialog != nil {
		e.saveStackDialog.SetStyles(
			S.Title,
			S.Label,
			S.Muted,
			S.Highlighted,
			S.Error,
			S.Button,
			S.ButtonFocus,
			DefaultTheme.BorderFocus,
			Icons.Add,
			Icons.Arrow,
			Icons.Cross,
		)
	}
	if e.loadStackDialog != nil {
		e.loadStackDialog.SetStyles(
			S.Title,
			S.Label,
			S.Muted,
			S.Selected,
			S.Error,
			DefaultTheme.BorderFocus,
			Icons.Chart,
			Icons.Arrow,
		)
	}

	// Set Value Dialog
	if e.setValueDialog != nil {
		e.setValueDialog.SetStyles(
			S.Title,
			S.Label,
			S.Muted,
			S.Error,
			S.Button,
			S.ButtonFocus,
			DefaultTheme.BorderFocus,
		)
	}

	// Value Sources Dialog
	if e.valueSourcesDialog != nil {
		e.valueSourcesDialog.SetStyles(
			S.Title,
			S.Label,
			S.Muted,
			S.Error,
			S.Highlighted,
			S.Button,
			S.ButtonFocus,
			DefaultTheme.BorderFocus,
			Icons.Check,
			Icons.Cross,
		)
	}
}

// initDialogs creates all dialog instances.
// Call this during ExploreView initialization.
func (e *ExploreView) initDialogs() {
	e.fileImportDialog = dialog.NewFileImportDialog()
	e.versionDialog = dialog.NewVersionDialog()
	e.schemaDialog = dialog.NewSchemaDialog()
	e.readmeDialog = dialog.NewReadmeDialog()
	e.securityDialog = dialog.NewSecurityDialog()
	e.templateDialog = dialog.NewTemplateDialog()
	e.progressDialog = dialog.NewProgressDialog()
	e.installDialog = dialog.NewInstallDialog()
	e.multiChartDialog = dialog.NewMultiChartDialog()
	e.saveTemplateDialog = dialog.NewSaveTemplateDialog()
	e.saveStackDialog = dialog.NewSaveStackDialog()
	e.loadStackDialog = dialog.NewLoadStackDialog()
	e.setValueDialog = dialog.NewSetValueDialog()
	e.valueSourcesDialog = dialog.NewValueSourcesDialog()
}

// updateDialogSizes updates dialog dimensions after window resize.
func (e *ExploreView) updateDialogSizes(width, height int) {
	if e.fileImportDialog != nil {
		e.fileImportDialog.SetSize(width, height)
	}
	if e.versionDialog != nil {
		e.versionDialog.SetSize(width, height)
	}
	if e.schemaDialog != nil {
		e.schemaDialog.SetSize(width, height)
	}
	if e.readmeDialog != nil {
		e.readmeDialog.SetSize(width, height)
	}
	if e.securityDialog != nil {
		e.securityDialog.SetSize(width, height)
	}
	if e.templateDialog != nil {
		e.templateDialog.SetSize(width, height)
	}
	if e.progressDialog != nil {
		e.progressDialog.SetSize(width, height)
	}
	if e.installDialog != nil {
		e.installDialog.SetSize(width, height)
	}
	if e.multiChartDialog != nil {
		e.multiChartDialog.SetSize(width, height)
	}
	if e.saveTemplateDialog != nil {
		e.saveTemplateDialog.SetSize(width, height)
	}
	if e.saveStackDialog != nil {
		e.saveStackDialog.SetSize(width, height)
	}
	if e.loadStackDialog != nil {
		e.loadStackDialog.SetSize(width, height)
	}
	if e.setValueDialog != nil {
		e.setValueDialog.SetSize(width, height)
	}
	if e.valueSourcesDialog != nil {
		e.valueSourcesDialog.SetSize(width, height)
	}
}

// hasAnyDialogOpen returns true if any extracted dialog is open.
func (e *ExploreView) hasAnyDialogOpen() bool {
	return (e.fileImportDialog != nil && e.fileImportDialog.IsOpen()) ||
		(e.versionDialog != nil && e.versionDialog.IsOpen()) ||
		(e.schemaDialog != nil && e.schemaDialog.IsOpen()) ||
		(e.readmeDialog != nil && e.readmeDialog.IsOpen()) ||
		(e.securityDialog != nil && e.securityDialog.IsOpen()) ||
		(e.templateDialog != nil && e.templateDialog.IsOpen()) ||
		(e.progressDialog != nil && e.progressDialog.IsOpen()) ||
		(e.installDialog != nil && e.installDialog.IsOpen()) ||
		(e.multiChartDialog != nil && e.multiChartDialog.IsOpen()) ||
		(e.saveTemplateDialog != nil && e.saveTemplateDialog.IsOpen()) ||
		(e.saveStackDialog != nil && e.saveStackDialog.IsOpen()) ||
		(e.loadStackDialog != nil && e.loadStackDialog.IsOpen()) ||
		(e.setValueDialog != nil && e.setValueDialog.IsOpen()) ||
		(e.valueSourcesDialog != nil && e.valueSourcesDialog.IsOpen())
}

// ConvertToInstallTemplates converts config templates to dialog templates.
func ConvertToInstallTemplates(templates []struct {
	Name        string
	Description string
	Values      string
}) []dialog.InstallTemplate {
	result := make([]dialog.InstallTemplate, len(templates))
	for i, t := range templates {
		result[i] = dialog.InstallTemplate{
			Name:        t.Name,
			Description: t.Description,
			Values:      t.Values,
		}
	}
	return result
}

// ConvertToLocalVersions converts helm versions to dialog local versions.
func ConvertToLocalVersions(versions []struct {
	Version    string
	AppVersion string
}) []dialog.LocalVersion {
	result := make([]dialog.LocalVersion, len(versions))
	for i, v := range versions {
		result[i] = dialog.LocalVersion{
			Version:    v.Version,
			AppVersion: v.AppVersion,
		}
	}
	return result
}

// ConvertToHubVersions converts Artifact Hub versions to dialog hub versions.
func ConvertToHubVersions(versions []struct {
	Version    string
	PreRelease bool
}) []dialog.HubVersion {
	result := make([]dialog.HubVersion, len(versions))
	for i, v := range versions {
		result[i] = dialog.HubVersion{
			Version:    v.Version,
			PreRelease: v.PreRelease,
		}
	}
	return result
}

// ConvertToSecurityResults converts trivy results to dialog security results.
func ConvertToSecurityResults(results interface {
	GetResults() []struct {
		Target            string
		Misconfigurations []struct {
			ID          string
			Title       string
			Description string
			Message     string
			Severity    string
		}
	}
	GetCritical() int
	GetHigh() int
	GetMedium() int
	GetLow() int
	GetTotal() int
}) *dialog.SecurityScanResults {
	rawResults := results.GetResults()
	dialogResults := make([]dialog.ScanResult, len(rawResults))

	for i, r := range rawResults {
		misconfigs := make([]dialog.Misconfiguration, len(r.Misconfigurations))
		for j, m := range r.Misconfigurations {
			misconfigs[j] = dialog.Misconfiguration{
				ID:          m.ID,
				Title:       m.Title,
				Description: m.Description,
				Message:     m.Message,
				Severity:    m.Severity,
			}
		}
		dialogResults[i] = dialog.ScanResult{
			Target:            r.Target,
			Misconfigurations: misconfigs,
		}
	}

	return &dialog.SecurityScanResults{
		Results:  dialogResults,
		Critical: results.GetCritical(),
		High:     results.GetHigh(),
		Medium:   results.GetMedium(),
		Low:      results.GetLow(),
		Total:    results.GetTotal(),
	}
}

// ConvertToPodStatuses converts helm pod status to dialog pod status.
func ConvertToPodStatuses(pods []struct {
	Name            string
	Status          string
	Ready           string
	Restarts        int
	ContainerStatus []struct {
		Name   string
		State  string
		Reason string
	}
}) []dialog.PodStatus {
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
			Restarts:        p.Restarts,
			ContainerStatus: containers,
		}
	}
	return result
}

// ConvertToResourceStatuses converts helm resource status to dialog resource status.
func ConvertToResourceStatuses(resources []struct {
	Kind   string
	Name   string
	Ready  bool
	Status string
}) []dialog.ResourceStatus {
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

// ConvertToEventInfos converts kubernetes events to dialog event info.
func ConvertToEventInfos(events []struct {
	Type      string
	Reason    string
	Object    string
	Message   string
	Timestamp interface{ Unix() int64 }
}) []dialog.EventInfo {
	result := make([]dialog.EventInfo, len(events))
	for i, e := range events {
		result[i] = dialog.EventInfo{
			Type:    e.Type,
			Reason:  e.Reason,
			Object:  e.Object,
			Message: e.Message,
			// Note: Timestamp conversion handled by caller
		}
	}
	return result
}

// GetHighlightedStyle returns the highlighted style for use in dialogs.
func GetHighlightedStyle() lipgloss.Style {
	return S.Highlighted
}

// GetSelectedStyle returns the selected style for use in dialogs.
func GetSelectedStyle() lipgloss.Style {
	return S.Selected
}

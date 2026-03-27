package dialog

import (
	"fmt"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestBaseDialog(t *testing.T) {
	t.Run("starts closed", func(t *testing.T) {
		d := &BaseDialog{}
		if d.IsOpen() {
			t.Error("BaseDialog should start closed")
		}
	})

	t.Run("Open sets open to true", func(t *testing.T) {
		d := &BaseDialog{}
		d.Open()
		if !d.IsOpen() {
			t.Error("BaseDialog should be open after Open()")
		}
	})

	t.Run("Close sets open to false", func(t *testing.T) {
		d := &BaseDialog{}
		d.Open()
		d.Close()
		if d.IsOpen() {
			t.Error("BaseDialog should be closed after Close()")
		}
	})

	t.Run("SetSize updates dimensions", func(t *testing.T) {
		d := &BaseDialog{}
		d.SetSize(100, 50)
		if d.Width != 100 || d.Height != 50 {
			t.Errorf("SetSize failed, got Width=%d Height=%d, want Width=100 Height=50", d.Width, d.Height)
		}
	})
}

func TestFileImportDialog(t *testing.T) {
	t.Run("Open resets state", func(t *testing.T) {
		d := NewFileImportDialog()
		d.Open()

		if !d.IsOpen() {
			t.Error("FileImportDialog should be open after Open()")
		}
	})

	t.Run("Close resets state", func(t *testing.T) {
		d := NewFileImportDialog()
		d.Open()
		d.Close()

		if d.IsOpen() {
			t.Error("FileImportDialog should be closed after Close()")
		}
	})

	t.Run("View returns content when open", func(t *testing.T) {
		d := NewFileImportDialog()
		d.SetSize(80, 40)
		d.Open()

		view := d.View()
		if view == "" {
			t.Error("FileImportDialog View() should return content when open")
		}
	})

	t.Run("escape key closes dialog", func(t *testing.T) {
		d := NewFileImportDialog()
		d.Open()

		msg := tea.KeyMsg{Type: tea.KeyEscape}
		_, _ = d.Update(msg)

		if d.IsOpen() {
			t.Error("FileImportDialog should close on escape key")
		}
	})
}

func TestVersionDialog(t *testing.T) {
	t.Run("OpenForChart sets state", func(t *testing.T) {
		d := NewVersionDialog()
		d.OpenForChart("nginx", "bitnami/nginx", false)

		if !d.IsOpen() {
			t.Error("VersionDialog should be open after OpenForChart()")
		}
	})

	t.Run("SetLocalVersions sets versions", func(t *testing.T) {
		d := NewVersionDialog()
		d.Open()

		versions := []LocalVersion{
			{Version: "1.0.0", AppVersion: "1.0"},
			{Version: "2.0.0", AppVersion: "2.0"},
		}
		d.SetLocalVersions(versions)
		// Versions should be set
	})

	t.Run("SetLoading changes loading state", func(t *testing.T) {
		d := NewVersionDialog()
		d.Open()
		d.SetLoading(true)

		// Cannot check private field, but loading state should affect View()
	})

	t.Run("escape key closes dialog", func(t *testing.T) {
		d := NewVersionDialog()
		d.Open()

		msg := tea.KeyMsg{Type: tea.KeyEscape}
		_, _ = d.Update(msg)

		if d.IsOpen() {
			t.Error("VersionDialog should close on escape key")
		}
	})
}

func TestSchemaDialog(t *testing.T) {
	t.Run("OpenWithSchema parses JSON", func(t *testing.T) {
		d := NewSchemaDialog()
		schemaJSON := []byte(`{"title": "Test Schema", "properties": {"name": {"type": "string"}}}`)
		d.OpenWithSchema(schemaJSON, "test-chart")

		if !d.IsOpen() {
			t.Error("SchemaDialog should be open after OpenWithSchema()")
		}
	})

	t.Run("handles empty schema", func(t *testing.T) {
		d := NewSchemaDialog()
		d.OpenWithSchema(nil, "test-chart")

		if !d.IsOpen() {
			t.Error("SchemaDialog should be open even with nil schema")
		}
	})

	t.Run("escape key closes dialog", func(t *testing.T) {
		d := NewSchemaDialog()
		d.Open()

		msg := tea.KeyMsg{Type: tea.KeyEscape}
		_, _ = d.Update(msg)

		if d.IsOpen() {
			t.Error("SchemaDialog should close on escape key")
		}
	})
}

func TestReadmeDialog(t *testing.T) {
	t.Run("OpenWithContent sets content", func(t *testing.T) {
		d := NewReadmeDialog()
		d.OpenWithContent("# Test Readme\n\nThis is a test.", "test-chart")

		if !d.IsOpen() {
			t.Error("ReadmeDialog should be open after OpenWithContent()")
		}
	})

	t.Run("escape key closes dialog", func(t *testing.T) {
		d := NewReadmeDialog()
		d.Open()

		msg := tea.KeyMsg{Type: tea.KeyEscape}
		_, _ = d.Update(msg)

		if d.IsOpen() {
			t.Error("ReadmeDialog should close on escape key")
		}
	})
}

func TestSecurityDialog(t *testing.T) {
	t.Run("OpenForChart sets chart info", func(t *testing.T) {
		d := NewSecurityDialog()
		d.OpenForChart("nginx", "1.0.0")

		if !d.IsOpen() {
			t.Error("SecurityDialog should be open after OpenForChart()")
		}
	})

	t.Run("SetResults updates results", func(t *testing.T) {
		d := NewSecurityDialog()
		d.Open()

		results := &SecurityScanResults{
			Critical: 1,
			High:     2,
			Medium:   3,
			Low:      4,
			Total:    10,
		}
		d.SetResults(results)
		// Results should be set and viewport updated
	})

	t.Run("SetError sets error state", func(t *testing.T) {
		d := NewSecurityDialog()
		d.Open()

		testErr := fmt.Errorf("test error")
		d.SetError(testErr)
		// Error should be set
	})

	t.Run("escape key closes dialog", func(t *testing.T) {
		d := NewSecurityDialog()
		d.Open()

		msg := tea.KeyMsg{Type: tea.KeyEscape}
		_, _ = d.Update(msg)

		if d.IsOpen() {
			t.Error("SecurityDialog should close on escape key")
		}
	})
}

func TestTemplateDialog(t *testing.T) {
	t.Run("OpenWithChart sets chart info", func(t *testing.T) {
		d := NewTemplateDialog()
		d.OpenWithChart("nginx", "1.0.0", "# default values")

		if !d.IsOpen() {
			t.Error("TemplateDialog should be open after OpenWithChart()")
		}
	})

	t.Run("SetValues updates values", func(t *testing.T) {
		d := NewTemplateDialog()
		d.Open()

		testValues := "key: value"
		d.SetValues(testValues)

		if d.GetValues() != testValues {
			t.Errorf("GetValues() = %q, want %q", d.GetValues(), testValues)
		}
	})

	t.Run("SetTemplating updates templating state", func(t *testing.T) {
		d := NewTemplateDialog()
		d.Open()
		d.SetTemplating(true)
		// Templating state should be set
	})

	t.Run("escape key closes dialog", func(t *testing.T) {
		d := NewTemplateDialog()
		d.Open()

		msg := tea.KeyMsg{Type: tea.KeyEscape}
		_, _ = d.Update(msg)

		if d.IsOpen() {
			t.Error("TemplateDialog should close on escape key")
		}
	})
}

func TestProgressDialog(t *testing.T) {
	t.Run("OpenWithRelease returns command", func(t *testing.T) {
		d := NewProgressDialog()
		cmd := d.OpenWithRelease("test-release", "default")

		if !d.IsOpen() {
			t.Error("ProgressDialog should be open after OpenWithRelease()")
		}
		if cmd == nil {
			t.Error("OpenWithRelease should return a command")
		}
	})

	t.Run("SetData updates pod status", func(t *testing.T) {
		d := NewProgressDialog()
		d.Open()

		pods := []PodStatus{
			{Name: "pod-1", Status: "Running", Ready: "1/1"},
		}
		resources := []ResourceStatus{
			{Kind: "Deployment", Name: "test-deploy", Ready: true, Status: "Ready"},
		}
		events := []EventInfo{
			{Type: "Normal", Reason: "Scheduled", Message: "Pod scheduled"},
		}
		d.SetData(pods, resources, events, true, nil)
		// Data should be set
	})

	t.Run("escape key closes dialog", func(t *testing.T) {
		d := NewProgressDialog()
		d.Open()

		msg := tea.KeyMsg{Type: tea.KeyEscape}
		_, _ = d.Update(msg)

		if d.IsOpen() {
			t.Error("ProgressDialog should close on escape key")
		}
	})
}

func TestInstallDialog(t *testing.T) {
	t.Run("OpenForChart sets chart info", func(t *testing.T) {
		d := NewInstallDialog()
		d.OpenForChart("nginx", "1.0.0", "bitnami/nginx", "# values", nil, false, "")

		if !d.IsOpen() {
			t.Error("InstallDialog should be open after OpenForChart()")
		}
	})

	t.Run("SetInstalling updates installing state", func(t *testing.T) {
		d := NewInstallDialog()
		d.Open()
		d.SetInstalling(true)
		// Installing state should be set
	})

	t.Run("escape key closes dialog when not installing", func(t *testing.T) {
		d := NewInstallDialog()
		d.Open()
		d.SetInstalling(false)

		msg := tea.KeyMsg{Type: tea.KeyEscape}
		_, _ = d.Update(msg)

		if d.IsOpen() {
			t.Error("InstallDialog should close on escape key when not installing")
		}
	})
}

func TestMultiChartDialog(t *testing.T) {
	t.Run("starts with empty queue", func(t *testing.T) {
		d := NewMultiChartDialog()
		d.Open()

		queue := d.GetQueue()
		if len(queue) != 0 {
			t.Errorf("MultiChartDialog should start with empty queue, got %d items", len(queue))
		}
	})

	t.Run("AddToQueue adds item", func(t *testing.T) {
		d := NewMultiChartDialog()
		d.Open()

		item := MultiChartQueueItem{
			ChartRef:    "bitnami/postgresql",
			ChartName:   "postgresql",
			ReleaseName: "my-postgres",
			Namespace:   "default",
		}
		d.AddToQueue(item)

		queue := d.GetQueue()
		if len(queue) != 1 {
			t.Errorf("Expected 1 item in queue, got %d", len(queue))
		}
		if queue[0].ChartRef != "bitnami/postgresql" {
			t.Errorf("Expected chart ref 'bitnami/postgresql', got '%s'", queue[0].ChartRef)
		}
	})

	t.Run("AddToQueue prevents duplicates", func(t *testing.T) {
		d := NewMultiChartDialog()
		d.Open()

		item := MultiChartQueueItem{
			ChartRef:    "bitnami/postgresql",
			ChartName:   "postgresql",
			ReleaseName: "my-postgres",
		}
		d.AddToQueue(item)
		d.AddToQueue(item) // Same chart ref

		queue := d.GetQueue()
		if len(queue) != 1 {
			t.Errorf("Duplicate items should not be added, expected 1 item, got %d", len(queue))
		}
	})

	t.Run("ClearQueue removes all items", func(t *testing.T) {
		d := NewMultiChartDialog()
		d.Open()

		d.AddToQueue(MultiChartQueueItem{ChartRef: "bitnami/postgresql"})
		d.AddToQueue(MultiChartQueueItem{ChartRef: "bitnami/redis"})

		queue := d.GetQueue()
		if len(queue) != 2 {
			t.Errorf("Expected 2 items, got %d", len(queue))
		}

		d.ClearQueue()
		queue = d.GetQueue()
		if len(queue) != 0 {
			t.Errorf("ClearQueue should empty the queue, got %d items", len(queue))
		}
	})

	t.Run("UpdateItemStatus updates status", func(t *testing.T) {
		d := NewMultiChartDialog()
		d.Open()

		d.AddToQueue(MultiChartQueueItem{ChartRef: "bitnami/postgresql", Status: MultiChartPending})

		d.UpdateItemStatus(0, MultiChartInstalling, nil)
		queue := d.GetQueue()
		if queue[0].Status != MultiChartInstalling {
			t.Errorf("Expected status MultiChartInstalling, got %d", queue[0].Status)
		}

		testErr := fmt.Errorf("install failed")
		d.UpdateItemStatus(0, MultiChartFailed, testErr)
		queue = d.GetQueue()
		if queue[0].Status != MultiChartFailed {
			t.Errorf("Expected status MultiChartFailed, got %d", queue[0].Status)
		}
		if queue[0].Error == nil || queue[0].Error.Error() != "install failed" {
			t.Error("Error should be set on failed status")
		}
	})

	t.Run("SetInstalling updates installing state", func(t *testing.T) {
		d := NewMultiChartDialog()
		d.Open()

		d.SetInstalling(true)
		if !d.IsInstalling() {
			t.Error("IsInstalling should return true after SetInstalling(true)")
		}

		d.SetInstalling(false)
		if d.IsInstalling() {
			t.Error("IsInstalling should return false after SetInstalling(false)")
		}
	})

	t.Run("escape key closes dialog when not installing", func(t *testing.T) {
		d := NewMultiChartDialog()
		d.Open()
		d.SetInstalling(false)

		msg := tea.KeyMsg{Type: tea.KeyEscape}
		_, _ = d.Update(msg)

		if d.IsOpen() {
			t.Error("MultiChartDialog should close on escape key when not installing")
		}
	})

	t.Run("escape key cancels installation", func(t *testing.T) {
		d := NewMultiChartDialog()
		d.Open()

		d.AddToQueue(MultiChartQueueItem{ChartRef: "test", Status: MultiChartInstalling})
		d.SetInstalling(true)

		msg := tea.KeyMsg{Type: tea.KeyEscape}
		_, _ = d.Update(msg)

		if d.IsInstalling() {
			t.Error("Escape should cancel installation")
		}

		queue := d.GetQueue()
		if queue[0].Status != MultiChartPending {
			t.Error("Installing item should be reset to pending on cancel")
		}
	})
}

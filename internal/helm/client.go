package helm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/release"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

// DefaultHelmTimeout is the default timeout for Helm operations
const DefaultHelmTimeout = 5 * time.Minute

// Release is our simplified view of a Helm release
type Release struct {
	Name      string
	Namespace string
	Chart     string
	Version   string
	Status    string
	Revision  int
}

// Client wraps the Helm SDK
type Client struct {
	settings        *cli.EnvSettings
	kubeconfig      string
	secretsClient   *SecretsClient
	trivyClient     *TrivyClient
	cachedConfig    *rest.Config
	cachedClientset *kubernetes.Clientset
	clientsetOnce   sync.Once
	clientsetErr    error
}

// NewClient creates a new Helm client
// kubeconfig can be empty to use default (~/.kube/config or KUBECONFIG env)
func NewClient(kubeconfig string) (*Client, error) {
	if kubeconfig == "" {
		// Check KUBECONFIG env first, then default location
		kubeconfig = os.Getenv("KUBECONFIG")
		if kubeconfig == "" {
			kubeconfig = filepath.Join(homedir.HomeDir(), ".kube", "config")
		}
	}

	settings := cli.New()
	settings.KubeConfig = kubeconfig

	return &Client{
		settings:      settings,
		kubeconfig:    kubeconfig,
		secretsClient: NewSecretsClient(),
		trivyClient:   NewTrivyClient(),
	}, nil
}

// GetSecretsClient returns the secrets client for encryption operations
func (c *Client) GetSecretsClient() *SecretsClient {
	return c.secretsClient
}

// HasSecretsSupport returns true if helm-secrets plugin is available
func (c *Client) HasSecretsSupport() bool {
	return c.secretsClient != nil && c.secretsClient.IsAvailable()
}

// GetTrivyClient returns the Trivy client for security scanning
func (c *Client) GetTrivyClient() *TrivyClient {
	return c.trivyClient
}

// HasTrivySupport returns true if Trivy is available
func (c *Client) HasTrivySupport() bool {
	return c.trivyClient != nil && c.trivyClient.IsAvailable()
}

// GetCurrentContext returns the current kubectl context name
func (c *Client) GetCurrentContext() string {
	config, err := clientcmd.LoadFromFile(c.kubeconfig)
	if err != nil {
		return "unknown"
	}
	return config.CurrentContext
}

// getClientset returns a cached Kubernetes clientset and rest config.
// The clientset is created once and reused for all subsequent calls.
// It is invalidated when the context is switched via SwitchContext().
func (c *Client) getClientset() (*kubernetes.Clientset, *rest.Config, error) {
	c.clientsetOnce.Do(func() {
		config, err := clientcmd.BuildConfigFromFlags("", c.kubeconfig)
		if err != nil {
			c.clientsetErr = fmt.Errorf("failed to build config: %w", err)
			return
		}
		clientset, err := kubernetes.NewForConfig(config)
		if err != nil {
			c.clientsetErr = fmt.Errorf("failed to create clientset: %w", err)
			return
		}
		c.cachedConfig = config
		c.cachedClientset = clientset
	})
	return c.cachedClientset, c.cachedConfig, c.clientsetErr
}

// getActionConfig creates a Helm action configuration for the given namespace
func (c *Client) getActionConfig(namespace string) (*action.Configuration, error) {
	actionConfig := new(action.Configuration)

	// Initialize with the namespace
	// The debug function receives log messages from Helm
	if err := actionConfig.Init(
		c.settings.RESTClientGetter(),
		namespace,
		os.Getenv("HELM_DRIVER"), // Usually "secrets" or "configmaps"
		func(format string, v ...interface{}) {
			// Debug logging - could be exposed to TUI later
		},
	); err != nil {
		return nil, err
	}

	return actionConfig, nil
}

// ListReleases returns all releases in the given namespace (or all namespaces if empty)
func (c *Client) ListReleases(namespace string) ([]Release, error) {
	actionConfig, err := c.getActionConfig(namespace)
	if err != nil {
		return nil, err
	}

	listAction := action.NewList(actionConfig)
	listAction.AllNamespaces = namespace == ""
	listAction.All = true // Include all statuses

	results, err := listAction.Run()
	if err != nil {
		return nil, err
	}

	releases := make([]Release, len(results))
	for i, r := range results {
		releases[i] = Release{
			Name:      r.Name,
			Namespace: r.Namespace,
			Chart:     r.Chart.Metadata.Name + "-" + r.Chart.Metadata.Version,
			Version:   r.Chart.Metadata.Version,
			Status:    r.Info.Status.String(),
			Revision:  r.Version,
		}
	}

	return releases, nil
}

// GetRelease gets details of a specific release
func (c *Client) GetRelease(name, namespace string) (*release.Release, error) {
	actionConfig, err := c.getActionConfig(namespace)
	if err != nil {
		return nil, err
	}

	getAction := action.NewGet(actionConfig)
	return getAction.Run(name)
}

// Uninstall removes a release
func (c *Client) Uninstall(name, namespace string) error {
	actionConfig, err := c.getActionConfig(namespace)
	if err != nil {
		return err
	}

	uninstallAction := action.NewUninstall(actionConfig)
	_, err = uninstallAction.Run(name)
	return err
}

// Rollback rolls back a release to a previous revision
// If revision is 0, it rolls back to the previous revision
func (c *Client) Rollback(name, namespace string, revision int) error {
	actionConfig, err := c.getActionConfig(namespace)
	if err != nil {
		return err
	}

	rollbackAction := action.NewRollback(actionConfig)
	rollbackAction.Version = revision
	return rollbackAction.Run(name)
}

// GetValues returns the computed values for a release (chart defaults + user values merged)
func (c *Client) GetValues(name, namespace string) (map[string]interface{}, error) {
	actionConfig, err := c.getActionConfig(namespace)
	if err != nil {
		return nil, err
	}

	getValuesAction := action.NewGetValues(actionConfig)
	getValuesAction.AllValues = true
	return getValuesAction.Run(name)
}

// GetUserValues returns only the user-supplied values for a release (not chart defaults)
// Use this for upgrades so chart defaults can be updated automatically
func (c *Client) GetUserValues(name, namespace string) (map[string]interface{}, error) {
	actionConfig, err := c.getActionConfig(namespace)
	if err != nil {
		return nil, err
	}

	getValuesAction := action.NewGetValues(actionConfig)
	getValuesAction.AllValues = false // Only user-supplied values
	return getValuesAction.Run(name)
}

// GetHistory returns the release history
func (c *Client) GetHistory(name, namespace string) ([]*release.Release, error) {
	actionConfig, err := c.getActionConfig(namespace)
	if err != nil {
		return nil, err
	}

	historyAction := action.NewHistory(actionConfig)
	return historyAction.Run(name)
}

// GetRevision returns a specific revision of a release
func (c *Client) GetRevision(name, namespace string, revision int) (*release.Release, error) {
	history, err := c.GetHistory(name, namespace)
	if err != nil {
		return nil, err
	}

	for _, rel := range history {
		if rel.Version == revision {
			return rel, nil
		}
	}

	return nil, fmt.Errorf("revision %d not found for release %s", revision, name)
}

// Template renders a chart without installing it (for preview)
// Note: Use LoadChart from chart.go and PreviewInstall for full functionality
func (c *Client) Template(chartPath, releaseName, namespace string, values map[string]interface{}) (string, error) {
	return c.templateInternal(chartPath, releaseName, namespace, values)
}

func (c *Client) templateInternal(chartRef, releaseName, namespace string, values map[string]interface{}) (string, error) {
	ch, err := c.LoadChart(chartRef)
	if err != nil {
		return "", err
	}

	actionConfig, err := c.getActionConfig(namespace)
	if err != nil {
		return "", err
	}

	installAction := action.NewInstall(actionConfig)
	installAction.DryRun = true
	installAction.ReleaseName = releaseName
	installAction.Namespace = namespace
	installAction.ClientOnly = true

	rel, err := installAction.Run(ch, values)
	if err != nil {
		return "", err
	}

	return rel.Manifest, nil
}

// Install installs a chart to the cluster
func (c *Client) Install(chartRef, releaseName, namespace string, values map[string]interface{}, createNamespace bool) (*Release, error) {
	ch, err := c.LoadChart(chartRef)
	if err != nil {
		return nil, err
	}

	actionConfig, err := c.getActionConfig(namespace)
	if err != nil {
		return nil, err
	}

	installAction := action.NewInstall(actionConfig)
	installAction.ReleaseName = releaseName
	installAction.Namespace = namespace
	installAction.CreateNamespace = createNamespace
	installAction.Wait = false // Don't block waiting for resources
	installAction.Timeout = DefaultHelmTimeout

	rel, err := installAction.Run(ch, values)
	if err != nil {
		return nil, err
	}

	return &Release{
		Name:      rel.Name,
		Namespace: rel.Namespace,
		Chart:     rel.Chart.Metadata.Name + "-" + rel.Chart.Metadata.Version,
		Version:   rel.Chart.Metadata.Version,
		Status:    rel.Info.Status.String(),
		Revision:  rel.Version,
	}, nil
}

// ListNamespaces returns all unique namespaces that have Helm releases
func (c *Client) ListNamespaces() ([]string, error) {
	releases, err := c.ListReleases("")
	if err != nil {
		return nil, err
	}

	nsMap := make(map[string]bool)
	for _, r := range releases {
		nsMap[r.Namespace] = true
	}

	namespaces := make([]string, 0, len(nsMap))
	for ns := range nsMap {
		namespaces = append(namespaces, ns)
	}

	return namespaces, nil
}

// Upgrade upgrades an existing release with a new chart reference
func (c *Client) Upgrade(chartRef, releaseName, namespace string, values map[string]interface{}) (*Release, error) {
	ch, err := c.LoadChart(chartRef)
	if err != nil {
		return nil, err
	}

	actionConfig, err := c.getActionConfig(namespace)
	if err != nil {
		return nil, err
	}

	upgradeAction := action.NewUpgrade(actionConfig)
	upgradeAction.Namespace = namespace
	upgradeAction.Wait = false
	upgradeAction.Timeout = DefaultHelmTimeout

	rel, err := upgradeAction.Run(releaseName, ch, values)
	if err != nil {
		return nil, err
	}

	return &Release{
		Name:      rel.Name,
		Namespace: rel.Namespace,
		Chart:     rel.Chart.Metadata.Name + "-" + rel.Chart.Metadata.Version,
		Version:   rel.Chart.Metadata.Version,
		Status:    rel.Info.Status.String(),
		Revision:  rel.Version,
	}, nil
}

// UpgradeValues upgrades an existing release by reusing the deployed chart
// This is useful when you only want to change values without upgrading the chart version
func (c *Client) UpgradeValues(releaseName, namespace string, values map[string]interface{}) (*Release, error) {
	actionConfig, err := c.getActionConfig(namespace)
	if err != nil {
		return nil, err
	}

	// Get the existing release to find the chart name
	getAction := action.NewGet(actionConfig)
	existingRel, err := getAction.Run(releaseName)
	if err != nil {
		return nil, fmt.Errorf("failed to get existing release: %w", err)
	}

	// Try to find the chart in local repos first (to get proper dependencies)
	chartName := existingRel.Chart.Metadata.Name
	var ch *chart.Chart

	if chartRef := c.FindChartInRepos(chartName); chartRef != "" {
		// Found in repos - load fresh with dependencies
		ch, err = c.LoadChart(chartRef)
		if err != nil {
			// Fall back to release chart if load fails
			ch = existingRel.Chart
		}
	} else {
		// Not found in repos - use the chart from the release
		ch = existingRel.Chart
	}

	upgradeAction := action.NewUpgrade(actionConfig)
	upgradeAction.Namespace = namespace
	upgradeAction.Wait = false
	upgradeAction.Timeout = DefaultHelmTimeout

	rel, err := upgradeAction.Run(releaseName, ch, values)
	if err != nil {
		return nil, err
	}

	return &Release{
		Name:      rel.Name,
		Namespace: rel.Namespace,
		Chart:     rel.Chart.Metadata.Name + "-" + rel.Chart.Metadata.Version,
		Version:   rel.Chart.Metadata.Version,
		Status:    rel.Info.Status.String(),
		Revision:  rel.Version,
	}, nil
}

// GetManifest returns the manifest for a deployed release
func (c *Client) GetManifest(name, namespace string) (string, error) {
	rel, err := c.GetRelease(name, namespace)
	if err != nil {
		return "", err
	}
	return rel.Manifest, nil
}

// GetDeployedResources returns the list of resources deployed by a release
func (c *Client) GetDeployedResources(name, namespace string) ([]string, error) {
	manifest, err := c.GetManifest(name, namespace)
	if err != nil {
		return nil, err
	}
	return extractResourceKinds(manifest), nil
}

// DryRunUpgrade performs a dry-run upgrade and returns the rendered manifest
// This is useful for previewing what would change before actually upgrading
func (c *Client) DryRunUpgrade(chartRef, releaseName, namespace string, values map[string]interface{}) (string, error) {
	ch, err := c.LoadChart(chartRef)
	if err != nil {
		return "", err
	}

	actionConfig, err := c.getActionConfig(namespace)
	if err != nil {
		return "", err
	}

	upgradeAction := action.NewUpgrade(actionConfig)
	upgradeAction.Namespace = namespace
	upgradeAction.DryRun = true

	rel, err := upgradeAction.Run(releaseName, ch, values)
	if err != nil {
		return "", err
	}

	return rel.Manifest, nil
}

// DryRunUpgradeValues performs a dry-run upgrade reusing the existing chart
// This is useful for previewing value changes without a new chart version
func (c *Client) DryRunUpgradeValues(releaseName, namespace string, values map[string]interface{}) (string, error) {
	actionConfig, err := c.getActionConfig(namespace)
	if err != nil {
		return "", err
	}

	// Get the existing release to find the chart name
	getAction := action.NewGet(actionConfig)
	existingRel, err := getAction.Run(releaseName)
	if err != nil {
		return "", fmt.Errorf("failed to get existing release: %w", err)
	}

	// Try to find the chart in local repos first (to get proper dependencies)
	chartName := existingRel.Chart.Metadata.Name
	var ch *chart.Chart

	if chartRef := c.FindChartInRepos(chartName); chartRef != "" {
		// Found in repos - load fresh with dependencies
		ch, err = c.LoadChart(chartRef)
		if err != nil {
			// Fall back to release chart if load fails
			ch = existingRel.Chart
		}
	} else {
		// Not found in repos - use the chart from the release
		ch = existingRel.Chart
	}

	upgradeAction := action.NewUpgrade(actionConfig)
	upgradeAction.Namespace = namespace
	upgradeAction.DryRun = true

	rel, err := upgradeAction.Run(releaseName, ch, values)
	if err != nil {
		return "", err
	}

	return rel.Manifest, nil
}

// PodStatus represents the status of a pod
type PodStatus struct {
	Name            string
	Namespace       string
	Status          string // Running, Pending, Succeeded, Failed, Unknown
	Ready           string // e.g., "1/1", "0/1"
	Restarts        int32
	Age             string
	ContainerStatus []ContainerStatus
	// Resource utilization (from metrics-server, may be empty if unavailable)
	CPUUsage    string // e.g., "100m", "1.5"
	MemoryUsage string // e.g., "128Mi", "1Gi"
}

// ContainerStatus represents the status of a container in a pod
type ContainerStatus struct {
	Name         string
	Ready        bool
	RestartCount int32
	State        string // Running, Waiting, Terminated
	Reason       string // e.g., CrashLoopBackOff, ImagePullBackOff
}

// PodLogOptions configures log fetching behavior
type PodLogOptions struct {
	Container  string // Specific container name (empty = first container)
	TailLines  int64  // Number of lines from the end (default 100)
	Timestamps bool   // Include timestamps in output
	Previous   bool   // Get logs from previous instance (crashed pod)
}

// PodInfo represents minimal pod info for selection in log viewer
type PodInfo struct {
	Name       string
	Namespace  string
	Status     string
	Containers []string // List of container names
}

// GetPodStatus returns the status of pods in the given namespace with the given label selector
func (c *Client) GetPodStatus(namespace string, labelSelector string) ([]PodStatus, error) {
	clientset, _, err := c.getClientset()
	if err != nil {
		return nil, err
	}

	pods, err := clientset.CoreV1().Pods(namespace).List(context.Background(), metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return nil, err
	}

	var statuses []PodStatus
	for _, pod := range pods.Items {
		status := PodStatus{
			Name:      pod.Name,
			Namespace: pod.Namespace,
			Status:    string(pod.Status.Phase),
		}

		// Calculate ready containers
		readyCount := 0
		totalContainers := len(pod.Spec.Containers)
		for _, cs := range pod.Status.ContainerStatuses {
			if cs.Ready {
				readyCount++
			}
			status.Restarts += cs.RestartCount

			containerStatus := ContainerStatus{
				Name:         cs.Name,
				Ready:        cs.Ready,
				RestartCount: cs.RestartCount,
			}

			if cs.State.Running != nil {
				containerStatus.State = "Running"
			} else if cs.State.Waiting != nil {
				containerStatus.State = "Waiting"
				containerStatus.Reason = cs.State.Waiting.Reason
			} else if cs.State.Terminated != nil {
				containerStatus.State = "Terminated"
				containerStatus.Reason = cs.State.Terminated.Reason
			}

			status.ContainerStatus = append(status.ContainerStatus, containerStatus)
		}

		status.Ready = formatReady(readyCount, totalContainers)

		// Determine the actual status (may differ from phase)
		status.Status = determinePodStatus(&pod)

		statuses = append(statuses, status)
	}

	return statuses, nil
}

// GetReleasePodStatus returns the status of pods for a specific release
func (c *Client) GetReleasePodStatus(releaseName, namespace string) ([]PodStatus, error) {
	// Helm uses the label "app.kubernetes.io/instance" to identify releases
	labelSelector := "app.kubernetes.io/instance=" + releaseName
	return c.GetPodStatus(namespace, labelSelector)
}

// PodMetrics holds CPU and memory usage for a pod
type PodMetrics struct {
	Name        string
	Namespace   string
	CPUUsage    string // Formatted, e.g., "100m", "1.5"
	MemoryUsage string // Formatted, e.g., "128Mi", "1Gi"
}

// GetPodMetrics fetches resource usage metrics for pods in a namespace.
// Returns a map of pod name to metrics. If metrics-server is not available,
// returns an empty map without error.
func (c *Client) GetPodMetrics(namespace string, labelSelector string) (map[string]PodMetrics, error) {
	clientset, _, err := c.getClientset()
	if err != nil {
		return nil, err
	}

	// Use the REST client to call the metrics API
	// Fetch all pods in namespace (filtering by label in URL can cause issues)
	path := fmt.Sprintf("/apis/metrics.k8s.io/v1beta1/namespaces/%s/pods", namespace)

	result := clientset.CoreV1().RESTClient().Get().
		AbsPath(path).
		Do(context.Background())

	if result.Error() != nil {
		// Metrics server might not be installed - return empty map
		return make(map[string]PodMetrics), nil
	}

	raw, err := result.Raw()
	if err != nil {
		return make(map[string]PodMetrics), nil
	}

	// Parse the metrics response
	var metricsResponse struct {
		Items []struct {
			Metadata struct {
				Name      string `json:"name"`
				Namespace string `json:"namespace"`
			} `json:"metadata"`
			Containers []struct {
				Name  string `json:"name"`
				Usage struct {
					CPU    string `json:"cpu"`
					Memory string `json:"memory"`
				} `json:"usage"`
			} `json:"containers"`
		} `json:"items"`
	}

	if err := json.Unmarshal(raw, &metricsResponse); err != nil {
		return make(map[string]PodMetrics), nil
	}

	// Aggregate container metrics per pod
	metrics := make(map[string]PodMetrics)
	for _, item := range metricsResponse.Items {
		var totalCPUNano int64
		var totalMemBytes int64

		for _, container := range item.Containers {
			// Parse CPU (e.g., "100m" or "1" or "123456789n")
			cpuNano := parseCPU(container.Usage.CPU)
			totalCPUNano += cpuNano

			// Parse Memory (e.g., "128Mi" or "134217728")
			memBytes := parseMemory(container.Usage.Memory)
			totalMemBytes += memBytes
		}

		metrics[item.Metadata.Name] = PodMetrics{
			Name:        item.Metadata.Name,
			Namespace:   item.Metadata.Namespace,
			CPUUsage:    formatCPU(totalCPUNano),
			MemoryUsage: formatMemory(totalMemBytes),
		}
	}

	return metrics, nil
}

// parseCPU parses a CPU quantity string and returns nanocores
func parseCPU(s string) int64 {
	if s == "" {
		return 0
	}
	s = strings.TrimSpace(s)

	// Handle nanocores (e.g., "123456789n")
	if strings.HasSuffix(s, "n") {
		val, _ := strconv.ParseInt(strings.TrimSuffix(s, "n"), 10, 64)
		return val
	}
	// Handle millicores (e.g., "100m")
	if strings.HasSuffix(s, "m") {
		val, _ := strconv.ParseInt(strings.TrimSuffix(s, "m"), 10, 64)
		return val * 1000000 // millicores to nanocores
	}
	// Handle cores (e.g., "1" or "1.5")
	val, _ := strconv.ParseFloat(s, 64)
	return int64(val * 1000000000) // cores to nanocores
}

// parseMemory parses a memory quantity string and returns bytes
func parseMemory(s string) int64 {
	if s == "" {
		return 0
	}
	s = strings.TrimSpace(s)

	multipliers := map[string]int64{
		"Ki": 1024,
		"Mi": 1024 * 1024,
		"Gi": 1024 * 1024 * 1024,
		"Ti": 1024 * 1024 * 1024 * 1024,
		"K":  1000,
		"M":  1000 * 1000,
		"G":  1000 * 1000 * 1000,
		"T":  1000 * 1000 * 1000 * 1000,
	}

	for suffix, mult := range multipliers {
		if strings.HasSuffix(s, suffix) {
			val, _ := strconv.ParseInt(strings.TrimSuffix(s, suffix), 10, 64)
			return val * mult
		}
	}

	// Plain bytes
	val, _ := strconv.ParseInt(s, 10, 64)
	return val
}

// formatCPU formats nanocores to human-readable string
func formatCPU(nanocores int64) string {
	if nanocores == 0 {
		return "-"
	}
	// Convert to millicores (float for precision)
	millicores := float64(nanocores) / 1000000.0
	if millicores >= 1000 {
		// Show as cores
		return fmt.Sprintf("%.1f", millicores/1000.0)
	}
	if millicores >= 1 {
		// Show as whole millicores
		return fmt.Sprintf("%dm", int64(millicores))
	}
	// Sub-millicores: show with decimal or as "<1m"
	if millicores >= 0.1 {
		return fmt.Sprintf("%.1fm", millicores)
	}
	return "<1m"
}

// formatMemory formats bytes to human-readable string
func formatMemory(bytes int64) string {
	if bytes == 0 {
		return "-"
	}
	const (
		Ki = 1024
		Mi = Ki * 1024
		Gi = Mi * 1024
	)
	switch {
	case bytes >= Gi:
		return fmt.Sprintf("%.1fGi", float64(bytes)/float64(Gi))
	case bytes >= Mi:
		return fmt.Sprintf("%.0fMi", float64(bytes)/float64(Mi))
	case bytes >= Ki:
		return fmt.Sprintf("%.0fKi", float64(bytes)/float64(Ki))
	default:
		return fmt.Sprintf("%dB", bytes)
	}
}

// GetReleasePodStatusWithMetrics returns pod status including CPU/memory usage
func (c *Client) GetReleasePodStatusWithMetrics(releaseName, namespace string) ([]PodStatus, error) {
	labelSelector := "app.kubernetes.io/instance=" + releaseName

	// Get pod status
	statuses, err := c.GetPodStatus(namespace, labelSelector)
	if err != nil {
		return nil, err
	}

	// Get metrics (best effort - don't fail if unavailable)
	metrics, _ := c.GetPodMetrics(namespace, labelSelector)

	// Merge metrics into status
	for i := range statuses {
		if m, ok := metrics[statuses[i].Name]; ok {
			statuses[i].CPUUsage = m.CPUUsage
			statuses[i].MemoryUsage = m.MemoryUsage
		}
	}

	return statuses, nil
}

// GetPodLogs retrieves logs for a specific pod
func (c *Client) GetPodLogs(podName, namespace string, opts PodLogOptions) (string, error) {
	clientset, _, err := c.getClientset()
	if err != nil {
		return "", err
	}

	// Configure log options
	podLogOpts := &corev1.PodLogOptions{
		Timestamps: opts.Timestamps,
	}

	if opts.TailLines > 0 {
		podLogOpts.TailLines = &opts.TailLines
	}

	if opts.Container != "" {
		podLogOpts.Container = opts.Container
	}

	if opts.Previous {
		podLogOpts.Previous = true
	}

	// Get log stream
	req := clientset.CoreV1().Pods(namespace).GetLogs(podName, podLogOpts)
	stream, err := req.Stream(context.Background())
	if err != nil {
		return "", fmt.Errorf("failed to stream logs: %w", err)
	}
	defer stream.Close()

	// Read all logs into buffer
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, stream); err != nil {
		return "", fmt.Errorf("failed to read logs: %w", err)
	}

	return buf.String(), nil
}

// GetReleasePods returns pods for a release with container names for log viewing
func (c *Client) GetReleasePods(releaseName, namespace string) ([]PodInfo, error) {
	clientset, _, err := c.getClientset()
	if err != nil {
		return nil, err
	}

	labelSelector := "app.kubernetes.io/instance=" + releaseName
	pods, err := clientset.CoreV1().Pods(namespace).List(context.Background(), metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list pods: %w", err)
	}

	var podInfos []PodInfo
	for _, pod := range pods.Items {
		info := PodInfo{
			Name:      pod.Name,
			Namespace: pod.Namespace,
			Status:    determinePodStatus(&pod),
		}

		// Collect container names
		for _, container := range pod.Spec.Containers {
			info.Containers = append(info.Containers, container.Name)
		}

		podInfos = append(podInfos, info)
	}

	return podInfos, nil
}

// formatReady formats the ready count as "x/y"
func formatReady(ready, total int) string {
	return formatInt(ready) + "/" + formatInt(total)
}

func formatInt(n int) string {
	return strconv.Itoa(n)
}

// determinePodStatus determines the actual pod status accounting for container states.
//
// This function goes beyond the simple pod.Status.Phase to provide more
// actionable status information by inspecting container states.
//
// Possible return values (in priority order):
//   - "Terminating": Pod is being deleted
//   - "CrashLoopBackOff": Container is crash-looping
//   - "ImagePullBackOff": Container image cannot be pulled
//   - "ContainerCreating": Container is being created
//   - "Init:<reason>": Init container is waiting or failed
//   - "Init:Error": Init container terminated with non-zero exit
//   - Other waiting reasons from container state
//   - Pod phase as fallback: "Running", "Pending", "Succeeded", "Failed", "Unknown"
func determinePodStatus(pod *corev1.Pod) string {
	// Check for terminating state
	if pod.DeletionTimestamp != nil {
		return "Terminating"
	}

	// Check container statuses for more specific states
	for _, cs := range pod.Status.ContainerStatuses {
		if cs.State.Waiting != nil {
			reason := cs.State.Waiting.Reason
			if reason != "" {
				switch reason {
				case "CrashLoopBackOff":
					return "CrashLoopBackOff"
				case "ImagePullBackOff", "ErrImagePull":
					return "ImagePullBackOff"
				case "CreateContainerConfigError":
					return "ContainerCreating"
				default:
					return reason
				}
			}
		}
		if cs.State.Terminated != nil && cs.State.Terminated.Reason != "" {
			return cs.State.Terminated.Reason
		}
	}

	// Check init container statuses
	for _, cs := range pod.Status.InitContainerStatuses {
		if cs.State.Waiting != nil && cs.State.Waiting.Reason != "" {
			return "Init:" + cs.State.Waiting.Reason
		}
		if cs.State.Terminated != nil && cs.State.Terminated.ExitCode != 0 {
			return "Init:Error"
		}
	}

	return string(pod.Status.Phase)
}

// K8sEvent represents a Kubernetes event for display
type K8sEvent struct {
	Type      string // Normal, Warning
	Reason    string // Scheduled, Pulling, Pulled, Created, Started, etc.
	Message   string // Human-readable message
	Object    string // Pod name or other object
	Timestamp time.Time
	Count     int32 // How many times this event occurred
}

// GetReleaseEvents returns Kubernetes events for pods in a release
func (c *Client) GetReleaseEvents(releaseName, namespace string) ([]K8sEvent, error) {
	clientset, _, err := c.getClientset()
	if err != nil {
		return nil, err
	}

	// First get pods for this release
	labelSelector := "app.kubernetes.io/instance=" + releaseName
	pods, err := clientset.CoreV1().Pods(namespace).List(context.Background(), metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return nil, err
	}

	// Fetch events per pod using FieldSelector for server-side filtering
	var releaseEvents []K8sEvent
	for _, pod := range pods.Items {
		events, err := clientset.CoreV1().Events(namespace).List(context.Background(), metav1.ListOptions{
			FieldSelector: fmt.Sprintf("involvedObject.kind=Pod,involvedObject.name=%s", pod.Name),
		})
		if err != nil {
			continue // Skip pods whose events we can't fetch
		}

		for _, event := range events.Items {
			timestamp := event.LastTimestamp.Time
			if timestamp.IsZero() {
				timestamp = event.EventTime.Time
			}
			if timestamp.IsZero() {
				timestamp = event.CreationTimestamp.Time
			}

			releaseEvents = append(releaseEvents, K8sEvent{
				Type:      event.Type,
				Reason:    event.Reason,
				Message:   event.Message,
				Object:    event.InvolvedObject.Name,
				Timestamp: timestamp,
				Count:     event.Count,
			})
		}
	}

	// Sort by timestamp (newest first)
	sort.Slice(releaseEvents, func(i, j int) bool {
		return releaseEvents[i].Timestamp.After(releaseEvents[j].Timestamp)
	})

	return releaseEvents, nil
}

// AllPodsReady returns true if all pods for a release are in Running state with all containers ready
func (c *Client) AllPodsReady(releaseName, namespace string) (bool, error) {
	pods, err := c.GetReleasePodStatus(releaseName, namespace)
	if err != nil {
		return false, err
	}

	if len(pods) == 0 {
		return true, nil // No pods = ready (could be a Job that completed or ConfigMap-only chart)
	}

	for _, pod := range pods {
		// Handle completed/succeeded status (for Jobs)
		if pod.Status == "Succeeded" || pod.Status == "Completed" {
			continue
		}
		// Must be Running for regular pods
		if pod.Status != "Running" {
			return false, nil
		}
		// Check if all containers are ready (e.g., "2/2")
		parts := strings.Split(pod.Ready, "/")
		if len(parts) == 2 && parts[0] != parts[1] {
			return false, nil
		}
	}

	return true, nil
}

// ResourceStatus represents the status of a Kubernetes resource
type ResourceStatus struct {
	Kind   string // Deployment, Service, ConfigMap, etc.
	Name   string
	Ready  bool   // Exists and is "ready" for its type
	Status string // Brief status text (e.g., "2/3 ready", "Bound")
}

// GetReleaseResourceStatus returns the status of all resources deployed by a release
func (c *Client) GetReleaseResourceStatus(releaseName, namespace string) ([]ResourceStatus, error) {
	// Get the release manifest
	manifest, err := c.GetManifest(releaseName, namespace)
	if err != nil {
		return nil, err
	}

	// Extract resource kinds from manifest
	resourceStrings := extractResourceKinds(manifest)

	// Set up K8s client
	clientset, _, err := c.getClientset()
	if err != nil {
		return nil, err
	}

	ctx := context.Background()
	var statuses []ResourceStatus

	for _, res := range resourceStrings {
		parts := strings.SplitN(res, "/", 2)
		if len(parts) != 2 {
			continue
		}
		kind := parts[0]
		name := parts[1]

		status := ResourceStatus{
			Kind: kind,
			Name: name,
		}

		// Check status based on resource type
		switch kind {
		case "Deployment":
			status.Ready, status.Status = c.checkDeploymentStatus(ctx, clientset, name, namespace)
		case "StatefulSet":
			status.Ready, status.Status = c.checkStatefulSetStatus(ctx, clientset, name, namespace)
		case "DaemonSet":
			status.Ready, status.Status = c.checkDaemonSetStatus(ctx, clientset, name, namespace)
		case "Job":
			status.Ready, status.Status = c.checkJobStatus(ctx, clientset, name, namespace)
		case "PersistentVolumeClaim":
			status.Ready, status.Status = c.checkPVCStatus(ctx, clientset, name, namespace)
		case "Ingress":
			status.Ready, status.Status = c.checkIngressStatus(ctx, clientset, name, namespace)
		case "Service", "ConfigMap", "Secret", "ServiceAccount":
			// These are ready if they exist
			status.Ready, status.Status = c.checkResourceExists(ctx, clientset, kind, name, namespace)
		default:
			// For unknown kinds, assume ready if we got here (was in manifest)
			status.Ready = true
			status.Status = "Created"
		}

		statuses = append(statuses, status)
	}

	return statuses, nil
}

func (c *Client) checkDeploymentStatus(ctx context.Context, clientset *kubernetes.Clientset, name, namespace string) (bool, string) {
	deploy, err := clientset.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return false, "Not found"
	}

	desired := int32(1)
	if deploy.Spec.Replicas != nil {
		desired = *deploy.Spec.Replicas
	}
	ready := deploy.Status.ReadyReplicas

	if ready >= desired && desired > 0 {
		return true, fmt.Sprintf("%d/%d ready", ready, desired)
	}
	return false, fmt.Sprintf("%d/%d ready", ready, desired)
}

func (c *Client) checkStatefulSetStatus(ctx context.Context, clientset *kubernetes.Clientset, name, namespace string) (bool, string) {
	sts, err := clientset.AppsV1().StatefulSets(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return false, "Not found"
	}

	desired := int32(1)
	if sts.Spec.Replicas != nil {
		desired = *sts.Spec.Replicas
	}
	ready := sts.Status.ReadyReplicas

	if ready >= desired && desired > 0 {
		return true, fmt.Sprintf("%d/%d ready", ready, desired)
	}
	return false, fmt.Sprintf("%d/%d ready", ready, desired)
}

func (c *Client) checkDaemonSetStatus(ctx context.Context, clientset *kubernetes.Clientset, name, namespace string) (bool, string) {
	ds, err := clientset.AppsV1().DaemonSets(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return false, "Not found"
	}

	desired := ds.Status.DesiredNumberScheduled
	ready := ds.Status.NumberReady

	if ready >= desired && desired > 0 {
		return true, fmt.Sprintf("%d/%d ready", ready, desired)
	}
	return false, fmt.Sprintf("%d/%d ready", ready, desired)
}

func (c *Client) checkJobStatus(ctx context.Context, clientset *kubernetes.Clientset, name, namespace string) (bool, string) {
	job, err := clientset.BatchV1().Jobs(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return false, "Not found"
	}

	if job.Status.Succeeded >= 1 {
		return true, "Complete"
	}
	if job.Status.Failed > 0 {
		return false, "Failed"
	}
	if job.Status.Active > 0 {
		return false, "Running"
	}
	return false, "Pending"
}

func (c *Client) checkPVCStatus(ctx context.Context, clientset *kubernetes.Clientset, name, namespace string) (bool, string) {
	pvc, err := clientset.CoreV1().PersistentVolumeClaims(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return false, "Not found"
	}

	phase := string(pvc.Status.Phase)
	if pvc.Status.Phase == corev1.ClaimBound {
		return true, phase
	}
	return false, phase
}

func (c *Client) checkIngressStatus(ctx context.Context, clientset *kubernetes.Clientset, name, namespace string) (bool, string) {
	ing, err := clientset.NetworkingV1().Ingresses(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return false, "Not found"
	}

	// Check if ingress has an address assigned
	if len(ing.Status.LoadBalancer.Ingress) > 0 {
		addr := ing.Status.LoadBalancer.Ingress[0]
		if addr.IP != "" {
			return true, addr.IP
		}
		if addr.Hostname != "" {
			return true, addr.Hostname
		}
	}
	return true, "Created" // Ingress exists, might not have LB
}

func (c *Client) checkResourceExists(ctx context.Context, clientset *kubernetes.Clientset, kind, name, namespace string) (bool, string) {
	var err error

	switch kind {
	case "Service":
		_, err = clientset.CoreV1().Services(namespace).Get(ctx, name, metav1.GetOptions{})
	case "ConfigMap":
		_, err = clientset.CoreV1().ConfigMaps(namespace).Get(ctx, name, metav1.GetOptions{})
	case "Secret":
		_, err = clientset.CoreV1().Secrets(namespace).Get(ctx, name, metav1.GetOptions{})
	case "ServiceAccount":
		_, err = clientset.CoreV1().ServiceAccounts(namespace).Get(ctx, name, metav1.GetOptions{})
	default:
		return true, "Created"
	}

	if err != nil {
		return false, "Not found"
	}
	return true, "Created"
}

// Ensure unused imports are used
var (
	_ = appsv1.SchemeGroupVersion
	_ = batchv1.SchemeGroupVersion
	_ = networkingv1.SchemeGroupVersion
)

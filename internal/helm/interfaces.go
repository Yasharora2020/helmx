package helm

import (
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/release"
)

// ReleaseManager handles release lifecycle operations:
// listing, installing, upgrading, rolling back, and uninstalling releases.
type ReleaseManager interface {
	ListReleases(namespace string) ([]Release, error)
	GetRelease(name, namespace string) (*release.Release, error)
	GetHistory(name, namespace string) ([]*release.Release, error)
	GetRevision(name, namespace string, revision int) (*release.Release, error)
	GetValues(name, namespace string) (map[string]interface{}, error)
	GetUserValues(name, namespace string) (map[string]interface{}, error)
	GetManifest(name, namespace string) (string, error)
	GetDeployedResources(name, namespace string) ([]string, error)
	Install(chartRef, releaseName, namespace string, values map[string]interface{}, createNamespace bool) (*Release, error)
	Upgrade(chartRef, releaseName, namespace string, values map[string]interface{}) (*Release, error)
	UpgradeValues(releaseName, namespace string, values map[string]interface{}) (*Release, error)
	Rollback(name, namespace string, revision int) error
	Uninstall(name, namespace string) error
	DryRunUpgrade(chartRef, releaseName, namespace string, values map[string]interface{}) (string, error)
	DryRunUpgradeValues(releaseName, namespace string, values map[string]interface{}) (string, error)
	Template(chartPath, releaseName, namespace string, values map[string]interface{}) (string, error)
	ListNamespaces() ([]string, error)
}

// ChartExplorer handles chart discovery, loading, and inspection.
type ChartExplorer interface {
	LoadChart(chartRef string) (*chart.Chart, error)
	LoadChartWithVersion(chartRef, version string) (*chart.Chart, error)
	GetChartInfo(ch *chart.Chart) ChartInfo
	GetChartValues(ch *chart.Chart) ChartValues
	GetChartReadme(ch *chart.Chart) string
	SearchRepos(keyword string) ([]ChartInfo, error)
	FindChartInRepos(chartName string) string
	PreviewInstall(chartRef, releaseName, namespace string, values map[string]interface{}) (string, []string, error)
	PreviewInstallWithVersion(chartRef, version, releaseName, namespace string, values map[string]interface{}) (string, []string, error)
	ListChartVersions(repoName, chartName string) ([]ChartVersion, error)
}

// RepoManager handles Helm repository CRUD operations.
type RepoManager interface {
	ListRepos() ([]Repository, error)
	AddRepo(name, url string, auth *RepoAuthOptions) error
	RemoveRepo(name string) error
	UpdateRepo(name string) error
	UpdateAllRepos() error
}

// ClusterInspector handles Kubernetes read operations:
// pod status, logs, events, resource status, and metrics.
type ClusterInspector interface {
	GetReleasePodStatus(releaseName, namespace string) ([]PodStatus, error)
	GetReleasePodStatusWithMetrics(releaseName, namespace string) ([]PodStatus, error)
	GetReleasePods(releaseName, namespace string) ([]PodInfo, error)
	GetPodLogs(podName, namespace string, opts PodLogOptions) (string, error)
	GetPodPorts(podName, namespace string) ([]PodPort, error)
	GetReleaseEvents(releaseName, namespace string) ([]K8sEvent, error)
	GetReleaseResourceStatus(releaseName, namespace string) ([]ResourceStatus, error)
	AllPodsReady(releaseName, namespace string) (bool, error)
}

// ContextManager handles kubeconfig context operations.
type ContextManager interface {
	GetCurrentContext() string
	ListContexts() ([]KubeContext, error)
	SwitchContext(contextName string) error
}

// RBACManager handles RBAC user and permission operations.
type RBACManager interface {
	ListManagedUsers() ([]ManagedUser, error)
	GrantAccess(user ManagedUser, namespace, permission string) error
	RevokeAccess(user ManagedUser, namespace string) error
	DeleteUser(user ManagedUser) error
	GenerateKubeconfig(user ManagedUser) (string, error)
}

// SecurityProvider provides access to security tooling (secrets, scanning).
type SecurityProvider interface {
	HasSecretsSupport() bool
	GetSecretsClient() *SecretsClient
	HasTrivySupport() bool
	GetTrivyClient() *TrivyClient
}

// Compile-time checks: Client must satisfy all interfaces.
var (
	_ ReleaseManager   = (*Client)(nil)
	_ ChartExplorer    = (*Client)(nil)
	_ RepoManager      = (*Client)(nil)
	_ ClusterInspector = (*Client)(nil)
	_ ContextManager   = (*Client)(nil)
	_ RBACManager      = (*Client)(nil)
	_ SecurityProvider = (*Client)(nil)
)

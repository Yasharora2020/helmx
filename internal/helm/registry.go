package helm

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"helm.sh/helm/v3/pkg/registry"
	"oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"
)

// RegistryClient wraps OCI registry operations
type RegistryClient struct {
	client     *registry.Client
	configPath string
}

// NewRegistryClient creates a new OCI registry client
func (c *Client) NewRegistryClient() (*RegistryClient, error) {
	// Create registry client with default options
	opts := []registry.ClientOption{
		registry.ClientOptDebug(false),
		registry.ClientOptEnableCache(true),
	}

	// Use Docker config for credentials if available
	configPath := filepath.Join(os.Getenv("HOME"), ".docker", "config.json")
	if _, err := os.Stat(configPath); err == nil {
		opts = append(opts, registry.ClientOptCredentialsFile(configPath))
	}

	client, err := registry.NewClient(opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create registry client: %w", err)
	}

	return &RegistryClient{
		client:     client,
		configPath: configPath,
	}, nil
}

// Login authenticates with an OCI registry
func (rc *RegistryClient) Login(host, username, password string, insecure bool) error {
	return rc.client.Login(host, registry.LoginOptBasicAuth(username, password), registry.LoginOptInsecure(insecure))
}

// Logout removes credentials for a registry
func (rc *RegistryClient) Logout(host string) error {
	return rc.client.Logout(host)
}

// Pull downloads a chart from an OCI registry
// ref format: oci://registry/repo:tag or oci://registry/repo@digest
func (rc *RegistryClient) Pull(ref string, destDir string) (string, error) {
	result, err := rc.client.Pull(ref, registry.PullOptWithChart(true))
	if err != nil {
		return "", fmt.Errorf("failed to pull chart: %w", err)
	}

	// Save the chart to destDir
	if destDir != "" && result.Chart != nil && len(result.Chart.Data) > 0 {
		// Extract chart name from reference
		_, repo, tag, _ := ParseOCIReference(ref)
		chartName := filepath.Base(repo)
		if tag != "" && tag != "latest" {
			chartName = chartName + "-" + tag
		}
		chartPath := filepath.Join(destDir, chartName+".tgz")
		if err := os.WriteFile(chartPath, result.Chart.Data, 0644); err != nil {
			return "", fmt.Errorf("failed to save chart: %w", err)
		}
		return chartPath, nil
	}

	return ref, nil
}

// Push uploads a chart to an OCI registry
func (rc *RegistryClient) Push(chartPath string, ref string) error {
	chartData, err := os.ReadFile(chartPath)
	if err != nil {
		return fmt.Errorf("failed to read chart: %w", err)
	}
	_, err = rc.client.Push(chartData, ref)
	return err
}

// OCIChartInfo represents info about a chart in OCI registry
type OCIChartInfo struct {
	Reference   string
	Digest      string
	Tags        []string
	Description string
}

// ListTags lists available tags for a chart in an OCI registry
func (rc *RegistryClient) ListTags(ref string) ([]string, error) {
	// Parse the OCI reference
	ref = strings.TrimPrefix(ref, "oci://")

	// Split into registry and repository
	parts := strings.SplitN(ref, "/", 2)
	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid OCI reference: %s", ref)
	}

	registryHost := parts[0]
	repoPath := parts[1]

	// Remove tag if present
	if idx := strings.Index(repoPath, ":"); idx != -1 {
		repoPath = repoPath[:idx]
	}

	// Create ORAS registry client
	repo, err := remote.NewRepository(registryHost + "/" + repoPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create repository client: %w", err)
	}

	// Try to use Docker credentials
	repo.Client = &auth.Client{
		Credential: auth.StaticCredential(registryHost, auth.EmptyCredential),
	}

	// List tags
	var tags []string
	err = repo.Tags(context.Background(), "", func(t []string) error {
		tags = append(tags, t...)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list tags: %w", err)
	}

	return tags, nil
}

// IsOCIReference checks if a reference is an OCI URL
func IsOCIReference(ref string) bool {
	return strings.HasPrefix(ref, "oci://")
}

// ParseOCIReference parses an OCI reference into its components
func ParseOCIReference(ref string) (registry, repo, tag string, err error) {
	ref = strings.TrimPrefix(ref, "oci://")

	// Split registry from repo/tag
	parts := strings.SplitN(ref, "/", 2)
	if len(parts) < 2 {
		return "", "", "", fmt.Errorf("invalid OCI reference: %s", ref)
	}

	registry = parts[0]
	repoTag := parts[1]

	// Split repo from tag
	if idx := strings.LastIndex(repoTag, ":"); idx != -1 {
		repo = repoTag[:idx]
		tag = repoTag[idx+1:]
	} else {
		repo = repoTag
		tag = "latest"
	}

	return registry, repo, tag, nil
}

// Common OCI registries
var CommonOCIRegistries = []struct {
	Name string
	URL  string
}{
	{"Docker Hub", "oci://registry-1.docker.io"},
	{"GitHub Container Registry", "oci://ghcr.io"},
	{"Google Artifact Registry", "oci://us-docker.pkg.dev"},
	{"AWS ECR Public", "oci://public.ecr.aws"},
	{"Azure Container Registry", "oci://azurecr.io"},
}

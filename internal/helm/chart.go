package helm

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/repo"
)

// ChartInfo represents metadata about a chart
type ChartInfo struct {
	Name        string
	Version     string
	Description string
	Home        string
	Sources     []string
	Keywords    []string
}

// ChartValues represents the values.yaml structure
type ChartValues struct {
	Raw         string                 // Raw YAML
	Parsed      map[string]interface{} // Parsed values
	Schema      []byte                 // JSON schema if available
	Comments    map[string]string      // Extracted comments for each path
	IsEncrypted bool                   // Whether content is SOPS-encrypted
	EncryptType EncryptionType         // Type of encryption (pgp, age, kms, etc.)
	Decrypted   string                 // Decrypted content (if IsEncrypted and decryption succeeded)
}

// LoadChart loads a chart from path (local dir, tgz, or repo/name)
func (c *Client) LoadChart(chartRef string) (*chart.Chart, error) {
	pathOptions := action.ChartPathOptions{}

	// Locate the chart - handles local paths, tgz files, and repo/chart format
	chartPath, err := pathOptions.LocateChart(chartRef, c.settings)
	if err != nil {
		return nil, err
	}

	// Load the chart from the resolved path
	return loader.Load(chartPath)
}

// LoadChartWithVersion loads a chart with a specific version
func (c *Client) LoadChartWithVersion(chartRef, version string) (*chart.Chart, error) {
	pathOptions := action.ChartPathOptions{
		Version: version,
	}

	// Locate the chart with the specified version
	chartPath, err := pathOptions.LocateChart(chartRef, c.settings)
	if err != nil {
		return nil, err
	}

	// Load the chart from the resolved path
	return loader.Load(chartPath)
}

// GetChartInfo extracts metadata from a chart
func (c *Client) GetChartInfo(ch *chart.Chart) ChartInfo {
	meta := ch.Metadata
	return ChartInfo{
		Name:        meta.Name,
		Version:     meta.Version,
		Description: meta.Description,
		Home:        meta.Home,
		Sources:     meta.Sources,
		Keywords:    meta.Keywords,
	}
}

// GetChartValues returns the default values from a chart
func (c *Client) GetChartValues(ch *chart.Chart) ChartValues {
	values := ChartValues{
		Parsed:   ch.Values,
		Comments: make(map[string]string),
	}

	// Get raw values.yaml content
	for _, f := range ch.Raw {
		if f.Name == "values.yaml" {
			values.Raw = string(f.Data)
			break
		}
	}

	// Get JSON schema if available
	if ch.Schema != nil {
		values.Schema = ch.Schema
	}

	// Check for encryption
	if values.Raw != "" {
		values.IsEncrypted = IsSOPSEncrypted(values.Raw)
		if values.IsEncrypted {
			values.EncryptType = DetectEncryptionType(values.Raw)
		}
	}

	return values
}

// GetChartReadme extracts the README from a chart
func (c *Client) GetChartReadme(ch *chart.Chart) string {
	// Check chart.Files first (newer Helm versions)
	for _, f := range ch.Files {
		name := strings.ToLower(f.Name)
		if name == "readme.md" || name == "readme.txt" || name == "readme" {
			return string(f.Data)
		}
	}

	// Also check Raw files (some charts store it there)
	for _, f := range ch.Raw {
		name := strings.ToLower(f.Name)
		if name == "readme.md" || name == "readme.txt" || name == "readme" {
			return string(f.Data)
		}
	}

	return ""
}

// SearchRepos searches for charts in configured repositories
func (c *Client) SearchRepos(keyword string) ([]ChartInfo, error) {
	// Load repository file
	repoFile, err := repo.LoadFile(c.settings.RepositoryConfig)
	if err != nil {
		return nil, err
	}

	var results []ChartInfo

	for _, re := range repoFile.Repositories {
		// Load the index file for this repo from the cache directory
		indexPath := filepath.Join(c.settings.RepositoryCache, re.Name+"-index.yaml")
		indexFile, err := repo.LoadIndexFile(indexPath)
		if err != nil {
			continue // Skip repos we can't load
		}

		// Search through chart entries
		for name, versions := range indexFile.Entries {
			if containsIgnoreCase(name, keyword) && len(versions) > 0 {
				latest := versions[0]
				results = append(results, ChartInfo{
					Name:        re.Name + "/" + name,
					Version:     latest.Version,
					Description: latest.Description,
					Home:        latest.Home,
					Keywords:    latest.Keywords,
				})
			}
		}
	}

	return results, nil
}

// containsIgnoreCase checks if s contains substr (case-insensitive)
func containsIgnoreCase(s, substr string) bool {
	return strings.Contains(
		strings.ToLower(s),
		strings.ToLower(substr),
	)
}

// FindChartInRepos searches for a chart by exact name in configured repositories
// Returns the full chart reference (repo/chart) or empty string if not found
func (c *Client) FindChartInRepos(chartName string) string {
	// Load repository file
	repoFile, err := repo.LoadFile(c.settings.RepositoryConfig)
	if err != nil {
		return ""
	}

	for _, re := range repoFile.Repositories {
		// Load the index file for this repo
		indexPath := filepath.Join(c.settings.RepositoryCache, re.Name+"-index.yaml")
		indexFile, err := repo.LoadIndexFile(indexPath)
		if err != nil {
			continue
		}

		// Look for exact chart name match
		if _, exists := indexFile.Entries[chartName]; exists {
			return re.Name + "/" + chartName
		}
	}

	return ""
}

// PreviewInstall renders what would be installed without actually installing
func (c *Client) PreviewInstall(chartRef, releaseName, namespace string, values map[string]interface{}) (string, []string, error) {
	ch, err := c.LoadChart(chartRef)
	if err != nil {
		return "", nil, err
	}

	actionConfig, err := c.getActionConfig(namespace)
	if err != nil {
		return "", nil, err
	}

	install := action.NewInstall(actionConfig)
	install.DryRun = true
	install.ReleaseName = releaseName
	install.Namespace = namespace
	install.ClientOnly = true // Don't need cluster access for dry-run

	rel, err := install.Run(ch, values)
	if err != nil {
		return "", nil, err
	}

	// Extract resource kinds that will be created
	resources := extractResourceKinds(rel.Manifest)

	return rel.Manifest, resources, nil
}

// PreviewInstallWithVersion does a dry-run install with a specific chart version
func (c *Client) PreviewInstallWithVersion(chartRef, version, releaseName, namespace string, values map[string]interface{}) (string, []string, error) {
	ch, err := c.LoadChartWithVersion(chartRef, version)
	if err != nil {
		return "", nil, err
	}

	actionConfig, err := c.getActionConfig(namespace)
	if err != nil {
		return "", nil, err
	}

	install := action.NewInstall(actionConfig)
	install.DryRun = true
	install.ReleaseName = releaseName
	install.Namespace = namespace
	install.ClientOnly = true

	rel, err := install.Run(ch, values)
	if err != nil {
		return "", nil, err
	}

	resources := extractResourceKinds(rel.Manifest)
	return rel.Manifest, resources, nil
}

// extractResourceKinds parses the manifest to find what resources will be created
func extractResourceKinds(manifest string) []string {
	var resources []string
	seen := make(map[string]bool)

	// Split manifest into individual documents
	docs := strings.Split(manifest, "---")

	for _, doc := range docs {
		doc = strings.TrimSpace(doc)
		if doc == "" {
			continue
		}

		// Parse each YAML document
		var obj map[string]interface{}
		if err := yaml.Unmarshal([]byte(doc), &obj); err != nil {
			continue
		}

		// Extract kind and name
		kind, _ := obj["kind"].(string)
		metadata, _ := obj["metadata"].(map[string]interface{})
		name, _ := metadata["name"].(string)

		if kind != "" {
			resourceStr := kind
			if name != "" {
				resourceStr = kind + "/" + name
			}
			if !seen[resourceStr] {
				seen[resourceStr] = true
				resources = append(resources, resourceStr)
			}
		}
	}

	return resources
}

// ChartVersion represents a specific version of a chart
type ChartVersion struct {
	Version     string
	AppVersion  string
	Description string
	Created     time.Time
}

// ListChartVersions returns all versions of a chart from a specific repo
func (c *Client) ListChartVersions(repoName, chartName string) ([]ChartVersion, error) {
	// Load the index file for this repo from the cache directory
	indexPath := filepath.Join(c.settings.RepositoryCache, repoName+"-index.yaml")
	indexFile, err := repo.LoadIndexFile(indexPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load repo index: %w", err)
	}

	versions, exists := indexFile.Entries[chartName]
	if !exists {
		return nil, fmt.Errorf("chart %s not found in repo %s", chartName, repoName)
	}

	// Convert to our type (versions are already sorted by version descending)
	var result []ChartVersion
	for _, v := range versions {
		result = append(result, ChartVersion{
			Version:     v.Version,
			AppVersion:  v.AppVersion,
			Description: v.Description,
			Created:     v.Created,
		})
	}
	return result, nil
}

// ChartDependency represents a node in the dependency tree
type ChartDependency struct {
	Name       string             // Chart name
	Version    string             // Chart version
	Repository string             // Repository URL
	Condition  string             // Condition for enabling (e.g., "redis.enabled")
	IsOptional bool               // Has condition/tags (may not be loaded)
	IsLoaded   bool               // Whether sub-chart was actually loaded
	Children   []*ChartDependency // Sub-dependencies
}

// BuildDependencyTree extracts the dependency tree from a loaded chart
func BuildDependencyTree(ch *chart.Chart) *ChartDependency {
	if ch == nil {
		return nil
	}

	root := &ChartDependency{
		Name:     ch.Name(),
		Version:  ch.Metadata.Version,
		IsLoaded: true,
	}

	// Map declared dependencies for condition/repository info
	declared := make(map[string]*chart.Dependency)
	if ch.Metadata.Dependencies != nil {
		for _, d := range ch.Metadata.Dependencies {
			name := d.Name
			if d.Alias != "" {
				name = d.Alias
			}
			declared[name] = d
		}
	}

	// Build from loaded sub-charts (recursive)
	for _, sub := range ch.Dependencies() {
		child := BuildDependencyTree(sub)
		// Enrich with declared dependency info
		subName := sub.Name()
		if decl, ok := declared[subName]; ok {
			child.Repository = decl.Repository
			child.Condition = decl.Condition
			child.IsOptional = decl.Condition != "" || len(decl.Tags) > 0
			delete(declared, subName) // Mark as processed
		}
		root.Children = append(root.Children, child)
	}

	// Add declared but unloaded dependencies (optional ones that weren't enabled)
	for name, decl := range declared {
		root.Children = append(root.Children, &ChartDependency{
			Name:       name,
			Version:    decl.Version,
			Repository: decl.Repository,
			Condition:  decl.Condition,
			IsOptional: true,
			IsLoaded:   false,
		})
	}

	return root
}

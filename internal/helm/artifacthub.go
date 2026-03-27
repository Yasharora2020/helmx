package helm

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	DefaultArtifactHubAPI = "https://artifacthub.io/api/v1"
	artifactHubTimeout    = 10 * time.Second
)

// ArtifactHubResult represents a chart from Artifact Hub
type ArtifactHubResult struct {
	Name        string
	Version     string
	AppVersion  string
	Description string
	RepoName    string
	RepoURL     string
	Official    bool
	Stars       int
}

// artifactHubResponse matches the API response structure
type artifactHubResponse struct {
	Packages []artifactHubPackage `json:"packages"`
}

type artifactHubPackage struct {
	Name        string                `json:"name"`
	Version     string                `json:"version"`
	AppVersion  string                `json:"app_version"`
	Description string                `json:"description"`
	Stars       int                   `json:"stars"`
	Repository  artifactHubRepository `json:"repository"`
}

type artifactHubRepository struct {
	Name     string `json:"name"`
	URL      string `json:"url"`
	Official bool   `json:"official"`
}

// SearchArtifactHub queries the Artifact Hub API for Helm charts
// registryURL can be empty to use the default public Artifact Hub
func SearchArtifactHub(query string, limit int, registryURL string) ([]ArtifactHubResult, error) {
	if query == "" {
		return nil, nil
	}

	if limit <= 0 {
		limit = 10
	}

	// Use default if not specified
	baseURL := registryURL
	if baseURL == "" {
		baseURL = DefaultArtifactHubAPI
	}

	// Ensure URL ends correctly
	baseURL = strings.TrimSuffix(baseURL, "/")

	// Build request URL
	params := url.Values{}
	params.Set("ts_query_web", query)
	params.Set("kind", "0") // 0 = Helm charts
	params.Set("limit", fmt.Sprintf("%d", limit))

	reqURL := fmt.Sprintf("%s/packages/search?%s", baseURL, params.Encode())

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: artifactHubTimeout,
	}

	resp, err := client.Get(reqURL)
	if err != nil {
		return nil, fmt.Errorf("failed to query Artifact Hub: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("artifact hub returned status %d", resp.StatusCode)
	}

	// Parse response
	var apiResp artifactHubResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("failed to parse Artifact Hub response: %w", err)
	}

	// Convert to our result type
	results := make([]ArtifactHubResult, len(apiResp.Packages))
	for i, pkg := range apiResp.Packages {
		results[i] = ArtifactHubResult{
			Name:        pkg.Name,
			Version:     pkg.Version,
			AppVersion:  pkg.AppVersion,
			Description: pkg.Description,
			RepoName:    pkg.Repository.Name,
			RepoURL:     pkg.Repository.URL,
			Official:    pkg.Repository.Official,
			Stars:       pkg.Stars,
		}
	}

	return results, nil
}

// artifactHubPackageDetail represents detailed package info from Artifact Hub
type artifactHubPackageDetail struct {
	Readme string `json:"readme"`
}

// GetArtifactHubReadme fetches the README for a specific package from Artifact Hub
func GetArtifactHubReadme(repoName, packageName string, registryURL string) (string, error) {
	// Use default if not specified
	baseURL := registryURL
	if baseURL == "" {
		baseURL = DefaultArtifactHubAPI
	}

	// Ensure URL ends correctly
	baseURL = strings.TrimSuffix(baseURL, "/")

	// Build request URL: /packages/helm/{repo}/{package}
	reqURL := fmt.Sprintf("%s/packages/helm/%s/%s", baseURL, repoName, packageName)

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: artifactHubTimeout,
	}

	resp, err := client.Get(reqURL)
	if err != nil {
		return "", fmt.Errorf("failed to fetch package details: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("artifact hub returned status %d", resp.StatusCode)
	}

	// Parse response
	var detail artifactHubPackageDetail
	if err := json.NewDecoder(resp.Body).Decode(&detail); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	return detail.Readme, nil
}

// ArtifactHubVersion represents a version from Artifact Hub
type ArtifactHubVersion struct {
	Version          string
	CreatedAt        int64 // Unix timestamp
	PreRelease       bool
	ContainsSecurity bool
}

// artifactHubPackageVersions matches the API response for package details
type artifactHubPackageVersions struct {
	AvailableVersions []struct {
		Version          string `json:"version"`
		CreatedAt        int64  `json:"created_at"`
		PreRelease       bool   `json:"prerelease"`
		ContainsSecurity bool   `json:"contains_security_updates"`
	} `json:"available_versions"`
}

// GetArtifactHubVersions fetches all versions of a package from Artifact Hub
func GetArtifactHubVersions(repoName, packageName, registryURL string) ([]ArtifactHubVersion, error) {
	// Use default if not specified
	baseURL := registryURL
	if baseURL == "" {
		baseURL = DefaultArtifactHubAPI
	}

	// Ensure URL ends correctly
	baseURL = strings.TrimSuffix(baseURL, "/")

	// Build request URL: /packages/helm/{repo}/{package}
	reqURL := fmt.Sprintf("%s/packages/helm/%s/%s", baseURL, repoName, packageName)

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: artifactHubTimeout,
	}

	resp, err := client.Get(reqURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch package versions: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("artifact hub returned status %d", resp.StatusCode)
	}

	// Parse response
	var pkg artifactHubPackageVersions
	if err := json.NewDecoder(resp.Body).Decode(&pkg); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Convert to our type
	var versions []ArtifactHubVersion
	for _, v := range pkg.AvailableVersions {
		versions = append(versions, ArtifactHubVersion{
			Version:          v.Version,
			CreatedAt:        v.CreatedAt,
			PreRelease:       v.PreRelease,
			ContainsSecurity: v.ContainsSecurity,
		})
	}

	return versions, nil
}

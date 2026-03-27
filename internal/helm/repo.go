package helm

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"helm.sh/helm/v3/pkg/getter"
	"helm.sh/helm/v3/pkg/repo"
)

// Repository represents a Helm repository.
type Repository struct {
	Name string
	URL  string
	// Status indicates the repository state. Possible values:
	//   - "ready": Repository is configured and usable
	//   - "updating": Repository index is being refreshed
	//   - "error": Repository has a configuration or connectivity issue
	Status string
	// LastUpdated is when the repository index was last synced.
	// Zero value means the index file doesn't exist or couldn't be read.
	LastUpdated time.Time
	// Authentication fields
	Username              string // Username for basic auth (empty if no auth)
	HasAuth               bool   // True if repository requires authentication
	InsecureSkipTLSVerify bool   // Skip TLS certificate verification
}

// RepoAuthOptions contains authentication options for adding a repository.
// All fields are optional - leave empty for public repositories.
type RepoAuthOptions struct {
	Username              string // Username for basic auth
	Password              string // Password for basic auth
	CertFile              string // Path to TLS client certificate file
	KeyFile               string // Path to TLS client key file
	CAFile                string // Path to CA certificate file
	InsecureSkipTLSVerify bool   // Skip TLS certificate verification
	PassCredentialsAll    bool   // Pass credentials to all domains (for redirects)
}

// ListRepos returns all configured repositories
func (c *Client) ListRepos() ([]Repository, error) {
	repoFile := c.settings.RepositoryConfig
	repoCache := c.settings.RepositoryCache

	f, err := repo.LoadFile(repoFile)
	if err != nil {
		if os.IsNotExist(err) {
			return []Repository{}, nil
		}
		return nil, err
	}

	repos := make([]Repository, len(f.Repositories))
	for i, r := range f.Repositories {
		repos[i] = Repository{
			Name:                  r.Name,
			URL:                   r.URL,
			Status:                "ready",
			Username:              r.Username,
			HasAuth:               r.Username != "" || r.Password != "",
			InsecureSkipTLSVerify: r.InsecureSkipTLSverify,
		}

		// Get last updated time from index file modification time
		indexFile := filepath.Join(repoCache, r.Name+"-index.yaml")
		if info, err := os.Stat(indexFile); err == nil {
			repos[i].LastUpdated = info.ModTime()
		}
	}

	return repos, nil
}

// AddRepo adds a new repository with optional authentication.
// Pass nil for auth if the repository is public.
func (c *Client) AddRepo(name, url string, auth *RepoAuthOptions) error {
	repoFile := c.settings.RepositoryConfig
	repoCache := c.settings.RepositoryCache

	// Ensure directories exist
	if err := os.MkdirAll(filepath.Dir(repoFile), 0755); err != nil {
		return err
	}
	if err := os.MkdirAll(repoCache, 0755); err != nil {
		return err
	}

	// Load existing file or create new
	f, err := repo.LoadFile(repoFile)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if f == nil {
		f = repo.NewFile()
	}

	// Check if repo already exists
	if f.Has(name) {
		return nil // Already exists, treat as success
	}

	// Create new repo entry
	entry := &repo.Entry{
		Name: name,
		URL:  url,
	}

	// Add authentication if provided
	if auth != nil {
		entry.Username = auth.Username
		entry.Password = auth.Password
		entry.CertFile = auth.CertFile
		entry.KeyFile = auth.KeyFile
		entry.CAFile = auth.CAFile
		entry.InsecureSkipTLSverify = auth.InsecureSkipTLSVerify
		entry.PassCredentialsAll = auth.PassCredentialsAll
	}

	// Download and verify the repo index
	chartRepo, err := repo.NewChartRepository(entry, getter.All(c.settings))
	if err != nil {
		return err
	}

	chartRepo.CachePath = repoCache

	// Download index
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	if _, err := chartRepo.DownloadIndexFile(); err != nil {
		// Try with context if the method supports it
		_ = ctx // Use context in future if needed
		return err
	}

	// Add to repo file
	f.Update(entry)

	// Save
	return f.WriteFile(repoFile, 0644)
}

// RemoveRepo removes a repository
func (c *Client) RemoveRepo(name string) error {
	repoFile := c.settings.RepositoryConfig

	f, err := repo.LoadFile(repoFile)
	if err != nil {
		return err
	}

	if !f.Remove(name) {
		return nil // Didn't exist, treat as success
	}

	// Also remove the cache file
	cacheFile := filepath.Join(c.settings.RepositoryCache, name+"-index.yaml")
	_ = os.Remove(cacheFile) // Ignore error if file doesn't exist

	return f.WriteFile(repoFile, 0644)
}

// UpdateRepo updates a single repository's index
func (c *Client) UpdateRepo(name string) error {
	repoFile := c.settings.RepositoryConfig
	repoCache := c.settings.RepositoryCache

	f, err := repo.LoadFile(repoFile)
	if err != nil {
		return err
	}

	var entry *repo.Entry
	for _, e := range f.Repositories {
		if e.Name == name {
			entry = e
			break
		}
	}

	if entry == nil {
		return fmt.Errorf("repository %q not found", name)
	}

	chartRepo, err := repo.NewChartRepository(entry, getter.All(c.settings))
	if err != nil {
		return err
	}

	chartRepo.CachePath = repoCache

	_, err = chartRepo.DownloadIndexFile()
	return err
}

// UpdateAllRepos updates all repository indexes
func (c *Client) UpdateAllRepos() error {
	repos, err := c.ListRepos()
	if err != nil {
		return err
	}

	var errs []error
	for _, r := range repos {
		if err := c.UpdateRepo(r.Name); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", r.Name, err))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("failed to update %d repos: %v", len(errs), errs)
	}
	return nil
}

// SearchAllRepos searches for charts across all repositories
func (c *Client) SearchAllRepos(keyword string) ([]ChartInfo, error) {
	repoCache := c.settings.RepositoryCache

	repos, err := c.ListRepos()
	if err != nil {
		return nil, err
	}

	var results []ChartInfo

	for _, r := range repos {
		indexFile := filepath.Join(repoCache, r.Name+"-index.yaml")
		idx, err := repo.LoadIndexFile(indexFile)
		if err != nil {
			continue // Skip repos we can't load
		}

		for name, versions := range idx.Entries {
			if containsIgnoreCase(name, keyword) || containsIgnoreCase(r.Name, keyword) {
				if len(versions) > 0 {
					latest := versions[0]
					results = append(results, ChartInfo{
						Name:        r.Name + "/" + name,
						Version:     latest.Version,
						Description: latest.Description,
						Home:        latest.Home,
						Keywords:    latest.Keywords,
					})
				}
			}
		}
	}

	return results, nil
}

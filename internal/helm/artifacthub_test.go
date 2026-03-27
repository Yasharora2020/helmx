package helm

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSearchArtifactHub(t *testing.T) {
	// Mock server response
	mockResponse := artifactHubResponse{
		Packages: []artifactHubPackage{
			{
				Name:        "nginx",
				Version:     "1.0.0",
				AppVersion:  "1.21.0",
				Description: "NGINX Ingress Controller",
				Stars:       100,
				Repository: artifactHubRepository{
					Name:     "bitnami",
					URL:      "https://charts.bitnami.com/bitnami",
					Official: true,
				},
			},
			{
				Name:        "nginx-ingress",
				Version:     "2.0.0",
				AppVersion:  "1.22.0",
				Description: "NGINX Ingress Controller by Ingress-NGINX",
				Stars:       50,
				Repository: artifactHubRepository{
					Name:     "ingress-nginx",
					URL:      "https://kubernetes.github.io/ingress-nginx",
					Official: false,
				},
			},
		},
	}

	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request path
		if r.URL.Path != "/packages/search" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		// Verify query parameters
		query := r.URL.Query()
		if query.Get("kind") != "0" {
			t.Errorf("expected kind=0, got %s", query.Get("kind"))
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(mockResponse)
	}))
	defer server.Close()

	// Test search
	results, err := SearchArtifactHub("nginx", 10, server.URL)
	if err != nil {
		t.Fatalf("SearchArtifactHub failed: %v", err)
	}

	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}

	// Verify first result
	if results[0].Name != "nginx" {
		t.Errorf("expected name 'nginx', got '%s'", results[0].Name)
	}
	if results[0].Version != "1.0.0" {
		t.Errorf("expected version '1.0.0', got '%s'", results[0].Version)
	}
	if results[0].RepoName != "bitnami" {
		t.Errorf("expected repo 'bitnami', got '%s'", results[0].RepoName)
	}
	if !results[0].Official {
		t.Error("expected official to be true")
	}
	if results[0].Stars != 100 {
		t.Errorf("expected 100 stars, got %d", results[0].Stars)
	}
}

func TestSearchArtifactHub_EmptyQuery(t *testing.T) {
	results, err := SearchArtifactHub("", 10, "")
	if err != nil {
		t.Fatalf("SearchArtifactHub with empty query failed: %v", err)
	}
	if results != nil {
		t.Errorf("expected nil results for empty query, got %v", results)
	}
}

func TestSearchArtifactHub_DefaultLimit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify limit is set to 10 when 0 or negative is passed
		limit := r.URL.Query().Get("limit")
		if limit != "10" {
			t.Errorf("expected default limit of 10, got %s", limit)
		}
		_ = json.NewEncoder(w).Encode(artifactHubResponse{})
	}))
	defer server.Close()

	_, _ = SearchArtifactHub("test", 0, server.URL)
	_, _ = SearchArtifactHub("test", -1, server.URL)
}

func TestSearchArtifactHub_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	_, err := SearchArtifactHub("nginx", 10, server.URL)
	if err == nil {
		t.Error("expected error for HTTP 500, got nil")
	}
}

func TestGetArtifactHubReadme(t *testing.T) {
	expectedReadme := "# NGINX Chart\n\nThis is a test readme."

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request path
		expectedPath := "/packages/helm/bitnami/nginx"
		if r.URL.Path != expectedPath {
			t.Errorf("unexpected path: %s, expected: %s", r.URL.Path, expectedPath)
		}

		response := artifactHubPackageDetail{
			Readme: expectedReadme,
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	readme, err := GetArtifactHubReadme("bitnami", "nginx", server.URL)
	if err != nil {
		t.Fatalf("GetArtifactHubReadme failed: %v", err)
	}

	if readme != expectedReadme {
		t.Errorf("expected readme %q, got %q", expectedReadme, readme)
	}
}

func TestGetArtifactHubReadme_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	_, err := GetArtifactHubReadme("bitnami", "nonexistent", server.URL)
	if err == nil {
		t.Error("expected error for HTTP 404, got nil")
	}
}

func TestGetArtifactHubVersions(t *testing.T) {
	// Mock response with available versions
	mockResponse := artifactHubPackageVersions{
		AvailableVersions: []struct {
			Version          string `json:"version"`
			CreatedAt        int64  `json:"created_at"`
			PreRelease       bool   `json:"prerelease"`
			ContainsSecurity bool   `json:"contains_security_updates"`
		}{
			{Version: "15.0.0", CreatedAt: 1700000000, PreRelease: false, ContainsSecurity: false},
			{Version: "14.0.0", CreatedAt: 1690000000, PreRelease: false, ContainsSecurity: true},
			{Version: "14.0.0-beta.1", CreatedAt: 1685000000, PreRelease: true, ContainsSecurity: false},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request path
		expectedPath := "/packages/helm/bitnami/postgresql"
		if r.URL.Path != expectedPath {
			t.Errorf("unexpected path: %s, expected: %s", r.URL.Path, expectedPath)
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(mockResponse)
	}))
	defer server.Close()

	versions, err := GetArtifactHubVersions("bitnami", "postgresql", server.URL)
	if err != nil {
		t.Fatalf("GetArtifactHubVersions failed: %v", err)
	}

	if len(versions) != 3 {
		t.Errorf("expected 3 versions, got %d", len(versions))
	}

	// Verify first version
	if versions[0].Version != "15.0.0" {
		t.Errorf("expected version '15.0.0', got '%s'", versions[0].Version)
	}
	if versions[0].PreRelease {
		t.Error("expected first version to not be pre-release")
	}

	// Verify pre-release version
	if !versions[2].PreRelease {
		t.Error("expected third version to be pre-release")
	}

	// Verify security update flag
	if !versions[1].ContainsSecurity {
		t.Error("expected second version to contain security updates")
	}
}

func TestGetArtifactHubVersions_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	_, err := GetArtifactHubVersions("bitnami", "nonexistent", server.URL)
	if err == nil {
		t.Error("expected error for HTTP 404, got nil")
	}
}

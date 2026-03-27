// Package helm provides a high-level wrapper around the Helm SDK v3.
//
// This package abstracts the complexity of the Helm SDK into simple, reusable
// functions that can be called from the TUI layer without requiring knowledge
// of Helm internals.
//
// # Core Components
//
// Client is the main entry point for all Helm operations:
//   - Release management: ListReleases, Install, Upgrade, Rollback, Uninstall
//   - Chart operations: LoadChart, GetChartInfo, GetChartValues, SearchCharts
//   - Repository management: ListRepos, AddRepo, RemoveRepo, UpdateRepo
//   - Kubernetes integration: GetReleasePodStatus, GetManifest
//   - Dry-run support: DryRunUpgrade for previewing changes
//
// # Additional Clients
//
// SecretsClient handles helm-secrets plugin integration for SOPS-encrypted values.
// Use Client.GetSecretsClient() to access encryption/decryption operations.
//
// TrivyClient provides security scanning capabilities for container images.
// Use Client.GetTrivyClient() to access vulnerability scanning.
//
// # Chart Discovery
//
// Charts can be discovered from multiple sources:
//   - Local repositories via SearchRepos()
//   - Artifact Hub via SearchArtifactHub() and GetArtifactHubReadme()
//   - OCI registries via the registry.go implementation
//
// # Kubernetes Context
//
// The context.go file provides kubeconfig context management:
//   - ListContexts() returns all available contexts
//   - SwitchContext() changes the active context
//   - ValidateContext() checks if a context is valid before switching
//
// # Error Handling
//
// Most functions return errors from the underlying Helm SDK or Kubernetes client.
// Callers should handle these errors appropriately, especially for user-facing
// operations where meaningful error messages improve the experience.
//
// # Thread Safety
//
// The Client is NOT thread-safe. Each concurrent operation should use its own
// Client instance or implement external synchronization.
package helm

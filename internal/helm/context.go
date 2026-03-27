package helm

import (
	"fmt"
	"sort"
	"sync"

	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
)

// KubeContext represents a kubectl context
type KubeContext struct {
	Name      string
	Cluster   string
	User      string
	Namespace string
	IsCurrent bool
}

// ListContexts returns all available kubectl contexts
func (c *Client) ListContexts() ([]KubeContext, error) {
	config, err := clientcmd.LoadFromFile(c.kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("failed to load kubeconfig: %w", err)
	}

	currentContext := config.CurrentContext
	contexts := make([]KubeContext, 0, len(config.Contexts))

	for name, ctx := range config.Contexts {
		contexts = append(contexts, KubeContext{
			Name:      name,
			Cluster:   ctx.Cluster,
			User:      ctx.AuthInfo,
			Namespace: ctx.Namespace,
			IsCurrent: name == currentContext,
		})
	}

	// Sort by name for consistent ordering
	sort.Slice(contexts, func(i, j int) bool {
		return contexts[i].Name < contexts[j].Name
	})

	return contexts, nil
}

// SwitchContext changes the current kubectl context
func (c *Client) SwitchContext(contextName string) error {
	config, err := clientcmd.LoadFromFile(c.kubeconfig)
	if err != nil {
		return fmt.Errorf("failed to load kubeconfig: %w", err)
	}

	// Verify the context exists
	if _, exists := config.Contexts[contextName]; !exists {
		return fmt.Errorf("context %q not found", contextName)
	}

	// Update the current context
	config.CurrentContext = contextName

	// Write the config back
	if err := clientcmd.WriteToFile(*config, c.kubeconfig); err != nil {
		return fmt.Errorf("failed to write kubeconfig: %w", err)
	}

	// Update the client's settings to use the new context
	c.settings.KubeContext = contextName

	// Reset cached clientset since context changed
	c.clientsetOnce = sync.Once{}
	c.cachedClientset = nil
	c.cachedConfig = nil
	c.clientsetErr = nil

	return nil
}

// GetContextInfo returns detailed information about a specific context
func (c *Client) GetContextInfo(contextName string) (*KubeContext, error) {
	config, err := clientcmd.LoadFromFile(c.kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("failed to load kubeconfig: %w", err)
	}

	ctx, exists := config.Contexts[contextName]
	if !exists {
		return nil, fmt.Errorf("context %q not found", contextName)
	}

	return &KubeContext{
		Name:      contextName,
		Cluster:   ctx.Cluster,
		User:      ctx.AuthInfo,
		Namespace: ctx.Namespace,
		IsCurrent: contextName == config.CurrentContext,
	}, nil
}

// ValidateContext checks if a context is valid and can connect
func (c *Client) ValidateContext(contextName string) error {
	config, err := clientcmd.LoadFromFile(c.kubeconfig)
	if err != nil {
		return fmt.Errorf("failed to load kubeconfig: %w", err)
	}

	ctx, exists := config.Contexts[contextName]
	if !exists {
		return fmt.Errorf("context %q not found", contextName)
	}

	// Check if cluster exists
	if _, exists := config.Clusters[ctx.Cluster]; !exists {
		return fmt.Errorf("cluster %q referenced by context not found", ctx.Cluster)
	}

	// Check if user exists
	if _, exists := config.AuthInfos[ctx.AuthInfo]; !exists {
		return fmt.Errorf("user %q referenced by context not found", ctx.AuthInfo)
	}

	return nil
}

// RenameContext renames a context
func (c *Client) RenameContext(oldName, newName string) error {
	config, err := clientcmd.LoadFromFile(c.kubeconfig)
	if err != nil {
		return fmt.Errorf("failed to load kubeconfig: %w", err)
	}

	// Check if old context exists
	ctx, exists := config.Contexts[oldName]
	if !exists {
		return fmt.Errorf("context %q not found", oldName)
	}

	// Check if new name already exists
	if _, exists := config.Contexts[newName]; exists {
		return fmt.Errorf("context %q already exists", newName)
	}

	// Copy context to new name
	config.Contexts[newName] = ctx

	// Delete old context
	delete(config.Contexts, oldName)

	// Update current context if it was the renamed one
	if config.CurrentContext == oldName {
		config.CurrentContext = newName
	}

	// Write the config back
	if err := clientcmd.WriteToFile(*config, c.kubeconfig); err != nil {
		return fmt.Errorf("failed to write kubeconfig: %w", err)
	}

	return nil
}

// DeleteContext removes a context from kubeconfig
func (c *Client) DeleteContext(contextName string) error {
	config, err := clientcmd.LoadFromFile(c.kubeconfig)
	if err != nil {
		return fmt.Errorf("failed to load kubeconfig: %w", err)
	}

	// Check if context exists
	if _, exists := config.Contexts[contextName]; !exists {
		return fmt.Errorf("context %q not found", contextName)
	}

	// Don't allow deleting the current context
	if config.CurrentContext == contextName {
		return fmt.Errorf("cannot delete current context %q", contextName)
	}

	// Delete the context
	delete(config.Contexts, contextName)

	// Write the config back
	if err := clientcmd.WriteToFile(*config, c.kubeconfig); err != nil {
		return fmt.Errorf("failed to write kubeconfig: %w", err)
	}

	return nil
}

// GetCurrentContextDetails returns full details about the current context
func (c *Client) GetCurrentContextDetails() (*KubeContext, error) {
	currentContext := c.GetCurrentContext()
	if currentContext == "unknown" {
		return nil, fmt.Errorf("no current context set")
	}
	return c.GetContextInfo(currentContext)
}

// SetContextNamespace sets the default namespace for a context
func (c *Client) SetContextNamespace(contextName, namespace string) error {
	config, err := clientcmd.LoadFromFile(c.kubeconfig)
	if err != nil {
		return fmt.Errorf("failed to load kubeconfig: %w", err)
	}

	ctx, exists := config.Contexts[contextName]
	if !exists {
		return fmt.Errorf("context %q not found", contextName)
	}

	// Update the namespace
	ctx.Namespace = namespace
	config.Contexts[contextName] = ctx

	// Write the config back
	if err := clientcmd.WriteToFile(*config, c.kubeconfig); err != nil {
		return fmt.Errorf("failed to write kubeconfig: %w", err)
	}

	return nil
}

// CreateContext creates a new context with the given parameters
func (c *Client) CreateContext(name, cluster, user, namespace string) error {
	config, err := clientcmd.LoadFromFile(c.kubeconfig)
	if err != nil {
		return fmt.Errorf("failed to load kubeconfig: %w", err)
	}

	// Check if context already exists
	if _, exists := config.Contexts[name]; exists {
		return fmt.Errorf("context %q already exists", name)
	}

	// Check if cluster exists
	if _, exists := config.Clusters[cluster]; !exists {
		return fmt.Errorf("cluster %q not found", cluster)
	}

	// Check if user exists
	if _, exists := config.AuthInfos[user]; !exists {
		return fmt.Errorf("user %q not found", user)
	}

	// Create the context
	config.Contexts[name] = &api.Context{
		Cluster:   cluster,
		AuthInfo:  user,
		Namespace: namespace,
	}

	// Write the config back
	if err := clientcmd.WriteToFile(*config, c.kubeconfig); err != nil {
		return fmt.Errorf("failed to write kubeconfig: %w", err)
	}

	return nil
}

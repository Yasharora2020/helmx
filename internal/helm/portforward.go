package helm

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
)

// PortForward represents an active port-forward connection.
//
// Port forwards run in background goroutines and can be started/stopped
// independently. The Status field tracks the connection state.
type PortForward struct {
	mu           sync.RWMutex
	ID           string // Unique identifier for this forward
	PodName      string // Kubernetes pod name
	PodNamespace string // Kubernetes namespace
	LocalPort    int    // Local port to listen on
	RemotePort   int    // Remote container port to forward to
	ReleaseName  string // Associated Helm release name (for display, optional)
	// Status indicates the port-forward state. Possible values:
	//   - "running": Port-forward is active and accepting connections
	//   - "stopped": Port-forward was gracefully stopped
	//   - "error": Port-forward failed (see Error field for details)
	Status    string
	Error     string    // Error message when Status is "error"
	StartedAt time.Time // When the port-forward was started

	// Internal fields for goroutine management
	cancelFunc context.CancelFunc
	stopChan   chan struct{}
	readyChan  chan struct{}
}

// GetStatus returns the current status in a thread-safe manner.
func (pf *PortForward) GetStatus() string {
	pf.mu.RLock()
	defer pf.mu.RUnlock()
	return pf.Status
}

// setStatus sets the status in a thread-safe manner.
func (pf *PortForward) setStatus(status string) {
	pf.mu.Lock()
	defer pf.mu.Unlock()
	pf.Status = status
}

// setError sets both status and error message in a thread-safe manner.
func (pf *PortForward) setError(status, errMsg string) {
	pf.mu.Lock()
	defer pf.mu.Unlock()
	pf.Status = status
	pf.Error = errMsg
}

// PortForwardManager manages multiple port-forward connections
type PortForwardManager struct {
	mu         sync.RWMutex
	forwards   map[string]*PortForward
	kubeconfig string
	nextID     int
	restConfig *rest.Config
	clientset  *kubernetes.Clientset
	initOnce   sync.Once
	initErr    error
}

// NewPortForwardManager creates a new port-forward manager
func NewPortForwardManager(kubeconfig string) *PortForwardManager {
	return &PortForwardManager{
		forwards:   make(map[string]*PortForward),
		kubeconfig: kubeconfig,
		nextID:     1,
	}
}

// initialize sets up the kubernetes client using sync.Once for thread safety.
func (m *PortForwardManager) initialize() error {
	m.initOnce.Do(func() {
		kubeconfig := m.kubeconfig
		if kubeconfig == "" {
			kubeconfig = os.Getenv("KUBECONFIG")
			if kubeconfig == "" {
				home, _ := os.UserHomeDir()
				kubeconfig = home + "/.kube/config"
			}
		}

		config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			m.initErr = fmt.Errorf("failed to build config: %w", err)
			return
		}

		clientset, err := kubernetes.NewForConfig(config)
		if err != nil {
			m.initErr = fmt.Errorf("failed to create clientset: %w", err)
			return
		}

		m.restConfig = config
		m.clientset = clientset
	})
	return m.initErr
}

// StartPortForward starts a new port-forward to a pod
func (m *PortForwardManager) StartPortForward(podName, namespace string, localPort, remotePort int, releaseName string) (*PortForward, error) {
	if err := m.initialize(); err != nil {
		return nil, err
	}

	m.mu.Lock()

	// Generate unique ID
	id := fmt.Sprintf("pf-%d", m.nextID)
	m.nextID++

	// If localPort is 0, we'll let the system choose
	if localPort == 0 {
		localPort = m.findAvailablePort()
	}

	pf := &PortForward{
		ID:           id,
		PodName:      podName,
		PodNamespace: namespace,
		LocalPort:    localPort,
		RemotePort:   remotePort,
		ReleaseName:  releaseName,
		Status:       "starting",
		StartedAt:    time.Now(),
		stopChan:     make(chan struct{}, 1),
		readyChan:    make(chan struct{}),
	}

	m.forwards[id] = pf
	m.mu.Unlock()

	// Start the port-forward in a goroutine
	go m.runPortForward(pf)

	// Wait for ready or timeout
	select {
	case <-pf.readyChan:
		// Port forward is ready
	case <-time.After(10 * time.Second):
		pf.setError("error", "timeout waiting for port-forward to start")
	}

	return pf, nil
}

// findAvailablePort finds an available local port starting from 8080
func (m *PortForwardManager) findAvailablePort() int {
	basePort := 8080
	usedPorts := make(map[int]bool)

	for _, pf := range m.forwards {
		if pf.GetStatus() == "running" {
			usedPorts[pf.LocalPort] = true
		}
	}

	for port := basePort; port < 65535; port++ {
		if !usedPorts[port] {
			return port
		}
	}
	return basePort
}

// runPortForward runs the actual port-forward
func (m *PortForwardManager) runPortForward(pf *PortForward) {
	ctx, cancel := context.WithCancel(context.Background())
	pf.cancelFunc = cancel

	// Build the port-forward request URL
	path := fmt.Sprintf("/api/v1/namespaces/%s/pods/%s/portforward", pf.PodNamespace, pf.PodName)
	hostURL := strings.TrimRight(m.restConfig.Host, "/") + path

	// Create the SPDY transport
	transport, upgrader, err := spdy.RoundTripperFor(m.restConfig)
	if err != nil {
		pf.setError("error", fmt.Sprintf("failed to create transport: %v", err))
		close(pf.readyChan)
		return
	}

	// Parse the hostURL to create the request for the dialer
	req, err := http.NewRequest(http.MethodPost, hostURL, nil)
	if err != nil {
		pf.setError("error", fmt.Sprintf("failed to create request: %v", err))
		close(pf.readyChan)
		return
	}

	// Create the SPDY dialer with the proper request URL
	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: transport}, http.MethodPost, req.URL)

	// Ports to forward
	ports := []string{fmt.Sprintf("%d:%d", pf.LocalPort, pf.RemotePort)}

	// Create the port-forwarder
	fw, err := portforward.New(dialer, ports, pf.stopChan, pf.readyChan, io.Discard, io.Discard)
	if err != nil {
		pf.setError("error", fmt.Sprintf("failed to create port-forwarder: %v", err))
		close(pf.readyChan)
		return
	}

	// Run the port-forward
	go func() {
		<-ctx.Done()
		close(pf.stopChan)
	}()

	pf.setStatus("running")

	if err := fw.ForwardPorts(); err != nil {
		if pf.GetStatus() != "stopped" {
			pf.setError("error", fmt.Sprintf("port-forward failed: %v", err))
		}
	}
}

// StopPortForward stops a port-forward by ID
func (m *PortForwardManager) StopPortForward(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	pf, ok := m.forwards[id]
	if !ok {
		return fmt.Errorf("port-forward %s not found", id)
	}

	pf.setStatus("stopped")
	if pf.cancelFunc != nil {
		pf.cancelFunc()
	}

	delete(m.forwards, id)
	return nil
}

// StopAll stops all active port-forwards
func (m *PortForwardManager) StopAll() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for id, pf := range m.forwards {
		pf.setStatus("stopped")
		if pf.cancelFunc != nil {
			pf.cancelFunc()
		}
		delete(m.forwards, id)
	}
}

// ListPortForwards returns all active port-forwards
func (m *PortForwardManager) ListPortForwards() []*PortForward {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*PortForward, 0, len(m.forwards))
	for _, pf := range m.forwards {
		result = append(result, pf)
	}
	return result
}

// ListPortForwardsByRelease returns port-forwards for a specific release
func (m *PortForwardManager) ListPortForwardsByRelease(releaseName, namespace string) []*PortForward {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*PortForward, 0)
	for _, pf := range m.forwards {
		if pf.ReleaseName == releaseName && pf.PodNamespace == namespace {
			result = append(result, pf)
		}
	}
	return result
}

// GetPortForward returns a specific port-forward by ID
func (m *PortForwardManager) GetPortForward(id string) (*PortForward, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	pf, ok := m.forwards[id]
	return pf, ok
}

// GetServicePorts returns the ports exposed by a service
func (c *Client) GetServicePorts(serviceName, namespace string) ([]ServicePort, error) {
	clientset, _, err := c.getClientset()
	if err != nil {
		return nil, err
	}

	svc, err := clientset.CoreV1().Services(namespace).Get(context.Background(), serviceName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get service: %w", err)
	}

	ports := make([]ServicePort, 0, len(svc.Spec.Ports))
	for _, p := range svc.Spec.Ports {
		ports = append(ports, ServicePort{
			Name:       p.Name,
			Port:       int(p.Port),
			TargetPort: p.TargetPort.IntValue(),
			Protocol:   string(p.Protocol),
		})
	}

	return ports, nil
}

// ServicePort represents a port exposed by a service
type ServicePort struct {
	Name       string
	Port       int
	TargetPort int
	Protocol   string
}

// GetPodPorts returns the ports exposed by containers in a pod
func (c *Client) GetPodPorts(podName, namespace string) ([]PodPort, error) {
	clientset, _, err := c.getClientset()
	if err != nil {
		return nil, err
	}

	pod, err := clientset.CoreV1().Pods(namespace).Get(context.Background(), podName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get pod: %w", err)
	}

	ports := make([]PodPort, 0)
	for _, c := range pod.Spec.Containers {
		for _, p := range c.Ports {
			ports = append(ports, PodPort{
				ContainerName: c.Name,
				Name:          p.Name,
				ContainerPort: int(p.ContainerPort),
				Protocol:      string(p.Protocol),
			})
		}
	}

	return ports, nil
}

// PodPort represents a port exposed by a container in a pod
type PodPort struct {
	ContainerName string
	Name          string
	ContainerPort int
	Protocol      string
}

// ParsePortSpec parses a port specification like "8080:80" or "80"
func ParsePortSpec(spec string) (localPort, remotePort int, err error) {
	parts := strings.Split(spec, ":")
	switch len(parts) {
	case 1:
		// Just remote port, local port is the same
		port, err := strconv.Atoi(parts[0])
		if err != nil {
			return 0, 0, fmt.Errorf("invalid port: %s", parts[0])
		}
		return port, port, nil
	case 2:
		// local:remote
		local, err := strconv.Atoi(parts[0])
		if err != nil {
			return 0, 0, fmt.Errorf("invalid local port: %s", parts[0])
		}
		remote, err := strconv.Atoi(parts[1])
		if err != nil {
			return 0, 0, fmt.Errorf("invalid remote port: %s", parts[1])
		}
		return local, remote, nil
	default:
		return 0, 0, fmt.Errorf("invalid port specification: %s", spec)
	}
}

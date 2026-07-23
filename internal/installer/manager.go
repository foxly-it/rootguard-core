package installer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	StateNotInstalled = "not_installed"
	StateDeploying    = "deploying"
	StateInstalled    = "installed"
	StateFailed       = "failed"
)

var (
	ErrInvalidConfig = errors.New("invalid installation configuration")
	ErrDeploying     = errors.New("installation is already running")
)

type Config struct {
	DNSBindAddress string `json:"dns_bind_address"`
	DNSPort        int    `json:"dns_port"`
}

type Check struct {
	ID      string `json:"id"`
	OK      bool   `json:"ok"`
	Message string `json:"message"`
}

type Preflight struct {
	Ready  bool    `json:"ready"`
	Config Config  `json:"config"`
	Checks []Check `json:"checks"`
}

type Step struct {
	ID      string `json:"id"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

type Status struct {
	State     string    `json:"state"`
	Config    *Config   `json:"config,omitempty"`
	Steps     []Step    `json:"steps"`
	Error     string    `json:"error,omitempty"`
	UpdatedAt time.Time `json:"updated_at"`
}

type CommandRunner func(context.Context, ...string) ([]byte, error)
type BootstrapFunc func(context.Context) error

type Options struct {
	DataDir        string
	CoreContainer  string
	UnboundImage   string
	AdGuardImage   string
	DNSNetworkCIDR string
	Run            CommandRunner
	Bootstrap      BootstrapFunc
}

type Manager struct {
	mu             sync.RWMutex
	status         Status
	dataDir        string
	coreContainer  string
	unboundImage   string
	adGuardImage   string
	dnsNetworkCIDR string
	run            CommandRunner
	bootstrap      BootstrapFunc
}

func NewManager(options Options) *Manager {
	if options.Run == nil {
		options.Run = runDocker
	}
	if options.Bootstrap == nil {
		options.Bootstrap = func(context.Context) error { return nil }
	}
	manager := &Manager{
		dataDir:        options.DataDir,
		coreContainer:  options.CoreContainer,
		unboundImage:   options.UnboundImage,
		adGuardImage:   options.AdGuardImage,
		dnsNetworkCIDR: options.DNSNetworkCIDR,
		run:            options.Run,
		bootstrap:      options.Bootstrap,
		status: Status{
			State:     StateNotInstalled,
			Steps:     []Step{},
			UpdatedAt: time.Now().UTC(),
		},
	}
	manager.load()
	if manager.status.State == StateDeploying {
		manager.status.State = StateFailed
		manager.status.Error = "The previous deployment was interrupted. It can be started again safely."
		manager.status.UpdatedAt = time.Now().UTC()
		_ = manager.persist()
	}
	return manager
}

func (m *Manager) Status() Status {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return cloneStatus(m.status)
}

// Reconcile reconnects the long-lived controller after it was recreated by an
// update. Docker preserves a manual network attachment on restart, but not when
// Compose replaces the controller container.
func (m *Manager) Reconcile(ctx context.Context) error {
	status := m.Status()
	if status.State != StateInstalled {
		return nil
	}
	output, err := m.run(ctx, "network", "connect", "rootguard-dns", m.coreContainer)
	if err == nil || strings.Contains(strings.ToLower(string(output)), "already exists") {
		return nil
	}
	return fmt.Errorf("reconnect RootGuard controller to DNS network: %w: %s", err, strings.TrimSpace(string(output)))
}

func (m *Manager) Preflight(ctx context.Context, config Config) Preflight {
	config = normalizeConfig(config)
	checks := validateConfig(config)

	if _, err := m.run(ctx, "version", "--format", "{{.Server.Version}}"); err != nil {
		checks = append(checks, Check{
			ID: "docker", OK: false,
			Message: "Docker Engine is not reachable through the RootGuard controller.",
		})
	} else {
		checks = append(checks, Check{
			ID: "docker", OK: true,
			Message: "Docker Engine is reachable.",
		})
	}

	if _, err := m.run(ctx, "compose", "version", "--short"); err != nil {
		checks = append(checks, Check{
			ID: "compose", OK: false,
			Message: "The Docker Compose plugin is not available in the controller.",
		})
	} else {
		checks = append(checks, Check{
			ID: "compose", OK: true,
			Message: "Docker Compose is available.",
		})
	}

	ready := true
	for _, check := range checks {
		if !check.OK {
			ready = false
		}
	}
	return Preflight{Ready: ready, Config: config, Checks: checks}
}

func (m *Manager) Start(ctx context.Context, config Config) (Status, error) {
	report := m.Preflight(ctx, config)
	if !report.Ready {
		return m.Status(), ErrInvalidConfig
	}

	m.mu.Lock()
	if m.status.State == StateDeploying {
		m.mu.Unlock()
		return Status{}, ErrDeploying
	}
	m.status = Status{
		State:  StateDeploying,
		Config: &report.Config,
		Steps: []Step{
			{ID: "prepare", Status: "pending", Message: "Preparing the managed stack"},
			{ID: "pull", Status: "pending", Message: "Downloading configured service images"},
			{ID: "start", Status: "pending", Message: "Starting Unbound and AdGuard Home"},
			{ID: "connect", Status: "pending", Message: "Connecting the controller"},
			{ID: "bootstrap", Status: "pending", Message: "Configuring the protected DNS chain"},
		},
		UpdatedAt: time.Now().UTC(),
	}
	if err := m.persistLocked(); err != nil {
		m.status.State = StateFailed
		m.status.Error = fmt.Sprintf("persist installation state: %v", err)
		m.status.UpdatedAt = time.Now().UTC()
		status := cloneStatus(m.status)
		m.mu.Unlock()
		return status, fmt.Errorf("persist installation state: %w", err)
	}
	status := cloneStatus(m.status)
	m.mu.Unlock()

	go m.deploy(report.Config)
	return status, nil
}

func (m *Manager) deploy(config Config) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	if err := m.setStep("prepare", "running", "Writing the versioned RootGuard stack definition"); err != nil {
		m.fail(err)
		return
	}
	composePath, err := m.writeCompose(config)
	if err != nil {
		m.fail(err)
		return
	}
	_ = m.setStep("prepare", "done", "Managed stack definition is ready")

	_ = m.setStep("pull", "running", "Downloading configured service images")
	if _, err := m.run(ctx, "compose", "--project-name", "rootguard-dns", "-f", composePath, "pull"); err != nil {
		m.fail(fmt.Errorf("pull RootGuard service images: %w", err))
		return
	}
	_ = m.setStep("pull", "done", "Service images are available")

	_ = m.setStep("start", "running", "Starting Unbound and AdGuard Home")
	if _, err := m.run(ctx, "compose", "--project-name", "rootguard-dns", "-f", composePath, "up", "-d"); err != nil {
		m.fail(fmt.Errorf("start RootGuard DNS stack: %w", err))
		return
	}
	_ = m.setStep("start", "done", "DNS containers were created")

	_ = m.setStep("connect", "running", "Connecting the controller to the private DNS network")
	if output, err := m.run(ctx, "network", "connect", "rootguard-dns", m.coreContainer); err != nil &&
		!strings.Contains(strings.ToLower(string(output)), "already exists") {
		m.fail(fmt.Errorf("connect RootGuard controller to DNS network: %w: %s", err, strings.TrimSpace(string(output))))
		return
	}
	_ = m.setStep("connect", "done", "Controller is connected to the private DNS network")

	_ = m.setStep("bootstrap", "running", "Waiting for Unbound and securing AdGuard Home")
	if err := m.waitForUnbound(ctx); err != nil {
		m.fail(err)
		return
	}
	if err := m.bootstrap(ctx); err != nil {
		m.fail(fmt.Errorf("bootstrap AdGuard Home: %w", err))
		return
	}
	_ = m.setStep("bootstrap", "done", "AdGuard Home forwards exclusively to Unbound")

	m.mu.Lock()
	m.status.State = StateInstalled
	m.status.Error = ""
	m.status.UpdatedAt = time.Now().UTC()
	_ = m.persistLocked()
	m.mu.Unlock()
}

func (m *Manager) waitForUnbound(ctx context.Context) error {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		output, err := m.run(ctx, "inspect", "--format", "{{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}}", "rootguard-unbound")
		if err == nil && strings.TrimSpace(string(output)) == "healthy" {
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("wait for Unbound health: %w", ctx.Err())
		case <-ticker.C:
		}
	}
}

func (m *Manager) writeCompose(config Config) (string, error) {
	if err := os.MkdirAll(m.dataDir, 0700); err != nil {
		return "", fmt.Errorf("create installation data directory: %w", err)
	}
	content, err := renderCompose(config, m.unboundImage, m.adGuardImage, m.dnsNetworkCIDR)
	if err != nil {
		return "", err
	}
	path := filepath.Join(m.dataDir, "compose.yaml")
	temp := path + ".tmp"
	if err := os.WriteFile(temp, []byte(content), 0600); err != nil {
		return "", fmt.Errorf("write temporary stack definition: %w", err)
	}
	if err := os.Rename(temp, path); err != nil {
		return "", fmt.Errorf("activate stack definition: %w", err)
	}
	return path, nil
}

func renderCompose(config Config, unboundImage, adGuardImage, networkCIDR string) (string, error) {
	resolverAddress, err := resolverAddress(networkCIDR)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf(`name: rootguard-dns

services:
  unbound:
    image: %s
    container_name: rootguard-unbound
    restart: unless-stopped
    read_only: true
    cap_drop: [ALL]
    security_opt:
      - no-new-privileges:true
    labels:
      io.rootguard.managed: "true"
      io.rootguard.component: "unbound"
    volumes:
      - rootguard-unbound-config:/etc/unbound/unbound.d
      - rootguard-unbound-state:/var/lib/unbound
    networks:
      dns:
        ipv4_address: %s

  adguard:
    image: %s
    container_name: rootguard-adguard
    restart: unless-stopped
    depends_on:
      unbound:
        condition: service_healthy
    labels:
      io.rootguard.managed: "true"
      io.rootguard.component: "adguard"
    ports:
      - "%s:%d:53/tcp"
      - "%s:%d:53/udp"
    volumes:
      - rootguard-adguard-work:/opt/adguardhome/work
      - rootguard-adguard-config:/opt/adguardhome/conf
    networks:
      - dns

networks:
  dns:
    name: rootguard-dns
    ipam:
      config:
        - subnet: %s

volumes:
  rootguard-unbound-config:
    external: true
  rootguard-unbound-state:
    name: rootguard-unbound-state
  rootguard-adguard-work:
    name: rootguard-adguard-work
  rootguard-adguard-config:
    name: rootguard-adguard-config
`, unboundImage, resolverAddress, adGuardImage, config.DNSBindAddress, config.DNSPort,
		config.DNSBindAddress, config.DNSPort, networkCIDR), nil
}

func resolverAddress(networkCIDR string) (string, error) {
	ip, network, err := net.ParseCIDR(networkCIDR)
	if err != nil || ip.To4() == nil {
		return "", fmt.Errorf("invalid IPv4 DNS network %q", networkCIDR)
	}
	address := append(net.IP(nil), ip.To4()...)
	address[3] += 2
	if !network.Contains(address) {
		return "", fmt.Errorf("DNS network %q has no resolver address", networkCIDR)
	}
	return address.String(), nil
}

func normalizeConfig(config Config) Config {
	config.DNSBindAddress = strings.TrimSpace(config.DNSBindAddress)
	return config
}

func validateConfig(config Config) []Check {
	var checks []Check
	ip := net.ParseIP(config.DNSBindAddress)
	if ip == nil || ip.To4() == nil {
		checks = append(checks, Check{
			ID: "dns_address", OK: false,
			Message: "Enter an IPv4 address already assigned to the Docker host, or 0.0.0.0 for all addresses.",
		})
	} else {
		checks = append(checks, Check{
			ID: "dns_address", OK: true,
			Message: "The DNS bind address has a valid IPv4 format.",
		})
	}
	if config.DNSPort < 1 || config.DNSPort > 65535 {
		checks = append(checks, Check{
			ID: "dns_port", OK: false,
			Message: "The DNS port must be between 1 and 65535.",
		})
	} else {
		checks = append(checks, Check{
			ID: "dns_port", OK: true,
			Message: "The DNS port is valid. Docker performs the final host availability check during deployment.",
		})
	}
	return checks
}

func (m *Manager) setStep(id, status, message string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for index := range m.status.Steps {
		if m.status.Steps[index].ID == id {
			m.status.Steps[index].Status = status
			m.status.Steps[index].Message = message
			m.status.UpdatedAt = time.Now().UTC()
			return m.persistLocked()
		}
	}
	return fmt.Errorf("unknown installation step %q", id)
}

func (m *Manager) fail(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.status.State = StateFailed
	m.status.Error = err.Error()
	m.status.UpdatedAt = time.Now().UTC()
	for index := range m.status.Steps {
		if m.status.Steps[index].Status == "running" {
			m.status.Steps[index].Status = "failed"
		}
	}
	_ = m.persistLocked()
}

func (m *Manager) load() {
	data, err := os.ReadFile(filepath.Join(m.dataDir, "status.json"))
	if err != nil {
		return
	}
	var status Status
	if json.Unmarshal(data, &status) == nil && status.State != "" {
		m.status = status
	}
}

func (m *Manager) persist() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.persistLocked()
}

func (m *Manager) persistLocked() error {
	if err := os.MkdirAll(m.dataDir, 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(m.status, "", "  ")
	if err != nil {
		return err
	}
	path := filepath.Join(m.dataDir, "status.json")
	temp := path + ".tmp"
	if err := os.WriteFile(temp, data, 0600); err != nil {
		return err
	}
	return os.Rename(temp, path)
}

func cloneStatus(status Status) Status {
	clone := status
	if status.Config != nil {
		config := *status.Config
		clone.Config = &config
	}
	clone.Steps = make([]Step, len(status.Steps))
	copy(clone.Steps, status.Steps)
	return clone
}

func runDocker(ctx context.Context, arguments ...string) ([]byte, error) {
	command := exec.CommandContext(ctx, "docker", arguments...)
	output, err := command.CombinedOutput()
	if err != nil {
		return output, fmt.Errorf("docker %s: %w: %s", strings.Join(arguments, " "), err, strings.TrimSpace(string(output)))
	}
	return output, nil
}

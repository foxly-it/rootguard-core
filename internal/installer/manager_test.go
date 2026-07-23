package installer

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestPreflightRejectsInvalidNetworkValues(t *testing.T) {
	manager := NewManager(Options{
		DataDir: t.TempDir(),
		Run: func(_ context.Context, _ ...string) ([]byte, error) {
			return []byte("ok"), nil
		},
	})

	report := manager.Preflight(context.Background(), Config{
		DNSBindAddress: "not-an-ip",
		DNSPort:        70000,
	})

	if report.Ready {
		t.Fatal("expected preflight to reject invalid settings")
	}
	if len(report.Checks) != 4 {
		t.Fatalf("expected four checks, got %d", len(report.Checks))
	}
}

func TestInitialStatusUsesEmptyStepsArray(t *testing.T) {
	manager := NewManager(Options{DataDir: t.TempDir()})
	status := manager.Status()
	if status.Steps == nil {
		t.Fatal("expected an empty steps array, got nil")
	}
}

func TestPreflightRequiresDockerAndCompose(t *testing.T) {
	manager := NewManager(Options{
		DataDir: t.TempDir(),
		Run: func(_ context.Context, arguments ...string) ([]byte, error) {
			if arguments[0] == "compose" {
				return nil, errors.New("missing")
			}
			return []byte("ok"), nil
		},
	})

	report := manager.Preflight(context.Background(), Config{
		DNSBindAddress: "192.168.1.2",
		DNSPort:        53,
	})

	if report.Ready {
		t.Fatal("expected missing compose plugin to fail preflight")
	}
}

func TestRenderedComposeKeepsAdministrationPrivate(t *testing.T) {
	content, err := renderCompose(
		Config{DNSBindAddress: "192.168.1.2", DNSPort: 53},
		"ghcr.io/foxly-it/rootguard-unbound:latest",
		"adguard/adguardhome:v0.107.78",
		"172.29.53.0/24",
	)
	if err != nil {
		t.Fatal(err)
	}

	for _, expected := range []string{
		`"192.168.1.2:53:53/tcp"`,
		`"192.168.1.2:53:53/udp"`,
		`io.rootguard.managed: "true"`,
		`external: true`,
	} {
		if !strings.Contains(content, expected) {
			t.Fatalf("rendered compose is missing %q", expected)
		}
	}
	if strings.Contains(content, "3000:3000") || strings.Contains(content, "80:80") {
		t.Fatal("AdGuard administration must not be published")
	}
}

func TestDeploymentPersistsCompletedState(t *testing.T) {
	dataDir := t.TempDir()
	var mu sync.Mutex
	var commands []string
	manager := NewManager(Options{
		DataDir:        dataDir,
		CoreContainer:  "rootguard-core",
		UnboundImage:   "rootguard-unbound:test",
		AdGuardImage:   "adguard:test",
		DNSNetworkCIDR: "172.29.53.0/24",
		Run: func(_ context.Context, arguments ...string) ([]byte, error) {
			mu.Lock()
			commands = append(commands, strings.Join(arguments, " "))
			mu.Unlock()
			if arguments[0] == "inspect" {
				return []byte("healthy\n"), nil
			}
			return []byte("ok"), nil
		},
	})

	_, err := manager.Start(context.Background(), Config{
		DNSBindAddress: "192.168.1.2",
		DNSPort:        53,
	})
	if err != nil {
		t.Fatal(err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for manager.Status().State == StateDeploying && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if status := manager.Status(); status.State != StateInstalled {
		t.Fatalf("expected installed state, got %#v", status)
	}

	reloaded := NewManager(Options{DataDir: dataDir})
	if status := reloaded.Status(); status.State != StateInstalled {
		t.Fatalf("expected persisted installed state, got %#v", status)
	}

	mu.Lock()
	allCommands := strings.Join(commands, "\n")
	mu.Unlock()
	for _, expected := range []string{"compose version", "compose --project-name rootguard-dns", "network connect", "inspect --format"} {
		if !strings.Contains(allCommands, expected) {
			t.Fatalf("expected command containing %q in:\n%s", expected, allCommands)
		}
	}
}

func TestRenderedComposeRejectsInvalidInternalNetwork(t *testing.T) {
	_, err := renderCompose(
		Config{DNSBindAddress: "0.0.0.0", DNSPort: 53},
		"rootguard-unbound:test",
		"adguard:test",
		"not-a-network",
	)
	if err == nil {
		t.Fatal("expected invalid internal network to be rejected")
	}
}

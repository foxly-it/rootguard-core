package unbound

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestDefaultSettingsRender(t *testing.T) {
	config, err := DefaultSettings().Render()
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		"# Privacy:",
		"qname-minimisation: yes",
		"# Performance:",
		"prefetch: yes",
		"# Availability:",
		"serve-expired: yes",
		"cache-max-ttl: 86400",
		"num-threads: 2",
	} {
		if !strings.Contains(string(config), expected) {
			t.Errorf("rendered config does not contain %q", expected)
		}
	}
}

func TestActiveConfigurationReadsRunningContainerFiles(t *testing.T) {
	manager := NewManager(t.TempDir(), "/etc/unbound/unbound.d", "rootguard-unbound")
	manager.now = func() time.Time { return time.Unix(123, 0) }
	manager.run = func(_ context.Context, name string, arguments ...string) ([]byte, error) {
		if name != "docker" || len(arguments) != 4 || arguments[0] != "exec" || arguments[2] != "cat" {
			t.Fatalf("unexpected command: %s %v", name, arguments)
		}
		switch arguments[3] {
		case "/etc/unbound/unbound.conf":
			return []byte("include: /etc/unbound/unbound.d/*.conf\n"), nil
		case "/etc/unbound/unbound.d/50-rootguard.conf":
			return []byte("server:\n    prefetch: yes\n"), nil
		default:
			t.Fatalf("unexpected path: %s", arguments[3])
			return nil, nil
		}
	}

	active, err := manager.ActiveConfiguration(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(active.BaseConfig, "include:") || !strings.Contains(active.ManagedConfig, "prefetch: yes") {
		t.Fatalf("unexpected active configuration: %#v", active)
	}
	if active.CustomConfig != "" || !active.CheckedAt.Equal(time.Unix(123, 0)) {
		t.Fatalf("unexpected active metadata: %#v", active)
	}
}

func TestSettingsValidation(t *testing.T) {
	settings := DefaultSettings()
	settings.Threads = 0
	if err := settings.Validate(); !errors.Is(err, ErrInvalidSettings) {
		t.Fatalf("expected ErrInvalidSettings, got %v", err)
	}
}

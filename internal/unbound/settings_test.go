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

func TestForwardZonesRenderCanonicalOrderedTargets(t *testing.T) {
	settings := DefaultSettings()
	settings.ForwardZones = []ForwardZone{
		{
			Name:         "corp.example.",
			Servers:      []string{"192.0.2.53", "2001:db8::53"},
			ForwardFirst: true,
		},
	}
	config, err := settings.Render()
	if err != nil {
		t.Fatal(err)
	}
	rendered := string(config)
	for _, expected := range []string{
		"# Conditional forwarding:",
		"forward-zone:",
		`name: "corp.example."`,
		"forward-addr: 192.0.2.53",
		"forward-addr: 2001:db8::53",
		"forward-first: yes",
	} {
		if !strings.Contains(rendered, expected) {
			t.Errorf("rendered forwarding config does not contain %q", expected)
		}
	}
	if strings.Index(rendered, "192.0.2.53") > strings.Index(rendered, "2001:db8::53") {
		t.Fatal("forward target order was not preserved")
	}
}

func TestForwardZoneValidationRejectsUnsafeInputs(t *testing.T) {
	tests := []struct {
		name string
		zone ForwardZone
	}{
		{name: "root zone", zone: ForwardZone{Name: ".", Servers: []string{"192.0.2.53"}}},
		{name: "non canonical name", zone: ForwardZone{Name: "Corp.Example", Servers: []string{"192.0.2.53"}}},
		{name: "invalid label", zone: ForwardZone{Name: "-corp.example.", Servers: []string{"192.0.2.53"}}},
		{name: "missing target", zone: ForwardZone{Name: "corp.example."}},
		{name: "non canonical IPv6", zone: ForwardZone{Name: "corp.example.", Servers: []string{"2001:0db8::53"}}},
		{name: "loopback target", zone: ForwardZone{Name: "corp.example.", Servers: []string{"127.0.0.1"}}},
		{name: "RootGuard network target", zone: ForwardZone{Name: "corp.example.", Servers: []string{"172.29.53.3"}}},
		{name: "mapped RootGuard target", zone: ForwardZone{Name: "corp.example.", Servers: []string{"::ffff:172.29.53.3"}}},
		{name: "link-local target", zone: ForwardZone{Name: "corp.example.", Servers: []string{"fe80::53"}}},
		{name: "duplicate target", zone: ForwardZone{Name: "corp.example.", Servers: []string{"192.0.2.53", "192.0.2.53"}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			settings := DefaultSettings()
			settings.ForwardZones = []ForwardZone{test.zone}
			if err := settings.Validate(); !errors.Is(err, ErrInvalidSettings) {
				t.Fatalf("expected ErrInvalidSettings, got %v", err)
			}
		})
	}
}

func TestForwardZoneValidationRejectsDuplicateZonesAndLimits(t *testing.T) {
	settings := DefaultSettings()
	settings.ForwardZones = []ForwardZone{
		{Name: "corp.example.", Servers: []string{"192.0.2.53"}},
		{Name: "corp.example.", Servers: []string{"192.0.2.54"}},
	}
	if err := settings.Validate(); !errors.Is(err, ErrInvalidSettings) {
		t.Fatalf("expected duplicate zone rejection, got %v", err)
	}

	settings.ForwardZones = make([]ForwardZone, maxForwardZones+1)
	if err := settings.Validate(); !errors.Is(err, ErrInvalidSettings) {
		t.Fatalf("expected zone limit rejection, got %v", err)
	}
}

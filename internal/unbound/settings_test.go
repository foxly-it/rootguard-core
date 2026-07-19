package unbound

import (
	"errors"
	"strings"
	"testing"
)

func TestDefaultSettingsRender(t *testing.T) {
	config, err := DefaultSettings().Render()
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		"qname-minimisation: yes",
		"prefetch: yes",
		"serve-expired: yes",
		"cache-max-ttl: 86400",
		"num-threads: 2",
	} {
		if !strings.Contains(string(config), expected) {
			t.Errorf("rendered config does not contain %q", expected)
		}
	}
}

func TestSettingsValidation(t *testing.T) {
	settings := DefaultSettings()
	settings.Threads = 0
	if err := settings.Validate(); !errors.Is(err, ErrInvalidSettings) {
		t.Fatalf("expected ErrInvalidSettings, got %v", err)
	}
}

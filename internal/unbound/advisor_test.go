package unbound

import (
	"errors"
	"testing"
)

func TestPresetsAreValidAndUnique(t *testing.T) {
	seen := map[string]bool{}
	for _, preset := range Presets() {
		if preset.ID == "" || seen[preset.ID] {
			t.Fatalf("invalid or duplicate preset id %q", preset.ID)
		}
		seen[preset.ID] = true
		if err := preset.Settings.Validate(); err != nil {
			t.Fatalf("preset %s is invalid: %v", preset.ID, err)
		}
	}
	if len(seen) != 4 {
		t.Fatalf("expected four presets, got %d", len(seen))
	}
}

func TestAdvisorHighlightsPrivacyAvailabilityAndResourceRisks(t *testing.T) {
	settings := DefaultSettings()
	settings.QnameMinimisation = false
	settings.ServeExpired = false
	settings.CacheMinTTL = 600
	settings.CacheMaxTTL = 604800
	settings.Threads = 16
	advice, err := Advise(settings)
	if err != nil {
		t.Fatal(err)
	}
	if advice.Status != "review" {
		t.Fatalf("expected review status, got %s", advice.Status)
	}
	wanted := map[string]bool{"enable-qname-minimisation": false, "enable-serve-expired": false, "lower-cache-min-ttl": false, "lower-cache-max-ttl": false, "review-threads": false}
	for _, recommendation := range advice.Recommendations {
		if _, ok := wanted[recommendation.ID]; ok {
			wanted[recommendation.ID] = true
		}
	}
	for id, found := range wanted {
		if !found {
			t.Errorf("missing recommendation %s", id)
		}
	}
}

func TestAdvisorAcceptsBalancedDefaults(t *testing.T) {
	advice, err := Advise(DefaultSettings())
	if err != nil {
		t.Fatal(err)
	}
	if advice.Status != "optimized" || len(advice.Recommendations) != 1 || advice.Recommendations[0].Severity != "success" {
		t.Fatalf("unexpected balanced advice: %+v", advice)
	}
}

func TestAdvisorRejectsInvalidSettings(t *testing.T) {
	settings := DefaultSettings()
	settings.Threads = 0
	if _, err := Advise(settings); !errors.Is(err, ErrInvalidSettings) {
		t.Fatalf("expected invalid settings, got %v", err)
	}
}

package unbound

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCustomPreviewValidatesWithoutChangingActiveFile(t *testing.T) {
	manager := newTestManager(t)
	content := "server:\n    hide-identity: yes\n"
	preview, err := manager.PreviewCustom(context.Background(), content)
	if err != nil {
		t.Fatal(err)
	}
	if !preview.Changed || preview.Validation == "" || len(preview.Advice) == 0 {
		t.Fatalf("unexpected preview: %+v", preview)
	}
	if _, err := os.Stat(filepath.Join(manager.hostConfigDir, "90-rootguard-custom.conf")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("preview changed active config: %v", err)
	}
}

func TestCustomConfigIsActivatedVersionedAndRestored(t *testing.T) {
	manager := newTestManager(t)
	first := "server:\n    hide-identity: yes\n"
	second := "server:\n    hide-version: yes\n"
	if _, err := manager.ApplyCustom(context.Background(), first); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.ApplyCustom(context.Background(), second); err != nil {
		t.Fatal(err)
	}
	history, err := manager.History()
	if err != nil {
		t.Fatal(err)
	}
	var firstID string
	for _, entry := range history {
		if entry.CustomConfig == first {
			firstID = entry.ID
		}
	}
	if firstID == "" {
		t.Fatalf("first custom version not recorded: %+v", history)
	}
	if _, err := manager.Restore(context.Background(), firstID); err != nil {
		t.Fatal(err)
	}
	document, err := manager.LoadCustom()
	if err != nil {
		t.Fatal(err)
	}
	if document.Content != first {
		t.Fatalf("unexpected restored custom config: %q", document.Content)
	}
}

func TestCustomConfigRejectsManagedAndDangerousDirectives(t *testing.T) {
	for _, content := range []string{
		"server:\n    num-threads: 8\n",
		"include: \"/etc/passwd\"\n",
		"server:\n    interface: 0.0.0.0\n",
	} {
		if _, err := normalizeCustom(content); !errors.Is(err, ErrInvalidCustomConfig) {
			t.Fatalf("expected policy rejection for %q, got %v", content, err)
		}
	}
}

func TestCustomConfigRestoresFilesWhenEffectiveCheckFails(t *testing.T) {
	manager := newTestManager(t)
	initial := "server:\n    hide-identity: yes\n"
	if _, err := manager.ApplyCustom(context.Background(), initial); err != nil {
		t.Fatal(err)
	}
	manager.run = func(_ context.Context, _ string, args ...string) ([]byte, error) {
		if strings.Contains(strings.Join(args, " "), "/etc/unbound/unbound.conf") {
			return []byte("duplicate or invalid option"), errors.New("exit 1")
		}
		return []byte("OK"), nil
	}
	if _, err := manager.ApplyCustom(context.Background(), "server:\n    hide-version: yes\n"); err == nil {
		t.Fatal("expected effective configuration failure")
	}
	document, err := manager.LoadCustom()
	if err != nil {
		t.Fatal(err)
	}
	if document.Content != initial {
		t.Fatalf("failed validation left candidate active: %q", document.Content)
	}
}

func TestCustomAdvisorFlagsHardeningAndForwarding(t *testing.T) {
	advice := adviseCustom("server:\n    hide-version: no\nforward-zone:\n    name: \".\"\n    forward-addr: 1.1.1.1\n")
	foundWarning, foundForwarder := false, false
	for _, item := range advice {
		foundWarning = foundWarning || item.Severity == "warning"
		foundForwarder = foundForwarder || strings.HasPrefix(item.ID, "forwarding-")
	}
	if !foundWarning || !foundForwarder {
		t.Fatalf("expected hardening and forwarding advice: %+v", advice)
	}
}

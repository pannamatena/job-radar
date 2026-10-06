package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	jobradar "github.com/pannamatena/job-radar"
	"github.com/pannamatena/job-radar/internal/config"
)

func TestInitCreatesConfigAndAcceptsNotice(t *testing.T) {
	home := filepath.Join(t.TempDir(), "private")

	if err := cmdInit(context.Background(), []string{"--home", home, "--yes"}); err != nil {
		t.Fatalf("cmdInit: %v", err)
	}

	// The example files are present.
	for _, f := range []string{"config.yaml", "profile.md", "rubric.md"} {
		if _, err := os.Stat(filepath.Join(home, f)); err != nil {
			t.Errorf("expected %s to be created: %v", f, err)
		}
	}

	// The created config loads and records notice acceptance.
	cfg, err := config.Load(home)
	if err != nil {
		t.Fatalf("loading created config: %v", err)
	}
	if !cfg.Notice.Accepted {
		t.Error("notice.accepted should be true after init")
	}
	if cfg.Notice.Version != jobradar.NoticeVersion {
		t.Errorf("notice.version = %q, want %q", cfg.Notice.Version, jobradar.NoticeVersion)
	}
}

func TestInitRefusesToOverwriteWithoutForce(t *testing.T) {
	home := filepath.Join(t.TempDir(), "private")
	if err := cmdInit(context.Background(), []string{"--home", home, "--yes"}); err != nil {
		t.Fatalf("first init: %v", err)
	}
	err := cmdInit(context.Background(), []string{"--home", home, "--yes"})
	if err == nil {
		t.Fatal("expected init to refuse overwriting an existing config")
	}
	if !strings.Contains(err.Error(), "--force") {
		t.Errorf("error should mention --force, got: %v", err)
	}
}

func TestAcceptNotice(t *testing.T) {
	got := string(acceptNotice([]byte("notice:\n  accepted: false\n")))
	if !strings.Contains(got, "accepted: true") {
		t.Errorf("accepted not flipped: %q", got)
	}
	if !strings.Contains(got, jobradar.NoticeVersion) {
		t.Errorf("version not recorded: %q", got)
	}
}

func TestShortVersionExtractsSection(t *testing.T) {
	notice := "intro\n\n## The short version\n\n- point one\n- point two\n\n---\n\n## 1. Next\nmore"
	got := shortVersion(notice)
	if !strings.Contains(got, "point one") || !strings.Contains(got, "point two") {
		t.Errorf("short version missing content: %q", got)
	}
	if strings.Contains(got, "## 1. Next") {
		t.Errorf("short version bled into the next section: %q", got)
	}
}

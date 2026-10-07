package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParse_ValidConfig(t *testing.T) {
	data := []byte(`
scorer: rules
companies:
  - name: Acme
    ats: greenhouse
    slug: acme
    filters:
      departments: ["Engineering"]
  - name: Globex
    ats: none
    careers_url: "https://globex.example/careers"
location:
  home_area: "Manchester"
  counts_as_home: ["Manchester", "Greater Manchester"]
  allowed_work_models:
    hybrid: home_only
    remote_open_to: ["United Kingdom", "EMEA"]
    not_stated: keep_if_home_or_unknown
notice:
  accepted: true
  version: "2026-10-04"
`)
	cfg, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(cfg.Companies) != 2 {
		t.Fatalf("got %d companies, want 2", len(cfg.Companies))
	}
	if cfg.Companies[0].Filters.Departments[0] != "Engineering" {
		t.Errorf("department filter not parsed: %+v", cfg.Companies[0].Filters)
	}
	if cfg.Location.HomeArea != "Manchester" {
		t.Errorf("home_area = %q", cfg.Location.HomeArea)
	}
	if !cfg.Notice.Accepted {
		t.Errorf("notice.accepted should be true")
	}
}

func TestParse_DefaultsScorerToRules(t *testing.T) {
	cfg, err := Parse([]byte("companies: []\n"))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if cfg.Scorer != ScorerRules {
		t.Errorf("scorer default = %q, want %q", cfg.Scorer, ScorerRules)
	}
}

func TestParse_UnknownFieldIsRejected(t *testing.T) {
	_, err := Parse([]byte("compaines: []\n")) // deliberate typo
	if err == nil {
		t.Fatal("expected an error for an unknown field")
	}
	if !strings.Contains(err.Error(), "compaines") {
		t.Errorf("error should mention the offending field, got: %v", err)
	}
}

func TestValidate_ReportsAllProblems(t *testing.T) {
	data := []byte(`
scorer: magic
companies:
  - name: ""
    ats: greenhouse
  - name: NoSlug
    ats: lever
  - name: Bad
    ats: workday
`)
	_, err := Parse(data)
	if err == nil {
		t.Fatal("expected validation errors")
	}
	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("want *ValidationError, got %T: %v", err, err)
	}
	// Expect: bad scorer, missing name, missing slug (lever), unsupported ats.
	if len(ve.Problems) < 4 {
		t.Errorf("expected at least 4 problems, got %d:\n%v", len(ve.Problems), ve.Problems)
	}
	msg := err.Error()
	for _, want := range []string{"scorer", "needs a slug", "workday", "needs a name"} {
		if !strings.Contains(msg, want) {
			t.Errorf("error missing %q:\n%s", want, msg)
		}
	}
}

func TestLoad_MissingFileGivesFriendlyError(t *testing.T) {
	dir := t.TempDir()
	_, err := Load(dir)
	if err == nil {
		t.Fatal("expected an error for a missing config file")
	}
	if !strings.Contains(err.Error(), "job-radar init") {
		t.Errorf("error should suggest `job-radar init`, got: %v", err)
	}
}

func TestLoad_ReadsFromDir(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ConfigFileName), []byte("companies: []\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Home != dir {
		t.Errorf("Home = %q, want %q", cfg.Home, dir)
	}
}

func TestHome_RespectsEnv(t *testing.T) {
	t.Setenv(EnvHome, "/tmp/my-radar")
	if got := Home(); got != "/tmp/my-radar" {
		t.Errorf("Home() = %q, want /tmp/my-radar", got)
	}
	t.Setenv(EnvHome, "")
	if got := Home(); got != DefaultHome {
		t.Errorf("Home() default = %q, want %q", got, DefaultHome)
	}
}

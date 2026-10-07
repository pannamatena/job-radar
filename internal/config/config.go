// Package config loads and validates a user's job-radar configuration.
//
// Everything personal lives in a single config directory, chosen by the
// JOB_RADAR_HOME environment variable and defaulting to ./private (which is
// git-ignored). The directory holds config.yaml plus profile.md, rubric.md,
// eval cases and the SQLite database. Nothing here is ever committed to the
// public code repo.
//
// Validation is written for non-developers: errors say what is wrong, where,
// and often what to do about it, and all problems are reported together rather
// than one at a time.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// EnvHome is the environment variable that points at the config directory.
const EnvHome = "JOB_RADAR_HOME"

// DefaultHome is used when JOB_RADAR_HOME is unset.
const DefaultHome = "private"

// ConfigFileName is the config file inside the home directory.
const ConfigFileName = "config.yaml"

// ATS values a company entry may use.
const (
	ATSGreenhouse = "greenhouse"
	ATSLever      = "lever"
	ATSAshby      = "ashby"
	ATSNone       = "none" // no public API; covered by job-alert emails (phase 5)
)

// Scorer values.
const (
	ScorerRules = "rules"
	ScorerLLM   = "llm"
)

// Config is the whole user configuration. It will grow over later phases
// (rules scorer, LLM, email, pricing); phase 1 covers companies, location and
// the security-notice confirmation.
type Config struct {
	// Scorer selects how postings are scored: "rules" (free, default) or "llm".
	Scorer string `yaml:"scorer"`

	// Companies is the user's watch-list. It may be empty.
	Companies []Company `yaml:"companies"`

	// Location holds the user's location rules (used by the pre-filter).
	Location Location `yaml:"location"`

	// PreFilter holds the cheap title/keyword rules that drop obvious misses
	// before scoring. Empty lists fall back to sensible software-role defaults.
	PreFilter PreFilter `yaml:"prefilter"`

	// Notice records that the user has read the security-and-privacy notice.
	Notice Notice `yaml:"notice"`

	// Home is the resolved config directory this config was loaded from. It is
	// filled in by Load and is not part of the YAML file.
	Home string `yaml:"-"`
}

// Company is one entry on the watch-list.
type Company struct {
	Name       string         `yaml:"name"`
	ATS        string         `yaml:"ats"`
	Slug       string         `yaml:"slug,omitempty"`
	CareersURL string         `yaml:"careers_url,omitempty"`
	Notes      string         `yaml:"notes,omitempty"`
	Filters    CompanyFilters `yaml:"filters,omitempty"`
}

// CompanyFilters narrow a board's postings before scoring. They are applied in
// Go (not in the request URL) so they are easy to test.
type CompanyFilters struct {
	Departments         []string `yaml:"departments,omitempty"`
	OfficeIDs           []int64  `yaml:"office_ids,omitempty"`
	LocationNames       []string `yaml:"location_names,omitempty"`
	IncludeRemoteOpenTo []string `yaml:"include_remote_open_to,omitempty"`
}

// Location holds the user's location rules. The pre-filter (phase 2) applies
// them; phase 1 only loads and validates them.
type Location struct {
	HomeArea                    string            `yaml:"home_area"`
	CountsAsHome                []string          `yaml:"counts_as_home,omitempty"`
	NotHomeDespiteSoundingClose []string          `yaml:"not_home_despite_sounding_close,omitempty"`
	AllowedWorkModels           AllowedWorkModels `yaml:"allowed_work_models,omitempty"`
}

// AllowedWorkModels says which work models are acceptable and under what
// conditions. Values are small keywords interpreted by the pre-filter:
//   - Hybrid/Onsite: "home_only" (office must be in the home area) or "any".
//   - RemoteOpenTo: a remote role is kept only if open to one of these regions.
//   - NotStated: "keep_if_home_or_unknown" (never drop solely for a missing model).
type AllowedWorkModels struct {
	Hybrid       string   `yaml:"hybrid,omitempty"`
	Onsite       string   `yaml:"onsite,omitempty"`
	RemoteOpenTo []string `yaml:"remote_open_to,omitempty"`
	NotStated    string   `yaml:"not_stated,omitempty"`
}

// PreFilter configures the deterministic pre-filter that runs before scoring.
// It is deliberately cautious: the default is to keep a posting, and it only
// drops ones it is sure about (ADR-0012). A posting's age is never considered
// (ADR-0025). All matching is case-insensitive and on whole words.
type PreFilter struct {
	// TitleKeep rescues a title that would otherwise match TitleDrop. Rarely
	// needed because the default is already to keep.
	TitleKeep []string `yaml:"title_keep,omitempty"`
	// TitleDrop drops titles that are clearly not for you (e.g. junior, director).
	TitleDrop []string `yaml:"title_drop,omitempty"`
	// NotSoftware drops non-software-engineering roles by keyword, matched in the
	// title or the description (e.g. railway, chartered engineer).
	NotSoftware []string `yaml:"not_software,omitempty"`
}

// Notice records the user's acknowledgement of docs/security-and-privacy.md.
type Notice struct {
	Accepted bool   `yaml:"accepted"`
	Version  string `yaml:"version,omitempty"`
}

// Home returns the resolved config directory: $JOB_RADAR_HOME or ./private.
func Home() string {
	if h := strings.TrimSpace(os.Getenv(EnvHome)); h != "" {
		return h
	}
	return DefaultHome
}

// ConfigPath returns the path to config.yaml inside the given home directory.
func ConfigPath(home string) string { return filepath.Join(home, ConfigFileName) }

// Load reads and validates the config from the given home directory. A friendly
// error is returned if the directory or file is missing, the YAML is malformed,
// or validation fails.
func Load(home string) (*Config, error) {
	path := ConfigPath(home)
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("no config found at %s\n\nIf this is your first time, run `job-radar init` to create one, "+
				"or set %s to point at your config directory.", path, EnvHome)
		}
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}

	cfg, err := Parse(data)
	if err != nil {
		return nil, fmt.Errorf("in %s:\n%w", path, err)
	}
	cfg.Home = home
	return cfg, nil
}

// Parse decodes and validates config YAML from bytes (used by Load and tests).
// Unknown fields are rejected so typos surface as clear errors.
func Parse(data []byte) (*Config, error) {
	dec := yaml.NewDecoder(strings.NewReader(string(data)))
	dec.KnownFields(true)

	var cfg Config
	if err := dec.Decode(&cfg); err != nil {
		return nil, friendlyYAMLError(err)
	}
	cfg.applyDefaults()
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (c *Config) applyDefaults() {
	if strings.TrimSpace(c.Scorer) == "" {
		c.Scorer = ScorerRules
	}
}

// Validate collects every problem and reports them together.
func (c *Config) Validate() error {
	var problems []string

	switch c.Scorer {
	case ScorerRules, ScorerLLM:
	default:
		problems = append(problems, fmt.Sprintf("scorer: %q is not valid; use %q or %q", c.Scorer, ScorerRules, ScorerLLM))
	}

	seen := map[string]int{}
	for i, co := range c.Companies {
		where := fmt.Sprintf("companies[%d]", i)
		if strings.TrimSpace(co.Name) == "" {
			problems = append(problems, where+": every company needs a name")
		} else {
			key := strings.ToLower(strings.TrimSpace(co.Name))
			if seen[key]++; seen[key] == 2 {
				problems = append(problems, fmt.Sprintf("companies: %q is listed more than once", co.Name))
			}
			where = fmt.Sprintf("companies[%q]", co.Name)
		}

		switch co.ATS {
		case ATSGreenhouse, ATSLever, ATSAshby:
			if strings.TrimSpace(co.Slug) == "" {
				problems = append(problems, fmt.Sprintf("%s: ats %q needs a slug (the company's identifier on that job board)", where, co.ATS))
			}
		case ATSNone:
			if strings.TrimSpace(co.CareersURL) == "" {
				problems = append(problems, fmt.Sprintf("%s: ats \"none\" should include a careers_url so the gap is visible in `list`", where))
			}
		case "":
			problems = append(problems, fmt.Sprintf("%s: ats is required (one of %s)", where, strings.Join(KnownATS(), ", ")))
		default:
			problems = append(problems, fmt.Sprintf("%s: ats %q is not supported; use one of %s", where, co.ATS, strings.Join(KnownATS(), ", ")))
		}
	}

	if len(problems) == 0 {
		return nil
	}
	sort.Strings(problems)
	return &ValidationError{Problems: problems}
}

// KnownATS lists the supported ATS values, for error messages and docs.
func KnownATS() []string {
	return []string{ATSGreenhouse, ATSLever, ATSAshby, ATSNone}
}

// ValidationError aggregates all config problems into one readable message.
type ValidationError struct {
	Problems []string
}

func (e *ValidationError) Error() string {
	var b strings.Builder
	n := len(e.Problems)
	if n == 1 {
		b.WriteString("1 problem in your config:\n")
	} else {
		fmt.Fprintf(&b, "%d problems in your config:\n", n)
	}
	for _, p := range e.Problems {
		b.WriteString("  - ")
		b.WriteString(p)
		b.WriteByte('\n')
	}
	return strings.TrimRight(b.String(), "\n")
}

// friendlyYAMLError makes the yaml library's errors a little kinder.
func friendlyYAMLError(err error) error {
	msg := err.Error()
	msg = strings.TrimPrefix(msg, "yaml: ")
	if strings.Contains(msg, "field") && strings.Contains(msg, "not found in type") {
		// e.g. "line 4: field compaines not found in type config.Config"
		return fmt.Errorf("unexpected setting (check for a typo): %s", msg)
	}
	return fmt.Errorf("could not read the YAML: %s", msg)
}

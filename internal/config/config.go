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
	"net/mail"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

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

// Hybrid/onsite policy values in location.allowed_work_models.
const (
	WorkModelHomeOnly = "home_only" // only when the office is in a home city (default)
	WorkModelAny      = "any"       // wherever the office is
	WorkModelNever    = "never"     // never take this work model
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

	// Rules configures the free rules scorer. It runs on every posting, also
	// when scorer is "llm" (it's the gate in front of the AI, ADR-0007).
	Rules Rules `yaml:"rules"`

	// LLM configures the optional AI scorer (used when scorer is "llm").
	LLM LLM `yaml:"llm"`

	// Email configures alerts and the digest (phase 4). Leave `to` empty to
	// turn email off.
	Email Email `yaml:"email"`

	// Search configures finding jobs by web search (ADR-0037). Off by default
	// because searching costs money.
	Search Search `yaml:"search"`

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
	Office     string         `yaml:"office,omitempty"`
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

// Rules drives the free, deterministic rules scorer (phase 2b). Score = 1 +
// (2 if a track title matches) + up to 2 for must_have_any + up to 1 for
// nice_to_have − 1 per matched red flag, clamped to 1–5.
type Rules struct {
	// Tracks maps a track name (e.g. "manager") to its title patterns.
	Tracks map[string]Track `yaml:"tracks"`
	// MustHaveAny keywords each add a small boost (capped).
	MustHaveAny []string `yaml:"must_have_any"`
	// NiceToHave keywords add a smaller boost (capped).
	NiceToHave []string `yaml:"nice_to_have"`
	// RedFlags maps a flag name to the keyword phrases that trigger it; each
	// distinct flag matched lowers the score by one and is recorded.
	RedFlags map[string][]string `yaml:"red_flags"`
	// Thresholds set the alert and digest score cut-offs.
	Thresholds Thresholds `yaml:"thresholds"`
}

// Email is the notification setup. The sender's address and app password are
// secrets, so they come from the environment (RADAR_EMAIL_ADDRESS and
// RADAR_EMAIL_APP_PASSWORD, usually in .env), never from this file.
type Email struct {
	// To lists who receives alerts and the digest (normally your main inbox).
	// Empty means email is off.
	To []string `yaml:"to"`
	// MatchAlerts sends one email per run listing new strong matches. Default on.
	MatchAlerts *bool `yaml:"match_alerts"`
	// Digest sends one daily summary of everything at or above the digest
	// threshold from the last 24 hours. Default on.
	Digest *bool `yaml:"digest"`
	// DigestTime is the local time (HH:MM) after which the first run of the
	// day sends the digest. Default "08:00".
	DigestTime string `yaml:"digest_time"`
	// NearMisses is how many "just below the line" jobs the digest adds so
	// you can say one should have scored higher. Default 5; 0 turns it off.
	NearMisses *int `yaml:"near_misses"`
	// SMTPHost and SMTPPort are the sending server (Gmail defaults).
	SMTPHost string `yaml:"smtp_host"`
	SMTPPort int    `yaml:"smtp_port"`
	// IMAPHost and IMAPPort are the server used to read replies (Gmail defaults).
	IMAPHost string `yaml:"imap_host"`
	IMAPPort int    `yaml:"imap_port"`
}

// Enabled reports whether email is switched on (at least one recipient).
func (e Email) Enabled() bool { return len(e.To) > 0 }

// AlertsOn and DigestOn apply the "default on" rule.
func (e Email) AlertsOn() bool { return e.MatchAlerts == nil || *e.MatchAlerts }
func (e Email) DigestOn() bool { return e.Digest == nil || *e.Digest }

// NearMissCount is how many near-miss jobs the digest lists (default 5).
func (e Email) NearMissCount() int {
	if e.NearMisses == nil {
		return 5
	}
	return *e.NearMisses
}

// Track is one career track's title patterns.
type Track struct {
	Titles []string `yaml:"titles"`
	// Label is the heading the digest uses for this track. Optional: without it
	// the track's name is tidied up ("lead_ic" becomes "Lead IC").
	Label string `yaml:"label"`
}

// Thresholds are the score cut-offs for alerts and the digest.
type Thresholds struct {
	Alert  int `yaml:"alert"`
	Digest int `yaml:"digest"`
}

// LLM configures the optional AI scorer (phase 3). Everything that costs money
// is capped here (ADR-0009), and the model is always chosen from config, never
// hard-coded.
type LLM struct {
	// Model is the model id to score with, e.g. "claude-haiku-5-5".
	Model string `yaml:"model"`
	// MinRulesScore is the rules gate: only postings the free rules scorer
	// scores at or above this go to the AI (ADR-0007). Default 2.
	MinRulesScore int `yaml:"min_rules_score"`
	// MaxPostingsPerRun caps how many postings one run sends to the AI. Default 25.
	MaxPostingsPerRun int `yaml:"max_postings_per_run"`
	// MaxTokensPerPosting caps input+output tokens across all turns for one
	// posting; past it, the posting keeps its rules score. Default 40000.
	MaxTokensPerPosting int `yaml:"max_tokens_per_posting"`
	// MaxTurns caps model calls per posting. Default 6.
	MaxTurns int `yaml:"max_turns"`
	// MonthlySpendCapUSD stops AI scoring for the rest of the calendar month
	// once the estimated spend reaches it. Default 5.
	MonthlySpendCapUSD float64 `yaml:"monthly_spend_cap_usd"`
	// Pricing holds per-model prices, so estimates and caps don't depend on
	// numbers baked into the code.
	Pricing Pricing `yaml:"pricing"`
}

// Pricing is the per-model price table, in USD per million tokens.
type Pricing struct {
	// LastChecked is the date (YYYY-MM-DD) the prices were last compared with
	// the provider's pricing page. A warning is shown when it's over 90 days old.
	LastChecked string `yaml:"last_checked"`
	// Source is the pricing page the numbers came from.
	Source string `yaml:"source"`
	// Models maps a model id to its prices.
	Models map[string]ModelPrice `yaml:"models"`
}

// ModelPrice is one model's prices, in USD per million tokens.
type ModelPrice struct {
	Input       float64 `yaml:"input"`
	Output      float64 `yaml:"output"`
	CacheWrite  float64 `yaml:"cache_write"`
	CacheRead   float64 `yaml:"cache_read"`
	BatchInput  float64 `yaml:"batch_input"`
	BatchOutput float64 `yaml:"batch_output"`
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
	if c.Rules.Thresholds.Alert == 0 {
		c.Rules.Thresholds.Alert = 4
	}
	if c.Rules.Thresholds.Digest == 0 {
		c.Rules.Thresholds.Digest = 3
	}
	if c.Email.DigestTime == "" {
		c.Email.DigestTime = "08:00"
	}
	if c.Email.SMTPHost == "" {
		c.Email.SMTPHost = "smtp.gmail.com"
	}
	if c.Email.SMTPPort == 0 {
		c.Email.SMTPPort = 587
	}
	if c.Email.IMAPHost == "" {
		c.Email.IMAPHost = "imap.gmail.com"
	}
	if c.Email.IMAPPort == 0 {
		c.Email.IMAPPort = 993
	}
	if c.LLM.MinRulesScore == 0 {
		c.LLM.MinRulesScore = 2
	}
	if c.LLM.MaxPostingsPerRun == 0 {
		c.LLM.MaxPostingsPerRun = 25
	}
	if c.LLM.MaxTokensPerPosting == 0 {
		c.LLM.MaxTokensPerPosting = 40000
	}
	if c.LLM.MaxTurns == 0 {
		c.LLM.MaxTurns = 6
	}
	if c.LLM.MonthlySpendCapUSD == 0 {
		c.LLM.MonthlySpendCapUSD = 5
	}
	c.Search.applyDefaults()
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

	for field, v := range map[string]string{
		"hybrid": c.Location.AllowedWorkModels.Hybrid,
		"onsite": c.Location.AllowedWorkModels.Onsite,
	} {
		switch v {
		case "", WorkModelHomeOnly, WorkModelAny, WorkModelNever:
		default:
			problems = append(problems, fmt.Sprintf("location.allowed_work_models.%s: %q is not valid; use %q, %q or %q",
				field, v, WorkModelHomeOnly, WorkModelAny, WorkModelNever))
		}
	}

	if c.Scorer == ScorerLLM {
		problems = append(problems, c.LLM.validate()...)
	}
	problems = append(problems, c.Email.validate()...)
	if c.Search.Enabled {
		problems = append(problems, c.Search.validate(c)...)
	}

	if len(problems) == 0 {
		return nil
	}
	sort.Strings(problems)
	return &ValidationError{Problems: problems}
}

// validate checks the email block. Nothing is required when email is off.
func (e Email) validate() []string {
	var problems []string
	for i, a := range e.To {
		if _, err := mail.ParseAddress(a); err != nil {
			problems = append(problems, fmt.Sprintf("email.to[%d]: %q is not an email address", i, a))
		}
	}
	if _, err := time.Parse("15:04", e.DigestTime); err != nil {
		problems = append(problems, fmt.Sprintf("email.digest_time: %q should be a 24-hour time like 08:00", e.DigestTime))
	}
	if e.NearMisses != nil && (*e.NearMisses < 0 || *e.NearMisses > 20) {
		problems = append(problems, "email.near_misses should be between 0 and 20")
	}
	if e.SMTPPort < 0 || e.IMAPPort < 0 {
		problems = append(problems, "email: ports can't be negative")
	}
	return problems
}

// validate checks the llm block. Only called when scorer is "llm", so users of
// the free scorer never see errors about settings they don't use.
func (l LLM) validate() []string {
	var problems []string
	if strings.TrimSpace(l.Model) == "" {
		problems = append(problems, "llm.model is required when scorer is \"llm\" (e.g. claude-haiku-5-5)")
	} else if _, ok := l.Pricing.Models[l.Model]; !ok {
		problems = append(problems, fmt.Sprintf("llm.pricing.models has no prices for %q; add them so costs can be estimated and capped", l.Model))
	}
	if l.MinRulesScore < 1 || l.MinRulesScore > 5 {
		problems = append(problems, fmt.Sprintf("llm.min_rules_score: %d is not valid; use 1–5", l.MinRulesScore))
	}
	if l.MaxPostingsPerRun < 0 || l.MaxTokensPerPosting < 0 || l.MaxTurns < 0 || l.MonthlySpendCapUSD < 0 {
		problems = append(problems, "llm: limits can't be negative")
	}
	if l.Pricing.LastChecked != "" {
		if _, err := time.Parse("2006-01-02", l.Pricing.LastChecked); err != nil {
			problems = append(problems, fmt.Sprintf("llm.pricing.last_checked: %q should be a date like 2026-10-09", l.Pricing.LastChecked))
		}
	}
	return problems
}

// PricingStale reports whether the price table was last checked more than 90
// days before now (or never), so the CLI can warn that estimates may be off.
func (l LLM) PricingStale(now time.Time) bool {
	t, err := time.Parse("2006-01-02", l.Pricing.LastChecked)
	if err != nil {
		return true
	}
	return now.Sub(t) > 90*24*time.Hour
}

// AllowedFlags is the closed set of flags a scorer may set: the built-in
// generic flags plus the user's own red-flag names. The AI scorer's
// record_score tool is validated against it.
func (c *Config) AllowedFlags() []string {
	set := map[string]bool{}
	for _, f := range BuiltinFlags {
		set[f] = true
	}
	for name := range c.Rules.RedFlags {
		set[name] = true
	}
	out := make([]string, 0, len(set))
	for f := range set {
		out = append(out, f)
	}
	sort.Strings(out)
	return out
}

// BuiltinFlags are generic flags every user gets (users add their own through
// rules.red_flags).
var BuiltinFlags = []string{
	"relocation", "outside_home_area", "outside_eligible_region", "work_model_not_accepted",
	"too_junior", "too_senior", "not_software", "unqualified_specialism", "limited_info",
}

// TrackNames lists the configured track names, sorted.
func (c *Config) TrackNames() []string {
	out := make([]string, 0, len(c.Rules.Tracks))
	for name := range c.Rules.Tracks {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
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

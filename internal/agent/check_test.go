package agent

import (
	"os"
	"strings"
	"testing"

	"github.com/pannamatena/job-radar/internal/config"
)

func checkConfig() *config.Config {
	return &config.Config{Rules: config.Rules{
		Tracks:   map[string]config.Track{"manager": {}, "lead_ic": {}},
		RedFlags: map[string][]string{"wants_go": {"golang"}, "wants_java": {}},
	}}
}

func TestCheckRubric_CleanRubricHasNoWarnings(t *testing.T) {
	rubric := "Tracks: `manager`, `lead_ic`. Flags: `wants_go`, `wants_java`, and built-in `outside_home_area`. Uses `record_score` and `work_model`."
	if w := CheckRubric(rubric, checkConfig()); len(w) != 0 {
		t.Errorf("want no warnings, got %v", w)
	}
}

func TestCheckRubric_FlagInRubricButNotInConfig(t *testing.T) {
	rubric := "`manager` `lead_ic` `wants_go` `wants_java` and `wants_rust` plus `no_ppl_management`"
	w := strings.Join(CheckRubric(rubric, checkConfig()), "\n")
	for _, name := range []string{"wants_rust", "no_ppl_management"} {
		if !strings.Contains(w, "`"+name+"`") || !strings.Contains(w, "red_flags") {
			t.Errorf("want a warning about %s that says where to define it, got:\n%s", name, w)
		}
	}
}

func TestCheckRubric_ConfigNameTheRubricNeverMentions(t *testing.T) {
	w := strings.Join(CheckRubric("only `manager` and `wants_go` here", checkConfig()), "\n")
	for _, name := range []string{"lead_ic", "wants_java"} {
		if !strings.Contains(w, "defines `"+name+"`") {
			t.Errorf("want a warning that the rubric never mentions %s, got:\n%s", name, w)
		}
	}
	if strings.Contains(w, "defines `manager`") || strings.Contains(w, "defines `wants_go`") {
		t.Errorf("names that are mentioned must not warn:\n%s", w)
	}
}

// The shipped example rubric must agree with the shipped example config, or a
// new user's first AI run would start with warnings.
func TestExampleRubricMatchesExampleConfig(t *testing.T) {
	cfg, err := config.Load("../../examples")
	if err != nil {
		t.Skipf("can't load the example config here: %v", err)
	}
	rubric, err := os.ReadFile("../../examples/rubric.md")
	if err != nil {
		t.Fatal(err)
	}
	if w := CheckRubric(string(rubric), cfg); len(w) != 0 {
		t.Errorf("examples/rubric.md and examples/config.yaml disagree:\n%s", strings.Join(w, "\n"))
	}
}

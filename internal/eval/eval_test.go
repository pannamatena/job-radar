package eval

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/pannamatena/job-radar/internal/posting"
	"github.com/pannamatena/job-radar/internal/scorer"
)

func writeCase(t *testing.T, root, name, label, ad string) {
	t.Helper()
	dir := filepath.Join(root, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(dir, "label.yaml"), []byte(label), 0o644)
	os.WriteFile(filepath.Join(dir, "ad.txt"), []byte(ad), 0o644)
}

const goodLabel = `status: confirmed
company: Acme
title: Engineering Manager
score: 4
track: manager
work_model: hybrid
office: Dublin
flags: [wants_go]
note: fits
`

func TestLoadCases_SplitsConfirmedAndDrafts(t *testing.T) {
	root := t.TempDir()
	writeCase(t, root, "a-good", goodLabel, "Engineering Manager — Acme (Dublin, hybrid)\n\nBody")
	writeCase(t, root, "b-draft", "status: draft\ncompany: X\ntitle: Y\nscore:\n", "ad")

	got, err := LoadCases(root, []string{"manager", "lead_ic"}, []string{"wants_go"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Confirmed) != 1 || got.Confirmed[0].Name != "a-good" {
		t.Errorf("confirmed = %+v", got.Confirmed)
	}
	if len(got.Drafts) != 1 || got.Drafts[0] != "b-draft" {
		t.Errorf("drafts = %v", got.Drafts)
	}
}

func TestLoadCases_ConfirmedButIncompleteIsAnError(t *testing.T) {
	root := t.TempDir()
	bad := strings.Replace(goodLabel, "track: manager", "track: wizard", 1)
	writeCase(t, root, "oops", bad, "ad")
	_, err := LoadCases(root, []string{"manager"}, []string{"wants_go"})
	if err == nil || !strings.Contains(err.Error(), "oops") || !strings.Contains(err.Error(), "track") {
		t.Errorf("want an error naming the case and the track, got %v", err)
	}
}

func TestLoadCases_UnknownFlagIsAnError(t *testing.T) {
	root := t.TempDir()
	writeCase(t, root, "typo", strings.Replace(goodLabel, "[wants_go]", "[wants_gooo]", 1), "ad")
	_, err := LoadCases(root, []string{"manager"}, []string{"wants_go"})
	if err == nil || !strings.Contains(err.Error(), "wants_gooo") {
		t.Errorf("want an error naming the unknown flag, got %v", err)
	}
}

func TestLoadCases_MissingFolder(t *testing.T) {
	if _, err := LoadCases(filepath.Join(t.TempDir(), "nope"), nil, nil); err == nil {
		t.Error("want an error for a missing cases folder")
	}
}

func TestCasePosting_ReadsLocationFromHeaderAndHidesTheLabel(t *testing.T) {
	for _, tc := range []struct {
		header, wantLoc string
		wantRemote      posting.RemoteKind
	}{
		{"Lead Engineer — Acme (Ireland (Hybrid))", "Ireland (Hybrid)", posting.RemoteHybrid},
		{"Engineering Manager — Globex (Remote)", "Remote", posting.RemoteYes},
		{"Manager — Initech (Dublin, Ireland)", "Dublin, Ireland", posting.RemoteUnknown},
	} {
		c := Case{Name: "n", Label: Label{Company: "Acme", Title: "T", Office: "SECRET-LABEL-OFFICE"}, Ad: tc.header + "\n\nThe body."}
		p := c.Posting()
		if p.Location != tc.wantLoc || p.Remote != tc.wantRemote {
			t.Errorf("%q: got location %q remote %q", tc.header, p.Location, p.Remote)
		}
		if p.DescriptionText != "The body." {
			t.Errorf("header should be stripped from the body, got %q", p.DescriptionText)
		}
		if strings.Contains(p.Location+p.DescriptionText, "SECRET") {
			t.Error("the human's label must never reach the scorer")
		}
	}
	// No header: the whole text is the body, location unknown.
	p := Case{Label: Label{Company: "A", Title: "T"}, Ad: "About the job\nStuff"}.Posting()
	if p.Location != "" || !strings.HasPrefix(p.DescriptionText, "About the job") {
		t.Errorf("no-header case: %+v", p)
	}
}

// fixtureResults is small enough to check every number by hand. Alert threshold 4.
//
//	case  human  attempt  note
//	a     4      4        exact, true alert
//	b     2      4        wrong alert (+2 off)
//	c     4      3        missed good role (-1, within ±1)
//	d     1      1        exact, correctly quiet
//	e     3      (unscored)
func fixtureResults() Results {
	at := func(score int, track, wm string, flags ...string) Attempt {
		return Attempt{Scored: true, Score: score, Track: track, WorkModel: wm, Flags: flags, Turns: 2, CostUSD: 0.001, LatencyMS: 2000, InputTokens: 100, OutputTokens: 50}
	}
	h := func(score int, track, wm string, flags ...string) Human {
		return Human{Score: score, Track: track, WorkModel: wm, Flags: flags, Note: "n"}
	}
	return Results{
		Version: ResultsVersion, Scorer: "llm", AlertThreshold: 4, GateMin: 2, Repeats: 1,
		Cases: []CaseResult{
			{Name: "a", Human: h(4, "manager", "hybrid", "wants_go"), RulesScore: 5, Attempts: []Attempt{at(4, "manager", "hybrid", "wants_go")}},
			{Name: "b", Human: h(2, "none", "remote"), RulesScore: 4, Attempts: []Attempt{at(4, "manager", "remote", "ml_team")}},
			{Name: "c", Human: h(4, "lead_ic", "hybrid"), RulesScore: 1, Attempts: []Attempt{at(3, "lead_ic", "onsite")}},
			{Name: "d", Human: h(1, "none", "remote", "not_software"), RulesScore: 1, Attempts: []Attempt{at(1, "none", "remote")}},
			{Name: "e", Human: h(3, "none", "remote"), RulesScore: 3, Attempts: []Attempt{{Scored: false, Score: 3, Turns: 6, CostUSD: 0.002}}},
		},
	}
}

func TestSummarise_ByHand(t *testing.T) {
	s := Summarise(fixtureResults())

	if s.Cases != 5 || s.Attempts != 5 || s.Unscored != 1 {
		t.Errorf("counts: %+v", s)
	}
	// 4 scored attempts: exact on a and d = 2/4; within ±1 on a, c, d = 3/4.
	approx(t, "exact", s.Exact, 0.5)
	approx(t, "within1", s.Within1, 0.75)
	// Alert threshold 4: a is a true alert, b a wrong alert, c a missed role, d correctly quiet.
	if s.TruePos != 1 || s.WrongAlerts != 1 || s.Missed != 1 || s.TrueNeg != 1 {
		t.Errorf("alert counts: tp=%d wrong=%d missed=%d tn=%d", s.TruePos, s.WrongAlerts, s.Missed, s.TrueNeg)
	}
	// Track right on a (manager), c (lead_ic), d (none) = 3/4. Work model right on a, b, d = 3/4 (c: hybrid vs onsite).
	approx(t, "track", s.TrackAcc, 0.75)
	approx(t, "work model", s.WorkModelAcc, 0.75)

	flags := map[string]FlagStat{}
	for _, f := range s.Flags {
		flags[f.Flag] = f
	}
	if f := flags["wants_go"]; f.TP != 1 || f.FP != 0 || f.FN != 0 {
		t.Errorf("wants_go: %+v", f)
	}
	if f := flags["ml_team"]; f.TP != 0 || f.FP != 1 || f.Precision != 0 || f.Rec != -1 {
		t.Errorf("ml_team (set but not expected): %+v", f)
	}
	if f := flags["not_software"]; f.FN != 1 || f.Rec != 0 || f.Precision != -1 {
		t.Errorf("not_software (expected but missed): %+v", f)
	}

	// The gate (min 2): c and d have rules score 1. Only c is a good role.
	if s.BelowGate != 2 || len(s.BelowGateGood) != 1 || s.BelowGateGood[0] != "c" {
		t.Errorf("gate: below=%d good=%v", s.BelowGate, s.BelowGateGood)
	}
	if len(s.Disagreements) != 2 {
		t.Errorf("want disagreements for b and c, got %+v", s.Disagreements)
	}
	approx(t, "cost total", s.CostTotal, 0.006)
}

func approx(t *testing.T, name string, got, want float64) {
	t.Helper()
	if d := got - want; d > 1e-9 || d < -1e-9 {
		t.Errorf("%s = %v, want %v", name, got, want)
	}
}

func TestSummarise_Consistency(t *testing.T) {
	at := func(score int) Attempt {
		return Attempt{Scored: true, Score: score, Track: "none", WorkModel: "remote"}
	}
	r := Results{AlertThreshold: 4, Repeats: 3, Cases: []CaseResult{
		{Name: "steady", Human: Human{Score: 3}, Attempts: []Attempt{at(3), at(3), at(3)}},
		{Name: "wobbly", Human: Human{Score: 3}, Attempts: []Attempt{at(2), at(4), at(3)}},
	}}
	s := Summarise(r)
	if s.Consistent != 1 {
		t.Errorf("consistent = %d, want 1", s.Consistent)
	}
	approx(t, "avg span", s.AvgScoreSpan, 1) // (0 + 2) / 2
}

func TestRun_UsesFakeScorersAndRepeats(t *testing.T) {
	cases := []Case{{Name: "one", Label: Label{Company: "A", Title: "T", Score: 4, Track: "none", WorkModel: "remote"}, Ad: "ad"}}
	calls := 0
	fake := func(_ context.Context, p posting.Posting) scorer.Result {
		calls++
		return scorer.Result{Score: 3, Track: "none", WorkModel: "remote", AI: &scorer.AIDetails{Status: scorer.AIScored, Turns: 2, CostUSD: 0.01}}
	}
	rules := func(context.Context, posting.Posting) scorer.Result { return scorer.Result{Score: 2} }
	res, err := Run(context.Background(), cases, fake, rules, Options{Scorer: "llm", Repeats: 3, AlertThreshold: 4, Now: time.Now()})
	if err != nil {
		t.Fatal(err)
	}
	if calls != 3 || len(res.Cases[0].Attempts) != 3 || res.Cases[0].RulesScore != 2 {
		t.Errorf("calls=%d result=%+v", calls, res.Cases[0])
	}
	if res.Cases[0].Human.Score != 4 {
		t.Error("the human label should be carried into the results")
	}
}

func TestSaveAndLoad_NeverOverwrites(t *testing.T) {
	dir := t.TempDir()
	r := fixtureResults()
	r.Date, r.Model, r.PromptVersion = "2026-10-13", "m", "abc"
	p1, err := Save(dir, r)
	if err != nil {
		t.Fatal(err)
	}
	p2, err := Save(dir, r)
	if err != nil {
		t.Fatal(err)
	}
	if p1 == p2 || filepath.Base(p1) != "2026-10-13_abc_m.json" {
		t.Errorf("paths: %s, %s", p1, p2)
	}
	back, err := Load(p1)
	if err != nil || len(back.Cases) != 5 {
		t.Errorf("load: %v %+v", err, back)
	}
	os.WriteFile(filepath.Join(dir, "old.json"), []byte(`{"version": 99}`), 0o644)
	if _, err := Load(filepath.Join(dir, "old.json")); err == nil {
		t.Error("want an error for a results file from another format version")
	}
}

func TestReportAndCompare_Smoke(t *testing.T) {
	r := fixtureResults()
	var buf bytes.Buffer
	WriteReport(&buf, r, Summarise(r), 2)
	out := buf.String()
	for _, want := range []string{"wrong alerts", "missed good roles", "would not have reached the AI", "hidden a good role", "Disagreements", "Small set", "2 draft"} {
		if !strings.Contains(out, want) {
			t.Errorf("report missing %q:\n%s", want, out)
		}
	}
	b := fixtureResults()
	b.Cases[1].Attempts[0].Score = 2
	buf.Reset()
	WriteCompare(&buf, r, b, "a.json", "b.json")
	if cmp := buf.String(); !strings.Contains(cmp, "≠ b") || !strings.Contains(cmp, "wrong alerts") {
		t.Errorf("compare output:\n%s", cmp)
	}
}

func TestEstimateCost_ScalesWithCasesAndRepeats(t *testing.T) {
	p := Pricing{Input: 0.1, Output: 0.5, CacheWrite: 0.125, CacheRead: 0.01}
	cases := []Case{{Ad: strings.Repeat("x", 4000)}, {Ad: strings.Repeat("x", 4000)}}
	one := EstimateCost(cases, 1, 20000, p)
	three := EstimateCost(cases, 3, 20000, p)
	if one.Scorings != 2 || three.Scorings != 6 || three.CostUSD <= one.CostUSD {
		t.Errorf("one=%+v three=%+v", one, three)
	}
	if one.CostUSD <= 0 || one.CostUSD > 0.05 {
		t.Errorf("a two-ad Haiku eval should cost cents at most, got $%.4f", one.CostUSD)
	}
}

func TestRun_PreFilterDropsBeforeScoring(t *testing.T) {
	cases := []Case{
		{Name: "rail", Label: Label{Company: "A", Title: "Rail Engineering Manager", Score: 1}, Ad: "x"},
		{Name: "good", Label: Label{Company: "A", Title: "Engineering Manager", Score: 5}, Ad: "x"},
		{Name: "rail-but-good", Label: Label{Company: "A", Title: "Railway Software Lead", Score: 4}, Ad: "x"},
	}
	scored := 0
	fake := func(_ context.Context, p posting.Posting) scorer.Result {
		scored++
		return scorer.Result{Score: 5, Track: "none", WorkModel: "remote"}
	}
	rules := func(context.Context, posting.Posting) scorer.Result { return scorer.Result{Score: 3} }
	pre := func(p posting.Posting) (bool, string) {
		if strings.Contains(strings.ToLower(p.Title), "rail") {
			return false, "drop: not software"
		}
		return true, "keep"
	}
	res, err := Run(context.Background(), cases, fake, rules, Options{Scorer: "rules", Repeats: 1, AlertThreshold: 4, PreFilter: pre, Now: time.Now()})
	if err != nil {
		t.Fatal(err)
	}
	if scored != 1 || !res.Cases[0].Dropped || res.Cases[1].Dropped || len(res.Cases[0].Attempts) != 0 {
		t.Errorf("scored=%d cases=%+v", scored, res.Cases)
	}
	s := Summarise(res)
	// rail: dropped and you said 1 → correctly quiet. good: alert and you said 5 → right.
	// rail-but-good: dropped but you said 4 → the filter hid a good role.
	if s.Dropped != 2 || s.TrueNeg != 1 || s.Missed != 1 || s.TruePos != 1 || len(s.DroppedGood) != 1 || s.DroppedGood[0] != "rail-but-good" {
		t.Errorf("summary: %+v", s)
	}
	var buf bytes.Buffer
	WriteReport(&buf, res, s, 0)
	if !strings.Contains(buf.String(), "pre-filter would have hidden a good role") {
		t.Errorf("report should warn about the hidden good role:\n%s", buf.String())
	}
}

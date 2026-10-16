package eval

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/pannamatena/job-radar/internal/posting"
	"github.com/pannamatena/job-radar/internal/scorer"
)

// ResultsVersion is bumped if the results file shape changes, so --compare can
// refuse files it can't read.
const ResultsVersion = 1

// ScoreFunc scores one posting. The rules scorer and the AI agent are both
// wrapped into this shape, so the runner doesn't care which one it is driving.
// (In Go a function type like this is the usual lightweight alternative to a
// one-method interface; think of a TypeScript callback type.)
type ScoreFunc func(ctx context.Context, p posting.Posting) scorer.Result

// Human is the label the scorer is measured against.
type Human struct {
	Score     int      `json:"score"`
	Track     string   `json:"track"`
	WorkModel string   `json:"work_model"`
	Flags     []string `json:"flags"`
	Note      string   `json:"note"`
}

// Attempt is one scoring of one case (a case is scored once per repeat).
type Attempt struct {
	Scored         bool     `json:"scored"` // false: the AI didn't finish, so the numbers below are the rules fallback
	Note           string   `json:"note,omitempty"`
	Score          int      `json:"score"`
	Track          string   `json:"track"`
	WorkModel      string   `json:"work_model"`
	Flags          []string `json:"flags"`
	Why            string   `json:"why"`
	Gap            string   `json:"gap"`
	Turns          int      `json:"turns"`
	SchemaFailures int      `json:"schema_failures"`
	InputTokens    int64    `json:"input_tokens"`
	OutputTokens   int64    `json:"output_tokens"`
	CacheWrite     int64    `json:"cache_write_tokens"`
	CacheRead      int64    `json:"cache_read_tokens"`
	CostUSD        float64  `json:"cost_usd"`
	LatencyMS      int64    `json:"latency_ms"`
}

// CaseResult is everything measured for one case.
type CaseResult struct {
	Name       string    `json:"name"`
	Human      Human     `json:"human"`
	RulesScore int       `json:"rules_score"`       // what the free scorer gave, for the gate statistics
	Dropped    bool      `json:"dropped,omitempty"` // the pre-filter would have removed this ad before any scoring
	DropReason string    `json:"drop_reason,omitempty"`
	Attempts   []Attempt `json:"attempts"`
}

// Results is one eval run, saved as a JSON file in the config folder.
type Results struct {
	Version        int          `json:"version"`
	Date           string       `json:"date"`
	Scorer         string       `json:"scorer"` // "rules" or "llm"
	Model          string       `json:"model,omitempty"`
	PromptVersion  string       `json:"prompt_version,omitempty"`
	Repeats        int          `json:"repeats"`
	AlertThreshold int          `json:"alert_threshold"`
	GateMin        int          `json:"gate_min,omitempty"` // rules score below this never reaches the AI in real runs
	Batch          bool         `json:"batch,omitempty"`    // run through the batch API: cheaper, and latency isn't measured
	Cases          []CaseResult `json:"cases"`
}

// Options describe a run.
type Options struct {
	Scorer         string // "rules" or "llm"
	Model          string
	PromptVersion  string
	Repeats        int
	AlertThreshold int
	GateMin        int
	Now            time.Time
	// PreFilter, if set, is applied first, as in a real run: an ad it drops is
	// never scored. It returns whether to keep the ad and why.
	PreFilter func(p posting.Posting) (keep bool, reason string)
	Progress  func(done, total int, name string) // optional
}

// Run scores every case Repeats times with score, and uses rules to record the
// free scorer's opinion alongside (needed for the gate statistics). Cases run
// one after another: the point is clean numbers, not speed.
func Run(ctx context.Context, cases []Case, score, rules ScoreFunc, o Options) (Results, error) {
	if o.Repeats < 1 {
		o.Repeats = 1
	}
	res := Results{
		Version: ResultsVersion, Date: o.Now.Format("2006-01-02"),
		Scorer: o.Scorer, Model: o.Model, PromptVersion: o.PromptVersion,
		Repeats: o.Repeats, AlertThreshold: o.AlertThreshold, GateMin: o.GateMin,
	}
	for i, c := range cases {
		if err := ctx.Err(); err != nil {
			return res, err
		}
		p := c.Posting()
		cr := CaseResult{
			Name: c.Name,
			Human: Human{
				Score: c.Label.Score, Track: c.Label.Track, WorkModel: c.Label.WorkModel,
				Flags: c.Label.Flags, Note: c.Label.Note,
			},
		}
		if o.PreFilter != nil {
			if keep, reason := o.PreFilter(p); !keep {
				cr.Dropped, cr.DropReason = true, reason
				res.Cases = append(res.Cases, cr)
				if o.Progress != nil {
					o.Progress(i+1, len(cases), c.Name+" (dropped by the pre-filter)")
				}
				continue
			}
		}
		cr.RulesScore = rules(ctx, p).Score
		for r := 0; r < o.Repeats; r++ {
			start := time.Now()
			out := score(ctx, p)
			cr.Attempts = append(cr.Attempts, attemptFrom(out, time.Since(start)))
		}
		res.Cases = append(res.Cases, cr)
		if o.Progress != nil {
			o.Progress(i+1, len(cases), c.Name)
		}
	}
	return res, nil
}

func attemptFrom(r scorer.Result, took time.Duration) Attempt {
	a := Attempt{
		Scored: true, Score: r.Score, Track: r.Track, WorkModel: r.WorkModel,
		Flags: r.Flags, Why: r.Why, Gap: r.Gap, LatencyMS: took.Milliseconds(),
	}
	if ai := r.AI; ai != nil {
		a.Turns, a.SchemaFailures = ai.Turns, ai.SchemaFailures
		a.InputTokens, a.OutputTokens = ai.InputTokens, ai.OutputTokens
		a.CacheWrite, a.CacheRead = ai.CacheWriteTokens, ai.CacheReadTokens
		a.CostUSD = ai.CostUSD
		if ai.Status != scorer.AIScored {
			a.Scored, a.Note = false, ai.Note
		}
	}
	return a
}

// Save writes the results to <dir>/<date>_<label>.json and returns the path.
// If that name is taken (a second run the same day) it adds -2, -3, … rather
// than overwriting an earlier run.
func Save(dir string, r Results) (string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	label := "rules"
	if r.Scorer == "llm" {
		label = r.PromptVersion + "_" + r.Model
		if r.Batch {
			label += "_batch"
		}
	}
	base := r.Date + "_" + label
	path := filepath.Join(dir, base+".json")
	for n := 2; ; n++ {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			break
		}
		path = filepath.Join(dir, fmt.Sprintf("%s-%d.json", base, n))
	}
	b, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return "", err
	}
	return path, os.WriteFile(path, append(b, '\n'), 0o644)
}

// Load reads a results file.
func Load(path string) (Results, error) {
	var r Results
	b, err := os.ReadFile(path)
	if err != nil {
		return r, err
	}
	if err := json.Unmarshal(b, &r); err != nil {
		return r, fmt.Errorf("%s is not a results file: %w", path, err)
	}
	if r.Version != ResultsVersion {
		return r, fmt.Errorf("%s has results format version %d; this build reads version %d", path, r.Version, ResultsVersion)
	}
	return r, nil
}

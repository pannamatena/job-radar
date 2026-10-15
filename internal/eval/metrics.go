package eval

import (
	"sort"
	"strings"
)

// FlagStat is precision/recall counts for one flag across all scored attempts.
type FlagStat struct {
	Flag           string
	TP, FP, FN     int
	Precision, Rec float64 // -1 when undefined (nothing predicted / nothing expected)
}

// Disagreement is a scored attempt whose score differs from the human's.
type Disagreement struct {
	Case       string
	Human, Got int
	Why, Gap   string // the model's words
	Note       string // the human's words
}

// Summary is every number the report prints.
type Summary struct {
	Cases, Attempts, Unscored int

	// The pre-filter runs before any scoring in a real run. Dropped cases are
	// never scored; they count as "quiet" in the alert numbers below.
	Dropped     int
	DroppedGood []string // dropped cases the human rated at or above the alert threshold

	Exact, Within1 float64 // share of scored attempts

	// At the alert threshold: a "wrong alert" is one the human wouldn't want;
	// a "missed" role is a good one the scorer wouldn't have alerted.
	Threshold                             int
	TruePos, WrongAlerts, Missed, TrueNeg int

	TrackAcc, WorkModelAcc float64
	Flags                  []FlagStat

	SchemaFailures int
	AvgTurns       float64
	Input, Output  int64
	CacheW, CacheR int64
	CostTotal      float64
	CostPerCase    float64 // per attempt
	AvgLatencyMS   float64

	// The rules gate: how many cases a real run would not have sent to the AI.
	GateMin       int
	BelowGate     int
	BelowGateGood []string // blocked cases the human rated at or above the alert threshold

	// Repeats > 1: how often a case got the same score every time.
	Repeats      int
	Consistent   int     // cases whose repeats all agree
	AvgScoreSpan float64 // mean of (max - min) score per case

	Disagreements []Disagreement
}

// Summarise computes the Summary for a results file. Attempts where the AI
// didn't finish are counted but left out of the accuracy numbers, because
// their score is the rules fallback, not the AI's.
func Summarise(r Results) Summary {
	s := Summary{Cases: len(r.Cases), Threshold: r.AlertThreshold, GateMin: r.GateMin, Repeats: r.Repeats}
	var exact, within1, trackOK, wmOK, scored int
	flagTP, flagFP, flagFN := map[string]int{}, map[string]int{}, map[string]int{}
	var turns int

	for _, c := range r.Cases {
		humanGood := c.Human.Score >= r.AlertThreshold
		if c.Dropped {
			s.Dropped++
			if humanGood {
				s.DroppedGood = append(s.DroppedGood, c.Name)
				s.Missed++
			} else {
				s.TrueNeg++
			}
			continue
		}
		if r.GateMin > 0 && c.RulesScore < r.GateMin {
			s.BelowGate++
			if humanGood {
				s.BelowGateGood = append(s.BelowGateGood, c.Name)
			}
		}
		lo, hi, n := 6, 0, 0
		for _, a := range c.Attempts {
			s.Attempts++
			s.SchemaFailures += a.SchemaFailures
			turns += a.Turns
			s.Input += a.InputTokens
			s.Output += a.OutputTokens
			s.CacheW += a.CacheWrite
			s.CacheR += a.CacheRead
			s.CostTotal += a.CostUSD
			s.AvgLatencyMS += float64(a.LatencyMS)
			if !a.Scored {
				s.Unscored++
				continue
			}
			scored++
			d := a.Score - c.Human.Score
			if d == 0 {
				exact++
			}
			if d >= -1 && d <= 1 {
				within1++
			}
			switch gotAlert := a.Score >= r.AlertThreshold; {
			case gotAlert && humanGood:
				s.TruePos++
			case gotAlert && !humanGood:
				s.WrongAlerts++
			case !gotAlert && humanGood:
				s.Missed++
			default:
				s.TrueNeg++
			}
			if a.Track == c.Human.Track {
				trackOK++
			}
			if a.WorkModel == c.Human.WorkModel {
				wmOK++
			}
			got := toSet(a.Flags)
			want := toSet(c.Human.Flags)
			for f := range got {
				if want[f] {
					flagTP[f]++
				} else {
					flagFP[f]++
				}
			}
			for f := range want {
				if !got[f] {
					flagFN[f]++
				}
			}
			if d != 0 {
				s.Disagreements = append(s.Disagreements, Disagreement{
					Case: c.Name, Human: c.Human.Score, Got: a.Score, Why: a.Why, Gap: a.Gap, Note: strings.TrimSpace(c.Human.Note),
				})
			}
			lo, hi, n = min(lo, a.Score), max(hi, a.Score), n+1
		}
		if r.Repeats > 1 && n == r.Repeats {
			s.AvgScoreSpan += float64(hi - lo)
			if hi == lo {
				s.Consistent++
			}
		}
	}

	if scored > 0 {
		s.Exact = pct(exact, scored)
		s.Within1 = pct(within1, scored)
		s.TrackAcc = pct(trackOK, scored)
		s.WorkModelAcc = pct(wmOK, scored)
	}
	if s.Attempts > 0 {
		s.AvgTurns = float64(turns) / float64(s.Attempts)
		s.CostPerCase = s.CostTotal / float64(s.Attempts)
		s.AvgLatencyMS /= float64(s.Attempts)
	}
	if r.Repeats > 1 && s.Cases > 0 {
		s.AvgScoreSpan /= float64(s.Cases)
	}

	names := map[string]bool{}
	for f := range flagTP {
		names[f] = true
	}
	for f := range flagFP {
		names[f] = true
	}
	for f := range flagFN {
		names[f] = true
	}
	for f := range names {
		fs := FlagStat{Flag: f, TP: flagTP[f], FP: flagFP[f], FN: flagFN[f], Precision: -1, Rec: -1}
		if fs.TP+fs.FP > 0 {
			fs.Precision = float64(fs.TP) / float64(fs.TP+fs.FP)
		}
		if fs.TP+fs.FN > 0 {
			fs.Rec = float64(fs.TP) / float64(fs.TP+fs.FN)
		}
		s.Flags = append(s.Flags, fs)
	}
	sort.Slice(s.Flags, func(i, j int) bool { return s.Flags[i].Flag < s.Flags[j].Flag })
	return s
}

func pct(n, d int) float64 { return float64(n) / float64(d) }

func toSet(xs []string) map[string]bool {
	m := make(map[string]bool, len(xs))
	for _, x := range xs {
		m[x] = true
	}
	return m
}

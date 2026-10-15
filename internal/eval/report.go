package eval

import (
	"fmt"
	"io"
	"strings"
)

// WriteReport prints one run's numbers in plain language.
func WriteReport(w io.Writer, r Results, s Summary, drafts int) {
	label := "free rules scorer"
	if r.Scorer == "llm" {
		label = fmt.Sprintf("AI scorer, %s, prompt %s", r.Model, r.PromptVersion)
	}
	fmt.Fprintf(w, "Eval of the %s — %s\n", label, r.Date)
	fmt.Fprintf(w, "%d confirmed case(s)", s.Cases)
	if r.Repeats > 1 {
		fmt.Fprintf(w, " × %d repeats = %d scorings", r.Repeats, s.Attempts)
	}
	if drafts > 0 {
		fmt.Fprintf(w, "  (%d draft case(s) left out: they are not labelled yet)", drafts)
	}
	fmt.Fprintln(w)
	if s.Cases < 20 {
		fmt.Fprintf(w, "Small set: %d case(s). Treat these as a rough guide, not a measurement.\n", s.Cases)
	}
	if s.Unscored > 0 {
		fmt.Fprintf(w, "%d scoring(s) did not finish and are left out of the accuracy numbers.\n", s.Unscored)
	}

	if s.Dropped > 0 {
		fmt.Fprintf(w, "%d case(s) were removed by the pre-filter before scoring, as in a real run, and count as quiet:\n", s.Dropped)
		for _, c := range r.Cases {
			if c.Dropped {
				fmt.Fprintf(w, "  %s — %s\n", c.Name, c.DropReason)
			}
		}
		if len(s.DroppedGood) > 0 {
			fmt.Fprintf(w, "  ⚠ you rated %d of those at or above the alert threshold, so the pre-filter would have hidden a good role: %s\n", len(s.DroppedGood), strings.Join(s.DroppedGood, ", "))
		}
	}

	scored := s.Attempts - s.Unscored
	fmt.Fprintf(w, "\nScore accuracy (%d scored)\n", scored)
	fmt.Fprintf(w, "  exact match         %s\n", percent(s.Exact))
	fmt.Fprintf(w, "  within ±1           %s\n", percent(s.Within1))

	fmt.Fprintf(w, "\nAt the alert threshold (score ≥ %d) — the number that matters most\n", s.Threshold)
	fmt.Fprintf(w, "  wrong alerts        %d   (it would alert you, you'd have said no)\n", s.WrongAlerts)
	fmt.Fprintf(w, "  missed good roles   %d   (you'd have said yes, it would stay quiet)\n", s.Missed)
	fmt.Fprintf(w, "  right calls         %d alerts, %d correctly quiet\n", s.TruePos, s.TrueNeg)

	fmt.Fprintf(w, "\nTrack right         %s\n", percent(s.TrackAcc))
	fmt.Fprintf(w, "Work model right    %s\n", percent(s.WorkModelAcc))
	if len(s.Flags) > 0 {
		fmt.Fprintln(w, "\nFlags (precision = flags it set that you agree with; recall = flags you expected that it set)")
		for _, f := range s.Flags {
			fmt.Fprintf(w, "  %-32s precision %-5s recall %-5s  (right %d, extra %d, missed %d)\n",
				f.Flag, percent(f.Precision), percent(f.Rec), f.TP, f.FP, f.FN)
		}
	}

	if r.Scorer == "llm" {
		fmt.Fprintf(w, "\nCost and reliability\n")
		fmt.Fprintf(w, "  turns per scoring   %.1f\n", s.AvgTurns)
		fmt.Fprintf(w, "  schema failures     %d invalid record_score call(s)\n", s.SchemaFailures)
		fmt.Fprintf(w, "  tokens              %d in, %d out, %d cache write, %d cache read\n", s.Input, s.Output, s.CacheW, s.CacheR)
		fmt.Fprintf(w, "  cost                $%.4f total, $%.5f per scoring\n", s.CostTotal, s.CostPerCase)
		fmt.Fprintf(w, "  latency             %.1fs per scoring\n", s.AvgLatencyMS/1000)
		fmt.Fprintf(w, "\n  estimated monthly cost for the AI alone: $%.2f at 10 ads/day, $%.2f at 20 ads/day\n",
			s.CostPerCase*10*30, s.CostPerCase*20*30)
		if s.GateMin > 0 && s.Cases > 0 {
			share := float64(s.Cases-s.BelowGate) / float64(s.Cases)
			fmt.Fprintf(w, "  with the rules gate, only %s of these cases would reach the AI: $%.2f and $%.2f a month\n",
				percent(share), s.CostPerCase*share*10*30, s.CostPerCase*share*20*30)
		}
	}

	if s.GateMin > 0 {
		fmt.Fprintf(w, "\nThe rules gate (rules score must be ≥ %d for the AI to see an ad)\n", s.GateMin)
		fmt.Fprintf(w, "  %d of %d case(s) would not have reached the AI\n", s.BelowGate, s.Cases)
		if len(s.BelowGateGood) > 0 {
			fmt.Fprintf(w, "  ⚠ of those, you rated %d at or above the alert threshold, so the gate would have hidden a good role: %s\n",
				len(s.BelowGateGood), strings.Join(s.BelowGateGood, ", "))
		} else if s.BelowGate > 0 {
			fmt.Fprintln(w, "  none of them were good roles, so the gate lost nothing here")
		}
	}

	if r.Repeats > 1 {
		fmt.Fprintf(w, "\nConsistency across %d repeats\n", r.Repeats)
		fmt.Fprintf(w, "  %d of %d case(s) got the same score every time; average spread %.2f points\n", s.Consistent, s.Cases, s.AvgScoreSpan)
	}

	if len(s.Disagreements) > 0 {
		fmt.Fprintf(w, "\nDisagreements (%d)\n", len(s.Disagreements))
		for _, d := range s.Disagreements {
			fmt.Fprintf(w, "  %s: you %d, scorer %d\n", d.Case, d.Human, d.Got)
			fmt.Fprintf(w, "    scorer why: %s\n    scorer gap: %s\n    your note:  %s\n", d.Why, d.Gap, orDash(d.Note))
		}
	}
}

// WriteCompare prints two runs side by side, then every case's scores.
func WriteCompare(w io.Writer, a, b Results, fileA, fileB string) {
	sa, sb := Summarise(a), Summarise(b)
	la, lb := runLabel(a), runLabel(b)
	fmt.Fprintf(w, "A = %s [%s]\nB = %s [%s]\n\n", la, fileA, lb, fileB)
	if a.AlertThreshold != b.AlertThreshold {
		fmt.Fprintf(w, "Note: the alert thresholds differ (A %d, B %d), so the alert counts are not directly comparable.\n\n", a.AlertThreshold, b.AlertThreshold)
	}
	row := func(name, x, y string) { fmt.Fprintf(w, "  %-26s %-14s %-14s\n", name, x, y) }
	fmt.Fprintf(w, "  %-26s %-14s %-14s\n", "", "A", "B")
	row("cases", fmt.Sprint(sa.Cases), fmt.Sprint(sb.Cases))
	row("exact match", percent(sa.Exact), percent(sb.Exact))
	row("within ±1", percent(sa.Within1), percent(sb.Within1))
	row("wrong alerts", fmt.Sprint(sa.WrongAlerts), fmt.Sprint(sb.WrongAlerts))
	row("missed good roles", fmt.Sprint(sa.Missed), fmt.Sprint(sb.Missed))
	row("track right", percent(sa.TrackAcc), percent(sb.TrackAcc))
	row("work model right", percent(sa.WorkModelAcc), percent(sb.WorkModelAcc))
	row("schema failures", fmt.Sprint(sa.SchemaFailures), fmt.Sprint(sb.SchemaFailures))
	row("turns per scoring", fmt.Sprintf("%.1f", sa.AvgTurns), fmt.Sprintf("%.1f", sb.AvgTurns))
	row("cost per scoring", fmt.Sprintf("$%.5f", sa.CostPerCase), fmt.Sprintf("$%.5f", sb.CostPerCase))
	row("avg latency", fmt.Sprintf("%.1fs", sa.AvgLatencyMS/1000), fmt.Sprintf("%.1fs", sb.AvgLatencyMS/1000))
	if a.Repeats > 1 || b.Repeats > 1 {
		row("same score every repeat", consistency(sa), consistency(sb))
	}

	bByName := map[string]CaseResult{}
	for _, c := range b.Cases {
		bByName[c.Name] = c
	}
	fmt.Fprintf(w, "\nPer case (you / A / B), alert threshold %d; ≠ marks a difference between A and B\n", a.AlertThreshold)
	shared := 0
	for _, ca := range a.Cases {
		cb, ok := bByName[ca.Name]
		if !ok {
			continue
		}
		shared++
		mark := " "
		if firstScore(ca) != firstScore(cb) {
			mark = "≠"
		}
		fmt.Fprintf(w, "  %s %-44s %d / %-2s / %-2s\n", mark, ca.Name, ca.Human.Score, scoreCell(ca), scoreCell(cb))
	}
	if shared != len(a.Cases) || shared != len(b.Cases) {
		fmt.Fprintf(w, "\nOnly %d case(s) appear in both runs; the summary rows above cover each run's own cases.\n", shared)
	}
}

func runLabel(r Results) string {
	if r.Scorer == "llm" {
		return fmt.Sprintf("%s, %s, prompt %s (%s)", "AI", r.Model, r.PromptVersion, r.Date)
	}
	return fmt.Sprintf("rules scorer (%s)", r.Date)
}

func consistency(s Summary) string {
	if s.Repeats < 2 {
		return "n/a"
	}
	return fmt.Sprintf("%d of %d", s.Consistent, s.Cases)
}

func firstScore(c CaseResult) int {
	if len(c.Attempts) == 0 {
		return 0
	}
	return c.Attempts[0].Score
}

// scoreCell shows the score, or all repeats' scores like "3,4,3", or "?" if
// the AI didn't finish.
func scoreCell(c CaseResult) string {
	if c.Dropped {
		return "—"
	}
	var parts []string
	for _, a := range c.Attempts {
		if !a.Scored {
			parts = append(parts, "?")
			continue
		}
		parts = append(parts, fmt.Sprint(a.Score))
	}
	return strings.Join(parts, ",")
}

func percent(f float64) string {
	if f < 0 {
		return "n/a"
	}
	return fmt.Sprintf("%.0f%%", f*100)
}

func orDash(s string) string {
	if s == "" {
		return "—"
	}
	return s
}

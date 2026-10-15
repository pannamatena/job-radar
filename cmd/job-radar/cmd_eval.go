package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pannamatena/job-radar/internal/config"
	"github.com/pannamatena/job-radar/internal/eval"
	"github.com/pannamatena/job-radar/internal/filter"
	"github.com/pannamatena/job-radar/internal/posting"
	"github.com/pannamatena/job-radar/internal/scorer"
	"github.com/pannamatena/job-radar/internal/store"
)

const evalUsage = `usage:
  job-radar eval [--scorer rules|llm] [--model NAME] [--repeats N] [--skip-prefilter] [--yes]
  job-radar eval --compare <results_a> <results_b>`

// cmdEval measures a scorer against the owner's hand-labelled cases, or
// compares two saved runs. The rules scorer is free; the AI scorer costs money,
// so it prints an estimate and asks first.
func cmdEval(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("eval", flag.ContinueOnError)
	home := fs.String("home", config.Home(), "config directory to read")
	scorerName := fs.String("scorer", "llm", "rules or llm")
	model := fs.String("model", "", "AI model for this run (default: llm.model in your config)")
	repeats := fs.Int("repeats", 1, "score each case this many times, to see how consistent the scores are")
	yes := fs.Bool("yes", false, "don't ask before spending money")
	compare := fs.Bool("compare", false, "compare two saved results files given as arguments")
	skipPre := fs.Bool("skip-prefilter", false, "score every case, even ones the pre-filter would drop in a real run")
	batch := fs.Bool("batch", false, "use the provider's batch API (not built yet)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	resultsDir := filepath.Join(*home, "evals", "results")

	if *compare {
		if fs.NArg() != 2 {
			return fmt.Errorf("%s", evalUsage)
		}
		pathA, pathB := resolveResults(resultsDir, fs.Arg(0)), resolveResults(resultsDir, fs.Arg(1))
		a, err := eval.Load(pathA)
		if err != nil {
			return err
		}
		b, err := eval.Load(pathB)
		if err != nil {
			return err
		}
		eval.WriteCompare(os.Stdout, a, b, filepath.Base(pathA), filepath.Base(pathB))
		return nil
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("%s", evalUsage)
	}
	if *batch {
		return fmt.Errorf("--batch isn't built yet: the agent needs several back-and-forth calls per ad, which the batch API handles in rounds. Run without it for now")
	}
	if *repeats < 1 {
		return fmt.Errorf("--repeats must be at least 1")
	}

	cfg, err := config.Load(*home)
	if err != nil {
		return err
	}
	useLLM := false
	switch *scorerName {
	case config.ScorerRules:
	case config.ScorerLLM:
		useLLM = true
		cfg.Scorer = config.ScorerLLM
		if *model != "" {
			cfg.LLM.Model = *model
		}
		if err := cfg.Validate(); err != nil {
			return err
		}
	default:
		return fmt.Errorf("--scorer must be %q or %q", config.ScorerRules, config.ScorerLLM)
	}
	if !useLLM && *model != "" {
		return fmt.Errorf("--model only applies to --scorer llm")
	}

	loaded, err := eval.LoadCases(filepath.Join(*home, "evals", "cases"), cfg.TrackNames(), cfg.AllowedFlags())
	if err != nil {
		return err
	}
	if len(loaded.Confirmed) == 0 {
		return fmt.Errorf("no confirmed cases to evaluate (%d draft case(s) found). Fill in each label.yaml and change status: draft to status: confirmed — see docs/evaluation.md", len(loaded.Drafts))
	}
	cases := loaded.Confirmed

	rules := scorer.NewRules(cfg.Rules, cfg.Location)
	rulesFn := func(ctx context.Context, p posting.Posting) scorer.Result {
		r, _ := rules.Score(ctx, p) // the rules scorer never fails
		return r
	}
	opts := eval.Options{
		Scorer: config.ScorerRules, Repeats: *repeats, AlertThreshold: cfg.Rules.Thresholds.Alert,
		GateMin: cfg.LLM.MinRulesScore, Now: time.Now(),
		Progress: func(done, total int, name string) { fmt.Fprintf(os.Stderr, "  [%d/%d] %s\n", done, total, name) },
	}
	score := rulesFn
	if !*skipPre {
		pre := filter.New(cfg.PreFilter)
		opts.PreFilter = func(p posting.Posting) (bool, string) {
			d := pre.Check(p)
			return d.Keep, d.Reason
		}
	}

	if useLLM {
		a, err := newAgent(cfg)
		if err != nil {
			return err
		}
		opts.Scorer, opts.Model, opts.PromptVersion = config.ScorerLLM, a.Model(), a.PromptVersion()
		score = func(ctx context.Context, p posting.Posting) scorer.Result { return a.Score(ctx, p) }

		mp := cfg.LLM.Pricing.Models[cfg.LLM.Model]
		toScore := cases
		if opts.PreFilter != nil {
			toScore = nil
			for _, c := range cases {
				if keep, _ := opts.PreFilter(c.Posting()); keep {
					toScore = append(toScore, c)
				}
			}
		}
		est := eval.EstimateCost(toScore, *repeats, a.SystemChars(),
			eval.Pricing{Input: mp.Input, Output: mp.Output, CacheWrite: mp.CacheWrite, CacheRead: mp.CacheRead})
		if n := len(cases) - len(toScore); n > 0 {
			fmt.Printf("%d case(s) would be removed by the pre-filter first, as in a real run (--skip-prefilter scores them anyway).\n", n)
		}
		fmt.Printf("About to score %d case(s) × %d with %s (prompt %s).\nEstimated cost: about $%.3f (a rough guess; the real cost is printed afterwards).\n",
			len(toScore), *repeats, a.Model(), a.PromptVersion(), est.CostUSD)

		if err := checkMonthlyBudget(ctx, cfg, *home, est.CostUSD); err != nil {
			return err
		}
		if !*yes {
			ok, err := confirm("Go ahead? [y/N] ")
			if err != nil {
				return err
			}
			if !ok {
				fmt.Println("Cancelled. Nothing was spent.")
				return nil
			}
		}
	} else {
		fmt.Printf("Scoring %d case(s) with the free rules scorer.\n", len(cases))
	}

	res, err := eval.Run(ctx, cases, score, rulesFn, opts)
	if err != nil {
		return err
	}
	fmt.Println()
	eval.WriteReport(os.Stdout, res, eval.Summarise(res), len(loaded.Drafts))

	path, err := eval.Save(resultsDir, res)
	if err != nil {
		return err
	}
	fmt.Printf("\nSaved %s\n", path)
	if useLLM {
		fmt.Println("Note: eval spend is not added to the monthly spend the `run` command tracks, so watch your Console usage as well.")
	}
	return nil
}

// checkMonthlyBudget refuses an eval that, with the spend `run` has recorded
// this month, would pass the monthly cap.
func checkMonthlyBudget(ctx context.Context, cfg *config.Config, home string, estimate float64) error {
	st, err := store.Open(filepath.Join(home, "radar.db"))
	if err != nil {
		return err
	}
	defer st.Close()
	spent, err := st.AISpendSince(ctx, store.StartOfMonth(time.Now()))
	if err != nil {
		return err
	}
	if cap := cfg.LLM.MonthlySpendCapUSD; cap > 0 && spent+estimate > cap {
		return fmt.Errorf("this eval (~$%.3f) plus ~$%.2f already spent this month would pass your monthly cap of $%.2f; raise llm.monthly_spend_cap_usd to run it", estimate, spent, cap)
	}
	return nil
}

// confirm asks a yes/no question. It refuses to guess when nobody can answer.
func confirm(question string) (bool, error) {
	if fi, err := os.Stdin.Stat(); err != nil || fi.Mode()&os.ModeCharDevice == 0 {
		return false, fmt.Errorf("can't ask for confirmation here (no terminal); add --yes to approve the cost")
	}
	fmt.Print(question)
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil {
		return false, fmt.Errorf("no answer received; add --yes to approve the cost without being asked")
	}
	l := strings.ToLower(strings.TrimSpace(line))
	return l == "y" || l == "yes", nil
}

// resolveResults lets --compare take a bare file name from the results folder.
func resolveResults(dir, arg string) string {
	if _, err := os.Stat(arg); err == nil {
		return arg
	}
	if p := filepath.Join(dir, arg); fileExists(p) {
		return p
	}
	return arg
}

func fileExists(p string) bool { _, err := os.Stat(p); return err == nil }

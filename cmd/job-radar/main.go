// Command job-radar watches company job boards and scores new postings against
// your own career profile. This is the CLI entry point; each subcommand lives
// in its own file in this package.
//
// Commands not built yet (digest, eval, report, feedback) are present as stubs
// that say which phase adds them, so the help output reflects the whole tool.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/pannamatena/job-radar/internal/config"
	"github.com/pannamatena/job-radar/internal/logging"
	"github.com/pannamatena/job-radar/internal/version"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

// run dispatches a subcommand and returns a process exit code. Keeping the real
// logic here (rather than in main) makes it straightforward to test.
func run(args []string) int {
	if len(args) == 0 {
		printUsage(os.Stderr)
		return 2
	}

	cmd, rest := args[0], args[1:]

	// Secrets (the API key) come from a git-ignored .env file: the one in the
	// current folder, then one in the config folder. Real environment
	// variables always win.
	for _, path := range []string{".env", filepath.Join(config.Home(), ".env")} {
		if err := config.LoadDotEnv(path); err != nil {
			fmt.Fprintf(os.Stderr, "job-radar: %s\n", err)
			return 1
		}
	}

	// Structured logger for operational logs (to stderr). User-facing output is
	// printed directly to stdout by each command. JSON in CI, text otherwise.
	logger := logging.New(logging.Options{
		Level: slog.LevelInfo,
		JSON:  os.Getenv("CI") != "",
	})

	// Cancel the context on Ctrl-C / SIGTERM so long runs stop cleanly.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	switch cmd {
	case "init":
		return exit(cmdInit(ctx, rest))
	case "list":
		return exit(cmdList(ctx, logger, rest))
	case "version", "--version", "-v":
		fmt.Printf("job-radar %s\n", version.Version)
		return 0
	case "help", "--help", "-h":
		printUsage(os.Stdout)
		return 0

	case "run":
		return exit(cmdRun(ctx, logger, rest))
	case "score-one":
		return exit(cmdScoreOne(ctx, rest))

	// Stubs for commands added in later phases.
	case "digest":
		return stub(cmd, "phase 4 (email digest)")
	case "discover":
		return exit(cmdDiscover(ctx, rest))
	case "eval":
		return stub(cmd, "phase 3b (evaluation)")
	case "report":
		return stub(cmd, "phase 3b/4 (weekly report)")
	case "feedback":
		return stub(cmd, "phase 4 (record 👍/👎)")

	default:
		fmt.Fprintf(os.Stderr, "job-radar: unknown command %q\n\n", cmd)
		printUsage(os.Stderr)
		return 2
	}
}

// exit turns a command error into an exit code, printing a friendly message.
func exit(err error) int {
	if err == nil {
		return 0
	}
	fmt.Fprintf(os.Stderr, "job-radar: %s\n", err)
	return 1
}

func stub(cmd, phase string) int {
	fmt.Fprintf(os.Stderr, "job-radar: `%s` isn't available yet — it's coming in %s.\n", cmd, phase)
	return 2
}

func printUsage(w *os.File) {
	fmt.Fprintf(w, `job-radar %s — watch job boards and score new postings against your profile.

Usage:
  job-radar <command> [flags]

Available now:
  init            Create your config folder from a worked example.
  list            Fetch and print current postings for your watch-list.
  run             Fetch, score and print strong matches. --scorer rules|llm
  score-one       Score one posting and show why. --scorer llm shows the AI's
                  full transcript and cost.
  discover        Detect a company's job board from its careers URL.
  version         Print the version.
  help            Show this help.

Coming in later phases:
  digest          Send the daily digest email (phase 4).
  eval            Run the evaluation suite (phase 3b).
  report          Weekly report of alerts, feedback and cost (phase 3b/4).
  feedback        Record 👍/👎 for a posting (phase 4).

Your personal config lives in $JOB_RADAR_HOME (default ./private), which is
never committed to git. Run 'job-radar init' to get started.
`, version.Version)
}

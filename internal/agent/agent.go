package agent

import (
	"context"
	"fmt"
	"strings"

	"github.com/pannamatena/job-radar/internal/config"
	"github.com/pannamatena/job-radar/internal/llm"
	"github.com/pannamatena/job-radar/internal/posting"
	"github.com/pannamatena/job-radar/internal/scorer"
)

// outputTokensPerTurn caps each model reply. A record_score call is ~150
// tokens; this leaves room without letting a runaway reply cost much.
const outputTokensPerTurn = 1024

// Agent scores one posting at a time with a tool-calling loop.
type Agent struct {
	client    llm.Client
	model     string
	price     llm.Price
	system    string
	version   string
	tools     []llm.Tool
	tracks    []string
	flags     []string
	maxTurns  int
	maxTokens int64
	companies map[string]config.Company
}

// Options configure an Agent.
type Options struct {
	Client  llm.Client
	Config  *config.Config
	Profile string // contents of profile.md
	Rubric  string // contents of rubric.md
}

// New builds an Agent from the user's config, profile and rubric.
func New(o Options) *Agent {
	cfg := o.Config
	tracks, flags := cfg.TrackNames(), cfg.AllowedFlags()
	system := BuildSystemPrompt(o.Profile, o.Rubric, tracks, flags)
	mp := cfg.LLM.Pricing.Models[cfg.LLM.Model]

	companies := map[string]config.Company{}
	for _, c := range cfg.Companies {
		companies[c.Name] = c
	}
	return &Agent{
		client:    o.Client,
		model:     cfg.LLM.Model,
		price:     llm.Price{Input: mp.Input, Output: mp.Output, CacheWrite: mp.CacheWrite, CacheRead: mp.CacheRead},
		system:    system,
		version:   PromptVersion(system),
		tools:     toolDefs(tracks, flags),
		tracks:    tracks,
		flags:     flags,
		maxTurns:  cfg.LLM.MaxTurns,
		maxTokens: int64(cfg.LLM.MaxTokensPerPosting),
		companies: companies,
	}
}

// PromptVersion is the version hash stored with this agent's scores.
func (a *Agent) PromptVersion() string { return a.version }

// SystemChars is the length of the cached system prompt (profile, rubric and
// instructions), used to estimate what an eval run will cost.
func (a *Agent) SystemChars() int { return len(a.system) }

// Model is the model this agent scores with.
func (a *Agent) Model() string { return a.model }

// Score runs the loop for one posting:
//
//	send (cached system prompt + tools + conversation)
//	  → model replies, maybe with tool calls
//	  → run each tool, send the results back
//	  → repeat until record_score succeeds, or a limit is hit.
//
// It always returns a Result whose AI field says what happened. When the AI
// couldn't finish (turn limit, token budget, API error), the Result has
// Status AIUnscored and no score of its own; the caller keeps the rules score.
func (a *Agent) Score(ctx context.Context, p posting.Posting) scorer.Result {
	ai := &scorer.AIDetails{Model: a.model, PromptVersion: a.version}
	var usage llm.Usage
	finish := func(r scorer.Result, status, note string) scorer.Result {
		ai.Status, ai.Note = status, note
		ai.InputTokens, ai.OutputTokens = usage.InputTokens, usage.OutputTokens
		ai.CacheWriteTokens, ai.CacheReadTokens = usage.CacheWriteTokens, usage.CacheReadTokens
		ai.CostUSD = a.price.Cost(usage)
		r.AI = ai
		return r
	}
	logf := func(format string, args ...any) {
		ai.Transcript = append(ai.Transcript, fmt.Sprintf(format, args...))
	}

	env := toolEnv{p: p, company: a.companies[p.Company]}
	messages := []llm.Message{{
		Role:   llm.RoleUser,
		Blocks: []llm.Block{{Type: llm.BlockText, Text: userKickoff(p.ID)}},
	}}
	logf("user: %s", userKickoff(p.ID))

	for turn := 1; turn <= a.maxTurns; turn++ {
		ai.Turns = turn
		resp, err := a.client.Send(ctx, llm.Request{
			Model:       a.model,
			MaxTokens:   outputTokensPerTurn,
			System:      a.system,
			CacheSystem: true,
			Tools:       a.tools,
			Messages:    messages,
		})
		if err != nil {
			logf("error: %v", err)
			return finish(scorer.Result{}, scorer.AIUnscored, "API error: "+err.Error())
		}
		usage = usage.Add(resp.Usage)
		logf("model (turn %d, %d in / %d out / %d cache-read tokens, stop=%s)",
			turn, resp.Usage.InputTokens, resp.Usage.OutputTokens, resp.Usage.CacheReadTokens, resp.StopReason)

		// The conversation must include the model's reply exactly as sent,
		// tool calls and all, before our tool results. The API rejects a
		// message with no content, and the model occasionally returns an
		// empty reply, so stand in a short placeholder in that case.
		sent := resp.Blocks
		if len(sent) == 0 {
			sent = []llm.Block{{Type: llm.BlockText, Text: "(no reply)"}}
			logf("  (the model sent an empty reply)")
		}
		messages = append(messages, llm.Message{Role: llm.RoleAssistant, Blocks: sent})

		var results []llm.Block
		for _, b := range resp.Blocks {
			switch b.Type {
			case llm.BlockText:
				if t := strings.TrimSpace(b.Text); t != "" {
					logf("  says: %s", oneLine(t, 300))
				}
			case llm.BlockToolUse:
				logf("  calls %s %s", b.ToolName, oneLine(string(b.Input), 600))
				if b.ToolName == ToolRecordScore {
					r, verr := validateRecord(b.Input, a.tracks, a.flags)
					if verr == nil {
						logf("  → recorded score %d/5 (%s)", r.Score, r.Track)
						return finish(r, scorer.AIScored, "")
					}
					ai.SchemaFailures++
					logf("  → rejected: %v", verr)
					results = append(results, llm.Block{Type: llm.BlockToolResult, ToolUseID: b.ToolUseID, Text: verr.Error(), IsError: true})
					continue
				}
				out, terr := env.run(b.ToolName, b.Input)
				if terr != nil {
					logf("  → error: %v", terr)
					results = append(results, llm.Block{Type: llm.BlockToolResult, ToolUseID: b.ToolUseID, Text: terr.Error(), IsError: true})
					continue
				}
				logf("  → %s", oneLine(out, 200))
				results = append(results, llm.Block{Type: llm.BlockToolResult, ToolUseID: b.ToolUseID, Text: out})
			}
		}

		if usage.Total() > a.maxTokens {
			note := fmt.Sprintf("token budget exceeded (%d > %d)", usage.Total(), a.maxTokens)
			logf("stop: %s", note)
			return finish(scorer.Result{}, scorer.AIUnscored, note)
		}

		if len(results) == 0 {
			// The model stopped without calling record_score (e.g. wrote its
			// answer as text). Nudge it once per turn; the turn limit still applies.
			results = append(results, llm.Block{Type: llm.BlockText,
				Text: "Please record your assessment by calling record_score."})
			logf("user: Please record your assessment by calling record_score.")
		}
		messages = append(messages, llm.Message{Role: llm.RoleUser, Blocks: results})
	}

	note := fmt.Sprintf("no valid record_score within %d turns", a.maxTurns)
	logf("stop: %s", note)
	return finish(scorer.Result{}, scorer.AIUnscored, note)
}

// oneLine squashes whitespace and truncates, for the transcript.
func oneLine(s string, max int) string {
	s = strings.Join(strings.Fields(s), " ")
	if r := []rune(s); len(r) > max {
		return string(r[:max]) + "…"
	}
	return s
}

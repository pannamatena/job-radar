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
	client     llm.Client
	model      string
	price      llm.Price // standard prices
	batchPrice llm.Price // discounted prices for batch runs
	system     string
	version    string
	tools      []llm.Tool
	tracks     []string
	flags      []string
	maxTurns   int
	maxTokens  int64
	companies  map[string]config.Company
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
	// Batch prices: the config lists batch_input/batch_output; cache prices scale
	// by the same discount (they stack on top of it). If a model has no batch
	// prices configured, assume the usual half price.
	bi, bo := mp.BatchInput, mp.BatchOutput
	if bi == 0 {
		bi = mp.Input / 2
	}
	if bo == 0 {
		bo = mp.Output / 2
	}
	ratio := 0.5
	if mp.Input > 0 {
		ratio = bi / mp.Input
	}
	return &Agent{
		client:     o.Client,
		model:      cfg.LLM.Model,
		price:      llm.Price{Input: mp.Input, Output: mp.Output, CacheWrite: mp.CacheWrite, CacheRead: mp.CacheRead},
		batchPrice: llm.Price{Input: bi, Output: bo, CacheWrite: mp.CacheWrite * ratio, CacheRead: mp.CacheRead * ratio},
		system:     system,
		version:    PromptVersion(system),
		tools:      toolDefs(tracks, flags),
		tracks:     tracks,
		flags:      flags,
		maxTurns:   cfg.LLM.MaxTurns,
		maxTokens:  int64(cfg.LLM.MaxTokensPerPosting),
		companies:  companies,
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
	s := a.NewSession(p)
	for {
		req, ok := s.Next()
		if !ok {
			return s.Result()
		}
		resp, err := a.client.Send(ctx, req)
		s.Apply(resp, err)
	}
}

// Session is one posting's scoring, advanced one model call at a time. Score
// drives it in a plain loop; the batch eval drives many sessions together, one
// round of requests per batch. The loop is the same either way:
//
//	for { req, ok := s.Next(); if !ok { break }; resp, err := send(req); s.Apply(resp, err) }
type Session struct {
	a        *Agent
	price    llm.Price
	env      toolEnv
	messages []llm.Message
	ai       *scorer.AIDetails
	usage    llm.Usage
	turn     int
	done     bool
	result   scorer.Result
}

// NewSession starts scoring a posting at standard prices.
func (a *Agent) NewSession(p posting.Posting) *Session { return a.newSession(p, a.price) }

// NewBatchSession is NewSession, but costed at the discounted batch prices.
func (a *Agent) NewBatchSession(p posting.Posting) *Session { return a.newSession(p, a.batchPrice) }

func (a *Agent) newSession(p posting.Posting, price llm.Price) *Session {
	s := &Session{
		a:     a,
		price: price,
		env:   toolEnv{p: p, company: a.companies[p.Company]},
		ai:    &scorer.AIDetails{Model: a.model, PromptVersion: a.version},
		messages: []llm.Message{{
			Role:   llm.RoleUser,
			Blocks: []llm.Block{{Type: llm.BlockText, Text: userKickoff(p.ID)}},
		}},
	}
	s.logf("user: %s", userKickoff(p.ID))
	return s
}

func (s *Session) logf(format string, args ...any) {
	s.ai.Transcript = append(s.ai.Transcript, fmt.Sprintf(format, args...))
}

func (s *Session) finish(r scorer.Result, status, note string) {
	ai := s.ai
	ai.Status, ai.Note = status, note
	ai.InputTokens, ai.OutputTokens = s.usage.InputTokens, s.usage.OutputTokens
	ai.CacheWriteTokens, ai.CacheReadTokens = s.usage.CacheWriteTokens, s.usage.CacheReadTokens
	ai.CostUSD = s.price.Cost(s.usage)
	r.AI = ai
	s.result, s.done = r, true
}

// Done reports whether the scoring has finished (scored, or given up).
func (s *Session) Done() bool { return s.done }

// Result is the outcome once Done; before that it is the zero value.
func (s *Session) Result() scorer.Result { return s.result }

// Next returns the request for the next model call, or false once the scoring
// has finished. Running out of turns finishes it, unscored.
func (s *Session) Next() (llm.Request, bool) {
	if s.done {
		return llm.Request{}, false
	}
	if s.turn >= s.a.maxTurns {
		note := fmt.Sprintf("no valid record_score within %d turns", s.a.maxTurns)
		s.logf("stop: %s", note)
		s.finish(scorer.Result{}, scorer.AIUnscored, note)
		return llm.Request{}, false
	}
	s.turn++
	s.ai.Turns = s.turn
	return llm.Request{
		Model:       s.a.model,
		MaxTokens:   outputTokensPerTurn,
		System:      s.a.system,
		CacheSystem: true,
		Tools:       s.a.tools,
		Messages:    s.messages,
	}, true
}

// Apply takes the model's reply to the request Next returned (or the error
// that stopped it), runs any tool calls and either finishes the scoring or
// prepares the next request.
func (s *Session) Apply(resp llm.Response, err error) {
	if s.done {
		return
	}
	a := s.a
	if err != nil {
		s.logf("error: %v", err)
		s.finish(scorer.Result{}, scorer.AIUnscored, "API error: "+err.Error())
		return
	}
	s.usage = s.usage.Add(resp.Usage)
	s.logf("model (turn %d, %d in / %d out / %d cache-read tokens, stop=%s)",
		s.turn, resp.Usage.InputTokens, resp.Usage.OutputTokens, resp.Usage.CacheReadTokens, resp.StopReason)

	// The conversation must include the model's reply exactly as sent,
	// tool calls and all, before our tool results. The API rejects a
	// message with no content, and the model occasionally returns an
	// empty reply, so stand in a short placeholder in that case.
	sent := resp.Blocks
	if len(sent) == 0 {
		sent = []llm.Block{{Type: llm.BlockText, Text: "(no reply)"}}
		s.logf("  (the model sent an empty reply)")
	}
	s.messages = append(s.messages, llm.Message{Role: llm.RoleAssistant, Blocks: sent})

	var results []llm.Block
	for _, b := range resp.Blocks {
		switch b.Type {
		case llm.BlockText:
			if t := strings.TrimSpace(b.Text); t != "" {
				s.logf("  says: %s", oneLine(t, 300))
			}
		case llm.BlockToolUse:
			s.logf("  calls %s %s", b.ToolName, oneLine(string(b.Input), 600))
			if b.ToolName == ToolRecordScore {
				r, verr := validateRecord(b.Input, a.tracks, a.flags)
				if verr == nil {
					s.logf("  → recorded score %d/5 (%s)", r.Score, r.Track)
					s.finish(r, scorer.AIScored, "")
					return
				}
				s.ai.SchemaFailures++
				s.logf("  → rejected: %v", verr)
				results = append(results, llm.Block{Type: llm.BlockToolResult, ToolUseID: b.ToolUseID, Text: verr.Error(), IsError: true})
				continue
			}
			out, terr := s.env.run(b.ToolName, b.Input)
			if terr != nil {
				s.logf("  → error: %v", terr)
				results = append(results, llm.Block{Type: llm.BlockToolResult, ToolUseID: b.ToolUseID, Text: terr.Error(), IsError: true})
				continue
			}
			s.logf("  → %s", oneLine(out, 200))
			results = append(results, llm.Block{Type: llm.BlockToolResult, ToolUseID: b.ToolUseID, Text: out})
		}
	}

	if s.usage.Total() > a.maxTokens {
		note := fmt.Sprintf("token budget exceeded (%d > %d)", s.usage.Total(), a.maxTokens)
		s.logf("stop: %s", note)
		s.finish(scorer.Result{}, scorer.AIUnscored, note)
		return
	}

	if len(results) == 0 {
		// The model stopped without calling record_score (e.g. wrote its
		// answer as text). Nudge it once per turn; the turn limit still applies.
		results = append(results, llm.Block{Type: llm.BlockText,
			Text: "Please record your assessment by calling record_score."})
		s.logf("user: Please record your assessment by calling record_score.")
	}
	s.messages = append(s.messages, llm.Message{Role: llm.RoleUser, Blocks: results})
}

// oneLine squashes whitespace and truncates, for the transcript.
func oneLine(s string, max int) string {
	s = strings.Join(strings.Fields(s), " ")
	if r := []rune(s); len(r) > max {
		return string(r[:max]) + "…"
	}
	return s
}

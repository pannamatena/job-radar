package eval

import (
	"context"
	"fmt"

	"github.com/pannamatena/job-radar/internal/llm"
	"github.com/pannamatena/job-radar/internal/posting"
	"github.com/pannamatena/job-radar/internal/scorer"
)

// Conversation is one scoring in progress, advanced a model call at a time.
// The agent's Session satisfies it. Eval only needs these three methods, so it
// doesn't have to import the agent.
type Conversation interface {
	Next() (req llm.Request, ok bool)
	Apply(resp llm.Response, err error)
	Result() scorer.Result
}

// BatchOptions are the extras RunBatch needs on top of Options.
type BatchOptions struct {
	Options
	// NewConversation starts a scoring of one posting.
	NewConversation func(p posting.Posting) Conversation
	// Round is called as each round of the batch starts (optional).
	Round func(round, active int)
	// Waiting is called while a batch runs, with how many of its requests have
	// finished (optional).
	Waiting func(round, done, total int)
}

// RunBatch is Run for the batch API. The agent needs several model calls per ad
// (read the ad, look something up, record the score), and a batch is a set of
// independent requests, so the work goes in rounds: each round sends one
// request for every scoring that isn't finished, waits for the batch, feeds the
// replies back, and repeats until every scoring is done.
//
// It is cheaper (about half price) but slower: a round can take minutes, so an
// eval that needs three rounds takes a while. Latency isn't measured.
func RunBatch(ctx context.Context, cases []Case, rules ScoreFunc, client llm.BatchClient, o BatchOptions) (Results, error) {
	if o.Repeats < 1 {
		o.Repeats = 1
	}
	res := Results{
		Version: ResultsVersion, Date: o.Now.Format("2006-01-02"),
		Scorer: o.Scorer, Model: o.Model, PromptVersion: o.PromptVersion,
		Repeats: o.Repeats, AlertThreshold: o.AlertThreshold, GateMin: o.GateMin, Batch: true,
	}

	type slot struct {
		caseIdx int
		conv    Conversation
	}
	var slots []slot
	for _, c := range cases {
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
				continue
			}
		}
		cr.RulesScore = rules(ctx, p).Score
		res.Cases = append(res.Cases, cr)
		for r := 0; r < o.Repeats; r++ {
			slots = append(slots, slot{caseIdx: len(res.Cases) - 1, conv: o.NewConversation(p)})
		}
	}

	for round := 1; ; round++ {
		var reqs []llm.BatchRequest
		var active []int // index into slots for each request, in order
		for i, s := range slots {
			if req, ok := s.conv.Next(); ok {
				reqs = append(reqs, llm.BatchRequest{ID: fmt.Sprintf("c%d", i), Request: req})
				active = append(active, i)
			}
		}
		if len(reqs) == 0 {
			break
		}
		if o.Round != nil {
			o.Round(round, len(reqs))
		}
		results, err := client.SendBatch(ctx, reqs, func(done, total int) {
			if o.Waiting != nil {
				o.Waiting(round, done, total)
			}
		})
		if err != nil {
			return res, fmt.Errorf("batch round %d: %w", round, err)
		}
		for k, r := range results {
			if r.Err != nil {
				slots[active[k]].conv.Apply(llm.Response{}, r.Err)
				continue
			}
			slots[active[k]].conv.Apply(r.Response, nil)
		}
	}

	for _, s := range slots {
		cr := &res.Cases[s.caseIdx]
		cr.Attempts = append(cr.Attempts, attemptFrom(s.conv.Result(), 0))
	}
	return res, nil
}

package llm

import (
	"context"
	"fmt"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
)

// defaultPollInterval is how often a running batch is checked. Batches usually
// finish within the hour, so checking every 30 seconds is plenty.
const defaultPollInterval = 30 * time.Second

// SendBatch submits the requests as one Message Batch, waits for it to end and
// returns one result per request, in the order they were given (the API returns
// them in any order, so they're matched by id). If the context is cancelled
// (Ctrl-C), it asks the API to cancel the batch so nothing keeps running.
func (a *Anthropic) SendBatch(ctx context.Context, reqs []BatchRequest, progress func(done, total int)) ([]BatchResult, error) {
	if len(reqs) == 0 {
		return nil, nil
	}
	var items []anthropic.MessageBatchNewParamsRequest
	for _, r := range reqs {
		p, err := buildParams(r.Request)
		if err != nil {
			return nil, fmt.Errorf("batch request %s: %w", r.ID, err)
		}
		items = append(items, anthropic.MessageBatchNewParamsRequest{
			CustomID: r.ID,
			Params: anthropic.MessageBatchNewParamsRequestParams{
				Model:     p.Model,
				MaxTokens: p.MaxTokens,
				System:    p.System,
				Tools:     p.Tools,
				Messages:  p.Messages,
			},
		})
	}

	batch, err := a.client.Messages.Batches.New(ctx, anthropic.MessageBatchNewParams{Requests: items})
	if err != nil {
		return nil, fmt.Errorf("anthropic: creating batch: %w", err)
	}

	interval := a.pollInterval
	if interval <= 0 {
		interval = defaultPollInterval
	}
	for batch.ProcessingStatus != anthropic.MessageBatchProcessingStatusEnded {
		select {
		case <-ctx.Done():
			a.cancelBatch(batch.ID)
			return nil, ctx.Err()
		case <-time.After(interval):
		}
		// Keep the id: on error Get returns a nil batch, and the error message
		// below still needs it.
		id := batch.ID
		next, err := a.client.Messages.Batches.Get(ctx, id, anthropic.MessageBatchGetParams{})
		if err != nil {
			if ctx.Err() != nil {
				a.cancelBatch(id) // we were stopped while checking: don't leave it running
				return nil, ctx.Err()
			}
			return nil, fmt.Errorf("anthropic: checking batch %s: %w", id, err)
		}
		batch = next
		if progress != nil {
			c := batch.RequestCounts
			progress(int(c.Succeeded+c.Errored+c.Canceled+c.Expired), len(reqs))
		}
	}

	byID := map[string]BatchResult{}
	stream := a.client.Messages.Batches.ResultsStreaming(ctx, batch.ID, anthropic.MessageBatchResultsParams{})
	defer stream.Close()
	for stream.Next() {
		item := stream.Current()
		res := BatchResult{ID: item.CustomID}
		switch v := item.Result.AsAny().(type) {
		case anthropic.MessageBatchSucceededResult:
			res.Response = fromMessage(&v.Message)
		case anthropic.MessageBatchErroredResult:
			res.Err = fmt.Errorf("anthropic: batched request errored: %s", v.Error.Error.Message)
		case anthropic.MessageBatchCanceledResult:
			res.Err = fmt.Errorf("anthropic: batched request was cancelled")
		case anthropic.MessageBatchExpiredResult:
			res.Err = fmt.Errorf("anthropic: batched request expired before it ran")
		default:
			res.Err = fmt.Errorf("anthropic: unrecognised batch result")
		}
		byID[res.ID] = res
	}
	if err := stream.Err(); err != nil {
		return nil, fmt.Errorf("anthropic: reading batch results: %w", err)
	}

	out := make([]BatchResult, 0, len(reqs))
	for _, r := range reqs {
		res, ok := byID[r.ID]
		if !ok {
			res = BatchResult{ID: r.ID, Err: fmt.Errorf("anthropic: no result came back for %s", r.ID)}
		}
		out = append(out, res)
	}
	return out, nil
}

// cancelBatch asks the server to stop a batch so it doesn't keep running (and
// costing money) after we have given up on it. It uses a fresh context because
// the caller's is usually the one that was cancelled. Failure is ignored: there
// is nothing more we can do about it.
func (a *Anthropic) cancelBatch(id string) {
	cctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	_, _ = a.client.Messages.Batches.Cancel(cctx, id, anthropic.MessageBatchCancelParams{})
}

package llm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

// EnvAPIKey is where the Anthropic API key is read from (normally via the
// git-ignored .env file).
const EnvAPIKey = "ANTHROPIC_API_KEY"

// ErrNoAPIKey means the AI scorer was asked for but no key is configured.
var ErrNoAPIKey = errors.New("no Anthropic API key: put ANTHROPIC_API_KEY=… in your .env file (see docs/ai-scorer.md)")

// Anthropic implements Client with the official Go SDK. It translates our
// neutral types to the SDK's and back — that translation is all it does.
type Anthropic struct {
	client anthropic.Client
}

// NewAnthropic builds a client from ANTHROPIC_API_KEY. The SDK retries
// rate-limit and server errors itself (2 retries with backoff by default).
func NewAnthropic() (*Anthropic, error) {
	key := os.Getenv(EnvAPIKey)
	if key == "" {
		return nil, ErrNoAPIKey
	}
	return &Anthropic{client: anthropic.NewClient(option.WithAPIKey(key))}, nil
}

// Send makes one Messages API call.
func (a *Anthropic) Send(ctx context.Context, req Request) (Response, error) {
	params := anthropic.MessageNewParams{
		Model:     anthropic.Model(req.Model),
		MaxTokens: int64(req.MaxTokens),
	}

	if req.System != "" {
		sys := anthropic.TextBlockParam{Text: req.System}
		if req.CacheSystem {
			// A cache breakpoint on the system block caches tools + system
			// (they're cached in that order), which is exactly the part that's
			// identical for every posting.
			sys.CacheControl = anthropic.NewCacheControlEphemeralParam()
		}
		params.System = []anthropic.TextBlockParam{sys}
	}

	for _, t := range req.Tools {
		tp := anthropic.ToolParam{
			Name:        t.Name,
			Description: anthropic.String(t.Description),
			InputSchema: toInputSchema(t.Schema),
		}
		if t.Strict {
			tp.Strict = anthropic.Bool(true)
		}
		params.Tools = append(params.Tools, anthropic.ToolUnionParam{OfTool: &tp})
	}

	for _, m := range req.Messages {
		mp, err := toMessageParam(m)
		if err != nil {
			return Response{}, err
		}
		params.Messages = append(params.Messages, mp)
	}

	msg, err := a.client.Messages.New(ctx, params)
	if err != nil {
		return Response{}, fmt.Errorf("anthropic: %w", err)
	}
	return fromMessage(msg), nil
}

// toInputSchema splits our plain map schema into the SDK's typed fields.
func toInputSchema(schema map[string]any) anthropic.ToolInputSchemaParam {
	s := anthropic.ToolInputSchemaParam{ExtraFields: map[string]any{}}
	for k, v := range schema {
		switch k {
		case "type":
			// always "object"; the SDK fills it in
		case "properties":
			s.Properties = v
		case "required":
			if req, ok := v.([]string); ok {
				s.Required = req
			}
		default:
			s.ExtraFields[k] = v // e.g. additionalProperties: false
		}
	}
	return s
}

func toMessageParam(m Message) (anthropic.MessageParam, error) {
	var blocks []anthropic.ContentBlockParamUnion
	for _, b := range m.Blocks {
		switch b.Type {
		case BlockText:
			blocks = append(blocks, anthropic.NewTextBlock(b.Text))
		case BlockToolUse:
			var input any = json.RawMessage(b.Input)
			if len(b.Input) == 0 {
				input = map[string]any{}
			}
			blocks = append(blocks, anthropic.NewToolUseBlock(b.ToolUseID, input, b.ToolName))
		case BlockToolResult:
			blocks = append(blocks, anthropic.NewToolResultBlock(b.ToolUseID, b.Text, b.IsError))
		default:
			return anthropic.MessageParam{}, fmt.Errorf("llm: unknown block type %q", b.Type)
		}
	}
	if m.Role == RoleAssistant {
		return anthropic.NewAssistantMessage(blocks...), nil
	}
	return anthropic.NewUserMessage(blocks...), nil
}

func fromMessage(msg *anthropic.Message) Response {
	r := Response{
		StopReason: StopReason(msg.StopReason),
		Usage: Usage{
			InputTokens:      msg.Usage.InputTokens,
			OutputTokens:     msg.Usage.OutputTokens,
			CacheWriteTokens: msg.Usage.CacheCreationInputTokens,
			CacheReadTokens:  msg.Usage.CacheReadInputTokens,
		},
	}
	for _, c := range msg.Content {
		switch c.Type {
		case "text":
			r.Blocks = append(r.Blocks, Block{Type: BlockText, Text: c.Text})
		case "tool_use":
			r.Blocks = append(r.Blocks, Block{Type: BlockToolUse, ToolUseID: c.ID, ToolName: c.Name, Input: c.Input})
		}
		// Other block types (thinking, server tools) aren't used by job-radar.
	}
	return r
}

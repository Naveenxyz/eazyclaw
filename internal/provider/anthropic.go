package provider

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

// AnthropicProvider implements Provider using the Anthropic Messages API.
type AnthropicProvider struct {
	client *anthropic.Client
	apiKey string
	model  string
	name   string
}

// NewAnthropicProvider creates a new Anthropic provider.
func NewAnthropicProvider(apiKey, model string) *AnthropicProvider {
	client := anthropic.NewClient(
		option.WithAPIKey(apiKey),
	)
	return &AnthropicProvider{
		client: &client,
		apiKey: apiKey,
		model:  model,
		name:   "anthropic",
	}
}

// NewAnthropicCompatProvider creates a provider using the Anthropic Messages API
// with a custom base URL and provider name (e.g., kimi-coding).
func NewAnthropicCompatProvider(name, apiKey, model, baseURL string) *AnthropicProvider {
	client := anthropic.NewClient(
		option.WithAPIKey(apiKey),
		option.WithBaseURL(baseURL),
	)
	return &AnthropicProvider{
		client: &client,
		apiKey: apiKey,
		model:  model,
		name:   name,
	}
}

// Name returns the provider name.
func (p *AnthropicProvider) Name() string {
	return p.name
}

// ChatCompletion sends a synchronous chat completion request to Anthropic.
func (p *AnthropicProvider) ChatCompletion(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
	params := p.buildParams(req)

	msg, err := p.client.Messages.New(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("anthropic chat completion: %w", err)
	}

	return p.mapResponse(msg), nil
}

// ChatCompletionStream sends a streaming chat completion request to Anthropic.
func (p *AnthropicProvider) ChatCompletionStream(ctx context.Context, req *ChatRequest) (<-chan StreamEvent, error) {
	params := p.buildParams(req)
	ch := make(chan StreamEvent, 64)

	stream := p.client.Messages.NewStreaming(ctx, params)

	go func() {
		defer close(ch)

		accumulated := anthropic.Message{}
		for stream.Next() {
			event := stream.Current()
			if err := accumulated.Accumulate(event); err != nil {
				ch <- StreamEvent{Type: "error", Error: err}
				return
			}

			switch ev := event.AsAny().(type) {
			case anthropic.ContentBlockDeltaEvent:
				switch delta := ev.Delta.AsAny().(type) {
				case anthropic.TextDelta:
					ch <- StreamEvent{Type: "content", Content: delta.Text}
				case anthropic.InputJSONDelta:
					// Input JSON deltas are accumulated; we emit tool_call on block stop.
					_ = delta
				}
			case anthropic.ContentBlockStopEvent:
				// Check if the completed block is a tool use block.
				idx := ev.Index
				if int(idx) < len(accumulated.Content) {
					block := accumulated.Content[idx]
					if tu, ok := block.AsAny().(anthropic.ToolUseBlock); ok {
						argsJSON, _ := json.Marshal(tu.Input)
						ch <- StreamEvent{
							Type: "tool_call",
							ToolCall: &ToolCall{
								ID:        tu.ID,
								Name:      tu.Name,
								Arguments: argsJSON,
							},
						}
					}
				}
			case anthropic.MessageStopEvent:
				ch <- StreamEvent{Type: "done"}
			}
		}

		if err := stream.Err(); err != nil {
			ch <- StreamEvent{Type: "error", Error: err}
		}
	}()

	return ch, nil
}

// buildParams maps a unified ChatRequest to Anthropic MessageNewParams.
func (p *AnthropicProvider) buildParams(req *ChatRequest) anthropic.MessageNewParams {
	model := req.Model
	if model == "" {
		model = p.model
	}

	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = 4096
	}

	// Map messages.
	messages := make([]anthropic.MessageParam, 0, len(req.Messages))
	for _, m := range req.Messages {
		switch m.Role {
		case "user":
			messages = append(messages, anthropic.NewUserMessage(
				anthropic.NewTextBlock(m.Content),
			))
		case "assistant":
			if len(m.ToolCalls) > 0 {
				blocks := make([]anthropic.ContentBlockParamUnion, 0)
				if m.Content != "" {
					blocks = append(blocks, anthropic.NewTextBlock(m.Content))
				}
				for _, tc := range m.ToolCalls {
					blocks = append(blocks, anthropic.ContentBlockParamUnion{
						OfToolUse: &anthropic.ToolUseBlockParam{
							ID:    tc.ID,
							Name:  tc.Name,
							Input: json.RawMessage(tc.Arguments),
						},
					})
				}
				messages = append(messages, anthropic.NewAssistantMessage(blocks...))
			} else {
				messages = append(messages, anthropic.NewAssistantMessage(
					anthropic.NewTextBlock(m.Content),
				))
			}
		case "tool":
			messages = append(messages, anthropic.NewUserMessage(
				anthropic.NewToolResultBlock(m.ToolCallID, m.Content, false),
			))
		}
	}

	params := anthropic.MessageNewParams{
		Model:     anthropic.Model(model),
		MaxTokens: int64(maxTokens),
		Messages:  messages,
	}

	// System prompt.
	if req.SystemPrompt != "" {
		params.System = []anthropic.TextBlockParam{
			{Text: req.SystemPrompt},
		}
	}

	// Temperature.
	if req.Temperature > 0 {
		params.Temperature = anthropic.Float(req.Temperature)
	}

	// Tools.
	if len(req.Tools) > 0 {
		tools := make([]anthropic.ToolUnionParam, 0, len(req.Tools))
		for _, t := range req.Tools {
			schema := anthropic.ToolInputSchemaParam{}
			if len(t.Parameters) > 0 {
				var schemaMap map[string]interface{}
				if err := json.Unmarshal(t.Parameters, &schemaMap); err == nil {
					if props, ok := schemaMap["properties"]; ok {
						if propsOrdered, ok := props.(map[string]interface{}); ok {
							schema.Properties = propsOrdered
						}
					}
				}
			}
			tools = append(tools, anthropic.ToolUnionParam{
				OfTool: &anthropic.ToolParam{
					Name:        t.Name,
					Description: anthropic.String(t.Description),
					InputSchema: schema,
				},
			})
		}
		params.Tools = tools
	}

	return params
}

// mapResponse maps an Anthropic Message to a unified ChatResponse.
func (p *AnthropicProvider) mapResponse(msg *anthropic.Message) *ChatResponse {
	resp := &ChatResponse{
		Usage: Usage{
			InputTokens:  int(msg.Usage.InputTokens),
			OutputTokens: int(msg.Usage.OutputTokens),
		},
		StopReason: string(msg.StopReason),
	}

	for _, block := range msg.Content {
		switch b := block.AsAny().(type) {
		case anthropic.TextBlock:
			resp.Content += b.Text
		case anthropic.ToolUseBlock:
			argsJSON, _ := json.Marshal(b.Input)
			resp.ToolCalls = append(resp.ToolCalls, ToolCall{
				ID:        b.ID,
				Name:      b.Name,
				Arguments: argsJSON,
			})
		}
	}

	return resp
}

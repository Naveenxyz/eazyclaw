package provider

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/openai/openai-go/shared"
)

// OpenAICompatProvider implements Provider using an OpenAI-compatible API.
// It works with OpenAI, Moonshot, Zhipu, and any other OpenAI-compatible endpoint.
type OpenAICompatProvider struct {
	client  *openai.Client
	apiKey  string
	model   string
	baseURL string
	name    string
}

// NewOpenAICompatProvider creates a new OpenAI-compatible provider.
func NewOpenAICompatProvider(name, apiKey, model, baseURL string) *OpenAICompatProvider {
	client := openai.NewClient(
		option.WithAPIKey(apiKey),
		option.WithBaseURL(baseURL),
	)
	return &OpenAICompatProvider{
		client:  &client,
		apiKey:  apiKey,
		model:   model,
		baseURL: baseURL,
		name:    name,
	}
}

// Name returns the provider name.
func (p *OpenAICompatProvider) Name() string {
	return p.name
}

// ChatCompletion sends a synchronous chat completion request.
func (p *OpenAICompatProvider) ChatCompletion(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
	params := p.buildParams(req)

	completion, err := p.client.Chat.Completions.New(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("%s chat completion: %w", p.name, err)
	}

	return p.mapResponse(completion), nil
}

// ChatCompletionStream sends a streaming chat completion request.
func (p *OpenAICompatProvider) ChatCompletionStream(ctx context.Context, req *ChatRequest) (<-chan StreamEvent, error) {
	params := p.buildParams(req)
	ch := make(chan StreamEvent, 64)

	stream := p.client.Chat.Completions.NewStreaming(ctx, params)

	go func() {
		defer close(ch)

		acc := openai.ChatCompletionAccumulator{}

		for stream.Next() {
			chunk := stream.Current()
			acc.AddChunk(chunk)

			// Emit completed tool calls.
			if tc, ok := acc.JustFinishedToolCall(); ok {
				ch <- StreamEvent{
					Type: "tool_call",
					ToolCall: &ToolCall{
						ID:        tc.ID,
						Name:      tc.Name,
						Arguments: json.RawMessage(tc.Arguments),
					},
				}
			}

			// Emit content deltas.
			if len(chunk.Choices) > 0 {
				delta := chunk.Choices[0].Delta
				if delta.Content != "" {
					ch <- StreamEvent{
						Type:    "content",
						Content: delta.Content,
					}
				}

				// Check for finish reason.
				fr := chunk.Choices[0].FinishReason
				if fr == "stop" || fr == "tool_calls" {
					ch <- StreamEvent{Type: "done"}
				}
			}
		}

		if err := stream.Err(); err != nil {
			ch <- StreamEvent{Type: "error", Error: err}
		}
	}()

	return ch, nil
}

// buildParams maps a unified ChatRequest to OpenAI ChatCompletionNewParams.
func (p *OpenAICompatProvider) buildParams(req *ChatRequest) openai.ChatCompletionNewParams {
	model := req.Model
	if model == "" {
		model = p.model
	}

	// Build messages.
	messages := make([]openai.ChatCompletionMessageParamUnion, 0, len(req.Messages)+1)

	// System prompt as a developer/system message.
	if req.SystemPrompt != "" {
		messages = append(messages, openai.DeveloperMessage(req.SystemPrompt))
	}

	for _, m := range req.Messages {
		switch m.Role {
		case "user":
			messages = append(messages, openai.UserMessage(m.Content))
		case "assistant":
			if len(m.ToolCalls) > 0 {
				toolCalls := make([]openai.ChatCompletionMessageToolCallParam, 0, len(m.ToolCalls))
				for _, tc := range m.ToolCalls {
					toolCalls = append(toolCalls, openai.ChatCompletionMessageToolCallParam{
						ID: tc.ID,
						Function: openai.ChatCompletionMessageToolCallFunctionParam{
							Name:      tc.Name,
							Arguments: string(tc.Arguments),
						},
					})
				}
				messages = append(messages, openai.ChatCompletionMessageParamUnion{
					OfAssistant: &openai.ChatCompletionAssistantMessageParam{
						Content:   openai.ChatCompletionAssistantMessageParamContentUnion{OfString: openai.String(m.Content)},
						ToolCalls: toolCalls,
					},
				})
			} else {
				messages = append(messages, openai.AssistantMessage(m.Content))
			}
		case "tool":
			messages = append(messages, openai.ToolMessage(m.Content, m.ToolCallID))
		}
	}

	params := openai.ChatCompletionNewParams{
		Model:    openai.ChatModel(model),
		Messages: messages,
	}

	// Max tokens.
	if req.MaxTokens > 0 {
		params.MaxCompletionTokens = openai.Int(int64(req.MaxTokens))
	}

	// Temperature.
	if req.Temperature > 0 {
		params.Temperature = openai.Float(req.Temperature)
	}

	// Tools.
	if len(req.Tools) > 0 {
		tools := make([]openai.ChatCompletionToolParam, 0, len(req.Tools))
		for _, t := range req.Tools {
			var paramsMap map[string]interface{}
			if len(t.Parameters) > 0 {
				_ = json.Unmarshal(t.Parameters, &paramsMap)
			}
			if paramsMap == nil {
				paramsMap = map[string]interface{}{"type": "object"}
			}
			tools = append(tools, openai.ChatCompletionToolParam{
				Function: shared.FunctionDefinitionParam{
					Name:        t.Name,
					Description: openai.String(t.Description),
					Parameters:  shared.FunctionParameters(paramsMap),
				},
			})
		}
		params.Tools = tools
	}

	return params
}

// mapResponse maps an OpenAI ChatCompletion to a unified ChatResponse.
func (p *OpenAICompatProvider) mapResponse(completion *openai.ChatCompletion) *ChatResponse {
	resp := &ChatResponse{
		Usage: Usage{
			InputTokens:  int(completion.Usage.PromptTokens),
			OutputTokens: int(completion.Usage.CompletionTokens),
		},
	}

	if len(completion.Choices) > 0 {
		choice := completion.Choices[0]
		resp.Content = choice.Message.Content
		resp.StopReason = choice.FinishReason

		for _, tc := range choice.Message.ToolCalls {
			resp.ToolCalls = append(resp.ToolCalls, ToolCall{
				ID:        tc.ID,
				Name:      tc.Function.Name,
				Arguments: json.RawMessage(tc.Function.Arguments),
			})
		}
	}

	return resp
}

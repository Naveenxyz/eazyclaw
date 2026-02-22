package provider

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
)

// GeminiProvider implements Provider using the Google Gemini API.
type GeminiProvider struct {
	apiKey string
	model  string
}

// NewGeminiProvider creates a new Gemini provider.
func NewGeminiProvider(apiKey, model string) *GeminiProvider {
	return &GeminiProvider{
		apiKey: apiKey,
		model:  model,
	}
}

// Name returns the provider name.
func (p *GeminiProvider) Name() string {
	return "gemini"
}

// newClient creates a new genai client. It must be closed by the caller.
func (p *GeminiProvider) newClient(ctx context.Context) (*genai.Client, error) {
	client, err := genai.NewClient(ctx, option.WithAPIKey(p.apiKey))
	if err != nil {
		return nil, fmt.Errorf("gemini: failed to create client: %w", err)
	}
	return client, nil
}

// ChatCompletion sends a synchronous chat completion request to Gemini.
func (p *GeminiProvider) ChatCompletion(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
	client, err := p.newClient(ctx)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	model := p.configureModel(client, req)

	// Build content parts from messages.
	contents := p.buildContents(req.Messages)

	// Flatten the last user message parts for GenerateContent; use chat for multi-turn.
	var resp *genai.GenerateContentResponse

	if len(contents) == 1 {
		resp, err = model.GenerateContent(ctx, contents[0].Parts...)
	} else {
		cs := model.StartChat()
		// Set history to all but the last message.
		cs.History = contents[:len(contents)-1]
		lastContent := contents[len(contents)-1]
		resp, err = cs.SendMessage(ctx, lastContent.Parts...)
	}
	if err != nil {
		return nil, fmt.Errorf("gemini chat completion: %w", err)
	}

	return p.mapResponse(resp), nil
}

// ChatCompletionStream sends a streaming chat completion request to Gemini.
func (p *GeminiProvider) ChatCompletionStream(ctx context.Context, req *ChatRequest) (<-chan StreamEvent, error) {
	client, err := p.newClient(ctx)
	if err != nil {
		return nil, err
	}

	model := p.configureModel(client, req)
	contents := p.buildContents(req.Messages)

	ch := make(chan StreamEvent, 64)

	go func() {
		defer close(ch)
		defer client.Close()

		var iter *genai.GenerateContentResponseIterator

		if len(contents) == 1 {
			iter = model.GenerateContentStream(ctx, contents[0].Parts...)
		} else {
			cs := model.StartChat()
			cs.History = contents[:len(contents)-1]
			lastContent := contents[len(contents)-1]
			iter = cs.SendMessageStream(ctx, lastContent.Parts...)
		}

		for {
			resp, err := iter.Next()
			if err == iterator.Done {
				ch <- StreamEvent{Type: "done"}
				break
			}
			if err != nil {
				ch <- StreamEvent{Type: "error", Error: err}
				return
			}

			for _, cand := range resp.Candidates {
				if cand.Content == nil {
					continue
				}
				for _, part := range cand.Content.Parts {
					switch v := part.(type) {
					case genai.Text:
						ch <- StreamEvent{
							Type:    "content",
							Content: string(v),
						}
					case genai.FunctionCall:
						argsJSON, _ := json.Marshal(v.Args)
						ch <- StreamEvent{
							Type: "tool_call",
							ToolCall: &ToolCall{
								ID:        v.Name, // Gemini doesn't provide a separate ID.
								Name:      v.Name,
								Arguments: argsJSON,
							},
						}
					}
				}
			}
		}
	}()

	return ch, nil
}

// configureModel creates and configures a GenerativeModel from the request.
func (p *GeminiProvider) configureModel(client *genai.Client, req *ChatRequest) *genai.GenerativeModel {
	modelName := req.Model
	if modelName == "" {
		modelName = p.model
	}

	model := client.GenerativeModel(modelName)

	// System instruction.
	if req.SystemPrompt != "" {
		model.SystemInstruction = genai.NewUserContent(genai.Text(req.SystemPrompt))
	}

	// Temperature.
	if req.Temperature > 0 {
		model.Temperature = genai.Ptr(float32(req.Temperature))
	}

	// Max tokens.
	if req.MaxTokens > 0 {
		model.MaxOutputTokens = genai.Ptr(int32(req.MaxTokens))
	}

	// Tools.
	if len(req.Tools) > 0 {
		funcDecls := make([]*genai.FunctionDeclaration, 0, len(req.Tools))
		for _, t := range req.Tools {
			fd := &genai.FunctionDeclaration{
				Name:        t.Name,
				Description: t.Description,
			}
			if len(t.Parameters) > 0 {
				fd.Parameters = p.jsonSchemaToGenaiSchema(t.Parameters)
			}
			funcDecls = append(funcDecls, fd)
		}
		model.Tools = []*genai.Tool{
			{FunctionDeclarations: funcDecls},
		}
	}

	return model
}

// buildContents converts unified Messages to Gemini Content slices.
func (p *GeminiProvider) buildContents(messages []Message) []*genai.Content {
	contents := make([]*genai.Content, 0, len(messages))
	for _, m := range messages {
		var role string
		switch m.Role {
		case "user", "tool":
			role = "user"
		case "assistant":
			role = "model"
		default:
			role = m.Role
		}

		var parts []genai.Part

		if m.Role == "tool" {
			// Tool results are sent as FunctionResponse parts.
			var result map[string]any
			if err := json.Unmarshal([]byte(m.Content), &result); err != nil {
				result = map[string]any{"result": m.Content}
			}
			parts = append(parts, genai.FunctionResponse{
				Name:     m.ToolCallID,
				Response: result,
			})
		} else if len(m.ToolCalls) > 0 {
			// Assistant messages with tool calls.
			if m.Content != "" {
				parts = append(parts, genai.Text(m.Content))
			}
			for _, tc := range m.ToolCalls {
				var args map[string]interface{}
				_ = json.Unmarshal(tc.Arguments, &args)
				parts = append(parts, genai.FunctionCall{
					Name: tc.Name,
					Args: args,
				})
			}
		} else {
			parts = append(parts, genai.Text(m.Content))
		}

		contents = append(contents, &genai.Content{
			Role:  role,
			Parts: parts,
		})
	}
	return contents
}

// mapResponse maps a Gemini GenerateContentResponse to a unified ChatResponse.
func (p *GeminiProvider) mapResponse(resp *genai.GenerateContentResponse) *ChatResponse {
	cr := &ChatResponse{}

	if resp.UsageMetadata != nil {
		cr.Usage = Usage{
			InputTokens:  int(resp.UsageMetadata.PromptTokenCount),
			OutputTokens: int(resp.UsageMetadata.CandidatesTokenCount),
		}
	}

	for _, cand := range resp.Candidates {
		if cand.Content == nil {
			continue
		}
		if cand.FinishReason > 0 {
			cr.StopReason = cand.FinishReason.String()
		}
		for _, part := range cand.Content.Parts {
			switch v := part.(type) {
			case genai.Text:
				cr.Content += string(v)
			case genai.FunctionCall:
				argsJSON, _ := json.Marshal(v.Args)
				cr.ToolCalls = append(cr.ToolCalls, ToolCall{
					ID:        v.Name,
					Name:      v.Name,
					Arguments: argsJSON,
				})
			}
		}
	}

	return cr
}

// jsonSchemaToGenaiSchema converts a JSON Schema (as json.RawMessage) to a genai.Schema.
func (p *GeminiProvider) jsonSchemaToGenaiSchema(raw json.RawMessage) *genai.Schema {
	var schemaMap map[string]interface{}
	if err := json.Unmarshal(raw, &schemaMap); err != nil {
		return nil
	}
	return mapToGenaiSchema(schemaMap)
}

// mapToGenaiSchema recursively converts a JSON Schema map to a genai.Schema.
func mapToGenaiSchema(m map[string]interface{}) *genai.Schema {
	if m == nil {
		return nil
	}

	schema := &genai.Schema{}

	if t, ok := m["type"].(string); ok {
		switch t {
		case "object":
			schema.Type = genai.TypeObject
		case "string":
			schema.Type = genai.TypeString
		case "number":
			schema.Type = genai.TypeNumber
		case "integer":
			schema.Type = genai.TypeInteger
		case "boolean":
			schema.Type = genai.TypeBoolean
		case "array":
			schema.Type = genai.TypeArray
		}
	}

	if desc, ok := m["description"].(string); ok {
		schema.Description = desc
	}

	if props, ok := m["properties"].(map[string]interface{}); ok {
		schema.Properties = make(map[string]*genai.Schema)
		for k, v := range props {
			if propMap, ok := v.(map[string]interface{}); ok {
				schema.Properties[k] = mapToGenaiSchema(propMap)
			}
		}
	}

	if req, ok := m["required"].([]interface{}); ok {
		for _, r := range req {
			if s, ok := r.(string); ok {
				schema.Required = append(schema.Required, s)
			}
		}
	}

	if items, ok := m["items"].(map[string]interface{}); ok {
		schema.Items = mapToGenaiSchema(items)
	}

	if enumVals, ok := m["enum"].([]interface{}); ok {
		for _, e := range enumVals {
			if s, ok := e.(string); ok {
				schema.Enum = append(schema.Enum, s)
			}
		}
	}

	return schema
}

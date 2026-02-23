package agent

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/cloudwego/eino/components/model"
	einotool "github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	"github.com/eino-contrib/jsonschema"

	providerPkg "github.com/eazyclaw/eazyclaw/internal/provider"
	"github.com/eazyclaw/eazyclaw/internal/tool"
)

// ---------------------------------------------------------------------------
// Model adapter: wraps provider.Provider as eino model.ToolCallingChatModel
// ---------------------------------------------------------------------------

// einoModelAdapter wraps an EazyClaw Provider so eino's react agent can use it.
type einoModelAdapter struct {
	prov      providerPkg.Provider
	modelName string
	tools     []*schema.ToolInfo
}

var _ model.ToolCallingChatModel = (*einoModelAdapter)(nil)

func newEinoModelAdapter(prov providerPkg.Provider, modelName string) *einoModelAdapter {
	return &einoModelAdapter{prov: prov, modelName: modelName}
}

// Generate calls the underlying provider synchronously.
func (a *einoModelAdapter) Generate(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	systemPrompt, messages := extractSystemPrompt(input)
	toolDefs := einoToolInfoToProviderDefs(a.tools)

	req := &providerPkg.ChatRequest{
		Model:        a.modelName,
		Messages:     toProviderMessages(messages),
		Tools:        toolDefs,
		SystemPrompt: systemPrompt,
	}

	resp, err := a.prov.ChatCompletion(ctx, req)
	if err != nil {
		return nil, err
	}

	return providerResponseToEinoMessage(resp), nil
}

// Stream falls back to non-streaming Generate, wrapping the result as a single-item stream.
func (a *einoModelAdapter) Stream(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	msg, err := a.Generate(ctx, input, opts...)
	if err != nil {
		return nil, err
	}
	return schema.StreamReaderFromArray([]*schema.Message{msg}), nil
}

// WithTools returns a new adapter instance with the given tools bound.
// This satisfies the ToolCallingChatModel interface without mutating state.
func (a *einoModelAdapter) WithTools(tools []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	return &einoModelAdapter{
		prov:      a.prov,
		modelName: a.modelName,
		tools:     tools,
	}, nil
}

// ---------------------------------------------------------------------------
// Tool adapter: wraps tool.Tool as eino InvokableTool
// ---------------------------------------------------------------------------

// einoToolAdapter wraps an EazyClaw tool as an eino InvokableTool.
type einoToolAdapter struct {
	t tool.Tool
}

var _ einotool.InvokableTool = (*einoToolAdapter)(nil)

// Info returns the tool metadata for eino.
func (a *einoToolAdapter) Info(_ context.Context) (*schema.ToolInfo, error) {
	info := &schema.ToolInfo{
		Name: a.t.Name(),
		Desc: a.t.Description(),
	}
	params := a.t.Parameters()
	if len(params) > 0 {
		var js jsonschema.Schema
		if err := json.Unmarshal(params, &js); err == nil {
			info.ParamsOneOf = schema.NewParamsOneOfByJSONSchema(&js)
		}
	}
	return info, nil
}

// InvokableRun executes the tool with JSON arguments.
func (a *einoToolAdapter) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...einotool.Option) (string, error) {
	result, err := a.t.Execute(ctx, json.RawMessage(argumentsInJSON))
	if err != nil {
		return "", err
	}
	if result.IsError && result.Error != "" {
		return "Error: " + result.Error + "\n" + result.Content, nil
	}
	return result.Content, nil
}

// wrapToolsForEino converts all tools in an EazyClaw registry to eino InvokableTools.
func wrapToolsForEino(reg *tool.Registry) []einotool.InvokableTool {
	names := reg.List()
	adapters := make([]einotool.InvokableTool, 0, len(names))
	for _, name := range names {
		t, ok := reg.Get(name)
		if !ok {
			continue
		}
		adapters = append(adapters, &einoToolAdapter{t: t})
	}
	return adapters
}

// wrapToolsAsBaseTool converts tool registry to eino BaseTool slice (for config).
func wrapToolsAsBaseTool(reg *tool.Registry) []einotool.BaseTool {
	names := reg.List()
	adapters := make([]einotool.BaseTool, 0, len(names))
	for _, name := range names {
		t, ok := reg.Get(name)
		if !ok {
			continue
		}
		adapters = append(adapters, &einoToolAdapter{t: t})
	}
	return adapters
}

// ---------------------------------------------------------------------------
// Message conversion helpers
// ---------------------------------------------------------------------------

// extractSystemPrompt pulls system messages out of the input and returns them
// concatenated, since the EazyClaw provider API uses a dedicated system prompt
// field rather than a system message role.
func extractSystemPrompt(msgs []*schema.Message) (string, []*schema.Message) {
	if len(msgs) == 0 {
		return "", nil
	}
	var systemParts []string
	var rest []*schema.Message
	for _, m := range msgs {
		if m.Role == schema.System {
			systemParts = append(systemParts, m.Content)
		} else {
			rest = append(rest, m)
		}
	}
	return strings.Join(systemParts, "\n\n"), rest
}

// toProviderMessages converts eino messages to EazyClaw provider messages.
func toProviderMessages(msgs []*schema.Message) []providerPkg.Message {
	out := make([]providerPkg.Message, 0, len(msgs))
	for _, m := range msgs {
		pm := providerPkg.Message{
			Content: m.Content,
		}
		switch m.Role {
		case schema.User:
			pm.Role = "user"
		case schema.Assistant:
			pm.Role = "assistant"
		case schema.Tool:
			pm.Role = "tool"
			pm.ToolCallID = m.ToolCallID
		default:
			pm.Role = string(m.Role)
		}
		for _, tc := range m.ToolCalls {
			pm.ToolCalls = append(pm.ToolCalls, providerPkg.ToolCall{
				ID:        tc.ID,
				Name:      tc.Function.Name,
				Arguments: json.RawMessage(tc.Function.Arguments),
			})
		}
		out = append(out, pm)
	}
	return out
}

// toEinoMessages converts EazyClaw provider messages to eino messages.
func toEinoMessages(msgs []providerPkg.Message) []*schema.Message {
	out := make([]*schema.Message, 0, len(msgs))
	for _, m := range msgs {
		em := &schema.Message{
			Content:    m.Content,
			ToolCallID: m.ToolCallID,
		}
		switch m.Role {
		case "user":
			em.Role = schema.User
		case "assistant":
			em.Role = schema.Assistant
		case "tool":
			em.Role = schema.Tool
		default:
			em.Role = schema.RoleType(m.Role)
		}
		for _, tc := range m.ToolCalls {
			em.ToolCalls = append(em.ToolCalls, schema.ToolCall{
				ID:   tc.ID,
				Type: "function",
				Function: schema.FunctionCall{
					Name:      tc.Name,
					Arguments: string(tc.Arguments),
				},
			})
		}
		out = append(out, em)
	}
	return out
}

// providerResponseToEinoMessage converts a provider ChatResponse to an eino Message.
func providerResponseToEinoMessage(resp *providerPkg.ChatResponse) *schema.Message {
	msg := &schema.Message{
		Role:    schema.Assistant,
		Content: resp.Content,
	}
	for _, tc := range resp.ToolCalls {
		msg.ToolCalls = append(msg.ToolCalls, schema.ToolCall{
			ID:   tc.ID,
			Type: "function",
			Function: schema.FunctionCall{
				Name:      tc.Name,
				Arguments: string(tc.Arguments),
			},
		})
	}
	if resp.Usage.InputTokens > 0 || resp.Usage.OutputTokens > 0 {
		msg.ResponseMeta = &schema.ResponseMeta{
			Usage: &schema.TokenUsage{
				PromptTokens:     resp.Usage.InputTokens,
				CompletionTokens: resp.Usage.OutputTokens,
				TotalTokens:      resp.Usage.InputTokens + resp.Usage.OutputTokens,
			},
		}
	}
	return msg
}

// fromEinoMessage converts an eino Message back to an EazyClaw provider Message.
func fromEinoMessage(m *schema.Message) providerPkg.Message {
	pm := providerPkg.Message{
		Content:    m.Content,
		ToolCallID: m.ToolCallID,
	}
	switch m.Role {
	case schema.User:
		pm.Role = "user"
	case schema.Assistant:
		pm.Role = "assistant"
	case schema.Tool:
		pm.Role = "tool"
	default:
		pm.Role = string(m.Role)
	}
	for _, tc := range m.ToolCalls {
		pm.ToolCalls = append(pm.ToolCalls, providerPkg.ToolCall{
			ID:        tc.ID,
			Name:      tc.Function.Name,
			Arguments: json.RawMessage(tc.Function.Arguments),
		})
	}
	return pm
}

// einoToolInfoToProviderDefs converts eino ToolInfo to EazyClaw provider ToolDefs.
func einoToolInfoToProviderDefs(tools []*schema.ToolInfo) []providerPkg.ToolDef {
	if len(tools) == 0 {
		return nil
	}
	defs := make([]providerPkg.ToolDef, 0, len(tools))
	for _, t := range tools {
		td := providerPkg.ToolDef{
			Name:        t.Name,
			Description: t.Desc,
		}
		if t.ParamsOneOf != nil {
			if js, err := t.ParamsOneOf.ToJSONSchema(); err == nil && js != nil {
				if raw, err := json.Marshal(js); err == nil {
					td.Parameters = raw
				}
			}
		}
		defs = append(defs, td)
	}
	return defs
}

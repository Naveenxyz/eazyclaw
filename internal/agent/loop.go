package agent

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/eazyclaw/eazyclaw/internal/bus"
	providerPkg "github.com/eazyclaw/eazyclaw/internal/provider"
	"github.com/eazyclaw/eazyclaw/internal/router"
	"github.com/eazyclaw/eazyclaw/internal/tool"
)

const defaultMaxIterations = 20

// AgentLoop is the core agent loop that processes inbound messages,
// calls the LLM provider, executes tools, and sends responses.
type AgentLoop struct {
	bus           *bus.Bus
	providers     *providerPkg.Registry
	tools         *tool.Registry
	sessions      *SessionStore
	context       *ContextBuilder
	maxIterations int
	router        *router.Router
}

// NewAgentLoop creates a new AgentLoop.
func NewAgentLoop(
	b *bus.Bus,
	providers *providerPkg.Registry,
	tools *tool.Registry,
	sessions *SessionStore,
	contextBuilder *ContextBuilder,
	r *router.Router,
) *AgentLoop {
	return &AgentLoop{
		bus:           b,
		providers:     providers,
		tools:         tools,
		sessions:      sessions,
		context:       contextBuilder,
		maxIterations: defaultMaxIterations,
		router:        r,
	}
}

// SetMaxIterations configures the maximum tool-call loop iterations.
func (a *AgentLoop) SetMaxIterations(n int) {
	if n > 0 {
		a.maxIterations = n
	}
}

// Run starts the agent loop, processing messages from the bus until ctx is cancelled.
func (a *AgentLoop) Run(ctx context.Context) error {
	slog.Info("agent loop started")
	for {
		select {
		case <-ctx.Done():
			slog.Info("agent loop stopped")
			return ctx.Err()
		case msg, ok := <-a.bus.Inbound:
			if !ok {
				slog.Info("agent loop: inbound channel closed")
				return nil
			}
			go a.handleMessage(ctx, msg)
		}
	}
}

// handleMessage processes a single inbound message through the LLM + tool loop.
func (a *AgentLoop) handleMessage(ctx context.Context, msg bus.Message) {
	// Check if the sender is allowed.
	if !a.router.IsAllowed(msg) {
		slog.Warn("agent: message from disallowed user", "channel", msg.ChannelID, "sender", msg.SenderID)
		return
	}

	sessionID := a.router.SessionID(msg)
	slog.Info("agent: handling message", "session_id", sessionID, "sender", msg.SenderID)

	// Load session.
	session, err := a.sessions.Load(sessionID)
	if err != nil {
		slog.Error("agent: failed to load session", "session_id", sessionID, "error", err)
		a.sendError(msg, "Internal error: failed to load session.")
		return
	}

	// Reject empty messages — some providers error on empty user content.
	if msg.Text == "" {
		slog.Debug("agent: ignoring empty message", "session_id", sessionID)
		return
	}

	// Append user message to session.
	session.Messages = append(session.Messages, providerPkg.Message{
		Role:    "user",
		Content: msg.Text,
	})

	// Build system prompt.
	systemPrompt := a.context.Build()

	// Get tool definitions.
	toolDefs := a.tools.ToolDefs()

	// Get the default provider.
	prov, model, err := a.providers.DefaultProvider()
	if err != nil {
		slog.Error("agent: no default provider", "error", err)
		a.sendError(msg, "Internal error: no LLM provider configured.")
		return
	}

	// Tool loop.
	for iteration := 0; iteration < a.maxIterations; iteration++ {
		req := &providerPkg.ChatRequest{
			Model:        model,
			Messages:     session.Messages,
			Tools:        toolDefs,
			SystemPrompt: systemPrompt,
		}

		resp, err := prov.ChatCompletion(ctx, req)
		if err != nil {
			slog.Error("agent: provider error", "error", err, "iteration", iteration)
			a.sendError(msg, fmt.Sprintf("LLM error: %v", err))
			a.saveSession(session)
			return
		}

		// If no tool calls, send the final text response.
		if len(resp.ToolCalls) == 0 {
			if resp.Content != "" {
				a.sendReply(msg, resp.Content)
			}
			// Append assistant message to session.
			session.Messages = append(session.Messages, providerPkg.Message{
				Role:    "assistant",
				Content: resp.Content,
			})
			break
		}

		// Append assistant message with tool calls.
		session.Messages = append(session.Messages, providerPkg.Message{
			Role:      "assistant",
			Content:   resp.Content,
			ToolCalls: resp.ToolCalls,
		})

		// Execute each tool call and append results.
		for _, tc := range resp.ToolCalls {
			slog.Info("agent: executing tool", "tool", tc.Name, "call_id", tc.ID)

			result, execErr := a.tools.Execute(ctx, tc.Name, tc.Arguments)
			if execErr != nil {
				slog.Error("agent: tool execution error", "tool", tc.Name, "error", execErr)
				result = &tool.Result{
					Error:   fmt.Sprintf("tool execution failed: %v", execErr),
					IsError: true,
				}
			}

			content := result.Content
			if result.IsError && result.Error != "" {
				content = fmt.Sprintf("Error: %s\n%s", result.Error, content)
			}

			session.Messages = append(session.Messages, providerPkg.Message{
				Role:       "tool",
				Content:    content,
				ToolCallID: tc.ID,
			})
		}

		slog.Debug("agent: tool loop iteration complete", "iteration", iteration+1)
	}

	// Trim and save session.
	a.sessions.Trim(session, 100)
	a.saveSession(session)
}

// sendReply sends a text response back to the originating chat.
func (a *AgentLoop) sendReply(msg bus.Message, text string) {
	chatID := msg.GroupID
	if chatID == "" {
		chatID = msg.SenderID
	}
	a.bus.Outbound <- bus.OutboundMessage{
		ChannelID: msg.ChannelID,
		ChatID:    chatID,
		Text:      text,
		ReplyTo:   msg.ID,
	}
}

// sendError sends an error message back to the originating chat.
func (a *AgentLoop) sendError(msg bus.Message, text string) {
	a.sendReply(msg, text)
}

// saveSession persists the session to disk, logging any errors.
func (a *AgentLoop) saveSession(session *Session) {
	if err := a.sessions.Save(session); err != nil {
		slog.Error("agent: failed to save session", "session_id", session.ID, "error", err)
	}
}

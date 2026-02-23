package agent

import (
	"fmt"
	"os"
	"strings"
	"time"
)

const basePrompt = `You are EazyClaw, an AI assistant accessible via messaging. You can execute real tasks using your tools.`

// ContextBuilder assembles the system prompt for the agent.
type ContextBuilder struct {
	basePrompt       string
	skills           []string
	memoryPath       string
	tools            []string
	agentsPath       string
	soulPath         string
	bootstrapPath    string
	identityPath     string
	userPath         string
	heartbeatPath    string
	toolDescriptions map[string]string
	memoryManager    *MemoryManager
}

// PromptContext carries per-turn details used when building the system prompt.
type PromptContext struct {
	SessionID                string
	Channel                  string // "discord", "telegram", "whatsapp", "web", "heartbeat"
	IsDirect                 bool
	IsHeartbeat              bool
	Now                      time.Time
	Provider                 string
	Model                    string
	ContextWindowTokens      int
	EstimatedContextTokens   int
	EstimatedRemainingTokens int
}

// NewContextBuilder creates a new ContextBuilder.
func NewContextBuilder(memoryPath string) *ContextBuilder {
	return &ContextBuilder{
		basePrompt: basePrompt,
		memoryPath: memoryPath,
	}
}

// SetSkills sets the skill instruction fragments.
func (cb *ContextBuilder) SetSkills(skills []string) {
	cb.skills = skills
}

// SetTools sets the available tool names.
func (cb *ContextBuilder) SetTools(toolNames []string) {
	cb.tools = toolNames
}

// SetToolDescriptions sets tool name to one-line description mapping.
func (cb *ContextBuilder) SetToolDescriptions(descs map[string]string) {
	cb.toolDescriptions = descs
}

// SetSoulPath sets the path to the SOUL.md file.
func (cb *ContextBuilder) SetSoulPath(path string) {
	cb.soulPath = path
}

// SetAgentsPath sets the path to the AGENTS.md file.
func (cb *ContextBuilder) SetAgentsPath(path string) {
	cb.agentsPath = path
}

// SetIdentityPath sets the path to the IDENTITY.md file.
func (cb *ContextBuilder) SetIdentityPath(path string) {
	cb.identityPath = path
}

// SetBootstrapPath sets the path to the BOOTSTRAP.md file.
func (cb *ContextBuilder) SetBootstrapPath(path string) {
	cb.bootstrapPath = path
}

// SetUserPath sets the path to the USER.md file.
func (cb *ContextBuilder) SetUserPath(path string) {
	cb.userPath = path
}

// SetHeartbeatPath sets the path to the HEARTBEAT.md file.
func (cb *ContextBuilder) SetHeartbeatPath(path string) {
	cb.heartbeatPath = path
}

// SetMemoryManager injects a memory manager for OpenClaw-style memory context behavior.
func (cb *ContextBuilder) SetMemoryManager(mm *MemoryManager) {
	cb.memoryManager = mm
}

// Build assembles the full 7-section system prompt.
func (cb *ContextBuilder) Build() string {
	return cb.BuildFor(PromptContext{
		IsDirect: true,
		Now:      time.Now(),
	})
}

// BuildFor assembles the full 7-section system prompt for a specific turn context.
func (cb *ContextBuilder) BuildFor(ctx PromptContext) string {
	if ctx.Now.IsZero() {
		ctx.Now = time.Now()
	}

	var sb strings.Builder

	// Section 1: Identity
	cb.buildAgentInstructions(&sb)
	cb.buildIdentity(&sb)
	cb.buildIdentityProfile(&sb)
	cb.buildUser(&sb)
	cb.buildBootstrap(&sb)

	// Section 2: Tools
	cb.buildTools(&sb)

	// Section 3: Skills
	cb.buildSkills(&sb)

	// Section 4: Active Tasks
	cb.buildActiveTasks(&sb, ctx)

	// Section 5: Memory
	cb.buildMemory(&sb, ctx)

	// Section 6: Guidelines
	cb.buildGuidelines(&sb)

	// Section 7: Runtime Info
	cb.buildRuntimeInfo(&sb, ctx)

	return sb.String()
}

// buildAgentInstructions writes AGENTS.md when available.
func (cb *ContextBuilder) buildAgentInstructions(sb *strings.Builder) {
	if cb.agentsPath == "" {
		return
	}
	data, err := os.ReadFile(cb.agentsPath)
	if err != nil || len(data) == 0 {
		return
	}
	sb.WriteString("## Agent Instructions\n")
	sb.WriteString(string(data))
	sb.WriteString("\n")
}

// buildIdentity writes the identity section from SOUL.md or falls back to base prompt.
func (cb *ContextBuilder) buildIdentity(sb *strings.Builder) {
	sb.WriteString("\n## Soul\n")
	if cb.soulPath != "" {
		data, err := os.ReadFile(cb.soulPath)
		if err == nil && len(data) > 0 {
			sb.WriteString(string(data))
			sb.WriteString("\n")
			return
		}
	}

	sb.WriteString(cb.basePrompt)
	sb.WriteString("\n")
}

// buildIdentityProfile writes IDENTITY.md when available.
func (cb *ContextBuilder) buildIdentityProfile(sb *strings.Builder) {
	if cb.identityPath == "" {
		return
	}
	data, err := os.ReadFile(cb.identityPath)
	if err != nil || len(data) == 0 {
		return
	}
	sb.WriteString("\n## Identity Profile\n")
	sb.WriteString(string(data))
	sb.WriteString("\n")
}

// buildUser writes the user profile section from USER.md when available.
func (cb *ContextBuilder) buildUser(sb *strings.Builder) {
	if cb.userPath == "" {
		return
	}
	data, err := os.ReadFile(cb.userPath)
	if err != nil || len(data) == 0 {
		return
	}
	sb.WriteString("\n## User\n")
	sb.WriteString(string(data))
	sb.WriteString("\n")
}

// buildBootstrap writes BOOTSTRAP.md when available.
func (cb *ContextBuilder) buildBootstrap(sb *strings.Builder) {
	if cb.bootstrapPath == "" {
		return
	}
	data, err := os.ReadFile(cb.bootstrapPath)
	if err != nil || len(data) == 0 {
		return
	}
	sb.WriteString("\n## Bootstrap Ritual\n")
	sb.WriteString(string(data))
	sb.WriteString("\n")
}

// buildTools writes the tools section with descriptions when available.
func (cb *ContextBuilder) buildTools(sb *strings.Builder) {
	if len(cb.tools) == 0 {
		return
	}

	sb.WriteString("\n## Available Tools\n")
	for _, t := range cb.tools {
		if desc, ok := cb.toolDescriptions[t]; ok {
			sb.WriteString(fmt.Sprintf("- **%s**: %s\n", t, desc))
		} else {
			sb.WriteString(fmt.Sprintf("- %s\n", t))
		}
	}
}

// buildSkills writes the skills section.
func (cb *ContextBuilder) buildSkills(sb *strings.Builder) {
	if len(cb.skills) == 0 {
		return
	}

	sb.WriteString("\n## Skills\n")
	for _, s := range cb.skills {
		sb.WriteString(s)
		sb.WriteString("\n")
	}
}

// buildActiveTasks writes the active tasks section from HEARTBEAT.md.
func (cb *ContextBuilder) buildActiveTasks(sb *strings.Builder, ctx PromptContext) {
	if !ctx.IsHeartbeat {
		return
	}
	if cb.heartbeatPath == "" {
		return
	}

	data, err := os.ReadFile(cb.heartbeatPath)
	if err != nil || len(data) == 0 {
		return
	}

	sb.WriteString("\n## Active Tasks\n")
	sb.WriteString(string(data))
	sb.WriteString("\n")
}

// buildMemory writes the memory section from MEMORY.md.
func (cb *ContextBuilder) buildMemory(sb *strings.Builder, ctx PromptContext) {
	if cb.memoryManager != nil {
		content := cb.memoryManager.BuildMemoryContext(ctx.IsDirect, ctx.Now)
		if strings.TrimSpace(content) == "" {
			return
		}
		sb.WriteString("\n## Memory\n")
		sb.WriteString(content)
		sb.WriteString("\n")
		return
	}

	if cb.memoryPath == "" {
		return
	}

	data, err := os.ReadFile(cb.memoryPath)
	if err != nil || len(data) == 0 {
		return
	}

	sb.WriteString("\n## Memory\n")
	sb.WriteString(string(data))
	sb.WriteString("\n")
}

// buildGuidelines writes behavioral rules for the agent.
func (cb *ContextBuilder) buildGuidelines(sb *strings.Builder) {
	sb.WriteString("\n## Guidelines\n")
	sb.WriteString("- Execute tasks step by step, confirming completion of each before moving on\n")
	sb.WriteString("- Always use the appropriate tool for the job; do not simulate tool output\n")
	sb.WriteString("- Be concise in responses; avoid unnecessary filler\n")
	sb.WriteString("- When a tool call fails, report the error clearly and suggest alternatives\n")
	sb.WriteString("- Maintain conversation context across messages in a session\n")
	sb.WriteString("- Never fabricate information; say you don't know when uncertain\n")
	sb.WriteString("- For durable recall, write stable facts to memory/MEMORY.md and daily updates to memory/YYYY-MM-DD.md\n")
	sb.WriteString("- User profile facts (name/preferred name/timezone) belong in memory/USER.md; agent self-identity belongs in memory/IDENTITY.md\n")
	sb.WriteString("- Never use read_file/write_file/edit_file for AGENTS.md, SOUL.md, IDENTITY.md, USER.md, HEARTBEAT.md, or MEMORY.md; use memory_* tools instead\n")
	sb.WriteString("- If a system turn asks for silent housekeeping, respond with NO_REPLY when no user-facing text is needed\n")
}

// buildRuntimeInfo writes dynamic runtime information.
func (cb *ContextBuilder) buildRuntimeInfo(sb *strings.Builder, ctx PromptContext) {
	sb.WriteString("\n## Runtime Info\n")
	sb.WriteString(fmt.Sprintf("- Current time: %s\n", ctx.Now.Format(time.RFC3339)))
	if ctx.SessionID != "" {
		sb.WriteString(fmt.Sprintf("- Session ID: %s\n", ctx.SessionID))
	}
	if ctx.Channel != "" {
		sb.WriteString(fmt.Sprintf("- Channel: %s\n", ctx.Channel))
	}
	if ctx.IsDirect {
		sb.WriteString("- Chat type: direct\n")
	} else {
		sb.WriteString("- Chat type: group\n")
	}
	if ctx.IsHeartbeat {
		sb.WriteString("- Turn type: heartbeat\n")
	}
	if ctx.Provider != "" {
		sb.WriteString(fmt.Sprintf("- Provider: %s\n", ctx.Provider))
	}
	if ctx.Model != "" {
		sb.WriteString(fmt.Sprintf("- Model: %s\n", ctx.Model))
	}
	if ctx.ContextWindowTokens > 0 {
		sb.WriteString(fmt.Sprintf("- Context window (configured): %d tokens\n", ctx.ContextWindowTokens))
	}
	if ctx.EstimatedContextTokens > 0 {
		sb.WriteString(fmt.Sprintf("- Estimated context length now: %d tokens\n", ctx.EstimatedContextTokens))
	}
	if ctx.EstimatedRemainingTokens > 0 {
		sb.WriteString(fmt.Sprintf("- Estimated remaining before limit: %d tokens\n", ctx.EstimatedRemainingTokens))
	}
	sb.WriteString("- Token counts are approximate planning hints, not exact tokenizer output\n")

	if len(cb.tools) > 0 {
		sb.WriteString(fmt.Sprintf("- Available tools: %d\n", len(cb.tools)))
	}

	if len(cb.skills) > 0 {
		sb.WriteString(fmt.Sprintf("- Loaded skills: %d\n", len(cb.skills)))
	}
}

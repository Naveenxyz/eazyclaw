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
	basePrompt string
	skills     []string
	memoryPath string
	tools      []string
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

// Build assembles the full system prompt.
func (cb *ContextBuilder) Build() string {
	var sb strings.Builder

	sb.WriteString(cb.basePrompt)
	sb.WriteString("\n")

	// Available tools.
	if len(cb.tools) > 0 {
		sb.WriteString("\n## Available Tools\n")
		for _, t := range cb.tools {
			sb.WriteString(fmt.Sprintf("- %s\n", t))
		}
	}

	// Skills.
	if len(cb.skills) > 0 {
		sb.WriteString("\n## Skills\n")
		for _, s := range cb.skills {
			sb.WriteString(s)
			sb.WriteString("\n")
		}
	}

	// Memory.
	if cb.memoryPath != "" {
		data, err := os.ReadFile(cb.memoryPath)
		if err == nil && len(data) > 0 {
			sb.WriteString("\n## Memory\n")
			sb.WriteString(string(data))
			sb.WriteString("\n")
		}
	}

	// Guidelines.
	sb.WriteString("\n## Guidelines\n")
	sb.WriteString("- Execute tasks step by step\n")
	sb.WriteString("- Use tools to accomplish goals\n")
	sb.WriteString("- Be concise in responses\n")
	sb.WriteString(fmt.Sprintf("- Current date: %s\n", time.Now().Format("2006-01-02")))

	return sb.String()
}

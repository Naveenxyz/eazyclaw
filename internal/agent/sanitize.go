package agent

import (
	"fmt"

	providerPkg "github.com/eazyclaw/eazyclaw/internal/provider"
)

// SanitizeReport tracks what SanitizeMessages repaired.
type SanitizeReport struct {
	DroppedOrphanedToolMessages  int
	InsertedSyntheticToolResults int
	DroppedInvalidAssistantTurns int
}

// SanitizeMessages repairs tool call/result pairing in message history.
// It drops orphaned tool messages and inserts synthetic error results for
// assistant tool calls that have no matching tool result.
func SanitizeMessages(messages []providerPkg.Message) ([]providerPkg.Message, SanitizeReport) {
	var report SanitizeReport
	if len(messages) == 0 {
		return messages, report
	}

	var result []providerPkg.Message

	for i := 0; i < len(messages); i++ {
		m := messages[i]

		switch m.Role {
		case "tool":
			// Tool message at position 0: orphaned, drop it.
			if len(result) == 0 {
				report.DroppedOrphanedToolMessages++
				continue
			}
			// Find the owning assistant in the result so far.
			ownerIdx := findOwningAssistant(result, m.ToolCallID)
			if ownerIdx < 0 {
				// No assistant owns this tool call ID — orphaned.
				report.DroppedOrphanedToolMessages++
				continue
			}
			result = append(result, m)

		case "assistant":
			if len(m.ToolCalls) > 0 && len(result) == 0 {
				// Assistant with tool calls at position 0 (no preceding user): drop.
				report.DroppedInvalidAssistantTurns++
				continue
			}
			result = append(result, m)

			// For assistant messages with tool calls, check that every tool call
			// has a matching result within the same turn (before the next user message).
			if len(m.ToolCalls) > 0 {
				turnSlice := messagesUntilNextUser(messages[i+1:])
				for _, tc := range m.ToolCalls {
					if !toolResultExistsInSlice(turnSlice, tc.ID) {
						// Insert synthetic error result.
						result = append(result, providerPkg.Message{
							Role:       "tool",
							ToolCallID: tc.ID,
							Content:    fmt.Sprintf("[eazyclaw] missing tool result for %s; synthetic error inserted during history repair", tc.Name),
						})
						report.InsertedSyntheticToolResults++
					}
				}
			}

		default:
			// user or other roles: keep.
			result = append(result, m)
		}
	}

	return result, report
}

// findOwningAssistant walks backward through result to find the assistant
// message that owns the given toolCallID.
func findOwningAssistant(result []providerPkg.Message, toolCallID string) int {
	for i := len(result) - 1; i >= 0; i-- {
		m := result[i]
		if m.Role == "assistant" && len(m.ToolCalls) > 0 {
			for _, tc := range m.ToolCalls {
				if tc.ID == toolCallID {
					return i
				}
			}
			// Hit an assistant with tool calls that doesn't own this ID.
			// Keep looking — there might be an earlier one.
			continue
		}
		if m.Role == "user" {
			// Crossed a user boundary without finding the owner.
			return -1
		}
	}
	return -1
}

// messagesUntilNextUser returns the slice of messages up to (but not including)
// the next user message. This scopes tool-result lookups to the current turn.
func messagesUntilNextUser(messages []providerPkg.Message) []providerPkg.Message {
	for i, m := range messages {
		if m.Role == "user" {
			return messages[:i]
		}
	}
	return messages
}

// toolResultExistsInSlice checks if any message in the slice is a tool result
// with the given toolCallID.
func toolResultExistsInSlice(messages []providerPkg.Message, toolCallID string) bool {
	for _, m := range messages {
		if m.Role == "tool" && m.ToolCallID == toolCallID {
			return true
		}
	}
	return false
}

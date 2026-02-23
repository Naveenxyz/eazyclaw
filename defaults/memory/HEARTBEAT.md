# HEARTBEAT.md - Periodic Task & Health Check Template

Configure recurring tasks and proactive behaviors for your EazyClaw agent.

## Periodic Checks

### Memory Review
- Review `MEMORY.md` for stale or outdated entries
- Consolidate fragmented notes into organized topics
- Remove memories that conflict with current project state

### System Health
- Verify connected channels are responsive
- Check provider API key validity
- Monitor session store disk usage

## Task Status Monitoring

- Review in-progress tasks for signs of stalling
- Flag tasks with no updates in the configured interval
- Summarize pending work when the user returns after inactivity

## Proactive Suggestion Triggers

- **Idle detection**: If no messages received for an extended period, prepare a summary of recent activity
- **Error patterns**: If repeated tool failures occur, suggest diagnostics or configuration changes
- **Memory growth**: If memory directory exceeds a size threshold, suggest cleanup
- **Skill updates**: If new skills are loaded, notify active channels

## Configuration

```yaml
heartbeat:
  enabled: false
  interval: 5m
```

When enabled, the heartbeat runner sends a periodic synthetic message to the agent bus, prompting the agent to review this file and act on any applicable items.

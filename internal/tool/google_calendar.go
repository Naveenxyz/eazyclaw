package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"
)

// CalendarTool implements the Tool interface for Google Calendar operations.
type CalendarTool struct {
	auth *GoogleAuth
}

// NewCalendarTool creates a new CalendarTool with the given auth.
func NewCalendarTool(auth *GoogleAuth) *CalendarTool {
	return &CalendarTool{auth: auth}
}

func (t *CalendarTool) Name() string        { return "google_calendar" }
func (t *CalendarTool) Description() string  { return "Manage Google Calendar events" }
func (t *CalendarTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
  "type": "object",
  "properties": {
    "action":      {"type": "string", "enum": ["list_events", "get_event", "create_event", "update_event", "delete_event", "list_calendars"], "description": "Action to perform"},
    "calendar_id": {"type": "string", "description": "Calendar ID (default: primary)"},
    "event_id":    {"type": "string", "description": "Event ID (for get/update/delete)"},
    "time_min":    {"type": "string", "description": "Start of time range (RFC3339)"},
    "time_max":    {"type": "string", "description": "End of time range (RFC3339)"},
    "summary":     {"type": "string", "description": "Event title"},
    "description": {"type": "string", "description": "Event description"},
    "start":       {"type": "string", "description": "Event start time (RFC3339)"},
    "end":         {"type": "string", "description": "Event end time (RFC3339)"},
    "attendees":   {"type": "array", "items": {"type": "string"}, "description": "Attendee email addresses"},
    "max_results": {"type": "integer", "description": "Maximum results to return (default 10)"}
  },
  "required": ["action"]
}`)
}

type calendarArgs struct {
	Action      string   `json:"action"`
	CalendarID  string   `json:"calendar_id"`
	EventID     string   `json:"event_id"`
	TimeMin     string   `json:"time_min"`
	TimeMax     string   `json:"time_max"`
	Summary     string   `json:"summary"`
	Description string   `json:"description"`
	Start       string   `json:"start"`
	End         string   `json:"end"`
	Attendees   []string `json:"attendees"`
	MaxResults  int64    `json:"max_results"`
}

func (t *CalendarTool) Execute(ctx context.Context, args json.RawMessage) (*Result, error) {
	var a calendarArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return &Result{Error: "invalid arguments: " + err.Error(), IsError: true}, nil
	}
	if a.CalendarID == "" {
		a.CalendarID = "primary"
	}
	if a.MaxResults <= 0 {
		a.MaxResults = 10
	}

	srv, err := t.service(ctx)
	if err != nil {
		return &Result{Error: err.Error(), IsError: true}, nil
	}

	switch a.Action {
	case "list_events":
		return t.listEvents(srv, a)
	case "get_event":
		return t.getEvent(srv, a)
	case "create_event":
		return t.createEvent(srv, a)
	case "update_event":
		return t.updateEvent(srv, a)
	case "delete_event":
		return t.deleteEvent(srv, a)
	case "list_calendars":
		return t.listCalendars(srv)
	default:
		return &Result{Error: fmt.Sprintf("unknown action: %s", a.Action), IsError: true}, nil
	}
}

func (t *CalendarTool) service(ctx context.Context) (*calendar.Service, error) {
	client, err := t.auth.Client(ctx)
	if err != nil {
		return nil, fmt.Errorf("calendar auth failed: %w", err)
	}
	srv, err := calendar.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return nil, fmt.Errorf("failed to create calendar service: %w", err)
	}
	return srv, nil
}

func (t *CalendarTool) listEvents(srv *calendar.Service, a calendarArgs) (*Result, error) {
	call := srv.Events.List(a.CalendarID).
		MaxResults(a.MaxResults).
		SingleEvents(true).
		OrderBy("startTime")

	if a.TimeMin != "" {
		call = call.TimeMin(a.TimeMin)
	} else {
		call = call.TimeMin(time.Now().Format(time.RFC3339))
	}

	if a.TimeMax != "" {
		call = call.TimeMax(a.TimeMax)
	} else {
		call = call.TimeMax(time.Now().AddDate(0, 0, 7).Format(time.RFC3339))
	}

	events, err := call.Do()
	if err != nil {
		return &Result{Error: fmt.Sprintf("failed to list events: %v", err), IsError: true}, nil
	}

	if len(events.Items) == 0 {
		return &Result{Content: "No upcoming events found."}, nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Found %d events:\n\n", len(events.Items)))
	for i, event := range events.Items {
		start := event.Start.DateTime
		if start == "" {
			start = event.Start.Date
		}
		end := event.End.DateTime
		if end == "" {
			end = event.End.Date
		}
		sb.WriteString(fmt.Sprintf("%d. %s\n   ID: %s\n   Start: %s\n   End: %s\n",
			i+1, event.Summary, event.Id, start, end))
		if event.Description != "" {
			sb.WriteString(fmt.Sprintf("   Description: %s\n", event.Description))
		}
		if len(event.Attendees) > 0 {
			emails := make([]string, 0, len(event.Attendees))
			for _, att := range event.Attendees {
				emails = append(emails, att.Email)
			}
			sb.WriteString(fmt.Sprintf("   Attendees: %s\n", strings.Join(emails, ", ")))
		}
		sb.WriteString("\n")
	}

	return &Result{Content: sb.String()}, nil
}

func (t *CalendarTool) getEvent(srv *calendar.Service, a calendarArgs) (*Result, error) {
	if a.EventID == "" {
		return &Result{Error: "get_event requires 'event_id'", IsError: true}, nil
	}

	event, err := srv.Events.Get(a.CalendarID, a.EventID).Do()
	if err != nil {
		return &Result{Error: fmt.Sprintf("failed to get event: %v", err), IsError: true}, nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Summary: %s\n", event.Summary))
	sb.WriteString(fmt.Sprintf("ID: %s\n", event.Id))
	sb.WriteString(fmt.Sprintf("Status: %s\n", event.Status))

	if event.Start != nil {
		start := event.Start.DateTime
		if start == "" {
			start = event.Start.Date
		}
		sb.WriteString(fmt.Sprintf("Start: %s\n", start))
	}
	if event.End != nil {
		end := event.End.DateTime
		if end == "" {
			end = event.End.Date
		}
		sb.WriteString(fmt.Sprintf("End: %s\n", end))
	}
	if event.Description != "" {
		sb.WriteString(fmt.Sprintf("Description: %s\n", event.Description))
	}
	if event.Location != "" {
		sb.WriteString(fmt.Sprintf("Location: %s\n", event.Location))
	}
	if event.HtmlLink != "" {
		sb.WriteString(fmt.Sprintf("Link: %s\n", event.HtmlLink))
	}
	if len(event.Attendees) > 0 {
		sb.WriteString("Attendees:\n")
		for _, att := range event.Attendees {
			status := att.ResponseStatus
			sb.WriteString(fmt.Sprintf("  - %s (%s)\n", att.Email, status))
		}
	}

	return &Result{Content: sb.String()}, nil
}

func (t *CalendarTool) createEvent(srv *calendar.Service, a calendarArgs) (*Result, error) {
	if a.Summary == "" || a.Start == "" || a.End == "" {
		return &Result{Error: "create_event requires 'summary', 'start', and 'end'", IsError: true}, nil
	}

	event := &calendar.Event{
		Summary:     a.Summary,
		Description: a.Description,
		Start:       &calendar.EventDateTime{DateTime: a.Start},
		End:         &calendar.EventDateTime{DateTime: a.End},
	}

	if len(a.Attendees) > 0 {
		attendees := make([]*calendar.EventAttendee, 0, len(a.Attendees))
		for _, email := range a.Attendees {
			attendees = append(attendees, &calendar.EventAttendee{Email: email})
		}
		event.Attendees = attendees
	}

	created, err := srv.Events.Insert(a.CalendarID, event).Do()
	if err != nil {
		return &Result{Error: fmt.Sprintf("failed to create event: %v", err), IsError: true}, nil
	}

	return &Result{Content: fmt.Sprintf("Event created successfully.\nID: %s\nLink: %s", created.Id, created.HtmlLink)}, nil
}

func (t *CalendarTool) updateEvent(srv *calendar.Service, a calendarArgs) (*Result, error) {
	if a.EventID == "" {
		return &Result{Error: "update_event requires 'event_id'", IsError: true}, nil
	}

	existing, err := srv.Events.Get(a.CalendarID, a.EventID).Do()
	if err != nil {
		return &Result{Error: fmt.Sprintf("failed to get event: %v", err), IsError: true}, nil
	}

	if a.Summary != "" {
		existing.Summary = a.Summary
	}
	if a.Description != "" {
		existing.Description = a.Description
	}
	if a.Start != "" {
		existing.Start = &calendar.EventDateTime{DateTime: a.Start}
	}
	if a.End != "" {
		existing.End = &calendar.EventDateTime{DateTime: a.End}
	}
	if len(a.Attendees) > 0 {
		attendees := make([]*calendar.EventAttendee, 0, len(a.Attendees))
		for _, email := range a.Attendees {
			attendees = append(attendees, &calendar.EventAttendee{Email: email})
		}
		existing.Attendees = attendees
	}

	updated, err := srv.Events.Update(a.CalendarID, a.EventID, existing).Do()
	if err != nil {
		return &Result{Error: fmt.Sprintf("failed to update event: %v", err), IsError: true}, nil
	}

	return &Result{Content: fmt.Sprintf("Event updated successfully.\nID: %s\nLink: %s", updated.Id, updated.HtmlLink)}, nil
}

func (t *CalendarTool) deleteEvent(srv *calendar.Service, a calendarArgs) (*Result, error) {
	if a.EventID == "" {
		return &Result{Error: "delete_event requires 'event_id'", IsError: true}, nil
	}

	if err := srv.Events.Delete(a.CalendarID, a.EventID).Do(); err != nil {
		return &Result{Error: fmt.Sprintf("failed to delete event: %v", err), IsError: true}, nil
	}

	return &Result{Content: fmt.Sprintf("Event %s deleted successfully.", a.EventID)}, nil
}

func (t *CalendarTool) listCalendars(srv *calendar.Service) (*Result, error) {
	list, err := srv.CalendarList.List().Do()
	if err != nil {
		return &Result{Error: fmt.Sprintf("failed to list calendars: %v", err), IsError: true}, nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Found %d calendars:\n\n", len(list.Items)))
	for _, cal := range list.Items {
		primary := ""
		if cal.Primary {
			primary = " [PRIMARY]"
		}
		sb.WriteString(fmt.Sprintf("- %s%s\n  ID: %s\n  Access: %s\n\n",
			cal.Summary, primary, cal.Id, cal.AccessRole))
	}

	return &Result{Content: sb.String()}, nil
}

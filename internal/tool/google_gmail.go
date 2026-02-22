package tool

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"
)

// GmailTool implements the Tool interface for Gmail operations.
type GmailTool struct {
	auth *GoogleAuth
}

// NewGmailTool creates a new GmailTool with the given auth.
func NewGmailTool(auth *GoogleAuth) *GmailTool {
	return &GmailTool{auth: auth}
}

func (t *GmailTool) Name() string        { return "gmail" }
func (t *GmailTool) Description() string  { return "Read, search, and send emails via Gmail" }
func (t *GmailTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
  "type": "object",
  "properties": {
    "action":      {"type": "string", "enum": ["search", "read", "send", "reply", "list_labels"], "description": "Action to perform"},
    "query":       {"type": "string", "description": "Search query (for search action)"},
    "message_id":  {"type": "string", "description": "Message ID (for read/reply actions)"},
    "to":          {"type": "string", "description": "Recipient email (for send/reply actions)"},
    "subject":     {"type": "string", "description": "Email subject (for send action)"},
    "body":        {"type": "string", "description": "Email body (for send/reply actions)"},
    "max_results": {"type": "integer", "description": "Maximum results to return (default 10)"}
  },
  "required": ["action"]
}`)
}

type gmailArgs struct {
	Action     string `json:"action"`
	Query      string `json:"query"`
	MessageID  string `json:"message_id"`
	To         string `json:"to"`
	Subject    string `json:"subject"`
	Body       string `json:"body"`
	MaxResults int64  `json:"max_results"`
}

func (t *GmailTool) Execute(ctx context.Context, args json.RawMessage) (*Result, error) {
	var a gmailArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return &Result{Error: "invalid arguments: " + err.Error(), IsError: true}, nil
	}
	if a.MaxResults <= 0 {
		a.MaxResults = 10
	}

	srv, err := t.service(ctx)
	if err != nil {
		return &Result{Error: err.Error(), IsError: true}, nil
	}

	switch a.Action {
	case "search":
		return t.search(srv, a)
	case "read":
		return t.read(srv, a)
	case "send":
		return t.send(srv, a)
	case "reply":
		return t.replyTo(srv, a)
	case "list_labels":
		return t.listLabels(srv)
	default:
		return &Result{Error: fmt.Sprintf("unknown action: %s", a.Action), IsError: true}, nil
	}
}

func (t *GmailTool) service(ctx context.Context) (*gmail.Service, error) {
	client, err := t.auth.Client(ctx)
	if err != nil {
		return nil, fmt.Errorf("gmail auth failed: %w", err)
	}
	srv, err := gmail.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return nil, fmt.Errorf("failed to create gmail service: %w", err)
	}
	return srv, nil
}

func (t *GmailTool) search(srv *gmail.Service, a gmailArgs) (*Result, error) {
	if a.Query == "" {
		return &Result{Error: "search requires 'query'", IsError: true}, nil
	}

	resp, err := srv.Users.Messages.List("me").Q(a.Query).MaxResults(a.MaxResults).Do()
	if err != nil {
		return &Result{Error: fmt.Sprintf("search failed: %v", err), IsError: true}, nil
	}

	if len(resp.Messages) == 0 {
		return &Result{Content: "No messages found."}, nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Found %d messages:\n\n", len(resp.Messages)))

	for i, msg := range resp.Messages {
		full, err := srv.Users.Messages.Get("me", msg.Id).Format("metadata").
			MetadataHeaders("From", "Subject", "Date").Do()
		if err != nil {
			continue
		}

		from, subject, date := "", "", ""
		for _, h := range full.Payload.Headers {
			switch h.Name {
			case "From":
				from = h.Value
			case "Subject":
				subject = h.Value
			case "Date":
				date = h.Value
			}
		}

		sb.WriteString(fmt.Sprintf("%d. ID: %s\n   From: %s\n   Subject: %s\n   Date: %s\n   Snippet: %s\n\n",
			i+1, msg.Id, from, subject, date, full.Snippet))
	}

	return &Result{Content: sb.String()}, nil
}

func (t *GmailTool) read(srv *gmail.Service, a gmailArgs) (*Result, error) {
	if a.MessageID == "" {
		return &Result{Error: "read requires 'message_id'", IsError: true}, nil
	}

	msg, err := srv.Users.Messages.Get("me", a.MessageID).Format("full").Do()
	if err != nil {
		return &Result{Error: fmt.Sprintf("failed to get message: %v", err), IsError: true}, nil
	}

	var sb strings.Builder
	for _, h := range msg.Payload.Headers {
		switch h.Name {
		case "From", "To", "Subject", "Date", "Cc":
			sb.WriteString(fmt.Sprintf("%s: %s\n", h.Name, h.Value))
		}
	}
	sb.WriteString("\n")

	body := extractBody(msg.Payload)
	sb.WriteString(body)

	return &Result{Content: sb.String()}, nil
}

func extractBody(payload *gmail.MessagePart) string {
	if payload.Body != nil && payload.Body.Data != "" {
		decoded, err := base64.URLEncoding.DecodeString(payload.Body.Data)
		if err == nil {
			return string(decoded)
		}
	}

	// Check parts for multipart messages.
	for _, part := range payload.Parts {
		if part.MimeType == "text/plain" && part.Body != nil && part.Body.Data != "" {
			decoded, err := base64.URLEncoding.DecodeString(part.Body.Data)
			if err == nil {
				return string(decoded)
			}
		}
	}

	// Fall back to HTML part.
	for _, part := range payload.Parts {
		if part.MimeType == "text/html" && part.Body != nil && part.Body.Data != "" {
			decoded, err := base64.URLEncoding.DecodeString(part.Body.Data)
			if err == nil {
				return string(decoded)
			}
		}
	}

	// Recurse into nested parts.
	for _, part := range payload.Parts {
		if len(part.Parts) > 0 {
			if body := extractBody(part); body != "" {
				return body
			}
		}
	}

	return "[No readable content]"
}

func (t *GmailTool) send(srv *gmail.Service, a gmailArgs) (*Result, error) {
	if a.To == "" || a.Subject == "" || a.Body == "" {
		return &Result{Error: "send requires 'to', 'subject', and 'body'", IsError: true}, nil
	}

	raw := fmt.Sprintf("To: %s\r\nSubject: %s\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s",
		a.To, a.Subject, a.Body)

	msg := &gmail.Message{
		Raw: base64.URLEncoding.EncodeToString([]byte(raw)),
	}

	sent, err := srv.Users.Messages.Send("me", msg).Do()
	if err != nil {
		return &Result{Error: fmt.Sprintf("failed to send: %v", err), IsError: true}, nil
	}

	return &Result{Content: fmt.Sprintf("Email sent successfully. Message ID: %s", sent.Id)}, nil
}

func (t *GmailTool) replyTo(srv *gmail.Service, a gmailArgs) (*Result, error) {
	if a.MessageID == "" || a.Body == "" {
		return &Result{Error: "reply requires 'message_id' and 'body'", IsError: true}, nil
	}

	original, err := srv.Users.Messages.Get("me", a.MessageID).Format("metadata").
		MetadataHeaders("From", "Subject", "Message-ID", "To").Do()
	if err != nil {
		return &Result{Error: fmt.Sprintf("failed to get original message: %v", err), IsError: true}, nil
	}

	var from, subject, messageID, to string
	for _, h := range original.Payload.Headers {
		switch h.Name {
		case "From":
			from = h.Value
		case "Subject":
			subject = h.Value
		case "Message-ID":
			messageID = h.Value
		case "To":
			to = h.Value
		}
	}

	replyTo := from
	if a.To != "" {
		replyTo = a.To
	}

	if !strings.HasPrefix(strings.ToLower(subject), "re:") {
		subject = "Re: " + subject
	}

	raw := fmt.Sprintf("To: %s\r\nSubject: %s\r\nIn-Reply-To: %s\r\nReferences: %s\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s",
		replyTo, subject, messageID, messageID, a.Body)

	msg := &gmail.Message{
		Raw:      base64.URLEncoding.EncodeToString([]byte(raw)),
		ThreadId: original.ThreadId,
	}

	_ = to // original recipient, not needed for reply

	sent, err := srv.Users.Messages.Send("me", msg).Do()
	if err != nil {
		return &Result{Error: fmt.Sprintf("failed to send reply: %v", err), IsError: true}, nil
	}

	return &Result{Content: fmt.Sprintf("Reply sent successfully. Message ID: %s", sent.Id)}, nil
}

func (t *GmailTool) listLabels(srv *gmail.Service) (*Result, error) {
	resp, err := srv.Users.Labels.List("me").Do()
	if err != nil {
		return &Result{Error: fmt.Sprintf("failed to list labels: %v", err), IsError: true}, nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Found %d labels:\n\n", len(resp.Labels)))
	for _, label := range resp.Labels {
		sb.WriteString(fmt.Sprintf("- %s (ID: %s, Type: %s)\n", label.Name, label.Id, label.Type))
	}

	return &Result{Content: sb.String()}, nil
}

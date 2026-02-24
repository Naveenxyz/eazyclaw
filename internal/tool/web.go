package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	htmltomarkdown "github.com/JohannesKaufmann/html-to-markdown/v2"
)

const (
	webFetchMaxBodyBytes = 2 << 20 // 2MB
	webFetchMaxRedirects = 5
)

var (
	webFetchLookupIP = net.LookupIP

	webFetchBlockedHostnames = map[string]struct{}{
		"localhost":                {},
		"localhost.localdomain":    {},
		"metadata.google.internal": {},
	}
	webFetchBlockedCIDRs = mustParseCIDRs([]string{
		"100.64.0.0/10", // carrier-grade NAT
		"198.18.0.0/15", // benchmarking networks
	})
)

func mustParseCIDRs(raw []string) []*net.IPNet {
	nets := make([]*net.IPNet, 0, len(raw))
	for _, cidr := range raw {
		_, network, err := net.ParseCIDR(cidr)
		if err == nil {
			nets = append(nets, network)
		}
	}
	return nets
}

func validateWebFetchURL(raw string) (*url.URL, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, fmt.Errorf("unsupported URL scheme: %s", parsed.Scheme)
	}
	if err := validateWebFetchHost(parsed.Hostname()); err != nil {
		return nil, err
	}
	return parsed, nil
}

func validateWebFetchHost(host string) error {
	host = strings.ToLower(strings.TrimSpace(host))
	if host == "" {
		return fmt.Errorf("missing hostname")
	}
	if _, blocked := webFetchBlockedHostnames[host]; blocked {
		return fmt.Errorf("blocked hostname: %s", host)
	}

	if ip := net.ParseIP(host); ip != nil {
		if isBlockedWebFetchIP(ip) {
			return fmt.Errorf("blocked IP address: %s", ip.String())
		}
		return nil
	}

	ips, err := webFetchLookupIP(host)
	if err != nil {
		return fmt.Errorf("failed to resolve host: %w", err)
	}
	if len(ips) == 0 {
		return fmt.Errorf("failed to resolve host: %s", host)
	}
	for _, ip := range ips {
		if isBlockedWebFetchIP(ip) {
			return fmt.Errorf("blocked host resolution: %s -> %s", host, ip.String())
		}
	}
	return nil
}

func isBlockedWebFetchIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsMulticast() || ip.IsUnspecified() {
		return true
	}
	if ip4 := ip.To4(); ip4 != nil {
		// 0.0.0.0/8 and 169.254.0.0/16
		if ip4[0] == 0 || (ip4[0] == 169 && ip4[1] == 254) {
			return true
		}
	}
	for _, blocked := range webFetchBlockedCIDRs {
		if blocked.Contains(ip) {
			return true
		}
	}
	return false
}

// --- WebFetchTool ---

// WebFetchTool fetches a URL and converts HTML to markdown.
type WebFetchTool struct{}

// NewWebFetchTool creates a new WebFetchTool.
func NewWebFetchTool() *WebFetchTool {
	return &WebFetchTool{}
}

func (t *WebFetchTool) Name() string        { return "web_fetch" }
func (t *WebFetchTool) Description() string { return "Fetch a URL and return content as markdown" }
func (t *WebFetchTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
  "type": "object",
  "properties": {
    "url": {"type": "string", "description": "URL to fetch"}
  },
  "required": ["url"]
}`)
}

func (t *WebFetchTool) Execute(ctx context.Context, args json.RawMessage) (*Result, error) {
	var params struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return &Result{Error: fmt.Sprintf("invalid arguments: %v", err), IsError: true}, nil
	}
	if params.URL == "" {
		return &Result{Error: "url is required", IsError: true}, nil
	}

	parsedURL, err := validateWebFetchURL(params.URL)
	if err != nil {
		return &Result{Error: err.Error(), IsError: true}, nil
	}

	client := &http.Client{
		Timeout: 30 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= webFetchMaxRedirects {
				return fmt.Errorf("too many redirects")
			}
			_, err := validateWebFetchURL(req.URL.String())
			return err
		},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, parsedURL.String(), nil)
	if err != nil {
		return &Result{Error: fmt.Sprintf("failed to create request: %v", err), IsError: true}, nil
	}
	req.Header.Set("User-Agent", "EazyClaw/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return &Result{Error: fmt.Sprintf("fetch failed: %v", err), IsError: true}, nil
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, webFetchMaxBodyBytes+1))
	if err != nil {
		return &Result{Error: fmt.Sprintf("failed to read response: %v", err), IsError: true}, nil
	}
	truncatedByBytes := false
	if len(body) > webFetchMaxBodyBytes {
		truncatedByBytes = true
		body = body[:webFetchMaxBodyBytes]
	}

	contentType := resp.Header.Get("Content-Type")
	content := string(body)

	// Convert HTML to markdown.
	if strings.Contains(contentType, "text/html") || strings.Contains(contentType, "application/xhtml") {
		md, err := htmltomarkdown.ConvertString(content)
		if err == nil {
			content = md
		}
	}

	// Truncate at 50000 chars.
	if len(content) > 50000 {
		content = content[:50000] + "\n... [content truncated at 50000 chars]"
	}
	if truncatedByBytes {
		content += "\n... [response truncated at 2MB]"
	}

	return &Result{Content: content}, nil
}

// --- WebSearchTool ---

// WebSearchTool searches the web using DuckDuckGo.
type WebSearchTool struct{}

// NewWebSearchTool creates a new WebSearchTool.
func NewWebSearchTool() *WebSearchTool {
	return &WebSearchTool{}
}

func (t *WebSearchTool) Name() string        { return "web_search" }
func (t *WebSearchTool) Description() string { return "Search the web using DuckDuckGo" }
func (t *WebSearchTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
  "type": "object",
  "properties": {
    "query": {"type": "string", "description": "Search query"}
  },
  "required": ["query"]
}`)
}

func (t *WebSearchTool) Execute(ctx context.Context, args json.RawMessage) (*Result, error) {
	var params struct {
		Query string `json:"query"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return &Result{Error: fmt.Sprintf("invalid arguments: %v", err), IsError: true}, nil
	}
	if params.Query == "" {
		return &Result{Error: "query is required", IsError: true}, nil
	}

	searchURL := "https://html.duckduckgo.com/html/?q=" + url.QueryEscape(params.Query)
	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, searchURL, nil)
	if err != nil {
		return &Result{Error: fmt.Sprintf("failed to create request: %v", err), IsError: true}, nil
	}
	req.Header.Set("User-Agent", "EazyClaw/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return &Result{Error: fmt.Sprintf("search failed: %v", err), IsError: true}, nil
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return &Result{Error: fmt.Sprintf("failed to read response: %v", err), IsError: true}, nil
	}

	results := parseDuckDuckGoResults(string(body))
	return &Result{Content: results}, nil
}

// parseDuckDuckGoResults extracts search results from DuckDuckGo HTML response.
func parseDuckDuckGoResults(html string) string {
	var sb strings.Builder
	resultCount := 0

	// DuckDuckGo HTML results are in <a class="result__a"> tags with
	// snippets in <a class="result__snippet"> tags.
	remaining := html
	for {
		// Find result link.
		linkStart := strings.Index(remaining, `class="result__a"`)
		if linkStart == -1 {
			break
		}
		remaining = remaining[linkStart:]

		// Extract href.
		hrefIdx := strings.Index(remaining, `href="`)
		if hrefIdx == -1 {
			break
		}
		remaining = remaining[hrefIdx+6:]
		hrefEnd := strings.Index(remaining, `"`)
		if hrefEnd == -1 {
			break
		}
		href := remaining[:hrefEnd]
		remaining = remaining[hrefEnd:]

		// Extract title (text between > and </a>).
		titleStart := strings.Index(remaining, ">")
		if titleStart == -1 {
			break
		}
		remaining = remaining[titleStart+1:]
		titleEnd := strings.Index(remaining, "</a>")
		if titleEnd == -1 {
			break
		}
		title := stripHTMLTags(remaining[:titleEnd])
		remaining = remaining[titleEnd:]

		// Extract snippet.
		snippet := ""
		snippetStart := strings.Index(remaining, `class="result__snippet"`)
		nextResult := strings.Index(remaining, `class="result__a"`)
		if snippetStart != -1 && (nextResult == -1 || snippetStart < nextResult) {
			snipRemaining := remaining[snippetStart:]
			snipTagStart := strings.Index(snipRemaining, ">")
			if snipTagStart != -1 {
				snipRemaining = snipRemaining[snipTagStart+1:]
				snipEnd := strings.Index(snipRemaining, "</")
				if snipEnd != -1 {
					snippet = stripHTMLTags(snipRemaining[:snipEnd])
				}
			}
		}

		resultCount++
		sb.WriteString(fmt.Sprintf("%d. %s\n   URL: %s\n", resultCount, strings.TrimSpace(title), href))
		if snippet != "" {
			sb.WriteString(fmt.Sprintf("   %s\n", strings.TrimSpace(snippet)))
		}
		sb.WriteString("\n")

		if resultCount >= 10 {
			break
		}
	}

	if resultCount == 0 {
		return "No results found."
	}
	return sb.String()
}

// stripHTMLTags removes HTML tags from a string.
func stripHTMLTags(s string) string {
	var sb strings.Builder
	inTag := false
	for _, r := range s {
		if r == '<' {
			inTag = true
			continue
		}
		if r == '>' {
			inTag = false
			continue
		}
		if !inTag {
			sb.WriteRune(r)
		}
	}
	return sb.String()
}

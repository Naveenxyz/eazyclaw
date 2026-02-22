package tool

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/chromedp/chromedp"
)

// BrowserManager manages a shared Chrome DevTools Protocol browser instance.
type BrowserManager struct {
	allocCtx context.Context
	cancel   context.CancelFunc
	headless bool
	mu       sync.Mutex
	tabCtx   context.Context
	tabCancel context.CancelFunc
}

// NewBrowserManager creates a new BrowserManager with an allocator context.
func NewBrowserManager(headless bool) *BrowserManager {
	opts := chromedp.DefaultExecAllocatorOptions[:]
	if !headless {
		// Remove headless option for visible browser.
		opts = append(opts[:0:0], chromedp.DefaultExecAllocatorOptions[:]...)
		opts = append(opts, chromedp.Flag("headless", false))
	}

	allocCtx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)
	return &BrowserManager{
		allocCtx: allocCtx,
		cancel:   cancel,
		headless: headless,
	}
}

// Close shuts down the browser manager and all contexts.
func (bm *BrowserManager) Close() {
	bm.mu.Lock()
	defer bm.mu.Unlock()
	if bm.tabCancel != nil {
		bm.tabCancel()
	}
	bm.cancel()
}

// getTabCtx returns the shared browser tab context, creating one if needed.
func (bm *BrowserManager) getTabCtx() (context.Context, error) {
	bm.mu.Lock()
	defer bm.mu.Unlock()
	if bm.tabCtx == nil {
		ctx, cancel := chromedp.NewContext(bm.allocCtx)
		bm.tabCtx = ctx
		bm.tabCancel = cancel
	}
	return bm.tabCtx, nil
}

// --- BrowserNavigateTool ---

// BrowserNavigateTool navigates the browser to a URL.
type BrowserNavigateTool struct {
	bm *BrowserManager
}

// NewBrowserNavigateTool creates a new BrowserNavigateTool.
func NewBrowserNavigateTool(bm *BrowserManager) *BrowserNavigateTool {
	return &BrowserNavigateTool{bm: bm}
}

func (t *BrowserNavigateTool) Name() string        { return "browser_navigate" }
func (t *BrowserNavigateTool) Description() string  { return "Navigate the browser to a URL" }
func (t *BrowserNavigateTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
  "type": "object",
  "properties": {
    "url": {"type": "string", "description": "URL to navigate to"}
  },
  "required": ["url"]
}`)
}

func (t *BrowserNavigateTool) Execute(ctx context.Context, args json.RawMessage) (*Result, error) {
	var params struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return &Result{Error: fmt.Sprintf("invalid arguments: %v", err), IsError: true}, nil
	}
	if params.URL == "" {
		return &Result{Error: "url is required", IsError: true}, nil
	}

	tabCtx, err := t.bm.getTabCtx()
	if err != nil {
		return &Result{Error: fmt.Sprintf("failed to get browser context: %v", err), IsError: true}, nil
	}

	if err := chromedp.Run(tabCtx, chromedp.Navigate(params.URL)); err != nil {
		return &Result{Error: fmt.Sprintf("navigation failed: %v", err), IsError: true}, nil
	}

	return &Result{Content: fmt.Sprintf("navigated to %s", params.URL)}, nil
}

// --- BrowserScreenshotTool ---

// BrowserScreenshotTool takes a screenshot of the current page.
type BrowserScreenshotTool struct {
	bm *BrowserManager
}

// NewBrowserScreenshotTool creates a new BrowserScreenshotTool.
func NewBrowserScreenshotTool(bm *BrowserManager) *BrowserScreenshotTool {
	return &BrowserScreenshotTool{bm: bm}
}

func (t *BrowserScreenshotTool) Name() string        { return "browser_screenshot" }
func (t *BrowserScreenshotTool) Description() string  { return "Take a screenshot of the current page" }
func (t *BrowserScreenshotTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
  "type": "object",
  "properties": {}
}`)
}

func (t *BrowserScreenshotTool) Execute(ctx context.Context, args json.RawMessage) (*Result, error) {
	tabCtx, err := t.bm.getTabCtx()
	if err != nil {
		return &Result{Error: fmt.Sprintf("failed to get browser context: %v", err), IsError: true}, nil
	}

	var buf []byte
	if err := chromedp.Run(tabCtx, chromedp.FullScreenshot(&buf, 90)); err != nil {
		return &Result{Error: fmt.Sprintf("screenshot failed: %v", err), IsError: true}, nil
	}

	encoded := base64.StdEncoding.EncodeToString(buf)
	return &Result{Content: encoded}, nil
}

// --- BrowserClickTool ---

// BrowserClickTool clicks an element on the page.
type BrowserClickTool struct {
	bm *BrowserManager
}

// NewBrowserClickTool creates a new BrowserClickTool.
func NewBrowserClickTool(bm *BrowserManager) *BrowserClickTool {
	return &BrowserClickTool{bm: bm}
}

func (t *BrowserClickTool) Name() string        { return "browser_click" }
func (t *BrowserClickTool) Description() string  { return "Click an element by CSS selector" }
func (t *BrowserClickTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
  "type": "object",
  "properties": {
    "selector": {"type": "string", "description": "CSS selector of the element to click"}
  },
  "required": ["selector"]
}`)
}

func (t *BrowserClickTool) Execute(ctx context.Context, args json.RawMessage) (*Result, error) {
	var params struct {
		Selector string `json:"selector"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return &Result{Error: fmt.Sprintf("invalid arguments: %v", err), IsError: true}, nil
	}
	if params.Selector == "" {
		return &Result{Error: "selector is required", IsError: true}, nil
	}

	tabCtx, err := t.bm.getTabCtx()
	if err != nil {
		return &Result{Error: fmt.Sprintf("failed to get browser context: %v", err), IsError: true}, nil
	}

	if err := chromedp.Run(tabCtx, chromedp.Click(params.Selector, chromedp.ByQuery)); err != nil {
		return &Result{Error: fmt.Sprintf("click failed: %v", err), IsError: true}, nil
	}

	return &Result{Content: fmt.Sprintf("clicked %s", params.Selector)}, nil
}

// --- BrowserTypeTool ---

// BrowserTypeTool types text into an element.
type BrowserTypeTool struct {
	bm *BrowserManager
}

// NewBrowserTypeTool creates a new BrowserTypeTool.
func NewBrowserTypeTool(bm *BrowserManager) *BrowserTypeTool {
	return &BrowserTypeTool{bm: bm}
}

func (t *BrowserTypeTool) Name() string        { return "browser_type" }
func (t *BrowserTypeTool) Description() string  { return "Type text into an element" }
func (t *BrowserTypeTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
  "type": "object",
  "properties": {
    "selector": {"type": "string", "description": "CSS selector of the element to type into"},
    "text": {"type": "string", "description": "Text to type"}
  },
  "required": ["selector", "text"]
}`)
}

func (t *BrowserTypeTool) Execute(ctx context.Context, args json.RawMessage) (*Result, error) {
	var params struct {
		Selector string `json:"selector"`
		Text     string `json:"text"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return &Result{Error: fmt.Sprintf("invalid arguments: %v", err), IsError: true}, nil
	}
	if params.Selector == "" {
		return &Result{Error: "selector is required", IsError: true}, nil
	}

	tabCtx, err := t.bm.getTabCtx()
	if err != nil {
		return &Result{Error: fmt.Sprintf("failed to get browser context: %v", err), IsError: true}, nil
	}

	if err := chromedp.Run(tabCtx, chromedp.SendKeys(params.Selector, params.Text, chromedp.ByQuery)); err != nil {
		return &Result{Error: fmt.Sprintf("type failed: %v", err), IsError: true}, nil
	}

	return &Result{Content: fmt.Sprintf("typed into %s", params.Selector)}, nil
}

// --- BrowserEvalTool ---

// BrowserEvalTool executes JavaScript in the browser.
type BrowserEvalTool struct {
	bm *BrowserManager
}

// NewBrowserEvalTool creates a new BrowserEvalTool.
func NewBrowserEvalTool(bm *BrowserManager) *BrowserEvalTool {
	return &BrowserEvalTool{bm: bm}
}

func (t *BrowserEvalTool) Name() string        { return "browser_eval" }
func (t *BrowserEvalTool) Description() string  { return "Execute JavaScript in the browser" }
func (t *BrowserEvalTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
  "type": "object",
  "properties": {
    "expression": {"type": "string", "description": "JavaScript expression to evaluate"}
  },
  "required": ["expression"]
}`)
}

func (t *BrowserEvalTool) Execute(ctx context.Context, args json.RawMessage) (*Result, error) {
	var params struct {
		Expression string `json:"expression"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return &Result{Error: fmt.Sprintf("invalid arguments: %v", err), IsError: true}, nil
	}
	if params.Expression == "" {
		return &Result{Error: "expression is required", IsError: true}, nil
	}

	tabCtx, err := t.bm.getTabCtx()
	if err != nil {
		return &Result{Error: fmt.Sprintf("failed to get browser context: %v", err), IsError: true}, nil
	}

	var result interface{}
	if err := chromedp.Run(tabCtx, chromedp.Evaluate(params.Expression, &result)); err != nil {
		return &Result{Error: fmt.Sprintf("eval failed: %v", err), IsError: true}, nil
	}

	output, err := json.Marshal(result)
	if err != nil {
		return &Result{Content: fmt.Sprintf("%v", result)}, nil
	}
	return &Result{Content: string(output)}, nil
}

// --- BrowserContentTool ---

// BrowserContentTool gets the text content of the current page.
type BrowserContentTool struct {
	bm *BrowserManager
}

// NewBrowserContentTool creates a new BrowserContentTool.
func NewBrowserContentTool(bm *BrowserManager) *BrowserContentTool {
	return &BrowserContentTool{bm: bm}
}

func (t *BrowserContentTool) Name() string        { return "browser_content" }
func (t *BrowserContentTool) Description() string  { return "Get the text content of the current page" }
func (t *BrowserContentTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
  "type": "object",
  "properties": {}
}`)
}

func (t *BrowserContentTool) Execute(ctx context.Context, args json.RawMessage) (*Result, error) {
	tabCtx, err := t.bm.getTabCtx()
	if err != nil {
		return &Result{Error: fmt.Sprintf("failed to get browser context: %v", err), IsError: true}, nil
	}

	var content string
	if err := chromedp.Run(tabCtx, chromedp.Evaluate(`document.body.innerText`, &content)); err != nil {
		return &Result{Error: fmt.Sprintf("failed to get content: %v", err), IsError: true}, nil
	}

	return &Result{Content: content}, nil
}

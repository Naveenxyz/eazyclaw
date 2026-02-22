package channel

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"gopkg.in/yaml.v3"

	"github.com/eazyclaw/eazyclaw/internal/agent"
	"github.com/eazyclaw/eazyclaw/internal/bus"
	"github.com/eazyclaw/eazyclaw/internal/config"
	providerPkg "github.com/eazyclaw/eazyclaw/internal/provider"
	"github.com/eazyclaw/eazyclaw/internal/skill"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

const sessionCookieName = "eazyclaw_session"
const sessionExpiry = 24 * time.Hour

// WebChannel provides a browser-based chat interface and dashboard.
type WebChannel struct {
	port      int
	password  string
	bus       *bus.Bus
	sessions  *agent.SessionStore
	providers *providerPkg.Registry
	channels  []string // names of all active channels (for status)
	server    *http.Server

	mu      sync.RWMutex
	clients map[string]*websocket.Conn // sessionID -> ws conn

	// Auth session store: token -> expiry time.
	authSessions sync.Map

	// Data for API endpoints.
	skills     []skill.Skill
	channelCfg *config.ChannelsConfig
	configPath string

	// Embedded frontend filesystem.
	frontendFS fs.FS
}

// NewWebChannel creates a new WebChannel.
func NewWebChannel(port int, password string, sessions *agent.SessionStore, providers *providerPkg.Registry) *WebChannel {
	return &WebChannel{
		port:      port,
		password:  password,
		sessions:  sessions,
		providers: providers,
		clients:   make(map[string]*websocket.Conn),
	}
}

// SetActiveChannels sets the list of active channel names for the status API.
func (w *WebChannel) SetActiveChannels(names []string) {
	w.channels = names
}

// SetSkills sets the skills data for the /api/skills endpoint.
func (w *WebChannel) SetSkills(skills []skill.Skill) {
	w.skills = skills
}

// SetChannelsConfig sets the channels config for the /api/config endpoint.
func (w *WebChannel) SetChannelsConfig(cfg *config.ChannelsConfig) {
	w.channelCfg = cfg
}

// SetConfigPath sets the path to the config file for write-back.
func (w *WebChannel) SetConfigPath(path string) {
	w.configPath = path
}

// SetFrontendFS sets the embedded frontend filesystem for serving the SPA.
func (w *WebChannel) SetFrontendFS(fsys fs.FS) {
	w.frontendFS = fsys
}

// Name returns the channel identifier.
func (w *WebChannel) Name() string { return "web" }

// Start begins the HTTP server and connects to the bus.
func (w *WebChannel) Start(ctx context.Context, b *bus.Bus) error {
	w.bus = b

	mux := http.NewServeMux()

	// Unauthenticated routes.
	mux.HandleFunc("/api/login", w.handleAPILogin)

	// Authenticated routes.
	mux.Handle("/ws", w.authMiddleware(http.HandlerFunc(w.handleWS)))
	mux.Handle("/api/status", w.authMiddleware(http.HandlerFunc(w.handleStatus)))
	mux.Handle("/api/sessions", w.authMiddleware(http.HandlerFunc(w.handleSessions)))
	mux.Handle("/api/sessions/", w.authMiddleware(http.HandlerFunc(w.handleSessionDetail)))
	mux.Handle("/api/skills", w.authMiddleware(http.HandlerFunc(w.handleSkills)))
	mux.Handle("/api/config", w.authMiddleware(http.HandlerFunc(w.handleConfig)))

	// SPA handler: serve frontend assets or fall back to index.html.
	mux.HandleFunc("/", w.handleSPA)

	w.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", w.port),
		Handler: mux,
	}

	slog.Info("web channel started", "port", w.port, "auth_enabled", w.password != "")
	go func() {
		if err := w.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("web server error", "error", err)
		}
	}()
	return nil
}

// authMiddleware wraps handlers requiring authentication.
func (w *WebChannel) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		// No password configured = no auth required.
		if w.password == "" {
			next.ServeHTTP(rw, r)
			return
		}

		cookie, err := r.Cookie(sessionCookieName)
		if err != nil || cookie.Value == "" {
			http.Error(rw, "unauthorized", http.StatusUnauthorized)
			return
		}

		expiryVal, ok := w.authSessions.Load(cookie.Value)
		if !ok {
			http.Error(rw, "unauthorized", http.StatusUnauthorized)
			return
		}

		expiry, ok := expiryVal.(time.Time)
		if !ok || time.Now().After(expiry) {
			w.authSessions.Delete(cookie.Value)
			http.Error(rw, "session expired", http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(rw, r)
	})
}

// isAuthenticated checks if the request has a valid session cookie.
func (w *WebChannel) isAuthenticated(r *http.Request) bool {
	if w.password == "" {
		return true
	}

	cookie, err := r.Cookie(sessionCookieName)
	if err != nil || cookie.Value == "" {
		return false
	}

	expiryVal, ok := w.authSessions.Load(cookie.Value)
	if !ok {
		return false
	}

	expiry, ok := expiryVal.(time.Time)
	return ok && time.Now().Before(expiry)
}

// handleSPA serves the React SPA. It serves static files from the embedded
// frontend FS, falling back to index.html for client-side routes.
func (w *WebChannel) handleSPA(rw http.ResponseWriter, r *http.Request) {
	if w.frontendFS == nil {
		http.Error(rw, "frontend not available", http.StatusInternalServerError)
		return
	}

	// Try to serve the exact file path.
	path := strings.TrimPrefix(r.URL.Path, "/")
	if path == "" {
		path = "index.html"
	}

	// Check if the file exists in the embedded FS.
	if f, err := w.frontendFS.Open(path); err == nil {
		f.Close()
		http.FileServerFS(w.frontendFS).ServeHTTP(rw, r)
		return
	}

	// Fall back to index.html for SPA client-side routing.
	indexData, err := fs.ReadFile(w.frontendFS, "index.html")
	if err != nil {
		http.Error(rw, "index.html not found", http.StatusInternalServerError)
		return
	}
	rw.Header().Set("Content-Type", "text/html; charset=utf-8")
	rw.Write(indexData)
}

// handleAPILogin validates the password and sets a session cookie.
func (w *WebChannel) handleAPILogin(rw http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(rw, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var body struct {
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(rw, "invalid request", http.StatusBadRequest)
		return
	}

	if body.Password != w.password {
		rw.Header().Set("Content-Type", "application/json")
		rw.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(rw).Encode(map[string]string{"error": "invalid password"})
		return
	}

	// Generate session token.
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		http.Error(rw, "internal error", http.StatusInternalServerError)
		return
	}
	token := hex.EncodeToString(tokenBytes)

	w.authSessions.Store(token, time.Now().Add(sessionExpiry))

	http.SetCookie(rw, &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   int(sessionExpiry.Seconds()),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})

	rw.Header().Set("Content-Type", "application/json")
	json.NewEncoder(rw).Encode(map[string]string{"status": "ok"})
}

// Send delivers an outbound message to the connected WebSocket client.
func (w *WebChannel) Send(ctx context.Context, msg bus.OutboundMessage) error {
	w.mu.RLock()
	conn, ok := w.clients[msg.ChatID]
	w.mu.RUnlock()
	if !ok {
		slog.Warn("web: no connected client for session", "chat_id", msg.ChatID)
		return nil
	}

	payload := map[string]string{
		"type":    "message",
		"role":    "assistant",
		"content": msg.Text,
	}
	data, _ := json.Marshal(payload)
	if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
		slog.Error("web: failed to write ws message", "error", err)
		return err
	}
	return nil
}

// Stop gracefully shuts down the web server.
func (w *WebChannel) Stop() error {
	if w.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := w.server.Shutdown(ctx); err != nil {
			return fmt.Errorf("web: shutdown error: %w", err)
		}
	}
	w.mu.Lock()
	for id, conn := range w.clients {
		conn.Close()
		delete(w.clients, id)
	}
	w.mu.Unlock()
	slog.Info("web channel stopped")
	return nil
}

// handleWS upgrades to WebSocket and handles bidirectional chat messages.
func (w *WebChannel) handleWS(rw http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(rw, r, nil)
	if err != nil {
		slog.Error("web: ws upgrade failed", "error", err)
		return
	}

	senderID := "web-user"

	w.mu.Lock()
	if old, ok := w.clients[senderID]; ok {
		old.Close()
	}
	w.clients[senderID] = conn
	w.mu.Unlock()

	slog.Info("web: client connected", "sender_id", senderID)

	defer func() {
		w.mu.Lock()
		if w.clients[senderID] == conn {
			delete(w.clients, senderID)
		}
		w.mu.Unlock()
		conn.Close()
		slog.Info("web: client disconnected", "sender_id", senderID)
	}()

	for {
		_, msgData, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				slog.Error("web: ws read error", "error", err)
			}
			return
		}

		var incoming struct {
			Text    string `json:"text"`
			Content string `json:"content"`
		}
		if err := json.Unmarshal(msgData, &incoming); err != nil {
			continue
		}
		text := incoming.Text
		if text == "" {
			text = incoming.Content
		}
		if text == "" {
			continue
		}

		w.bus.Inbound <- bus.Message{
			ID:        fmt.Sprintf("web-%d", time.Now().UnixNano()),
			ChannelID: "web",
			SenderID:  senderID,
			Text:      text,
			Timestamp: time.Now(),
		}
	}
}

// handleStatus returns provider/channel status as JSON.
func (w *WebChannel) handleStatus(rw http.ResponseWriter, r *http.Request) {
	providerNames := w.providers.ListProviders()

	type statusResponse struct {
		Providers []string `json:"providers"`
		Channels  []string `json:"channels"`
	}

	resp := statusResponse{
		Providers: providerNames,
		Channels:  w.channels,
	}

	rw.Header().Set("Content-Type", "application/json")
	json.NewEncoder(rw).Encode(resp)
}

// handleSessions returns a list of session summaries.
func (w *WebChannel) handleSessions(rw http.ResponseWriter, r *http.Request) {
	summaries, err := w.sessions.ListSessions()
	if err != nil {
		http.Error(rw, err.Error(), http.StatusInternalServerError)
		return
	}
	if summaries == nil {
		summaries = []agent.SessionSummary{}
	}

	rw.Header().Set("Content-Type", "application/json")
	json.NewEncoder(rw).Encode(summaries)
}

// handleSessionDetail returns the full message history for a session.
func (w *WebChannel) handleSessionDetail(rw http.ResponseWriter, r *http.Request) {
	id := r.URL.Path[len("/api/sessions/"):]
	if id == "" {
		http.Error(rw, "session ID required", http.StatusBadRequest)
		return
	}

	session, err := w.sessions.Load(id)
	if err != nil {
		http.Error(rw, err.Error(), http.StatusInternalServerError)
		return
	}

	rw.Header().Set("Content-Type", "application/json")
	json.NewEncoder(rw).Encode(session)
}

// handleSkills returns the list of loaded skills.
func (w *WebChannel) handleSkills(rw http.ResponseWriter, r *http.Request) {
	type skillToolResp struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Command     string `json:"command"`
	}
	type skillDepResp struct {
		Manager string `json:"manager"`
		Package string `json:"package"`
	}
	type skillResp struct {
		Name         string         `json:"name"`
		Description  string         `json:"description"`
		Tools        []skillToolResp `json:"tools"`
		Dependencies []skillDepResp  `json:"dependencies"`
	}

	var resp []skillResp
	for _, s := range w.skills {
		sr := skillResp{
			Name:        s.Name,
			Description: s.Description,
		}
		for _, t := range s.Tools {
			sr.Tools = append(sr.Tools, skillToolResp{
				Name:        t.Name,
				Description: t.Description,
				Command:     t.Command,
			})
		}
		for _, d := range s.Dependencies {
			sr.Dependencies = append(sr.Dependencies, skillDepResp{
				Manager: d.Manager,
				Package: d.Package,
			})
		}
		if sr.Tools == nil {
			sr.Tools = []skillToolResp{}
		}
		if sr.Dependencies == nil {
			sr.Dependencies = []skillDepResp{}
		}
		resp = append(resp, sr)
	}

	if resp == nil {
		resp = []skillResp{}
	}

	rw.Header().Set("Content-Type", "application/json")
	json.NewEncoder(rw).Encode(resp)
}

// handleConfig handles GET (read) and PUT (write) for channel configuration.
func (w *WebChannel) handleConfig(rw http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		w.handleConfigGet(rw, r)
	case http.MethodPut:
		w.handleConfigPut(rw, r)
	default:
		http.Error(rw, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleConfigGet returns sanitized channel configuration.
func (w *WebChannel) handleConfigGet(rw http.ResponseWriter, r *http.Request) {
	if w.channelCfg == nil {
		rw.Header().Set("Content-Type", "application/json")
		json.NewEncoder(rw).Encode(map[string]any{})
		return
	}

	// Return sanitized config (no tokens, no passwords).
	type sanitizedConfig struct {
		Discord  config.DiscordChannelConfig  `json:"discord"`
		Telegram config.TelegramChannelConfig `json:"telegram"`
		Web      struct {
			Enabled       bool `json:"enabled"`
			Port          int  `json:"port"`
			HasPassword   bool `json:"has_password"`
		} `json:"web"`
	}

	resp := sanitizedConfig{
		Discord:  w.channelCfg.Discord,
		Telegram: w.channelCfg.Telegram,
	}
	resp.Web.Enabled = w.channelCfg.Web.Enabled
	resp.Web.Port = w.channelCfg.Web.Port
	resp.Web.HasPassword = w.channelCfg.Web.Password != ""

	rw.Header().Set("Content-Type", "application/json")
	json.NewEncoder(rw).Encode(resp)
}

// handleConfigPut saves updated channel configuration to the config file.
func (w *WebChannel) handleConfigPut(rw http.ResponseWriter, r *http.Request) {
	if w.configPath == "" {
		http.Error(rw, "config path not set", http.StatusInternalServerError)
		return
	}

	var incoming struct {
		Discord  *config.DiscordChannelConfig  `json:"discord,omitempty"`
		Telegram *config.TelegramChannelConfig `json:"telegram,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&incoming); err != nil {
		http.Error(rw, "invalid request body", http.StatusBadRequest)
		return
	}

	// Read existing config file.
	data, err := os.ReadFile(w.configPath)
	if err != nil {
		http.Error(rw, "failed to read config", http.StatusInternalServerError)
		return
	}

	var fullCfg map[string]any
	if err := yaml.Unmarshal(data, &fullCfg); err != nil {
		http.Error(rw, "failed to parse config", http.StatusInternalServerError)
		return
	}

	// Merge channel changes.
	channels, _ := fullCfg["channels"].(map[string]any)
	if channels == nil {
		channels = make(map[string]any)
	}

	if incoming.Discord != nil {
		channels["discord"] = incoming.Discord
		w.channelCfg.Discord = *incoming.Discord
	}
	if incoming.Telegram != nil {
		channels["telegram"] = incoming.Telegram
		w.channelCfg.Telegram = *incoming.Telegram
	}

	fullCfg["channels"] = channels

	// Write back.
	out, err := yaml.Marshal(fullCfg)
	if err != nil {
		http.Error(rw, "failed to marshal config", http.StatusInternalServerError)
		return
	}
	if err := os.WriteFile(w.configPath, out, 0644); err != nil {
		http.Error(rw, "failed to write config", http.StatusInternalServerError)
		return
	}

	rw.Header().Set("Content-Type", "application/json")
	json.NewEncoder(rw).Encode(map[string]string{
		"status":  "ok",
		"message": "Config saved. Restart container to apply changes.",
	})
}

// Ensure WebChannel satisfies the Channel interface at compile time.
var _ Channel = (*WebChannel)(nil)

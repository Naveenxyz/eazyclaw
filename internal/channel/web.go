package channel

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
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
	"github.com/eazyclaw/eazyclaw/internal/tool"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

const sessionCookieName = "eazyclaw_session"
const sessionExpiry = 24 * time.Hour

// Login rate limiting.
const (
	loginMaxAttempts = 5
	loginLockoutDur  = 2 * time.Minute
)

type loginAttempt struct {
	count    int
	lockedAt time.Time
}

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

	// Login rate limiter: IP -> loginAttempt.
	loginAttempts sync.Map

	// Data for API endpoints.
	skills     []skill.Skill
	channelCfg *config.ChannelsConfig
	configPath string

	// Embedded frontend filesystem.
	frontendFS fs.FS

	// Memory explorer directory.
	memoryDir string

	discord  *DiscordChannel
	telegram *TelegramChannel
	whatsapp *WhatsAppChannel

	// Google OAuth for Gmail + Calendar tools.
	googleAuth *tool.GoogleAuth

	// Monitoring: cron and heartbeat.
	cronManager     tool.CronManager
	heartbeatRunner *agent.HeartbeatRunner
	heartbeatCfg    *config.HeartbeatConfig
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

// SetMemoryDir sets the memory directory path for the memory explorer API.
func (w *WebChannel) SetMemoryDir(dir string) {
	w.memoryDir = dir
}


// SetDiscordChannel sets a live Discord channel reference for admin APIs.
func (w *WebChannel) SetDiscordChannel(ch *DiscordChannel) {
	w.discord = ch
}

// SetTelegramChannel sets a live Telegram channel reference for admin APIs.
func (w *WebChannel) SetTelegramChannel(ch *TelegramChannel) {
	w.telegram = ch
}

// SetWhatsAppChannel sets a live WhatsApp channel reference for admin APIs.
func (w *WebChannel) SetWhatsAppChannel(ch *WhatsAppChannel) {
	w.whatsapp = ch
}

// SetGoogleAuth sets the Google OAuth manager for the OAuth flow APIs.
func (w *WebChannel) SetGoogleAuth(ga *tool.GoogleAuth) {
	w.googleAuth = ga
}

// SetCronManager sets the cron manager for the cron API.
func (w *WebChannel) SetCronManager(cm tool.CronManager) {
	w.cronManager = cm
}

// SetHeartbeatRunner sets the heartbeat runner for monitoring.
func (w *WebChannel) SetHeartbeatRunner(hb *agent.HeartbeatRunner) {
	w.heartbeatRunner = hb
}

// SetHeartbeatConfig sets the heartbeat config for status display.
func (w *WebChannel) SetHeartbeatConfig(cfg *config.HeartbeatConfig) {
	w.heartbeatCfg = cfg
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
	mux.HandleFunc("/api/status", w.handleStatus) // Unauthenticated — used as healthcheck endpoint
	mux.Handle("/api/sessions", w.authMiddleware(http.HandlerFunc(w.handleSessions)))
	mux.Handle("/api/sessions/", w.authMiddleware(http.HandlerFunc(w.handleSessionDetail)))
	mux.Handle("/api/skills", w.authMiddleware(http.HandlerFunc(w.handleSkills)))
	mux.Handle("/api/config", w.authMiddleware(http.HandlerFunc(w.handleConfig)))
	mux.Handle("/api/discord", w.authMiddleware(http.HandlerFunc(w.handleDiscordAdmin)))
	mux.Handle("/api/discord/approvals", w.authMiddleware(http.HandlerFunc(w.handleDiscordApprovals)))
	mux.Handle("/api/telegram", w.authMiddleware(http.HandlerFunc(w.handleTelegramAdmin)))
	mux.Handle("/api/telegram/approvals", w.authMiddleware(http.HandlerFunc(w.handleTelegramApprovals)))
	mux.Handle("/api/memory/", w.authMiddleware(http.HandlerFunc(w.handleMemoryPath)))
	mux.Handle("/api/memory", w.authMiddleware(http.HandlerFunc(w.handleMemoryList)))
	mux.Handle("/api/cron/", w.authMiddleware(http.HandlerFunc(w.handleCronJob)))
	mux.Handle("/api/cron", w.authMiddleware(http.HandlerFunc(w.handleCron)))
	mux.Handle("/api/heartbeat", w.authMiddleware(http.HandlerFunc(w.handleHeartbeat)))

	// WhatsApp admin APIs.
	mux.Handle("/api/whatsapp/status", w.authMiddleware(http.HandlerFunc(w.handleWhatsAppStatus)))
	mux.Handle("/api/whatsapp/qr", w.authMiddleware(http.HandlerFunc(w.handleWhatsAppQR)))
	mux.Handle("/api/whatsapp/disconnect", w.authMiddleware(http.HandlerFunc(w.handleWhatsAppDisconnect)))
	mux.Handle("/api/whatsapp/settings", w.authMiddleware(http.HandlerFunc(w.handleWhatsAppSettings)))
	mux.Handle("/api/whatsapp/approve", w.authMiddleware(http.HandlerFunc(w.handleWhatsAppApprove)))
	mux.Handle("/api/whatsapp/reject", w.authMiddleware(http.HandlerFunc(w.handleWhatsAppReject)))

	// Google OAuth APIs.
	mux.Handle("/api/google/status", w.authMiddleware(http.HandlerFunc(w.handleGoogleStatus)))
	mux.Handle("/api/google/auth/url", w.authMiddleware(http.HandlerFunc(w.handleGoogleAuthURL)))
	mux.HandleFunc("/api/google/auth/callback", w.handleGoogleAuthCallback) // No auth — OAuth redirect
	mux.Handle("/api/google/disconnect", w.authMiddleware(http.HandlerFunc(w.handleGoogleDisconnect)))

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

// clientIP extracts the client IP for rate limiting.
func clientIP(r *http.Request) string {
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		if i := strings.IndexByte(fwd, ','); i > 0 {
			return strings.TrimSpace(fwd[:i])
		}
		return strings.TrimSpace(fwd)
	}
	if host, _, ok := strings.Cut(r.RemoteAddr, ":"); ok {
		return host
	}
	return r.RemoteAddr
}

// isSecureRequest returns true if the request arrived over HTTPS
// (direct TLS or via a reverse proxy like Railway/nginx).
func isSecureRequest(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	return strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
}

// handleAPILogin validates the password and sets a session cookie.
func (w *WebChannel) handleAPILogin(rw http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(rw, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Rate limiting by client IP.
	ip := clientIP(r)
	if val, ok := w.loginAttempts.Load(ip); ok {
		attempt := val.(*loginAttempt)
		if !attempt.lockedAt.IsZero() && time.Since(attempt.lockedAt) < loginLockoutDur {
			remaining := loginLockoutDur - time.Since(attempt.lockedAt)
			rw.Header().Set("Content-Type", "application/json")
			rw.Header().Set("Retry-After", fmt.Sprintf("%d", int(remaining.Seconds())))
			rw.WriteHeader(http.StatusTooManyRequests)
			json.NewEncoder(rw).Encode(map[string]string{
				"error": fmt.Sprintf("Too many attempts. Try again in %d seconds.", int(remaining.Seconds())),
			})
			return
		}
		// Reset if lockout has expired.
		if !attempt.lockedAt.IsZero() && time.Since(attempt.lockedAt) >= loginLockoutDur {
			attempt.count = 0
			attempt.lockedAt = time.Time{}
		}
	}

	var body struct {
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(rw, "invalid request", http.StatusBadRequest)
		return
	}

	if body.Password != w.password {
		// Track failed attempt.
		val, _ := w.loginAttempts.LoadOrStore(ip, &loginAttempt{})
		attempt := val.(*loginAttempt)
		attempt.count++
		if attempt.count >= loginMaxAttempts {
			attempt.lockedAt = time.Now()
			slog.Warn("login rate limit triggered", "ip", ip)
		}

		rw.Header().Set("Content-Type", "application/json")
		rw.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(rw).Encode(map[string]string{"error": "invalid password"})
		return
	}

	// Successful login — clear any tracked attempts.
	w.loginAttempts.Delete(ip)

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
		Secure:   isSecureRequest(r),
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
			UserID:    senderID,
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
		Name         string          `json:"name"`
		Description  string          `json:"description"`
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
		WhatsApp config.WhatsAppChannelConfig `json:"whatsapp"`
		Web      struct {
			Enabled     bool `json:"enabled"`
			Port        int  `json:"port"`
			HasPassword bool `json:"has_password"`
		} `json:"web"`
	}

	resp := sanitizedConfig{
		Discord:  w.channelCfg.Discord,
		Telegram: w.channelCfg.Telegram,
		WhatsApp: w.channelCfg.WhatsApp,
	}
	if resp.Discord.AllowedUsers == nil {
		resp.Discord.AllowedUsers = []string{}
	}
	if resp.Telegram.AllowedUsers == nil {
		resp.Telegram.AllowedUsers = []string{}
	}
	if resp.WhatsApp.AllowedUsers == nil {
		resp.WhatsApp.AllowedUsers = []string{}
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
		WhatsApp *config.WhatsAppChannelConfig `json:"whatsapp,omitempty"`
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
		if w.discord != nil {
			w.discord.ApplyConfig(*incoming.Discord)
		}
	}
	if incoming.Telegram != nil {
		channels["telegram"] = incoming.Telegram
		w.channelCfg.Telegram = *incoming.Telegram
		if w.telegram != nil {
			w.telegram.ApplyConfig(*incoming.Telegram)
		}
	}
	if incoming.WhatsApp != nil {
		channels["whatsapp"] = incoming.WhatsApp
		w.channelCfg.WhatsApp = *incoming.WhatsApp
		if w.whatsapp != nil {
			w.whatsapp.ApplyConfig(*incoming.WhatsApp)
		}
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

func (w *WebChannel) handleDiscordAdmin(rw http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(rw, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	state := channelDiscordSnapshot(w.channelCfg, w.discord)
	rw.Header().Set("Content-Type", "application/json")
	json.NewEncoder(rw).Encode(state)
}

func (w *WebChannel) handleDiscordApprovals(rw http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		state := channelDiscordSnapshot(w.channelCfg, w.discord)
		rw.Header().Set("Content-Type", "application/json")
		json.NewEncoder(rw).Encode(state)
		return
	case http.MethodPost:
	default:
		http.Error(rw, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Action string `json:"action"`
		UserID string `json:"user_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(rw, "invalid request body", http.StatusBadRequest)
		return
	}

	userID := strings.TrimSpace(req.UserID)
	if userID == "" {
		http.Error(rw, "user_id required", http.StatusBadRequest)
		return
	}

	switch strings.ToLower(strings.TrimSpace(req.Action)) {
	case "approve":
		if w.discord != nil {
			w.discord.ApproveUser(userID)
		}
		if err := w.persistDiscordAllowedUser(userID); err != nil {
			http.Error(rw, err.Error(), http.StatusInternalServerError)
			return
		}
	case "reject":
		if w.discord != nil {
			w.discord.RejectUser(userID)
		}
	default:
		http.Error(rw, "action must be approve or reject", http.StatusBadRequest)
		return
	}

	state := channelDiscordSnapshot(w.channelCfg, w.discord)
	rw.Header().Set("Content-Type", "application/json")
	json.NewEncoder(rw).Encode(state)
}

func channelDiscordSnapshot(cfg *config.ChannelsConfig, ch *DiscordChannel) DiscordAdminState {
	if ch != nil {
		state := ch.Snapshot()
		if state.AllowedUsers == nil {
			state.AllowedUsers = []string{}
		}
		if state.Pending == nil {
			state.Pending = []DiscordPendingApproval{}
		}
		return state
	}

	state := DiscordAdminState{
		AllowedUsers: []string{},
		Pending:      []DiscordPendingApproval{},
	}
	if cfg != nil {
		state.GroupPolicy = cfg.Discord.GroupPolicy
		state.DMPolicy = cfg.Discord.DM.Policy
		if len(cfg.Discord.AllowedUsers) > 0 {
			state.AllowedUsers = append([]string{}, cfg.Discord.AllowedUsers...)
		}
	}
	return state
}

func (w *WebChannel) persistDiscordAllowedUser(userID string) error {
	if w.channelCfg == nil {
		return fmt.Errorf("channel config unavailable")
	}
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return fmt.Errorf("invalid user id")
	}

	exists := false
	for _, u := range w.channelCfg.Discord.AllowedUsers {
		if strings.TrimSpace(u) == userID {
			exists = true
			break
		}
	}
	if !exists {
		w.channelCfg.Discord.AllowedUsers = append(w.channelCfg.Discord.AllowedUsers, userID)
	}

	if w.configPath == "" {
		return nil
	}

	data, err := os.ReadFile(w.configPath)
	if err != nil {
		return fmt.Errorf("failed to read config: %w", err)
	}

	var fullCfg map[string]any
	if err := yaml.Unmarshal(data, &fullCfg); err != nil {
		return fmt.Errorf("failed to parse config: %w", err)
	}

	channels, _ := fullCfg["channels"].(map[string]any)
	if channels == nil {
		channels = make(map[string]any)
	}
	channels["discord"] = w.channelCfg.Discord
	fullCfg["channels"] = channels

	out, err := yaml.Marshal(fullCfg)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}
	if err := os.WriteFile(w.configPath, out, 0o644); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}
	return nil
}

// --- Telegram Admin API ---

func (w *WebChannel) handleTelegramAdmin(rw http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(rw, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	state := channelTelegramSnapshot(w.channelCfg, w.telegram)
	rw.Header().Set("Content-Type", "application/json")
	json.NewEncoder(rw).Encode(state)
}

func (w *WebChannel) handleTelegramApprovals(rw http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		state := channelTelegramSnapshot(w.channelCfg, w.telegram)
		rw.Header().Set("Content-Type", "application/json")
		json.NewEncoder(rw).Encode(state)
		return
	case http.MethodPost:
	default:
		http.Error(rw, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Action string `json:"action"`
		UserID string `json:"user_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(rw, "invalid request body", http.StatusBadRequest)
		return
	}

	userID := strings.TrimSpace(req.UserID)
	if userID == "" {
		http.Error(rw, "user_id required", http.StatusBadRequest)
		return
	}

	switch strings.ToLower(strings.TrimSpace(req.Action)) {
	case "approve":
		if w.telegram != nil {
			w.telegram.ApproveUser(userID)
		}
		if err := w.persistTelegramAllowedUser(userID); err != nil {
			http.Error(rw, err.Error(), http.StatusInternalServerError)
			return
		}
	case "reject":
		if w.telegram != nil {
			w.telegram.RejectUser(userID)
		}
	default:
		http.Error(rw, "action must be approve or reject", http.StatusBadRequest)
		return
	}

	state := channelTelegramSnapshot(w.channelCfg, w.telegram)
	rw.Header().Set("Content-Type", "application/json")
	json.NewEncoder(rw).Encode(state)
}

func channelTelegramSnapshot(cfg *config.ChannelsConfig, ch *TelegramChannel) TelegramAdminState {
	if ch != nil {
		state := ch.Snapshot()
		if state.AllowedUsers == nil {
			state.AllowedUsers = []string{}
		}
		if state.Pending == nil {
			state.Pending = []TelegramPendingApproval{}
		}
		return state
	}

	state := TelegramAdminState{
		AllowedUsers: []string{},
		Pending:      []TelegramPendingApproval{},
	}
	if cfg != nil {
		state.GroupPolicy = cfg.Telegram.GroupPolicy
		state.DMPolicy = cfg.Telegram.DM.Policy
		if len(cfg.Telegram.AllowedUsers) > 0 {
			state.AllowedUsers = append([]string{}, cfg.Telegram.AllowedUsers...)
		}
	}
	return state
}

func (w *WebChannel) persistTelegramAllowedUser(userID string) error {
	if w.channelCfg == nil {
		return fmt.Errorf("channel config unavailable")
	}
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return fmt.Errorf("invalid user id")
	}

	exists := false
	for _, u := range w.channelCfg.Telegram.AllowedUsers {
		if strings.TrimSpace(u) == userID {
			exists = true
			break
		}
	}
	if !exists {
		w.channelCfg.Telegram.AllowedUsers = append(w.channelCfg.Telegram.AllowedUsers, userID)
	}

	if w.configPath == "" {
		return nil
	}

	data, err := os.ReadFile(w.configPath)
	if err != nil {
		return fmt.Errorf("failed to read config: %w", err)
	}

	var fullCfg map[string]any
	if err := yaml.Unmarshal(data, &fullCfg); err != nil {
		return fmt.Errorf("failed to parse config: %w", err)
	}

	channels, _ := fullCfg["channels"].(map[string]any)
	if channels == nil {
		channels = make(map[string]any)
	}
	channels["telegram"] = w.channelCfg.Telegram
	fullCfg["channels"] = channels

	out, err := yaml.Marshal(fullCfg)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}
	if err := os.WriteFile(w.configPath, out, 0o644); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}
	return nil
}

// --- Memory Explorer API ---

// memoryNode represents a file or directory in the memory tree.
type memoryNode struct {
	Name     string       `json:"name"`
	Path     string       `json:"path"`
	Type     string       `json:"type"` // "file" or "dir"
	Children []memoryNode `json:"children,omitempty"`
}

// handleMemoryList returns a JSON tree of all files in the memory directory.
func (w *WebChannel) handleMemoryList(rw http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(rw, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if w.memoryDir == "" {
		http.Error(rw, "memory directory not configured", http.StatusInternalServerError)
		return
	}

	root := memoryNode{Name: "memory", Path: "", Type: "dir"}
	root.Children = buildMemoryTree(w.memoryDir, "")

	rw.Header().Set("Content-Type", "application/json")
	json.NewEncoder(rw).Encode(root)
}

// buildMemoryTree recursively builds a tree of files and directories.
func buildMemoryTree(baseDir, relPath string) []memoryNode {
	absDir := filepath.Join(baseDir, relPath)
	entries, err := os.ReadDir(absDir)
	if err != nil {
		return nil
	}

	var nodes []memoryNode
	for _, entry := range entries {
		entryRel := filepath.Join(relPath, entry.Name())
		node := memoryNode{
			Name: entry.Name(),
			Path: entryRel,
		}
		if entry.IsDir() {
			node.Type = "dir"
			node.Children = buildMemoryTree(baseDir, entryRel)
		} else {
			node.Type = "file"
		}
		nodes = append(nodes, node)
	}
	return nodes
}

// handleMemoryPath handles GET (read) and PUT (write) for individual memory files.
func (w *WebChannel) handleMemoryPath(rw http.ResponseWriter, r *http.Request) {
	if w.memoryDir == "" {
		http.Error(rw, "memory directory not configured", http.StatusInternalServerError)
		return
	}

	// Extract path after /api/memory/
	relPath := strings.TrimPrefix(r.URL.Path, "/api/memory/")
	if relPath == "" {
		w.handleMemoryList(rw, r)
		return
	}

	// Path traversal protection.
	if strings.Contains(relPath, "..") {
		http.Error(rw, "invalid path", http.StatusBadRequest)
		return
	}

	absPath := filepath.Join(w.memoryDir, relPath)
	// Ensure resolved path is still within memoryDir.
	resolved, err := filepath.Abs(absPath)
	if err != nil || !strings.HasPrefix(resolved, w.memoryDir) {
		http.Error(rw, "invalid path", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		data, err := os.ReadFile(absPath)
		if err != nil {
			if os.IsNotExist(err) {
				http.Error(rw, "file not found", http.StatusNotFound)
			} else {
				http.Error(rw, "failed to read file", http.StatusInternalServerError)
			}
			return
		}
		rw.Header().Set("Content-Type", "application/json")
		json.NewEncoder(rw).Encode(map[string]string{
			"path":    relPath,
			"content": string(data),
		})

	case http.MethodPut:
		body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20)) // 1MB limit
		if err != nil {
			http.Error(rw, "failed to read request body", http.StatusBadRequest)
			return
		}

		var payload struct {
			Content string `json:"content"`
		}
		if err := json.Unmarshal(body, &payload); err != nil {
			http.Error(rw, "invalid request body", http.StatusBadRequest)
			return
		}

		// Create parent directories if needed.
		if err := os.MkdirAll(filepath.Dir(absPath), 0o755); err != nil {
			http.Error(rw, "failed to create directories", http.StatusInternalServerError)
			return
		}

		if err := os.WriteFile(absPath, []byte(payload.Content), 0o644); err != nil {
			http.Error(rw, "failed to write file", http.StatusInternalServerError)
			return
		}

		rw.Header().Set("Content-Type", "application/json")
		json.NewEncoder(rw).Encode(map[string]string{"status": "ok", "path": relPath})

	default:
		http.Error(rw, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// --- Cron API ---

func (w *WebChannel) handleCron(rw http.ResponseWriter, r *http.Request) {
	if w.cronManager == nil {
		http.Error(rw, "cron not configured", http.StatusServiceUnavailable)
		return
	}

	switch r.Method {
	case http.MethodGet:
		jobs := w.cronManager.ListJobs()
		if jobs == nil {
			jobs = []tool.CronJob{}
		}
		rw.Header().Set("Content-Type", "application/json")
		json.NewEncoder(rw).Encode(jobs)

	case http.MethodPost:
		var req struct {
			Schedule string `json:"schedule"`
			Task     string `json:"task"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(rw, "invalid request body", http.StatusBadRequest)
			return
		}
		if req.Schedule == "" || req.Task == "" {
			http.Error(rw, "schedule and task are required", http.StatusBadRequest)
			return
		}
		id, err := w.cronManager.AddJob(req.Schedule, req.Task)
		if err != nil {
			http.Error(rw, err.Error(), http.StatusBadRequest)
			return
		}
		rw.Header().Set("Content-Type", "application/json")
		json.NewEncoder(rw).Encode(map[string]string{"id": id, "status": "created"})

	default:
		http.Error(rw, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (w *WebChannel) handleCronJob(rw http.ResponseWriter, r *http.Request) {
	if w.cronManager == nil {
		http.Error(rw, "cron not configured", http.StatusServiceUnavailable)
		return
	}

	// Extract job ID from path: /api/cron/{id} or /api/cron/{id}/toggle
	path := strings.TrimPrefix(r.URL.Path, "/api/cron/")
	parts := strings.SplitN(path, "/", 2)
	jobID := parts[0]
	action := ""
	if len(parts) > 1 {
		action = parts[1]
	}

	if jobID == "" {
		http.Error(rw, "job ID required", http.StatusBadRequest)
		return
	}

	switch {
	case action == "toggle" && r.Method == http.MethodPost:
		// Find current state and toggle
		jobs := w.cronManager.ListJobs()
		for _, j := range jobs {
			if j.ID == jobID {
				err := w.cronManager.UpdateJob(jobID, "", "", !j.Enabled)
				if err != nil {
					http.Error(rw, err.Error(), http.StatusInternalServerError)
					return
				}
				rw.Header().Set("Content-Type", "application/json")
				json.NewEncoder(rw).Encode(map[string]any{"id": jobID, "enabled": !j.Enabled})
				return
			}
		}
		http.Error(rw, "job not found", http.StatusNotFound)

	case r.Method == http.MethodPut:
		var req struct {
			Schedule string `json:"schedule"`
			Task     string `json:"task"`
			Enabled  *bool  `json:"enabled"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(rw, "invalid request body", http.StatusBadRequest)
			return
		}
		enabled := true
		if req.Enabled != nil {
			enabled = *req.Enabled
		}
		if err := w.cronManager.UpdateJob(jobID, req.Schedule, req.Task, enabled); err != nil {
			http.Error(rw, err.Error(), http.StatusBadRequest)
			return
		}
		rw.Header().Set("Content-Type", "application/json")
		json.NewEncoder(rw).Encode(map[string]string{"id": jobID, "status": "updated"})

	case r.Method == http.MethodDelete:
		if err := w.cronManager.RemoveJob(jobID); err != nil {
			http.Error(rw, err.Error(), http.StatusNotFound)
			return
		}
		rw.Header().Set("Content-Type", "application/json")
		json.NewEncoder(rw).Encode(map[string]string{"id": jobID, "status": "deleted"})

	default:
		http.Error(rw, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// --- Heartbeat API ---

func (w *WebChannel) handleHeartbeat(rw http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(rw, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if w.heartbeatRunner != nil {
		status := w.heartbeatRunner.Status()
		rw.Header().Set("Content-Type", "application/json")
		json.NewEncoder(rw).Encode(status)
		return
	}

	// Heartbeat not running - return disabled status
	resp := agent.HeartbeatStatus{
		Enabled: false,
	}
	if w.heartbeatCfg != nil {
		resp.Interval = w.heartbeatCfg.Interval.String()
	}
	rw.Header().Set("Content-Type", "application/json")
	json.NewEncoder(rw).Encode(resp)
}

// --- WhatsApp Admin API ---

func (w *WebChannel) handleWhatsAppStatus(rw http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(rw, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	state := channelWhatsAppSnapshot(w.channelCfg, w.whatsapp)
	rw.Header().Set("Content-Type", "application/json")
	json.NewEncoder(rw).Encode(state)
}

func (w *WebChannel) handleWhatsAppQR(rw http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(rw, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if w.whatsapp == nil {
		http.Error(rw, "whatsapp not configured", http.StatusServiceUnavailable)
		return
	}

	qr := w.whatsapp.QRCode()
	if qr == "" {
		http.Error(rw, "no QR code available", http.StatusNotFound)
		return
	}

	rw.Header().Set("Content-Type", "application/json")
	json.NewEncoder(rw).Encode(map[string]string{"qr_code": qr})
}

func (w *WebChannel) handleWhatsAppDisconnect(rw http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(rw, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if w.whatsapp == nil {
		http.Error(rw, "whatsapp not configured", http.StatusServiceUnavailable)
		return
	}

	w.whatsapp.Disconnect()
	rw.Header().Set("Content-Type", "application/json")
	json.NewEncoder(rw).Encode(map[string]string{"status": "disconnected"})
}

func (w *WebChannel) handleWhatsAppSettings(rw http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		http.Error(rw, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		AllowedUsers []string `json:"allowed_users,omitempty"`
		GroupPolicy  string   `json:"group_policy,omitempty"`
		DMPolicy     string   `json:"dm_policy,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(rw, "invalid request body", http.StatusBadRequest)
		return
	}

	if w.channelCfg == nil {
		http.Error(rw, "channel config unavailable", http.StatusInternalServerError)
		return
	}

	// Apply changes to config.
	if req.AllowedUsers != nil {
		w.channelCfg.WhatsApp.AllowedUsers = req.AllowedUsers
	}
	if req.GroupPolicy != "" {
		w.channelCfg.WhatsApp.GroupPolicy = req.GroupPolicy
	}
	if req.DMPolicy != "" {
		w.channelCfg.WhatsApp.DM.Policy = req.DMPolicy
	}

	// Hot-reload to channel.
	if w.whatsapp != nil {
		w.whatsapp.ApplyConfig(w.channelCfg.WhatsApp)
	}

	// Persist to config file.
	if err := w.persistWhatsAppConfig(); err != nil {
		http.Error(rw, err.Error(), http.StatusInternalServerError)
		return
	}

	rw.Header().Set("Content-Type", "application/json")
	json.NewEncoder(rw).Encode(map[string]string{"status": "ok"})
}

func (w *WebChannel) handleWhatsAppApprove(rw http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(rw, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		UserID string `json:"user_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(rw, "invalid request body", http.StatusBadRequest)
		return
	}

	userID := strings.TrimSpace(req.UserID)
	if userID == "" {
		http.Error(rw, "user_id required", http.StatusBadRequest)
		return
	}

	if w.whatsapp != nil {
		w.whatsapp.ApproveUser(userID)
	}
	if err := w.persistWhatsAppAllowedUser(userID); err != nil {
		http.Error(rw, err.Error(), http.StatusInternalServerError)
		return
	}

	state := channelWhatsAppSnapshot(w.channelCfg, w.whatsapp)
	rw.Header().Set("Content-Type", "application/json")
	json.NewEncoder(rw).Encode(state)
}

func (w *WebChannel) handleWhatsAppReject(rw http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(rw, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		UserID string `json:"user_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(rw, "invalid request body", http.StatusBadRequest)
		return
	}

	userID := strings.TrimSpace(req.UserID)
	if userID == "" {
		http.Error(rw, "user_id required", http.StatusBadRequest)
		return
	}

	if w.whatsapp != nil {
		w.whatsapp.RejectUser(userID)
	}

	state := channelWhatsAppSnapshot(w.channelCfg, w.whatsapp)
	rw.Header().Set("Content-Type", "application/json")
	json.NewEncoder(rw).Encode(state)
}

func channelWhatsAppSnapshot(cfg *config.ChannelsConfig, ch *WhatsAppChannel) WhatsAppAdminState {
	if ch != nil {
		return ch.Snapshot()
	}

	state := WhatsAppAdminState{
		AllowedUsers: []string{},
		Pending:      []WhatsAppPendingApproval{},
		Status:       "disconnected",
	}
	if cfg != nil {
		state.GroupPolicy = cfg.WhatsApp.GroupPolicy
		state.DMPolicy = cfg.WhatsApp.DM.Policy
		if len(cfg.WhatsApp.AllowedUsers) > 0 {
			state.AllowedUsers = append([]string{}, cfg.WhatsApp.AllowedUsers...)
		}
	}
	return state
}

func (w *WebChannel) persistWhatsAppConfig() error {
	if w.configPath == "" {
		return nil
	}

	data, err := os.ReadFile(w.configPath)
	if err != nil {
		return fmt.Errorf("failed to read config: %w", err)
	}

	var fullCfg map[string]any
	if err := yaml.Unmarshal(data, &fullCfg); err != nil {
		return fmt.Errorf("failed to parse config: %w", err)
	}

	channels, _ := fullCfg["channels"].(map[string]any)
	if channels == nil {
		channels = make(map[string]any)
	}
	channels["whatsapp"] = w.channelCfg.WhatsApp
	fullCfg["channels"] = channels

	out, err := yaml.Marshal(fullCfg)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}
	if err := os.WriteFile(w.configPath, out, 0o644); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}
	return nil
}

func (w *WebChannel) persistWhatsAppAllowedUser(userID string) error {
	if w.channelCfg == nil {
		return fmt.Errorf("channel config unavailable")
	}
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return fmt.Errorf("invalid user id")
	}

	exists := false
	for _, u := range w.channelCfg.WhatsApp.AllowedUsers {
		if strings.TrimSpace(u) == userID {
			exists = true
			break
		}
	}
	if !exists {
		w.channelCfg.WhatsApp.AllowedUsers = append(w.channelCfg.WhatsApp.AllowedUsers, userID)
	}

	return w.persistWhatsAppConfig()
}

// --- Google OAuth API ---

func (w *WebChannel) handleGoogleStatus(rw http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(rw, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	authenticated := false
	if w.googleAuth != nil {
		authenticated = w.googleAuth.IsAuthenticated()
	}

	rw.Header().Set("Content-Type", "application/json")
	json.NewEncoder(rw).Encode(map[string]bool{"authenticated": authenticated})
}

func (w *WebChannel) handleGoogleAuthURL(rw http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(rw, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if w.googleAuth == nil {
		http.Error(rw, "google not configured", http.StatusServiceUnavailable)
		return
	}

	url := w.googleAuth.AuthURL()
	rw.Header().Set("Content-Type", "application/json")
	json.NewEncoder(rw).Encode(map[string]string{"url": url})
}

func (w *WebChannel) handleGoogleAuthCallback(rw http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(rw, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if w.googleAuth == nil {
		http.Error(rw, "google not configured", http.StatusServiceUnavailable)
		return
	}

	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(rw, "missing authorization code", http.StatusBadRequest)
		return
	}

	if err := w.googleAuth.HandleCallback(code); err != nil {
		slog.Error("google oauth callback failed", "error", err)
		http.Error(rw, "OAuth failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Redirect back to settings page.
	http.Redirect(rw, r, "/settings", http.StatusFound)
}

func (w *WebChannel) handleGoogleDisconnect(rw http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(rw, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if w.googleAuth == nil {
		http.Error(rw, "google not configured", http.StatusServiceUnavailable)
		return
	}

	if err := w.googleAuth.Revoke(); err != nil {
		slog.Error("google disconnect failed", "error", err)
		http.Error(rw, "disconnect failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	rw.Header().Set("Content-Type", "application/json")
	json.NewEncoder(rw).Encode(map[string]string{"status": "disconnected"})
}

// Ensure WebChannel satisfies the Channel interface at compile time.
var _ Channel = (*WebChannel)(nil)

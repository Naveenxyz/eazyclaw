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
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
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
	"github.com/eazyclaw/eazyclaw/internal/state"
	"github.com/eazyclaw/eazyclaw/internal/tool"
)

const sessionCookieName = "eazyclaw_session"
const sessionExpiry = 24 * time.Hour

// Login rate limiting.
const (
	loginMaxAttempts    = 5
	loginLockoutDur     = 2 * time.Minute
	googleOAuthStateTTL = 10 * time.Minute
)

type loginAttempt struct {
	count    int
	lockedAt time.Time
}

type googleOAuthState struct {
	SessionToken string
	ExpiresAt    time.Time
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

	// SQLite state store for mutable runtime data.
	store *state.Store

	// Google OAuth for Gmail + Calendar tools.
	googleAuth *tool.GoogleAuth

	// Google OAuth state store: state -> bound auth session and expiry.
	googleOAuthStates sync.Map

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

// SetStateStore sets the SQLite state store for mutable runtime data.
func (w *WebChannel) SetStateStore(s *state.Store) {
	w.store = s
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
	remoteHost := r.RemoteAddr
	if host, _, ok := strings.Cut(r.RemoteAddr, ":"); ok {
		remoteHost = host
	}
	remoteIP := net.ParseIP(strings.TrimSpace(remoteHost))
	if remoteIP == nil {
		return strings.TrimSpace(remoteHost)
	}

	// Trust X-Forwarded-For only when a local/private reverse proxy is in front.
	if remoteIP.IsLoopback() || remoteIP.IsPrivate() {
		if fwd := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); fwd != "" {
			if i := strings.IndexByte(fwd, ','); i >= 0 {
				fwd = strings.TrimSpace(fwd[:i])
			}
			if ip := net.ParseIP(fwd); ip != nil {
				return ip.String()
			}
		}
	}
	return remoteIP.String()
}

// isSecureRequest returns true if the request arrived over HTTPS
// (direct TLS or via a reverse proxy like Railway/nginx).
func isSecureRequest(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	return strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
}

func parseHostPort(hostport string) (string, string) {
	hostport = strings.TrimSpace(hostport)
	if hostport == "" {
		return "", ""
	}

	if i := strings.IndexByte(hostport, ','); i >= 0 {
		hostport = strings.TrimSpace(hostport[:i])
	}

	host, port, err := net.SplitHostPort(hostport)
	if err != nil {
		return strings.ToLower(hostport), ""
	}
	return strings.ToLower(host), port
}

func requestHostPort(r *http.Request) (string, string) {
	hostport := r.Header.Get("X-Forwarded-Host")
	if hostport == "" {
		hostport = r.Host
	}
	host, port := parseHostPort(hostport)
	if host == "" {
		return "", ""
	}
	if port == "" {
		if isSecureRequest(r) {
			port = "443"
		} else {
			port = "80"
		}
	}
	return host, port
}

func originHostPort(origin string) (string, string, bool) {
	u, err := url.Parse(origin)
	if err != nil {
		return "", "", false
	}
	if !strings.EqualFold(u.Scheme, "http") && !strings.EqualFold(u.Scheme, "https") {
		return "", "", false
	}
	host := strings.ToLower(u.Hostname())
	if host == "" {
		return "", "", false
	}
	port := u.Port()
	if port == "" {
		if strings.EqualFold(u.Scheme, "https") {
			port = "443"
		} else {
			port = "80"
		}
	}
	return host, port, true
}

func allowWSOrigin(r *http.Request) bool {
	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if origin == "" {
		// Non-browser clients may not send Origin.
		return true
	}

	originHost, originPort, ok := originHostPort(origin)
	if !ok {
		return false
	}
	reqHost, reqPort := requestHostPort(r)
	if reqHost == "" || reqPort == "" {
		return false
	}
	return originHost == reqHost && originPort == reqPort
}

func (w *WebChannel) createGoogleOAuthState(sessionToken string) (string, error) {
	sessionToken = strings.TrimSpace(sessionToken)
	if sessionToken == "" {
		return "", fmt.Errorf("missing authenticated session")
	}

	stateBytes := make([]byte, 24)
	if _, err := rand.Read(stateBytes); err != nil {
		return "", fmt.Errorf("failed to generate oauth state: %w", err)
	}
	state := hex.EncodeToString(stateBytes)
	w.googleOAuthStates.Store(state, googleOAuthState{
		SessionToken: sessionToken,
		ExpiresAt:    time.Now().Add(googleOAuthStateTTL),
	})
	return state, nil
}

func (w *WebChannel) consumeGoogleOAuthState(state, sessionToken string) bool {
	state = strings.TrimSpace(state)
	sessionToken = strings.TrimSpace(sessionToken)
	if state == "" || sessionToken == "" {
		return false
	}

	val, ok := w.googleOAuthStates.LoadAndDelete(state)
	if !ok {
		return false
	}
	st, ok := val.(googleOAuthState)
	if !ok {
		return false
	}
	if time.Now().After(st.ExpiresAt) {
		return false
	}
	return st.SessionToken == sessionToken
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
	upgrader := websocket.Upgrader{
		CheckOrigin: allowWSOrigin,
	}
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

func parseIntQuery(r *http.Request, key string, def int) int {
	raw := strings.TrimSpace(r.URL.Query().Get(key))
	if raw == "" {
		return def
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return def
	}
	return n
}

func parseBeforeSeqQuery(r *http.Request) *int64 {
	raw := strings.TrimSpace(r.URL.Query().Get("before_seq"))
	if raw == "" {
		return nil
	}
	n, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || n <= 0 {
		return nil
	}
	return &n
}

func normalizeDeliveryForAdd(channel, chatID *string) (string, string, error) {
	channelValue, channelSet := normalizeOptionalString(channel)
	chatValue, chatSet := normalizeOptionalString(chatID)
	if channelSet != chatSet {
		return "", "", fmt.Errorf("delivery_channel and delivery_chat_id must be provided together")
	}
	if !channelSet {
		return "", "", nil
	}
	if (channelValue == "") != (chatValue == "") {
		return "", "", fmt.Errorf("delivery_channel and delivery_chat_id must both be empty or both non-empty")
	}
	return channelValue, chatValue, nil
}

func normalizeDeliveryForUpdate(channel, chatID *string) (*string, *string, error) {
	channelValue, channelSet := normalizeOptionalString(channel)
	chatValue, chatSet := normalizeOptionalString(chatID)
	if channelSet != chatSet {
		return nil, nil, fmt.Errorf("delivery_channel and delivery_chat_id must be provided together")
	}
	if !channelSet {
		return nil, nil, nil
	}
	if (channelValue == "") != (chatValue == "") {
		return nil, nil, fmt.Errorf("delivery_channel and delivery_chat_id must both be empty or both non-empty")
	}
	return &channelValue, &chatValue, nil
}

func normalizeOptionalString(v *string) (string, bool) {
	if v == nil {
		return "", false
	}
	return strings.TrimSpace(*v), true
}

type sessionsPagination struct {
	Limit   int  `json:"limit"`
	Offset  int  `json:"offset"`
	Total   int  `json:"total"`
	HasMore bool `json:"has_more"`
}

type sessionsListResponse struct {
	Items      []agent.SessionSummary `json:"items"`
	Pagination sessionsPagination     `json:"pagination"`
}

// handleSessions returns a list of session summaries.
func (w *WebChannel) handleSessions(rw http.ResponseWriter, r *http.Request) {
	limit := parseIntQuery(r, "limit", 50)
	offset := parseIntQuery(r, "offset", 0)

	page, err := w.sessions.ListSessionsPage(limit, offset)
	if err != nil {
		http.Error(rw, err.Error(), http.StatusInternalServerError)
		return
	}
	if page.Items == nil {
		page.Items = []agent.SessionSummary{}
	}

	rw.Header().Set("Content-Type", "application/json")
	json.NewEncoder(rw).Encode(sessionsListResponse{
		Items: page.Items,
		Pagination: sessionsPagination{
			Limit:   page.Limit,
			Offset:  page.Offset,
			Total:   page.Total,
			HasMore: page.HasMore,
		},
	})
}

// handleSessionDetail returns the full message history for a session.
func (w *WebChannel) handleSessionDetail(rw http.ResponseWriter, r *http.Request) {
	id := r.URL.Path[len("/api/sessions/"):]
	if id == "" {
		http.Error(rw, "session ID required", http.StatusBadRequest)
		return
	}

	limit := parseIntQuery(r, "limit", 120)
	beforeSeq := parseBeforeSeqQuery(r)

	page, err := w.sessions.LoadMessagesPage(id, limit, beforeSeq)
	if err != nil {
		http.Error(rw, err.Error(), http.StatusInternalServerError)
		return
	}

	type sessionDetailPagination struct {
		Limit         int    `json:"limit"`
		Total         int    `json:"total"`
		HasMore       bool   `json:"has_more"`
		NextBeforeSeq *int64 `json:"next_before_seq,omitempty"`
	}
	type sessionDetailResponse struct {
		ID         string                  `json:"id"`
		Messages   []providerPkg.Message   `json:"messages"`
		Created    time.Time               `json:"created"`
		Updated    time.Time               `json:"updated"`
		Pagination sessionDetailPagination `json:"pagination"`
	}

	rw.Header().Set("Content-Type", "application/json")
	json.NewEncoder(rw).Encode(sessionDetailResponse{
		ID:       page.ID,
		Messages: page.Messages,
		Created:  page.Created,
		Updated:  page.Updated,
		Pagination: sessionDetailPagination{
			Limit:         page.Limit,
			Total:         page.TotalMessages,
			HasMore:       page.HasMore,
			NextBeforeSeq: page.NextBeforeSeq,
		},
	})
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

	// Override allowed_users from SQLite store (source of truth) instead of static config.
	if w.store != nil {
		if users, err := w.store.AllowedUsers("discord"); err == nil {
			resp.Discord.AllowedUsers = users
		}
		if gp, err := w.store.Policy("discord", "group_policy"); err == nil && strings.TrimSpace(gp) != "" {
			resp.Discord.GroupPolicy = gp
		}
		if dp, err := w.store.Policy("discord", "dm_policy"); err == nil && strings.TrimSpace(dp) != "" {
			resp.Discord.DM.Policy = dp
		}
		if users, err := w.store.AllowedUsers("telegram"); err == nil {
			resp.Telegram.AllowedUsers = users
		}
		if gp, err := w.store.Policy("telegram", "group_policy"); err == nil && strings.TrimSpace(gp) != "" {
			resp.Telegram.GroupPolicy = gp
		}
		if dp, err := w.store.Policy("telegram", "dm_policy"); err == nil && strings.TrimSpace(dp) != "" {
			resp.Telegram.DM.Policy = dp
		}
		if users, err := w.store.AllowedUsers("whatsapp"); err == nil {
			resp.WhatsApp.AllowedUsers = users
		}
		if gp, err := w.store.Policy("whatsapp", "group_policy"); err == nil && strings.TrimSpace(gp) != "" {
			resp.WhatsApp.GroupPolicy = gp
		}
		if dp, err := w.store.Policy("whatsapp", "dm_policy"); err == nil && strings.TrimSpace(dp) != "" {
			resp.WhatsApp.DM.Policy = dp
		}
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

	// Route allowed_users changes to SQLite store; keep static config in YAML.
	if incoming.Discord != nil {
		if w.store != nil && incoming.Discord.AllowedUsers != nil {
			if err := w.store.SetAllowedUsers("discord", incoming.Discord.AllowedUsers); err != nil {
				http.Error(rw, err.Error(), http.StatusInternalServerError)
				return
			}
		}
		if w.store != nil && strings.TrimSpace(incoming.Discord.GroupPolicy) != "" {
			if err := w.store.SetPolicy("discord", "group_policy", incoming.Discord.GroupPolicy); err != nil {
				http.Error(rw, err.Error(), http.StatusInternalServerError)
				return
			}
		}
		if w.store != nil && strings.TrimSpace(incoming.Discord.DM.Policy) != "" {
			if err := w.store.SetPolicy("discord", "dm_policy", incoming.Discord.DM.Policy); err != nil {
				http.Error(rw, err.Error(), http.StatusInternalServerError)
				return
			}
		}
		// Strip allowed_users from YAML write — store is source of truth.
		yamlCopy := *incoming.Discord
		yamlCopy.AllowedUsers = nil
		yamlCopy.GroupPolicy = ""
		yamlCopy.DM.Policy = ""
		channels["discord"] = yamlCopy
		w.channelCfg.Discord = *incoming.Discord
		if w.discord != nil {
			w.discord.ApplyConfig(*incoming.Discord)
		}
	}
	if incoming.Telegram != nil {
		if w.store != nil && incoming.Telegram.AllowedUsers != nil {
			if err := w.store.SetAllowedUsers("telegram", incoming.Telegram.AllowedUsers); err != nil {
				http.Error(rw, err.Error(), http.StatusInternalServerError)
				return
			}
		}
		if w.store != nil && strings.TrimSpace(incoming.Telegram.GroupPolicy) != "" {
			if err := w.store.SetPolicy("telegram", "group_policy", incoming.Telegram.GroupPolicy); err != nil {
				http.Error(rw, err.Error(), http.StatusInternalServerError)
				return
			}
		}
		if w.store != nil && strings.TrimSpace(incoming.Telegram.DM.Policy) != "" {
			if err := w.store.SetPolicy("telegram", "dm_policy", incoming.Telegram.DM.Policy); err != nil {
				http.Error(rw, err.Error(), http.StatusInternalServerError)
				return
			}
		}
		yamlCopy := *incoming.Telegram
		yamlCopy.AllowedUsers = nil
		yamlCopy.GroupPolicy = ""
		yamlCopy.DM.Policy = ""
		channels["telegram"] = yamlCopy
		w.channelCfg.Telegram = *incoming.Telegram
		if w.telegram != nil {
			w.telegram.ApplyConfig(*incoming.Telegram)
		}
	}
	if incoming.WhatsApp != nil {
		if w.store != nil && incoming.WhatsApp.AllowedUsers != nil {
			if err := w.store.SetAllowedUsers("whatsapp", incoming.WhatsApp.AllowedUsers); err != nil {
				http.Error(rw, err.Error(), http.StatusInternalServerError)
				return
			}
		}
		if w.store != nil && strings.TrimSpace(incoming.WhatsApp.GroupPolicy) != "" {
			if err := w.store.SetPolicy("whatsapp", "group_policy", incoming.WhatsApp.GroupPolicy); err != nil {
				http.Error(rw, err.Error(), http.StatusInternalServerError)
				return
			}
		}
		if w.store != nil && strings.TrimSpace(incoming.WhatsApp.DM.Policy) != "" {
			if err := w.store.SetPolicy("whatsapp", "dm_policy", incoming.WhatsApp.DM.Policy); err != nil {
				http.Error(rw, err.Error(), http.StatusInternalServerError)
				return
			}
		}
		yamlCopy := *incoming.WhatsApp
		yamlCopy.AllowedUsers = nil
		yamlCopy.GroupPolicy = ""
		yamlCopy.DM.Policy = ""
		channels["whatsapp"] = yamlCopy
		w.channelCfg.WhatsApp = *incoming.WhatsApp
		if w.whatsapp != nil {
			w.whatsapp.ApplyConfig(*incoming.WhatsApp)
		}
	}
	fullCfg["channels"] = channels

	// Write back (without allowed_users — those live in SQLite now).
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

	state := channelDiscordSnapshot(w.channelCfg, w.discord, w.store)
	rw.Header().Set("Content-Type", "application/json")
	json.NewEncoder(rw).Encode(state)
}

func (w *WebChannel) handleDiscordApprovals(rw http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		state := channelDiscordSnapshot(w.channelCfg, w.discord, w.store)
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
		} else if w.store != nil {
			w.store.AddAllowedUser("discord", userID)
			w.store.DeletePending("discord", userID)
		}
	case "reject":
		if w.discord != nil {
			w.discord.RejectUser(userID)
		} else if w.store != nil {
			w.store.DeletePending("discord", userID)
		}
	default:
		http.Error(rw, "action must be approve or reject", http.StatusBadRequest)
		return
	}

	state := channelDiscordSnapshot(w.channelCfg, w.discord, w.store)
	rw.Header().Set("Content-Type", "application/json")
	json.NewEncoder(rw).Encode(state)
}

func channelDiscordSnapshot(cfg *config.ChannelsConfig, ch *DiscordChannel, store *state.Store) DiscordAdminState {
	if ch != nil {
		s := ch.Snapshot()
		if s.AllowedUsers == nil {
			s.AllowedUsers = []string{}
		}
		if s.Pending == nil {
			s.Pending = []DiscordPendingApproval{}
		}
		return s
	}

	st := DiscordAdminState{
		AllowedUsers: []string{},
		Pending:      []DiscordPendingApproval{},
	}
	if cfg != nil {
		st.GroupPolicy = cfg.Discord.GroupPolicy
		st.DMPolicy = cfg.Discord.DM.Policy
	}
	if store != nil {
		if gp, err := store.Policy("discord", "group_policy"); err == nil && strings.TrimSpace(gp) != "" {
			st.GroupPolicy = gp
		}
		if dp, err := store.Policy("discord", "dm_policy"); err == nil && strings.TrimSpace(dp) != "" {
			st.DMPolicy = dp
		}
		if users, err := store.AllowedUsers("discord"); err == nil && len(users) > 0 {
			st.AllowedUsers = users
		}
		if pending, err := store.PendingApprovals("discord"); err == nil && len(pending) > 0 {
			st.Pending = make([]DiscordPendingApproval, 0, len(pending))
			for _, p := range pending {
				st.Pending = append(st.Pending, DiscordPendingApproval{
					UserID:       p.UserID,
					Username:     p.Username,
					Preview:      p.Preview,
					MessageCount: p.MessageCount,
					FirstSeenAt:  p.FirstSeenAt,
					LastSeenAt:   p.LastSeenAt,
				})
			}
		}
	} else if cfg != nil && len(cfg.Discord.AllowedUsers) > 0 {
		st.AllowedUsers = append([]string{}, cfg.Discord.AllowedUsers...)
	}
	return st
}

// --- Telegram Admin API ---

func (w *WebChannel) handleTelegramAdmin(rw http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(rw, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	state := channelTelegramSnapshot(w.channelCfg, w.telegram, w.store)
	rw.Header().Set("Content-Type", "application/json")
	json.NewEncoder(rw).Encode(state)
}

func (w *WebChannel) handleTelegramApprovals(rw http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		state := channelTelegramSnapshot(w.channelCfg, w.telegram, w.store)
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
		} else if w.store != nil {
			w.store.AddAllowedUser("telegram", userID)
			w.store.DeletePending("telegram", userID)
		}
	case "reject":
		if w.telegram != nil {
			w.telegram.RejectUser(userID)
		} else if w.store != nil {
			w.store.DeletePending("telegram", userID)
		}
	default:
		http.Error(rw, "action must be approve or reject", http.StatusBadRequest)
		return
	}

	state := channelTelegramSnapshot(w.channelCfg, w.telegram, w.store)
	rw.Header().Set("Content-Type", "application/json")
	json.NewEncoder(rw).Encode(state)
}

func channelTelegramSnapshot(cfg *config.ChannelsConfig, ch *TelegramChannel, store *state.Store) TelegramAdminState {
	if ch != nil {
		s := ch.Snapshot()
		if s.AllowedUsers == nil {
			s.AllowedUsers = []string{}
		}
		if s.Pending == nil {
			s.Pending = []TelegramPendingApproval{}
		}
		return s
	}

	st := TelegramAdminState{
		AllowedUsers: []string{},
		Pending:      []TelegramPendingApproval{},
	}
	if cfg != nil {
		st.GroupPolicy = cfg.Telegram.GroupPolicy
		st.DMPolicy = cfg.Telegram.DM.Policy
	}
	if store != nil {
		if gp, err := store.Policy("telegram", "group_policy"); err == nil && strings.TrimSpace(gp) != "" {
			st.GroupPolicy = gp
		}
		if dp, err := store.Policy("telegram", "dm_policy"); err == nil && strings.TrimSpace(dp) != "" {
			st.DMPolicy = dp
		}
		if users, err := store.AllowedUsers("telegram"); err == nil && len(users) > 0 {
			st.AllowedUsers = users
		}
		if pending, err := store.PendingApprovals("telegram"); err == nil && len(pending) > 0 {
			st.Pending = make([]TelegramPendingApproval, 0, len(pending))
			for _, p := range pending {
				st.Pending = append(st.Pending, TelegramPendingApproval{
					UserID:       p.UserID,
					Username:     p.Username,
					Preview:      p.Preview,
					MessageCount: p.MessageCount,
					FirstSeenAt:  p.FirstSeenAt,
					LastSeenAt:   p.LastSeenAt,
				})
			}
		}
	} else if cfg != nil && len(cfg.Telegram.AllowedUsers) > 0 {
		st.AllowedUsers = append([]string{}, cfg.Telegram.AllowedUsers...)
	}
	return st
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
			Schedule        string  `json:"schedule"`
			Task            string  `json:"task"`
			DeliveryChannel *string `json:"delivery_channel"`
			DeliveryChatID  *string `json:"delivery_chat_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(rw, "invalid request body", http.StatusBadRequest)
			return
		}
		if req.Schedule == "" || req.Task == "" {
			http.Error(rw, "schedule and task are required", http.StatusBadRequest)
			return
		}
		deliveryChannel, deliveryChatID, err := normalizeDeliveryForAdd(req.DeliveryChannel, req.DeliveryChatID)
		if err != nil {
			http.Error(rw, err.Error(), http.StatusBadRequest)
			return
		}
		id, err := w.cronManager.AddJob(req.Schedule, req.Task, deliveryChannel, deliveryChatID)
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
				err := w.cronManager.UpdateJob(jobID, "", "", !j.Enabled, nil, nil)
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
			Schedule        string  `json:"schedule"`
			Task            string  `json:"task"`
			DeliveryChannel *string `json:"delivery_channel"`
			DeliveryChatID  *string `json:"delivery_chat_id"`
			Enabled         *bool   `json:"enabled"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(rw, "invalid request body", http.StatusBadRequest)
			return
		}
		enabled := false
		if req.Enabled == nil {
			found := false
			for _, j := range w.cronManager.ListJobs() {
				if j.ID == jobID {
					enabled = j.Enabled
					found = true
					break
				}
			}
			if !found {
				http.Error(rw, "job not found", http.StatusNotFound)
				return
			}
		} else {
			enabled = *req.Enabled
		}
		deliveryChannel, deliveryChatID, err := normalizeDeliveryForUpdate(req.DeliveryChannel, req.DeliveryChatID)
		if err != nil {
			http.Error(rw, err.Error(), http.StatusBadRequest)
			return
		}
		if err := w.cronManager.UpdateJob(jobID, req.Schedule, req.Task, enabled, deliveryChannel, deliveryChatID); err != nil {
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
	switch r.Method {
	case http.MethodGet:
		resp := w.heartbeatStatusSnapshot()
		rw.Header().Set("Content-Type", "application/json")
		json.NewEncoder(rw).Encode(resp)
		return

	case http.MethodPut:
		var req struct {
			DeliveryChannel *string `json:"delivery_channel"`
			DeliveryChatID  *string `json:"delivery_chat_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(rw, "invalid request body", http.StatusBadRequest)
			return
		}
		deliveryChannel, deliveryChatID, err := normalizeDeliveryForUpdate(req.DeliveryChannel, req.DeliveryChatID)
		if err != nil {
			http.Error(rw, err.Error(), http.StatusBadRequest)
			return
		}
		if deliveryChannel == nil || deliveryChatID == nil {
			http.Error(rw, "delivery_channel and delivery_chat_id are required", http.StatusBadRequest)
			return
		}

		if w.store != nil {
			if err := w.store.SetPolicy("heartbeat", "delivery_channel", *deliveryChannel); err != nil {
				http.Error(rw, "failed to persist heartbeat delivery channel", http.StatusInternalServerError)
				return
			}
			if err := w.store.SetPolicy("heartbeat", "delivery_chat_id", *deliveryChatID); err != nil {
				http.Error(rw, "failed to persist heartbeat delivery chat id", http.StatusInternalServerError)
				return
			}
		}
		if w.heartbeatRunner != nil {
			if err := w.heartbeatRunner.SetDeliveryTarget(*deliveryChannel, *deliveryChatID); err != nil {
				http.Error(rw, err.Error(), http.StatusBadRequest)
				return
			}
		}

		resp := w.heartbeatStatusSnapshot()
		resp.DeliveryChannel = *deliveryChannel
		resp.DeliveryChatID = *deliveryChatID
		rw.Header().Set("Content-Type", "application/json")
		json.NewEncoder(rw).Encode(resp)
		return

	default:
		http.Error(rw, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (w *WebChannel) heartbeatStatusSnapshot() agent.HeartbeatStatus {
	if w.heartbeatRunner != nil {
		return w.heartbeatRunner.Status()
	}

	resp := agent.HeartbeatStatus{
		Enabled: false,
	}
	if w.heartbeatCfg != nil {
		resp.Interval = w.heartbeatCfg.Interval.String()
	}
	if w.store != nil {
		channel, _ := w.store.Policy("heartbeat", "delivery_channel")
		chatID, _ := w.store.Policy("heartbeat", "delivery_chat_id")
		channel = strings.TrimSpace(channel)
		chatID = strings.TrimSpace(chatID)
		if channel != "" && chatID != "" {
			resp.DeliveryChannel = channel
			resp.DeliveryChatID = chatID
		}
	}
	return resp
}

// --- WhatsApp Admin API ---

func (w *WebChannel) handleWhatsAppStatus(rw http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(rw, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	state := channelWhatsAppSnapshot(w.channelCfg, w.whatsapp, w.store)
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

	// Persist mutable state to SQLite store.
	if w.store != nil {
		if req.AllowedUsers != nil {
			if err := w.store.SetAllowedUsers("whatsapp", req.AllowedUsers); err != nil {
				http.Error(rw, err.Error(), http.StatusInternalServerError)
				return
			}
		}
		if req.GroupPolicy != "" {
			if err := w.store.SetPolicy("whatsapp", "group_policy", req.GroupPolicy); err != nil {
				http.Error(rw, err.Error(), http.StatusInternalServerError)
				return
			}
		}
		if req.DMPolicy != "" {
			if err := w.store.SetPolicy("whatsapp", "dm_policy", req.DMPolicy); err != nil {
				http.Error(rw, err.Error(), http.StatusInternalServerError)
				return
			}
		}
	}

	// Update in-memory config for policies (static settings).
	if w.channelCfg != nil {
		if req.GroupPolicy != "" {
			w.channelCfg.WhatsApp.GroupPolicy = req.GroupPolicy
		}
		if req.DMPolicy != "" {
			w.channelCfg.WhatsApp.DM.Policy = req.DMPolicy
		}
	}

	// Hot-reload to channel.
	if w.whatsapp != nil && w.channelCfg != nil {
		w.whatsapp.ApplyConfig(w.channelCfg.WhatsApp)
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
	} else if w.store != nil {
		w.store.AddAllowedUser("whatsapp", userID)
		w.store.DeletePending("whatsapp", userID)
	}

	state := channelWhatsAppSnapshot(w.channelCfg, w.whatsapp, w.store)
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
	} else if w.store != nil {
		w.store.DeletePending("whatsapp", userID)
	}

	state := channelWhatsAppSnapshot(w.channelCfg, w.whatsapp, w.store)
	rw.Header().Set("Content-Type", "application/json")
	json.NewEncoder(rw).Encode(state)
}

func channelWhatsAppSnapshot(cfg *config.ChannelsConfig, ch *WhatsAppChannel, store *state.Store) WhatsAppAdminState {
	if ch != nil {
		return ch.Snapshot()
	}

	st := WhatsAppAdminState{
		AllowedUsers: []string{},
		Pending:      []WhatsAppPendingApproval{},
		Status:       "disconnected",
	}
	if cfg != nil {
		st.GroupPolicy = cfg.WhatsApp.GroupPolicy
		st.DMPolicy = cfg.WhatsApp.DM.Policy
	}
	if store != nil {
		if gp, err := store.Policy("whatsapp", "group_policy"); err == nil && strings.TrimSpace(gp) != "" {
			st.GroupPolicy = gp
		}
		if dp, err := store.Policy("whatsapp", "dm_policy"); err == nil && strings.TrimSpace(dp) != "" {
			st.DMPolicy = dp
		}
		if users, err := store.AllowedUsers("whatsapp"); err == nil && len(users) > 0 {
			st.AllowedUsers = users
		}
		if pending, err := store.PendingApprovals("whatsapp"); err == nil && len(pending) > 0 {
			st.Pending = make([]WhatsAppPendingApproval, 0, len(pending))
			for _, p := range pending {
				st.Pending = append(st.Pending, WhatsAppPendingApproval{
					UserID:       p.UserID,
					Username:     p.Username,
					Preview:      p.Preview,
					MessageCount: p.MessageCount,
					FirstSeenAt:  p.FirstSeenAt,
					LastSeenAt:   p.LastSeenAt,
				})
			}
		}
	} else if cfg != nil && len(cfg.WhatsApp.AllowedUsers) > 0 {
		st.AllowedUsers = append([]string{}, cfg.WhatsApp.AllowedUsers...)
	}
	return st
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

	cookie, err := r.Cookie(sessionCookieName)
	if err != nil || strings.TrimSpace(cookie.Value) == "" {
		http.Error(rw, "unauthorized", http.StatusUnauthorized)
		return
	}

	state, err := w.createGoogleOAuthState(cookie.Value)
	if err != nil {
		http.Error(rw, "failed to start OAuth flow", http.StatusInternalServerError)
		return
	}

	url := w.googleAuth.AuthURLWithState(state)
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

	if !w.isAuthenticated(r) {
		http.Error(rw, "unauthorized", http.StatusUnauthorized)
		return
	}
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil || strings.TrimSpace(cookie.Value) == "" {
		http.Error(rw, "unauthorized", http.StatusUnauthorized)
		return
	}

	oauthState := strings.TrimSpace(r.URL.Query().Get("state"))
	if oauthState == "" {
		http.Error(rw, "missing OAuth state", http.StatusBadRequest)
		return
	}
	if !w.consumeGoogleOAuthState(oauthState, cookie.Value) {
		http.Error(rw, "invalid OAuth state", http.StatusBadRequest)
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

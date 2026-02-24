package main

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/eazyclaw/eazyclaw/internal/agent"
	"github.com/eazyclaw/eazyclaw/internal/bus"
	channelPkg "github.com/eazyclaw/eazyclaw/internal/channel"
	"github.com/eazyclaw/eazyclaw/internal/config"
	providerPkg "github.com/eazyclaw/eazyclaw/internal/provider"
	"github.com/eazyclaw/eazyclaw/internal/router"
	"github.com/eazyclaw/eazyclaw/internal/skill"
	"github.com/eazyclaw/eazyclaw/internal/state"
	"github.com/eazyclaw/eazyclaw/internal/tool"
)

var dataDir string

func main() {
	rootCmd := &cobra.Command{
		Use:   "eazyclaw",
		Short: "EazyClaw - AI agent gateway for messaging platforms",
		Long: `EazyClaw is a self-hosted AI agent gateway that connects LLM providers
to messaging platforms like Telegram and Discord, with built-in tool
execution, memory, and skill support.`,
		RunE: runServe,
	}

	rootCmd.PersistentFlags().StringVar(&dataDir, "data-dir", "/data/eazyclaw", "Path to data directory")

	// serve command (also the default)
	serveCmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the EazyClaw gateway",
		Long:  "Start the EazyClaw gateway, connecting configured channels to LLM providers.",
		RunE:  runServe,
	}

	// health command
	healthCmd := &cobra.Command{
		Use:   "health",
		Short: "Run a health check",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("ok")
		},
	}

	// config command group
	configCmd := &cobra.Command{
		Use:   "config",
		Short: "Configuration management commands",
	}

	configCheckCmd := &cobra.Command{
		Use:   "check",
		Short: "Validate configuration and environment variables",
		RunE:  runConfigCheck,
	}
	configCmd.AddCommand(configCheckCmd)

	// auth command group
	authCmd := &cobra.Command{
		Use:   "auth",
		Short: "Authentication management commands",
	}

	authLoginCmd := &cobra.Command{
		Use:   "login [provider]",
		Short: "Authenticate with a provider via OAuth",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			provider := args[0]
			fmt.Printf("OAuth login for provider %q is not yet implemented.\n", provider)
			return nil
		},
	}
	authCmd.AddCommand(authLoginCmd)

	rootCmd.AddCommand(serveCmd, healthCmd, configCmd, authCmd)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func runServe(cmd *cobra.Command, args []string) error {
	slog.Info("starting eazyclaw", "data_dir", dataDir)

	// 1. Load config
	configPath := filepath.Join(dataDir, "config.yaml")
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// 2. Open state store (SQLite for mutable runtime data)
	stateStore, err := state.Open(cfg.DataDir)
	if err != nil {
		return fmt.Errorf("failed to open state store: %w", err)
	}
	defer stateStore.Close()
	if err := stateStore.SeedFromConfig(cfg.Channels); err != nil {
		slog.Warn("failed to seed state store from config", "error", err)
	}

	// 3. Create message bus
	msgBus := bus.New(100)

	// 4. Create provider registry and auto-register from env vars
	provReg := providerPkg.NewRegistry(cfg.Providers.DefaultModel)
	provReg.AutoRegister(cfg.Providers)

	memEnabled := cfg.Memory.Enabled == nil || *cfg.Memory.Enabled
	memCompactionEnabled := cfg.Memory.Compaction.Enabled == nil || *cfg.Memory.Compaction.Enabled
	memPreFlushEnabled := cfg.Memory.Compaction.PreFlushMemoryWrite == nil || *cfg.Memory.Compaction.PreFlushMemoryWrite
	memBackgroundEnabled := cfg.Memory.Background.Enabled == nil || *cfg.Memory.Background.Enabled

	memDir := filepath.Join(cfg.DataDir, "memory")
	memoryManager := agent.NewMemoryManager(cfg.DataDir, memDir, agent.MemoryOptions{
		Enabled:                 memEnabled,
		ContextMaxChars:         cfg.Memory.ContextMaxChars,
		DailyFilesInContext:     cfg.Memory.DailyFilesInContext,
		CompactionEnabled:       memCompactionEnabled,
		CompactionTriggerMsgs:   cfg.Memory.Compaction.TriggerMessages,
		CompactionKeepRecent:    cfg.Memory.Compaction.KeepRecentMessages,
		CompactionSummaryChars:  cfg.Memory.Compaction.SummaryMaxChars,
		ContextWindowTokens:     cfg.Memory.Compaction.ContextWindowTokens,
		ReserveTokensFloor:      cfg.Memory.Compaction.ReserveTokensFloor,
		SoftThresholdTokens:     cfg.Memory.Compaction.SoftThresholdTokens,
		PreCompactionFlush:      memPreFlushEnabled,
		BackgroundDigestEnabled: memBackgroundEnabled,
		BackgroundDigestEvery:   cfg.Memory.Background.Interval,
		BackgroundDigestMaxRuns: cfg.Memory.Background.MaxRuns,
		ModelContextWindows:     cfg.Memory.Compaction.ModelContextWindows,
	})
	if err := memoryManager.EnsureBootstrapFiles(time.Now()); err != nil {
		slog.Warn("failed to seed memory bootstrap files", "error", err)
	}

	// 4. Create tool registry and register core tools
	toolReg := tool.NewRegistry()

	// Register shell tool
	shellTool := tool.NewShellTool(cfg.Tools.Shell, cfg.WorkspaceDir)
	toolReg.Register(shellTool)

	// Register filesystem tools: read_file, write_file, edit_file, list_dir
	toolReg.Register(tool.NewReadFileTool(cfg.WorkspaceDir))
	toolReg.Register(tool.NewWriteFileTool(cfg.WorkspaceDir))
	toolReg.Register(tool.NewEditFileTool(cfg.WorkspaceDir))
	toolReg.Register(tool.NewListDirTool(cfg.WorkspaceDir))

	// Register web tools: web_fetch, web_search
	toolReg.Register(tool.NewWebFetchTool())
	toolReg.Register(tool.NewWebSearchTool())

	// Register memory tools
	toolReg.Register(tool.NewMemoryReadTool(cfg.DataDir))
	toolReg.Register(tool.NewMemoryWriteTool(cfg.DataDir))
	toolReg.Register(tool.NewMemorySearchTool(cfg.DataDir))

	slog.Info("tools registered", "count", len(toolReg.List()))

	// Register cron tool
	cronJobsPath := filepath.Join(cfg.DataDir, "cron", "jobs.json")
	cronRunner := agent.NewCronRunner(cronJobsPath, msgBus)
	toolReg.Register(tool.NewCronTool(cronRunner))

	// 5. Load skills
	skills, err := skill.LoadSkills(cfg.SkillsDir)
	if err != nil {
		slog.Warn("failed to load skills", "error", err)
	}
	slog.Info("skills loaded", "count", len(skills))

	// 6. Build context with enhanced system prompt
	ctxBuilder := agent.NewContextBuilder(memoryManager.LongTermPath())
	ctxBuilder.SetAgentsPath(memoryManager.AgentsPath())
	ctxBuilder.SetSoulPath(memoryManager.SoulPath())
	ctxBuilder.SetBootstrapPath(memoryManager.BootstrapPath())
	ctxBuilder.SetIdentityPath(memoryManager.IdentityPath())
	ctxBuilder.SetUserPath(memoryManager.UserPath())
	ctxBuilder.SetHeartbeatPath(memoryManager.HeartbeatPath())
	ctxBuilder.SetMemoryManager(memoryManager)
	ctxBuilder.SetToolDescriptions(toolReg.Descriptions())
	skillInstructions := make([]string, len(skills))
	for i, s := range skills {
		skillInstructions[i] = s.Instructions
	}
	ctxBuilder.SetSkills(skillInstructions)
	ctxBuilder.SetTools(toolReg.List())

	// 7. Create session store
	sessStore, err := agent.NewSessionStore(filepath.Join(cfg.DataDir, "sessions"))
	if err != nil {
		return fmt.Errorf("failed to open session store: %w", err)
	}
	defer sessStore.Close()

	// 8. Create router
	r := router.NewRouter(stateStore)

	// 9. Create agent loop
	agentLoop := agent.NewAgentLoop(msgBus, provReg, toolReg, sessStore, ctxBuilder, memoryManager, r)

	// 10. Start channels
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var channels []channelPkg.Channel
	var discordChannel *channelPkg.DiscordChannel
	var telegramChannel *channelPkg.TelegramChannel
	var whatsappChannel *channelPkg.WhatsAppChannel

	if token := os.Getenv("TELEGRAM_BOT_TOKEN"); token != "" {
		tg := channelPkg.NewTelegramChannel(token, cfg.Channels.Telegram, stateStore)
		channels = append(channels, tg)
		telegramChannel = tg
		slog.Info("telegram channel enabled")
	}

	if token := os.Getenv("DISCORD_BOT_TOKEN"); token != "" {
		dc := channelPkg.NewDiscordChannel(token, cfg.Channels.Discord, stateStore)
		channels = append(channels, dc)
		discordChannel = dc
		slog.Info("discord channel enabled")
	}

	if cfg.Channels.WhatsApp.Enabled || os.Getenv("WHATSAPP_ENABLED") == "true" {
		waDataDir := filepath.Join(cfg.DataDir, "whatsapp")
		wa := channelPkg.NewWhatsAppChannel(cfg.Channels.WhatsApp, waDataDir, stateStore)
		channels = append(channels, wa)
		whatsappChannel = wa
		slog.Info("whatsapp channel enabled")
	}

	// Register Google tools if credentials are available
	var googleAuth *tool.GoogleAuth
	googleClientID := os.Getenv("GOOGLE_CLIENT_ID")
	googleClientSecret := os.Getenv("GOOGLE_CLIENT_SECRET")
	if googleClientID != "" && googleClientSecret != "" {
		redirectURL := fmt.Sprintf("http://localhost:%d/api/google/auth/callback", cfg.Channels.Web.Port)
		googleAuth = tool.NewGoogleAuth(googleClientID, googleClientSecret, redirectURL, cfg.DataDir)
		toolReg.Register(tool.NewGmailTool(googleAuth))
		toolReg.Register(tool.NewCalendarTool(googleAuth))
		slog.Info("google tools registered (gmail, calendar)")
	}

	// Web dashboard channel (enabled by default or via config)
	if cfg.Channels.Web.Enabled || cfg.Channels.Web.Port > 0 {
		if cfg.Channels.Web.Password == "" {
			slog.Error("WEB_PASSWORD is required — set it as an environment variable to secure the dashboard")
			os.Exit(1)
		}
		web := channelPkg.NewWebChannel(cfg.Channels.Web.Port, cfg.Channels.Web.Password, sessStore, provReg)
		web.SetSkills(skills)
		web.SetChannelsConfig(&cfg.Channels)
		web.SetConfigPath(configPath)
		web.SetMemoryDir(memDir)
		web.SetStateStore(stateStore)
		web.SetDiscordChannel(discordChannel)
		web.SetTelegramChannel(telegramChannel)
		web.SetWhatsAppChannel(whatsappChannel)
		if googleAuth != nil {
			web.SetGoogleAuth(googleAuth)
		}
		web.SetCronManager(cronRunner)
		web.SetHeartbeatConfig(&cfg.Heartbeat)

		// Serve embedded frontend.
		uiDist, err := fs.Sub(frontendFS, "ui-dist")
		if err != nil {
			slog.Warn("frontend not available", "error", err)
		} else {
			web.SetFrontendFS(uiDist)
		}

		channels = append(channels, web)
		slog.Info("web channel enabled", "port", cfg.Channels.Web.Port)
	}

	// Set active channel names on the web channel now that all channels are registered.
	for _, ch := range channels {
		if wc, ok := ch.(*channelPkg.WebChannel); ok {
			names := make([]string, len(channels))
			for i, c := range channels {
				names[i] = c.Name()
			}
			wc.SetActiveChannels(names)
			break
		}
	}

	if len(channels) == 0 {
		slog.Warn("no channels configured - enable web or set TELEGRAM_BOT_TOKEN / DISCORD_BOT_TOKEN")
	}

	// Start outbound message dispatcher
	go func() {
		for msg := range msgBus.Outbound {
			for _, ch := range channels {
				if ch.Name() == msg.ChannelID {
					if err := ch.Send(ctx, msg); err != nil {
						slog.Error("failed to send message", "channel", msg.ChannelID, "error", err)
					}
				}
			}
		}
	}()

	// Start channels
	for _, ch := range channels {
		go func(c channelPkg.Channel) {
			if err := c.Start(ctx, msgBus); err != nil {
				slog.Error("channel error", "channel", c.Name(), "error", err)
			}
		}(ch)
	}

	// Start agent loop
	go agentLoop.Run(ctx)

	// Start heartbeat runner if enabled
	if cfg.Heartbeat.Enabled {
		hb := agent.NewHeartbeatRunner(cfg.Heartbeat.Interval, msgBus)
		// Wire into WebChannel for monitoring API
		for _, ch := range channels {
			if wc, ok := ch.(*channelPkg.WebChannel); ok {
				wc.SetHeartbeatRunner(hb)
			}
		}
		go hb.Start(ctx)
		slog.Info("heartbeat runner enabled", "interval", cfg.Heartbeat.Interval)
	}

	// Start cron runner
	go cronRunner.Start(ctx)
	slog.Info("cron runner started")

	slog.Info("eazyclaw is running", "channels", len(channels))

	// Wait for signal
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig

	slog.Info("shutting down")
	cancel()
	for _, ch := range channels {
		if err := ch.Stop(); err != nil {
			slog.Error("failed to stop channel", "channel", ch.Name(), "error", err)
		}
	}
	return nil
}

func runConfigCheck(cmd *cobra.Command, args []string) error {
	// Load config
	cfg, err := config.Load(filepath.Join(dataDir, "config.yaml"))
	if err != nil {
		fmt.Printf("Config: INVALID - %v\n", err)
		return err
	}
	fmt.Println("Config: OK")
	fmt.Printf("  Data dir:      %s\n", cfg.DataDir)
	fmt.Printf("  Workspace dir: %s\n", cfg.WorkspaceDir)
	fmt.Printf("  Skills dir:    %s\n", cfg.SkillsDir)
	fmt.Printf("  Default model: %s\n", cfg.Providers.DefaultModel)
	fmt.Println()

	// Check provider API keys
	fmt.Println("Providers:")
	providerEnvVars := []struct {
		name   string
		envVar string
		model  string
	}{
		{"Anthropic", "ANTHROPIC_API_KEY", cfg.Providers.Anthropic.Model},
		{"OpenAI", "OPENAI_API_KEY", cfg.Providers.OpenAI.Model},
		{"Gemini", "GEMINI_API_KEY", cfg.Providers.Gemini.Model},
		{"Kimi Coding", "KIMI_API_KEY", cfg.Providers.KimiCoding.Model},
		{"Moonshot", "MOONSHOT_API_KEY", cfg.Providers.Moonshot.Model},
		{"Zhipu", "ZHIPU_API_KEY", cfg.Providers.Zhipu.Model},
	}
	for _, p := range providerEnvVars {
		status := "not configured"
		if os.Getenv(p.envVar) != "" {
			status = fmt.Sprintf("configured (model: %s)", p.model)
		}
		fmt.Printf("  %-12s %s [%s]\n", p.name+":", status, p.envVar)
	}
	fmt.Println()

	// Check channel tokens
	fmt.Println("Channels:")
	channelEnvVars := []struct {
		name   string
		envVar string
	}{
		{"Telegram", "TELEGRAM_BOT_TOKEN"},
		{"Discord", "DISCORD_BOT_TOKEN"},
		{"WhatsApp", "WHATSAPP_ENABLED"},
	}
	for _, c := range channelEnvVars {
		status := "not configured"
		if os.Getenv(c.envVar) != "" {
			status = "configured"
		}
		fmt.Printf("  %-12s %s [%s]\n", c.name+":", status, c.envVar)
	}
	fmt.Println()

	// Check Google integration
	fmt.Println("Integrations:")
	googleStatus := "not configured"
	if os.Getenv("GOOGLE_CLIENT_ID") != "" && os.Getenv("GOOGLE_CLIENT_SECRET") != "" {
		googleStatus = "configured"
	}
	fmt.Printf("  %-12s %s [GOOGLE_CLIENT_ID, GOOGLE_CLIENT_SECRET]\n", "Google:", googleStatus)

	return nil
}

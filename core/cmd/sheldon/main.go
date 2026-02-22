package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/bowerhall/sheldon/internal/agent"
	"github.com/bowerhall/sheldon/internal/alerts"
	"github.com/bowerhall/sheldon/internal/bot"
	"github.com/bowerhall/sheldon/internal/budget"
	"github.com/bowerhall/sheldon/internal/coder"
	"github.com/bowerhall/sheldon/internal/config"
	"github.com/bowerhall/sheldon/internal/conversation"
	"github.com/bowerhall/sheldon/internal/cron"
	"github.com/bowerhall/sheldon/internal/deployer"
	"github.com/bowerhall/sheldon/internal/embedder"
	"github.com/bowerhall/sheldon/internal/llm"
	"github.com/bowerhall/sheldon/internal/logger"
	"github.com/bowerhall/sheldon/internal/storage"
	"github.com/bowerhall/sheldon/internal/tools"
	"github.com/bowerhall/sheldonmem"
	"github.com/joho/godotenv"
)

func init() {
	godotenv.Load()
}

func healthCheck(memory *sheldonmem.Store, essencePath string) error {
	soulPath := filepath.Join(essencePath, "SOUL.md")

	if _, err := os.Stat(soulPath); err != nil {
		return fmt.Errorf("SOUL.md not found at %s", soulPath)
	}

	logger.Debug("health check", "component", "soul", "status", "ok")

	domain, err := memory.GetDomain(1)
	if err != nil {
		return fmt.Errorf("memory check failed: %w", err)
	}

	logger.Debug("health check", "component", "memory", "status", "ok", "domain", domain.Name)

	sheldon, err := memory.FindEntityByName("Sheldon")
	if err != nil {
		return fmt.Errorf("sheldon entity not found: %w", err)
	}

	logger.Debug("health check", "component", "entity", "status", "ok", "id", sheldon.ID)

	return nil
}

func main() {
	cfg, err := config.Load()
	if err != nil {
		logger.Fatal("failed to load config", "error", err)
	}

	model, err := llm.New(llm.Config{
		Provider: cfg.LLM.Provider,
		APIKey:   cfg.LLM.APIKey,
		Model:    cfg.LLM.Model,
	})
	if err != nil {
		logger.Fatal("failed to create llm", "error", err)
	}

	extractor, err := llm.New(llm.Config{
		Provider: cfg.Extractor.Provider,
		APIKey:   cfg.Extractor.APIKey,
		Model:    cfg.Extractor.Model,
		BaseURL:  cfg.Extractor.BaseURL,
	})
	if err != nil {
		logger.Fatal("failed to create extractor", "error", err)
	}

	memory, err := sheldonmem.Open(cfg.MemoryPath)
	if err != nil {
		logger.Fatal("failed to open memory", "error", err)
	}

	defer memory.Close()

	emb, err := embedder.New(embedder.Config{
		Provider: cfg.Embedder.Provider,
		BaseURL:  cfg.Embedder.BaseURL,
		Model:    cfg.Embedder.Model,
	})
	if err != nil {
		logger.Fatal("failed to create embedder", "error", err)
	}

	if emb != nil {
		memory.SetEmbedder(emb)
		logger.Debug("embedder configured", "provider", cfg.Embedder.Provider)
	}

	if err := healthCheck(memory, cfg.EssencePath); err != nil {
		logger.Fatal("health check failed", "error", err)
	}

	agentLoop := agent.New(model, extractor, memory, cfg.EssencePath, cfg.Timezone)

	if cfg.Coder.Enabled {
		bridgeCfg := coder.BridgeConfig{
			SandboxDir:   cfg.Coder.SandboxDir,
			APIKey:       cfg.Coder.APIKey,
			FallbackKey:  cfg.Coder.FallbackKey,
			Model:        cfg.Coder.Model,
			SkillsDir:    cfg.Coder.SkillsDir,
			Isolated:     cfg.Coder.Isolated,
			Image:        cfg.Coder.Image,
			GitEnabled:   cfg.Coder.Git.Enabled,
			GitUserName:  cfg.Coder.Git.UserName,
			GitUserEmail: cfg.Coder.Git.UserEmail,
			GitOrgURL:    cfg.Coder.Git.OrgURL,
			GitToken:     cfg.Coder.Git.Token,
		}

		bridge, err := coder.NewBridgeWithConfig(bridgeCfg)
		if err != nil {
			logger.Fatal("failed to create coder bridge", "error", err)
		}

		tools.RegisterCoderTool(agentLoop.Registry(), bridge, memory)

		builder, err := deployer.NewBuilder(cfg.Coder.SandboxDir + "/builds")
		if err != nil {
			logger.Fatal("failed to create builder", "error", err)
		}

		// register deployer tools
		composeDeploy := deployer.NewComposeDeployer(cfg.Deployer.AppsFile, cfg.Deployer.Network)
		domain := os.Getenv("DOMAIN")
		if domain == "" {
			domain = "localhost"
		}
		tools.RegisterComposeDeployerTools(agentLoop.Registry(), builder, composeDeploy, domain)
		logger.Info("deployer enabled", "apps_file", cfg.Deployer.AppsFile)

		mode := "subprocess"
		if cfg.Coder.Isolated {
			mode = "isolated"
		}

		logger.Info("coder enabled", "mode", mode, "model", cfg.Coder.Model, "sandbox", cfg.Coder.SandboxDir)
	}

	// skills manager - directory alongside memory db
	skillsDir := filepath.Join(filepath.Dir(cfg.MemoryPath), "skills")
	skillsManager, err := tools.NewSkillsManager(skillsDir)
	if err != nil {
		logger.Fatal("failed to create skills manager", "error", err)
	}
	tools.RegisterSkillsTools(agentLoop.Registry(), skillsManager)
	agentLoop.SetSkillsDir(skillsDir)
	logger.Info("skills enabled", "dir", skillsDir)

	// browser tools for web browsing
	tools.RegisterBrowserTools(agentLoop.Registry(), tools.DefaultBrowserConfig())
	logger.Info("browser tools enabled")

	// github tools for PR management (if git token configured)
	if cfg.Coder.Git.Token != "" {
		tools.RegisterGitHubTools(agentLoop.Registry(), &cfg.Coder.Git)
		logger.Info("github tools enabled", "org", cfg.Coder.Git.OrgURL)
	}

	// cron store for scheduled reminders
	cronTz, _ := time.LoadLocation(cfg.Timezone)
	cronStore, err := cron.NewStore(memory.DB(), cronTz)
	if err != nil {
		logger.Fatal("failed to create cron store", "error", err)
	}
	tools.RegisterCronTools(agentLoop.Registry(), cronStore, cronTz)
	logger.Info("cron tools enabled", "timezone", cfg.Timezone)

	// conversation buffer for recent message continuity
	convoStore, err := conversation.NewStore(memory.DB())
	if err != nil {
		logger.Fatal("failed to create conversation store", "error", err)
	}
	agentLoop.SetConversationStore(convoStore)
	logger.Info("conversation buffer enabled", "max_messages", 12)

	// minio storage (optional)
	if cfg.Storage.Enabled {
		storageClient, err := storage.NewClient(storage.Config{
			Endpoint:  cfg.Storage.Endpoint,
			AccessKey: cfg.Storage.AccessKey,
			SecretKey: cfg.Storage.SecretKey,
			UseSSL:    cfg.Storage.UseSSL,
		})
		if err != nil {
			logger.Error("failed to create storage client", "error", err)
		} else {
			initCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			if err := storageClient.Init(initCtx); err != nil {
				logger.Error("failed to init storage buckets", "error", err)
			} else {
				tools.RegisterStorageTools(agentLoop.Registry(), storageClient)
				logger.Info("storage enabled", "endpoint", cfg.Storage.Endpoint)
			}
			cancel()
		}
	}

	// runtime config (for dynamic model switching)
	runtimeCfg, err := config.NewRuntimeConfig(filepath.Dir(cfg.MemoryPath))
	if err != nil {
		logger.Error("failed to create runtime config", "error", err)
	} else {
		tools.RegisterConfigTools(agentLoop.Registry(), runtimeCfg)
		logger.Info("runtime config enabled")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var bots []bot.Bot
	var enabledProviders []string

	if cfg.Bots.Telegram.Enabled {
		b, err := bot.NewTelegram(cfg.Bots.Telegram.Token, agentLoop)
		if err != nil {
			logger.Fatal("failed to create telegram bot", "error", err)
		}

		bots = append(bots, b)
		enabledProviders = append(enabledProviders, "telegram")

		go b.Start(ctx)
	}

	if cfg.Bots.Discord.Enabled {
		b, err := bot.NewDiscord(cfg.Bots.Discord.Token, agentLoop)
		if err != nil {
			logger.Fatal("failed to create discord bot", "error", err)
		}

		bots = append(bots, b)
		enabledProviders = append(enabledProviders, "discord")

		go b.Start(ctx)
	}

	if len(bots) == 0 {
		logger.Fatal("no bot providers enabled, set TELEGRAM_TOKEN or DISCORD_TOKEN")
	}

	notifyBot := bots[0]
	agentLoop.SetNotifyFunc(func(chatID int64, message string) {
		if err := notifyBot.Send(chatID, message); err != nil {
			logger.Error("notification failed", "error", err, "chatID", chatID)
		}
	})

	if cfg.Budget.Enabled {
		tz, _ := time.LoadLocation(cfg.Timezone)

		tracker := budget.NewTracker(
			budget.Config{
				DailyLimit: cfg.Budget.DailyLimit,
				WarnAt:     cfg.Budget.WarnAt,
				Timezone:   tz,
			},

			func(used, limit int) {
				msg := fmt.Sprintf("Budget warning: %d/%d tokens used (%.0f%%). Approaching daily limit.", used, limit, float64(used)/float64(limit)*100)

				if cfg.Heartbeat.ChatID != 0 {
					notifyBot.Send(cfg.Heartbeat.ChatID, msg)
				}

				logger.Warn("budget warning", "used", used, "limit", limit)
			},

			func(used, limit int) {
				msg := fmt.Sprintf("Budget exceeded: %d/%d tokens. Responses disabled until tomorrow.", used, limit)

				if cfg.Heartbeat.ChatID != 0 {
					notifyBot.Send(cfg.Heartbeat.ChatID, msg)
				}

				logger.Error("budget exceeded", "used", used, "limit", limit)
			},
		)

		agentLoop.SetBudget(tracker)
		logger.Info("budget tracking enabled", "limit", cfg.Budget.DailyLimit, "warnAt", cfg.Budget.WarnAt)
	}

	if cfg.Heartbeat.ChatID != 0 {
		alerter := alerts.New(
			func(message string) {
				notifyBot.Send(cfg.Heartbeat.ChatID, message)
			},
			time.Hour,
		)
		agentLoop.SetAlerter(alerter)
		logger.Info("error alerting enabled", "chatID", cfg.Heartbeat.ChatID)
	}

	go func() {
		for range time.Tick(24 * time.Hour) {
			deleted, err := memory.Decay(sheldonmem.DefaultDecayConfig)
			if err != nil {
				logger.Error("decay failed", "error", err)
			} else if deleted > 0 {
				logger.Info("decay completed", "deleted", deleted)
			}
		}
	}()

	// cron runner for scheduled triggers (reminders, check-ins, tasks)
	if len(bots) > 0 {
		tz, _ := time.LoadLocation(cfg.Timezone)
		provider := enabledProviders[0]

		cronRunner := agent.NewCronRunner(
			cronStore,
			memory,
			// TriggerFunc: injects into agent loop
			func(chatID int64, sessionID string, prompt string) (string, error) {
				return agentLoop.ProcessSystemTrigger(ctx, sessionID, prompt)
			},
			// NotifyFunc: sends response to chat
			func(chatID int64, msg string) {
				notifyBot.Send(chatID, msg)
			},
			tz,
		)
		go cronRunner.Run(ctx)
		logger.Info("cron runner started", "provider", provider)
	}

	embedderProvider := cfg.Embedder.Provider
	if embedderProvider == "" {
		embedderProvider = "none"
	}

	logger.Info("sheldon started",
		"bots", enabledProviders,
		"llm", cfg.LLM.Provider,
		"embedder", embedderProvider,
		"essence", cfg.EssencePath,
		"memory", cfg.MemoryPath,
	)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	logger.Info("shutting down")
	cancel()
}

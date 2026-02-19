package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"github.com/kadet/kora/internal/agent"
	"github.com/kadet/kora/internal/alerts"
	"github.com/kadet/kora/internal/bot"
	"github.com/kadet/kora/internal/budget"
	"github.com/kadet/kora/internal/coder"
	"github.com/kadet/kora/internal/config"
	"github.com/kadet/kora/internal/deployer"
	"github.com/kadet/kora/internal/embedder"
	"github.com/kadet/kora/internal/llm"
	"github.com/kadet/kora/internal/logger"
	"github.com/kadet/kora/internal/tools"
	"github.com/kadet/koramem"
)

func init() {
	godotenv.Load()
}

func healthCheck(memory *koramem.Store, essencePath string) error {
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

	kora, err := memory.FindEntityByName("Kora")
	if err != nil {
		return fmt.Errorf("kora entity not found: %w", err)
	}

	logger.Debug("health check", "component", "entity", "status", "ok", "id", kora.ID)

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
	})
	if err != nil {
		logger.Fatal("failed to create extractor", "error", err)
	}

	memory, err := koramem.Open(cfg.MemoryPath)
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
		bridge, err := coder.NewBridge(cfg.Coder.SandboxDir, cfg.Coder.APIKey, cfg.Coder.BaseURL)
		if err != nil {
			logger.Fatal("failed to create coder bridge", "error", err)
		}

		tools.RegisterCoderTool(agentLoop.Registry(), bridge, memory)

		builder, err := deployer.NewBuilder(cfg.Coder.SandboxDir + "/builds")
		if err != nil {
			logger.Fatal("failed to create builder", "error", err)
		}

		deploy := deployer.NewDeployer("kora-apps")
		tools.RegisterDeployerTools(agentLoop.Registry(), builder, deploy)

		provider := "anthropic"
		if cfg.Coder.BaseURL != "" {
			provider = cfg.Coder.BaseURL
		}

		logger.Info("coder enabled", "provider", provider, "sandbox", cfg.Coder.SandboxDir)
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
			deleted, err := memory.Decay(koramem.DefaultDecayConfig)
			if err != nil {
				logger.Error("decay failed", "error", err)
			} else if deleted > 0 {
				logger.Info("decay completed", "deleted", deleted)
			}
		}
	}()

	if cfg.Heartbeat.Enabled && cfg.Heartbeat.ChatID != 0 && len(bots) > 0 {
		heartbeatBot := bots[0]
		provider := enabledProviders[0]
		sessionID := fmt.Sprintf("%s:%d", provider, cfg.Heartbeat.ChatID)
		interval := time.Duration(cfg.Heartbeat.Interval) * time.Hour

		go func() {
			time.Sleep(10 * time.Second)
			sendHeartbeat := func() {
				message, err := agentLoop.Heartbeat(ctx, sessionID)
				if err != nil {
					logger.Error("heartbeat failed", "error", err)
					return
				}
				heartbeatBot.Send(cfg.Heartbeat.ChatID, message)
			}
			sendHeartbeat() // immediate first heartbeat
			for range time.Tick(interval) {
				sendHeartbeat()
			}
		}()

		logger.Info("heartbeat enabled", "interval", cfg.Heartbeat.Interval, "chatID", cfg.Heartbeat.ChatID, "provider", provider)
	}

	embedderProvider := cfg.Embedder.Provider
	if embedderProvider == "" {
		embedderProvider = "none"
	}

	logger.Info("kora started",
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

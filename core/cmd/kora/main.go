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
	"github.com/kadet/kora/internal/bot"
	"github.com/kadet/kora/internal/config"
	"github.com/kadet/kora/internal/embedder"
	"github.com/kadet/kora/internal/llm"
	"github.com/kadet/kora/internal/logger"
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

	agentLoop := agent.New(model, extractor, memory, cfg.EssencePath)

	botCfg := bot.Config{
		Provider: cfg.Bot.Provider,
		Token:    cfg.Bot.Token,
	}

	b, err := bot.New(botCfg, agentLoop)
	if err != nil {
		logger.Fatal("failed to create bot", "error", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go b.Start(ctx)

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

	embedderProvider := cfg.Embedder.Provider
	if embedderProvider == "" {
		embedderProvider = "none"
	}

	logger.Info("kora started",
		"bot", cfg.Bot.Provider,
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

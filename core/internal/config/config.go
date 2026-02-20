package config

import (
	"fmt"
	"os"
	"strconv"
)

func Load() (*Config, error) {
	essencePath := os.Getenv("KORA_ESSENCE")
	if essencePath == "" {
		essencePath = "essence"
	}

	memoryPath := os.Getenv("KORA_MEMORY")
	if memoryPath == "" {
		memoryPath = "kora.db"
	}

	timezone := os.Getenv("KORA_TIMEZONE")
	if timezone == "" {
		timezone = "UTC"
	}

	llmConfig, err := loadLLMConfig()
	if err != nil {
		return nil, err
	}

	extractorConfig, err := loadExtractorConfig()
	if err != nil {
		return nil, err
	}

	embedderConfig := loadEmbedderConfig()

	botConfig, err := loadBotConfig()
	if err != nil {
		return nil, err
	}

	heartbeatConfig := loadHeartbeatConfig()
	multiBot := loadMultiBotConfig()
	budgetConfig := loadBudgetConfig()
	coderConfig := loadCoderConfig()

	return &Config{
		EssencePath: essencePath,
		MemoryPath:  memoryPath,
		Timezone:    timezone,
		LLM:         llmConfig,
		Extractor:   extractorConfig,
		Embedder:    embedderConfig,
		Coder:       coderConfig,
		Bot:         botConfig,
		Bots:        multiBot,
		Heartbeat:   heartbeatConfig,
		Budget:      budgetConfig,
	}, nil
}

func loadCoderConfig() CoderConfig {
	apiKey := os.Getenv("CODER_API_KEY")
	if apiKey == "" {
		apiKey = os.Getenv("ANTHROPIC_API_KEY")
	}

	baseURL := os.Getenv("CODER_BASE_URL")

	sandboxDir := os.Getenv("CODER_SANDBOX")
	if sandboxDir == "" {
		sandboxDir = "/tmp/kora-sandbox"
	}

	// k8s Jobs mode settings
	useK8sJobs := os.Getenv("CODER_USE_K8S_JOBS") == "true"

	k8sNamespace := os.Getenv("CODER_K8S_NAMESPACE")
	if k8sNamespace == "" {
		k8sNamespace = "kora"
	}

	k8sImage := os.Getenv("CODER_K8S_IMAGE")
	if k8sImage == "" {
		k8sImage = "kora-claude-code:latest"
	}

	artifactsPVC := os.Getenv("CODER_ARTIFACTS_PVC")
	if artifactsPVC == "" {
		artifactsPVC = "kora-coder-artifacts"
	}

	skillsDir := os.Getenv("CODER_SKILLS_DIR")
	if skillsDir == "" {
		skillsDir = "/skills"
	}

	return CoderConfig{
		Enabled:      apiKey != "" || baseURL != "",
		APIKey:       apiKey,
		BaseURL:      baseURL,
		SandboxDir:   sandboxDir,
		SkillsDir:    skillsDir,
		UseK8sJobs:   useK8sJobs,
		K8sNamespace: k8sNamespace,
		K8sImage:     k8sImage,
		ArtifactsPVC: artifactsPVC,
	}
}

func loadBudgetConfig() BudgetConfig {
	enabled := os.Getenv("BUDGET_ENABLED") == "true"

	dailyLimit := 100000 // default 100k tokens
	if limit, err := strconv.Atoi(os.Getenv("BUDGET_DAILY_LIMIT")); err == nil && limit > 0 {
		dailyLimit = limit
	}

	warnAt := 0.8 // default 80%
	if warn, err := strconv.ParseFloat(os.Getenv("BUDGET_WARN_AT"), 64); err == nil && warn > 0 && warn < 1 {
		warnAt = warn
	}

	return BudgetConfig{
		Enabled:    enabled,
		DailyLimit: dailyLimit,
		WarnAt:     warnAt,
	}
}

func loadMultiBotConfig() MultiBot {
	telegramToken := os.Getenv("TELEGRAM_TOKEN")
	discordToken := os.Getenv("DISCORD_TOKEN")

	return MultiBot{
		Telegram: BotInstance{
			Enabled: telegramToken != "",
			Token:   telegramToken,
		},
		Discord: BotInstance{
			Enabled: discordToken != "",
			Token:   discordToken,
		},
	}
}

func loadHeartbeatConfig() HeartbeatConfig {
	enabled := os.Getenv("HEARTBEAT_ENABLED") == "true"

	interval := 3 // default 3 hours
	if i, err := strconv.Atoi(os.Getenv("HEARTBEAT_INTERVAL")); err == nil && i > 0 {
		interval = i
	}

	var chatID int64
	if id, err := strconv.ParseInt(os.Getenv("HEARTBEAT_CHAT_ID"), 10, 64); err == nil {
		chatID = id
	}

	return HeartbeatConfig{
		Enabled:  enabled,
		Interval: interval,
		ChatID:   chatID,
	}
}

func loadEmbedderConfig() EmbedderConfig {
	return EmbedderConfig{
		Provider: os.Getenv("EMBEDDER_PROVIDER"),
		BaseURL:  os.Getenv("EMBEDDER_URL"),
		Model:    os.Getenv("EMBEDDER_MODEL"),
	}
}

func loadBotConfig() (BotConfig, error) {
	provider := os.Getenv("BOT_PROVIDER")
	if provider == "" {
		provider = "telegram"
	}

	var token string
	switch provider {
	case "telegram":
		token = os.Getenv("TELEGRAM_TOKEN")
		if token == "" {
			return BotConfig{}, fmt.Errorf("TELEGRAM_TOKEN not set")
		}
	case "discord":
		token = os.Getenv("DISCORD_TOKEN")
		if token == "" {
			return BotConfig{}, fmt.Errorf("DISCORD_TOKEN not set")
		}
	default:
		return BotConfig{}, fmt.Errorf("unknown BOT_PROVIDER: %s", provider)
	}

	return BotConfig{
		Provider: provider,
		Token:    token,
	}, nil
}

func loadLLMConfig() (LLMConfig, error) {
	provider := os.Getenv("LLM_PROVIDER")
	if provider == "" {
		provider = "kimi"
	}

	apiKey, err := getAPIKey(provider, "LLM")
	if err != nil {
		return LLMConfig{}, err
	}

	return LLMConfig{
		Provider: provider,
		APIKey:   apiKey,
		Model:    os.Getenv("LLM_MODEL"),
	}, nil
}

func loadExtractorConfig() (LLMConfig, error) {
	provider := os.Getenv("EXTRACTOR_PROVIDER")
	if provider == "" {
		provider = "kimi"
	}

	apiKey, err := getAPIKey(provider, "EXTRACTOR")
	if err != nil {
		return LLMConfig{}, err
	}

	return LLMConfig{
		Provider: provider,
		APIKey:   apiKey,
		Model:    os.Getenv("EXTRACTOR_MODEL"),
	}, nil
}

func getAPIKey(provider, prefix string) (string, error) {
	envKey := os.Getenv(prefix + "_API_KEY")
	if envKey != "" {
		return envKey, nil
	}

	switch provider {
	case "claude":
		key := os.Getenv("ANTHROPIC_API_KEY")
		if key == "" {
			return "", fmt.Errorf("ANTHROPIC_API_KEY not set")
		}
		return key, nil
	case "openai":
		key := os.Getenv("OPENAI_API_KEY")
		if key == "" {
			return "", fmt.Errorf("OPENAI_API_KEY not set")
		}
		return key, nil
	case "kimi":
		key := os.Getenv("KIMI_API_KEY")
		if key == "" {
			return "", fmt.Errorf("KIMI_API_KEY not set")
		}
		return key, nil
	default:
		return "", fmt.Errorf("unknown provider: %s", provider)
	}
}

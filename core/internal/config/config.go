package config

import (
	"fmt"
	"os"
	"strconv"
)

func Load() (*Config, error) {
	essencePath := os.Getenv("SHELDON_ESSENCE")
	if essencePath == "" {
		essencePath = "essence"
	}

	memoryPath := os.Getenv("SHELDON_MEMORY")
	if memoryPath == "" {
		memoryPath = "sheldon.db"
	}

	timezone := os.Getenv("TZ")
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
	browserConfig := loadBrowserConfig()
	storageConfig := loadStorageConfig()
	deployerConfig := loadDeployerConfig()

	return &Config{
		EssencePath: essencePath,
		MemoryPath:  memoryPath,
		Timezone:    timezone,
		LLM:         llmConfig,
		Extractor:   extractorConfig,
		Embedder:    embedderConfig,
		Coder:       coderConfig,
		Browser:     browserConfig,
		Deployer:    deployerConfig,
		Storage:     storageConfig,
		Bot:         botConfig,
		Bots:        multiBot,
		Heartbeat:   heartbeatConfig,
		Budget:      budgetConfig,
	}, nil
}

func loadDeployerConfig() DeployerConfig {
	appsFile := os.Getenv("DEPLOYER_APPS_FILE")
	if appsFile == "" {
		appsFile = "/opt/sheldon/apps.yml"
	}

	network := os.Getenv("DEPLOYER_NETWORK")
	if network == "" {
		network = "sheldon-net"
	}

	return DeployerConfig{
		AppsFile: appsFile,
		Network:  network,
	}
}

func loadStorageConfig() StorageConfig {
	enabled := os.Getenv("STORAGE_ENABLED") == "true"
	if !enabled {
		return StorageConfig{Enabled: false}
	}

	endpoint := os.Getenv("STORAGE_ENDPOINT")
	if endpoint == "" {
		endpoint = "minio:9000"
	}

	return StorageConfig{
		Enabled:   true,
		Endpoint:  endpoint,
		AccessKey: os.Getenv("STORAGE_ACCESS_KEY"),
		SecretKey: os.Getenv("STORAGE_SHELDON_PASSWORD"),
		UseSSL:    os.Getenv("STORAGE_USE_SSL") == "true",
	}
}

func loadCoderConfig() CoderConfig {
	// Provider for coder (inferred from model or explicit)
	provider := os.Getenv("CODER_PROVIDER")
	model := os.Getenv("CODER_MODEL")
	if model == "" {
		model = "kimi-k2.5:cloud"
	}
	if provider == "" {
		provider = InferProviderFromModel(model)
		if provider == "" {
			provider = "kimi" // default provider
		}
	}

	sandboxDir := os.Getenv("CODER_SANDBOX")
	if sandboxDir == "" {
		sandboxDir = "/tmp/sheldon-sandbox"
	}

	// isolated mode uses ephemeral Docker containers (default: true for security)
	isolated := os.Getenv("CODER_ISOLATED") != "false"

	image := os.Getenv("CODER_IMAGE")
	if image == "" {
		image = "sheldon-coder-sandbox:latest"
	}

	skillsDir := os.Getenv("CODER_SKILLS_DIR")
	if skillsDir == "" {
		skillsDir = "/skills"
	}

	// git integration for pushing code to repos
	gitConfig := GitConfig{
		UserName:  os.Getenv("GIT_USER_NAME"),
		UserEmail: os.Getenv("GIT_USER_EMAIL"),
		Token:     os.Getenv("GIT_TOKEN"),
		OrgURL:    os.Getenv("GIT_ORG_URL"),
	}
	gitConfig.Enabled = gitConfig.Token != "" && gitConfig.OrgURL != ""

	// enabled if we have an API key for the provider (or it's ollama)
	envKey := EnvKeyForProvider(provider)
	enabled := provider == "ollama" || os.Getenv(envKey) != ""

	return CoderConfig{
		Enabled:    enabled,
		Provider:   provider,
		Model:      model,
		SandboxDir: sandboxDir,
		SkillsDir:  skillsDir,
		Isolated:   isolated,
		Image:      image,
		Git:        gitConfig,
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

func loadBrowserConfig() BrowserConfig {
	// sandbox enabled by default, set BROWSER_SANDBOX_ENABLED=false to disable
	sandboxEnabled := os.Getenv("BROWSER_SANDBOX_ENABLED") != "false"

	image := os.Getenv("BROWSER_SANDBOX_IMAGE")
	if image == "" {
		image = "sheldon-browser-sandbox:latest"
	}

	return BrowserConfig{
		SandboxEnabled: sandboxEnabled,
		Image:          image,
	}
}

func loadHeartbeatConfig() HeartbeatConfig {
	var chatID int64
	if id, err := strconv.ParseInt(os.Getenv("HEARTBEAT_CHAT_ID"), 10, 64); err == nil {
		chatID = id
	}

	return HeartbeatConfig{
		ChatID: chatID,
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

	// Base URL for Ollama (defaults handled in llm package)
	baseURL := os.Getenv("EXTRACTOR_BASE_URL")

	return LLMConfig{
		Provider: provider,
		APIKey:   apiKey,
		Model:    os.Getenv("EXTRACTOR_MODEL"),
		BaseURL:  baseURL,
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
	case "ollama":
		// Ollama doesn't need an API key
		return "ollama", nil
	default:
		return "", fmt.Errorf("unknown provider: %s", provider)
	}
}

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
		appsFile = "/data/apps.yml"
	}

	hostAppsFile := os.Getenv("DEPLOYER_HOST_APPS_FILE")

	// path prefix translation for container → host paths
	pathPrefix := os.Getenv("DEPLOYER_PATH_PREFIX")
	if pathPrefix == "" {
		pathPrefix = "/data"
	}
	hostPrefix := os.Getenv("DEPLOYER_HOST_PREFIX")

	network := os.Getenv("DEPLOYER_NETWORK")
	if network == "" {
		network = "sheldon-net"
	}

	return DeployerConfig{
		AppsFile:     appsFile,
		HostAppsFile: hostAppsFile,
		PathPrefix:   pathPrefix,
		HostPrefix:   hostPrefix,
		Network:      network,
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
	// Provider for coder - Claude Code CLI supports multiple backends
	provider := os.Getenv("CODER_PROVIDER")
	model := os.Getenv("CODER_MODEL")
	if provider == "" {
		provider = DetectProvider()
	}
	if model == "" {
		model = DefaultCoderModel(provider)
	}

	sandboxDir := os.Getenv("CODER_SANDBOX")
	if sandboxDir == "" {
		sandboxDir = "/tmp/sheldon-sandbox"
	}

	// host path for Docker volume mounts (when Sheldon runs in a container)
	hostSandboxDir := os.Getenv("CODER_HOST_SANDBOX")

	// isolated mode uses ephemeral Docker containers (default: true for security)
	isolated := os.Getenv("CODER_ISOLATED") != "false"

	image := os.Getenv("CODER_IMAGE")
	if image == "" {
		image = "ghcr.io/bowerhall/sheldon-coder-sandbox:latest"
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
		Enabled:        enabled,
		Provider:       provider,
		Model:          model,
		SandboxDir:     sandboxDir,
		HostSandboxDir: hostSandboxDir,
		SkillsDir:      skillsDir,
		Isolated:       isolated,
		Image:          image,
		Git:            gitConfig,
	}
}

func loadBudgetConfig() BudgetConfig {
	// enabled by default, set BUDGET_ENABLED=false to disable
	enabled := os.Getenv("BUDGET_ENABLED") != "false"

	dailyLimit := 2000000 // default 2M tokens
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

	var ownerChatID int64
	if id, err := strconv.ParseInt(os.Getenv("OWNER_CHAT_ID"), 10, 64); err == nil {
		ownerChatID = id
	}

	return MultiBot{
		Telegram: BotInstance{
			Enabled:     telegramToken != "",
			Token:       telegramToken,
			OwnerChatID: ownerChatID,
		},
		Discord: BotInstance{
			Enabled:        discordToken != "",
			Token:          discordToken,
			GuildID:        os.Getenv("DISCORD_GUILD_ID"),
			OwnerID:        os.Getenv("DISCORD_OWNER_ID"),
			TrustedChannel: os.Getenv("DISCORD_TRUSTED_CHANNEL"),
		},
	}
}

func loadBrowserConfig() BrowserConfig {
	// sandbox enabled by default, set BROWSER_SANDBOX_ENABLED=false to disable
	sandboxEnabled := os.Getenv("BROWSER_SANDBOX_ENABLED") != "false"

	image := os.Getenv("BROWSER_SANDBOX_IMAGE")
	if image == "" {
		image = "ghcr.io/bowerhall/sheldon-browser-sandbox:latest"
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
		provider = DetectProvider()
	}

	apiKey, err := getAPIKey(provider, "LLM")
	if err != nil {
		return LLMConfig{}, err
	}

	model := os.Getenv("LLM_MODEL")
	if model == "" {
		model = defaultLLMModel(provider)
	}

	return LLMConfig{
		Provider: provider,
		APIKey:   apiKey,
		Model:    model,
	}, nil
}

func defaultLLMModel(provider string) string {
	switch provider {
	case "kimi":
		return "kimi-k2-0711-preview"
	case "claude":
		return "claude-sonnet-4-20250514"
	case "openai":
		return "gpt-4o"
	default:
		return "qwen2.5:3b"
	}
}

func loadExtractorConfig() (LLMConfig, error) {
	provider := os.Getenv("EXTRACTOR_PROVIDER")
	if provider == "" {
		provider = DetectProvider()
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

// DetectProvider returns the first available LLM provider based on API keys
func DetectProvider() string {
	if os.Getenv("KIMI_API_KEY") != "" {
		return "kimi"
	}
	if os.Getenv("ANTHROPIC_API_KEY") != "" {
		return "claude"
	}
	if os.Getenv("OPENAI_API_KEY") != "" {
		return "openai"
	}
	return "ollama"
}

// DefaultCoderModel returns the default model for code generation based on provider
func DefaultCoderModel(provider string) string {
	switch provider {
	case "kimi":
		return "kimi-k2.5"
	case "claude":
		return "claude-sonnet-4-20250514"
	case "openai":
		return "gpt-4o"
	default:
		return "qwen2.5-coder:7b"
	}
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

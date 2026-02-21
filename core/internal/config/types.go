package config

type Config struct {
	EssencePath string
	MemoryPath  string
	Timezone    string
	LLM         LLMConfig
	Extractor   LLMConfig
	Embedder    EmbedderConfig
	Coder       CoderConfig
	Deployer    DeployerConfig
	Storage     StorageConfig
	Bot         BotConfig
	Bots        MultiBot
	Heartbeat   HeartbeatConfig
	Budget      BudgetConfig
}

type DeployerConfig struct {
	AppsFile string // path to apps.yml
	Network  string // docker network name
}

type StorageConfig struct {
	Enabled   bool
	Endpoint  string
	AccessKey string
	SecretKey string
	UseSSL    bool
}

type CoderConfig struct {
	Enabled     bool
	APIKey      string // NVIDIA NIM API key (primary)
	FallbackKey string // Moonshot Kimi API key (fallback)
	Model       string // model to use (default: kimi-k2.5)
	SandboxDir  string
	SkillsDir   string // directory with skill patterns
	Isolated    bool   // use ephemeral Docker containers for isolation
	Image       string // coder container image (default: sheldon-coder-sandbox:latest)
	Git         GitConfig
}

type GitConfig struct {
	Enabled   bool
	UserName  string // git commit author name
	UserEmail string // git commit author email
	Token     string // GitHub PAT for pushing
	OrgURL    string // base URL for org repos (e.g., https://github.com/myorg)
}

type LLMConfig struct {
	Provider string
	APIKey   string
	Model    string
	BaseURL  string
}

type EmbedderConfig struct {
	Provider string
	BaseURL  string
	Model    string
}

type BotConfig struct {
	Provider string
	Token    string
}

type MultiBot struct {
	Telegram BotInstance
	Discord  BotInstance
}

type BotInstance struct {
	Enabled bool
	Token   string
}

type HeartbeatConfig struct {
	Enabled  bool
	Interval int   // hours
	ChatID   int64 // telegram chat ID to send proactive messages
}

type BudgetConfig struct {
	Enabled    bool
	DailyLimit int     // max tokens per day (0 = unlimited)
	WarnAt     float64 // warn at this percentage (0.8 = 80%)
}

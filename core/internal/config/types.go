package config

type Config struct {
	EssencePath string
	MemoryPath  string
	Timezone    string
	LLM         LLMConfig
	Extractor   LLMConfig
	Embedder    EmbedderConfig
	Coder       CoderConfig
	Bot         BotConfig
	Bots        MultiBot
	Heartbeat   HeartbeatConfig
	Budget      BudgetConfig
}

type CoderConfig struct {
	Enabled    bool
	APIKey     string
	BaseURL    string
	SandboxDir string
}

type LLMConfig struct {
	Provider string
	APIKey   string
	Model    string
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

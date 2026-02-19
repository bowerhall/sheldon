package config

type Config struct {
	EssencePath string
	MemoryPath  string
	LLM         LLMConfig
	Extractor   LLMConfig
	Embedder    EmbedderConfig
	Bot         BotConfig   // legacy single-provider
	Bots        MultiBot    // multi-provider
	Heartbeat   HeartbeatConfig
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
	Interval int    // hours
	ChatID   int64  // telegram chat ID to send proactive messages
}

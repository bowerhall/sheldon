package config

type Config struct {
	EssencePath string
	MemoryPath  string
	LLM         LLMConfig
	Extractor   LLMConfig
	Bot         BotConfig
}

type LLMConfig struct {
	Provider string
	APIKey   string
	Model    string
}

type BotConfig struct {
	Provider string
	Token    string
}

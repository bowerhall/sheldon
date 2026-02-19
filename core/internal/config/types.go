package config

type Config struct {
	EssencePath string
	MemoryPath  string
	LLM         LLMConfig
	Extractor   LLMConfig
	Embedder    EmbedderConfig
	Bot         BotConfig
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

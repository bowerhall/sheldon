package agent

import (
	"github.com/kadet/kora/internal/llm"
	"github.com/kadet/kora/internal/session"
	"github.com/kadet/koramem"
)

type Agent struct {
	llm          llm.LLM
	extractor    llm.LLM
	memory       *koramem.Store
	sessions     *session.Store
	systemPrompt string
}

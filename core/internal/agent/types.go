package agent

import (
	"github.com/kadet/kora/internal/llm"
	"github.com/kadet/kora/internal/session"
	"github.com/kadet/kora/internal/tools"
	"github.com/kadet/koramem"
)

type Agent struct {
	llm          llm.LLM
	extractor    llm.LLM
	memory       *koramem.Store
	sessions     *session.Store
	tools        *tools.Registry
	systemPrompt string
}

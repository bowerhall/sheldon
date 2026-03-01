package agent

import (
	"context"
	"fmt"
	"strings"

	"github.com/bowerhall/sheldon/internal/llm"
	"github.com/bowerhall/sheldon/internal/logger"
	"github.com/bowerhall/sheldonmem"
)

// entityResolver implements sheldonmem.EntityResolver
type entityResolver struct {
	agent *Agent
}

func (r *entityResolver) GetOrCreateUserEntity(sessionID string) int64 {
	return r.agent.getOrCreateUserEntity(sessionID)
}

func (r *entityResolver) GetSheldonEntityID() int64 {
	return r.agent.getSheldonEntityID()
}

func (r *entityResolver) GetOrCreateNamedEntity(name, entityType string) int64 {
	return r.agent.getOrCreateNamedEntity(name, entityType)
}

func (r *entityResolver) ResolveEntityID(name, sessionID string, userID, sheldonID int64) int64 {
	return r.agent.resolveEntityID(name, sessionID, userID, sheldonID)
}

// llmAdapter adapts our LLM interface to sheldonmem.LLM
type llmAdapter struct {
	llm llm.LLM
}

func (a *llmAdapter) Chat(ctx context.Context, systemPrompt string, messages []sheldonmem.LLMMessage) (string, error) {
	llmMsgs := make([]llm.Message, len(messages))
	for i, m := range messages {
		llmMsgs[i] = llm.Message{Role: m.Role, Content: m.Content}
	}
	return a.llm.Chat(ctx, systemPrompt, llmMsgs)
}

func (a *Agent) getSheldonEntityID() int64 {
	entity, err := a.memory.FindEntityByName("Sheldon")
	if err != nil {
		logger.Error("failed to find Sheldon entity", "error", err)
		return 0
	}
	return entity.ID
}

func (a *Agent) getOrCreateUserEntity(sessionID string) int64 {
	parts := strings.SplitN(sessionID, ":", 2)
	entityName := sessionID
	if len(parts) == 2 {
		entityName = fmt.Sprintf("user_%s_%s", parts[0], parts[1])
	}

	entity, err := a.memory.FindEntityByName(entityName)
	if err == nil {
		return entity.ID
	}

	entity, err = a.memory.CreateEntity(entityName, "user", 1, "")
	if err != nil {
		logger.Error("failed to create user entity", "error", err)
		return 0
	}

	logger.Info("user entity created", "name", entityName, "id", entity.ID)
	return entity.ID
}

func (a *Agent) resolveEntityID(name, sessionID string, userID, sheldonID int64) int64 {
	lower := strings.ToLower(name)
	if lower == "user" || lower == "me" {
		return userID
	}
	if lower == "sheldon" || lower == "assistant" {
		return sheldonID
	}
	return a.getOrCreateNamedEntity(name, "person")
}

func (a *Agent) getOrCreateNamedEntity(name, entityType string) int64 {
	entity, err := a.memory.FindEntityByName(name)
	if err == nil {
		return entity.ID
	}

	domainID := 6 // relationships domain
	if entityType == "place" {
		domainID = 9 // place domain
	} else if entityType == "organization" {
		domainID = 7 // career domain
	}

	entity, err = a.memory.CreateEntity(name, entityType, domainID, "")
	if err != nil {
		logger.Error("failed to create entity", "error", err, "name", name)
		return 0
	}

	logger.Info("entity created", "name", name, "type", entityType, "id", entity.ID)
	return entity.ID
}

// ProcessEndOfDay runs extraction and summarization for yesterday's conversations
// This is triggered by a system cron at ~3am
func (a *Agent) ProcessEndOfDay(ctx context.Context) error {
	resolver := &entityResolver{agent: a}
	adapter := &llmAdapter{llm: a.getLLM()}

	if err := a.memory.ProcessEndOfDay(ctx, adapter, resolver); err != nil {
		return fmt.Errorf("sheldonmem processing failed: %w", err)
	}

	return nil
}

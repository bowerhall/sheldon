package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/bowerhall/sheldon/internal/llm"
	"github.com/bowerhall/sheldon/internal/logger"
)

const extractPrompt = `You are a fact extractor. Analyze the conversation and extract any facts worth remembering about the user or topics discussed.

Return a JSON array of facts. Each fact should have:
- "field": short key (e.g., "name", "city", "preference", "interest")
- "value": the actual information
- "domain": one of: identity, health, mind, beliefs, knowledge, relationships, career, finances, place, goals, preferences, routines, events, patterns
- "confidence": 0.0-1.0 based on how certain the fact is

Only extract facts that are explicitly stated or strongly implied. Do not invent facts.
If no facts are worth remembering, return an empty array: []

Example output:
[
  {"field": "name", "value": "John", "domain": "identity", "confidence": 0.95},
  {"field": "favorite_color", "value": "blue", "domain": "preferences", "confidence": 0.8}
]

Conversation:
%s

Extract facts (JSON only, no explanation):`

type extractedFact struct {
	Field      string  `json:"field"`
	Value      string  `json:"value"`
	Domain     string  `json:"domain"`
	Confidence float64 `json:"confidence"`
}

var domainSlugToID = map[string]int{
	"identity":      1,
	"health":        2,
	"mind":          3,
	"beliefs":       4,
	"knowledge":     5,
	"relationships": 6,
	"career":        7,
	"finances":      8,
	"place":         9,
	"goals":         10,
	"preferences":   11,
	"routines":      12,
	"events":        13,
	"patterns":      14,
}

func (a *Agent) remember(ctx context.Context, sessionID string, messages []llm.Message) {
	if len(messages) < 2 {
		return
	}

	conversation := formatConversation(messages)
	prompt := fmt.Sprintf(extractPrompt, conversation)

	response, err := a.extractor.Chat(ctx, "", []llm.Message{{Role: "user", Content: prompt}})
	if err != nil {
		logger.Error("fact extraction failed", "error", err)
		return
	}

	facts, err := parseExtractedFacts(response)
	if err != nil {
		logger.Error("fact parsing failed", "error", err, "response", response)
		return
	}

	if len(facts) == 0 {
		logger.Debug("no facts extracted")
		return
	}

	userEntityID := a.getOrCreateUserEntity(sessionID)
	var contradictions []Contradiction

	for _, fact := range facts {
		domainID, ok := domainSlugToID[fact.Domain]
		if !ok {
			domainID = 1
		}

		result, err := a.memory.AddFact(&userEntityID, domainID, fact.Field, fact.Value, fact.Confidence)
		if err != nil {
			logger.Error("failed to store fact", "error", err, "field", fact.Field)
			continue
		}

		if result.Superseded != nil {
			contradictions = append(contradictions, Contradiction{
				Field:    fact.Field,
				OldValue: result.Superseded.Value,
				NewValue: fact.Value,
			})
			logger.Info("fact superseded", "field", fact.Field, "old", result.Superseded.Value, "new", fact.Value)
		} else {
			logger.Info("fact remembered", "field", fact.Field, "value", fact.Value, "domain", fact.Domain)
		}
	}

	if len(contradictions) > 0 && a.notify != nil {
		chatID := extractChatID(sessionID)
		if chatID != 0 {
			message := a.formatContradictionAlert(ctx, contradictions)
			a.notify(chatID, message)
		}
	}
}

func extractChatID(sessionID string) int64 {
	parts := strings.SplitN(sessionID, ":", 2)
	if len(parts) != 2 {
		return 0
	}
	var chatID int64
	fmt.Sscanf(parts[1], "%d", &chatID)
	return chatID
}

const contradictionPrompt = `You just noticed some inconsistencies between what you previously knew and what the user just said. Ask them about it naturally in your own words.

Changes detected:
%s

Keep it brief (1-2 sentences). Be curious, not accusatory. You've already updated your memory with the new info - just checking if they meant to change it.`

func (a *Agent) formatContradictionAlert(ctx context.Context, contradictions []Contradiction) string {
	var changes strings.Builder
	for _, c := range contradictions {
		fmt.Fprintf(&changes, "- %s: was \"%s\", now \"%s\"\n", c.Field, c.OldValue, c.NewValue)
	}

	prompt := fmt.Sprintf(contradictionPrompt, changes.String())

	response, err := a.extractor.Chat(ctx, a.systemPrompt, []llm.Message{
		{Role: "user", Content: prompt},
	})
	if err != nil {
		return fmt.Sprintf("I noticed you changed %s from \"%s\" to \"%s\" - just checking that's right?",
			contradictions[0].Field, contradictions[0].OldValue, contradictions[0].NewValue)
	}

	return response
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

func formatConversation(messages []llm.Message) string {
	var sb strings.Builder

	for _, msg := range messages {
		fmt.Fprintf(&sb, "%s: %s\n", msg.Role, msg.Content)
	}

	return sb.String()
}

func parseExtractedFacts(response string) ([]extractedFact, error) {
	response = strings.TrimSpace(response)

	start := strings.Index(response, "[")
	end := strings.LastIndex(response, "]")

	if start == -1 || end == -1 || end < start {
		return nil, fmt.Errorf("no JSON array found")
	}

	jsonStr := response[start : end+1]
	var facts []extractedFact

	if err := json.Unmarshal([]byte(jsonStr), &facts); err != nil {
		return nil, err
	}

	return facts, nil
}

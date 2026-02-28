package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/bowerhall/sheldon/internal/conversation"
	"github.com/bowerhall/sheldon/internal/llm"
	"github.com/bowerhall/sheldon/internal/logger"
	"github.com/bowerhall/sheldonmem"
)

const extractPrompt = `You are a fact and relationship extractor. Analyze the conversation and extract:
1. FACTS worth remembering
2. RELATIONSHIPS between people, places, or organizations

Extract facts about:
- The USER - preferences, life details, plans, events, changes
- SHELDON (the assistant) - instructions about behavior, communication style

Extract relationships when people mention:
- People they know (friends, family, colleagues)
- Places they work, live, or frequent
- Organizations they belong to

Return JSON with two arrays: "facts" and "relationships".

Facts format:
- "subject": "user" or "sheldon"
- "field": short key (e.g., "saturday_plans", "favorite_food")
- "value": the information
- "domain": identity, health, mind, beliefs, knowledge, relationships, career, finances, place, goals, preferences, routines, events, patterns
- "confidence": 0.0-1.0

Relationships format:
- "source": who/what the relationship is from (e.g., "user", "Sarah")
- "target": who/what the relationship is to (e.g., "Sarah", "Google")
- "target_type": "person", "place", or "organization"
- "relation": the relationship type (e.g., "knows", "works_at", "lives_in", "married_to", "friends_with", "sibling_of")
- "strength": 0.0-1.0

Only extract what is explicitly stated. If nothing to extract, use empty arrays.

Example:
{
  "facts": [
    {"subject": "user", "field": "name", "value": "John", "domain": "identity", "confidence": 0.95}
  ],
  "relationships": [
    {"source": "user", "target": "Sarah", "target_type": "person", "relation": "friends_with", "strength": 0.9},
    {"source": "user", "target": "Google", "target_type": "organization", "relation": "works_at", "strength": 0.95}
  ]
}

Conversation:
%s

Extract (JSON only):`

type extractedFact struct {
	Subject    string  `json:"subject"`
	Field      string  `json:"field"`
	Value      string  `json:"value"`
	Domain     string  `json:"domain"`
	Confidence float64 `json:"confidence"`
}

type extractedRelationship struct {
	Source     string  `json:"source"`
	Target     string  `json:"target"`
	TargetType string  `json:"target_type"`
	Relation   string  `json:"relation"`
	Strength   float64 `json:"strength"`
}

type extractionResult struct {
	Facts         []extractedFact         `json:"facts"`
	Relationships []extractedRelationship `json:"relationships"`
}

func (a *Agent) rememberExchange(ctx context.Context, sessionID string, userMessage, assistantResponse string) {
	conversation := fmt.Sprintf("user: %s\nassistant: %s\n", userMessage, assistantResponse)
	prompt := fmt.Sprintf(extractPrompt, conversation)

	response, err := a.extractor.Chat(ctx, "", []llm.Message{{Role: "user", Content: prompt}})
	if err != nil {
		logger.Error("fact extraction failed", "error", err)
		return
	}

	result, err := parseExtraction(response)
	if err != nil {
		logger.Error("extraction parsing failed", "error", err, "response", response)
		return
	}

	userEntityID := a.getOrCreateUserEntity(sessionID)
	sheldonEntityID := a.getSheldonEntityID()

	for _, fact := range result.Facts {
		domainID, ok := sheldonmem.DomainSlugToID[fact.Domain]
		if !ok {
			domainID = 1
		}

		var entityID int64
		subject := strings.ToLower(fact.Subject)
		if subject == "sheldon" || subject == "assistant" {
			entityID = sheldonEntityID
		} else {
			entityID = userEntityID
		}

		if entityID == 0 {
			logger.Error("no entity ID for fact", "subject", fact.Subject)
			continue
		}

		res, err := a.memory.AddFactWithContext(ctx, &entityID, domainID, fact.Field, fact.Value, fact.Confidence, false)
		if err != nil {
			logger.Error("failed to store fact", "error", err, "field", fact.Field)
			continue
		}

		if res.Superseded != nil {
			logger.Info("fact superseded", "subject", subject, "field", fact.Field, "old", res.Superseded.Value, "new", fact.Value)
		} else {
			logger.Info("fact remembered", "subject", subject, "field", fact.Field, "value", fact.Value, "domain", fact.Domain)
		}
	}

	for _, rel := range result.Relationships {
		sourceID := a.resolveEntityID(rel.Source, sessionID, userEntityID, sheldonEntityID)
		targetID := a.getOrCreateNamedEntity(rel.Target, rel.TargetType)

		if sourceID == 0 || targetID == 0 {
			logger.Warn("skipping relationship, missing entity", "source", rel.Source, "target", rel.Target)
			continue
		}

		_, err := a.memory.AddEdge(sourceID, targetID, rel.Relation, rel.Strength, "")
		if err != nil {
			logger.Error("failed to store relationship", "error", err, "relation", rel.Relation)
			continue
		}

		logger.Info("relationship remembered", "source", rel.Source, "relation", rel.Relation, "target", rel.Target)
	}
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

func parseExtraction(response string) (*extractionResult, error) {
	response = strings.TrimSpace(response)

	start := strings.Index(response, "{")
	end := strings.LastIndex(response, "}")

	if start == -1 || end == -1 || end < start {
		return nil, fmt.Errorf("no JSON object found")
	}

	jsonStr := response[start : end+1]
	var result extractionResult

	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// saveOverflowAsChunk stores evicted buffer messages for daily summaries
func (a *Agent) saveOverflowAsChunk(sessionID string, messages []conversation.Message) {
	var chunk strings.Builder
	for _, m := range messages {
		fmt.Fprintf(&chunk, "%s: %s\n", m.Role, m.Content)
	}
	if err := a.memory.SaveChunk(sessionID, chunk.String()); err != nil {
		logger.Error("failed to save conversation chunk", "error", err)
	}
	logger.Debug("saved overflow chunk", "session", sessionID, "messages", len(messages))
}

const summaryPrompt = `Summarize this day's conversations concisely. Focus on:
- Key topics discussed
- Decisions made
- Plans or events mentioned (especially changes/cancellations)
- Important information shared

Keep it to 2-3 paragraphs max. Write in past tense, third person ("User discussed...", "They decided...").

Conversations:
%s

Summary:`

// generatePendingSummaries creates daily summaries for any days with unsummarized chunks
func (a *Agent) generatePendingSummaries(ctx context.Context, sessionID string) {
	pendingDates, err := a.memory.GetPendingChunkDates(sessionID)
	if err != nil {
		logger.Error("failed to get pending chunk dates", "error", err)
		return
	}

	if len(pendingDates) == 0 {
		return
	}

	logger.Info("generating daily summaries", "session", sessionID, "days", len(pendingDates))

	for _, date := range pendingDates {
		chunks, err := a.memory.GetChunksForDate(sessionID, date)
		if err != nil {
			logger.Error("failed to get chunks for date", "error", err, "date", date)
			continue
		}

		if len(chunks) == 0 {
			continue
		}

		// combine all chunks for the day
		var combined strings.Builder
		for _, c := range chunks {
			combined.WriteString(c.Content)
			combined.WriteString("\n\n---\n\n")
		}

		// generate summary using extractor LLM
		prompt := fmt.Sprintf(summaryPrompt, combined.String())
		summary, err := a.extractor.Chat(ctx, "", []llm.Message{{Role: "user", Content: prompt}})
		if err != nil {
			logger.Error("failed to generate summary", "error", err, "date", date)
			continue
		}

		// save the summary (embedding handled internally by sheldonmem)
		if err := a.memory.SaveDailySummary(ctx, sessionID, date, summary); err != nil {
			logger.Error("failed to save daily summary", "error", err, "date", date)
			continue
		}

		logger.Info("daily summary generated", "session", sessionID, "date", date.Format("2006-01-02"))
	}
}

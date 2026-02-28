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

const extractPrompt = `You are a fact extractor. Analyze the conversation and extract facts worth remembering.

Extract facts about:
1. The USER - preferences, life details, plans, events, changes, cancellations
2. SHELDON (the assistant) - instructions about behavior, communication style, things to remember

IMPORTANT: Extract both static facts AND dynamic events:
- Static: "lives in Berlin", "allergic to peanuts", "works at Google"
- Events/Changes: "cancelled Saturday date", "moved grocery day to Sunday", "finished project X"

For updates/cancellations, use the SAME field key as the original fact would have:
- If "saturday_plans" was "date with Sarah", and now cancelled → {"field": "saturday_plans", "value": "cancelled"}
- If routine changed → use the same field, new value

Return a JSON array of facts. Each fact should have:
- "subject": either "user" or "sheldon"
- "field": short consistent key (e.g., "saturday_plans", "grocery_day", "current_project")
- "value": the actual information (can be "cancelled", "completed", "changed to X")
- "domain": one of: identity, health, mind, beliefs, knowledge, relationships, career, finances, place, goals, preferences, routines, events, patterns
- "confidence": 0.0-1.0

Only extract facts that are explicitly stated or strongly implied. Do not invent facts.
If no facts are worth remembering, return an empty array: []

Examples:
[
  {"subject": "user", "field": "name", "value": "John", "domain": "identity", "confidence": 0.95},
  {"subject": "user", "field": "saturday_plans", "value": "cancelled", "domain": "events", "confidence": 0.9},
  {"subject": "user", "field": "grocery_day", "value": "moved to Sunday", "domain": "routines", "confidence": 0.85},
  {"subject": "sheldon", "field": "humor_style", "value": "use dad jokes", "domain": "preferences", "confidence": 0.9}
]

Conversation:
%s

Extract facts (JSON only, no explanation):`

type extractedFact struct {
	Subject    string  `json:"subject"`
	Field      string  `json:"field"`
	Value      string  `json:"value"`
	Domain     string  `json:"domain"`
	Confidence float64 `json:"confidence"`
}

// rememberExchange extracts facts only from the latest user message and response
// This avoids re-extracting from the conversation buffer
func (a *Agent) rememberExchange(ctx context.Context, sessionID string, userMessage, assistantResponse string) {
	conversation := fmt.Sprintf("user: %s\nassistant: %s\n", userMessage, assistantResponse)
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
	sheldonEntityID := a.getSheldonEntityID()

	for _, fact := range facts {
		domainID, ok := sheldonmem.DomainSlugToID[fact.Domain]
		if !ok {
			domainID = 1
		}

		// determine which entity to save to
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

		result, err := a.memory.AddFactWithContext(ctx, &entityID, domainID, fact.Field, fact.Value, fact.Confidence, false)
		if err != nil {
			logger.Error("failed to store fact", "error", err, "field", fact.Field)
			continue
		}

		if result.Superseded != nil {
			logger.Info("fact superseded", "subject", subject, "field", fact.Field, "old", result.Superseded.Value, "new", fact.Value)
		} else {
			logger.Info("fact remembered", "subject", subject, "field", fact.Field, "value", fact.Value, "domain", fact.Domain)
		}
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

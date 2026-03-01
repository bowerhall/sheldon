package sheldonmem

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// AddDailyMessage stores a message for same-day recall
func (s *Store) AddDailyMessage(sessionID, role, content string) error {
	_, err := s.db.Exec(`
		INSERT INTO daily_messages (session_id, role, content)
		VALUES (?, ?, ?)`,
		sessionID, role, content)
	return err
}

// SearchToday searches today's messages for a query string
func (s *Store) SearchToday(sessionID, query string) ([]DailyMessage, error) {
	rows, err := s.db.Query(`
		SELECT id, session_id, role, content, created_at, date
		FROM daily_messages
		WHERE session_id = ? AND date = date('now') AND content LIKE ?
		ORDER BY created_at ASC`,
		sessionID, "%"+query+"%")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []DailyMessage
	for rows.Next() {
		var m DailyMessage
		var createdAt string
		if err := rows.Scan(&m.ID, &m.SessionID, &m.Role, &m.Content, &createdAt, &m.Date); err != nil {
			return nil, err
		}
		// SQLite stores CURRENT_TIMESTAMP as UTC; parse and convert to local time
		if t, err := time.Parse("2006-01-02 15:04:05", createdAt); err == nil {
			m.CreatedAt = t.In(time.Local)
		}
		messages = append(messages, m)
	}
	return messages, nil
}

// SearchRecentByKeyword searches recent messages (last N days) for a keyword
// Used by cron system to find same-day context not yet embedded
// Supports smart matching: case-insensitive, splits on delimiters
// Requires at least 2 tokens to match (or all if only 1-2 tokens)
func (s *Store) SearchRecentByKeyword(sessionID, keyword string, daysBack int) ([]DailyMessage, error) {
	if daysBack <= 0 {
		daysBack = 1 // default to today only
	}

	// Tokenize keyword: split on common delimiters, filter empty
	tokens := tokenizeKeyword(keyword)
	if len(tokens) == 0 {
		return nil, nil
	}

	// Build OR query for all tokens (case-insensitive) - we filter in Go
	var conditions []string
	var args []any
	args = append(args, sessionID, fmt.Sprintf("-%d", daysBack))

	for _, token := range tokens {
		conditions = append(conditions, "LOWER(content) LIKE LOWER(?)")
		args = append(args, "%"+token+"%")
	}

	query := fmt.Sprintf(`
		SELECT id, session_id, role, content, created_at, date
		FROM daily_messages
		WHERE session_id = ?
		AND date >= date('now', ? || ' days')
		AND (%s)
		ORDER BY created_at DESC
		LIMIT 50`, strings.Join(conditions, " OR "))

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Determine minimum matches required
	minMatches := 2
	if len(tokens) <= 2 {
		minMatches = len(tokens) // require all if only 1-2 tokens
	}

	var messages []DailyMessage
	for rows.Next() {
		var m DailyMessage
		var createdAt string
		if err := rows.Scan(&m.ID, &m.SessionID, &m.Role, &m.Content, &createdAt, &m.Date); err != nil {
			return nil, err
		}
		if t, err := time.Parse("2006-01-02 15:04:05", createdAt); err == nil {
			m.CreatedAt = t.In(time.Local)
		}

		// Count token matches
		contentLower := strings.ToLower(m.Content)
		matchCount := 0
		for _, token := range tokens {
			if strings.Contains(contentLower, token) {
				matchCount++
			}
		}

		// Only include if enough tokens match
		if matchCount >= minMatches {
			messages = append(messages, m)
			if len(messages) >= 20 {
				break
			}
		}
	}
	return messages, nil
}

// tokenizeKeyword splits a keyword into searchable tokens
func tokenizeKeyword(keyword string) []string {
	// Replace common delimiters with space
	keyword = strings.ToLower(keyword)
	for _, delim := range []string{"-", "_", ".", ":"} {
		keyword = strings.ReplaceAll(keyword, delim, " ")
	}

	// Split and filter
	var tokens []string
	for _, token := range strings.Fields(keyword) {
		token = strings.TrimSpace(token)
		if len(token) >= 2 { // skip single chars
			tokens = append(tokens, token)
		}
	}
	return tokens
}

// GetTodayMessages returns all messages from today
func (s *Store) GetTodayMessages(sessionID string) ([]DailyMessage, error) {
	return s.GetMessagesForDate(sessionID, time.Now().Format("2006-01-02"))
}

// GetMessagesForDate returns all messages for a specific date
func (s *Store) GetMessagesForDate(sessionID, date string) ([]DailyMessage, error) {
	rows, err := s.db.Query(`
		SELECT id, session_id, role, content, created_at, date
		FROM daily_messages
		WHERE session_id = ? AND date = ?
		ORDER BY created_at ASC`,
		sessionID, date)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []DailyMessage
	for rows.Next() {
		var m DailyMessage
		var createdAt string
		if err := rows.Scan(&m.ID, &m.SessionID, &m.Role, &m.Content, &createdAt, &m.Date); err != nil {
			return nil, err
		}
		// SQLite stores CURRENT_TIMESTAMP as UTC; parse and convert to local time
		if t, err := time.Parse("2006-01-02 15:04:05", createdAt); err == nil {
			m.CreatedAt = t.In(time.Local)
		}
		messages = append(messages, m)
	}
	return messages, nil
}

// GetSessionsWithMessagesForDate returns all session IDs that have messages for a date
func (s *Store) GetSessionsWithMessagesForDate(date string) ([]string, error) {
	rows, err := s.db.Query(`
		SELECT DISTINCT session_id
		FROM daily_messages
		WHERE date = ?`,
		date)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessions []string
	for rows.Next() {
		var sessionID string
		if err := rows.Scan(&sessionID); err != nil {
			return nil, err
		}
		sessions = append(sessions, sessionID)
	}
	return sessions, nil
}

// DeleteMessagesForDate removes messages for a specific date (after processing)
func (s *Store) DeleteMessagesForDate(sessionID, date string) error {
	_, err := s.db.Exec(`DELETE FROM daily_messages WHERE session_id = ? AND date = ?`, sessionID, date)
	return err
}

// EntityResolver resolves entity names to IDs for fact storage
type EntityResolver interface {
	GetOrCreateUserEntity(sessionID string) int64
	GetSheldonEntityID() int64
	GetOrCreateNamedEntity(name, entityType string) int64
	ResolveEntityID(name, sessionID string, userID, sheldonID int64) int64
}

const endOfDayPrompt = `Analyze this day's conversations and provide two things:

1. EXTRACTION: Extract facts and relationships worth remembering long-term.
2. SUMMARY: A concise summary of the day's conversations.

Conversations:
%s

Return JSON in this exact format:
{
  "facts": [
    {"subject": "user", "field": "short_key", "value": "the info", "domain": "one of: identity, health, mind, beliefs, knowledge, relationships, career, finances, place, goals, preferences, routines, events, patterns", "confidence": 0.0-1.0}
  ],
  "relationships": [
    {"source": "user", "target": "name", "target_type": "person/place/organization", "relation": "knows/works_at/lives_in/etc", "strength": 0.0-1.0}
  ],
  "summary": "2-3 paragraph summary in past tense, third person"
}

Rules:
- Extract facts about USER (preferences, life details, plans) and SHELDON (behavioral instructions)
- Only extract explicitly stated facts, never infer
- If no facts worth extracting, use empty arrays
- Summary should focus on: key topics, decisions made, plans mentioned, important information shared
- If conversations had contradictions (e.g., "going to Portland" then "actually Seattle"), extract only the final/corrected value`

// LLM interface for end-of-day processing
type LLM interface {
	Chat(ctx context.Context, systemPrompt string, messages []LLMMessage) (string, error)
}

type LLMMessage struct {
	Role    string
	Content string
}

// ProcessEndOfDay runs extraction and summarization for all pending conversations
// If includeToday is false, only processes dates before today (normal 3am behavior)
// If includeToday is true, processes ALL pending messages including today (for manual triggers)
// Messages are kept after extraction for same-day keyword search
func (s *Store) ProcessEndOfDay(ctx context.Context, llm LLM, resolver EntityResolver, includeToday bool) error {
	var excludeDate string
	if includeToday {
		// Include everything by using tomorrow as the cutoff
		excludeDate = time.Now().UTC().AddDate(0, 0, 1).Format("2006-01-02")
	} else {
		// Normal: exclude today, only process previous days
		excludeDate = time.Now().UTC().Format("2006-01-02")
	}

	// Get all pending (session, date) pairs
	pending, err := s.getPendingDailyMessages(excludeDate)
	if err != nil {
		return fmt.Errorf("failed to get pending messages: %w", err)
	}

	if len(pending) == 0 {
		return nil
	}

	for _, p := range pending {
		if err := s.processSessionEndOfDay(ctx, llm, resolver, p.sessionID, p.date); err != nil {
			// log error but continue with other sessions
			continue
		}
		// Messages are kept for same-day keyword search, cleaned up separately
	}

	return nil
}

type pendingSession struct {
	sessionID string
	date      string
}

// getPendingDailyMessages returns all (session_id, date) pairs with unprocessed messages
func (s *Store) getPendingDailyMessages(excludeDate string) ([]pendingSession, error) {
	rows, err := s.db.Query(`
		SELECT DISTINCT session_id, date
		FROM daily_messages
		WHERE date < ?
		ORDER BY date ASC`,
		excludeDate)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var pending []pendingSession
	for rows.Next() {
		var p pendingSession
		if err := rows.Scan(&p.sessionID, &p.date); err != nil {
			return nil, err
		}
		pending = append(pending, p)
	}
	return pending, nil
}

func (s *Store) processSessionEndOfDay(ctx context.Context, llm LLM, resolver EntityResolver, sessionID, date string) error {
	messages, err := s.GetMessagesForDate(sessionID, date)
	if err != nil {
		return fmt.Errorf("failed to get messages: %w", err)
	}

	if len(messages) == 0 {
		return nil
	}

	// Build conversation text
	var convoText strings.Builder
	for _, m := range messages {
		fmt.Fprintf(&convoText, "%s: %s\n", m.Role, m.Content)
	}

	// One LLM call for extraction + summary
	prompt := fmt.Sprintf(endOfDayPrompt, convoText.String())
	response, err := llm.Chat(ctx, "", []LLMMessage{{Role: "user", Content: prompt}})
	if err != nil {
		return fmt.Errorf("LLM call failed: %w", err)
	}

	// Parse the combined result
	result, err := parseEndOfDayResult(response)
	if err != nil {
		return fmt.Errorf("failed to parse result: %w", err)
	}

	// Store extracted facts
	userEntityID := resolver.GetOrCreateUserEntity(sessionID)
	sheldonEntityID := resolver.GetSheldonEntityID()

	for _, fact := range result.Facts {
		domainID, ok := DomainSlugToID[fact.Domain]
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
			continue
		}

		_, err := s.AddFactWithContext(ctx, &entityID, domainID, fact.Field, fact.Value, fact.Confidence, false)
		if err != nil {
			continue
		}
	}

	// Store extracted relationships
	for _, rel := range result.Relationships {
		sourceID := resolver.ResolveEntityID(rel.Source, sessionID, userEntityID, sheldonEntityID)
		targetID := resolver.GetOrCreateNamedEntity(rel.Target, rel.TargetType)

		if sourceID == 0 || targetID == 0 {
			continue
		}

		s.AddEdge(sourceID, targetID, rel.Relation, rel.Strength, "")
	}

	// Store summary (with embedding for semantic search)
	if result.Summary != "" {
		dateTime, _ := time.Parse("2006-01-02", date)
		s.SaveDailySummary(ctx, sessionID, dateTime, result.Summary)
	}

	return nil
}

// deleteProcessedMessages removes messages after extraction (called separately to allow skipping today)
func (s *Store) deleteProcessedMessages(sessionID, date string) {
	s.DeleteMessagesForDate(sessionID, date)
}

var unquotedKeyRe = regexp.MustCompile(`(\s|,)([a-z_]+):\s*`)
var invalidEscapeRe = regexp.MustCompile(`\\([^"\\\/bfnrtu])`)

// ProcessEndOfDayForSession processes a specific session with provided conversation content
// Used for legacy chunk processing or manual processing
func (s *Store) ProcessEndOfDayForSession(ctx context.Context, llm LLM, resolver EntityResolver, sessionID, date, conversationContent string) error {
	// One LLM call for extraction + summary
	prompt := fmt.Sprintf(endOfDayPrompt, conversationContent)
	response, err := llm.Chat(ctx, "", []LLMMessage{{Role: "user", Content: prompt}})
	if err != nil {
		return fmt.Errorf("LLM call failed: %w", err)
	}

	// Parse the combined result
	result, err := parseEndOfDayResult(response)
	if err != nil {
		return fmt.Errorf("failed to parse result: %w", err)
	}

	// Store extracted facts
	userEntityID := resolver.GetOrCreateUserEntity(sessionID)
	sheldonEntityID := resolver.GetSheldonEntityID()

	for _, fact := range result.Facts {
		domainID, ok := DomainSlugToID[fact.Domain]
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
			continue
		}

		s.AddFactWithContext(ctx, &entityID, domainID, fact.Field, fact.Value, fact.Confidence, false)
	}

	// Store extracted relationships
	for _, rel := range result.Relationships {
		sourceID := resolver.ResolveEntityID(rel.Source, sessionID, userEntityID, sheldonEntityID)
		targetID := resolver.GetOrCreateNamedEntity(rel.Target, rel.TargetType)

		if sourceID == 0 || targetID == 0 {
			continue
		}

		s.AddEdge(sourceID, targetID, rel.Relation, rel.Strength, "")
	}

	// Store summary (with embedding for semantic search)
	if result.Summary != "" {
		dateTime, _ := time.Parse("2006-01-02", date)
		s.SaveDailySummary(ctx, sessionID, dateTime, result.Summary)
	}

	return nil
}

func parseEndOfDayResult(response string) (*EndOfDayResult, error) {
	response = strings.TrimSpace(response)

	start := strings.Index(response, "{")
	end := strings.LastIndex(response, "}")

	if start == -1 || end == -1 || end < start {
		return nil, fmt.Errorf("no JSON object found")
	}

	jsonStr := response[start : end+1]

	// Fix unquoted keys
	jsonStr = unquotedKeyRe.ReplaceAllString(jsonStr, `$1"$2": `)
	// Fix invalid escape sequences
	jsonStr = invalidEscapeRe.ReplaceAllString(jsonStr, `$1`)

	var result EndOfDayResult
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		return nil, err
	}

	return &result, nil
}

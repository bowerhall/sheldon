# Phase 1: Daily Driver

**Timeline: 3-5 days**
**Depends on: Phase 0**
**Goal: Proactive assistant you actually use every day**

## Tasks

### 1. Morning Briefing (Day 1)
- HEARTBEAT.md entry: "Every morning at 8am, sheldonmem.Recall for D10 (Goals) + D12 (Rhythms)"
- Summarize upcoming deadlines and today's priorities
- Sheldon's heartbeat spawns subagent

### 2. Setup Interview (Day 1)
- Structured conversation to seed initial facts across all 14 domains
- sheldonmem.Remember handles extraction automatically
- No engineering — just a conversation

### 3. Budget Tracker (Day 2)
- Log Claude API token counts per response
- Track daily running total in workspace JSON
- Warn at 80%, hard stop at 100%
- Cron resets daily

### 4. Contradiction Alerts (Day 2-3)
- sheldonmem.Remember returns contradiction info when superseding
- Alert: "You previously said X, but now mentioned Y. Which is correct?"
- Inline keyboard: [Keep old] [Accept new]

### 5. Error Alerting (Day 3)
- Telegram notification: SQLite errors, critical failures, budget breach
- Cooldown: max 1 alert per type per hour

### 6. Goal Tracking Cron (Day 4)
- Weekly cron: sheldonmem.Recall D10, prompt for progress updates
- "You set a goal to finish OMSCS application by March. How's it going?"

## Success Criteria
- [ ] Morning briefing fires at 8am
- [ ] Budget tracker enforces daily limit
- [ ] You message Sheldon 10+ times/day

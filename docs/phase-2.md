# Phase 2: Action Skills

**Timeline: 1-1.5 weeks**
**Depends on: Phase 1**
**Goal: Sheldon takes autonomous action**

## Tasks

### 1. Claude Code Bridge (Days 1-3)

Full design: **[coder-bridge.md](coder-bridge.md)**

- `claude_code` tool registered in ToolRegistry
- Invoked via Go Agent SDK wrapping Claude Code CLI subprocess
- Context bridging: Sheldon generates per-task `CLAUDE.md` from sheldonmem (preferences, deploy target, relevant facts)
- Sandbox: isolated workspace, stripped environment (no Sheldon credentials), restricted tool set
- Complexity tiers: simple (5min/10 turns) → project (45min/multi-pass orchestration)
- Output sanitization: credential regex scan on all results before surfacing to user
- Multi-pass orchestration for complex tasks (backend → frontend → Dockerfile → tests)
- Integration with Phase 6: deployable artifacts (Dockerfile) chain into self-extension pipeline

### 2. Apartment Hunter Skill (Days 3-5)
- SKILL.md: query sheldonmem D9 + D8 + D11 for criteria
- WebSearch + WebFetch for listings
- Filter against budget, location, size from memory
- Generate Bewerbung using personal facts from sheldonmem
- Cron: scan every 30 minutes
- Autonomy: confirm (default), autonomous (opt-in)

### 3. Strategy Engine Skill (Days 5-6)
- Triggered when router classifies as decision-type → Opus
- Multi-framework analysis: pros/cons, BATNA, regret minimization, weighted matrix
- Pulls context from all relevant domains via sheldonmem.Recall

### 4. Financial Advisor Skill (Days 6-7)
- Budget analysis, expense categorization, investment tracking
- Monthly cron: spending patterns, investment performance
- Reads D8 from sheldonmem

### 5. Output Sanitizer (Day 7)
- Integrated into Coder Bridge (see [coder-bridge.md](coder-bridge.md#output-handling))
- Regex patterns: Anthropic keys, Telegram tokens, AWS keys, GitHub PATs, Voyage keys, password assignments
- Redacts matches in-place, surfaces warnings to outer agent loop
- Runs on every Claude Code invocation before results reach user or memory

## Success Criteria
- [ ] Claude Code runs sandboxed
- [ ] Apartment hunter: search → filter → present → Bewerbung
- [ ] Strategy engine: multi-framework analysis

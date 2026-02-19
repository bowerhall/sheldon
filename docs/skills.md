# Skills System

## How Skills Work

PicoClaw's skill framework: SKILL.md files in `workspace/skills/` loaded on demand.

When a skill is triggered (by router classification or explicit command), PicoClaw loads the SKILL.md and passes it to the LLM as additional context. The LLM then uses available tools to execute the skill.

## Built-in Skills

### Apartment Hunter
- Trigger: router detects apartment/housing query, or cron schedule
- Memory: koramem.Recall D9 (Place) + D8 (Finances) + D11 (Preferences)
- Actions: WebSearch → WebFetch → filter → present → generate Bewerbung
- Autonomy: confirm (default), autonomous (opt-in via toggle)

### Strategy Engine
- Trigger: router classifies as decision-type → selects Opus model
- Memory: koramem.Recall across all relevant domains
- Frameworks: pros/cons, BATNA, regret minimization, weighted matrix, pre-mortem
- Output: structured analysis with clear recommendation

### Financial Advisor
- Trigger: money/budget queries, or monthly cron
- Memory: koramem.Recall D8 (Finances)
- Actions: spending analysis, budget tracking, investment review

### Claude Code
- Full design: [claude-code-bridge.md](claude-code-bridge.md)
- Trigger: code/technical tasks requiring execution
- Invocation: Go Agent SDK → Claude Code CLI subprocess
- Context: per-task CLAUDE.md generated from koramem (language, style, deploy target)
- Sandbox: isolated workspace, stripped env, restricted tools, complexity-tiered timeouts
- Multi-pass: complex tasks orchestrated across multiple invocations
- Output sanitized before display

## Custom Skills

Users can create new SKILL.md files and drop them into `workspace/skills/`. Kora discovers them automatically. Skills can reference koramem domains, use any PicoClaw tool, and register cron jobs.

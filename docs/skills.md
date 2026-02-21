# Skills System

## How Skills Work

Skill framework: SKILL.md files in `workspace/skills/` loaded on demand.

When a skill is triggered (by router classification or explicit command), Sheldon loads the SKILL.md and passes it to the LLM as additional context. The LLM then uses available tools to execute the skill.

## Built-in Skills

### Apartment Hunter
- Trigger: router detects apartment/housing query, or cron schedule
- Memory: sheldonmem.Recall D9 (Place) + D8 (Finances) + D11 (Preferences)
- Actions: WebSearch → WebFetch → filter → present → generate Bewerbung
- Autonomy: confirm (default), autonomous (opt-in via toggle)

### Strategy Engine
- Trigger: router classifies as decision-type → selects Opus model
- Memory: sheldonmem.Recall across all relevant domains
- Frameworks: pros/cons, BATNA, regret minimization, weighted matrix, pre-mortem
- Output: structured analysis with clear recommendation

### Financial Advisor
- Trigger: money/budget queries, or monthly cron
- Memory: sheldonmem.Recall D8 (Finances)
- Actions: spending analysis, budget tracking, investment review

### Coder
- Full design: [coder-bridge.md](coder-bridge.md)
- Trigger: code/technical tasks requiring execution
- Modes:
  - **Subprocess**: Direct Ollama CLI in sandbox directory
  - **Isolated**: Ephemeral Docker container (`CODER_ISOLATED=true`)
- Context: per-task CONTEXT.md generated from sheldonmem (language, style, preferences)
- Sandbox: isolated workspace, stripped env, restricted tools, complexity-tiered timeouts
- Model: Kimi K2.5 via Ollama (or NVIDIA NIM)
- Output sanitized before display

## Custom Skills

Users can create new SKILL.md files and drop them into `workspace/skills/`. Sheldon discovers them automatically. Skills can reference sheldonmem domains, use any available tool, and register cron jobs.

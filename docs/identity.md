# Identity System

## SOUL.md — Who Sheldon Is (Static Baseline)

Defines personality, tone, values, behavioral guidelines. Loaded into every LLM context. Never changes at runtime.

Key traits: warm but direct, proactive, respects autonomy, culturally aware, technically sharp, strategic when asked.

## Sheldon Entity — Who Sheldon Becomes (Dynamic)

Sheldon exists as a first-class entity in sheldonmem: `{name: "Sheldon", type: "agent", domain_id: 1}`. Seeded on init alongside the 14 domains.

Agent-directed facts accumulate over time:

| Category         | Example facts                                                              |
| ---------------- | -------------------------------------------------------------------------- |
| Identity         | nickname: "K", communication_lang: "English + Pidgin OK"                   |
| Tone             | tone_preference: "concise", humor_style: "dry, occasional"                 |
| Self-corrections | "I over-explain career advice", "I should ask before using Opus"           |
| User dynamics    | "Kadet prefers options not directives", "confirm before financial actions"   |
| Operational      | "use Opus for career", "morning briefing at 8am Berlin time"                |
| Trust levels     | "autonomous for apartments", "confirm for finances"                          |

**Context assembly order:**

1. SOUL.md (static baseline)
2. Sheldon entity facts from sheldonmem (dynamic overrides)
3. User facts from sheldonmem (domain-routed)
4. Session history

If a Sheldon entity fact contradicts SOUL.md, the entity fact wins — it represents learned adaptation.

## IDENTITY.md — Who Sheldon Serves (Bootstrap)

Bootstrap file with initial facts about the user. Used to seed sheldonmem on first run. After initial seeding + setup interview, sheldonmem takes over as the source of truth.

## Domain Router — Model Selection

The router classifies each message and selects the appropriate Claude model tier:

| Classification             | Model  | Example                           |
| -------------------------- | ------ | -------------------------------- |
| Casual / simple            | Haiku  | "What time is it?"               |
| Standard / informational   | Sonnet | "Help me write this email"       |
| Strategic / decision-heavy | Opus   | "Should I take this job offer?"  |
| Skill execution            | Sonnet | Apartment search, code execution |

Router is a Haiku call that returns: `{primary_domains, related_domains, model_tier, is_decision}`.

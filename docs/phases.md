# Project Phases

## Overview

Sheldon is built in concentric rings. Each phase is independently useful.

```
                    Phase 0 (1-2 weeks)
                    The Brain + sheldonmem
                        │
              ┌─────────┼─────────┐
              ▼                   ▼
        Phase 1 (3-5 days)   Phase 3 (3-5 days)
        Daily Driver         Infrastructure
              │                   │
              ▼                   ├──────────┐
        Phase 2 (1-1.5 wk)       ▼          ▼
        Action Skills        Phase 4 (1 wk) Phase 6 (2-3 wk)
                             The Voice      Self-Extend
                                  │
                                  ▼
                             Phase 5 (2-3 wk)
                             Mac App
                                  │
                                  ▼
                             Phase 7 (4-6 wk)
                             Mobile
```

**Total to daily-usable assistant (Phase 0+1): ~2.5 weeks.**
**Total to fully autonomous with skills (Phase 0+1+2): ~4 weeks.**
**Total to complete platform: ~5-6 months.**

## Phase Summary

| Phase | Name | Timeline | What You Get |
|-------|------|----------|-------------|
| 0 | The Brain + sheldonmem | 1-2 weeks | Telegram bot with 14-domain graph memory |
| 1 | Daily Driver | 3-5 days | Proactive briefings, reminders, review flow |
| 2 | Action Skills | 1-1.5 weeks | Apartment hunter, strategy engine, Coder |
| 3 | Infrastructure | 3-5 days | MinIO, backups, monitoring, full RBAC |
| 4 | The Voice | 1 week | Bidirectional voice via Telegram |
| 5 | Mac App | 2-3 weeks | Menu bar companion with voice |
| 6 | Self-Extend | 2-3 weeks | Sheldon builds and deploys its own services |
| 7 | Mobile | 4-6 weeks | iOS + Android + generative UI |

## Dependencies

Phases 1+2 and Phase 3 can run in parallel.
Phases 4/5 and Phase 6 can run in parallel.

## Cost Estimates

### API Costs (Monthly, Active Daily Use)

| Component | Calls/Day | Model | Est. Cost |
|-----------|-----------|-------|-----------|
| Domain Router | 20-30 | Haiku | ~$0.15 |
| sheldonmem extraction | 20-30 | Haiku | ~$0.15 |
| Embedding generation | 20-30 | Voyage AI | ~$0.10 |
| Response generation | 20-30 | Sonnet | ~$2.00 |
| Strategic decisions | 2-5 | Opus | ~$1.50 |
| Skill execution | 2-5 | Sonnet + Code | ~$1.00 |
| **Total API** | | | **~$5-7/mo** |

### Infrastructure Costs (Monthly)

| Phase | Infrastructure | Cost |
|-------|---------------|------|
| 0-3 | Hetzner CX32 | ~€8/mo |
| 4+ | Hetzner CX42 | ~€15/mo |

**Total monthly: ~€8-15.**

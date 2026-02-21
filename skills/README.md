# Sheldon Skills

Skills are markdown files that inject context into the coder's prompt based on task keywords.

## Format

Skills follow the [ClawHub standard](https://github.com/openclaw/clawhub/blob/main/docs/skill-format.md):

```yaml
---
name: skill-name
description: Short description
version: 1.0.0
metadata:
  openclaw:
    requires:
      bins:
        - required-binary
      env:
        - REQUIRED_ENV_VAR
    alwaysActive: false
---

# Skill Content

Instructions and examples here...
```

## Included Skills

| Skill | Description |
|-------|-------------|
| `general` | Coding guidelines, always active |
| `go-api` | Go API patterns with net/http |
| `python-api` | FastAPI patterns |
| `dockerfile` | Multi-stage Docker builds |
| `compose` | Docker Compose with Traefik |
| `git-workflow` | Branching and conventional commits |

## How Skills Work

1. User asks Sheldon to write code
2. Coder analyzes the task for keywords
3. Matching skills are injected into the prompt
4. `general` is always included

## Adding Skills

Create a new `.md` file in this directory with YAML frontmatter.

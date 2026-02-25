---
name: general
description: General coding guidelines and best practices
version: 1.0.0
metadata:
  openclaw:
    alwaysActive: true
---

# General Coding Guidelines

## Git Workflow

- initialize git repo: `git init`
- configure git user if GIT_USER_NAME and GIT_USER_EMAIL are set
- NEW projects: work on main branch
- EXISTING repos: create feature branch (feat/, fix/, chore/)
- use conventional commits: `feat:`, `fix:`, `test:`, `chore:`, `docs:`
- commit after each logical unit of work
- test before committing
- never commit secrets or .env files

## Conventional Commits

```
feat: add user authentication
fix: handle null response from api
test: add unit tests for payment service
chore: add dockerfile and compose config
docs: add api documentation
```

## Always

- handle errors, don't ignore them
- use environment variables for configuration
- add .gitignore for language-specific ignores
- keep code minimal and focused

## Database

- **Default to SQLite** for simple/single-user apps - no external deps, just a file
- **External DB URL** for production apps - ask user if they have one (Supabase, PlanetScale, etc.)
- Always use a volume mount for SQLite persistence: `/data/app.db`
- Never hardcode database credentials - use environment variables

## Never

- don't add features not requested
- don't over-engineer simple tasks
- don't add unnecessary dependencies
- don't push to main on existing repos

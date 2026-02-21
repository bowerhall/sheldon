---
name: git-workflow
description: Git branching and conventional commits
version: 1.0.0
metadata:
  openclaw:
    requires:
      bins:
        - git
    env:
      - GIT_USER_NAME
      - GIT_USER_EMAIL
---

# Git Workflow

## Branching Strategy

**New projects:** work on main
```bash
git init
git checkout -b main
```

**Existing repos:** create feature branch
```bash
git checkout main && git pull
git checkout -b feat/add-feature
```

## Branch Naming

- `feat/` - new features
- `fix/` - bug fixes
- `chore/` - maintenance
- `docs/` - documentation
- `refactor/` - restructuring
- `test/` - test additions

## Conventional Commits

Format: `<type>: <description>`

```
feat: add user authentication
fix: handle null response from API
docs: add setup instructions
refactor: extract email service
test: add payment processor tests
chore: update dependencies
```

Rules:
- lowercase, no period
- imperative mood ("add" not "added")
- max 72 characters

## Workflow Example

```bash
git checkout -b feat/add-weather-api

git add src/weather.py
git commit -m "feat: add weather api client"

git add tests/
git commit -m "test: add weather service tests"

git push -u origin feat/add-weather-api
```

## Rules

- NEVER commit secrets or .env files
- NEVER push to main on protected repos
- always test before committing
- keep commits atomic

# git workflow for project development

## branching strategy

### new projects
for brand new repos, work directly on main:
```bash
git init
git checkout -b main
# ... work ...
git push -u origin main
```

### existing repos (features, fixes, improvements)
ALWAYS create a new branch from main:
```bash
# fetch latest
git fetch origin
git checkout main
git pull origin main

# create feature branch
git checkout -b feat/add-voice-support
# or
git checkout -b fix/handle-timeout-errors
# or
git checkout -b chore/update-dependencies
```

### branch naming convention
use conventional prefixes:
- `feat/` - new features (feat/add-authentication)
- `fix/` - bug fixes (fix/null-pointer-exception)
- `chore/` - maintenance (chore/update-deps)
- `docs/` - documentation (docs/add-api-reference)
- `refactor/` - code restructuring (refactor/extract-service)
- `test/` - test additions (test/add-integration-tests)

use kebab-case, be descriptive but concise.

## initialization
- run `git init` at the start of any new project
- configure git if env vars are set:
  ```bash
  git config user.name "$GIT_USER_NAME"
  git config user.email "$GIT_USER_EMAIL"
  ```
- add appropriate .gitignore for the language/framework

## conventional commits (REQUIRED)

format: `<type>: <description>`

### types
- `feat` - new feature for the user
- `fix` - bug fix for the user
- `docs` - documentation only
- `style` - formatting, no code change
- `refactor` - code change that neither fixes nor adds
- `test` - adding or updating tests
- `chore` - build process, deps, tooling

### examples
```
feat: add user authentication with JWT
fix: handle null response from weather API
docs: add setup instructions to README
refactor: extract email service from controller
test: add unit tests for payment processor
chore: update Go to 1.24
```

### rules
- lowercase, no period at end
- imperative mood ("add" not "added" or "adds")
- max 72 characters
- explain WHAT and WHY, not HOW

## commit strategy
commit after EACH milestone:
1. initial project structure
2. each feature or component completed and working
3. each bug fix
4. tests added/updated
5. documentation updates
6. deployment files

## workflow example (existing repo)
```bash
# start new feature
git checkout main
git pull origin main
git checkout -b feat/add-weather-api

# work and commit incrementally
git add src/weather.py
git commit -m "feat: add weather api client"

git add src/weather.py
git commit -m "feat: add caching for weather responses"

git add tests/
git commit -m "test: add weather service tests"

git add Dockerfile docker-compose.yml
git commit -m "chore: add dockerfile and compose config"

# push branch (NOT main)
git push -u origin feat/add-weather-api
```

## pushing to remote
if GIT_TOKEN, GIT_ORG_URL, and GIT_REPO_NAME are configured:

### new project (push to main)
```bash
git remote add origin https://${GIT_TOKEN}@${GIT_ORG_URL#https://}/${GIT_REPO_NAME}.git
git push -u origin main
```

### existing repo (push feature branch)
```bash
git remote add origin https://${GIT_TOKEN}@${GIT_ORG_URL#https://}/${GIT_REPO_NAME}.git
git push -u origin feat/your-feature-name
```

## rules
- NEVER commit secrets, api keys, or .env files
- NEVER push directly to main on existing repos with branch protection
- always test before committing
- keep commits atomic (one logical change per commit)
- use conventional commit format consistently

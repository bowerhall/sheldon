# git workflow for project development

## initialization
- run `git init` at the start of any new project
- configure git if env vars are set:
  ```bash
  git config user.name "$GIT_USER_NAME"
  git config user.email "$GIT_USER_EMAIL"
  ```
- add appropriate .gitignore for the language/framework

## commit strategy (IMPORTANT)
commit after EACH of these milestones:
1. initial project structure (files, folders, config)
2. each feature or component completed and working
3. each bug fix
4. tests added/updated
5. documentation updates
6. deployment files (dockerfile, k8s manifests)

## commit message format
use conventional commits:
- `feat: add weather api client`
- `fix: handle null response from api`
- `test: add unit tests for weather service`
- `docs: add readme with setup instructions`
- `chore: add dockerfile and k8s manifests`

## workflow example
```bash
# after creating initial structure
git add .
git commit -m "feat: initial project structure"

# after implementing a feature
git add src/weather.py
git commit -m "feat: add weather api integration"

# after adding tests
git add tests/
git commit -m "test: add weather service tests"

# after fixing a bug
git add src/weather.py
git commit -m "fix: handle api timeout gracefully"

# after adding deployment files
git add Dockerfile k8s/
git commit -m "chore: add dockerfile and k8s manifests"
```

## pushing to remote
if GIT_TOKEN, GIT_ORG_URL, and GIT_REPO_NAME are configured:
```bash
# set up remote with token auth (note: use @ before github.com, token goes in URL)
git remote add origin https://${GIT_TOKEN}@${GIT_ORG_URL#https://}/${GIT_REPO_NAME}.git

# push all commits
git push -u origin main
```

alternatively, if GIT_ORG_URL is "https://github.com/myorg":
```bash
git remote add origin https://${GIT_TOKEN}@github.com/myorg/${GIT_REPO_NAME}.git
git push -u origin main
```

## rules
- never commit secrets, api keys, or .env files
- always test before committing
- keep commits atomic (one logical change per commit)
- write clear commit messages that explain WHY, not just WHAT

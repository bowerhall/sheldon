# general coding guidelines

## git workflow (IMPORTANT)
- initialize git repo at start: `git init`
- configure git user if GIT_USER_NAME and GIT_USER_EMAIL are set
- commit frequently - after each logical unit of work is complete and tested
- write clear commit messages describing what changed and why
- test before committing - don't commit broken code
- commit structure example:
  - "feat: add basic http server with health endpoint"
  - "feat: add weather api integration"
  - "test: add unit tests for weather service"
  - "feat: add dockerfile and k8s manifests"
  - "fix: handle api timeout errors gracefully"
- push to remote if GIT_REMOTE_URL is set
- never commit secrets, api keys, or .env files

## always
- handle errors, don't ignore them
- no hardcoded secrets or api keys
- use environment variables for configuration
- add .gitignore for language-specific ignores
- keep code minimal and focused on the task

## never
- don't add features not requested
- don't over-engineer simple tasks
- don't add unnecessary dependencies

## file structure
- for simple tasks: single file is fine
- for multi-file: use standard language conventions
- always include dockerfile if deployment is mentioned
- always include k8s manifests if deployment is mentioned

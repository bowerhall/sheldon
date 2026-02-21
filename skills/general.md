# general coding guidelines

## git workflow (IMPORTANT)
- initialize git repo at start: `git init`
- configure git user if GIT_USER_NAME and GIT_USER_EMAIL are set
- for NEW projects: work on main branch
- for EXISTING repos: create feature branch (feat/, fix/, chore/, etc.)
- use conventional commits: `feat:`, `fix:`, `test:`, `chore:`, `docs:`
- commit frequently - after each logical unit of work
- test before committing - don't commit broken code
- push feature branches, not main (for existing repos)
- never commit secrets, api keys, or .env files

## conventional commit examples
- `feat: add user authentication`
- `fix: handle null response from api`
- `test: add unit tests for payment service`
- `chore: add dockerfile and compose config`
- `docs: add api documentation`

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
- don't push to main on existing repos (use feature branches)

## file structure
- for simple tasks: single file is fine
- for multi-file: use standard language conventions
- always include dockerfile if deployment is mentioned
- always include docker-compose.yml if deployment is mentioned

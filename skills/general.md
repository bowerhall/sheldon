# general coding guidelines

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

#!/bin/bash
# Coder Sandbox Entrypoint
# Runs Claude Code CLI via Ollama (local or cloud models)

set -e

# Configure Claude Code to use Ollama as backend
export ANTHROPIC_AUTH_TOKEN=ollama
export ANTHROPIC_API_KEY=""
export ANTHROPIC_BASE_URL="${OLLAMA_HOST:-http://ollama:11434}"

# Model defaults to qwen3-coder if not specified
MODEL="${CODER_MODEL:-qwen3-coder}"

# Run claude with the model and provided arguments
exec claude --model "$MODEL" "$@"

#!/bin/bash
# Coder Sandbox Entrypoint
# Runs Claude Code CLI with configurable LLM backend

set -e

# Determine provider and configure accordingly
if [ -n "$KIMI_API_KEY" ]; then
    # Kimi/Moonshot - use their Anthropic-compatible endpoint
    export ANTHROPIC_BASE_URL="https://api.moonshot.ai/anthropic"
    export ANTHROPIC_API_KEY="$KIMI_API_KEY"
    MODEL="${CODER_MODEL:-kimi-k2.5}"
elif [ -n "$ANTHROPIC_API_KEY" ]; then
    # Native Anthropic - ANTHROPIC_API_KEY already set
    MODEL="${CODER_MODEL:-claude-sonnet-4-20250514}"
elif [ -n "$OPENAI_API_KEY" ]; then
    # OpenAI-compatible (via Ollama proxy or direct)
    export ANTHROPIC_BASE_URL="${OLLAMA_HOST:-http://ollama:11434}"
    export ANTHROPIC_API_KEY="ollama"
    MODEL="${CODER_MODEL:-gpt-4o}"
else
    # Fallback to local Ollama
    export ANTHROPIC_BASE_URL="${OLLAMA_HOST:-http://ollama:11434}"
    export ANTHROPIC_API_KEY="ollama"
    MODEL="${CODER_MODEL:-qwen3-coder}"
fi

# Run claude with the model and provided arguments
exec claude --model "$MODEL" "$@"

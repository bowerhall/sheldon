#!/bin/bash
# Coder Sandbox Entrypoint
# Runs ollama's claude CLI with the provided arguments

set -e

# Determine which API key to use
if [ -n "$NVIDIA_API_KEY" ]; then
    export OLLAMA_API_KEY="$NVIDIA_API_KEY"
elif [ -n "$KIMI_API_KEY" ]; then
    export OLLAMA_API_KEY="$KIMI_API_KEY"
fi

# Model defaults to kimi-k2.5:cloud if not specified
MODEL="${CODER_MODEL:-kimi-k2.5:cloud}"

# Run ollama claude with provided arguments
exec ollama launch claude --model "$MODEL" "$@"

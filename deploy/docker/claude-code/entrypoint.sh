#!/bin/bash
set -e

# Start Ollama in background
ollama serve &
OLLAMA_PID=$!

# Wait for Ollama to be ready
echo "Waiting for Ollama to start..."
for i in {1..30}; do
    if curl -s http://localhost:11434/api/tags > /dev/null 2>&1; then
        echo "Ollama is ready"
        break
    fi
    sleep 1
done

# Configure model - prefer NVIDIA NIM, fallback to local
MODEL="${CODER_MODEL:-kimi-k2.5}"

# If NVIDIA_API_KEY is set, use cloud model
if [ -n "$NVIDIA_API_KEY" ]; then
    echo "Using NVIDIA NIM cloud model: ${MODEL}:cloud"
    MODEL="${MODEL}:cloud"
fi

# Launch Claude Code with the configured model
echo "Launching Claude Code with model: $MODEL"
exec ollama launch claude --model "$MODEL" -- "$@"

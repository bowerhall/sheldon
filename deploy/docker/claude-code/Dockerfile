# Ephemeral code runner container using Ollama + Claude Code
# Uses NVIDIA NIM (kimi-k2.5) or fallback model for code generation
FROM ubuntu:22.04

# Install dependencies
RUN apt-get update && apt-get install -y \
    curl \
    git \
    jq \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# Install Ollama
RUN curl -fsSL https://ollama.com/install.sh | sh

# Create non-root user
RUN useradd -m -u 1001 -s /bin/bash coder

# Working directory for artifacts
WORKDIR /workspace
RUN chown coder:coder /workspace

# Copy entrypoint script
COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

USER coder

# Ollama will be started by entrypoint
ENV OLLAMA_HOST=0.0.0.0

ENTRYPOINT ["/entrypoint.sh"]

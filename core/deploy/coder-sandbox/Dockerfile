# Coder Sandbox - Ephemeral code generation container
# Uses Ollama CLI to proxy to Kimi/NVIDIA NIM for code generation
#
# This container is spawned on-demand by Sheldon when CODER_ISOLATED=true
# and automatically discarded after the job completes.

FROM ubuntu:22.04

ENV DEBIAN_FRONTEND=noninteractive

# Install dependencies
RUN apt-get update && apt-get install -y \
    curl \
    git \
    ca-certificates \
    build-essential \
    python3 \
    python3-pip \
    nodejs \
    npm \
    zstd \
    && rm -rf /var/lib/apt/lists/*

# Install Ollama from GitHub release (install script doesn't work in Docker)
RUN curl -fsSL https://github.com/ollama/ollama/releases/download/v0.16.3/ollama-linux-amd64.tar.zst -o /tmp/ollama.tar.zst && \
    tar --zstd -xf /tmp/ollama.tar.zst -C /usr/local && \
    rm /tmp/ollama.tar.zst

# Create non-root user for security
RUN useradd -m -s /bin/bash coder && \
    mkdir -p /workspace && \
    chown -R coder:coder /workspace

COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

USER coder
WORKDIR /workspace

ENTRYPOINT ["/entrypoint.sh"]

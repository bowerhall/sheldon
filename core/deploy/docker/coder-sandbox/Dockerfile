# Coder Sandbox - Ephemeral code generation container
# Uses Ollama's claude CLI to proxy to Kimi/NVIDIA NIM for code generation
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
    && rm -rf /var/lib/apt/lists/*

# Install Ollama
RUN curl -fsSL https://ollama.com/install.sh | sh

# Create non-root user for security
RUN useradd -m -s /bin/bash coder && \
    mkdir -p /workspace && \
    chown -R coder:coder /workspace

COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

USER coder
WORKDIR /workspace

ENTRYPOINT ["/entrypoint.sh"]

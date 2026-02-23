# Coder Sandbox - Ephemeral code generation container
# Uses Claude Code CLI for code generation with various LLM backends
#
# This container is spawned on-demand by Sheldon when CODER_ISOLATED=true
# and automatically discarded after the job completes.

FROM ubuntu:22.04

ENV DEBIAN_FRONTEND=noninteractive

# Install dependencies + Node.js 20.x for Claude Code
RUN apt-get update && apt-get install -y \
    curl \
    git \
    ca-certificates \
    build-essential \
    python3 \
    python3-pip \
    && curl -fsSL https://deb.nodesource.com/setup_20.x | bash - \
    && apt-get install -y nodejs \
    && rm -rf /var/lib/apt/lists/*

# Install Claude Code CLI globally
RUN npm install -g @anthropic-ai/claude-code

# Create non-root user for security
RUN useradd -m -s /bin/bash coder && \
    mkdir -p /workspace && \
    chown -R coder:coder /workspace

COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

USER coder
WORKDIR /workspace

ENTRYPOINT ["/entrypoint.sh"]

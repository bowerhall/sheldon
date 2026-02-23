# Coder Sandbox - Ephemeral code generation container
# Uses Claude Code CLI for autonomous coding tasks
#
# This container is spawned on-demand by Sheldon when CODER_ISOLATED=true
# and automatically discarded after the job completes.

FROM node:20-slim

ENV DEBIAN_FRONTEND=noninteractive

# Install system dependencies
RUN apt-get update && apt-get install -y \
    curl \
    git \
    ca-certificates \
    build-essential \
    python3 \
    python3-pip \
    && rm -rf /var/lib/apt/lists/*

# Install Claude Code CLI
RUN npm install -g @anthropic-ai/claude-code

# Create non-root user for security
RUN useradd -m -s /bin/bash coder && \
    mkdir -p /workspace && \
    chown -R coder:coder /workspace

USER coder
WORKDIR /workspace

ENTRYPOINT ["claude"]

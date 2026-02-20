# Ephemeral Claude Code runner container
# Used by Kora for isolated code execution tasks
FROM node:20-alpine

# Install claude-code CLI and basic tools
RUN npm install -g @anthropic-ai/claude-code && \
    apk add --no-cache git curl jq

# Create non-root user (use node's existing group 1000)
# node:20-alpine already has node user/group at 1000
RUN adduser -u 1001 -G node -h /home/coder -D coder

# Working directory for artifacts
WORKDIR /workspace
RUN chown coder:node /workspace

USER coder

# Entrypoint runs claude with provided arguments
ENTRYPOINT ["claude"]

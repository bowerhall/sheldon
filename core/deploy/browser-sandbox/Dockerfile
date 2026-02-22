# Browser Sandbox - Isolated agent-browser container
# Provides headless browser automation for Sheldon without direct shell access
#
# This container is spawned on-demand when browser automation is needed
# and automatically discarded after the session completes.

FROM node:20-slim

ENV DEBIAN_FRONTEND=noninteractive

# Install Chromium and dependencies
RUN apt-get update && apt-get install -y \
    chromium \
    fonts-liberation \
    libasound2 \
    libatk-bridge2.0-0 \
    libatk1.0-0 \
    libatspi2.0-0 \
    libcups2 \
    libdbus-1-3 \
    libdrm2 \
    libgbm1 \
    libgtk-3-0 \
    libnspr4 \
    libnss3 \
    libxcomposite1 \
    libxdamage1 \
    libxfixes3 \
    libxkbcommon0 \
    libxrandr2 \
    xdg-utils \
    && rm -rf /var/lib/apt/lists/*

# Set Chromium path for agent-browser
ENV CHROME_PATH=/usr/bin/chromium
ENV PUPPETEER_SKIP_CHROMIUM_DOWNLOAD=true
ENV PUPPETEER_EXECUTABLE_PATH=/usr/bin/chromium

# Install agent-browser globally
RUN npm install -g agent-browser

# Create non-root user for security
RUN useradd -m -s /bin/bash browser && \
    mkdir -p /workspace && \
    chown -R browser:browser /workspace

USER browser
WORKDIR /workspace

# Default to shell so we can run agent-browser commands
ENTRYPOINT ["/bin/sh"]

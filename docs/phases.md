# Project Phases

## Phase 1: Core Platform (Complete)

Everything needed for a daily-usable personal assistant:

- **Telegram bot** with natural language interface
- **sheldonmem** — SQLite + sqlite-vec for entities, facts, relationships, vectors
- **14 life domains** — structured memory organization
- **Coder** — isolated code generation in ephemeral Docker containers
- **Deployer** — deploy apps via Docker Compose + Traefik
- **Skills system** — markdown-based skill injection
- **Cron/reminders** — scheduled notifications with memory context
- **Browser tools** — web search and page fetching
- **Git integration** — push code to GitHub repos
- **Ollama sidecar** — local embeddings + fact extraction (zero API cost)
- **VPS deployment** — one-click deploy via GitHub Actions + Doppler

**Infrastructure:** Hetzner CX33 (~€8/mo)

## Phase 2: Voice + Mac App (Planned)

Native voice interface bypassing Telegram:

- **Mac menu bar app** — SwiftUI, keyboard shortcut activation
- **Push-to-talk voice** — local STT via whisper.cpp
- **TTS responses** — Piper on VPS
- **WebSocket connection** — real-time communication with Sheldon

**Architecture:**
```
┌─────────────────────┐         ┌─────────────────────┐
│     Mac App         │         │        VPS          │
│                     │  WSS    │                     │
│  whisper.cpp (STT)  │◄───────►│  Sheldon            │
│  Audio playback     │         │  Piper (TTS)        │
└─────────────────────┘         └─────────────────────┘
```

## Future Ideas

- Mobile apps (iOS/Android)
- Generative UI components
- Multi-user support
- Distributed/HA setup

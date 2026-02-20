# Session: 2026-02-20 - Infrastructure & Security

## Completed

### Skills System
- Implemented skill management tools: `install_skill`, `save_skill`, `list_skills`, `read_skill`, `remove_skill`, `use_skill`
- Skills stored at `/data/skills` (persisted on PVC)
- Command-based activation (`/skillname`) and programmatic activation via `use_skill` tool
- Skills survive container/VPS restarts

### Memory Improvements
- Added `depth` parameter to `recall_memory` tool
- Agent can request deeper graph traversal (1-3 levels) based on context
- `RecallWithOptions` added to koramem

### Code Tools Polish
- `cleanup_workspaces` tool for removing old workspaces
- Streaming progress via `ExecuteWithProgress` - real-time notifications during code tasks

### Infrastructure
- **Network Policies**: Default deny, scoped egress/ingress per namespace
- **MinIO**: Object storage for backups (10GB PVC)
- **Automated Backups**: Daily CronJob copies kora.db to MinIO
- **Resource Overlays**: minimal (4GB), lite (8GB), full (16GB) configurations

### Network Security Architecture
```
Kora → apps:     ALLOWED (can call deployed services)
Apps → Kora:     BLOCKED (can't reach back to core)
Apps → Internet: ALLOWED (external API calls)
Kora → Internet: ALLOWED (Telegram, LLM APIs)
Inbound:         NONE (Telegram uses polling)
```

### Web Browsing Tools
- `fetch_url` - Fetch and extract content from URLs (text, links, or metadata)
- `search_web` - Web search via DuckDuckGo Lite
- HTML to text conversion, entity decoding, content truncation
- File: `core/internal/tools/browser.go`

### Ephemeral Claude Code Jobs
- k8s Jobs for isolated code execution
- Separate API key (CODER_API_KEY)
- Artifacts via shared PVC
- Auto-cleanup via ttlSecondsAfterFinished
- Files: `job_runner.go`, `bridge.go`, `coder-artifacts.yaml`

## Decisions Made

### Voice Architecture
- **STT**: Whisper small (500MB) bundled in Mac app via whisper.cpp
- **TTS**: Piper hosted on k8s cluster
- Mac native APIs rejected due to accent accuracy concerns
- Documented in `docs/voice-architecture.md`

### Claude Code Security (IMPLEMENTED)
Ephemeral k8s Jobs for isolated code execution.

Architecture:
```
User request
    ↓
Kora spawns ephemeral k8s Job
    ↓
Job runs Claude Code (isolated container, CODER_API_KEY)
    ↓
Artifacts written to shared PVC (kora-coder-artifacts)
    ↓
Job auto-deleted (ttlSecondsAfterFinished: 60)
    ↓
Kora copies artifacts locally for build/deploy
```

Files created/modified:
- `deploy/docker/claude-code/Dockerfile` - Container image for Claude Code
- `core/internal/coder/job_runner.go` - k8s Job lifecycle management
- `core/internal/coder/bridge.go` - Dual-mode: subprocess (local) or Jobs (k8s)
- `deploy/k8s/base/coder-artifacts.yaml` - 5GB PVC for artifacts
- `deploy/k8s/base/rbac.yaml` - Added batch/jobs permissions
- `deploy/k8s/base/config.yaml` - CODER_USE_K8S_JOBS, CODER_K8S_IMAGE
- `core/internal/config/` - New config fields for Job mode

Configuration:
```yaml
CODER_USE_K8S_JOBS: "true"      # enable ephemeral Jobs
CODER_K8S_IMAGE: "kora-claude-code:latest"
CODER_ARTIFACTS_PVC: "kora-coder-artifacts"
```

Benefits:
- Complete filesystem isolation from Kora
- Separate API key (CODER_API_KEY) - rotatable independently
- No access to kora.db or Kora secrets
- Clean environment per task
- Automatic cleanup via ttlSecondsAfterFinished

### Skill Security Concerns
Malicious skills can instruct Kora to:
- Dump all memories via `recall_memory`
- Exfiltrate data via `write_code`
- Deploy malicious apps

Mitigations (to implement):
- Skill content preview before install
- Claude Code container isolation (above)
- Audit logging for skill activations

### Web Browsing (IMPLEMENTED)
Added browser tools for web content access:

Tools:
- `fetch_url` - Fetch and extract content from URLs
  - `extract: "text"` - Readable text content (default)
  - `extract: "links"` - All links on page
  - `extract: "meta"` - Page metadata (title, description, etc.)
- `search_web` - Web search via DuckDuckGo Lite

Features:
- HTML to text conversion (removes scripts, styles, tags)
- Common HTML entity decoding
- Content truncation (10k chars max for LLM context)
- Link extraction with relative URL resolution
- Redirect following (up to 5 hops)
- 30s timeout per request
- 5MB body size limit

File: `core/internal/tools/browser.go`

## Next Steps
1. ~~Implement ephemeral Claude Code containers (k8s Jobs)~~ DONE
2. ~~Implement web browsing tools~~ DONE
3. Build and push kora-claude-code container image
4. Deploy to Hetzner VPS
5. Test full pipeline: skill install → code → build → deploy

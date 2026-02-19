# Security Architecture

## Four-Layer Defense

### Layer 1: Kubernetes RBAC
- **kora-core**: can read/write own namespace, cannot modify RBAC, cannot access kora-storage k8s API
- **kora-deployer**: can create Deployments/Services in kora-services only
- **kora-observer**: read-only access for monitoring

### Layer 2: Network Policies
- kora-system: outbound internet + inbound from kora-voice only
- kora-storage: ingress only from kora-system
- kora-services: egress internet only, no cluster-internal access
- kora-voice: ingress from kora-system, egress to internet (API calls)

### Layer 3: Container Sandboxing
- PodSecurity: "restricted" profile on kora-services (non-root, no privilege escalation)
- ResourceQuota on kora-services: max 10 pods, 4 CPU, 4Gi memory
- kora-system: "baseline" profile (needs some capabilities)

### Layer 4: Application Security
- Claude Code Bridge: full sandboxing spec in [claude-code-bridge.md](claude-code-bridge.md)
  - Isolated workspace per task (`/workspace/sandbox/<task-id>/`)
  - Environment stripped and rebuilt from empty (not inherited from Kora process)
  - Restricted tool set: Read, Write, Edit, Bash, Grep, Glob — no WebSearch, WebFetch, or MCP
  - Complexity-tiered timeouts (5min–45min based on task scope)
- Output sanitizer: regex scan for credentials before logging/displaying
- Prompt injection defense: scraped content framed with injection warnings
- Skill autonomy levels: confirm (default) or autonomous (opt-in)

## koramem Security

- SQLite file permissions: 0600, owned by kora process user
- WAL mode: concurrent reads, single writer (no corruption risk)
- No network exposure: koramem is in-process, no HTTP API
- Backup encryption: SQLite snapshot encrypted before MinIO upload
- Fact deletion: `/review` command allows user to delete any stored fact
- No PII in logs: extracted facts logged with domain ID only, not values

## Credential Management

- k8s Secrets for: Claude API key, Telegram bot token, Voyage API key, MinIO credentials
- Never in env vars directly — mounted as files or injected via downward API
- Claude Code Bridge sandbox env built from empty — only PATH, LANG, TERM, and a dedicated API key (see [claude-code-bridge.md](claude-code-bridge.md#environment-stripping))
- Output sanitizer patterns: `sk-ant-*`, `bot*:*`, `AKIA*`, password-like strings

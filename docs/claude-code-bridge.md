# Claude Code Bridge

> How Kora invokes Claude Code for code generation, multi-file builds, and autonomous execution tasks.

**Phase: 2 (Action Skills)**
**Depends on: Phase 0 (core), Phase 1 (tools)**

## Why a Bridge, Not Direct API

Kora's outer agent loop (PicoClaw) handles conversation — routing, memory, context assembly. But code-heavy tasks need a different kind of agent: one that can read files, write code, run tests, see errors, fix them, and iterate. That's Claude Code.

The bridge connects two nested agent loops:

```
Kora Agent Loop (PicoClaw / Sonnet|Opus)
  │
  ├── receives Telegram message
  ├── routes (Haiku), recalls memory (koramem), builds context
  ├── decides: "this needs code execution"
  ├── calls claude_code tool
  │     │
  │     ▼
  │   ┌─────────────────────────────────────────────────┐
  │   │  Claude Code Agent Loop (separate subprocess)   │
  │   │                                                 │
  │   │  reads files → writes code → runs it →           │
  │   │  sees errors → fixes → runs again →              │
  │   │  iterates until done or max_turns reached       │
  │   │                                                 │
  │   │  Tools: Read, Write, Edit, Bash, Grep, Glob     │
  │   │  Model: inherited from Kora's routing decision  │
  │   └─────────────────────────────────────────────────┘
  │     │
  │     ▼
  ├── receives structured result (files created, stdout, errors)
  ├── sanitizes output (credential scan)
  ├── decides next action (deploy? present? iterate?)
  └── stores memory (koramem.Remember)
```

Key distinction: Kora's loop is **conversational** (user intent → memory → response). Claude Code's loop is **operational** (task → code → test → fix → done). They have different context windows, different tool sets, and potentially different models.

## Invocation Method: Go Agent SDK

Rather than raw `os/exec`, use a Go Agent SDK that wraps the Claude Code CLI subprocess with structured streaming, permission control, and session management.

**Dependency**: Claude Code CLI installed in the container (`npm install -g @anthropic-ai/claude-code`).

**SDK selection**: Evaluate community Go SDKs (multiple exist with MIT licenses). Key requirements:
- Subprocess transport via CLI's `--output-format stream-json`
- Channel-based message streaming (idiomatic Go)
- Permission callbacks (for sandbox enforcement)
- MaxTurns control
- System prompt and allowed tools configuration
- Working directory (`Cwd`) isolation

If no community SDK meets stability requirements, fall back to raw subprocess with `--output-format json` and parse stdout. The CLI interface is stable and well-documented.

### Basic Invocation Pattern

```go
// Inside the claude_code tool handler
func (t *ClaudeCodeTool) Execute(ctx context.Context, task TaskRequest) (*TaskResult, error) {
    // 1. Create sandboxed workspace
    workspace, err := t.sandbox.Create(task.ID)
    if err != nil {
        return nil, fmt.Errorf("sandbox create: %w", err)
    }
    defer t.sandbox.Cleanup(workspace)

    // 2. Generate CLAUDE.md with memory context
    if err := t.writeContextFile(workspace, task.MemoryContext); err != nil {
        return nil, fmt.Errorf("context file: %w", err)
    }

    // 3. Build SDK options
    opts := &claude.Options{
        Model:        task.Model,              // from Kora's router
        MaxTurns:     task.Complexity.MaxTurns, // from complexity tier
        Cwd:          workspace.Path,
        AllowedTools: t.sandbox.AllowedTools(), // Read, Write, Edit, Bash (restricted)
        SystemPrompt: task.SystemPrompt,        // minimal, context in CLAUDE.md
        Env:          t.sandbox.CleanEnv(),     // stripped of all credentials
    }

    // 4. Execute with timeout
    timeoutCtx, cancel := context.WithTimeout(ctx, task.Complexity.Timeout)
    defer cancel()

    msgChan, errChan := claude.Query(timeoutCtx, task.Prompt, opts)

    // 5. Collect results
    var result TaskResult
    for {
        select {
        case msg, ok := <-msgChan:
            if !ok {
                return &result, nil
            }
            t.processMessage(msg, &result)
        case err := <-errChan:
            if err != nil {
                return &result, fmt.Errorf("claude code: %w", err)
            }
        case <-timeoutCtx.Done():
            return &result, fmt.Errorf("timeout after %v", task.Complexity.Timeout)
        }
    }
}
```

## Context Bridging

Claude Code starts with a blank slate — it doesn't see Kora's memory, conversation history, or personality. The bridge must inject relevant context. Two mechanisms:

### 1. CLAUDE.md Generation (Primary)

Claude Code automatically reads `CLAUDE.md` files in the working directory. Kora generates one per task from koramem context:

```go
func (t *ClaudeCodeTool) writeContextFile(ws *Workspace, mem *MemoryContext) error {
    var buf bytes.Buffer

    buf.WriteString("# Project Context\n\n")
    buf.WriteString("## User Preferences\n")
    buf.WriteString(fmt.Sprintf("- Language: %s\n", mem.PreferredLanguage))  // "Go"
    buf.WriteString(fmt.Sprintf("- Style: %s\n", mem.CodeStyle))             // "minimal, well-commented"
    buf.WriteString(fmt.Sprintf("- Target: %s\n", mem.DeployTarget))         // "k3s via kora-services namespace"

    if len(mem.RelevantFacts) > 0 {
        buf.WriteString("\n## Relevant Context\n")
        for _, fact := range mem.RelevantFacts {
            buf.WriteString(fmt.Sprintf("- %s: %s\n", fact.Field, fact.Value))
        }
    }

    if mem.ExistingPatterns != "" {
        buf.WriteString("\n## Existing Patterns\n")
        buf.WriteString(mem.ExistingPatterns + "\n")
    }

    buf.WriteString("\n## Constraints\n")
    buf.WriteString("- Do not access environment variables for secrets\n")
    buf.WriteString("- All network calls must handle timeouts\n")
    buf.WriteString("- Include a Dockerfile if the output is a deployable service\n")

    return os.WriteFile(filepath.Join(ws.Path, "CLAUDE.md"), buf.Bytes(), 0644)
}
```

### 2. System Prompt (Supplementary)

Keep the `--system-prompt` minimal — it supplements CLAUDE.md, not replaces it. Long system prompts can cause issues with tool permissions in some environments.

```
"You are building a tool for Kora, a personal AI assistant running on k3s.
Output clean, production-ready code. Include error handling.
If creating a deployable service, include a Dockerfile and k8s manifest."
```

The CLAUDE.md carries the heavy context; the system prompt carries behavioral directives.

## Sandboxing

**UPDATE (2026-02-20):** Claude Code now runs in ephemeral k8s Jobs, not inside the Kora pod. This provides complete isolation.

### Ephemeral Container Architecture

```
┌─────────────┐                  ┌──────────────────────────┐
│    KORA     │    spawn Job     │   Ephemeral k8s Job      │
│             │─────────────────▶│  ┌────────────────────┐  │
│             │                  │  │ Claude Code        │  │
│             │                  │  │ - Own API key      │  │
│             │                  │  │ - Isolated FS      │  │
│             │                  │  │ - No kora.db       │  │
│             │◀─────────────────│  │ - No secrets       │  │
│             │   artifacts      │  └────────────────────┘  │
└─────────────┘   (shared PVC)   │  /output volume          │
                                 └──────────────────────────┘
                                            │
                                       auto-delete
```

### Why Ephemeral Containers

Previous approach (subprocess in Kora pod) had vulnerabilities:
- Claude Code could read `/data/kora.db`
- Claude Code could access Kora's environment variables
- Malicious skills could instruct data exfiltration

New approach benefits:
- **Complete filesystem isolation**: Job only sees its workspace + output volume
- **Separate API key**: CODER_API_KEY rotatable without affecting Kora
- **No secrets exposure**: Kora's ANTHROPIC_API_KEY, TELEGRAM_TOKEN not accessible
- **Clean slate**: Container deleted after task, no persistent compromise
- **Network isolation**: Job has own NetworkPolicy (egress to API only)

### Job Specification

```yaml
apiVersion: batch/v1
kind: Job
metadata:
  name: claude-code-<task-id>
  namespace: kora
spec:
  ttlSecondsAfterFinished: 60  # auto-delete after completion
  template:
    spec:
      restartPolicy: Never
      containers:
        - name: claude-code
          image: kora-claude-code:latest
          env:
            - name: ANTHROPIC_API_KEY
              valueFrom:
                secretKeyRef:
                  name: coder-credentials
                  key: api-key
          volumeMounts:
            - name: workspace
              mountPath: /workspace
            - name: output
              mountPath: /output
          resources:
            limits:
              memory: "2Gi"
              cpu: "1000m"
      volumes:
        - name: workspace
          emptyDir: {}
        - name: output
          persistentVolumeClaim:
            claimName: kora-coder-output
```

### Workflow

1. Kora receives code task
2. Creates Job with task context in ConfigMap
3. Job runs Claude Code, writes artifacts to /output
4. Kora polls Job status, waits for completion
5. Kora reads artifacts from shared PVC
6. Job auto-deletes (ttlSecondsAfterFinished)
7. Kora deploys artifacts to kora-apps if applicable

### Environment Stripping

The Job container starts clean:
```yaml
env:
  - name: HOME
    value: "/tmp"
  - name: ANTHROPIC_API_KEY
    valueFrom:
      secretKeyRef:
        name: coder-credentials  # NOT kora-secrets
        key: api-key
```

No access to:
- Kora's ANTHROPIC_API_KEY
- TELEGRAM_TOKEN
- LLM_API_KEY
- MINIO_CREDENTIALS
- /data/kora.db

### Network Policy

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: claude-code-job-policy
  namespace: kora
spec:
  podSelector:
    matchLabels:
      job-type: claude-code
  policyTypes:
    - Egress
  egress:
    # DNS
    - ports:
        - protocol: UDP
          port: 53
    # Anthropic API only
    - ports:
        - protocol: TCP
          port: 443
      to:
        - ipBlock:
            cidr: 0.0.0.0/0  # Allow HTTPS, API enforces endpoint
```

## Complexity Tiers

Not all tasks are equal. A one-file script needs different bounds than a full-stack app. The router and strategy engine estimate complexity, which maps to resource allocation.

| Tier | Example | MaxTurns | Timeout | Strategy |
|------|---------|----------|---------|----------|
| **simple** | "write a bash script to rename files" | 10 | 5 min | Single invocation |
| **standard** | "write a Go scraper with SQLite" | 25 | 10 min | Single invocation |
| **complex** | "build a budgeting app with frontend" | 50 | 20 min | Multi-pass (see below) |
| **project** | "build a full dashboard service" | N/A | 45 min | Orchestrated multi-pass |

### Complexity Detection

The outer agent loop (Kora) determines the tier before invoking Claude Code:

```go
type ComplexityTier struct {
    Level    string        // simple, standard, complex, project
    MaxTurns int
    Timeout  time.Duration
    Strategy string        // single, multi-pass
}

// Determined by Kora's router + strategy engine
// Signals: number of output artifacts, cross-domain memory, "app" vs "script" language
```

### Multi-Pass Orchestration

For `complex` and `project` tiers, Kora's outer loop breaks the task into subtasks and calls Claude Code multiple times. Each pass gets its own CLAUDE.md with updated context:

```
Pass 1: "Build the Go backend with SQLite schema and REST API"
  → Claude Code writes: main.go, db.go, handlers.go, go.mod
  → Kora reviews output, updates CLAUDE.md with file list

Pass 2: "Build the frontend — here are the API endpoints from Pass 1: [...]"
  → Claude Code writes: index.html, app.js, style.css
  → Same workspace, can see and reference backend files

Pass 3: "Write Dockerfile and k8s manifests for the complete service"
  → Claude Code writes: Dockerfile, deployment.yaml, service.yaml
  → Has full project context from previous passes

Pass 4: "Run tests and fix any issues"
  → Claude Code runs: go test ./..., checks Dockerfile builds
  → Fixes issues found, iterates within its own loop
```

Each pass is a separate `claude.Query()` call. Kora manages the orchestration, determines what context to carry forward, and decides when the project is complete.

### Session Continuity (Alternative)

For tasks where pass-by-pass orchestration is too rigid, use Claude Code's `--continue` flag to maintain conversation state:

```go
// First invocation captures session ID
result1 := invoke(ctx, "Build the backend", opts)

// Subsequent invocations continue the same session
opts.Continue = true  // or opts.Resume = result1.SessionID
result2 := invoke(ctx, "Now add the frontend", opts)
```

This gives Claude Code full memory of previous work within the session. Trade-off: less control over individual pass boundaries, but more natural for iterative work.

## Output Handling

### Result Parsing

Claude Code returns structured messages via the SDK. The bridge collects:

```go
type TaskResult struct {
    // What was produced
    FilesCreated  []string          // paths relative to workspace
    FilesModified []string
    Stdout        string            // captured bash output
    FinalMessage  string            // Claude Code's summary

    // Metadata
    TurnsUsed     int
    Model         string
    Duration      time.Duration
    SessionID     string            // for potential --continue

    // Errors (non-fatal — Claude Code may have recovered)
    Warnings      []string
}

func (t *ClaudeCodeTool) processMessage(msg claude.Message, result *TaskResult) {
    switch m := msg.(type) {
    case *claude.AssistantMessage:
        for _, block := range m.Content {
            if text, ok := block.(*claude.TextBlock); ok {
                result.FinalMessage = text.Text
            }
        }
    case *claude.ResultMessage:
        result.TurnsUsed = m.TurnsUsed
        result.Duration = m.Duration
        result.SessionID = m.SessionID
    }
}
```

### Output Sanitization

Runs after every Claude Code invocation, before results reach the user or get stored:

```go
var sensitivePatterns = []*regexp.Regexp{
    regexp.MustCompile(`sk-ant-[a-zA-Z0-9\-_]{20,}`),    // Anthropic API keys
    regexp.MustCompile(`bot\d+:[a-zA-Z0-9_-]{35}`),       // Telegram bot tokens
    regexp.MustCompile(`AKIA[0-9A-Z]{16}`),                // AWS access keys
    regexp.MustCompile(`(?i)password\s*[:=]\s*\S+`),       // password assignments
    regexp.MustCompile(`ghp_[a-zA-Z0-9]{36}`),             // GitHub PATs
    regexp.MustCompile(`voyage-[a-zA-Z0-9]{20,}`),         // Voyage API keys
}

func Sanitize(output string) (string, []string) {
    var warnings []string
    sanitized := output
    for _, pat := range sensitivePatterns {
        if pat.MatchString(sanitized) {
            warnings = append(warnings, fmt.Sprintf("redacted: %s pattern", pat.String()[:20]))
            sanitized = pat.ReplaceAllString(sanitized, "[REDACTED]")
        }
    }
    return sanitized, warnings
}
```

If any pattern matches, the result is still returned but with redacted values. Warnings are logged and surfaced to Kora's outer loop, which can flag the issue to the user.

## Integration with Phase 6 (Self-Extension)

When Claude Code produces deployable artifacts (Dockerfile + k8s manifests), Kora's outer loop can chain into the Phase 6 self-extension pipeline:

```
Claude Code output → Kora inspects files → finds Dockerfile + deployment.yaml
  → Kora checks autonomy level (koramem: "confirm for deployments")
  → Asks user: "Want me to deploy this?"
  → On confirmation: hands off to Container Builder (kaniko) + Deployment Tool
  → Service goes live in kora-services namespace
```

The bridge doesn't handle deployment directly — it produces artifacts. Phase 6 tools consume those artifacts. Clean separation of concerns.

## Tool Registration

The `claude_code` tool registers in PicoClaw's ToolRegistry like any other tool:

```go
type ClaudeCodeTool struct {
    sandbox    *Sandbox
    sanitizer  *OutputSanitizer
    sdkConfig  *claude.Options   // base config, overridden per task
}

func (t *ClaudeCodeTool) Name() string { return "claude_code" }

func (t *ClaudeCodeTool) Description() string {
    return "Execute code generation and multi-file development tasks " +
           "using Claude Code agent. Handles writing, testing, and " +
           "iterating on code in a sandboxed environment."
}

func (t *ClaudeCodeTool) Schema() map[string]interface{} {
    return map[string]interface{}{
        "type": "object",
        "properties": map[string]interface{}{
            "task": map[string]interface{}{
                "type":        "string",
                "description": "Natural language description of the code task",
            },
            "complexity": map[string]interface{}{
                "type":        "string",
                "enum":        []string{"simple", "standard", "complex", "project"},
                "description": "Estimated complexity tier for resource allocation",
            },
        },
        "required": []string{"task"},
    }
}
```

The outer agent (Sonnet/Opus) decides when to call this tool based on the conversation. It provides the task description and optionally the complexity hint. The bridge handles everything else.

## Container Requirements

Claude Code CLI requires Node.js. This adds to the Kora container:

```dockerfile
# In Kora's Dockerfile
FROM golang:1.23-bookworm AS builder
# ... build Kora binary ...

FROM debian:bookworm-slim
# Node.js for Claude Code CLI
RUN curl -fsSL https://deb.nodesource.com/setup_22.x | bash - && \
    apt-get install -y nodejs && \
    npm install -g @anthropic-ai/claude-code && \
    apt-get clean && rm -rf /var/lib/apt/lists/*

COPY --from=builder /app/kora /usr/local/bin/kora
# ... rest of Kora setup ...
```

This adds ~150MB to the container image. Trade-off is acceptable given Claude Code's capabilities. The alternative (reimplementing Claude Code's agent loop in Go via raw API calls) would be significantly more work and miss out on Claude Code's built-in tools, file checkpointing, and iterative fixing.

## Error Handling

| Error | Source | Handling |
|-------|--------|----------|
| CLI not found | SDK/subprocess | Fatal on startup — container misconfigured |
| API key invalid | Claude Code subprocess | Return error to outer loop, surface to user |
| Timeout | Context cancellation | Return partial results + warning |
| Max turns exceeded | SDK | Return whatever was produced, Kora decides to retry or accept |
| Credential leak detected | Output sanitizer | Redact, warn user, log for review |
| Disk space exhaustion | Workspace | Cleanup oldest sandboxes, retry |
| Subprocess crash | SDK transport | Return error, Kora retries once with same prompt |

## Resolved Questions

1. **API key sharing**: ✅ RESOLVED - Separate key (CODER_API_KEY). Stored in `coder-credentials` secret, rotatable independently. Better security isolation.

2. **Streaming to user**: ✅ RESOLVED - Implemented via `ExecuteWithProgress`. Sends notifications: "🔨 Working on...", "💭 Thinking...", "🔧 Using: <tool>", "✅ Complete".

3. **Go SDK choice**: ✅ RESOLVED - Using Claude Code CLI directly via subprocess with `--output-format stream-json`. No external SDK dependency.

4. **Model passthrough**: ✅ RESOLVED - Claude Code uses its own default model. Kora doesn't override.

## Open Questions

1. **Job startup latency**: Ephemeral containers add ~3-5 sec startup. Acceptable for code tasks, but monitor if it impacts UX.

2. **Artifact size limits**: What's the max artifact size to copy from Job output volume? Need to prevent disk exhaustion.

3. **Concurrent jobs**: Should we limit concurrent Claude Code jobs? Risk of resource exhaustion with multiple parallel requests.

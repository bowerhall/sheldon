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

Every Claude Code invocation runs in an isolated workspace. The sandbox enforces security boundaries that prevent the code agent from accessing Kora's own secrets or infrastructure.

### Workspace Isolation

```go
type Sandbox struct {
    BaseDir     string // /workspace/sandbox/
    MaxDiskMB   int    // 500MB per task
}

type Workspace struct {
    Path    string // /workspace/sandbox/<task-id>/
    TaskID  string
    Created time.Time
}

func (s *Sandbox) Create(taskID string) (*Workspace, error) {
    path := filepath.Join(s.BaseDir, taskID)
    if err := os.MkdirAll(path, 0755); err != nil {
        return nil, err
    }
    return &Workspace{Path: path, TaskID: taskID, Created: time.Now()}, nil
}

func (s *Sandbox) Cleanup(ws *Workspace) {
    // Copy artifacts to persistent storage first if needed
    os.RemoveAll(ws.Path)
}
```

### Environment Stripping

```go
func (s *Sandbox) CleanEnv() map[string]string {
    // Start from empty — NOT os.Environ()
    return map[string]string{
        "HOME":     "/tmp",
        "PATH":     "/usr/local/bin:/usr/bin:/bin",
        "LANG":     "en_US.UTF-8",
        "TERM":     "dumb",
        // Claude Code needs its own API key — mounted separately
        // NOT Kora's ANTHROPIC_API_KEY
        "ANTHROPIC_API_KEY": s.codeApiKey,
    }
}
```

### Tool Restrictions

```go
func (s *Sandbox) AllowedTools() []string {
    return []string{
        "Read",
        "Write",
        "Edit",
        "Bash",     // restricted to workspace dir via CLAUDE.md instruction
        "Grep",
        "Glob",
        // NOT: WebSearch, WebFetch (no network except API)
        // NOT: any MCP servers (no access to Kora's internal tools)
    }
}
```

### Network Enforcement

In k3s, the sandbox runs within Kora's pod but with restricted capabilities. Network access for the Claude Code subprocess is limited to the Anthropic API endpoint. This is enforced at the Kubernetes NetworkPolicy level (kora-system namespace allows outbound HTTPS) — the subprocess inherits this. If tighter isolation is needed, use a sidecar container with its own restrictive NetworkPolicy.

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

## Open Questions

1. **API key sharing**: Should Claude Code use the same API key as Kora's main agent, or a separate key with its own rate limits? Separate key gives better cost tracking but adds credential management complexity.

2. **Streaming to user**: Should Kora stream Claude Code's progress to the user in real-time via Telegram ("writing backend... running tests... fixing error..."), or wait for completion? Streaming is better UX for long tasks but adds complexity.

3. **Go SDK choice**: Multiple community Go SDKs exist (unofficial ports of the Python SDK). Need to evaluate for stability, maintenance activity, and feature completeness before committing. Alternatively, PicoClaw may already have a TypeScript-based bridge we can adapt.

4. **Model passthrough**: Should the model be set by Kora's router (Sonnet for code, Opus for complex architecture) or let Claude Code use its own default? Router control is better for cost management but may miss cases where Claude Code benefits from model escalation mid-task.

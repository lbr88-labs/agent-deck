# Coding Conventions

**Analysis Date:** 2026-03-11

## Naming Patterns

**Files:**
- Lowercase with underscores: `instance.go`, `session_cmd.go`, `claude_hooks_test.go`
- Test files end in `_test.go`
- Package-level test utilities in `testmain_test.go`

**Functions:**
- PascalCase for exported functions: `NewInstance()`, `Start()`, `GetStatusThreadSafe()`
- camelCase for private functions: `handleSession()`, `printSessionHelp()`
- Descriptive handler names: `handleSessionStart()`, `printUpdateNotice()`

**Variables:**
- camelCase for local variables and package-level: `inst`, `tmuxSess`, `userCfg`
- SCREAMING_SNAKE_CASE for constants: `StatusRunning`, `tableColTitle`
- Logger instances use lowercase descriptive names: `sessionLog`, `uiLog`, `mcpLog`

**Types:**
- PascalCase for exported types: `Instance`, `Status`, `SandboxConfig`
- Private types in lowercase with underscore: `type claudeMessage struct` (lowercase to start)

**Interfaces and Methods:**
- Export methods with full documentation: `GetStatusThreadSafe()`, `SetStatusThreadSafe()`
- Thread-safe accessors explicitly named: methods ending in `ThreadSafe`, methods with `GetStatus()` for fast-path access

## Code Style

**Formatting:**
- `go fmt` enforced via `make fmt`
- Standard Go formatting conventions
- Long lines broken at logical boundaries (visible in multi-line imports)

**Linting:**
- `golangci-lint` enforced in pre-push hook
- Run via `make lint`

**Line Length:**
- No explicit limit, but generally breaks at ~120 characters for readability
- Import blocks organized by standard library, then dependencies, then internal packages

## Import Organization

**Order:**
1. Standard library (bufio, context, flag, fmt, log/slog, os, etc.)
2. Third-party packages (github.com/charmbracelet/*, github.com/stretchr/testify, etc.)
3. Internal packages (github.com/asheshgoplani/agent-deck/internal/*)

**Example:**
```go
import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/asheshgoplani/agent-deck/internal/clipboard"
	"github.com/asheshgoplani/agent-deck/internal/git"
	"github.com/asheshgoplani/agent-deck/internal/session"
)
```

**Path Aliases:**
- No path aliases used; imports use full canonical paths
- Module: `github.com/asheshgoplani/agent-deck`

## Error Handling

**Patterns:**
- Explicit error checks: `if err != nil { return err }` or `if err != nil { t.Fatal(err) }`
- Errors wrapped with context when appropriate
- In CLI handlers, errors typically logged to `os.Stderr` via `fmt.Fprintf(os.Stderr, "Error: ...")`
- Test errors use `require.NoError()` for critical failures, `assert.Error()` for checking specific conditions

**Convention:**
- `require.NoError()`: Stop test on error (critical operation)
- `assert.Error()`: Check error exists, continue test
- Raw `if err != nil` checks in non-test code with explicit handling per context

## Logging

**Framework:** `log/slog` (structured logging)

**Patterns:**
- Component-scoped loggers initialized at package level:
```go
var (
    sessionLog = logging.ForComponent(logging.CompSession)
    mcpLog     = logging.ForComponent(logging.CompMCP)
    uiLog      = logging.ForComponent(logging.CompUI)
)
```

- Logging with structured fields:
```go
sessionLog.Info("claude_hooks_installed", slog.String("config_dir", configDir))
mcpLog.Info("regenerating_mcp_config",
    slog.String("session_id", i.ID),
    slog.String("tool", i.Tool))
```

**When to Log:**
- Session lifecycle events (start, stop, fork)
- Configuration changes and hook installations
- MCP operations (attach, detach, regenerate)
- Integration events (Claude/Gemini session detection)
- Errors and warnings (always include context)

**Levels:**
- `Info()`: Normal operations, state changes, lifecycle events
- `Error()`: Failures, exceptions, recovery attempts (always include error details)
- `Debug()`: Not commonly used in codebase (slog configured for Info level in production)

## Comments

**When to Comment:**
- Document exported types and functions with full package-level comment blocks
- Clarify non-obvious logic or edge cases
- Document known issues or workarounds (e.g., tmux-specific quirks)
- Explain why, not what (code already shows what)

**JSDoc/TSDoc:**
- Not used (Go codebase). Use standard Go comment conventions instead
- Every exported identifier should have a comment starting with the identifier name

**Example:**
```go
// Status represents the current state of a session
type Status string

// Instance represents a single agent/shell session
type Instance struct {
    ID       string    // Unique identifier for the session
    Title    string    // Display name
    Status   Status    // Current state (running, idle, etc.)
    Command  string    // Bash command to execute
}

// GetStatusThreadSafe returns the session status with read-lock protection.
// Use this when reading Status from a goroutine concurrent with backgroundStatusUpdate.
func (inst *Instance) GetStatusThreadSafe() Status {
    // ...
}
```

## Function Design

**Size:**
- Prefer functions under 50 lines; break complex operations into helpers
- Large functions (200+ lines) indicate need for refactoring
- Example: `instance.go` contains many single-purpose methods like `buildClaudeCommand()`, `buildGeminiCommand()`

**Parameters:**
- Use pointers for receiver types on methods (almost all methods use `*Instance`, `*Session`, etc.)
- Pass structs by pointer when larger than 128 bytes
- Use value receivers for small types or when immutability is desired

**Return Values:**
- Single return value for simple getters: `GetStatusThreadSafe() Status`
- Error as last return: `Start() error`, `Get() (T, error)`
- Use named return values only when semantically important; implicit returns are preferred

**Example from codebase:**
```go
func (i *Instance) IsSandboxed() bool {
    return i.Sandbox != nil && i.Sandbox.Enabled
}

func (i *Instance) Start() error {
    // implementation
}

func (i *Instance) buildClaudeCommand(baseCommand string) string {
    // implementation
}
```

## Module Design

**Exports:**
- Exported types define the public API: `Instance`, `Session`, `Status`
- Private types used internally: `claudeMessage`, `claudeRecord` (lowercase)
- Getter/setter methods for complex state: `GetStatusThreadSafe()`, `SetStatusThreadSafe()`

**Barrel Files:**
- No barrel files used
- Each package exports its primary types and functions directly
- Import organization relies on explicit full paths

**Package Structure:**
- `cmd/agent-deck/`: CLI handlers and main entry point
- `internal/session/`: Session model and lifecycle
- `internal/tmux/`: tmux integration
- `internal/ui/`: Bubble Tea TUI components
- `internal/logging/`: Structured logging
- Other `internal/*`: Supporting functionality (git, docker, web, etc.)

## Concurrency

**Mutex Usage:**
- Explicit mutex protection for shared state: `sync.RWMutex`
- Read lock (`RLock()`) for read-only access: `GetStatusThreadSafe()`
- Write lock (`Lock()`) for mutations: `SetStatusThreadSafe()`
- Pattern: Lock → Read/Modify → Unlock in deferred cleanup or explicit statements

**Goroutines:**
- Background workers explicitly documented (e.g., `backgroundStatusUpdate()`)
- Context-based cancellation where long-running
- Channels for event signaling (tmux activity pipe)

**Thread Safety:**
- All access to `Instance.Status` must use `GetStatusThreadSafe()`/`SetStatusThreadSafe()`
- Direct field access documented as non-thread-safe in comments

---

*Convention analysis: 2026-03-11*

# rho — Progress Handoff

> **Goal**: Make Go-based `rho` a 1:1 functional clone of TypeScript-based `pi`.
> **Reference repo**: `/home/whick/Coding/rho/pi/` (full TypeScript source).
> **Workspace**: `/home/whick/Coding/rho`

---

## What Is Already Done ✅

All items below compile and all existing tests pass (`go test ./...`).

| File | What was done |
|------|--------------|
| [cmd/rho/args.go](file:///home/whick/Coding/rho/cmd/rho/args.go) | Custom POSIX-compliant CLI argument parser — no `flag` package; handles `@file`, unknown extension flags, short forms |
| [cmd/rho/args_test.go](file:///home/whick/Coding/rho/cmd/rho/args_test.go) | Unit tests for the parser |
| [cmd/rho/attachments.go](file:///home/whick/Coding/rho/cmd/rho/attachments.go) | `@file` processor: text wrapping in XML, image base64 + resize, MIME detection |
| [cmd/rho/attachments_test.go](file:///home/whick/Coding/rho/cmd/rho/attachments_test.go) | Unit tests for attachment processing |
| [cmd/rho/main.go](file:///home/whick/Coding/rho/cmd/rho/main.go) | Fully rewritten: uses `ParseArgs`, detects package subcommands (`install`/`remove`/`update`/`list`/`info`/`config`) early, processes `@file` attachments, resolves mode (interactive/print/json/rpc) matching pi's `resolveAppMode` logic |
| [pkg/agent/rpc/rpc.go](file:///home/whick/Coding/rho/pkg/agent/rpc/rpc.go) | Replaced old JSON-RPC 2.0 server with flat JSON Lines protocol matching pi's `rpc-mode.ts` + `rpc-types.ts`: streaming agent events, all RPC commands, synchronous extension UI blocking via UUID-keyed channels |
| [pkg/agent/rpc/bash.go](file:///home/whick/Coding/rho/pkg/agent/rpc/bash.go) | Bash executor helper used by the RPC `bash` command |
| [pkg/agent/agent_loop.go](file:///home/whick/Coding/rho/pkg/agent/agent_loop.go) | Added `SetModel`, `SetAPIKey`, `SetSystemPrompt`, `SetThinkingLevel` methods; `buildLLMContext` constructs image content blocks from `AgentMessage.Images` |
| [pkg/agent/codecore/agent_session_runtime.go](file:///home/whick/Coding/rho/pkg/agent/codecore/agent_session_runtime.go) | Added `SendMessageStream(content, images, callback)`, `SetUIContext`, `GetModel`/`SetModel`, `GetThinkingLevel`/`SetThinkingLevel`, `GetSystemPrompt`/`SetSystemPrompt`, `GetAPIKey`/`SetAPIKey`, `GetModelRegistry` |

---

## What Remains ⬜

The high-level clone is structurally complete. The gaps below are deeper feature areas that still diverge from `pi`. Work on them in order of impact.

---

### 1. Session Manager — fork / clone / switch (HIGH IMPACT)

**pi reference**: [`pi/packages/coding-agent/src/core/session-manager.ts`](file:///home/whick/Coding/rho/pi/packages/coding-agent/src/core/session-manager.ts) (~1700 LOC)

**rho current**: [`pkg/agent/session.go`](file:///home/whick/Coding/rho/pkg/agent/session.go) — minimal (list/load/save/delete only).

**What to add**:
- `SessionManager.ForkFrom(sourcePath, cwd)` — copy a session file and create a new one branched at a chosen `entryId`.
- `SessionManager.ContinueRecent(cwd)` — load the most recently modified session file from `~/.rho/sessions/<cwd-hash>/`.
- `SessionManager.Open(path)` — open an existing session by path.
- `SessionManager.InMemory()` — in-memory session for `--no-session`.
- Session entries must track `entryId` (UUID per turn) to support `fork` at a specific message.
- The RPC `fork`, `clone`, `switch_session`, `get_fork_messages` commands in [rpc.go](file:///home/whick/Coding/rho/pkg/agent/rpc/rpc.go) currently stub out; wire them to the real session manager once it supports the above.

---

### 2. Model Registry (MEDIUM IMPACT)

**pi reference**: [`pi/packages/coding-agent/src/core/model-registry.ts`](file:///home/whick/Coding/rho/pi/packages/coding-agent/src/core/model-registry.ts) (~900 LOC)

**rho current**: [`pkg/agent/codecore/model_registry.go`](file:///home/whick/Coding/rho/pkg/agent/codecore/model_registry.go) — fetches models from providers but doesn't persist scoped model lists.

**What to add**:
- `GetAvailable()` — merges default models + provider-fetched models, filtered by configured auth.
- Scoped model list support: `--models claude-3-5-sonnet,gpt-4o` lets the user cycle through a subset with `cycle_model` (Ctrl+P).
- `ModelRegistry.Find(provider, id)` — used by RPC `set_model`.
- Wire `GetModelRegistry()` from `AgentSessionRuntime` into the RPC server's `set_model` and `get_available_models` handlers (currently those handlers call `ai.DefaultModels()` directly).

---

### 3. RPC — Abort & streaming cancel (MEDIUM IMPACT)

**pi reference**: `session.abort()` in `agent-session.ts`; uses an `AbortController` on every streaming request.

**rho current**: The RPC `abort` command returns success immediately but does not actually cancel an in-flight `runAgentStream` goroutine.

**What to add**:
- Add a `context.CancelFunc` field to `Server`; store it when `runAgentStream` starts; call it on `abort`.
- Pass the context into `agent.AgentLoop.Run(ctx, ...)` — the loop must pass it to the provider SSE stream.
- Reference: [pkg/agent/agent_loop.go](file:///home/whick/Coding/rho/pkg/agent/agent_loop.go) `Run()` method.

---

### 4. Package Manager — real npm/git install (MEDIUM IMPACT)

**pi reference**: [`pi/packages/coding-agent/src/core/package-manager.ts`](file:///home/whick/Coding/rho/pi/packages/coding-agent/src/core/package-manager.ts) (~1900 LOC)

**rho current**: [pkg/agent/codecore/package_manager_cli.go](file:///home/whick/Coding/rho/pkg/agent/codecore/package_manager_cli.go) — stubs out install/update with placeholder manifest files; download is a no-op.

**What to add**:
- Parse source strings: `npm:<pkg>`, `git:<url>`, `https://...`, `./local/path`.
- For `npm:` sources: run `npm install --prefix <extDir>/<name> <pkg>` via `os/exec`.
- For `git:` / `https:` sources: run `git clone <url> <extDir>/<name>`.
- For local paths: symlink or copy the directory.
- `update` should re-run the appropriate fetch command.
- Persist the source string in `extension.json` so future `update` knows where to fetch from.

---

### 5. Config subcommand — interactive TUI (LOW IMPACT)

**pi reference**: [`pi/packages/coding-agent/src/cli/config-selector.ts`](file:///home/whick/Coding/rho/pi/packages/coding-agent/src/cli/config-selector.ts)

**rho current**: `runConfigMode()` in [main.go](file:///home/whick/Coding/rho/cmd/rho/main.go) just prints the path to the settings file.

**What to add**: Wire through to the existing `showSettingsSelector()` TUI in [interactive_settings.go](file:///home/whick/Coding/rho/cmd/rho/interactive_settings.go) — or at minimum open the settings JSON in `$EDITOR`.

---

### 6. Export HTML (LOW IMPACT)

**pi reference**: [`pi/packages/coding-agent/src/core/export-html/`](file:///home/whick/Coding/rho/pi/packages/coding-agent/src/core/export-html/)

**rho current**: [pkg/agent/export/](file:///home/whick/Coding/rho/pkg/agent/export/) exists but the RPC `export_html` command returns "not supported". Wire it up:
1. Call `export.ExportSession(messages, outputPath)`.
2. Return `{"path": "<absolute output path>"}` in the RPC response.

---

### 7. RPC tests (LOW IMPACT)

There are no test files in `pkg/agent/rpc/`. Add:
- A test that feeds JSON Lines commands into the server via a pipe and asserts JSON Lines responses.
- Tests for the UUID generator, extension UI blocking flow (using mock channels).

---

## Key Architecture Notes

- **No external Go deps** (beyond `golang.org/x/term`). All solutions must use stdlib only.
- **JSON Lines protocol** in RPC mode: one JSON object per line on stdout; one JSON object per line from stdin. The server captures `os.Stdout` on startup and redirects it to `os.Stderr` so `fmt.Print` calls don't corrupt framing.
- **Extension UI blocking**: RPC server maintains `map[string]*pendingUI` (keyed by UUID). `Select`/`Confirm`/`Input` write an `extension_ui_request` line and block on a channel. When stdin receives `{"type":"extension_ui_response","id":"<uuid>",...}` the matching channel is unblocked.
- **`pi` reference is at `/home/whick/Coding/rho/pi/`** — always cross-reference TypeScript source before implementing anything to ensure behavioural parity.

---

## Quick Commands

```bash
# Build
go build -o rho ./cmd/rho

# Run all tests
go test ./...

# Run RPC mode manually (paste JSON on stdin)
./rho --mode rpc
{"type":"get_state"}
{"type":"prompt","message":"hello"}

# Run interactive mode
./rho
```

# Comprehensive Architectural Analysis: pi (TypeScript) vs. rho (Go)

This document details the architectural, design, and code-level differences between the original TypeScript coding agent **pi** and **rho** (its Go port). While **rho** is intended to be a near 1:1 parity reimplementation of **pi**, a deep audit of both codebases reveals critical implementation gaps, bugs, stubs, and dead code in **rho**.

---

## 1. High-Level Architecture & Core Package Mapping

The package structure of the original TypeScript monorepo maps to internal Go packages, but their underlying dependencies and execution models differ:

| Component | pi (TypeScript) Source Path | rho (Go) Source Path | Functional & Architectural Parity |
| :--- | :--- | :--- | :--- |
| **Core TUI Engine** | [packages/tui/src/](file:///home/whick/Coding/rho/pi/packages/tui) | [pkg/tui/](file:///home/whick/Coding/rho/pkg/tui) | **Low Parity**. `pi` uses a custom layout engine with Synced Output (`CSI 2026`) and zero-width markers for IME terminal rendering. `rho` wraps layouts inside a custom [Bubble Tea Model](file:///home/whick/Coding/rho/pkg/tui/bubbletea_model.go) using Bubble Tea and Lip Gloss. |
| **Session Manager** | [session-manager.ts](file:///home/whick/Coding/rho/pi/packages/coding-agent/src/core/session-manager.ts) | [session.go](file:///home/whick/Coding/rho/pkg/agent/session.go) | **Low Parity**. `pi` stores history in append-only JSONL files forming a single-file tree structure via `id` and `parentId` pointers. `rho` saves flat pretty-printed JSON files and copies them in their entirety for forks, ignoring the tree model. |
| **Search Tools** | [find.ts](file:///home/whick/Coding/rho/pi/packages/coding-agent/src/core/tools/find.ts) <br> [grep.ts](file:///home/whick/Coding/rho/pi/packages/coding-agent/src/core/tools/grep.ts) | [tools.go](file:///home/whick/Coding/rho/pkg/agent/tools/tools.go) | **Low Parity & Buggy**. `pi` downloads and executes compiled binaries `fd` and `ripgrep` for speed and `.gitignore` compliance. `rho` performs naive, slow scans on the main thread using Go's `filepath.Walk`, with glob path matching bugs in `Find` and literal-only substring searches in `Grep`. |
| **Compaction** | [compaction/](file:///home/whick/Coding/rho/pi/packages/coding-agent/src/core/compaction) | [compaction/](file:///home/whick/Coding/rho/pkg/agent/compaction) | **None (Dead Code)**. `pi` runs a complex context-compaction routine with token calculations, LLM-based structured summarization prompts, split-turn merges, and file tracking. `rho` has a placeholder package that is completely unimported and unused. |
| **Extension Runtime** | [extensions/](file:///home/whick/Coding/rho/pi/packages/coding-agent/src/core/extensions) | [extensions/](file:///home/whick/Coding/rho/pkg/agent/extensions) | **Partial (Dead Hooks)**. `pi` executes ESM scripts in-process using runtime TypeScript compilation (`jiti`). `rho` supervises out-of-process subprocesses using JSON-RPC 2.0 or parses static configuration manifests. 5 of 12 lifecycle hooks are completely dead code in `rho`. |
| **Slash Commands** | [slash-commands.ts](file:///home/whick/Coding/rho/pi/packages/coding-agent/src/core/slash-commands.ts) | [slash_commands.go](file:///home/whick/Coding/rho/pkg/agent/codecore/slash_commands.go) | **Partial (Mocks & Stubs)**. Most session commands in `rho`'s command registry are empty stubs printing mock notifications. `interactive.go` handles some, but `/share` is a manual instruction prompt, and `/export` has been integrated with the HTML exporter. |
| **Skills System** | [skills.ts](file:///home/whick/Coding/rho/pi/packages/coding-agent/src/core/skills.ts) | [skills.go](file:///home/whick/Coding/rho/pkg/agent/skills/skills.go) | **Low Parity & Token Bloat**. `pi` formats skills as an XML-based metadata list to support dynamic lazy-loading via the read tool. `rho` eagerly formats and dumps the full text of all skills directly into the system prompt. `rho`'s custom frontmatter parser is fragile (no real YAML parser) and validation is completely absent. |
| **Prompt Templates** | [prompt-templates.ts](file:///home/whick/Coding/rho/pi/packages/coding-agent/src/core/prompt-templates.ts) | [prompt_templates.go](file:///home/whick/Coding/rho/pkg/agent/codecore/prompt_templates.go) | **Low Parity**. `pi` parses markdown templates with frontmatter, supporting bash-style positional argument substitution and TUI command expansion. `rho` uses Go-style `{{.Key}}` placeholders parsed via `strings.ReplaceAll` and completely lacks user-facing command expansion. |
| **TUI Multiline Editor** | [editor.ts](file:///home/whick/Coding/rho/pi/packages/tui/src/components/editor.ts) | [components.go](file:///home/whick/Coding/rho/pkg/tui/components.go) <br> [interactive.go](file:///home/whick/Coding/rho/cmd/rho/interactive.go) | **Low Parity & Buggy**. `pi` implements a full 2200+ line multiline editor with word wrapping, autocomplete dropdown, undo/redo stacks, and a kill-ring. `rho` uses a Bubble Tea `chatModel` with a single-line input field, ignores arrow up/down cursor movements, and suffers from a layout bug that breaks terminal rendering when newlines are entered. However, undo/redo stacks and kill-ring (yank/yank-pop) have been successfully integrated. |
| **Image Protocol Cleanup** | [terminal-image.ts](file:///home/whick/Coding/rho/pi/packages/tui/src/terminal-image.ts) <br> [tui.ts](file:///home/whick/Coding/rho/pi/packages/tui/src/tui.ts) | [image.go](file:///home/whick/Coding/rho/pkg/tui/image.go) | **None (Dead Code)**. `pi` tracks image IDs and actively deletes them when scrolled out of view to avoid overlapping rendering artifacts. `rho` implements `DeleteKittyImage` but never invokes it anywhere in the application. |
| **Telemetry Collection** | [telemetry.ts](file:///home/whick/Coding/rho/pi/packages/coding-agent/src/core/telemetry.ts) | [telemetry.go](file:///home/whick/Coding/rho/pkg/agent/codecore/telemetry.go) | **Buggy**. `pi` collects opt-in version pings. `rho` implements an opt-in telemetry collector, but its manual JSON string encoder uses `%v` formatting, outputting strings without quotes and creating invalid, unparseable JSON files. |

---

## 2. Session Persistence & Tree-Based Branching

The representation of conversation history is one of the most significant architectural divergences.

### pi: Single-File JSONL Append-Only Trees
- **Files**: [session-manager.ts](file:///home/whick/Coding/rho/pi/packages/coding-agent/src/core/session-manager.ts)
- **Data Format**: Newline-delimited JSON (`.jsonl`). Each line is a discrete, typed `SessionEntry` containing an `id` (short 8-character hex ID) and a `parentId`.
- **Tree Structure**: Instead of a flat list, messages form a tree within a single file. When a user forks or branches the conversation, a new branch is created by simply appending new entries whose `parentId` references a historical node.
- **LLM Context Reconstruction**: Built via [buildSessionContext()](file:///home/whick/Coding/rho/pi/packages/coding-agent/src/core/session-manager.ts#L314-L421). It starts at the active `leafId` and walks backwards via `parentId` pointers to the root. It reverses the path, applies configuration changes (model, thinking level), handles custom injected messages, and folds in the latest compaction summary.

### rho: Flat JSON Deep Copies
- **Files**: [session.go](file:///home/whick/Coding/rho/pkg/agent/session.go)
- **Data Format**: Pretty-printed JSON arrays (`.json`) containing `SessionEntry` structs.
- **Divergence**: Although `SessionEntry` defines a `ParentID` field:
  ```go
  type SessionEntry struct {
      Type     string        `json:"type"`
      ID       string        `json:"id"`
      ParentID string        `json:"parentId"` // Defined but unassigned
      ...
  }
  ```
  The [Save()](file:///home/whick/Coding/rho/pkg/agent/session.go#L63) method never sets `ParentID` when serializing messages.
- **Fork Implementation**: Forking is implemented via [ForkSession()](file:///home/whick/Coding/rho/pkg/agent/session.go#L266) by:
  1. Loading the entire parent session `.json` file into memory.
  2. Copying the full flat list of messages.
  3. Appending any new prompt messages.
  4. Writing a completely new, duplicated `.json` file.
  No single-file tree branching or history traversal exists in `rho`.

---

## 3. Built-in Search Tools & Glob Path Matching Bugs

Search and indexing tools show a heavy performance and reliability gap.

### pi: Compiled Binaries & Optimized Execution
- **Files**: [find.ts](file:///home/whick/Coding/rho/pi/packages/coding-agent/src/core/tools/find.ts), [grep.ts](file:///home/whick/Coding/rho/pi/packages/coding-agent/src/core/tools/grep.ts)
- **Binary Spawning**: Uses [ensureTool()](file:///home/whick/Coding/rho/pi/packages/coding-agent/src/core/utils/tools-manager.ts) to verify or download compiled native executables for `fd` and `rg` (`ripgrep`).
- **Hierarchical Paths**: If a search pattern contains `/` (e.g., `src/**/*.go`), the `find` tool automatically appends `--full-path` and formats the expression with a leading `**/` to ensure hierarchical matches resolve correctly.
- **Grep Richness**: Ripgrep is spawned with `--json` and `--fixed-strings` (if literal matches are desired). It supports regex searches natively, streams output via `readline`, and terminates the process immediately if the match limit is reached.

### rho: Naive main-thread filepath.Walk & Glob Matching Bugs
- **Files**: [tools.go](file:///home/whick/Coding/rho/pkg/agent/tools/tools.go)
- **Execution Model**: Performs synchronous file traversal on the main thread using Go's standard library `filepath.Walk`. It does not read or respect `.gitignore` files, except for hardcoded exclusions of `.git`, `node_modules`, and `vendor`.
- **The Glob Matching Bug**: In `findFiles()`:
  ```go
  err := filepath.Walk(searchPath, func(path string, info os.FileInfo, err error) error {
      ...
      matched, err := filepath.Match(pattern, info.Name()) // Evaluates filename only!
      ...
  })
  ```
  Because `filepath.Match` is run against `info.Name()` (which returns only the base filename, e.g. `main.go`), **any query with directory paths (e.g. `src/**/*.go` or `cmd/main.go`) fails to match anything.**
- **Dummy Glob Tool**: The `Glob` tool is a stub that delegates directly to `Find`, inheriting the glob matching bugs and lacking double-star (`**`) support.
- **Literal-only Grep**: The `Grep` tool ignores regex completely:
  ```go
  if ignoreCase {
      found = strings.Contains(strings.ToLower(line), strings.ToLower(pattern))
  } else {
      found = strings.Contains(line, pattern)
  }
  ```
  It executes a simple line-by-line scanner with `strings.Contains`, failing to support regex, fixed-strings, context lines, or line limit thresholds.

---

## 4. Dead Context Compaction Runtime

### pi: Interactive, LLM-based Compaction
- **Files**: [compaction.ts](file:///home/whick/Coding/rho/pi/packages/coding-agent/src/core/compaction/compaction.ts), [utils.ts](file:///home/whick/Coding/rho/pi/packages/coding-agent/src/core/compaction/utils.ts)
- **Summarization Prompts**: If conversation history exceeds the token budget (context window minus `reserveTokens`), it triggers a structured LLM summarization.
  - **Initial Summarization**: Formats conversation text and uses `SUMMARIZATION_PROMPT` to generate structured Markdown (`## Goal`, `## Constraints & Preferences`, `## Progress`, `## Key Decisions`, `## Next Steps`, `## Critical Context`).
  - **Incremental Updates**: Uses `UPDATE_SUMMARIZATION_PROMPT` to iteratively merge new turns into an existing compaction summary.
  - **Split Turns**: If the compaction point falls in the middle of a multi-message turn, it summarizes the early turn prefix using `TURN_PREFIX_SUMMARIZATION_PROMPT` and keeps the active turn suffix, merging the two.
- **State Tracking**: Extracts file modifications (`CompactionDetails.modifiedFiles`) and reads (`readFiles`) to preserve details of work accomplished.

### rho: Completely Disconnected Package
- **Files**: [compaction.go](file:///home/whick/Coding/rho/pkg/agent/compaction/compaction.go)
- **Status**: **Dead Code**. The entire package is completely unreferenced by the rest of the Go codebase. It is not imported by `pkg/agent`, `cmd/rho`, or `pkg/agent/codecore`.
- **Implementation**: The unused file implements a naive token estimator (`len(text)/4 + 1`) and a simple static message counts string generator (`generateSimpleSummary`), completely missing LLM prompts, split-turn merging, or file-state tracking.

---

## 5. Extension Lifecycle Hooks

Because Go is compiled and statically linked, `rho` replaces `pi`'s dynamic in-process TS compilation (`jiti`) with out-of-process subprocesses communicating over JSON-RPC 2.0. However, the hook dispatch pipeline in Go is incomplete:

> [!WARNING]
> 5 out of the 12 registered extension lifecycle hooks are dead code in Go.

| Extension Hook | pi Status | rho Status | Code Location |
| :--- | :--- | :--- | :--- |
| **`SessionStart`** | Active | Active | Fired in [agent_session_runtime.go:L97](file:///home/whick/Coding/rho/pkg/agent/codecore/agent_session_runtime.go#L97) |
| **`SessionShutdown`** | Active | Active | Fired in [agent_session_runtime.go:L118](file:///home/whick/Coding/rho/pkg/agent/codecore/agent_session_runtime.go#L118) |
| **`AgentStart`** | Active | Active | Fired in [agent_session_runtime.go:L153](file:///home/whick/Coding/rho/pkg/agent/codecore/agent_session_runtime.go#L153) and [interactive.go:L1889](file:///home/whick/Coding/rho/cmd/rho/interactive.go#L1889) |
| **`AgentEnd`** | Active | Active | Fired in [agent_session_runtime.go:L229](file:///home/whick/Coding/rho/pkg/agent/codecore/agent_session_runtime.go#L229) and [interactive.go:L1926](file:///home/whick/Coding/rho/cmd/rho/interactive.go#L1926) |
| **`BeforeAgentStart`** | Active | Active | Fired in [agent_session_runtime.go:L162](file:///home/whick/Coding/rho/pkg/agent/codecore/agent_session_runtime.go#L162) |
| **`Input`** | Active | Active | Fired in [interactive.go:L1852](file:///home/whick/Coding/rho/cmd/rho/interactive.go#L1852) |
| **`ToolCall`** | Active | Active | Fired in [agent_loop.go:L341](file:///home/whick/Coding/rho/pkg/agent/agent_loop.go#L341) |
| **`ToolResult`** | Active | Active | Fired in [agent_loop.go:L401](file:///home/whick/Coding/rho/pkg/agent/agent_loop.go#L401) |
| **`TurnStart`** | Active | **Dead Code** | Defined in `runtime.go` but never called. |
| **`TurnEnd`** | Active | **Dead Code** | Defined in `runtime.go` but never called. |
| **`Context`** | Active | **Dead Code** | Defined in `runtime.go` but never called. |
| **`BeforeProviderRequest`** | Active | **Dead Code** | Defined in `runtime.go` but never called. |
| **`UserBash`** | Active | **Dead Code** | Defined in `runtime.go` but never called. |

---

## 6. Slash Commands & Mock UI Stubs

Several interactive features in the `rho` CLI are mocked, returning text reminders rather than performing work.

### Mocks in the Slash Command Registry
- **Files**: [slash_commands.go](file:///home/whick/Coding/rho/pkg/agent/codecore/slash_commands.go)
- **Stubbed Commands**:
  - `/scoped-models`: Stubbed (notifies `"Scoped models registry"`).
  - `/cost`: Stubbed (notifies `"Token usage and cost information"`).
  - `/context`: Stubbed (notifies `"Context usage info"`).
  - `/import`: Stubbed (notifies `"Session import is not implemented in this TUI yet."`).
  - `/share`: Stubbed (notifies `"Session sharing is not implemented in this TUI yet."`).
  - `/clone`: Stubbed (notifies `"Session cloning is not implemented in this TUI yet."`).
  - `/tree`: Stubbed (notifies `"Session tree navigation is not implemented in this TUI yet."`).
  - `/new`: Stubbed (notifies `"Starting a new interactive session is not implemented in this TUI yet."`).
  - `/resume`: Stubbed (notifies `"Session resume selection is not implemented in this TUI yet."`).

### TUI-level Overrides & Bypassed HTML Exporter
In interactive mode ([interactive.go](file:///home/whick/Coding/rho/cmd/rho/interactive.go#L1975-L2035)), several commands are intercepted to present selector modals:
- `/sessions`, `/resume`, and `/tree` all spawn the same flat list selector modal. Tree structure navigation is completely unavailable.
- `/share` simply formats a string containing instructions to locate the JSON file under `~/.rho/sessions/` and manually upload it. No secret Gist integration exists.
- `/new` terminates the active runtime context and sets up a clean, blank session.

> [!NOTE]
> The `/export` command in `interactive.go` has been fully implemented to use `rho`'s Go HTML exporter package located in [export.go](file:///home/whick/Coding/rho/pkg/agent/export/export.go), generating a styled HTML output by default.

---

## 7. Agent Skills System & Context Token Bloat

The system for injecting specialized guidelines (skills) is architecturally different and suffers from severe efficiency and parsing issues in `rho`.

### pi: Lazy-Loaded XML Catalogs with Recursive Ignore Rules
- **Files**: [skills.ts](file:///home/whick/Coding/rho/pi/packages/coding-agent/src/core/skills.ts)
- **Constraint Validation**: Validates skill names against standard constraints (e.g. max length 64, regex `/^[a-z0-9-]+$/` (must be lowercase alphanumeric and hyphens only, no consecutive or terminal hyphens)). Validates that descriptions are present and under 1024 characters.
- **Recursive Discovery**: Recursively traverses folders for `SKILL.md` (which marks a folder as a skill root and stops further recursion) or raw `.md` files. It dynamically reads and respects `.gitignore`, `.ignore`, and `.fdignore` files to filter resources.
- **LLM Prompt Strategy**: Does not dump the skill file content into the prompt. Instead, it generates a clean XML block:
  ```xml
  <available_skills>
    <skill>
      <name>name</name>
      <description>description</description>
      <location>/path/to/SKILL.md</location>
    </skill>
  </available_skills>
  ```
  It instructs the LLM to locate appropriate skills by description and use its `read` tool to load the file contents dynamically (lazy-loading). This keeps the system prompt extremely small.
- **Feature Options**: Excludes skills that set `"disable-model-invocation": true` in their frontmatter, keeping them only triggerable via explicit slash commands.

### rho: Eager-Loaded Prompt Dumps and Fragile Custom Parsers
- **Files**: [skills.go](file:///home/whick/Coding/rho/pkg/agent/skills/skills.go)
- **Zero Validation**: Completely bypasses all name and description format constraints, skipping checks entirely.
- **Shallow Scanning**: Only scans files inside directories passed directly to `LoadSkills` using flat `os.ReadDir`, ignoring subdirectories and ignoring `.gitignore`/ignore files.
- **Token Bloat**: Instead of XML indexing, `FormatSkillsForPrompt()` iterates through all loaded skills and appends their entire markdown bodies into the system prompt:
  ```go
  parts = append(parts, "--- Skills ---")
  for _, skill := range skills {
      if skill.Description != "" {
          parts = append(parts, fmt.Sprintf("## %s", skill.Name))
          ...
      }
      parts = append(parts, skill.Content) // Dumps full content into the prompt!
  }
  ```
  This creates massive, unnecessary context overhead for the LLM.
- **Fragile Line Parser**: Instead of a standard YAML library, frontmatter is extracted using a naive line-by-line prefix matcher:
  ```go
  switch {
  case strings.HasPrefix(line, "name:"):
      name = strings.TrimSpace(strings.TrimPrefix(line, "name:"))
  ...
  ```
  This parser fails if frontmatter keys are nested, contain formatting variations, or are documented inside multiline YAML blocks.
- **Ignored Features**: Bypasses `"disable-model-invocation"` metadata completely, forcing all files into the prompt.

---

## 8. Prompt Template Engine & Command Expansion

Prompt templates allow formatting repeatable workflows (e.g. `/explain <arg>`), but `rho` lacks crucial parser features and user-facing integration.

### pi: Bash-Style Positional Substitution and Command Interception
- **Files**: [prompt-templates.ts](file:///home/whick/Coding/rho/pi/packages/coding-agent/src/core/prompt-templates.ts)
- **YAML Frontmatter**: Templates are parsed as markdown files with standard frontmatter support.
- **Bash Parity Sub**: Substitutes arguments with full bash-like flexibility via [substituteArgs()](file:///home/whick/Coding/rho/pi/packages/coding-agent/src/core/prompt-templates.ts#L68-L102):
  - `$1`, `$2`, etc. for individual arguments.
  - `$@` and `$ARGUMENTS` for all arguments.
  - `${@:N}` and `${@:N:L}` for slicing arguments (bash-style indices).
- **User Commands**: Integrates with [expandPromptTemplate()](file:///home/whick/Coding/rho/pi/packages/coding-agent/src/core/prompt-templates.ts#L282-L298). If a user types `/templateName arg1 arg2` in chat, the TUI intercepts it and expands it inline using the templates directory, presenting the fully populated template text as the prompt.

### rho: Naive ReplaceAll and No User-Facing Expansion
- **Files**: [prompt_templates.go](file:///home/whick/Coding/rho/pkg/agent/codecore/prompt_templates.go)
- **Builtin-Only Engine**: Relies on a hardcoded list of internal system templates (`summarize`, `compact`, `branch-summary`, `plan`, `code-review`, `explain`).
- **File Format**: Expects files with a `.tpl` suffix accompanied by an optional `<name>.json` file for metadata (instead of inline YAML frontmatter in `.md` files).
- **Format Limitation**: Naively replaces placeholders using `strings.ReplaceAll` for `{{.Key}}` syntax:
  ```go
  result = strings.ReplaceAll(result, "{{."+key+"}}", val)
  ```
  It has no support for positional indices (`$1`, `$2`), wildcards (`$@`), or array slicing.
- **Missing CLI Expansion**: `rho` does not have any code representing `expandPromptTemplate`. Writing `/explain some-code` in the input field does not expand the `explain` template; it is sent as-is or triggers an unrecognized slash command error because command expansion is completely missing.

---

## 9. TUI Multiline Input & Cursor Navigation

The terminal interaction experience in `rho` lacks a multi-line input component, making multiline typing frustrating and buggy.

### pi: Componentized Multiline Editor with Undo Stack & Kill Ring
- **Files**: [editor.ts](file:///home/whick/Coding/rho/pi/packages/tui/src/components/editor.ts), [undo-stack.ts](file:///home/whick/Coding/rho/pi/packages/tui/src/undo-stack.ts), [kill-ring.ts](file:///home/whick/Coding/rho/pi/packages/tui/src/kill-ring.ts)
- **Multiline Layout**: Features a complete, custom, word-wrapped, and scrolling multiline text area component (`Editor`) with cursor tracking.
- **Advanced Editing**: Implements an Emacs-style kill-ring (cut/paste stack), an undo/redo stack, and interactive inline autocomplete dropdowns.

### rho: Single-Line Input Wrapper and Terminal Layout Bugs
- **Files**: [components.go](file:///home/whick/Coding/rho/pkg/tui/components.go), [interactive.go](file:///home/whick/Coding/rho/cmd/rho/interactive.go)
- **Single-Line Limitation**: The `Input` component in Go is strictly single-line (its `Render` method returns `[]string{" " + displayContent + " "}`).
- **Key Navigation Deficit**: In `interactive.go`, the `chatModel` update loop intercepts keyboard inputs:
  - Arrow keys `left` and `right` navigate the cursor horizontally.
  - Arrow keys `up` and `down` are only bound to navigate the autocomplete dropdown menu when it is active. When autocomplete is closed, **arrow up and down are completely ignored.**
  - If a user enters a multiline message using `alt+enter` (which inserts a `\n` rune), they **cannot navigate the cursor vertically** to edit previous lines because arrow up/down do not move the cursor vertically.
- **TUI Layout Bug**: When a multiline string is edited:
  - `inputTextWithCursor()` appends `▏` and returns the multiline text.
  - `renderInput()` treats the input as a single line, slicing it by column width:
    ```go
    if tui.VisibleWidth(text) > width {
        text = tui.SliceByColumn(text, tui.VisibleWidth(text)-width, tui.VisibleWidth(text), true)
    }
    ```
    This ignores embedded newlines (`\n`). When written to the terminal, the string splits across lines anyway, rendering over the input area borders and breaking the TUI layout.
- **Undo/Kill Ring**: Redo/undo stacks and paste kill-rings have been integrated into the `chatModel` input loop, supporting Emacs-style commands (`ctrl+z`/`ctrl+_` to undo, `ctrl+y` to yank, `alt+y` to yank-pop, and kill accumulation via `ctrl+k`).

---

## 10. Kitty Terminal Image Protocol Cleanup

Image rendering support contains dead code that leads to terminal visual issues.

### pi: Active Viewport Tracking & Cleanup Escape Sequences
- **Files**: [terminal-image.ts](file:///home/whick/Coding/rho/pi/packages/tui/src/terminal-image.ts), [tui.ts](file:///home/whick/Coding/rho/pi/packages/tui/src/tui.ts)
- **Escape sequence**: Defines `deleteKittyImage` (`\x1b_Ga=d,d=I,i=<id>,q=2\x1b\\`) to delete an image and free its terminal graphics buffer.
- **Cleanup Loop**: The `TUI` rendering engine tracks which Kitty image IDs are currently visible on screen. When lines containing images scroll off-screen, `tui.ts` immediately transmits the deletion escape sequence to clear those images from the terminal's canvas.

### rho: Unreferenced Functions (Visual Overlaps)
- **Files**: [image.go](file:///home/whick/Coding/rho/pkg/tui/image.go)
- **Parity Deficit**: Go implements `DeleteKittyImage` and `DeleteAllKittyImages` in its image package:
  ```go
  func DeleteKittyImage(id int) string {
      return fmt.Sprintf("\x1b_Ga=d,i=%d\x07", id)
  }
  ```
  However, **these functions are completely uncalled and unimported** by any rendering loop or component in the codebase.
- **Terminal Rendering Bugs**: Because images are never deleted from the terminal canvas, scrolling the terminal window or clearing the console leaves orphan images floating on top of new text, corrupting the UI display.

---

## 11. Telemetry Event Marshalling Bug

The telemetry logging module in `rho` writes corrupt logs to disk when enabled.

### Files
- Go Path: [telemetry.go](file:///home/whick/Coding/rho/pkg/agent/codecore/telemetry.go)

### Parity Deficit & Serialization Bug
The `flush()` method in `TelemetryCollector` attempts to manually serialize events into JSONL format instead of using Go's standard library `json.Marshal` or `json.NewEncoder`:
```go
for _, event := range tc.events {
    line := fmt.Sprintf(`{"type":"%s","timestamp":%d,"sessionId":"%s"`,
        event.Type, event.Timestamp, event.SessionID)
    if len(event.Data) > 0 {
        line += `,"data":{`
        first := true
        for k, v := range event.Data {
            if !first {
                line += ","
            }
            line += fmt.Sprintf(`"%s":%v`, k, v) // BUG: v is formatted as raw string %v
            first = false
        }
        line += "}"
    }
    line += "}\n"
    f.WriteString(line)
}
```
Because arbitrary values in the `Data` map are serialized using `fmt.Sprintf("%v")`:
- String values (e.g. `provider: "openai"`, `model: "gpt-4o"`, `tool: "grep"`) are written without surrounding double quotes (e.g. `"provider":openai`, `"model":gpt-4o`).
- This outputs syntax-invalid JSON documents to `telemetry.jsonl`, rendering the telemetry log unparseable by any JSON decoder.

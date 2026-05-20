# `@earendil-works/pi-agent-core` — Comprehensive Feature List

**Package:** `pi/packages/agent` · Version 0.75.3  
**Description:** "General-purpose agent with transport abstraction, state management, and attachment support"  
**Entry points:** `.` (all exports), `./node` (adds `NodeExecutionEnv`), `./package.json`

---

## 1. Core Agent Class

### 1.1 `Agent` class (`agent.ts`)
- **Purpose:** Stateful wrapper around the low-level agent loop. Owns transcript, emits lifecycle events, executes tools, manages queues.
- **Constructor fields:**
  - `initialState` — partial `AgentState` seed
  - `convertToLlm` — custom AgentMessage→Message converter (default: filter to user/assistant/toolResult)
  - `transformContext` — optional AgentMessage-level transform before LLM calls
  - `streamFn` — pluggable streaming function (default: `streamSimple` from `@earendil-works/pi-ai`)
  - `getApiKey` — dynamic API key resolver per-provider
  - `onPayload` / `onResponse` — low-level stream lifecycle hooks
  - `beforeToolCall` / `afterToolCall` — tool lifecycle hooks
  - `prepareNextTurn` — cross-turn state/model/thinkingLevel replacement hook
  - `steeringMode` / `followUpMode` — queue drain modes (`"all"` or `"one-at-a-time"`)
  - `sessionId` — forwarded to provider for cache-aware backends
  - `thinkingBudgets` — per-level token budgets forwarded to stream
  - `transport` — preferred transport (`"auto"`, `"rest"`, `"grpc"`, etc.)
  - `maxRetryDelayMs` — cap on provider retry delays
  - `toolExecution` — `"sequential"` or `"parallel"` (default: parallel)
- **Lifecycle methods:**
  - `subscribe(listener)` — subscribe to `AgentEvent`s (returns unsubscribe fn)
  - `prompt(input)` — start new prompt (text, single message, or batch)
  - `continue()` — continue from current transcript (last message must be user or toolResult)
  - `abort()` — abort current run
  - `waitForIdle()` — resolve when current run + all event listeners settle
  - `reset()` — clear transcript, runtime state, and all queues
- **State access (`state` getter):** exposes `AgentState` (systemPrompt, model, thinkingLevel, tools, messages, isStreaming, streamingMessage, pendingToolCalls, errorMessage)
- **Queue management:**
  - `steer(message)` / `followUp(message)` — enqueue messages
  - `clearSteeringQueue()` / `clearFollowUpQueue()` / `clearAllQueues()`
  - `hasQueuedMessages()`
  - `steeringMode` / `followUpMode` getters/setters
- **Internal machinery:**
  - `PendingMessageQueue` — inner class with `all`/`one-at-a-time` drain modes
  - `ActiveRun` — tracks promise, resolve, and AbortController per run
  - `normalizePromptInput()` — normalizes text/message[] inputs
  - `runPromptMessages()` / `runContinuation()` — internal run executors
  - `createContextSnapshot()` / `createLoopConfig()` — snapshot current state for loop
  - `runWithLifecycle()` — orchestrates run lifecycle (starting, failure handling, finishing)
  - `handleRunFailure()` — emits synthetic assistant error message on failure
  - `processEvents()` — reduces events into internal state, then notifies listeners

---

## 2. Agent Loop (Low-Level)

### 2.1 `agent-loop.ts` — Loop Functions
- **`agentLoop(prompts, context, config, signal?, streamFn?)`** — starts loop with new prompt messages. Returns `EventStream<AgentEvent, AgentMessage[]>`.
- **`agentLoopContinue(context, config, signal?, streamFn?)`** — continues from existing context (no new prompt). Validates last message is not assistant role. Returns `EventStream<AgentEvent, AgentMessage[]>`.
- **`runAgentLoop(prompts, context, config, emit, signal?, streamFn?)`** — async low-level loop starting with prompts. Emits `agent_start`, `turn_start`, message events for prompts, runs inner loop, returns `AgentMessage[]`.
- **`runAgentLoopContinue(context, config, emit, signal?, streamFn?)`** — async low-level loop continuing from context. Returns `AgentMessage[]`.
- **`createAgentStream()`** — creates `EventStream` with `agent_end` as terminal event.
- **`runLoop()`** — shared main loop:
  - Outer loop: drains follow-up queue when agent would otherwise stop
  - Inner loop: processes tool calls + steering messages
  - Calls `streamAssistantResponse()` → `executeToolCalls()` → emits `turn_end` → checks `prepareNextTurn`/`shouldStopAfterTurn` → polls steering queue
- **`streamAssistantResponse()`** — transforms AgentMessage[]→Message[] via `config.convertToLlm`, applies `transformContext`, resolves API key, calls `streamFn`, processes streaming events, returns final `AssistantMessage`.
- **`executeToolCalls()`** — dispatches to sequential or parallel execution based on `config.toolExecution` and per-tool `executionMode`.
- **`executeToolCallsSequential()`** — one-by-one: prepare → execute → finalize → emit → collect tool result messages.
- **`executeToolCallsParallel()`** — prepare all sequentially, then `Promise.all` execute for non-immediate calls, emits `tool_execution_end` in completion order, emits tool-result messages in source order.
- **`prepareToolCall()`** — finds tool, runs `prepareArguments` shim, validates arguments via `validateToolArguments`, calls `beforeToolCall` hook, returns `PreparedToolCall` or `ImmediateToolCallOutcome`.
- **`executePreparedToolCall()`** — runs `tool.execute()`, collects `onUpdate` partials.
- **`finalizeExecutedToolCall()`** — runs `afterToolCall` hook, merges overrides.
- **Helper types/utilities:**
  - `AgentEventSink` — `(event: AgentEvent) => Promise<void> | void`
  - `PreparedToolCall` / `ImmediateToolCallOutcome` / `ExecutedToolCallOutcome` / `FinalizedToolCallOutcome` / `FinalizedToolCallEntry`
  - `shouldTerminateToolBatch()` — true if every finalized tool has `terminate: true`
  - `prepareToolCallArguments()` — applies per-tool `prepareArguments` shim
  - `createErrorToolResult()` — builds error `AgentToolResult`
  - `emitToolExecutionEnd()`, `createToolResultMessage()`, `emitToolResultMessage()`

---

## 3. Type System

### 3.1 `types.ts` — All Type Definitions
- **`StreamFn`** — stream function contract (must not throw, must encode failures in stream)
- **`ToolExecutionMode`** — `"sequential"` | `"parallel"`
- **`QueueMode`** — `"all"` | `"one-at-a-time"`
- **`ThinkingLevel`** — `"off"` | `"minimal"` | `"low"` | `"medium"` | `"high"` | `"xhigh"`
- **`AgentToolCall`** — tool-call content block from assistant message
- **`BeforeToolCallResult`** — `{ block?: boolean; reason?: string }`
- **`AfterToolCallResult`** — `{ content?, details?, isError?, terminate? }` (field-by-field merge)
- **`BeforeToolCallContext`** — assistantMessage, toolCall, args, context
- **`AfterToolCallContext`** — assistantMessage, toolCall, args, result, isError, context
- **`ShouldStopAfterTurnContext`** — message, toolResults, context, newMessages
- **`AgentLoopTurnUpdate`** — `{ context?, model?, thinkingLevel? }`
- **`PrepareNextTurnContext`** — extends `ShouldStopAfterTurnContext`
- **`AgentLoopConfig`** (extends `SimpleStreamOptions`):
  - `model`, `convertToLlm`, `transformContext?`, `getApiKey?`
  - `shouldStopAfterTurn?`, `prepareNextTurn?`
  - `getSteeringMessages?`, `getFollowUpMessages?`
  - `toolExecution?`, `beforeToolCall?`, `afterToolCall?`
- **`AgentMessage`** — union of standard LLM `Message` + `CustomAgentMessages[keyof CustomAgentMessages]`
- **`CustomAgentMessages`** — extensible interface for app-defined message types (declaration merging)
- **`AgentState`** — `systemPrompt`, `model`, `thinkingLevel`, `tools` (get/set with copy), `messages` (get/set with copy), `isStreaming`, `streamingMessage?`, `pendingToolCalls`, `errorMessage?`
- **`AgentToolResult<T>`** — `content`, `details`, `terminate?`
- **`AgentToolUpdateCallback<T>`** — `(partialResult: AgentToolResult<T>) => void`
- **`AgentTool<TParameters, TDetails>`** (extends `Tool<TParameters>`):
  - `label` — human-readable UI label
  - `prepareArguments?` — compatibility shim before schema validation
  - `execute()` — `(toolCallId, params, signal?, onUpdate?) => Promise<AgentToolResult<TDetails>>`
  - `executionMode?` — per-tool `"sequential"` | `"parallel"` override
- **`AgentContext`** — `systemPrompt`, `messages`, `tools?`
- **`AgentEvent`** — discriminated union:
  - `agent_start` / `agent_end` (with messages[])
  - `turn_start` / `turn_end` (with message + toolResults[])
  - `message_start` / `message_update` (with assistantMessageEvent) / `message_end`
  - `tool_execution_start` / `tool_execution_update` / `tool_execution_end`

---

## 4. Agent Harness

### 4.1 `AgentHarness` class (`harness/agent-harness.ts`)
- **Purpose:** High-level agent orchestrator with session persistence, skill invocation, prompt templates, compaction, branch summarization, and an extensive event/hook system.
- **Type parameters:** `<TSkill, TPromptTemplate, TTool>` — generic for app-specific types.
- **Constructor options** (via `AgentHarnessOptions`):
  - `env: ExecutionEnv`, `session: Session`
  - `tools?`, `resources?`, `systemPrompt?` (string or callback), `getApiKeyAndHeaders?`
  - `streamOptions?`, `model`, `thinkingLevel?`, `activeToolNames?`, `steeringMode?`, `followUpMode?`
- **Core prompt/skill/template methods:**
  - `prompt(text, options?)` — execute a turn with user text → returns `AssistantMessage`
  - `skill(name, additionalInstructions?)` — look up skill, format invocation, execute turn
  - `promptFromTemplate(name, args?)` — look up prompt template, substitute args, execute turn
- **Queue/steering methods:**
  - `steer(text, options?)` — queue mid-turn steering message (only when busy)
  - `followUp(text, options?)` — queue post-turn follow-up message (only when busy)
  - `nextTurn(text, options?)` — queue message for next turn (also when idle)
  - `getSteeringMode()` / `setSteeringMode()` / `getFollowUpMode()` / `setFollowUpMode()`
- **State management methods:**
  - `getModel()` / `getThinkingLevel()` / `setModel()` / `setThinkingLevel()` (persists via session)
  - `getResources()` / `setResources()` — skills + prompt templates
  - `getStreamOptions()` / `setStreamOptions()` — transport/timeout/retries/headers/metadata/cacheRetention
  - `setTools()` / `setActiveTools()` — tool registry and active subset
  - `appendMessage()` — append directly to session (immediate or pending)
- **Session lifecycle methods:**
  - `compact(customInstructions?)` — run session compaction (context window summarization)
  - `navigateTree(targetId, options?)` — navigate session tree to a different entry (branch summarization)
  - `abort()` — abort current run, clears queues, returns cleared messages
  - `waitForIdle()` — wait for current run to settle
- **Event/Hook system:**
  - `subscribe(listener)` — catch-all wildcard listener for all events
  - `on(type, handler)` — typed handler that can return result values
  - Events: `before_agent_start`, `context`, `before_provider_request`, `before_provider_payload`, `after_provider_response`, `tool_call`, `tool_result`, `session_before_compact`, `session_compact`, `session_before_tree`, `session_tree`, `model_select`, `thinking_level_select`, `resources_update`, `queue_update`, `save_point`, `abort`, `settled`
  - `emitBeforeProviderRequest()` — allows hooks to patch stream options per-request
  - `emitBeforeProviderPayload()` — allows hooks to transform the request payload
- **Internal machinery:**
  - `createTurnState()` — builds fresh state from session + resources
  - `createContext()` — builds `AgentContext` from turn state
  - `createStreamFn()` — constructs `StreamFn` with auth, headers, provider hooks
  - `createLoopConfig()` — constructs `AgentLoopConfig` wiring hooks to loop callbacks
  - `executeTurn()` — orchestrates a single turn with `runAgentLoop`
  - `handleAgentEvent()` — routes events to session persistence + emitter
  - `flushPendingSessionWrites()` — batch-persists pending writes to session
  - `drainQueuedMessages()` — drains queue respecting mode
  - `validateToolNames()` — validates tool name exists
  - `emitRunFailure()` — emits synthetic failure event sequence

### 4.2 Harness Types (`harness/types.ts`)
- **`Skill`** — `name`, `description`, `content`, `filePath`, `disableModelInvocation?`
- **`PromptTemplate`** — `name`, `description?`, `content`
- **`AgentHarnessResources<TSkill, TPromptTemplate>`** — `promptTemplates?`, `skills?`
- **`AgentHarnessStreamOptions`** — `transport?`, `timeoutMs?`, `maxRetries?`, `maxRetryDelayMs?`, `headers?`, `metadata?`, `cacheRetention?`
- **`AgentHarnessStreamOptionsPatch`** — partial overrides with header/metadata delete semantics
- **`ExecutionEnv` / `FileSystem` / `Shell`** — abstract filesystem and shell interfaces
- **`FileKind`** — `"file"` | `"directory"` | `"symlink"`
- **`FileError` / `FileErrorCode`** — typed file errors
- **`ExecutionError` / `ExecutionErrorCode`** — typed execution errors
- **`SessionError` / `SessionErrorCode`** — typed session errors
- **`CompactionError` / `CompactionErrorCode`** — typed compaction errors
- **`BranchSummaryError` / `BranchSummaryErrorCode`** — typed branch summary errors
- **`AgentHarnessError` / `AgentHarnessErrorCode`** — public harness failure classification
- **`FileInfo`** — `name`, `path`, `kind`, `size`, `mtimeMs`
- **`ExecutionEnvExecOptions`** — `cwd?`, `env?`, `timeout?`, `abortSignal?`, `onStdout?`, `onStderr?`
- **`Result<T, E>`** — `{ ok: true, value: T } | { ok: false, error: E }`
- **Utility functions:** `ok()`, `err()`, `getOrThrow()`, `getOrUndefined()`, `toError()`
- **`SessionTreeEntry`** — discriminated union of all entry types:
  - `MessageEntry` — stores an `AgentMessage`
  - `ThinkingLevelChangeEntry` — stores thinking level string
  - `ModelChangeEntry` — stores provider + modelId
  - `CompactionEntry<T>` — stores summary, firstKeptEntryId, tokensBefore, details, fromHook
  - `BranchSummaryEntry<T>` — stores fromId, summary, details, fromHook
  - `CustomEntry<T>` — stores customType + data
  - `CustomMessageEntry<T>` — stores customType + content + display + details
  - `LabelEntry` — stores targetId + label
  - `SessionInfoEntry` — stores name (legacy name)
  - `LeafEntry` — stores targetId (current leaf pointer)
- **`SessionContext`** — `messages`, `thinkingLevel`, `model`
- **`SessionMetadata` / `JsonlSessionMetadata`** — id, createdAt, cwd, path, parentSessionPath
- **`SessionStorage<TMetadata>`** — interface for persistent session storage
- **`SessionRepo` / `JsonlSessionRepoApi`** — repository interfaces for CRUD + fork
- **`SessionCreateOptions` / `SessionForkOptions`** — creation/fork options
- **Event types:** `QueueUpdateEvent`, `SavePointEvent`, `AbortEvent`, `SettledEvent`, `BeforeAgentStartEvent`, `ContextEvent`, `BeforeProviderRequestEvent`, `BeforeProviderPayloadEvent`, `AfterProviderResponseEvent`, `ToolCallEvent`, `ToolResultEvent`, `SessionBeforeCompactEvent`, `SessionCompactEvent`, `SessionBeforeTreeEvent`, `SessionTreeEvent`, `ModelSelectEvent`, `ThinkingLevelSelectEvent`, `ResourcesUpdateEvent`
- **`AgentHarnessOwnEvent`** — union of harness-specific events
- **`AgentHarnessEvent`** — union of `AgentEvent` + `AgentHarnessOwnEvent`
- **Hook result types:** `BeforeAgentStartResult`, `ContextResult`, `BeforeProviderRequestResult`, `BeforeProviderPayloadResult`, `ToolCallResult`, `ToolResultPatch`, `SessionBeforeCompactResult`, `SessionBeforeTreeResult`
- **`AgentHarnessEventResultMap`** — maps event type to its hook return type
- **`AbortResult`** — `clearedSteer`, `clearedFollowUp`
- **`CompactResult`** — `summary`, `firstKeptEntryId`, `tokensBefore`, `details?`
- **`NavigateTreeResult`** — `cancelled`, `editorText?`, `summaryEntry?`
- **`CompactionSettings`** — `enabled`, `reserveTokens`, `keepRecentTokens`
- **`CompactionPreparation`** — `firstKeptEntryId`, `messagesToSummarize`, `turnPrefixMessages`, `isSplitTurn`, `tokensBefore`, `previousSummary?`, `fileOps`, `settings`
- **`FileOperations`** — `read`, `written`, `edited` Sets
- **`TreePreparation`** — targetId, oldLeafId, commonAncestorId, entriesToSummarize, userWantsSummary, customInstructions?, replaceInstructions?, label?
- **`GenerateBranchSummaryOptions`** — model, apiKey, headers?, signal, customInstructions?, replaceInstructions?, reserveTokens?
- **`BranchSummaryResult`** — summary, readFiles, modifiedFiles
- **`AgentHarnessOptions`** — full constructor options interface
- **`AgentHarnessPhase`** — `"idle"` | `"turn"` | `"compaction"` | `"branch_summary"` | `"retry"`
- **`PendingSessionWrite`** — union of all entry writes (excluding id, parentId, timestamp)

---

## 5. Session System

### 5.1 `Session` class (`harness/session/session.ts`)
- **Purpose:** High-level session abstraction wrapping `SessionStorage`. Manages the session tree, builds LLM context, appends various entry types.
- **`getMetadata()`**, `getStorage()`, `getLeafId()`, `getEntry(id)`, `getEntries()`
- **`getBranch(fromId?)`** — get path from leaf to root
- **`buildContext()`** — builds `SessionContext` with messages, thinking level, model, handling compaction summaries and branch summaries
- **`getLabel(id)`** — get label for entry
- **`getSessionName()`** — find latest session_info name
- **Entry appenders:**
  - `appendMessage(message)` — creates `MessageEntry`
  - `appendThinkingLevelChange(level)` — creates `ThinkingLevelChangeEntry`
  - `appendModelChange(provider, modelId)` — creates `ModelChangeEntry`
  - `appendCompaction(...)` — creates `CompactionEntry`
  - `appendCustomEntry(customType, data?)` — creates `CustomEntry`
  - `appendCustomMessageEntry(customType, content, display, details?)` — creates `CustomMessageEntry`
  - `appendLabel(targetId, label)` — creates `LabelEntry` (validates target exists)
  - `appendSessionName(name)` — creates `SessionInfoEntry`
- **`moveTo(entryId, summary?)`** — navigate tree to different leaf, optionally writes `BranchSummaryEntry`
- **`buildSessionContext(entries)`** — static function: walks path entries, accumulates thinkingLevel/model changes, handles compaction summarization insertion, builds message list for LLM context

### 5.2 JSONL Session Storage (`harness/session/jsonl-storage.ts`)
- **`JsonlSessionStorage`** — file-based session storage using JSONL format (v3)
- **Session file format:** First line is `SessionHeader` (type, version: 3, id, timestamp, cwd, parentSession?), subsequent lines are `SessionTreeEntry` JSON objects
- **`loadJsonlSessionMetadata(fs, filePath)`** — reads just the header line for metadata
- **`JsonlSessionStorage.create(fs, filePath, options)`** — write header, return empty storage
- **`JsonlSessionStorage.open(fs, filePath)`** — load full file into memory
- **Implements:** `SessionStorage<JsonlSessionMetadata>` — in-memory cache with append-to-file persistence
- **Inner state:** `entries[]`, `byId` Map, `labelsById` Map, `currentLeafId`
- **Entry ID generation:** uses `uuidv7` truncated to first 8 hex chars, falls back to full UUID after 100 collisions
- **Parsing/validation:** `parseHeaderLine()`, `parseEntryLine()`, `leafIdAfterEntry()`, `updateLabelCache()`

### 5.3 Memory Session Storage (`harness/session/memory-storage.ts`)
- **`InMemorySessionStorage<TMetadata>`** — in-memory implementation of `SessionStorage`
- **Constructor:** accepts optional `entries[]` and `metadata`
- **Full implementation** of all `SessionStorage` methods without file I/O
- **Same entry ID generation** as JSONL storage

### 5.4 Session Repositories

#### `JsonlSessionRepo` (`harness/session/jsonl-repo.ts`)
- **Purpose:** Manages JSONL session files on disk. Creates, opens, lists, deletes, and forks sessions.
- **Session directory structure:** `{sessionsRoot}/{encoded-cwd}/{timestamp}_{sessionId}.jsonl`
- **`cwd encoding`:** `--{cwd-with-slashes-and-colons-replaced-by-dashes}--`
- **`create(options)`** — creates session file with header
- **`open(metadata)`** — opens existing session
- **`list(options?)`** — lists all sessions (optionally filtered by cwd), sorted newest-first
- **`delete(metadata)`** — removes session file
- **`fork(source, options)`** — copies entries from source session up to fork point into new session, optionally recording parent session path

#### `InMemorySessionRepo` (`harness/session/memory-repo.ts`)
- **Purpose:** In-memory session repository for testing.
- **Implements `SessionRepo<SessionMetadata>`** — create, open, list, delete, fork
- Stores sessions in a `Map<string, Session>` internally

### 5.5 Session Repository Utilities (`harness/session/repo-utils.ts`)
- **`createSessionId()`** — generates UUIDv7
- **`createTimestamp()`** — ISO timestamp
- **`toSession(storage)`** — wraps `SessionStorage` in `Session`
- **`getFileSystemResultOrThrow(result, message)`** — unwraps `Result`, throws `SessionError` on failure
- **`getEntriesToFork(storage, options)`** — determines which entries to copy when forking, respecting position (`"before"` or `"at"`)

### 5.6 UUID v7 Generator (`harness/session/uuid.ts`)
- **`uuidv7()`** — generates time-sorted UUIDs (UUID v7 scheme)
- Timestamp-based with sequence counter for collision resolution
- Uses `crypto.getRandomValues()` for random bytes (falls back to `Math.random()`)
- 36-char standard UUID format with hyphens

---

## 6. Message System

### 6.1 Message Types & Converters (`harness/messages.ts`)
- **Custom message roles** (via `CustomAgentMessages` declaration merging):
  - `bashExecution` — `{ role: "bashExecution", command, output, exitCode, cancelled, truncated, fullOutputPath?, excludeFromContext? }`
  - `custom` — `{ role: "custom", customType, content, display, details?, timestamp }`
  - `branchSummary` — `{ role: "branchSummary", summary, fromId, timestamp }`
  - `compactionSummary` — `{ role: "compactionSummary", summary, tokensBefore, timestamp }`
- **`bashExecutionToText(msg)`** — formats bash execution as readable text
- **`createBranchSummaryMessage(summary, fromId, timestamp)`** — factory
- **`createCompactionSummaryMessage(summary, tokensBefore, timestamp)`** — factory
- **`createCustomMessage(customType, content, display, details, timestamp)`** — factory
- **`convertToLlm(messages)`** — converts `AgentMessage[]` → `Message[]`:
  - `bashExecution` → user message with formatted text (or filtered out if `excludeFromContext`)
  - `custom` → user message (string or content array)
  - `branchSummary` → user message with XML-wrapped summary (`<summary>...</summary>`)
  - `compactionSummary` → user message with XML-wrapped summary
  - `user`/`assistant`/`toolResult` → pass through
  - Unknown roles → filtered out
- **Constants:** `COMPACTION_SUMMARY_PREFIX`, `COMPACTION_SUMMARY_SUFFIX`, `BRANCH_SUMMARY_PREFIX`, `BRANCH_SUMMARY_SUFFIX`

---

## 7. Compaction System

### 7.1 Compaction Core (`harness/compaction/compaction.ts`)
- **`CompactionSettings`** — `{ enabled, reserveTokens (default: 16384), keepRecentTokens (default: 20000) }`
- **`DEFAULT_COMPACTION_SETTINGS`**
- **`calculateContextTokens(usage)`** — sums total tokens from usage
- **`getLastAssistantUsage(entries)`** — finds last successful assistant usage in session tree entries
- **`estimateContextTokens(messages)`** — returns `ContextUsageEstimate` with `tokens`, `usageTokens`, `trailingTokens`, `lastUsageIndex`
- **`estimateTokens(message)`** — heuristic token counter (~4 chars per token), handles text/thinking/toolCall/image content
- **`shouldCompact(contextTokens, contextWindow, settings)`** — checks if compaction is needed
- **`findValidCutPoints(entries, startIndex, endIndex)`** — enumerates entry indices that are valid cut points (user, assistant, bashExecution, custom, branchSummary, compactionSummary, custom_message)
- **`findTurnStartIndex(entries, entryIndex, startIndex)`** — finds start of the turn containing an entry
- **`findCutPoint(entries, startIndex, endIndex, keepRecentTokens)`** — selects best cut point respecting token budget, returns `CutPointResult`
- **`prepareCompaction(pathEntries, settings)`** — analyzes session entries, returns `CompactionPreparation` or undefined
- **`generateSummary(messages, model, reserveTokens, apiKey, ...)`** — calls LLM to generate/update structured summary
- **`compact(preparation, model, apiKey, ...)`** — orchestrates full compaction: generates summary (handles split-turn cases), computes file operations, returns `CompactionResult`
- **`generateTurnPrefixSummary()`** — summarizes the prefix of a split turn
- **Prompt templates:**
  - `SUMMARIZATION_SYSTEM_PROMPT` — "You are a context summarization assistant..."
  - `SUMMARIZATION_PROMPT` — structured format with Goal / Constraints / Progress (Done/In Progress/Blocked) / Key Decisions / Next Steps / Critical Context
  - `UPDATE_SUMMARIZATION_PROMPT` — incremental update format for existing summaries
  - `TURN_PREFIX_SUMMARIZATION_PROMPT` — for split-turn prefix context
- **`CompactionDetails`** — `readFiles`, `modifiedFiles`
- **`CompactionResult<T>`** — `summary`, `firstKeptEntryId`, `tokensBefore`, `details?`
- **`CutPointResult`** — `firstKeptEntryIndex`, `turnStartIndex`, `isSplitTurn`
- **`ContextUsageEstimate`** — `tokens`, `usageTokens`, `trailingTokens`, `lastUsageIndex`

### 7.2 Compaction Utilities (`harness/compaction/utils.ts`)
- **`FileOperations`** — `{ read, written, edited }` Sets of file paths
- **`createFileOps()`** — returns empty FileOperations
- **`extractFileOpsFromMessage(message, fileOps)`** — scans assistant tool calls for `read`, `write`, `edit` operations
- **`computeFileLists(fileOps)`** — computes sorted `readFiles` and `modifiedFiles` (edited ∪ written)
- **`formatFileOperations(readFiles, modifiedFiles)`** — formats as XML `<read-files>` / `<modified-files>` blocks
- **`serializeConversation(messages)`** — converts `Message[]` to plain text for summarization prompts, handles user/assistant/toolResult roles, truncates tool results to 2000 chars

### 7.3 Branch Summarization (`harness/compaction/branch-summarization.ts`)
- **`collectEntriesForBranchSummary(session, oldLeafId, targetId)`** — finds entries from old leaf to common ancestor, returns `CollectEntriesResult`
- **`prepareBranchEntries(entries, tokenBudget)`** — selects messages within token budget, preserves compaction/branch summaries, extracts file ops
- **`generateBranchSummary(entries, options)`** — calls LLM to generate structured branch summary
- **`BranchPreparation`** — `messages`, `fileOps`, `totalTokens`
- **`CollectEntriesResult`** — `entries`, `commonAncestorId`
- **`BranchSummaryDetails`** — `readFiles`, `modifiedFiles`
- **Prompt:** `BRANCH_SUMMARY_PREAMBLE`, `BRANCH_SUMMARY_PROMPT` (same structured format as compaction)

---

## 8. Skill System

### 8.1 Skill Loading (`harness/skills.ts`)
- **Purpose:** Loads skill definitions from `SKILL.md` files and `.md` files in root directories.
- **`Skill` interface** — `name`, `description`, `content`, `filePath`, `disableModelInvocation?`
- **`formatSkillInvocation(skill, additionalInstructions?)`** — formats skill as XML block for model prompt
- **`loadSkills(env, dirs)`** — traverses directories recursively, loads `SKILL.md` files (one per directory), loads root-level `.md` files, respects ignore files (`.gitignore`, `.ignore`, `.fdignore`), returns `{ skills, diagnostics }`
- **`loadSourcedSkills(env, inputs, mapSkill?)`** — loads skills with source provenance tagging
- **Frontmatter parsing:** YAML frontmatter (`---...---`) with `name`, `description`, `disable-model-invocation` fields
- **Naming validation:** lowercase a-z, 0-9, hyphens only; max 64 chars; must match parent directory name
- **Description validation:** required, max 1024 chars
- **`SkillDiagnostic`** — warning type with `code`, `message`, `path`
- **`SkillDiagnosticCode`** — `"file_info_failed"` | `"list_failed"` | `"read_failed"` | `"parse_failed"` | `"invalid_metadata"`
- **Ignore file handling:** scans for `.gitignore`, `.ignore`, `.fdignore`, applies patterns prefixed with relative directory

### 8.2 System Prompt Formatting (`harness/system-prompt.ts`)
- **`formatSkillsForSystemPrompt(skills)`** — formats visible skills (not `disableModelInvocation`) as XML `<available_skills>` block for system prompt
- XML structure: `<skill>` with `<name>`, `<description>`, `<location>` tags
- XML-escaping utility: `escapeXml()`

---

## 9. Prompt Template System

### 9.1 Prompt Template Loading (`harness/prompt-templates.ts`)
- **Purpose:** Loads prompt templates from `.md` files (directories scanned non-recursively).
- **`PromptTemplate` interface** — `name`, `description?`, `content`
- **`loadPromptTemplates(env, paths)`** — loads from file/directory paths, returns `{ promptTemplates, diagnostics }`
- **`loadSourcedPromptTemplates(env, inputs, mapPromptTemplate?)`** — source-provenance tagged loading
- **Frontmatter parsing:** YAML frontmatter with `description`, `argument-hint`, etc.
- **`PromptTemplateDiagnostic`** — `type: "warning"`, `code`, `message`, `path`
- **`PromptTemplateDiagnosticCode`** — `"file_info_failed"` | `"list_failed"` | `"read_failed"` | `"parse_failed"`
- **`parseCommandArgs(argsString)`** — shell-style argument parsing (single/double quotes)
- **`substituteArgs(content, args)`** — replaces placeholders: `$1`, `$2`, ..., `$@`, `$ARGUMENTS`, `${@:N}`, `${@:N:L}`
- **`formatPromptTemplateInvocation(template, args?)`** — formats template with positional arguments

---

## 10. Proxy System

### 10.1 Proxy Streaming (`proxy.ts`)
- **Purpose:** Route LLM streaming through a proxy server that handles auth.
- **`streamProxy(model, context, options)`** — stream function that fetches from `{proxyUrl}/api/stream`
- **`ProxyMessageEventStream`** — `EventStream` subclass for proxy events
- **`ProxyAssistantMessageEvent`** — server-sent events with stripped `partial` field:
  - `start`, `text_start/delta/end`, `thinking_start/delta/end`, `toolcall_start/delta/end`, `done`, `error`
- **`ProxyStreamOptions`** — authToken, proxyUrl, signal, plus serializable stream options (temperature, maxTokens, reasoning, cacheRetention, sessionId, headers, metadata, transport, thinkingBudgets, maxRetryDelayMs)
- **`buildProxyRequestOptions()`** — extracts serializable subset from full options
- **`processProxyEvent(proxyEvent, partial)`** — reconstructs `AssistantMessageEvent` from proxy event, builds partial message client-side
- Handles SSE protocol (`data: ...` lines), abort signals, error reporting

---

## 11. Node.js Execution Environment

### 11.1 `NodeExecutionEnv` (`harness/env/nodejs.ts`)
- **Purpose:** Implements `ExecutionEnv` (FileSystem + Shell) using Node.js APIs.
- **Filesystem operations** (all return `Result<..., FileError>`):
  - `absolutePath`, `joinPath` — path resolution
  - `readTextFile` — UTF-8 file read
  - `readTextLines` — line-oriented read with maxLines support
  - `readBinaryFile` — binary file read
  - `writeFile` — write with parent dir creation
  - `appendFile` — append with parent dir creation
  - `fileInfo` — stat-based file metadata
  - `listDir` — directory listing with lstat
  - `canonicalPath` — realpath resolution
  - `exists` — existence check
  - `createDir` — mkdir (recursive by default)
  - `remove` — rm (with recursive/force options)
  - `createTempDir` — mkdtemp
  - `createTempFile` — temp dir + empty file
  - `cleanup` — no-op
- **Shell execution (`exec`):**
  - Spawns bash with `-c` flag (finds bash on PATH on Windows via Git/bin/bash)
  - Supports `cwd`, `env`, `timeout`, `abortSignal`, `onStdout`, `onStderr`
  - Process tree killing on timeout/abort (SIGKILL on Unix, `taskkill /F /T` on Windows)
  - Returns `{ stdout, stderr, exitCode }` wrapped in `Result`
  - Error codes: `aborted`, `timeout`, `shell_unavailable`, `spawn_error`, `callback_error`, `unknown`
- **Path helpers:** `resolvePath()`, `fileKindFromStats()`, `fileInfoFromStats()`, `toFileError()`, `isNodeError()`, `pathExists()`
- **Shell discovery:** `getShellConfig()`, `findBashOnPath()`, `runCommand()` (uses `which`/`where`)

---

## 12. Shell Output Capture

### 12.1 Shell Capture Utility (`harness/utils/shell-output.ts`)
- **`ShellCaptureOptions`** — `onChunk?`, plus all `ExecutionEnvExecOptions` except onStdout/onStderr
- **`ShellCaptureResult`** — `output`, `exitCode`, `cancelled`, `truncated`, `fullOutputPath?`
- **`executeShellWithCapture(env, command, options?)`** — runs shell command with output capture:
  - Sanitizes binary output (filters control chars, handles unpaired surrogates)
  - Streams output to temp file when exceeding `DEFAULT_MAX_BYTES`
  - Maintains rolling buffer for tail truncation
  - Applies `truncateTail()` with `DEFAULT_MAX_BYTES * 2` window
  - Returns structured result with truncation metadata
- **`sanitizeBinaryOutput(str)`** — removes control characters, preserves tabs/newlines/carriage returns

---

## 13. Truncation Utilities

### 13.1 Truncation (`harness/utils/truncate.ts`)
- **Constants:** `DEFAULT_MAX_LINES = 2000`, `DEFAULT_MAX_BYTES = 50KB`, `GREP_MAX_LINE_LENGTH = 500`
- **`TruncationResult`** — content, truncated, truncatedBy, totalLines, totalBytes, outputLines, outputBytes, lastLinePartial, firstLineExceedsLimit, maxLines, maxBytes
- **`TruncationOptions`** — `maxLines?`, `maxBytes?`
- **`truncateHead(content, options?)`** — keeps first N lines/bytes (for file reads)
- **`truncateTail(content, options?)`** — keeps last N lines/bytes (for shell output)
- **`truncateLine(line, maxChars?)`** — truncates single line with `[truncated]` suffix
- **`formatSize(bytes)`** — human-readable byte size
- **Internal:** `utf8ByteLength()`, `replaceUnpairedSurrogates()`, `truncateStringToBytesFromEnd()`

---

## 14. Exported API Surface

### From `index.ts` (re-exports everything):

| Source | Exports |
|--------|---------|
| `./agent.js` | `Agent`, `AgentOptions`, `QueueMode` |
| `./agent-loop.js` | `agentLoop`, `agentLoopContinue`, `runAgentLoop`, `runAgentLoopContinue` |
| `./harness/agent-harness.js` | `AgentHarness` |
| `./harness/compaction/branch-summarization.js` | `BranchPreparation`, `BranchSummaryDetails`, `CollectEntriesResult`, `collectEntriesForBranchSummary`, `generateBranchSummary`, `prepareBranchEntries` |
| `./harness/compaction/compaction.js` | `calculateContextTokens`, `compact`, `DEFAULT_COMPACTION_SETTINGS`, `estimateContextTokens`, `estimateTokens`, `findCutPoint`, `findTurnStartIndex`, `generateSummary`, `getLastAssistantUsage`, `prepareCompaction`, `serializeConversation`, `shouldCompact` |
| `./harness/messages.js` | `BashExecutionMessage`, `CustomMessage`, `BranchSummaryMessage`, `CompactionSummaryMessage`, `bashExecutionToText`, `createBranchSummaryMessage`, `createCompactionSummaryMessage`, `createCustomMessage`, `convertToLlm` |
| `./harness/prompt-templates.js` | `loadPromptTemplates`, `loadSourcedPromptTemplates`, `parseCommandArgs`, `substituteArgs`, `formatPromptTemplateInvocation`, `PromptTemplateDiagnosticCode`, `PromptTemplateDiagnostic` |
| `./harness/session/jsonl-repo.js` | `JsonlSessionRepo` |
| `./harness/session/memory-repo.js` | `InMemorySessionRepo` |
| `./harness/session/repo-utils.js` | (internal utilities re-exported) |
| `./harness/session/session.js` | `Session`, `buildSessionContext` |
| `./harness/session/uuid.js` | `uuidv7` |
| `./harness/skills.js` | `loadSkills`, `loadSourcedSkills`, `formatSkillInvocation`, `SkillDiagnosticCode`, `SkillDiagnostic` |
| `./harness/system-prompt.js` | `formatSkillsForSystemPrompt` |
| `./harness/types.js` | All harness types, error classes, interfaces |
| `./harness/utils/shell-output.js` | `executeShellWithCapture`, `sanitizeBinaryOutput`, `ShellCaptureOptions`, `ShellCaptureResult` |
| `./harness/utils/truncate.js` | `truncateHead`, `truncateTail`, `truncateLine`, `formatSize`, `DEFAULT_MAX_LINES`, `DEFAULT_MAX_BYTES`, etc. |
| `./proxy.js` | `streamProxy`, `ProxyAssistantMessageEvent`, `ProxyStreamOptions` |
| `./types.js` | All core types |

### From `node.ts`:
- `NodeExecutionEnv` (from `./harness/env/nodejs.js`)
- Re-exports everything from `./index.js`

---

## 15. Error Class Hierarchy

| Class | Code Type | Parent | Purpose |
|-------|-----------|--------|---------|
| `AgentHarnessError` | `AgentHarnessErrorCode` | `Error` | Public harness failures (busy, invalid_state, invalid_argument, session, hook, auth, compaction, branch_summary, unknown) |
| `SessionError` | `SessionErrorCode` | `Error` | Session storage/tree failures |
| `CompactionError` | `CompactionErrorCode` | `Error` | Compaction failures |
| `BranchSummaryError` | `BranchSummaryErrorCode` | `Error` | Branch summary failures |
| `ExecutionError` | `ExecutionErrorCode` | `Error` | Shell execution failures |
| `FileError` | `FileErrorCode` | `Error` | Filesystem operation failures |

All error classes carry stable `code` strings and optional `cause` (nested Error).

---

## 16. Event System Summary

### Agent Events (emitted by Agent and low-level loops):
| Event | Fields | When |
|-------|--------|------|
| `agent_start` | — | Beginning of a prompt/continue run |
| `agent_end` | `messages: AgentMessage[]` | End of a run |
| `turn_start` | — | Start of an assistant turn |
| `turn_end` | `message`, `toolResults` | End of a turn |
| `message_start` | `message` | Any message added to context |
| `message_update` | `message`, `assistantMessageEvent` | Streaming assistant delta |
| `message_end` | `message` | Message fully received |
| `tool_execution_start` | `toolCallId`, `toolName`, `args` | Tool call begins |
| `tool_execution_update` | `toolCallId`, `toolName`, `args`, `partialResult` | Tool streaming partial result |
| `tool_execution_end` | `toolCallId`, `toolName`, `result`, `isError` | Tool call completes |

### Harness-Specific Events (additional to Agent events):
| Event | Type | When |
|-------|------|------|
| `queue_update` | Own | Queue contents change |
| `save_point` | Own | After turn_end, session flushes |
| `abort` | Own | Harness abort requested |
| `settled` | Own | Agent run fully completed |
| `before_agent_start` | Hook | Before prompt execution |
| `context` | Hook | Context transform opportunity |
| `before_provider_request` | Hook | Before LLM API call |
| `before_provider_payload` | Hook | Payload transform opportunity |
| `after_provider_response` | Own | After LLM API response |
| `tool_call` | Hook | Before tool execution |
| `tool_result` | Hook | After tool execution |
| `session_before_compact` | Hook | Before compaction |
| `session_compact` | Own | After compaction entry saved |
| `session_before_tree` | Hook | Before tree navigation |
| `session_tree` | Own | After tree navigation |
| `model_select` | Own | Model changed |
| `thinking_level_select` | Own | Thinking level changed |
| `resources_update` | Own | Resources (skills/templates) changed |

---

## 17. Dependency Summary

| Dependency | Purpose |
|------------|---------|
| `@earendil-works/pi-ai` ^0.75.3 | AI model interfaces, streaming, tool validation |
| `ignore` ^7.0.5 | .gitignore-style pattern matching for skill loading |
| `typebox` ^1.1.24 | Runtime type validation for tool argument schemas |
| `yaml` ^2.8.2 | YAML frontmatter parsing for skills and prompt templates |

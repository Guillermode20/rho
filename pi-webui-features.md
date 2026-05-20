# @earendil-works/pi-web-ui — Comprehensive Feature List

> Package: `pi/packages/web-ui` (v0.75.3) — Reusable Web UI components for AI chat interfaces powered by `@earendil-works/pi-ai`.

---

## 1. High-Level Chat Panel (`ChatPanel.ts`)

- **`<pi-chat-panel>`** — Top-level web component that assembles a complete AI chat UI.
- **Responsive layout** — Switches between side-by-side (≥800px) and overlay (<800px) modes for the artifacts panel.
- **Floating artifact badge** — When artifacts exist but the panel is collapsed, shows a clickable pill badge ("Artifacts" + count).
- **Agent lifecycle management** — Creates and wires together `AgentInterface`, `ArtifactsPanel`, and tooling.
- **Configurable setup** via `setAgent(agent, config)`:
  - `onApiKeyRequired` — Prompt for API key when missing.
  - `onBeforeSend` — Hook before sending a message.
  - `onCostClick` — Handle click on cost display.
  - `onModelSelect` — Override model selection behavior.
  - `sandboxUrlProvider` — Custom sandbox URL for browser extensions.
  - `toolsFactory` — Add custom tools with access to the agent, interface, artifacts panel, and runtime providers.
- **Runtime providers factory** — Exposes `SandboxRuntimeProvider[]` (attachments + artifacts read-write) for REPL tools.
- **Artifact reconstruction** — On load, reconstructs artifacts from existing `ArtifactMessage` entries in the conversation history.
- **Artifact auto-open** — Shows artifacts panel automatically when a new artifact is created.

---

## 2. Agent Interface (`components/AgentInterface.ts`)

- **`<agent-interface>`** — Lower-level chat interface component for custom layouts.
- **Property-driven configuration**:
  - `session` — Bind an `Agent` instance
  - `enableAttachments` (default: true)
  - `enableModelSelector` (default: true)
  - `enableThinkingSelector` (default: true)
  - `showThemeToggle` (default: false)
  - `onApiKeyRequired` — Custom API key prompt handler
  - `onBeforeSend` — Pre-send hook
  - `onBeforeToolCall` — Pre-tool-execution hook (can return false to veto)
  - `onCostClick` / `onModelSelect`
- **Message sending** via `sendMessage(input, attachments?)` — checks for API key, prompts if missing, then sends the message.
- **Auto-scroll** — Scrolls to bottom on new content; disables when user scrolls up, re-enables when near bottom.
- **Streaming message container** — Batching of rapid streaming updates via `requestAnimationFrame` for performance.
- **Usage stats bar** — Accumulates input/output/cache/cost from all assistant messages, formatted via `formatUsage()`.
- **Proxy auto-configuration** — Sets default `streamFn` that reads proxy settings from storage.
- **API key auto-retrieval** — Sets default `getApiKey` that reads from `ProviderKeysStore`.
- **`setInput(text, attachments?)`** — Programmatically set the message editor content.
- **`setAutoScroll(enabled)`** — Control auto-scroll behavior.

---

## 3. Message Components (`components/Messages.ts`)

### 3.1 Message Types
- **`UserMessageWithAttachments`** — Custom message role `"user-with-attachments"` containing inline file attachments.
- **`ArtifactMessage`** — Custom message role `"artifact"` for session persistence of artifact operations (create/update/delete with filename, content, timestamp).
- **Type augmentation** via `declare module "@earendil-works/pi-agent-core"` — Both types are injected into the `CustomAgentMessages` interface.

### 3.2 Built-in Message Renderers
- **`<user-message>`** — Renders user messages with fancy gradient pill styling and attachment tiles.
- **`<assistant-message>`** — Renders full assistant response with:
  - Ordered content blocks (text → markdown, thinking blocks, tool calls)
  - Thinking block expansion UI
  - Tool call rendering (delegates to `renderTool()`)
  - Usage/cost display (clickable via `onCostClick`)
  - Error display (`stopReason === "error"`)
  - Aborted state display
  - `hidePendingToolCalls` support (prevents duplication during streaming)
- **`<tool-message>`** — Renders individual tool call results. Delegates to registered `ToolRenderer` for custom rendering, or wraps in a default card.
- **`<tool-message-debug>`** — Debug view showing raw JSON of tool call arguments and results.
- **`<aborted-message>`** — Simple "Request aborted" text display.

### 3.3 Message Transformer
- **`defaultConvertToLlm(messages)`** — Converts app-level messages to LLM-compatible format:
  - `UserMessageWithAttachments` → user message with text + image/document content blocks
  - `ArtifactMessage` → filtered out (UI-only)
  - Standard messages → passed through
- **`convertAttachments(attachments)`** — Converts attachments to `(TextContent | ImageContent)[]` blocks.
- **Type guards**: `isUserMessageWithAttachments()`, `isArtifactMessage()`

### 3.4 Custom Message Renderer System (`message-renderer-registry.ts`)
- **`registerMessageRenderer(role, renderer)`** — Register a custom renderer for any message role.
- **`getMessageRenderer(role)`** — Look up a renderer.
- **`renderMessage(message)`** — Render a message using its registered renderer (returns `TemplateResult | undefined`).

---

## 4. Message Editor (`components/MessageEditor.ts`)

- **`<message-editor>`** — Text input area with:
  - Auto-resizing textarea (via `field-sizing: content`)
  - **File attachment via button** — opens file picker with configurable accepted types
  - **Drag & drop** — visual overlay, processes dropped files as attachments
  - **Paste image support** — pasted images become attachments
  - **Attachment tiles** — inline attachment display with delete button
  - **File validation**: `maxFiles` (default: 10), `maxFileSize` (default: 20MB)
  - **Thinking level selector** — Dropdown for models that support reasoning (`off`, `minimal`, `low`, `medium`, `high`)
  - **Model selector button** — Opens `ModelSelector` dialog
  - **Send button** (rotated arrow icon)
  - **Abort button** (square icon, shown during streaming)
  - **Keyboard shortcuts**: Enter to send, Shift+Enter for newline, Escape to abort
  - **Drag-over visual feedback** — Highlighted border and overlay text "Drop files here"
  - **Processing indicator** — Spinner shown while files are being processed

---

## 5. Message List (`components/MessageList.ts`)

- **`<message-list>`** — Stable message list using `lit-repeat` for efficient updates.
- Skips `artifact`-role messages (UI-only).
- Delegates unknown roles to custom renderers via `renderMessage()`.
- Renders paired `tool-result` messages inline with their parent `assistant` message.
- Accepts `pendingToolCalls`, `isStreaming`, `tools`, `onCostClick`.

---

## 6. Streaming Message Container (`components/StreamingMessageContainer.ts`)

- **`<streaming-message-container>`** — Manages the actively streaming assistant message.
- **Batched updates** — Uses `requestAnimationFrame` and deep-clones messages to batch rapid stream updates.
- **Immediate clearing** — `setMessage(null, true)` clears immediately.
- Shows a pulsing cursor indicator during streaming.
- Handles `assistant`, `user`, and `toolResult` message roles during streaming.

---

## 7. Attachment Tile (`components/AttachmentTile.ts`)

- **`<attachment-tile>`** — Inline thumbnail for file attachments:
  - **Image preview** — Renders thumbnail from base64 preview data.
  - **Document icon fallback** — For non-image attachments (Excel spreadsheet icon or generic document icon).
  - **PDF badge overlay** — Shows "PDF" label over the preview thumbnail.
  - **Click to expand** — Opens full-screen `AttachmentOverlay`.
  - **Delete button** — Conditional, shown with `showDelete` property (fades in on hover via CSS).
  - Truncated filename display.

---

## 8. Attachment Overlay (`dialogs/AttachmentOverlay.ts`)

- **`<attachment-overlay>`** — Full-screen modal for viewing file attachments:
  - **Image viewer** — Full-resolution image display.
  - **PDF viewer** — Renders all pages via `pdfjs-dist` at 1.5x scale with page separators.
  - **DOCX viewer** — Renders via `docx-preview` library with responsive styling.
  - **Excel viewer** — Renders as interactive HTML tables with multi-sheet tab navigation via `xlsx` library.
  - **PPTX viewer** — Displays extracted text content.
  - **Text viewer** — Plain text display for text-based files.
  - **Format toggle** — Switch between rendered view and extracted text view (for PDF, DOCX, Excel).
  - **Download button** — Download the original file.
  - **Escape key** to close.
  - **Backdrop click** to close.
  - **Loading task cancellation** on close (for PDF).

---

## 9. Console Block (`components/ConsoleBlock.ts`)

- **`<console-block>`** — Styled terminal output display for tool results:
  - Monospace font, bordered container with header ("console" label).
  - **Copy button** — Copies content to clipboard, shows "Copied!" feedback.
  - **Auto-scroll** — Scrolls to bottom on content update.
  - **Error variant** — Destructive color styling for error output.

---

## 10. Thinking Block (`components/ThinkingBlock.ts`)

- **`<thinking-block>`** — Collapsible "Thinking..." section for reasoning models:
  - **Shimmer animation** — Animated gradient text during streaming.
  - **Expand/collapse** — Chevron toggle to show/hide thinking content.
  - Renders content via `<markdown-block>`.

---

## 11. Input Component (`components/Input.ts`)

- **`Input`** — Reusable form input (functional component via `fc`):
  - Types: `text`, `email`, `password`, `number`, `url`, `tel`, `search`
  - Sizes: `sm`, `md`, `lg`
  - States: label, error message, disabled, required
  - Ref support for programmatic focus
  - Tailwind-styled with focus ring, dark mode support

---

## 12. Provider Key Input (`components/ProviderKeyInput.ts`)

- **`<provider-key-input>`** — API key management for a specific provider:
  - Password input field for API key.
  - **Save button** — Tests the key before saving.
  - **Test API key** — Sends a test completion request to validate the key (uses provider-specific test models).
  - **Status display**: green checkmark (configured) / red "✗ Invalid" badge / "Testing..." badge.
  - **Proxy-aware testing** — Applies CORS proxy if enabled in settings.
  - Automatically checks existing key status on connect.

---

## 13. Custom Provider Card (`components/CustomProviderCard.ts`)

- **`<custom-provider-card>`** — Displays a configured custom provider:
  - Provider name, type (capitalized), base URL.
  - **Auto-discovery status**: Connected (green dot + model count), Checking... (yellow), Disconnected (red).
  - **Manual provider**: Shows model count.
  - **Action buttons**: Refresh (auto-discovery only), Edit, Delete.

---

## 14. Expandable Section (`components/ExpandableSection.ts`)

- **`<expandable-section>`** — Generic expandable/collapsible section for tool renderers.
  - Captures and re-renders light DOM children.
  - Chevron right/down toggle.
  - `defaultExpanded` property.

---

## 15. Sandbox System (`components/sandbox/`)

### 15.1 SandboxIframe (`components/SandboxedIframe.ts`)
- **`<sandbox-iframe>`** — Sandboxed iframe for safe code execution:
  - **Two modes**:
    - `loadContent()` — Loads HTML and keeps it displayed (for HTML artifacts).
    - `execute()` — Runs code and returns a `Promise<SandboxResult>` (for REPL).
  - **Browser extension support** — `sandboxUrlProvider` loads sandbox via URL instead of `srcdoc` (for strict CSP).
  - **HTML validation** — Validates HTML via `DOMParser` before loading.
  - **Standalone HTML generation** — `prepareHtmlDocument()` for downloadable standalone HTML.
  - **Navigation interceptor** — Prevents navigation, opens external links via `postMessage`.
  - **Timeout** — 120-second execution timeout.
  - **AbortSignal support** — For cancellation of execution.
  - **`SandboxResult`** — Contains `success`, `console[]`, `files[]`, `error`, `returnValue`.

### 15.2 RuntimeMessageRouter (`components/sandbox/RuntimeMessageRouter.ts`)
- **`RUNTIME_MESSAGE_ROUTER`** — Singleton for all sandbox ↔ host communication:
  - **Single global `message` listener** instead of multiple individual listeners.
  - **Sandbox registration/cleanup** lifecycle management.
  - **Provider routing** — Routes messages to `SandboxRuntimeProvider.handleMessage()`.
  - **Consumer broadcasting** — Broadcasts messages to all registered `MessageConsumer`s.
  - **User script support** — Chrome extension `chrome.runtime.onUserScriptMessage` listener.

### 15.3 RuntimeMessageBridge (`components/sandbox/RuntimeMessageBridge.ts`)
- Generates `sendRuntimeMessage()` code for injection into sandbox contexts.
- **Two bridge types**: `sandbox-iframe` (uses `window.parent.postMessage`) and `user-script` (uses `chrome.runtime.sendMessage`).
- Provides `window.onCompleted(callback)` for lifecycle hooks.

### 15.4 SandboxRuntimeProvider Interface
- `getData()` — Data injected into `window` scope.
- `getRuntime()` — Function stringified and injected into sandbox.
- `handleMessage()` — Bidirectional communication handler.
- `getDescription()` — Documentation for LLM tool descriptions.
- `onExecutionStart/End()` — Lifecycle callbacks.

### 15.5 ConsoleRuntimeProvider
- **REQUIRED** — Captures `console.log/warn/error/info` from sandbox.
- Overrides console methods to capture output AND forward to original console.
- Provides `window.complete(error?, returnValue?)` for sandbox code to signal completion.
- Handles `error` and `unhandledrejection` events.
- Supports completion callbacks via `window.onCompleted()`.

### 15.6 AttachmentsRuntimeProvider
- Provides `listAttachments()`, `readTextAttachment(id)`, `readBinaryAttachment(id)` in sandbox.
- Works both **online** (via injected data) and **offline** (snapshot data for standalone HTML).
- Read-only access to user-uploaded files.

### 15.7 ArtifactsRuntimeProvider
- Provides `listArtifacts()`, `getArtifact(filename)`, `createOrUpdateArtifact()`, `deleteArtifact()` in sandbox.
- **Read-write mode** (REPL) or **read-only mode** (HTML artifacts).
- Auto-parses/stringifies `.json` files.
- Works both online and offline (read-only snapshot).
- Creates `ArtifactMessage` entries in agent state for persistence.

### 15.8 FileDownloadRuntimeProvider
- Provides `returnDownloadableFile(fileName, content, mimeType?)` in sandbox.
- Supports `Blob`, `Uint8Array`, `string`, and object content types.
- Online mode: sends to extension. Offline mode: triggers browser download directly.

---

## 16. Dialog System (`dialogs/`)

### 16.1 SettingsDialog (`dialogs/SettingsDialog.ts`)
- **`<settings-dialog>`** — Tabbed settings panel.
- **Left sidebar** (desktop) / **horizontal tabs** (mobile) navigation.
- Extensible via `SettingsTab` abstract class with `getTabName()`.

### 16.2 ApiKeysTab
- Shows `ProviderKeyInput` for all known providers from `@earendil-works/pi-ai`.

### 16.3 ProxyTab
- **CORS proxy configuration**:
  - Toggle to enable/disable proxy.
  - Proxy URL input field.
  - Auto-saves to `SettingsStore` on change.
  - Shows format hint: `<proxy-url>/?url=<target-url>`.

### 16.4 ProvidersModelsTab (`dialogs/ProvidersModelsTab.ts`)
- **Cloud Providers section** — Lists all known providers with API key inputs.
- **Custom Providers section** — Lists configured custom providers with:
  - Add Provider dropdown (Ollama, llama.cpp, vLLM, LM Studio, OpenAI-compatible, Anthropic-compatible).
  - Refresh (auto-discovery), Edit, Delete buttons per provider.
  - Auto-discovery providers show connection status (connected/disconnected/checking).

### 16.5 CustomProviderDialog (`dialogs/CustomProviderDialog.ts`)
- **`<custom-provider-dialog>`** — Add/edit a custom LLM provider:
  - Fields: Provider Name, Provider Type, Base URL, API Key (optional).
  - **Auto-discovery types** (Ollama, llama.cpp, vLLM, LM Studio): "Test Connection" button that discovers models.
  - **Manual types** (OpenAI Completions, OpenAI Responses, Anthropic Messages): shows info to add models after saving.
  - Saves to `CustomProvidersStore`.

### 16.6 ModelSelector (`dialogs/ModelSelector.ts`)
- **`<agent-model-selector>`** — Full model selection dialog:
  - **Search** — Subsequence-based fuzzy matching (characters in order, gap-scored).
  - **Filter chips**: Thinking (reasoning) toggle, Vision (image input) toggle.
  - **Model list** — Shows model ID, provider badge, capability icons (brain for reasoning, image for vision), context window/max tokens, cost info.
  - **Current model** — Marked with green checkmark, sorted to top.
  - **Custom provider models** — Loads and displays models from custom providers.
  - **Keyboard navigation**: Arrow keys + Enter, scrolls to selected item.
  - **Mouse/keyboard mode detection** — Prevents keyboard selection from fighting with mouse hover.
  - **Allowed providers** — Optional filter to restrict which providers are shown.

### 16.7 ApiKeyPromptDialog (`dialogs/ApiKeyPromptDialog.ts`)
- **`<api-key-prompt-dialog>`** — Prompts user for API key when missing.
- Shows `ProviderKeyInput` for the required provider.
- Polls for key existence every 500ms; auto-closes when key is saved.
- Static `prompt(provider)` method returns `Promise<boolean>`.

### 16.8 SessionListDialog (`dialogs/SessionListDialog.ts`)
- **`<session-list-dialog>`** — Load a previous conversation:
  - Lists sessions with title, relative date (Today/Yesterday/X days ago), message count, usage stats.
  - **Delete** button per session (with confirmation).
  - **Select** loads the session.
  - Tracks deleted sessions and notifies callback.
  - Relative date formatting.

### 16.9 PersistentStorageDialog (`dialogs/PersistentStorageDialog.ts`)
- **`<persistent-storage-dialog>`** — Requests persistent storage permission:
  - Explains why persistent storage is needed.
  - Lists what it means for the user.
  - Static `request()` method — returns `Promise<boolean>`.
  - Checks if already persisted before showing dialog.

---

## 17. Prompt Templates (`prompts/prompts.ts`)

- **`JAVASCRIPT_REPL_TOOL_DESCRIPTION`** — Dynamic tool description including runtime provider descriptions.
  - Lists available libraries (XLSX, PapaParse, Chart.js, Three.js via ESM imports).
  - Explains persistence via artifacts, attachment access, and execution environment.
- **`ARTIFACTS_TOOL_DESCRIPTION`** — Explains when to use artifacts tool vs REPL.
  - Lists commands: create, update (preferred), rewrite (last resort), get, delete, htmlArtifactLogs.
  - HTML artifacts requirements and helper functions.
  - Supported file types.
- **`ARTIFACTS_RUNTIME_PROVIDER_DESCRIPTION_RW`** — Read-write artifact storage API docs.
- **`ARTIFACTS_RUNTIME_PROVIDER_DESCRIPTION_RO`** — Read-only artifact API docs.
- **`ATTACHMENTS_RUNTIME_DESCRIPTION`** — Attachment access API docs with examples.
- **`EXTRACT_DOCUMENT_DESCRIPTION`** — Document extraction tool description.

---

## 18. Storage System (`storage/`)

### 18.1 StorageBackend Interface (`storage/types.ts`)
- Abstract multi-store key-value storage.
- Methods: `get`, `set`, `delete`, `keys`, `getAllFromIndex`, `clear`, `has`, `transaction`, `getQuotaInfo`, `requestPersistence`.
- `StorageTransaction` interface for atomic multi-store operations.

### 18.2 IndexedDBStorageBackend (`storage/backends/indexeddb-storage-backend.ts`)
- Full IndexedDB implementation of `StorageBackend`.
- **Auto-schema migration** — Creates/upgrades object stores on `onupgradeneeded`.
- **Index support** — Creates indices from config.
- **Prefix filtering** — `keys(storeName, prefix)` uses `IDBKeyRange`.
- **Cursor-based index traversal** — `getAllFromIndex()` with direction support.
- **Atomic transactions** — Multi-store transaction support.
- **Quota estimation** — Uses `navigator.storage.estimate()`.

### 18.3 Store Base Class (`storage/store.ts`)
- Abstract base for all stores.
- `getConfig()` — Returns `StoreConfig` (name, keyPath, indices).
- `setBackend()` / `getBackend()` — Backend wiring.

### 18.4 SettingsStore (`storage/stores/settings-store.ts`)
- Key-value settings (no keyPath — out-of-line keys).
- Methods: `get<T>`, `set`, `delete`, `list`, `clear`.

### 18.5 ProviderKeysStore (`storage/stores/provider-keys-store.ts`)
- API keys by provider name.
- Methods: `get`, `set`, `delete`, `list`, `has`.

### 18.6 SessionsStore (`storage/stores/sessions-store.ts`)
- Two object stores: `sessions` (full data) + `sessions-metadata` (lightweight for listing).
- Both stores have `lastModified` index for sorting.
- Methods: `save`, `get`, `getMetadata`, `getAllMetadata` (sorted by lastModified desc), `delete`, `updateTitle`, `getQuotaInfo`, `requestPersistence`.
- **Alias methods**: `saveSession`, `loadSession`, `deleteSession`, `getLatestSessionId`.

### 18.7 CustomProvidersStore (`storage/stores/custom-providers-store.ts`)
- Supports auto-discovery types (Ollama, llama.cpp, vLLM, LM Studio) and manual types (OpenAI-compatible, Anthropic-compatible).
- Methods: `get`, `set`, `delete`, `getAll`, `has`.

### 18.8 AppStorage (`storage/app-storage.ts`)
- High-level API composing all stores + backend.
- `getQuotaInfo()`, `requestPersistence()`.
- Global singleton via `setAppStorage()` / `getAppStorage()`.

### 18.9 Types (`storage/types.ts`)
- `SessionMetadata` — Lightweight session info for listing (id, title, timestamps, messageCount, usage, thinkingLevel, preview).
- `SessionData` — Full session data (id, title, model, thinkingLevel, messages, timestamps).
- `IndexedDBConfig`, `StoreConfig`, `IndexConfig` — Schema configuration types.

---

## 19. Tool System (`tools/`)

### 19.1 Tool Renderer Registry (`tools/renderer-registry.ts`)
- **`registerToolRenderer(toolName, renderer)`** — Register a custom renderer for any tool.
- **`getToolRenderer(toolName)`** — Look up renderer.
- **`renderCollapsibleHeader()`** — Collapsible header with icon, text, state indicator (spinner/checkmark/error), and expand/collapse toggle.
- **`renderHeader()`** — Simple header with icon, text, state indicator.

### 19.2 Unified Tool Renderer (`tools/index.ts`)
- **`renderTool(toolName, params, result, isStreaming)`** — Unified rendering function.
- **`setShowJsonMode(enabled)`** — Force all tools to use the default JSON renderer.
- Auto-registers built-in renderers (javascript_repl, extract_document, bash).

### 19.3 JavaScript REPL Tool (`tools/javascript-repl.ts`)
- **`createJavaScriptReplTool()`** — Creates the JavaScript REPL agent tool:
  - Sandboxed execution via `SandboxIframe.execute()`.
  - `runtimeProvidersFactory` — Configure runtime providers (attachments, artifacts).
  - `sandboxUrlProvider` — For browser extensions.
  - Returns files as base64 in details.
- **`executeJavaScript(code, providers, signal, sandboxUrlProvider)`** — Standalone execution function.
- **`javascriptReplRenderer`** — Tool renderer showing code block, console output, and generated file attachments.

### 19.4 Extract Document Tool (`tools/extract-document.ts`)
- **`createExtractDocumentTool()`** — Fetches and extracts text from document URLs:
  - Supports PDF, DOCX, XLSX, PPTX.
  - 50MB size limit.
  - **CORS proxy fallback** — Tries direct fetch, falls back to proxy on CORS error.
  - Uses `loadAttachment()` for processing.
  - User-friendly error messages with instructions.
- **`extractDocumentRenderer`** — Shows URL, extracted text, and status.

### 19.5 Artifacts Tool (`tools/artifacts/artifacts.ts`)
- **`<artifacts-panel>`** — Full artifact management panel:
  - **Tab bar** — Scrollable file tabs with active indicator.
  - **Content area** — Dynamically created and managed `ArtifactElement` instances.
  - **Commands**: create, update (string replacement), rewrite, get, delete, logs.
  - **HTML artifact logs** — Captures console output from HTML artifacts via `waitForHtmlExecution()`.
  - **Artifact reconstruction** — `reconstructFromMessages()` rebuilds artifact state from conversation history.
  - **Cross-artifact dependency** — Reloads all HTML artifacts when any artifact changes (they can read each other).
  - **Download button** on active artifact (delegates to artifact element).
  - **Close button**.
  - **Overlay mode** — Full-screen for mobile.
  - **Collapsed mode** — Hidden with floating reopen pill.
  - **Runtime providers** — HTML artifacts get read-only access to other artifacts and attachments.

### 19.6 Artifact Element Types (`tools/artifacts/`)

| Element | File | Renderer |
|---------|------|----------|
| `<html-artifact>` | `HtmlArtifact.ts` | Sandboxed iframe preview with console logs. Preview/Code toggle. Refresh, Copy, Download buttons. |
| `<svg-artifact>` | `SvgArtifact.ts` | SVG preview via `Blob URL`. Preview/Code toggle. Copy, Download buttons. |
| `<markdown-artifact>` | `MarkdownArtifact.ts` | Markdown rendered via `<markdown-block>`. Preview/Code toggle. Copy, Download buttons. |
| `<image-artifact>` | `ImageArtifact.ts` | Image display with responsive sizing. Download button. |
| `<pdf-artifact>` | `PdfArtifact.ts` | PDF rendered page-by-page via pdfjs-dist (1.5x scale). Download button. |
| `<docx-artifact>` | `DocxArtifact.ts` | DOCX rendered via docx-preview. Download button. |
| `<excel-artifact>` | `ExcelArtifact.ts` | Excel as HTML tables with multi-sheet tabs. Download button. |
| `<text-artifact>` | `TextArtifact.ts` | Syntax-highlighted code display (supports many languages). Copy, Download buttons. |
| `<generic-artifact>` | `GenericArtifact.ts` | Fallback: filename display with download button and "Preview not available" message. |

- All extend `ArtifactElement` abstract class with `filename`, `content`, `getHeaderButtons()`.

### 19.7 Artifact Console (`tools/artifacts/Console.ts`)
- **`<artifact-console>`** — Console log viewer for HTML artifacts.
- Expandable panel with log count and error count.
- Autoscroll toggle, copy logs button.
- Color-coded log vs error entries.

### 19.8 ArtifactPill (`tools/artifacts/ArtifactPill.ts`)
- Clickable inline artifact filename badge (opens artifact in panel).

### 19.9 ArtifactsToolRenderer (`tools/artifacts/artifacts-tool-renderer.ts`)
- Renders artifact tool calls with:
  - Collapsible header showing command + filename pill.
  - Code display with syntax highlighting (language detection from extension).
  - Diff view for `update` commands.
  - Console block for HTML artifact execution logs.
  - Error state handling.

### 19.10 Built-in Tool Renderers (`tools/renderers/`)

| Renderer | File | Description |
|----------|------|-------------|
| `BashRenderer` | `BashRenderer.ts` | Shows bash command + console output. SquareTerminal icon. |
| `CalculateRenderer` | `CalculateRenderer.ts` | Shows expression = result inline. Calculator icon. |
| `GetCurrentTimeRenderer` | `GetCurrentTimeRenderer.ts` | Shows timezone info + result. Clock icon. |
| `DefaultRenderer` | `DefaultRenderer.ts` | Fallback: JSON params + output display. Code icon. |

### 19.11 Tool Types (`tools/types.ts`)
- `ToolRenderResult` — `{ content: TemplateResult, isCustom: boolean }`
- `ToolRenderer<TParams, TDetails>` — `render(params, result, isStreaming) => ToolRenderResult`

---

## 20. Utility Modules (`utils/`)

### 20.1 Attachment Utils (`utils/attachment-utils.ts`)
- **`loadAttachment(source, fileName?)`** — Load files from URL, File, Blob, or ArrayBuffer.
  - **PDF processing** — Extracts text with page XML structure (`<pdf><page number="1">...</page></pdf>`), generates preview image.
  - **DOCX processing** — Extracts text with structure, handles paragraphs, tables, runs.
  - **PPTX processing** — Extracts text from slide XML (`<a:t>` tags), slide notes, via JSZip.
  - **Excel processing** — Extracts text as CSV per sheet via SheetJS/xlsx.
  - **Image processing** — Detects by MIME type.
  - **Text processing** — Detects by extension (txt, md, json, xml, html, css, js, ts, etc.).
- `Attachment` interface: `id`, `type` (image|document), `fileName`, `mimeType`, `size`, `content` (base64), `extractedText?`, `preview?`.

### 20.2 Auth Token (`utils/auth-token.ts`)
- **`getAuthToken()`** — Prompts for auth token via dialog, stores in localStorage.
- **`clearAuthToken()`** — Removes from localStorage.

### 20.3 Format Utils (`utils/format.ts`)
- **`formatCost(cost)`** — `$0.0000` format.
- **`formatModelCost(cost)`** — `$input/$output` format with appropriate precision, "Free" for zero.
- **`formatUsage(usage)`** — Compact usage string: `↑1.2k ↓3.4k R500 W200 $0.0123`.
- **`formatTokenCount(count)`** — Human-readable token count: `123`, `1.2k`, `15k`.

### 20.4 Internationalization (`utils/i18n.ts`)
- **Full i18n support** via `@mariozechner/mini-lit/i18n`.
- **`translations`** — `en` (default) and `de` (German) translations for ~200 UI strings.
- **`setLanguage(lang)`** — Switch language.
- **`i18n(key)`** — Translate a string.
- Covers all UI text: messages, buttons, dialogs, tool renderers, artifacts, settings, storage, etc.

### 20.5 Model Discovery (`utils/model-discovery.ts`)

| Provider | Function | Method |
|----------|----------|--------|
| Ollama | `discoverOllamaModels()` | Uses `ollama/browser` SDK `list()` + `show()`. Filters for tool-supporting models. |
| llama.cpp | `discoverLlamaCppModels()` | Fetches `/v1/models` endpoint. |
| vLLM | `discoverVLLMModels()` | Fetches `/v1/models` endpoint. |
| LM Studio | `discoverLMStudioModels()` | Uses `@lmstudio/sdk` WebSocket client `listDownloadedModels()`. |

- **`discoverModels(type, baseUrl, apiKey)`** — Unified discovery function.

### 20.6 Proxy Utils (`utils/proxy-utils.ts`)
- **`shouldUseProxyForProvider(provider, apiKey)`** — Decision logic:
  - Z-AI: always needs proxy.
  - Anthropic OAuth tokens (`sk-ant-oat-*`): needs proxy.
  - OpenAI Codex: needs proxy.
  - Others (OpenAI, Google, Groq, etc.): no proxy.
- **`applyProxyIfNeeded(model, apiKey, proxyUrl)`** — Modifies model `baseUrl` to go through CORS proxy.
- **`isCorsError(error)`** — Detects CORS errors from browser fetch failures.
- **`createStreamFn(getProxyUrl)`** — Creates a streaming function that reads proxy settings from storage and applies them per-request.

### 20.7 Test Sessions (`utils/test-sessions.ts`)
- **`simpleHtml`** — Test fixture with a minimal HTML artifact creation conversation.
- **`longSession`** — Extended test fixture with multiple turns, tool calls, error handling, and React interaction simulation.

---

## 21. Styling (`app.css`)

- **Tailwind CSS v4** — Imported and compiled.
- **Claude theme** from mini-lit.
- **Custom scrollbar styles** — Thin, themed scrollbars.
- **Dialog close button cursor fix**.
- **Shimmer animation** — For thinking block streaming text.
- **User message styling** — Gradient background with orange tones, blur backdrop, border.

---

## 22. Package Config (`package.json`)

- **Dependencies**: `@earendil-works/pi-ai`, `@earendil-works/pi-tui`, `@lmstudio/sdk`, `ollama`, `typebox`, `lucide`, `pdfjs-dist`, `docx-preview`, `jszip`, `xlsx`.
- **Peer dependencies**: `@mariozechner/mini-lit`, `lit`.
- **Keywords**: ai, chat, ui, components, llm, web-components, mini-lit.
- **Exports**: `dist/index.js` (main), `dist/app.css` (styles).

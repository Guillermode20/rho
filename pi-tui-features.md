# @earendil-works/pi-tui — Comprehensive Feature Analysis

Package version: `0.75.3` — TypeScript Terminal User Interface library with differential rendering.

Source: `pi/packages/tui/src/` (16 source files across flat + components directory)

---

## 1. CORE TUI ENGINE (`tui.ts`)

### 1.1 Differential Rendering System
- **Three-strategy rendering pipeline** optimizes output to only what changed:
  - **First Render**: Output all lines without clearing scrollback.
  - **Width Changed or Change Above Viewport**: Full screen clear + complete re-render.
  - **Normal Update**: Move cursor to first changed line, clear to end, render only changed lines.
- **Synchronized output** via CSI `?2026h` / `?2026l` for atomic, flicker-free screen updates.
- **Kitty image-aware diffing**: Expands changed region to include lines containing Kitty image sequences, and deletes stale images before re-render.
- **Debug logging**: `PI_DEBUG_REDRAW` env var logs redraw reasons; `PI_TUI_DEBUG=1` dumps full render state to `/tmp/tui/`.

### 1.2 Component System
- **`Component` interface** — base contract with `render(width: number): string[]`, optional `handleInput(data: string)`, `invalidate()`, and `wantsKeyRelease` flag.
- **`Container`** — groups child components, delegates render/invalidate to children.
- **`TUI`** — extends `Container`, manages component life cycle, overlays, focus, and input routing.
- **`Focusable` interface** — components that display a hardware cursor. TUI sets `focused` property and scans rendered output for `CURSOR_MARKER` (a zero-width APC sequence) to position the terminal cursor for IME (CJK input method) candidate window positioning.
- **`isFocusable()`** type guard.
- **`CURSOR_MARKER`** — `\x1b_pi:c\x07` sentinel for IME cursor positioning.

### 1.3 Overlay System
- **`showOverlay(component, options?)`** — renders components on top of existing content without replacing it.
- **Overlay positioning** — supports:
  - **Anchor-based**: 9 positions (center, top-left, top-right, bottom-left, bottom-right, top-center, bottom-center, left-center, right-center).
  - **Percentage-based**: `row: "25%"`, `col: "50%"` relative to terminal dimensions.
  - **Absolute positioning**: exact `row`/`col` values.
  - **Offsets**: `offsetX`, `offsetY` from anchor point.
- **Overlay sizing** — `width` (number or percentage), `minWidth`, `maxHeight` (number or percentage).
- **Overlay margins** — `margin` (uniform number or `{top, right, bottom, left}` object) clamped to terminal bounds.
- **Responsive visibility** — `visible: (termWidth, termHeight) => boolean` callback evaluated each frame.
- **`nonCapturing` option** — overlay does not capture keyboard focus when shown.
- **`OverlayHandle`** — returned from `showOverlay()` with: `hide()`, `setHidden(bool)`, `isHidden()`, `focus()`, `unfocus()`, `isFocused()`.
- **`hideOverlay()`** — hides the topmost overlay and restores previous focus.
- **`hasOverlay()`** — checks if any visible overlay is active.
- **Focus ordering** — overlay stack with `focusOrder` counter; topmost visible capturing overlay receives focus.
- **Composite rendering** — overlays are composited onto base content with single-pass line compositing that preserves ANSI styling before/after overlay regions.

### 1.4 Focus Management
- **`setFocus(component | null)`** — sets/clears focused component, manages `focused` flag on `Focusable` components.
- **Input routing** — routes keyboard input to the currently focused component.
- **Key release filtering** — components can opt into key release events via `wantsKeyRelease`.
- **Visibility-aware focus** — when a focused overlay becomes invisible (via resize or `visible()` callback), focus is redirected to the topmost visible overlay or restored to `preFocus`.
- **Global debug key** — `Shift+Ctrl+D` invokes `tui.onDebug` callback.

### 1.5 Input Listeners
- **`addInputListener(listener)`** — register listeners that can consume or transform input before it reaches the focused component.
- **`removeInputListener(listener)`** — unregister a listener.
- **Listener return type** — `{ consume?: boolean; data?: string }` allows interception and modification.

### 1.6 Render Scheduling
- **Throttled rendering** — minimum 16ms between renders (60fps cap).
- **`requestRender(force?)`** — schedules a render on the next tick. With `force=true`, clears all state and schedules immediate re-render.
- **`fullRedraws` getter** — exposes count of full screen redraws for diagnostics.

### 1.7 Hardware Cursor Control
- **`showHardwareCursor`** — configurable via constructor arg or `PI_HARDWARE_CURSOR=1` env var.
- **`setShowHardwareCursor(enabled)`** — dynamic toggle.
- **IME cursor positioning** — after each render, positions the hardware cursor at the `CURSOR_MARKER` location for CJK IME candidate window placement.

### 1.8 Terminal Resize Handling
- **Automatic re-render** on terminal resize events.
- **Width changes** trigger full screen clear + re-render.
- **Height changes** trigger full re-render (except in Termux to avoid keyboard toggle flicker).
- **`setClearOnShrink(bool)`** — when enabled (default: `PI_CLEAR_ON_SHRINK=1`), triggers full re-render when content shrinks to clear empty rows.

---

## 2. TERMINAL INTERFACE (`terminal.ts`)

### 2.1 `Terminal` Interface
Abstract contract for terminal backends:

| Method | Purpose |
|--------|---------|
| `start(onInput, onResize)` | Initialize terminal with input and resize handlers |
| `stop()` | Shutdown and restore terminal state |
| `drainInput(maxMs?, idleMs?)` | Drain pending input before exit (Kitty key release cleanup) |
| `write(data)` | Write output to terminal |
| `columns` / `rows` | Get terminal dimensions |
| `kittyProtocolActive` | Whether Kitty keyboard protocol is active |
| `moveBy(lines)` | Move cursor up/down by N lines |
| `hideCursor()` / `showCursor()` | Cursor visibility control |
| `clearLine()` / `clearFromCursor()` / `clearScreen()` | Screen clearing operations |
| `setTitle(title)` | Set terminal window title via OSC 0 |
| `setProgress(active)` | Show/hide indeterminate progress indicator (OSC 9;4) |

### 2.2 `ProcessTerminal` — Real Terminal Implementation
- **Raw mode** — enables `process.stdin` raw mode.
- **Bracketed paste mode** — enables `\x1b[?2004h` / `\x1b[?2004l`.
- **Kitty keyboard protocol** — queries and enables protocol:
  - Sends `\x1b[?u` to query terminal.
  - On detection, enables flags 1 (disambiguate), 2 (event types), 4 (alternate keys with base layout support for non-Latin keyboards).
  - Falls back to xterm `modifyOtherKeys` mode 2 if no response within 150ms (for tmux compatibility).
- **StdinBuffer integration** — buffers stdin to split batched input into individual sequences.
- **Windows VT input** — uses `koffi` (optional native dependency) to call `SetConsoleMode` with `ENABLE_VIRTUAL_TERMINAL_INPUT` for proper Shift+Tab and modifier detection.
- **Resize handling** — listens to `process.stdout.resize` event; sends `SIGWINCH` on start for fresh dimensions.
- **Input drain** — on shutdown, disables Kitty protocol and drains input for up to 1s to prevent key releases leaking to parent shell.
- **Write logging** — `PI_TUI_WRITE_LOG` env var captures raw ANSI output to file.
- **Progress indicator** — OSC 9;4 with keepalive interval (1s).

### 2.3 `VirtualTerminal` (Testing)
- Referenced in README as using `@xterm/headless` for testing.

---

## 3. KEYBOARD INPUT HANDLING (`keys.ts`)

### 3.1 `Key` Helper Object
Type-safe key identifier builder with autocomplete:

**Special keys**: `escape`, `enter`, `tab`, `space`, `backspace`, `delete`, `insert`, `clear`, `home`, `end`, `pageUp`, `pageDown`, `up`, `down`, `left`, `right`, `f1`–`f12`.

**Symbol keys**: `backtick`, `hyphen`, `equals`, `leftbracket`, `rightbracket`, `backslash`, `semicolon`, `quote`, `comma`, `period`, `slash`, `exclamation`, `at`, `hash`, `dollar`, `percent`, `caret`, `ampersand`, `asterisk`, `leftparen`, `rightparen`, `underscore`, `plus`, `pipe`, `tilde`, `leftbrace`, `rightbrace`, `colon`, `lessthan`, `greaterthan`, `question`.

**Single modifiers**: `Key.ctrl("c")`, `Key.shift("tab")`, `Key.alt("x")`, `Key.super("k")`.

**Combined modifiers**: `Key.ctrlShift("p")`, `Key.ctrlAlt("x")`, `Key.ctrlSuper("k")`, `Key.shiftAlt("x")`, `Key.altSuper("k")`, `Key.shiftSuper("k")`.

**Triple modifiers**: `Key.ctrlShiftAlt("x")`, `Key.ctrlShiftSuper("k")`.

### 3.2 Key Matching
- **`matchesKey(data, keyId)`** — core matching function supporting:
  - Legacy terminal sequences (VT100/ANSI).
  - **Kitty keyboard protocol** (CSI-u format with flags 1, 2, 4).
  - **xterm modifyOtherKeys** format (CSI 27).
  - Numpad key equivalents.
  - Windows Terminal raw byte heuristics (0x08 for Ctrl+Backspace).
  - Base layout key fallback for non-Latin keyboard layouts (Cyrillic, etc.).
- **`parseKey(data)`** — returns parsed key identifier string or undefined.
- **`setKittyProtocolActive(active)`** / `isKittyProtocolActive()` — global protocol state.
- **`isKeyRelease(data)`** / `isKeyRepeat(data)` — detect event types from Kitty flag 2.
- **`decodeKittyPrintable(data)`** / `decodeModifyOtherKeysPrintable(data)` / `decodePrintableKey(data)` — extract printable characters from CSI-u sequences.

### 3.3 Event Type Detection
- **`KeyEventType`** — `"press"` | `"repeat"` | `"release"`.
- **`_lastEventType`** tracking for release/repeat queries.
- **Paste content protection** — bracketed paste markers prevent false release detection.

---

## 4. TERMINAL IMAGE SUPPORT (`terminal-image.ts`)

### 4.1 Terminal Capability Detection
- **`TerminalCapabilities`** — `{ images: ImageProtocol; trueColor: boolean; hyperlinks: boolean }`.
- **`detectCapabilities()`** — auto-detects based on environment variables and terminal programs:
  - **Kitty protocol**: Kitty, Ghostty, WezTerm.
  - **iTerm2 protocol**: iTerm.app.
  - **Hyperlinks (OSC 8)**: Active for Kitty, Ghostty, WezTerm, iTerm2, VSCode.
  - **tmux/screen guard**: Forces images=null, hyperlinks=false for multiplexers.
- **`getCapabilities()` / `setCapabilities()` / `resetCapabilitiesCache()`** — cached access with override support.

### 4.2 Image Encoding
- **`encodeKitty(base64, options)`** — Kitty graphics protocol (APC `\x1b_G`), supports:
  - Chunked transmission (4096-byte chunks) for large images.
  - Image ID assignment and reuse.
  - Cursor movement control (`moveCursor`).
  - Width/height in cells.
- **`encodeITerm2(base64, options)`** — iTerm2 inline image protocol (OSC 1337), supports:
  - Width/height (number or percentage).
  - Preserve aspect ratio control.
  - Named images.
- **`allocateImageId()`** — random ID in range [1, 0xffffffff] to avoid collisions.

### 4.3 Image Deletion
- **`deleteKittyImage(imageId)`** — delete specific image by ID.
- **`deleteAllKittyImages()`** — delete all visible Kitty images.

### 4.4 Image Dimension Parsing
Automatic dimension extraction from base64-encoded image data:
- **`getPngDimensions()`** — reads IHDR chunk (width at offset 16, height at 20).
- **`getJpegDimensions()`** — scans SOF0/SOF1/SOF2 markers (0xFF 0xCx).
- **`getGifDimensions()`** — reads Logical Screen Descriptor (width at offset 6, height at 8).
- **`getWebpDimensions()`** — handles VP8, VP8L, VP8X chunks.
- **`getImageDimensions(base64, mimeType)`** — dispatcher by MIME type.

### 4.5 Cell Size Management
- **`CellDimensions`** — `{ widthPx, heightPx }` for pixel-to-cell conversion.
- **`getCellDimensions()` / `setCellDimensions()`** — defaults to 9x18 pixels, updated by TUI via CSI 16 t query.
- **`calculateImageCellSize()`** — computes columns/rows from image dimensions, respecting max width/height and aspect ratio.
- **`calculateImageRows()`** — computes row count for a given target width.

### 4.6 Image Rendering
- **`renderImage(base64, dimensions, options)`** — generates terminal sequence for the best available protocol.
- **`imageFallback(mimeType, dimensions, filename)`** — text placeholder for unsupported terminals.
- **`isImageLine(line)`** — detects image sequences in rendered lines.

### 4.7 Hyperlink Support
- **`hyperlink(text, url)`** — wraps text in OSC 8 hyperlink sequence.
- Detects terminal capability to decide whether to render inline URL or use hyperlinks.

---

## 5. UTILITY FUNCTIONS (`utils.ts`)

### 5.1 Width Calculation
- **`visibleWidth(str)`** — calculates visible terminal width:
  - Fast path for pure ASCII printable strings.
  - LRU cache (512 entries) for non-ASCII strings.
  - Strips ANSI/OSC/APC escape sequences.
  - Expands tabs to 3 spaces.
  - Grapheme-aware via `Intl.Segmenter`.
  - Emoji width (2) with pre-filter optimization.
  - East Asian width via `get-east-asian-width` library.
  - Handles regional indicators, VS16, ZWJ sequences, Thai/Lao AM vowels.

### 5.2 Text Truncation
- **`truncateToWidth(text, maxWidth, ellipsis?, pad?)`** — truncates text to fit:
  - Preserves ANSI escape codes.
  - Customizable ellipsis (default: `"..."`).
  - Optional padding to exact width.
  - Grapheme-aware truncation.

### 5.3 Text Wrapping
- **`wrapTextWithAnsi(text, width)`** — word-wrap with ANSI preservation:
  - Word-aware wrapping (breaks at word boundaries).
  - Falls back to character-level wrapping for long words.
  - Preserves active ANSI styles across line breaks.
  - Handles tabs, newlines, and embedded ANSI codes.
  - Uses `AnsiCodeTracker` to track and reapply styling.

### 5.4 ANSI Processing
- **`extractAnsiCode(str, pos)`** — extracts CSI, OSC, and APC sequences.
- **`AnsiCodeTracker`** — tracks active SGR attributes (bold, dim, italic, underline, blink, inverse, hidden, strikethrough, fg/bg colors including 256-color and RGB), and OSC 8 hyperlinks.
- **`normalizeTerminalOutput(str)`** — normalizes Thai/Lao AM vowels (U+0E33 → U+0E4D+U+0E32, U+0EB3 → U+0ECD+U+0EB2) to prevent stale-cell artifacts.
- **`applyBackgroundToLine(line, width, bgFn)`** — applies background color with padding.
- **`sliceByColumn(line, startCol, length, strict?)`** — extracts a range of visible columns, handling ANSI codes and wide chars.
- **`sliceWithWidth(line, startCol, length, strict?)`** — like `sliceByColumn` but returns both text and actual width.
- **`extractSegments(line, beforeEnd, afterStart, afterLen)`** — single-pass extraction of "before" and "after" segments for overlay compositing, preserving styling inheritance.

### 5.5 Character Classification
- **`isWhitespaceChar(char)`** — whitespace detection.
- **`isPunctuationChar(char)`** — punctuation detection.
- **`getSegmenter()`** — shared `Intl.Segmenter` instance.

---

## 6. STDIN BUFFERING (`stdin-buffer.ts`)

### 6.1 Sequence Buffering
- **`StdinBuffer`** — extends `EventEmitter`, buffers partial stdin data chunks:
  - Detects complete escape sequences (CSI, OSC, DCS, APC, SS3).
  - Handles partial sequences arriving across multiple `data` events.
  - Configurable timeout (default: 10ms) to flush incomplete sequences.
- **`process(data)`** — feeds input data to the buffer.
- **`flush()`** — returns any remaining buffered data.
- **`clear()` / `destroy()`** — cleanup methods.

### 6.2 Bracketed Paste Handling
- Detects `\x1b[200~` / `\x1b[201~` markers.
- Emits `paste` event with complete paste content.
- Handles paste content split across multiple chunks.
- **Deduplication**: Suppresses duplicate Kitty printable sequences (same codepoint arriving both as raw char and CSI-u).

### 6.3 Sequence Completion Detection
- **`isCompleteSequence(data)`** — classifies data as `"complete"`, `"incomplete"`, or `"not-escape"`.
- Handles CSI, OSC, DCS, APC, SS3 sequences.
- Special handling for SGR mouse sequences (SGR format validation).
- Special handling for meta-key sequences (`\x1b\x1b` followed by CSI/OSC/SS3/DCS/APC).

---

## 7. FUZZY MATCHING (`fuzzy.ts`)

### 7.1 Fuzzy Matching
- **`fuzzyMatch(query, text)`** — checks if all query characters appear in order (not necessarily consecutive):
  - Word boundary bonus (starts after `/`, `-`, `_`, `.`, `:`, whitespace).
  - Consecutive match bonus.
  - Gap penalty.
  - Position penalty (later matches cost more).
  - Exact match gets max bonus (-100 score = best).
  - **Digit-Letter swapping** — auto-tries swapped query (e.g., `test123` ↔ `123test`).

### 7.2 Fuzzy Filtering
- **`fuzzyFilter(items, query, getText)`** — filters and sorts items by match quality:
  - **Multi-token support** — space-separated tokens: all tokens must match.
  - Returns items sorted by score (best first).

---

## 8. AUTOCOMPLETE (`autocomplete.ts`)

### 8.1 Autocomplete Provider Interface
- **`AutocompleteProvider`** interface with:
  - `getSuggestions(lines, cursor, options)` — returns suggestions or null.
  - `applyCompletion(lines, cursor, item, prefix)` — applies selected completion.
  - `shouldTriggerFileCompletion?(lines, cursor)` — optional file completion trigger check.

### 8.2 `CombinedAutocompleteProvider`
- **Dual support**: slash commands + file path completion.
- **Slash command autocomplete**:
  - Type `/` to see available commands.
  - Fuzzy filtering over command names.
  - Command argument completion via `getArgumentCompletions()`.
  - Appends a space after completing to continue typing arguments.
- **File path autocomplete**:
  - Uses `readdirSync` for fast directory listing.
  - Supports `~/`, `./`, `../`, and absolute `/` paths.
  - **`@` prefix** — special token for file attachments.
  - **Quoted path support** — paths with spaces in quotes (`"path/with spaces/file"`).
  - **`fd` integration** — optional fast directory walker (respects `.gitignore`), enabled via `fdPath` constructor arg.
  - Fuzzy scoring: exact filename > starts with > substring > path substring.
  - Directories sorted first, then alphabetically.
  - Directories get `/` suffix, trailing space only for non-directory completions.
- **Completion application**:
  - Different insertion logic for slash commands vs file paths vs attachments.
  - Handles quote management (trailing quotes after cursor).

### 8.3 Supporting Types
- **`AutocompleteItem`** — `{ value, label, description? }`.
- **`SlashCommand`** — `{ name, description?, argumentHint?, getArgumentCompletions? }`.
- **`AutocompleteSuggestions`** — `{ items, prefix }`.
- **Path parsing**: `findLastDelimiter`, `findUnclosedQuoteStart`, `extractQuotedPrefix`, `parsePathPrefix`.

---

## 9. KEYBINDINGS (`keybindings.ts`)

### 9.1 Keybinding Registry
- **`Keybinding`** type — union of all registered action names.
- **`TUI_KEYBINDINGS`** — default registry with ~30 editor/input/selection actions.
- **`KeybindingDefinitions`** — maps action IDs to `{ defaultKeys, description? }`.

### 9.2 `KeybindingsManager`
- **`constructor(definitions, userBindings?)`** — creates manager with optional user overrides.
- **`matches(data, keybinding)`** — checks if input matches any key of a keybinding.
- **`getKeys(keybinding)`** — returns resolved keys for a keybinding.
- **`getDefinition(keybinding)`** — returns the definition.
- **`getConflicts()`** — detects when user binds the same key to multiple actions.
- **`setUserBindings()` / `getUserBindings()`** — user customization.
- **`getResolvedBindings()`** — returns final effective binding map.
- **Duplicate deduplication** within each keybinding's key list.

### 9.3 Default Keybindings

| Action | Default Keys |
|--------|-------------|
| `tui.editor.cursorUp` | Up |
| `tui.editor.cursorDown` | Down |
| `tui.editor.cursorLeft` | Left, Ctrl+B |
| `tui.editor.cursorRight` | Right, Ctrl+F |
| `tui.editor.cursorWordLeft` | Alt+Left, Ctrl+Left, Alt+B |
| `tui.editor.cursorWordRight` | Alt+Right, Ctrl+Right, Alt+F |
| `tui.editor.cursorLineStart` | Home, Ctrl+A |
| `tui.editor.cursorLineEnd` | End, Ctrl+E |
| `tui.editor.jumpForward` | Ctrl+] |
| `tui.editor.jumpBackward` | Ctrl+Alt+] |
| `tui.editor.pageUp` | PageUp |
| `tui.editor.pageDown` | PageDown |
| `tui.editor.deleteCharBackward` | Backspace |
| `tui.editor.deleteCharForward` | Delete, Ctrl+D |
| `tui.editor.deleteWordBackward` | Ctrl+W, Alt+Backspace |
| `tui.editor.deleteWordForward` | Alt+D, Alt+Delete |
| `tui.editor.deleteToLineStart` | Ctrl+U |
| `tui.editor.deleteToLineEnd` | Ctrl+K |
| `tui.editor.yank` | Ctrl+Y |
| `tui.editor.yankPop` | Alt+Y |
| `tui.editor.undo` | Ctrl+- |
| `tui.input.newLine` | Shift+Enter |
| `tui.input.submit` | Enter |
| `tui.input.tab` | Tab |
| `tui.input.copy` | Ctrl+C |
| `tui.select.up` | Up |
| `tui.select.down` | Down |
| `tui.select.pageUp` | PageUp |
| `tui.select.pageDown` | PageDown |
| `tui.select.confirm` | Enter |
| `tui.select.cancel` | Escape, Ctrl+C |

### 9.4 Global State
- **`setKeybindings(manager)` / `getKeybindings()`** — singleton accessor, lazily initializes with defaults.

---

## 10. KILL RING (`kill-ring.ts`)

### 10.1 Emacs-Style Kill/Yank
- **`KillRing`** — ring buffer for killed/deleted text entries.
- **`push(text, { prepend, accumulate })`**:
  - `accumulate=true` merges with most recent entry (for consecutive kills).
  - `prepend=true` for backward deletion, `false` for forward deletion.
- **`peek()`** — returns most recent entry without modifying ring.
- **`rotate()`** — moves last entry to front (for yank-pop cycling).
- **`length`** — number of entries.

---

## 11. UNDO STACK (`undo-stack.ts`)

### 11.1 Generic Undo Support
- **`UndoStack<S>`** — generic clone-on-push undo stack.
- **`push(state)`** — stores a `structuredClone` of the state.
- **`pop()`** — returns most recent snapshot or undefined.
- **`clear()`** — removes all snapshots.
- **`length`** — snapshot count.

---

## 12. EDITOR COMPONENT INTERFACE (`editor-component.ts`)

### 12.1 `EditorComponent` Interface
Extension point for custom editor implementations:

- **Core**: `getText()`, `setText(text)`, `handleInput(data)`.
- **Callbacks**: `onSubmit`, `onChange`.
- **Optional history**: `addToHistory(text)`.
- **Optional text manipulation**: `insertTextAtCursor(text)`, `getExpandedText()`.
- **Optional autocomplete**: `setAutocompleteProvider(provider)`.
- **Optional appearance**: `borderColor`, `setPaddingX(n)`, `setAutocompleteMaxVisible(n)`.

---

## 13. BUILT-IN COMPONENTS

### 13.1 `Text` (`components/text.ts`)
- Multi-line text display with word wrapping.
- Configurable padding (X/Y).
- Custom background function (`setCustomBgFn`).
- Tab expansion (3 spaces).
- Render caching (text + width as cache key).
- Padding to exact width.

### 13.2 `TruncatedText` (`components/truncated-text.ts`)
- Single-line text that truncates to fit viewport.
- Picks first line before any newline.
- Configurable padding (X/Y).
- Pads to exact width.

### 13.3 `Input` (`components/input.ts`)
- Single-line text input with horizontal scrolling.
- **Focusable** — implements `Focusable` with `CURSOR_MARKER` for IME support.
- **Key bindings**: All editor navigation/deletion keys (via keybindings system).
- **Undo support** via `UndoStack<InputState>`.
- **Kill ring** — yank (`Ctrl+Y`) and yank-pop (`Alt+Y`).
- **Bracketed paste** handling.
- **Grapheme-aware** cursor movement/backspace (emojis, combining chars).
- **Kitty CSI-u printable** decoding for terminal compatibility.
- **Emacs-style word movements** — punctuation-aware word boundaries.
- **Callbacks**: `onSubmit`, `onEscape`.
- **Horizontal scrolling** when content exceeds width, with centered cursor region.
- **Fake cursor rendering** using reverse video (`\x1b[7m`).

### 13.4 `Editor` (`components/editor.ts`)
Multi-line text editor — the most complex component (~2293 lines).

**Core features:**
- Multi-line editing with word wrap (word-aware line breaking).
- Word wrapping with paste-marker-aware grapheme segmentation.
- Vertical scrolling when content exceeds 30% of terminal height.
- Scrolling indicators (`↑ N more` / `↓ N more`).
- Hardware cursor (IME support via `CURSOR_MARKER`).
- **Focusable** implementation with `focused` flag.

**Text manipulation:**
- Character insertion with undo coalescing (fish-style: consecutive word chars = 1 undo unit, space captures state before itself).
- Grapheme-aware backspace/forward delete.
- Word-aware backward/forward deletion (with kill ring integration).
- Delete to line start/end (kill ring integration).
- Yank (`Ctrl+Y`) / yank-pop (`Alt+Y`).
- Undo (`Ctrl+-`) via structured clone.

**Cursor movement:**
- Arrow keys (vertical with sticky column logic).
- Word left/right (punctuation-aware).
- Line start/end.
- Character jump (forward `Ctrl+]` / backward `Ctrl+Alt+]` — awaits next keypress, jumps to first occurrence).
- Page up/down.
- **Sticky column** for vertical movement with clamped/rewrapped edge cases (7-case decision table).
- **Paste marker snapping** — cursor snaps to start/end of paste marker segments.

**History & Navigation:**
- Up/down arrow prompt history navigation.
- History browsing mode (captures state on entry, restores on exit).
- Max 100 history entries, no consecutive duplicates.

**Autocomplete integration:**
- Auto-triggers on `/` (slash commands), `@`/`#` (symbol completions).
- Updates on typing/deletion in completable contexts.
- Tab key triggers file completion.
- SelectList-based dropdown for autocomplete suggestions.
- Abort controller for cancelling in-flight autocomplete requests.
- Debounced autocomplete (20ms for attachment completions).

**Paste handling:**
- Bracketed paste mode with buffering.
- **Large paste markers** — pastes >10 lines or >1000 chars get a `[paste #N +M lines]` or `[paste #N X chars]` marker.
- Paste content stored in `Map<number, string>` for expansion.
- `getExpandedText()` expands markers for external processing.
- CSI-u control byte decoding inside pastes (tmux compatibility).
- Smart space insertion when pasting paths after word characters.
- Normalizes line endings and expands tabs (4 spaces).

**Submit behavior:**
- `Enter` submits (configurable `disableSubmit` flag).
- `Shift+Enter` / `Ctrl+Enter` / `Alt+Enter` inserts newline.
- Backslash-Enter workaround: if char before cursor is `\`, deletes it and inserts newline.
- Submit expands paste markers and trims result.
- Clears editor state after submit (undo stack too).

**Rendering:**
- Top border (`─` repeated to width).
- Bottom border.
- Scroll indicators with line counts.
- Autocomplete rendered beneath editor.
- Padding config (dynamic `setPaddingX`).
- Max visible lines: 30% of terminal height (min 5).

**Dynamic configuration:**
- `setPaddingX(n)`, `getPaddingX()`.
- `setAutocompleteMaxVisible(n)`, `getAutocompleteMaxVisible()`.
- `setAutocompleteProvider(provider)`.
- `borderColor` (function property).
- `disableSubmit` flag.

**Public API:**
- `getText()`, `setText(text)`, `insertTextAtCursor(text)`.
- `getExpandedText()`, `getLines()`, `getCursor()`.
- `addToHistory(text)`.
- `onSubmit`, `onChange` callbacks.

### 13.5 `Markdown` (`components/markdown.ts`)
Renders Markdown with syntax highlighting and theiring.

**Features:**
- Full block-level rendering: headings, paragraphs, code blocks, lists (ordered/unordered/nested), blockquotes, horizontal rules, tables, HTML passthrough.
- Inline rendering: bold, italic, codespan, links (with OSC 8 hyperlink support), strikethrough, image text.
- **Custom strikethrough tokenizer** — strict `~~text~~` syntax (avoids false matches).
- **Table rendering** — column-width-aware cell wrapping, borders (`┌─┬─┐`, `│`, `├─┼─┤`, `└─┴─┘`), natural width distribution with shrink/grow logic.
- **List rendering** — nested lists with proper indentation, task list support (`[x]`/`[ ]`), ordered list numbering.
- **Blockquote rendering** — italic with `│` border, wraps content, strips trailing empty lines before spacing.
- **Link rendering** — OSC 8 hyperlinks on capable terminals, URL in parentheses fallback.
- **Code block rendering** — optional `highlightCode` function, configurable indent (default: 2 spaces), border lines.
- **Default text style** — applies foreground color and text decorations (bold/italic/strikethrough/underline) with proper ANSI reapplication after inline styling.
- **Heading level differentiation** — H1: bold+underline, H2: bold, H3+: heading color only.
- **Render caching** — cache keyed on text + width.
- **Padding** (X/Y) and background color support.
- **Image line passthrough** — doesn't wrap or pad lines containing image sequences.

### 13.6 `Loader` (`components/loader.ts`)
Animated loading spinner.

- Braille spinner animation (default: `⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏` at 80ms interval).
- Configurable frames and interval via `LoaderIndicatorOptions`.
- Customizable spinner color and message color functions.
- Start/stop control.
- Dynamic message update via `setMessage()`.
- Renders with leading empty line for visual separation.
- Requests TUI re-render on each frame.

### 13.7 `CancellableLoader` (`components/cancellable-loader.ts`)
Extends `Loader` with cancellation support.

- **`AbortSignal`** — `signal` property for cancelling async operations.
- **`aborted`** — boolean check.
- **`onAbort`** — callback when user presses Escape.
- **Key binding** — listens for `tui.select.cancel` (Escape/Ctrl+C) to abort.
- **`dispose()`** — cleanup (stops spinner).

### 13.8 `SelectList` (`components/select-list.ts`)
Keyboard-navigable selection list.

- **Items**: `{ value, label, description? }` format.
- **Scrolling**: centered viewport with max visible items.
- **Filtering**: `setFilter(query)` — prefix-based item filtering.
- **Keyboard navigation** — up/down arrows (wraps around), Enter to confirm, Escape to cancel.
- **Selection callbacks**: `onSelect`, `onCancel`, `onSelectionChange`.
- **Description column** — when terminal >40 cols, shows descriptions aligned in second column.
- **Primary column width** — auto-sizing between min/max bounds (default: 32), with custom truncation.
- **Scroll indicator** — shows `(N/M)` when items exceed viewport.
- **No-match display** — "No matching commands".
- **Cursor render**: `→` for selected item.

### 13.9 `SettingsList` (`components/settings-list.ts`)
Settings panel with value cycling and submenus.

- **Items**: `{ id, label, description?, currentValue, values?, submenu? }`.
- **Value cycling**: Enter/Space cycles through `values[]` array.
- **Submenu support**: `submenu(currentValue, done)` returns a Component shown as nested UI.
- **Search/filter**: optional search mode (`enableSearch` option) with an embedded `Input` component and fuzzy filtering.
- **Keyboard navigation**: up/down, Enter/Space to activate, Escape to cancel.
- **Description display**: shows selected item's description wrapped to width.
- **Value alignment**: labels left-aligned at max label width (capped at 30).
- **Scroll indicator**: `(N/M)` when items exceed viewport.
- **Hint line**: contextual help at bottom.
- **`updateValue(id, newValue)`** — programmatic update.
- **`onChange(id, newValue)`** callback.
- **Submenu lifecycle**: replaces main list on render, restores selection when closed.

### 13.10 `Spacer` (`components/spacer.ts`)
- Renders N empty lines (default: 1).
- `setLines(n)` to adjust.

### 13.11 `Box` (`components/box.ts`)
- Container with padding and background applied to all children.
- Configurable `paddingX`, `paddingY`.
- Background function (optional, changeable via `setBgFn`).
- Render caching with background change detection (samples `bgFn` output).
- Child management: `addChild`, `removeChild`, `clear`.
- Proper invalidation cascade.

### 13.12 `Image` (`components/image.ts`)
Displays images inline in terminals.

- **Protocol support**: Kitty graphics protocol and iTerm2 inline images.
- **Format support**: PNG, JPEG, GIF, WebP (dimensions parsed from headers).
- **Auto-detection** of terminal capabilities.
- **Fallback**: text placeholder `[Image: filename [MIME] WxH]` on unsupported terminals.
- **Sizing**: `maxWidthCells`, `maxHeightCells` options.
- **Image ID**: supports reuse for animations via `imageId` option.
- **`getImageId()`** — retrieve the allocated Kitty image ID.
- **Render caching**.

---

## 14. COMPLETE PUBLIC API (as exported from `index.ts`)

### Core
| Export | Source |
|--------|--------|
| `TUI` | `tui.ts` |
| `Container` | `tui.ts` |
| `Component` | `tui.ts` |
| `Focusable` | `tui.ts` |
| `isFocusable` | `tui.ts` |
| `CURSOR_MARKER` | `tui.ts` |
| `OverlayAnchor` | `tui.ts` |
| `OverlayHandle` | `tui.ts` |
| `OverlayMargin` | `tui.ts` |
| `OverlayOptions` | `tui.ts` |
| `SizeValue` | `tui.ts` |

### Terminal
| Export | Source |
|--------|--------|
| `Terminal` | `terminal.ts` |
| `ProcessTerminal` | `terminal.ts` |

### Keyboard
| Export | Source |
|--------|--------|
| `Key` | `keys.ts` |
| `matchesKey` | `keys.ts` |
| `parseKey` | `keys.ts` |
| `setKittyProtocolActive` | `keys.ts` |
| `isKittyProtocolActive` | `keys.ts` |
| `isKeyRelease` | `keys.ts` |
| `isKeyRepeat` | `keys.ts` |
| `decodeKittyPrintable` | `keys.ts` |
| `KeyEventType` | `keys.ts` |
| `KeyId` | `keys.ts` |

### Utilities
| Export | Source |
|--------|--------|
| `visibleWidth` | `utils.ts` |
| `truncateToWidth` | `utils.ts` |
| `wrapTextWithAnsi` | `utils.ts` |

### Stdin Buffering
| Export | Source |
|--------|--------|
| `StdinBuffer` | `stdin-buffer.ts` |
| `StdinBufferEventMap` | `stdin-buffer.ts` |
| `StdinBufferOptions` | `stdin-buffer.ts` |

### Terminal Image
| Export | Source |
|--------|--------|
| `allocateImageId` | `terminal-image.ts` |
| `calculateImageRows` | `terminal-image.ts` |
| `deleteKittyImage` | `terminal-image.ts` |
| `deleteAllKittyImages` | `terminal-image.ts` |
| `detectCapabilities` | `terminal-image.ts` |
| `encodeITerm2` | `terminal-image.ts` |
| `encodeKitty` | `terminal-image.ts` |
| `getCapabilities` | `terminal-image.ts` |
| `getCellDimensions` | `terminal-image.ts` |
| `getImageDimensions` | `terminal-image.ts` |
| `getPngDimensions` | `terminal-image.ts` |
| `getJpegDimensions` | `terminal-image.ts` |
| `getGifDimensions` | `terminal-image.ts` |
| `getWebpDimensions` | `terminal-image.ts` |
| `hyperlink` | `terminal-image.ts` |
| `imageFallback` | `terminal-image.ts` |
| `renderImage` | `terminal-image.ts` |
| `resetCapabilitiesCache` | `terminal-image.ts` |
| `setCapabilities` | `terminal-image.ts` |
| `setCellDimensions` | `terminal-image.ts` |
| `ImageProtocol` | `terminal-image.ts` |
| `TerminalCapabilities` | `terminal-image.ts` |
| `ImageDimensions` | `terminal-image.ts` |
| `ImageRenderOptions` | `terminal-image.ts` |
| `CellDimensions` | `terminal-image.ts` |

### Autocomplete
| Export | Source |
|--------|--------|
| `CombinedAutocompleteProvider` | `autocomplete.ts` |
| `AutocompleteItem` | `autocomplete.ts` |
| `AutocompleteProvider` | `autocomplete.ts` |
| `AutocompleteSuggestions` | `autocomplete.ts` |
| `SlashCommand` | `autocomplete.ts` |

### Keybindings
| Export | Source |
|--------|--------|
| `KeybindingsManager` | `keybindings.ts` |
| `setKeybindings` | `keybindings.ts` |
| `getKeybindings` | `keybindings.ts` |
| `TUI_KEYBINDINGS` | `keybindings.ts` |
| `Keybinding` | `keybindings.ts` |
| `KeybindingConflict` | `keybindings.ts` |
| `KeybindingDefinition` | `keybindings.ts` |
| `KeybindingDefinitions` | `keybindings.ts` |
| `Keybindings` | `keybindings.ts` |
| `KeybindingsConfig` | `keybindings.ts` |

### Editor Component Interface
| Export | Source |
|--------|--------|
| `EditorComponent` | `editor-component.ts` |

### Fuzzy Matching
| Export | Source |
|--------|--------|
| `fuzzyMatch` | `fuzzy.ts` |
| `fuzzyFilter` | `fuzzy.ts` |
| `FuzzyMatch` | `fuzzy.ts` |

### Built-in Components
| Export | Source |
|--------|--------|
| `Box` | `components/box.ts` |
| `CancellableLoader` | `components/cancellable-loader.ts` |
| `Editor` | `components/editor.ts` |
| `EditorOptions`, `EditorTheme` | `components/editor.ts` |
| `Image` | `components/image.ts` |
| `ImageOptions`, `ImageTheme` | `components/image.ts` |
| `Input` | `components/input.ts` |
| `Loader` | `components/loader.ts` |
| `LoaderIndicatorOptions` | `components/loader.ts` |
| `Markdown` | `components/markdown.ts` |
| `MarkdownTheme`, `DefaultTextStyle` | `components/markdown.ts` |
| `SelectList` | `components/select-list.ts` |
| `SelectItem`, `SelectListTheme`, `SelectListLayoutOptions`, `SelectListTruncatePrimaryContext` | `components/select-list.ts` |
| `SettingsList` | `components/settings-list.ts` |
| `SettingItem`, `SettingsListTheme` | `components/settings-list.ts` |
| `Spacer` | `components/spacer.ts` |
| `Text` | `components/text.ts` |
| `TruncatedText` | `components/truncated-text.ts` |

---

## 15. ENVIRONMENT VARIABLES

| Variable | Purpose |
|----------|---------|
| `PI_TUI_WRITE_LOG` | Log raw ANSI output to file |
| `PI_HARDWARE_CURSOR` | Show hardware cursor (default: off) |
| `PI_CLEAR_ON_SHRINK` | Clear empty rows when content shrinks (default: off) |
| `PI_DEBUG_REDRAW` | Log full re-render reasons to `~/.pi/agent/pi-debug.log` |
| `PI_TUI_DEBUG` | Dump full render state to `/tmp/tui/` |
| `TERMUX_VERSION` | Termux detection (suppresses full re-render on height changes) |
| `COLORTERM` | True color detection |
| `KITTY_WINDOW_ID` | Kitty terminal detection |
| `WEZTERM_PANE` | WezTerm terminal detection |
| `ITERM_SESSION_ID` | iTerm2 terminal detection |
| `GHOSTTY_RESOURCES_DIR` | Ghostty terminal detection |
| `TMUX` | tmux detection (disables images/hyperlinks) |
| `WT_SESSION` | Windows Terminal detection |

---

## 16. DEPENDENCIES

**Runtime:**
- `get-east-asian-width` (^1.3.0) — East Asian character width calculation.
- `marked` (^15.0.12) — Markdown parser.

**Optional:**
- `koffi` (^2.9.0) — Windows native FFI for VT input mode.

**Dev:**
- `@xterm/headless`, `@xterm/xterm` — Virtual terminal for testing.
- `chalk` — Terminal string styling (used in tests/examples).

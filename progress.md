# rho Progress

Last updated: 2026-05-20

## Current Objective

Improve `rho` into a Go/Bubble Tea reimplementation of `pi`, using the local pi architecture and interactive TUI maps as the porting reference.

This is not complete yet. The current work has moved the interactive shell closer to pi's component-driven behavior, but several pi parity areas remain.

## Completed So Far

### Reference Docs

- Added `docs/pi-architecture-map.md` as the broad porting map for pi's package structure, agent core, provider layer, coding-agent services, extensions, resources, settings, auth, and modes.
- Added `docs/pi-interactive-tui-map.md` as the detailed interactive-mode map for Bubble Tea/TUI parity work.

### Bubble Tea Interactive Mode

- Reworked `cmd/rho/interactive.go` around a Bubble Tea chat model.
- Added transcript rendering, fixed input area, status line, key hints, scroll handling, prompt mode, selector mode, and slash autocomplete.
- Added app-level actions for model selection, settings, session resume, and thinking-level cycling.
- Added async message/status handling through Bubble Tea messages so agent events and extension UI can update the live TUI.

### Slash Command UX

- Added interactive handling for:
  - `/login`
  - `/logout`
  - `/model`
  - `/settings`
  - `/theme`
  - `/copy`
  - `/sessions`
  - `/resume`
  - `/name`
  - `/tree`
  - `/fork`
  - `/new`
  - `/commands`
- Added autocomplete for built-in slash commands, extension slash commands, model arguments, and auth provider arguments.
- Registered `/theme` in the shared slash command registry.

### Auth, Model, Settings, Sessions

- `/login` with no args opens a login method selector.
- API-key login opens a provider selector and stores keys through `AuthStorage`.
- OAuth login has a provider selector and structured placeholder, but the actual OAuth callback/token flow is still incomplete.
- `/logout` can remove saved provider keys through a selector or direct provider argument.
- `/model` opens a searchable model selector or accepts a direct model name.
- `/settings` opens a live settings selector for model/auth/theme/show-images/thinking/cwd/commands.
- `/theme` opens a searchable theme selector, applies the active theme to the live Bubble Tea UI, and persists the selected theme in user settings.
- `/sessions` and `/resume` open searchable saved-session selectors.
- `/name` persists a session display name in settings.
- `/fork` saves the current visible conversation as a child session.
- `/new` starts a fresh session in the current interactive process.

### Theme Work

- Initialized `ThemeManager` in `cmd/rho/main.go`.
- Loaded themes from `~/.rho/themes`.
- Applied persisted `theme` setting on startup.
- Mapped agent themes into Bubble Tea/lipgloss styles and markdown styling.
- Added tests for opening the theme selector and persisting a selected theme.

### Extension UI Bridge

- Extension UI `Select`, `Confirm`, and `Input` now route through the Bubble Tea selector/prompt model.
- Extension `Notify` appends system messages.
- Extension `SetStatus` updates the interactive status line.
- Extension slash commands execute asynchronously so UI requests can be answered without blocking the Bubble Tea event loop.

### Run Helper

- Added `scripts/run-latest-rho.sh`.
- The script rebuilds the current checkout to `${RHO_BIN:-./rho}` and then execs it with any passed args.

### Bug Fixes

- Fixed `pkg/agent/utils/html.go`, replacing a JavaScript-style regexp backreference with Go-compatible script/style regexes.
- Added a regression test for stripping script/style blocks.

### Extension Examples

- Existing extension port status remains: 69/69 pi extension examples are represented as ported in this repo.

## Verification

Latest successful commands:

```bash
GOCACHE=/tmp/go-build-cache GOPATH=/tmp/go go test ./cmd/rho
GOCACHE=/tmp/go-build-cache GOPATH=/tmp/go go test ./pkg/agent/codecore
GOCACHE=/tmp/go-build-cache GOPATH=/tmp/go go test ./...
GOCACHE=/tmp/go-build-cache GOPATH=/tmp/go go build -o /tmp/rho ./cmd/rho
GOCACHE=/tmp/go-build-cache GOPATH=/tmp/go scripts/run-latest-rho.sh -version
```

Observed script output:

```text
Building latest rho -> ./rho
Running ./rho -version
rho v0.2.0
```

## Known Gaps

- OAuth login is not fully implemented. It still needs the pi-style focused login dialog, callback/manual redirect handling, token persistence, refresh, and model/auth state refresh.
- Session tree behavior is shallow. `/tree` currently reuses the session selector instead of showing a true branch/tree navigator.
- Settings parity is partial. The selector covers core items, but not the full pi settings surface such as transport, warnings, scoped models, themes beyond selection, and dynamic component updates for every setting.
- Footer parity is partial. The current status line shows model/provider/cwd/session name/extension statuses, but not full pi footer data such as git branch, token usage, cost, context percentage, subscription/OAuth indicator, and richer warning state.
- Tool and bash execution rendering is still simplified compared with pi's expandable, stateful components.
- Editor parity is partial. The Bubble Tea input supports common editing/navigation, autocomplete, prompt, and selectors, but not the full pi editor feature set such as full kill-ring/undo behavior, image paste, configurable keybindings, and richer focused component routing.
- Extension parity is incomplete. Lifecycle hooks and examples exist, and UI select/confirm/input/notify/status are wired, but custom overlays, message renderers, keybindings, providers, OAuth providers, and richer UI contribution surfaces need deeper verification.
- Shared runtime parity is incomplete. More behavior still lives directly in `cmd/rho/interactive.go`; pi separates this through coding-agent core services.
- Print/json/rpc modes need an audit against the improved shared runtime expectations so interactive-only behavior does not diverge from other modes.

## What To Do Next

1. Implement the OAuth login dialog end-to-end.
   - Use the existing `pkg/agent/codecore/ui/login_dialog.go` and `pkg/agent/codecore/ui/oauth_selector.go` concepts.
   - Store OAuth credentials through auth storage.
   - Refresh `APIKey`, model availability, and footer/status state after auth changes.

2. Build a real session tree selector.
   - Use `SessionHeader.ParentSession`.
   - Show parent/child branches instead of a flat saved-session list.
   - Add tests for forked session visibility and tree selection.

3. Move interactive session/runtime behavior out of `cmd/rho/interactive.go`.
   - Push model/auth/settings/session coordination into `pkg/agent/codecore`.
   - Keep Bubble Tea as presentation, not the owner of product behavior.

4. Expand the footer/status model.
   - Add git branch, session name, model/provider, thinking level, token usage, cost, context usage, and extension statuses.
   - Add focused tests around footer data generation.

5. Upgrade tool execution rendering.
   - Add stateful running/completed/error tool components.
   - Support collapsed/expanded output and bash execution views.

6. Audit extension parity.
   - Verify every ported example actually works through the current Bubble Tea bridge.
   - Add missing bridge APIs for custom overlays, keybindings, message renderers, custom providers, and OAuth providers.

7. Re-run the completion audit against `docs/pi-architecture-map.md` and `docs/pi-interactive-tui-map.md`.
   - Treat this as the gate before considering rho a full pi clone in Go.

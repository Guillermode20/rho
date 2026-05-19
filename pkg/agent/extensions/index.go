// Package extensions implements rho's plugin system.
//
// Extensions can:
//   - Register custom LLM-callable tools (any tool the agent can invoke)
//   - Subscribe to lifecycle events (agent start/end, turn start/end, session events)
//   - Register slash commands (e.g., /help, /example)
//   - Register custom AI providers (local models, custom endpoints)
//   - Intercept and modify messages before they reach the LLM
//   - Block or modify tool calls before execution
//   - Register keyboard shortcuts and CLI flags
//   - Provide custom message renderers for the TUI
//
// Architecture:
//
//   Runtime            Manages extensions and dispatches events
//   ExtensionDef       Defines an extension's handlers and registered items
//   ExtensionContext   Context passed to all handlers (UI, CWD, abort, etc.)
//   EventBus           Inter-extension communication
//
// Event flow:
//
//   User Input → OnInput → OnBeforeAgentStart → OnContext →
//   FireToolCall → Tool executes → FireToolResult →
//   OnTurnEnd → OnAgentEnd
//
// Examples:
//
//   See ../../examples/extensions/example-extension.go
package extensions

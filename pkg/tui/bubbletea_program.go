package tui

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
)

// ─── Program wrapper ────────────────────────────────────────────────────────

// BTCtx holds a running Bubble Tea program and its model.
// This replaces the old TUI+ProcessTerminal pairing.
type BTCtx struct {
	Program *tea.Program
	Model   *BTModel
}

// NewBTCtx creates a new Bubble Tea program context.
// The program uses the alternate screen buffer by default.
// Call Start() to begin.
func NewBTCtx() *BTCtx {
	model := NewBTModel()
	p := tea.NewProgram(model)
	return &BTCtx{
		Program: p,
		Model:   model,
	}
}

// NewBTCtxInline creates a new Bubble Tea program that runs inline
// (without the alternate screen buffer).
func NewBTCtxInline() *BTCtx {
	model := NewBTModel()
	p := tea.NewProgram(model)
	return &BTCtx{
		Program: p,
		Model:   model,
	}
}

// Start starts the Bubble Tea program and blocks until it quits.
// In most cases you want to call this directly (it blocks).
func (ctx *BTCtx) Start() error {
	_, err := ctx.Program.Run()
	return err
}

// StartAsync starts the Bubble Tea program in a goroutine.
// Use Stop() to quit.
func (ctx *BTCtx) StartAsync() {
	go func() {
		if _, err := ctx.Program.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "Bubble Tea error: %v\n", err)
			os.Exit(1)
		}
	}()
}

// Stop sends a quit message to the program.
func (ctx *BTCtx) Stop() {
	ctx.Program.Quit()
}

// Send sends a message to the program.
func (ctx *BTCtx) Send(msg tea.Msg) {
	ctx.Program.Send(msg)
}

// RequestRender sends a render request.
func (ctx *BTCtx) RequestRender(force bool) {
	if force {
		ctx.Send(ClearScreenMsg{})
	}
	ctx.Send(RenderRequest{})
}

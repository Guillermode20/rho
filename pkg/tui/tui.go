package tui

import (
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Component is the interface all UI components must implement.
type Component interface {
	// Render renders the component to lines for the given viewport width.
	Render(width int) []string

	// HandleInput handles keyboard input when the component has focus.
	HandleInput(data string)

	// Invalidate clears any cached rendering state.
	Invalidate()

	// WantsKeyRelease returns true if the component wants key release events.
	WantsKeyRelease() bool
}

// Focusable is an interface for components that can receive focus and display a hardware cursor.
type Focusable interface {
	// SetFocused sets whether this component is focused.
	SetFocused(focused bool)
	// Focused returns whether this component is focused.
	Focused() bool
}

// IsFocusable checks if a component implements Focusable.
func IsFocusable(c Component) bool {
	_, ok := c.(Focusable)
	return ok
}

// OverlayAnchor defines anchor positions for overlays.
type OverlayAnchor int

const (
	AnchorCenter       OverlayAnchor = iota
	AnchorTopLeft
	AnchorTopRight
	AnchorBottomLeft
	AnchorBottomRight
	AnchorTopCenter
	AnchorBottomCenter
	AnchorLeftCenter
	AnchorRightCenter
)

// OverlayMargin configures margins for overlays.
type OverlayMargin struct {
	Top    int
	Right  int
	Bottom int
	Left   int
}

// SizeValue can be an absolute number or a percentage string (e.g., "50%").
type SizeValue string

// ParseSizeValue parses a SizeValue into an absolute value given a reference size.
func ParseSizeValue(v string, referenceSize int) (int, bool) {
	if v == "" {
		return 0, false
	}
	if strings.HasSuffix(v, "%") {
		pctStr := strings.TrimSuffix(v, "%")
		pct, err := strconv.ParseFloat(pctStr, 64)
		if err != nil {
			return 0, false
		}
		return int(math.Floor(float64(referenceSize) * pct / 100)), true
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0, false
	}
	return n, true
}

// OverlayOptions configures overlay positioning and sizing.
type OverlayOptions struct {
	// Width in columns, or percentage (e.g., "50%").
	Width string
	// Minimum width in columns.
	MinWidth int
	// Maximum height in rows, or percentage (e.g., "50%").
	MaxHeight string

	// Anchor point for positioning (default: AnchorCenter).
	Anchor OverlayAnchor
	// Horizontal offset from anchor position (positive = right).
	OffsetX int
	// Vertical offset from anchor position (positive = down).
	OffsetY int

	// Row position: absolute number, or percentage (e.g., "25%").
	Row string
	// Column position: absolute number, or percentage (e.g., "50%").
	Col string

	// Margin from terminal edges.
	Margin *OverlayMargin

	// Visible returns true if the overlay should be rendered.
	// Called each render cycle with current terminal dimensions.
	Visible func(termWidth, termHeight int) bool

	// NonCapturing means don't capture keyboard focus when shown.
	NonCapturing bool
}

// OverlayHandle controls an overlay's visibility.
type OverlayHandle struct {
	Hide      func()
	SetHidden func(hidden bool)
	IsHidden  func() bool
	Focus     func()
	Unfocus   func()
	IsFocused func() bool
}

// Container is a component that contains other components.
type Container struct {
	children []Component
}

// NewContainer creates a new Container.
func NewContainer() *Container {
	return &Container{}
}

func (c *Container) AddChild(component Component) {
	c.children = append(c.children, component)
}

func (c *Container) RemoveChild(component Component) {
	for i, child := range c.children {
		if child == component {
			c.children = append(c.children[:i], c.children[i+1:]...)
			return
		}
	}
}

func (c *Container) Clear() {
	c.children = nil
}

func (c *Container) Invalidate() {
	for _, child := range c.children {
		child.Invalidate()
	}
}

func (c *Container) Render(width int) []string {
	var lines []string
	for _, child := range c.children {
		childLines := child.Render(width)
		lines = append(lines, childLines...)
	}
	return lines
}

func (c *Container) HandleInput(data string) {
	// Container passes input to focused child if it implements HandleInput
}

func (c *Container) WantsKeyRelease() bool { return false }

// InputListenerResult is returned by input listeners.
type InputListenerResult struct {
	Consume bool
	Data    string
}

// InputListener is a function that intercepts input before it reaches the focused component.
type InputListener func(data string) *InputListenerResult

// overlayEntry represents a single overlay in the stack.
type overlayEntry struct {
	component  Component
	options    *OverlayOptions
	preFocus   Component
	hidden     bool
	focusOrder int
}

// TUI is the main class for managing terminal UI with differential rendering.
type TUI struct {
	*Container
	terminal               Terminal
	previousLines          []string
	previousKittyImageIDs  map[int]struct{}
	previousWidth          int
	previousHeight         int
	focusedComponent       Component
	inputListeners         []InputListener
	onDebug                func()
	renderRequested        bool
	renderTimer            *time.Timer
	lastRenderAt           time.Time
	cursorRow              int
	hardwareCursorRow      int
	showHardwareCursor     bool
	clearOnShrink          bool
	maxLinesRendered       int
	previousViewportTop    int
	fullRedrawCount        int
	stopped                bool
	focusOrderCounter      int
	overlayStack           []overlayEntry
	mu                     sync.Mutex
}

const minRenderIntervalMs = 16

// NewTUI creates a new TUI instance.
func NewTUI(terminal Terminal) *TUI {
	return &TUI{
		Container:              NewContainer(),
		terminal:               terminal,
		previousKittyImageIDs:  make(map[int]struct{}),
		showHardwareCursor:     os.Getenv("PI_HARDWARE_CURSOR") == "1",
		clearOnShrink:          os.Getenv("PI_CLEAR_ON_SHRINK") != "0",
	}
}

func (t *TUI) FullRedraws() int {
	return t.fullRedrawCount
}

func (t *TUI) GetShowHardwareCursor() bool {
	return t.showHardwareCursor
}

func (t *TUI) SetShowHardwareCursor(enabled bool) {
	if t.showHardwareCursor == enabled {
		return
	}
	t.showHardwareCursor = enabled
	if !enabled {
		t.terminal.HideCursor()
	}
	t.RequestRender(false)
}

func (t *TUI) GetClearOnShrink() bool {
	return t.clearOnShrink
}

func (t *TUI) SetClearOnShrink(enabled bool) {
	t.clearOnShrink = enabled
}

func (t *TUI) SetFocus(component Component) {
	// Clear focused flag on old component
	if f, ok := t.focusedComponent.(Focusable); ok {
		f.SetFocused(false)
	}

	t.focusedComponent = component

	// Set focused flag on new component
	if f, ok := component.(Focusable); ok {
		f.SetFocused(true)
	}
}

func (t *TUI) showOverlay(component Component, options *OverlayOptions) OverlayHandle {
	t.focusOrderCounter++
	entry := overlayEntry{
		component:  component,
		options:    options,
		preFocus:   t.focusedComponent,
		hidden:     false,
		focusOrder: t.focusOrderCounter,
	}
	t.overlayStack = append(t.overlayStack, entry)

	if (options == nil || !options.NonCapturing) && t.isOverlayVisible(&entry) {
		t.SetFocus(component)
	}
	t.terminal.HideCursor()
	t.RequestRender(false)

	return OverlayHandle{
		Hide: func() {
			t.mu.Lock()
			idx := -1
			for i, e := range t.overlayStack {
				if e.component == component {
					idx = i
					break
				}
			}
			if idx >= 0 {
				t.overlayStack = append(t.overlayStack[:idx], t.overlayStack[idx+1:]...)
				if t.focusedComponent == component {
					topVisible := t.getTopmostVisibleOverlay()
					if topVisible != nil {
						t.SetFocus(topVisible.component)
					} else {
						t.SetFocus(entry.preFocus)
					}
				}
				if len(t.overlayStack) == 0 {
					t.terminal.HideCursor()
				}
				t.mu.Unlock()
				t.RequestRender(false)
			} else {
				t.mu.Unlock()
			}
		},
		SetHidden: func(hidden bool) {
			t.mu.Lock()
			if entry.hidden == hidden {
				t.mu.Unlock()
				return
			}
			entry.hidden = hidden
			if hidden {
				if t.focusedComponent == component {
					topVisible := t.getTopmostVisibleOverlay()
					if topVisible != nil {
						t.SetFocus(topVisible.component)
					} else {
						t.SetFocus(entry.preFocus)
					}
				}
			} else {
				if (options == nil || !options.NonCapturing) && t.isOverlayVisible(&entry) {
					t.focusOrderCounter++
					entry.focusOrder = t.focusOrderCounter
					t.SetFocus(component)
				}
			}
			t.mu.Unlock()
			t.RequestRender(false)
		},
		IsHidden: func() bool {
			t.mu.Lock()
			defer t.mu.Unlock()
			return entry.hidden
		},
		Focus: func() {
			t.mu.Lock()
			found := false
			for _, e := range t.overlayStack {
				if e.component == component {
					found = true
					break
				}
			}
			if !found || !t.isOverlayVisible(&entry) {
				t.mu.Unlock()
				return
			}
			if t.focusedComponent != component {
				t.SetFocus(component)
			}
			t.focusOrderCounter++
			entry.focusOrder = t.focusOrderCounter
			t.mu.Unlock()
			t.RequestRender(false)
		},
		Unfocus: func() {
			t.mu.Lock()
			if t.focusedComponent != component {
				t.mu.Unlock()
				return
			}
			topVisible := t.getTopmostVisibleOverlay()
			if topVisible != nil && topVisible.component != component {
				t.SetFocus(topVisible.component)
			} else {
				t.SetFocus(entry.preFocus)
			}
			t.mu.Unlock()
			t.RequestRender(false)
		},
		IsFocused: func() bool {
			t.mu.Lock()
			defer t.mu.Unlock()
			return t.focusedComponent == component
		},
	}
}

func (t *TUI) HideOverlay() {
	if len(t.overlayStack) == 0 {
		return
	}
	overlay := t.overlayStack[len(t.overlayStack)-1]
	t.overlayStack = t.overlayStack[:len(t.overlayStack)-1]
	if t.focusedComponent == overlay.component {
		topVisible := t.getTopmostVisibleOverlay()
		if topVisible != nil {
			t.SetFocus(topVisible.component)
		} else {
			t.SetFocus(overlay.preFocus)
		}
	}
	if len(t.overlayStack) == 0 {
		t.terminal.HideCursor()
	}
	t.RequestRender(false)
}

func (t *TUI) HasOverlay() bool {
	for _, o := range t.overlayStack {
		if t.isOverlayVisible(&o) {
			return true
		}
	}
	return false
}

func (t *TUI) isOverlayVisible(entry *overlayEntry) bool {
	if entry.hidden {
		return false
	}
	if entry.options != nil && entry.options.Visible != nil {
		return entry.options.Visible(t.terminal.Columns(), t.terminal.Rows())
	}
	return true
}

func (t *TUI) getTopmostVisibleOverlay() *overlayEntry {
	for i := len(t.overlayStack) - 1; i >= 0; i-- {
		if t.overlayStack[i].options != nil && t.overlayStack[i].options.NonCapturing {
			continue
		}
		if t.isOverlayVisible(&t.overlayStack[i]) {
			return &t.overlayStack[i]
		}
	}
	return nil
}

func (t *TUI) Invalidate() {
	t.Container.Invalidate()
	for _, overlay := range t.overlayStack {
		overlay.component.Invalidate()
	}
}

func (t *TUI) Start() {
	t.stopped = false
	t.terminal.Start(func(data string) {
		t.handleInput(data)
	}, func() {
		t.RequestRender(false)
	})
	t.terminal.HideCursor()
	t.queryCellSize()
	t.RequestRender(false)
}

func (t *TUI) AddInputListener(listener InputListener) func() {
	t.mu.Lock()
	t.inputListeners = append(t.inputListeners, listener)
	idx := len(t.inputListeners) - 1
	t.mu.Unlock()
	return func() {
		t.mu.Lock()
		t.inputListeners = append(t.inputListeners[:idx], t.inputListeners[idx+1:]...)
		t.mu.Unlock()
	}
}

func (t *TUI) RemoveInputListener(listener InputListener) {
	t.mu.Lock()
	for i, l := range t.inputListeners {
		if fmt.Sprintf("%p", l) == fmt.Sprintf("%p", listener) {
			t.inputListeners = append(t.inputListeners[:i], t.inputListeners[i+1:]...)
			break
		}
	}
	t.mu.Unlock()
}

func (t *TUI) queryCellSize() {
	// Query terminal for cell size in pixels: CSI 16 t
	// Response format: CSI 6 ; height ; width t
	t.terminal.Write("\x1b[16t")
}

func (t *TUI) Stop() {
	t.stopped = true
	if t.renderTimer != nil {
		t.renderTimer.Stop()
		t.renderTimer = nil
	}

	// Move cursor to the end of the content
	if len(t.previousLines) > 0 {
		targetRow := len(t.previousLines)
		lineDiff := targetRow - t.hardwareCursorRow
		if lineDiff > 0 {
			t.terminal.Write(fmt.Sprintf("\x1b[%dB", lineDiff))
		} else if lineDiff < 0 {
			t.terminal.Write(fmt.Sprintf("\x1b[%dA", -lineDiff))
		}
		t.terminal.Write("\r\n")
	}

	t.terminal.ShowCursor()
	t.terminal.Stop()
}

func (t *TUI) RequestRender(force bool) {
	if force {
		t.previousLines = nil
		t.previousWidth = -1
		t.previousHeight = -1
		t.cursorRow = 0
		t.hardwareCursorRow = 0
		t.maxLinesRendered = 0
		t.previousViewportTop = 0
		if t.renderTimer != nil {
			t.renderTimer.Stop()
			t.renderTimer = nil
		}
		t.renderRequested = true
		go func() {
			time.Sleep(time.Millisecond)
			t.mu.Lock()
			if t.stopped || !t.renderRequested {
				t.mu.Unlock()
				return
			}
			t.renderRequested = false
			t.lastRenderAt = time.Now()
			t.mu.Unlock()
			t.doRender()
		}()
		return
	}

	t.mu.Lock()
	if t.renderRequested {
		t.mu.Unlock()
		return
	}
	t.renderRequested = true
	t.mu.Unlock()

	go t.scheduleRender()
}

func (t *TUI) scheduleRender() {
	t.mu.Lock()
	if t.stopped || t.renderTimer != nil || !t.renderRequested {
		t.mu.Unlock()
		return
	}
	elapsed := time.Since(t.lastRenderAt)
	delay := time.Duration(math.Max(0, float64(minRenderIntervalMs-elapsed.Milliseconds()))) * time.Millisecond

	t.renderTimer = time.AfterFunc(delay, func() {
		t.mu.Lock()
		t.renderTimer = nil
		if t.stopped || !t.renderRequested {
			t.mu.Unlock()
			return
		}
		t.renderRequested = false
		t.lastRenderAt = time.Now()
		t.mu.Unlock()
		t.doRender()

		t.mu.Lock()
		if t.renderRequested {
			t.mu.Unlock()
			t.scheduleRender()
		} else {
			t.mu.Unlock()
		}
	})
	t.mu.Unlock()
}

func (t *TUI) handleInput(data string) {
	// Check input listeners
	for _, listener := range t.inputListeners {
		result := listener(data)
		if result != nil && result.Consume {
			return
		}
		if result != nil && result.Data != "" {
			data = result.Data
		}
	}
	if data == "" {
		return
	}

	// Consume cell size responses
	if t.consumeCellSizeResponse(data) {
		return
	}

	// Global debug key (Shift+Ctrl+D)
	if MatchesKey(data, "shift+ctrl+d") && t.onDebug != nil {
		t.onDebug()
		return
	}

	// Check if focused overlay is still visible
	for i := len(t.overlayStack) - 1; i >= 0; i-- {
		o := &t.overlayStack[i]
		if o.component == t.focusedComponent && !t.isOverlayVisible(o) {
			topVisible := t.getTopmostVisibleOverlay()
			if topVisible != nil {
				t.SetFocus(topVisible.component)
			} else {
				t.SetFocus(o.preFocus)
			}
			break
		}
	}

	// Pass input to focused component
	if t.focusedComponent != nil {
		if IsKeyRelease(data) && !t.focusedComponent.WantsKeyRelease() {
			return
		}
		t.focusedComponent.HandleInput(data)
		t.RequestRender(false)
	}
}

func (t *TUI) consumeCellSizeResponse(data string) bool {
	// Response format: ESC [ 6 ; height ; width t
	if strings.HasPrefix(data, "\x1b[6;") && strings.HasSuffix(data, "t") {
		parts := strings.Split(strings.TrimSuffix(strings.TrimPrefix(data, "\x1b[6;"), "t"), ";")
		if len(parts) == 2 {
			h, err1 := strconv.Atoi(parts[0])
			w, err2 := strconv.Atoi(parts[1])
			if err1 == nil && err2 == nil && h > 0 && w > 0 {
				// Set cell dimensions - would need to update terminal image capabilities
				t.Invalidate()
				t.RequestRender(false)
				return true
			}
		}
	}
	return false
}

// resolveOverlayLayout computes position and size for an overlay.
func (t *TUI) resolveOverlayLayout(options *OverlayOptions, overlayHeight, termWidth, termHeight int) (width, row, col int, maxHeight int) {
	if options == nil {
		options = &OverlayOptions{}
	}

	// Parse margin
	margin := OverlayMargin{}
	if options.Margin != nil {
		margin = *options.Margin
	}
	marginTop := max(0, margin.Top)
	marginRight := max(0, margin.Right)
	marginBottom := max(0, margin.Bottom)
	marginLeft := max(0, margin.Left)

	availWidth := max(1, termWidth-marginLeft-marginRight)
	availHeight := max(1, termHeight-marginTop-marginBottom)

	// Resolve width
	if w, ok := ParseSizeValue(options.Width, termWidth); ok {
		width = w
	} else {
		width = min(80, availWidth)
	}
	if options.MinWidth > 0 {
		width = max(width, options.MinWidth)
	}
	width = max(1, min(width, availWidth))

	// Resolve maxHeight
	if mh, ok := ParseSizeValue(options.MaxHeight, termHeight); ok {
		maxHeight = max(1, min(mh, availHeight))
	}

	effectiveHeight := overlayHeight
	if maxHeight > 0 {
		effectiveHeight = min(overlayHeight, maxHeight)
	}

	// Resolve position
	anchor := AnchorCenter
	if options.Anchor != 0 {
		anchor = options.Anchor
	}

	if options.Row != "" {
		if r, ok := ParseSizeValue(options.Row, termHeight); ok {
			maxRowVal := max(0, availHeight-effectiveHeight)
			row = marginTop + int(math.Floor(float64(maxRowVal)*float64(r)/float64(termHeight)))
		} else {
			row = t.resolveAnchorRow(anchor, effectiveHeight, availHeight, marginTop)
		}
	} else {
		row = t.resolveAnchorRow(anchor, effectiveHeight, availHeight, marginTop)
	}

	if options.Col != "" {
		if c, ok := ParseSizeValue(options.Col, termWidth); ok {
			maxColVal := max(0, availWidth-width)
			col = marginLeft + int(math.Floor(float64(maxColVal)*float64(c)/float64(termWidth)))
		} else {
			col = t.resolveAnchorCol(anchor, width, availWidth, marginLeft)
		}
	} else {
		col = t.resolveAnchorCol(anchor, width, availWidth, marginLeft)
	}

	row += options.OffsetY
	col += options.OffsetX

	// Clamp to terminal bounds
	row = max(marginTop, min(row, termHeight-marginBottom-effectiveHeight))
	col = max(marginLeft, min(col, termWidth-marginRight-width))

	return
}

func (t *TUI) resolveAnchorRow(anchor OverlayAnchor, height, availHeight, marginTop int) int {
	switch anchor {
	case AnchorTopLeft, AnchorTopCenter, AnchorTopRight:
		return marginTop
	case AnchorBottomLeft, AnchorBottomCenter, AnchorBottomRight:
		return marginTop + availHeight - height
	default:
		return marginTop + (availHeight-height)/2
	}
}

func (t *TUI) resolveAnchorCol(anchor OverlayAnchor, width, availWidth, marginLeft int) int {
	switch anchor {
	case AnchorTopLeft, AnchorLeftCenter, AnchorBottomLeft:
		return marginLeft
	case AnchorTopRight, AnchorRightCenter, AnchorBottomRight:
		return marginLeft + availWidth - width
	default:
		return marginLeft + (availWidth-width)/2
	}
}

// compositeOverlays blends overlay content into the rendered lines.
func (t *TUI) compositeOverlays(lines []string, termWidth, termHeight int) []string {
	if len(t.overlayStack) == 0 {
		return lines
	}

	type renderedOverlay struct {
		overlayLines []string
		row, col, w  int
	}

	result := make([]string, len(lines))
	copy(result, lines)

	var rendered []renderedOverlay
	minLinesNeeded := len(result)

	// Collect visible entries sorted by focus order
	var visibleEntries []overlayEntry
	for _, e := range t.overlayStack {
		if t.isOverlayVisible(&e) {
			visibleEntries = append(visibleEntries, e)
		}
	}
	// Sort by focus order (stable insertion)
	for i := 0; i < len(visibleEntries); i++ {
		for j := i + 1; j < len(visibleEntries); j++ {
			if visibleEntries[j].focusOrder < visibleEntries[i].focusOrder {
				visibleEntries[i], visibleEntries[j] = visibleEntries[j], visibleEntries[i]
			}
		}
	}

	for _, entry := range visibleEntries {
		overlayWidth, _, _, maxH := t.resolveOverlayLayout(entry.options, 0, termWidth, termHeight)
		overlayLines := entry.component.Render(overlayWidth)
		if maxH > 0 && len(overlayLines) > maxH {
			overlayLines = overlayLines[:maxH]
		}

		_, row, col, _ := t.resolveOverlayLayout(entry.options, len(overlayLines), termWidth, termHeight)
		rendered = append(rendered, renderedOverlay{overlayLines, row, col, overlayWidth})
		minLinesNeeded = max(minLinesNeeded, row+len(overlayLines))
	}

	// Pad result
	workingHeight := max(max(len(result), termHeight), minLinesNeeded)
	for len(result) < workingHeight {
		result = append(result, "")
	}

	viewportStart := max(0, workingHeight-termHeight)

	// Composite each overlay
	for _, r := range rendered {
		for i := 0; i < len(r.overlayLines); i++ {
			idx := viewportStart + r.row + i
			if idx >= 0 && idx < len(result) {
				overlayLine := r.overlayLines[i]
				if VisibleWidth(overlayLine) > r.w {
					overlayLine = SliceByColumn(overlayLine, 0, r.w, true)
				}
				result[idx] = t.compositeLineAt(result[idx], overlayLine, r.col, r.w, termWidth)
			}
		}
	}

	return result
}

// compositeLineAt splices overlay content into a base line at a specific column.
func (t *TUI) compositeLineAt(baseLine, overlayLine string, startCol, overlayWidth, totalWidth int) string {
	if IsImageLine(baseLine) {
		return baseLine
	}

	afterStart := startCol + overlayWidth
	before, _, after := ExtractSegments(baseLine, startCol, afterStart, totalWidth-afterStart, true)
	overlay := SliceByColumn(overlayLine, 0, overlayWidth, true)

	beforeW := VisibleWidth(before)
	overlayW := VisibleWidth(overlay)
	afterW := VisibleWidth(after)

	beforePad := max(0, startCol-beforeW)
	overlayPad := max(0, overlayWidth-overlayW)
	actualBeforeW := max(startCol, beforeW)
	actualOverlayW := max(overlayWidth, overlayW)
	afterTarget := max(0, totalWidth-actualBeforeW-actualOverlayW)
	afterPad := max(0, afterTarget-afterW)

	result := before + strings.Repeat(" ", beforePad) + SegmentReset +
		overlay + strings.Repeat(" ", overlayPad) + SegmentReset +
		after + strings.Repeat(" ", afterPad)

	resultW := VisibleWidth(result)
	if resultW <= totalWidth {
		return result
	}
	return SliceByColumn(result, 0, totalWidth, true)
}

// applyLineResets applies segment resets to all non-image lines.
func (t *TUI) applyLineResets(lines []string) []string {
	result := make([]string, len(lines))
	for i, line := range lines {
		if !IsImageLine(line) {
			result[i] = NormalizeTerminalOutput(line) + SegmentReset
		} else {
			result[i] = line
		}
	}
	return result
}

// doRender performs the actual rendering pass.
func (t *TUI) doRender() {
	width := t.terminal.Columns()
	height := t.terminal.Rows()
	widthChanged := t.previousWidth != 0 && t.previousWidth != width
	heightChanged := t.previousHeight != 0 && t.previousHeight != height
	prevViewportTop := t.previousViewportTop

	// Render all components
	newLines := t.Container.Render(width)

	// Composite overlays
	if len(t.overlayStack) > 0 {
		newLines = t.compositeOverlays(newLines, width, height)
	}

	// Extract cursor position (row, col, found)
	cursorRow, cursorCol, cursorFound := t.extractCursorPosition(newLines, height)

	newLines = t.applyLineResets(newLines)

	// Helper for full render
	fullRender := func(clear bool) {
		t.fullRedrawCount++
		var buf strings.Builder
		buf.WriteString("\x1b[?2026h") // Begin synchronized output
		if clear {
			// Delete old kitty images
			for id := range t.previousKittyImageIDs {
				buf.WriteString(fmt.Sprintf("\x1b_Ga=d,i=%d\x07", id))
			}
			buf.WriteString("\x1b[2J\x1b[H\x1b[3J")
		}
		for i, line := range newLines {
			if i > 0 {
				buf.WriteString("\r\n")
			}
			buf.WriteString(line)
		}
		buf.WriteString("\x1b[?2026l") // End synchronized output
		t.terminal.Write(buf.String())
		t.cursorRow = max(0, len(newLines)-1)
		t.hardwareCursorRow = t.cursorRow
		if clear {
			t.maxLinesRendered = len(newLines)
		} else {
			t.maxLinesRendered = max(t.maxLinesRendered, len(newLines))
		}
		t.previousViewportTop = max(0, len(newLines)-height)
		t.positionHardwareCursor(cursorRow, cursorCol, cursorFound)
		t.previousLines = newLines
		t.previousKittyImageIDs = t.collectKittyImageIDs(newLines)
		t.previousWidth = width
		t.previousHeight = height
	}

	// First render
	if len(t.previousLines) == 0 && !widthChanged && !heightChanged {
		fullRender(false)
		return
	}

	// Width change
	if widthChanged {
		fullRender(true)
		return
	}

	// Height change (unless Termux)
	if heightChanged {
		fullRender(true)
		return
	}

	// Content shrunk
	if t.clearOnShrink && len(newLines) < t.maxLinesRendered && len(t.overlayStack) == 0 {
		fullRender(true)
		return
	}

	// Differential rendering
	firstChanged, lastChanged := -1, -1
	maxLen := max(len(newLines), len(t.previousLines))
	for i := 0; i < maxLen; i++ {
		var oldLine, newLine string
		if i < len(t.previousLines) {
			oldLine = t.previousLines[i]
		}
		if i < len(newLines) {
			newLine = newLines[i]
		}
		if oldLine != newLine {
			if firstChanged == -1 {
				firstChanged = i
			}
			lastChanged = i
		}
	}
	if len(newLines) > len(t.previousLines) {
		if firstChanged == -1 {
			firstChanged = len(t.previousLines)
		}
		lastChanged = len(newLines) - 1
	}

	if firstChanged == -1 {
		t.positionHardwareCursor(cursorRow, cursorCol, cursorFound)
		t.previousViewportTop = prevViewportTop
		t.previousHeight = height
		return
	}

	// All changes are in deleted lines
	if firstChanged >= len(newLines) {
		if len(t.previousLines) > len(newLines) {
			var buf strings.Builder
			buf.WriteString("\x1b[?2026h")
			targetRow := max(0, len(newLines)-1)
			lineDiff := targetRow - t.hardwareCursorRow
			if lineDiff > 0 {
				buf.WriteString(fmt.Sprintf("\x1b[%dB", lineDiff))
			} else if lineDiff < 0 {
				buf.WriteString(fmt.Sprintf("\x1b[%dA", -lineDiff))
			}
			buf.WriteString("\r")
			extraLines := len(t.previousLines) - len(newLines)
			if extraLines > 0 {
				buf.WriteString("\x1b[1B")
			}
			for i := 0; i < extraLines; i++ {
				buf.WriteString("\r\x1b[2K")
				if i < extraLines-1 {
					buf.WriteString("\x1b[1B")
				}
			}
			if extraLines > 0 {
				buf.WriteString(fmt.Sprintf("\x1b[%dA", extraLines))
			}
			buf.WriteString("\x1b[?2026l")
			t.terminal.Write(buf.String())
			t.cursorRow = targetRow
			t.hardwareCursorRow = targetRow
		}
		t.positionHardwareCursor(cursorRow, cursorCol, cursorFound)
		t.previousLines = newLines
		t.previousKittyImageIDs = t.collectKittyImageIDs(newLines)
		t.previousWidth = width
		t.previousHeight = height
		t.previousViewportTop = prevViewportTop
		return
	}

	// Differential render
	var buf strings.Builder
	buf.WriteString("\x1b[?2026h")
	moveTargetRow := firstChanged
	if moveTargetRow > t.hardwareCursorRow {
		buf.WriteString(fmt.Sprintf("\x1b[%dB", moveTargetRow-t.hardwareCursorRow))
	} else if moveTargetRow < t.hardwareCursorRow {
		buf.WriteString(fmt.Sprintf("\x1b[%dA", t.hardwareCursorRow-moveTargetRow))
	}
	buf.WriteString("\r")

	renderEnd := min(lastChanged, len(newLines)-1)
	for i := firstChanged; i <= renderEnd; i++ {
		if i > firstChanged {
			buf.WriteString("\r\n")
		}
		buf.WriteString("\x1b[2K") // Clear line
		line := newLines[i]
		if !IsImageLine(line) && VisibleWidth(line) > width {
			// Log crash info and stop
			t.Stop()
			panic(fmt.Sprintf("Rendered line %d exceeds terminal width (%d > %d)", i, VisibleWidth(line), width))
		}
		buf.WriteString(line)
	}

	finalCursorRow := renderEnd

	// Clear extra lines if content shrunk
	if len(t.previousLines) > len(newLines) {
		if renderEnd < len(newLines)-1 {
			buf.WriteString(fmt.Sprintf("\x1b[%dB", len(newLines)-1-renderEnd))
			finalCursorRow = len(newLines) - 1
		}
		extraLines := len(t.previousLines) - len(newLines)
		for i := len(newLines); i < len(t.previousLines); i++ {
			buf.WriteString("\r\n\x1b[2K")
		}
		buf.WriteString(fmt.Sprintf("\x1b[%dA", extraLines))
	}

	buf.WriteString("\x1b[?2026l")
	t.terminal.Write(buf.String())

	t.cursorRow = max(0, len(newLines)-1)
	t.hardwareCursorRow = finalCursorRow
	t.maxLinesRendered = max(t.maxLinesRendered, len(newLines))
	t.previousViewportTop = max(prevViewportTop, finalCursorRow-height+1)

	t.positionHardwareCursor(cursorRow, cursorCol, cursorFound)

	t.previousLines = newLines
	t.previousKittyImageIDs = t.collectKittyImageIDs(newLines)
	t.previousWidth = width
	t.previousHeight = height
}

func (t *TUI) extractCursorPosition(lines []string, height int) (int, int, bool) {
	viewportTop := max(0, len(lines)-height)
	for row := len(lines) - 1; row >= viewportTop; row-- {
		line := lines[row]
		idx := strings.Index(line, CursorMarker)
		if idx >= 0 {
			col := VisibleWidth(line[:idx])
			lines[row] = line[:idx] + line[idx+len(CursorMarker):]
			return row, col, true
		}
	}
	return 0, 0, false
}

func (t *TUI) positionHardwareCursor(row, col int, found bool) {
	if !found || len(t.previousLines) <= 0 {
		t.terminal.HideCursor()
		return
	}

	targetRow := max(0, min(row, len(t.previousLines)-1))
	targetCol := max(0, col)

	rowDelta := targetRow - t.hardwareCursorRow
	var buf strings.Builder
	if rowDelta > 0 {
		buf.WriteString(fmt.Sprintf("\x1b[%dB", rowDelta))
	} else if rowDelta < 0 {
		buf.WriteString(fmt.Sprintf("\x1b[%dA", -rowDelta))
	}
	buf.WriteString(fmt.Sprintf("\x1b[%dG", targetCol+1))

	if buf.Len() > 0 {
		t.terminal.Write(buf.String())
	}

	t.hardwareCursorRow = targetRow
	if t.showHardwareCursor {
		t.terminal.ShowCursor()
	} else {
		t.terminal.HideCursor()
	}
}

func (t *TUI) collectKittyImageIDs(lines []string) map[int]struct{} {
	ids := make(map[int]struct{})
	for _, line := range lines {
		// Extract Kitty image IDs from lines
		// Format: \x1b_Ga=d,i=<id>,...
		if strings.HasPrefix(line, "\x1b_G") {
			parts := strings.Split(line, ";")
			for _, part := range parts {
				if strings.HasPrefix(part, "i=") {
					idStr := strings.Split(part, ",")[0][2:]
					if id, err := strconv.Atoi(idStr); err == nil {
						ids[id] = struct{}{}
					}
				}
			}
		}
	}
	return ids
}

// SetOnDebug sets the debug key callback (Shift+Ctrl+D).
func (t *TUI) SetOnDebug(fn func()) {
	t.onDebug = fn
}

package codecui

import (
	"strings"
	"sync"
	"time"
)

// DaxnutState represents Daxnuts' emotional state.
type DaxnutState int

const (
	DaxnutIdle     DaxnutState = iota
	DaxnutHappy
	DaxnutConfused
	DaxnutExcited
	DaxnutSleepy
)

// Daxnuts is an animated ASCII cat character.
type Daxnuts struct {
	state    DaxnutState
	frame    int
	mu       sync.Mutex
	active   bool
	stopChan chan struct{}
	message  string
}

// NewDaxnuts creates a new Daxnuts character.
func NewDaxnuts() *Daxnuts {
	return &Daxnuts{
		state:    DaxnutIdle,
		stopChan: make(chan struct{}),
	}
}

// SetState changes Daxnuts' visual state.
func (d *Daxnuts) SetState(state DaxnutState) {
	d.mu.Lock()
	d.state = state
	d.mu.Unlock()
}

// SetMessage sets the message.
func (d *Daxnuts) SetMessage(msg string) {
	d.mu.Lock()
	d.message = msg
	d.mu.Unlock()
}

// Start begins animation.
func (d *Daxnuts) Start() {
	d.mu.Lock()
	if d.active {
		d.mu.Unlock()
		return
	}
	d.active = true
	d.mu.Unlock()

	go func() {
		ticker := time.NewTicker(600 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				d.mu.Lock()
				d.frame = (d.frame + 1) % 3
				d.mu.Unlock()
			case <-d.stopChan:
				return
			}
		}
	}()
}

// Stop stops animation.
func (d *Daxnuts) Stop() {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.active {
		d.active = false
		close(d.stopChan)
		d.stopChan = make(chan struct{})
	}
}

// Render returns the current frame.
func (d *Daxnuts) Render(width int) []string {
	d.mu.Lock()
	state := d.state
	frame := d.frame
	msg := d.message
	d.mu.Unlock()

	states := daxnutFrames(state)
	f := states[frame%len(states)]

	var lines []string
	lines = append(lines, f...)
	if msg != "" {
		lines = append(lines, "  "+msg)
	}
	return lines
}

func daxnutFrames(state DaxnutState) [][]string {
	switch state {
	case DaxnutHappy:
		return [][]string{
			{
				"  /\\_/\\",
				" ( ^_^ )",
				"  > ^ <",
			},
			{
				"  /\\_/\\",
				" ( ^_^ )",
				"  > ^ <  ♪",
			},
			{
				"  /\\_/\\",
				" ( ^_^ )~",
				"  > ^ <",
			},
		}

	case DaxnutConfused:
		return [][]string{
			{
				"  /\\_/\\",
				" ( o_o ) ?",
				"  > ^ <",
			},
			{
				"  /\\_/\\",
				" ( ?_? )",
				"  > ^ <",
			},
			{
				"  /\\_/\\",
				" ( o_o )",
				"  > ^ <  ?",
			},
		}

	case DaxnutExcited:
		return [][]string{
			{
				"  /\\_/\\",
				" ( @_@ )",
				"  > ^ <  !!!",
			},
			{
				"  /\\_/\\",
				" ( @_@ )!!",
				"  > ^ <",
			},
			{
				"  /\\_/\\",
				" ( ★_★ )",
				"  > ^ <",
			},
		}

	case DaxnutSleepy:
		return [][]string{
			{
				"  /\\_/\\",
				" ( -_- ) zzz",
				"  > ^ <",
			},
			{
				"  /\\_/\\",
				" ( -_- )",
				"  > ^ <  zzz",
			},
			{
				"  /\\_/\\",
				" ( u_u )",
				"  > ^ <",
			},
		}

	default: // DaxnutIdle
		return [][]string{
			{
				"  /\\_/\\",
				" ( ._. )",
				"  > ^ <",
			},
			{
				"  /\\_/\\",
				" ( ._. )",
				"  > ^ <",
			},
			{
				"  /\\_/\\",
				" ( -_- )",
				"  > ^ <",
			},
		}
	}
}

var _ = strings.Repeat

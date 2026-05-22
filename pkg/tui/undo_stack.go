package tui

// UndoStack is a generic undo stack.
type UndoStack[S any] struct {
	stack []S
}

// NewUndoStack creates a new UndoStack.
func NewUndoStack[S any]() *UndoStack[S] {
	return &UndoStack[S]{
		stack: make([]S, 0),
	}
}

// Push adds a state to the stack.
// The caller should ensure the state is a deep copy if necessary.
func (us *UndoStack[S]) Push(state S) {
	us.stack = append(us.stack, state)
}

// Pop removes and returns the most recent state, or false if empty.
func (us *UndoStack[S]) Pop() (S, bool) {
	var empty S
	if len(us.stack) == 0 {
		return empty, false
	}
	lastIdx := len(us.stack) - 1
	last := us.stack[lastIdx]
	us.stack = us.stack[:lastIdx]
	return last, true
}

// Clear removes all states.
func (us *UndoStack[S]) Clear() {
	us.stack = us.stack[:0]
}

// Length returns the number of states.
func (us *UndoStack[S]) Length() int {
	return len(us.stack)
}

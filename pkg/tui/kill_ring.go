package tui

// KillRing provides an Emacs-style ring buffer for kill/yank operations.
type KillRing struct {
	ring []string
}

// NewKillRing creates a new KillRing.
func NewKillRing() *KillRing {
	return &KillRing{
		ring: make([]string, 0),
	}
}

// Push adds text to the kill ring. If accumulate is true, it merges with the last entry.
func (kr *KillRing) Push(text string, prepend bool, accumulate bool) {
	if text == "" {
		return
	}

	if accumulate && len(kr.ring) > 0 {
		lastIdx := len(kr.ring) - 1
		last := kr.ring[lastIdx]
		if prepend {
			kr.ring[lastIdx] = text + last
		} else {
			kr.ring[lastIdx] = last + text
		}
	} else {
		kr.ring = append(kr.ring, text)
	}
}

// Peek gets the most recent entry without removing it.
func (kr *KillRing) Peek() string {
	if len(kr.ring) == 0 {
		return ""
	}
	return kr.ring[len(kr.ring)-1]
}

// Rotate moves the last entry to the front for yank-pop.
func (kr *KillRing) Rotate() {
	if len(kr.ring) > 1 {
		lastIdx := len(kr.ring) - 1
		last := kr.ring[lastIdx]
		kr.ring = kr.ring[:lastIdx]

		// Unshift
		kr.ring = append([]string{last}, kr.ring...)
	}
}

// Length returns the number of items in the ring.
func (kr *KillRing) Length() int {
	return len(kr.ring)
}

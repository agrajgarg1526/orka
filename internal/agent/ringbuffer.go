package agent

import "sync"

// RingBuffer stores the last N lines of agent output.
type RingBuffer struct {
	mu   sync.Mutex
	buf  []string
	size int
	pos  int
	full bool
}

func NewRingBuffer(size int) *RingBuffer {
	return &RingBuffer{buf: make([]string, size), size: size}
}

func (r *RingBuffer) Add(line string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.buf[r.pos] = line
	r.pos = (r.pos + 1) % r.size
	if r.pos == 0 {
		r.full = true
	}
}

// Lines returns buffered lines in order (oldest first).
func (r *RingBuffer) Lines() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.full {
		out := make([]string, r.pos)
		copy(out, r.buf[:r.pos])
		return out
	}
	out := make([]string, r.size)
	copy(out, r.buf[r.pos:])
	copy(out[r.size-r.pos:], r.buf[:r.pos])
	return out
}

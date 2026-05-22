package agent_test

import (
	"testing"

	"github.com/agrajgarg/orka/internal/agent"
)

func TestRingBufferOverflow(t *testing.T) {
	rb := agent.NewRingBuffer(3)
	rb.Add("line1")
	rb.Add("line2")
	rb.Add("line3")
	rb.Add("line4")

	lines := rb.Lines()
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d", len(lines))
	}
	if lines[0] != "line2" {
		t.Errorf("expected line2, got %q", lines[0])
	}
	if lines[2] != "line4" {
		t.Errorf("expected line4, got %q", lines[2])
	}
}

func TestRingBufferEmpty(t *testing.T) {
	rb := agent.NewRingBuffer(10)
	if lines := rb.Lines(); len(lines) != 0 {
		t.Errorf("expected empty, got %v", lines)
	}
}

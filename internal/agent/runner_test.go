package agent_test

import (
	"strings"
	"testing"
	"time"

	"github.com/agrajgarg/orka/internal/agent"
)

func TestRunnerRunsAndCapturesOutput(t *testing.T) {
	r := agent.NewRunner()
	done := make(chan error, 1)
	err := r.Start("echo", []string{"hello orka"}, "/tmp", done)
	if err != nil {
		t.Fatalf("start: %v", err)
	}

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for process")
	}

	lines := r.Lines()
	if len(lines) == 0 {
		t.Fatal("expected output lines")
	}
	combined := strings.Join(lines, " ")
	if !strings.Contains(combined, "hello orka") {
		t.Errorf("expected 'hello orka' in output, got: %v", lines)
	}
}

func TestRunnerStopKillsProcess(t *testing.T) {
	r := agent.NewRunner()
	done := make(chan error, 1)
	err := r.Start("sleep", []string{"60"}, "/tmp", done)
	if err != nil {
		t.Fatalf("start: %v", err)
	}

	if err := r.Stop(); err != nil {
		t.Fatalf("stop: %v", err)
	}

	select {
	case <-done:
		// process exited — pass
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for stop")
	}
}

package state_test

import (
	"path/filepath"
	"testing"

	"github.com/agrajgarg/orka/internal/state"
)

func TestLoadSaveRoundtrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	s := state.New()
	p := s.AddProject("my-app", "/projects/my-app")
	task := s.AddTask(p.ID, "Fix auth bug", "claude-code", "superpowers", false)

	if err := s.Save(path); err != nil {
		t.Fatalf("save: %v", err)
	}

	s2, err := state.Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	if len(s2.Tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(s2.Tasks))
	}
	if s2.Tasks[0].Title != task.Title {
		t.Errorf("title mismatch: got %q", s2.Tasks[0].Title)
	}
	if s2.Tasks[0].Phase != state.PhaseToBePicked {
		t.Errorf("expected ToBePicked phase, got %q", s2.Tasks[0].Phase)
	}
}

func TestLoadMissingFileReturnsEmpty(t *testing.T) {
	s, err := state.Load("/nonexistent/path/state.json")
	if err != nil {
		t.Fatalf("expected no error for missing file, got %v", err)
	}
	if len(s.Tasks) != 0 {
		t.Fatalf("expected empty tasks")
	}
}

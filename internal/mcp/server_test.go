package mcp_test

import (
	"encoding/json"
	"testing"

	"github.com/agrajgarg/orka/internal/state"
)

func TestListTasksReturnsAllTasks(t *testing.T) {
	st := state.New()
	p := st.AddProject("test", "/tmp/test")
	st.AddTask(p.ID, "Fix bug", "claude-code", "none", false)

	tasks := st.Tasks
	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}

	data, _ := json.Marshal(map[string]interface{}{"tasks": tasks})
	var out map[string]interface{}
	_ = json.Unmarshal(data, &out)
	if _, ok := out["tasks"]; !ok {
		t.Error("expected tasks key in response")
	}
}

func TestCompletePhaseAdvancesTask(t *testing.T) {
	st := state.New()
	p := st.AddProject("test", "/tmp/test")
	task := st.AddTask(p.ID, "Fix bug", "claude-code", "none", false)
	taskID := task.ID

	for i := range st.Tasks {
		if st.Tasks[i].ID == taskID {
			next := st.Tasks[i].NextPhase()
			if next == "" {
				t.Fatal("expected a next phase")
			}
			st.UpdateTaskPhase(taskID, next)
			break
		}
	}

	var found *state.Task
	for i := range st.Tasks {
		if st.Tasks[i].ID == taskID {
			found = &st.Tasks[i]
			break
		}
	}
	if found == nil {
		t.Fatal("task not found")
	}
	// skip_research=false so next phase after ToBePicked is Research
	if found.Phase != state.PhaseResearch {
		t.Errorf("expected Research, got %q", found.Phase)
	}
}

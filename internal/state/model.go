package state

import "time"

type Phase string

const (
	PhaseToBePicked Phase = "to_be_picked"
	PhaseResearch   Phase = "research"
	PhasePlanning   Phase = "planning"
	PhaseRunning    Phase = "running"
	PhaseReview     Phase = "review"
	PhaseDone       Phase = "done"
)

var PhaseOrder = []Phase{
	PhaseToBePicked,
	PhaseResearch,
	PhasePlanning,
	PhaseRunning,
	PhaseReview,
	PhaseDone,
}

type Project struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Path      string    `json:"path"`
	CreatedAt time.Time `json:"created_at"`
}

type Task struct {
	ID             string    `json:"id"`
	ProjectID      string    `json:"project_id"`
	Title          string    `json:"title"`
	Description    string    `json:"description"`
	Phase          Phase     `json:"phase"`
	Agent          string    `json:"agent"`
	Plugin         string    `json:"plugin"`
	Branch         string    `json:"branch"`
	AutoRun        bool      `json:"auto_run"`
	PRBaseBranch   string    `json:"pr_base_branch"`
	PRURL          string    `json:"pr_url"`
	Notes          string    `json:"notes"`
	SkipResearch   bool      `json:"skip_research"`
	SessionStarted bool      `json:"session_started"`
	CreatedAt      time.Time `json:"created_at"`
	PhaseStartedAt time.Time `json:"phase_started_at"`
	Error          *string   `json:"error"`
}

// NextPhase returns the next phase for a task, skipping Research if SkipResearch is true.
// Returns "" if already at Done.
func (t *Task) NextPhase() Phase {
	for i, p := range PhaseOrder {
		if p == t.Phase {
			for j := i + 1; j < len(PhaseOrder); j++ {
				next := PhaseOrder[j]
				if next == PhaseResearch && t.SkipResearch {
					continue
				}
				return next
			}
		}
	}
	return ""
}

// PrevPhase returns the previous phase, skipping Research if SkipResearch is true.
// Returns "" if already at ToBePicked.
func (t *Task) PrevPhase() Phase {
	for i, p := range PhaseOrder {
		if p == t.Phase {
			for j := i - 1; j >= 0; j-- {
				prev := PhaseOrder[j]
				if prev == PhaseResearch && t.SkipResearch {
					continue
				}
				return prev
			}
		}
	}
	return ""
}

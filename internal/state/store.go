package state

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
)

type State struct {
	Version  int       `json:"version"`
	Projects []Project `json:"projects"`
	Tasks    []Task    `json:"tasks"`
}

func New() *State {
	return &State{Version: 1}
}

func Load(path string) (*State, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return New(), nil
		}
		return nil, err
	}
	var s State
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

func (s *State) Save(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func (s *State) AddProject(name, path string) *Project {
	p := Project{
		ID:        uuid.NewString(),
		Name:      name,
		Path:      path,
		CreatedAt: time.Now(),
	}
	s.Projects = append(s.Projects, p)
	return &s.Projects[len(s.Projects)-1]
}

func (s *State) AddTask(projectID, title, description, branch, agent, plugin string, skipResearch bool) *Task {
	now := time.Now()
	t := Task{
		ID:             uuid.NewString(),
		ProjectID:      projectID,
		Title:          title,
		Description:    description,
		Branch:         branch,
		Phase:          PhaseToBePicked,
		Agent:          agent,
		Plugin:         plugin,
		SkipResearch:   skipResearch,
		CreatedAt:      now,
		PhaseStartedAt: now,
	}
	s.Tasks = append(s.Tasks, t)
	return &s.Tasks[len(s.Tasks)-1]
}

func (s *State) TasksByPhase(projectID string, phase Phase) []Task {
	var result []Task
	for _, t := range s.Tasks {
		if t.ProjectID == projectID && t.Phase == phase {
			result = append(result, t)
		}
	}
	return result
}

func (s *State) UpdateTaskPhase(taskID string, phase Phase) {
	for i := range s.Tasks {
		if s.Tasks[i].ID == taskID {
			s.Tasks[i].Phase = phase
			s.Tasks[i].PhaseStartedAt = time.Now()
			s.Tasks[i].Error = nil
			return
		}
	}
}

func (s *State) SetTaskError(taskID, msg string) {
	for i := range s.Tasks {
		if s.Tasks[i].ID == taskID {
			s.Tasks[i].Error = &msg
			return
		}
	}
}

func (s *State) RemoveProject(projectID string) {
	var projects []Project
	for _, p := range s.Projects {
		if p.ID != projectID {
			projects = append(projects, p)
		}
	}
	s.Projects = projects
	// also remove all tasks belonging to this project
	var tasks []Task
	for _, t := range s.Tasks {
		if t.ProjectID != projectID {
			tasks = append(tasks, t)
		}
	}
	s.Tasks = tasks
}

func DefaultStatePath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "orka", "state.json")
}

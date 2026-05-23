package config

import (
	"fmt"
	"strings"

	"github.com/agrajgarg/orka/internal/state"
)

var pluginPresets = map[string]map[state.Phase]string{
	"superpowers": {
		state.PhaseResearch: "/research {title}\n{description}",
		state.PhasePlanning: "/plan {title}\n{description}",
		state.PhaseRunning:  "/implement {title}\n{description}",
		state.PhaseReview:   "/review {title}\n{description}",
	},
	"gsd": {
		state.PhaseResearch: "/gsd research {title}\n{description}",
		state.PhasePlanning: "/gsd plan {title}\n{description}",
		state.PhaseRunning:  "/gsd run {title}\n{description}",
		state.PhaseReview:   "/gsd review {title}\n{description}",
	},
}

var defaultPrompts = map[state.Phase]string{
	state.PhaseResearch: "Research the following task and summarize your findings: {title}\n{description}",
	state.PhasePlanning: "Create a detailed implementation plan for: {title}\n{description}",
	state.PhaseRunning:  "Implement the following task: {title}\n{description}",
	state.PhaseReview:   "Review the changes made for: {title}\n{description}",
}

// ResolvePrompt returns the prompt to send to the agent for the given task and phase.
// Priority: toml override > plugin preset > hardcoded default.
func (c *Config) ResolvePrompt(task *state.Task, phase state.Phase) string {
	phaseStr := string(phase)

	if agentOverrides, ok := c.overrides[task.Agent]; ok {
		if tmpl, ok := agentOverrides[phaseStr]; ok {
			return interpolate(tmpl, task)
		}
	}

	if preset, ok := pluginPresets[task.Plugin]; ok {
		if tmpl, ok := preset[phase]; ok {
			return interpolate(tmpl, task)
		}
	}

	tmpl, ok := defaultPrompts[phase]
	if !ok {
		return fmt.Sprintf("Work on: %s\n%s", task.Title, task.Description)
	}
	return interpolate(tmpl, task)
}

func interpolate(tmpl string, task *state.Task) string {
	r := strings.NewReplacer(
		"{title}", task.Title,
		"{description}", task.Description,
		"{notes}", task.Notes,
	)
	return r.Replace(tmpl)
}

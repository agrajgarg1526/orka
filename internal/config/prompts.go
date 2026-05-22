package config

import (
	"fmt"
	"strings"

	"github.com/agrajgarg/orka/internal/state"
)

var pluginPresets = map[string]map[state.Phase]string{
	"superpowers": {
		state.PhaseResearch: "/research {title}\n{notes}",
		state.PhasePlanning: "/plan {title}\n{notes}",
		state.PhaseRunning:  "/implement {title}\n{notes}",
		state.PhaseReview:   "/review {title}\n{notes}",
	},
	"gsd": {
		state.PhaseResearch: "/gsd research {title}\n{notes}",
		state.PhasePlanning: "/gsd plan {title}\n{notes}",
		state.PhaseRunning:  "/gsd run {title}\n{notes}",
		state.PhaseReview:   "/gsd review {title}\n{notes}",
	},
}

var defaultPrompts = map[state.Phase]string{
	state.PhaseResearch: "Research the following task and summarize your findings: {title}\n{notes}",
	state.PhasePlanning: "Create a detailed implementation plan for: {title}\n{notes}",
	state.PhaseRunning:  "Implement the following task: {title}\n{notes}",
	state.PhaseReview:   "Review the changes made for: {title}\n{notes}",
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
		return fmt.Sprintf("Work on: %s\n%s", task.Title, task.Notes)
	}
	return interpolate(tmpl, task)
}

func interpolate(tmpl string, task *state.Task) string {
	r := strings.NewReplacer(
		"{title}", task.Title,
		"{notes}", task.Notes,
	)
	return r.Replace(tmpl)
}

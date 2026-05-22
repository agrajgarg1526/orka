package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/agrajgarg/orka/internal/config"
	"github.com/agrajgarg/orka/internal/state"
)

func TestResolvePromptSuperpowersPreset(t *testing.T) {
	cfg := config.Default()
	task := &state.Task{
		Title:  "Fix auth bug",
		Notes:  "Check token validation",
		Plugin: "superpowers",
	}
	got := cfg.ResolvePrompt(task, state.PhaseRunning)
	want := "/implement Fix auth bug\nCheck token validation"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestResolvePromptCustomOverride(t *testing.T) {
	dir := t.TempDir()
	tomlPath := filepath.Join(dir, "prompts.toml")
	os.WriteFile(tomlPath, []byte(`
[claude-code]
running = "do it: {title}"
`), 0644)

	cfg, err := config.Load(tomlPath)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	task := &state.Task{
		Title:  "Fix auth bug",
		Agent:  "claude-code",
		Plugin: "superpowers",
	}
	got := cfg.ResolvePrompt(task, state.PhaseRunning)
	want := "do it: Fix auth bug"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestResolvePromptNonePluginFallsBackToDefault(t *testing.T) {
	cfg := config.Default()
	task := &state.Task{
		Title:  "Fix auth bug",
		Notes:  "",
		Plugin: "none",
	}
	got := cfg.ResolvePrompt(task, state.PhasePlanning)
	want := "Create a detailed implementation plan for: Fix auth bug\n"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

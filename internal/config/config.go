package config

import (
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// TomlConfig holds per-agent prompt overrides from prompts.toml.
type TomlConfig map[string]map[string]string

// Config holds resolved configuration.
type Config struct {
	overrides TomlConfig
}

func Default() *Config {
	return &Config{overrides: TomlConfig{}}
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Default(), nil
		}
		return nil, err
	}
	var overrides TomlConfig
	if _, err := toml.Decode(string(data), &overrides); err != nil {
		return nil, err
	}
	return &Config{overrides: overrides}, nil
}

func DefaultConfigPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "orka", "prompts.toml")
}

func WriteDefaultPrompts(path string) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	content := `# orka prompt overrides
# Uncomment and edit to override prompts for any agent/phase.
# {title} and {notes} are substituted at runtime.

# [claude-code]
# research = "Research this task: {title}\n{notes}"
# planning = "Create a plan for: {title}\n{notes}"
# running  = "Implement this task: {title}\n{notes}"
# review   = "Review the changes for: {title}\n{notes}"
`
	return os.WriteFile(path, []byte(content), 0644)
}

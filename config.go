package redo

import (
	"fmt"
	"os"

	"github.com/bmatcuk/doublestar/v4"
	"gopkg.in/yaml.v3"
)

// Config for redo.
type Config struct {
	Commands []CommandConfig `yaml:"commands"`
}

// CommandConfig for a single command in the config.
type CommandConfig struct {
	Name  string   `yaml:"name"`
	Run   string   `yaml:"run"`
	Watch []string `yaml:"watch"`
}

// LoadConfig from the given path.
func LoadConfig(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parsing config: %w", err)
	}

	if err := cfg.validate(); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func (c Config) validate() error {
	if len(c.Commands) == 0 {
		return fmt.Errorf("no commands defined")
	}
	seen := make(map[string]bool)
	for i, cmd := range c.Commands {
		if cmd.Name == "" {
			return fmt.Errorf("command %d: missing name", i)
		}
		if seen[cmd.Name] {
			return fmt.Errorf("command %q: duplicate name", cmd.Name)
		}
		seen[cmd.Name] = true
		if cmd.Run == "" {
			return fmt.Errorf("command %q: missing run", cmd.Name)
		}
		if len(cmd.Watch) == 0 {
			return fmt.Errorf("command %q: missing watch patterns", cmd.Name)
		}
		for _, pattern := range cmd.Watch {
			if _, err := doublestar.Match(pattern, ""); err != nil {
				return fmt.Errorf("command %q: invalid watch pattern %q: %w", cmd.Name, pattern, err)
			}
		}
	}
	return nil
}

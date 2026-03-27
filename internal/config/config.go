package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type DatasetConfig struct {
	Connector    string            `yaml:"connector"`
	PageSize     int               `yaml:"page_size"`
	MaxErrorRate float64           `yaml:"max_error_rate"`
	Params       map[string]string `yaml:"params"`
}

type AlgorithmConfig struct {
	Type string `yaml:"type"`
}

type OutputConfig struct {
	Writer string `yaml:"writer"`
}

type RunConfig struct {
	TimeoutSeconds int `yaml:"timeout_seconds"`
}

type Config struct {
	Datasets   []DatasetConfig `yaml:"datasets"`
	KeyColumns []string        `yaml:"key_columns"`
	Algorithm  AlgorithmConfig `yaml:"algorithm"`
	Output     OutputConfig    `yaml:"output"`
	Run        RunConfig       `yaml:"run"`
}

func (c Config) validate() error {
	if len(c.KeyColumns) == 0 {
		return fmt.Errorf("key_columns is required")
	}
	if len(c.Datasets) < 2 {
		return fmt.Errorf("at least 2 datasets are required")
	}
	if c.Algorithm.Type == "" {
		return fmt.Errorf("algorithm.type is required")
	}
	if c.Output.Writer == "" {
		return fmt.Errorf("output.writer is required")
	}
	return nil
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("could not read config file: %s: %w", path, err)
	}

	expanded := os.Expand(string(data), os.Getenv)

	var cfg Config
	if err := yaml.Unmarshal([]byte(expanded), &cfg); err != nil {
		return nil, fmt.Errorf("could not parse config file %s: %w", path, err)
	}

	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return &cfg, nil
}

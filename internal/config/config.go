package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const ConfigFileName = "config.yaml"

type Config struct {
	Port      string `yaml:"port"`
	DB        string `yaml:"db"`
	APIKey    string `yaml:"api_key"`
	Retention int    `yaml:"retention"`
}

func DataDir() string {
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".agent-chat")
	}
	return ""
}

func DefaultConfigPath() string {
	return filepath.Join(DataDir(), ConfigFileName)
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	return &cfg, nil
}

func Exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

package config

import (
	"fmt"
	"os"

	"github.com/BurntSushi/toml"
)

type Resources struct {
	CPU    string `toml:"cpu"`
	Memory string `toml:"memory"`
}

type FileMapping struct {
	Local  string `toml:"local"`
	Remote string `toml:"remote"`
}

type Config struct {
	Workspace       string        `toml:"workspace"`
	Image           string        `toml:"image"`
	ImagePullPolicy string        `toml:"image_pull_policy"`
	Namespace       string        `toml:"namespace"`
	Kubecontext     string        `toml:"kubecontext"`
	SetupScript     string        `toml:"setup_script"`
	Packages        []string      `toml:"packages"`
	Resources       Resources     `toml:"resources"`
	Credentials     []FileMapping `toml:"credentials"`
	EnvVars         []string      `toml:"env_vars"`
	SSHAuthorizedKey string       `toml:"ssh_authorized_key"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	var cfg Config
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config file: %w", err)
	}

	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	cfg.applyDefaults()
	return &cfg, nil
}

func (c *Config) validate() error {
	if c.Workspace == "" {
		return fmt.Errorf("workspace is required")
	}
	return nil
}

func (c *Config) applyDefaults() {
	if c.Image == "" {
		c.Image = "ghcr.io/007/yolopod-base:latest"
	}
	if c.Namespace == "" {
		c.Namespace = "default"
	}
	if c.Resources.CPU == "" {
		c.Resources.CPU = "2"
	}
	if c.Resources.Memory == "" {
		c.Resources.Memory = "4Gi"
	}
}

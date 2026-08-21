package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type Record struct {
	Zone     string `yaml:"zone"`
	Hostname string `yaml:"hostname"`
}

type CloudflareConfig struct {
	APIToken string `yaml:"api_token"`
}

type Config struct {
	Cloudflare    CloudflareConfig `yaml:"cloudflare"`
	CheckInterval string           `yaml:"check_interval"`
	Records       []Record         `yaml:"records"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file %q: %w", path, err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config file %q: %w", path, err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func (c *Config) Validate() error {
	if c.Cloudflare.APIToken == "" {
		return fmt.Errorf("cloudflare.api_token is required")
	}
	if len(c.Records) == 0 {
		return fmt.Errorf("records must have at least one entry")
	}
	for i, r := range c.Records {
		if r.Zone == "" {
			return fmt.Errorf("records[%d].zone is required", i)
		}
		if r.Hostname == "" {
			return fmt.Errorf("records[%d].hostname is required", i)
		}
	}
	if _, err := c.Interval(); err != nil {
		return err
	}
	return nil
}

func (c *Config) Interval() (time.Duration, error) {
	d, err := time.ParseDuration(c.CheckInterval)
	if err != nil {
		return 0, fmt.Errorf("check_interval %q is not a valid duration: %w", c.CheckInterval, err)
	}
	if d <= 0 {
		return 0, fmt.Errorf("check_interval must be positive, got %q", c.CheckInterval)
	}
	return d, nil
}

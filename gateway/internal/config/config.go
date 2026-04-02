package config

import (
	"bytes"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type RuntimeConfig struct {
	Server   ServerConfig   `yaml:"server"`
	Upstream UpstreamConfig `yaml:"upstream"`
	Defaults RootDefaults   `yaml:"defaults"`
	Tokens   []TokenConfig  `yaml:"tokens"`
}

type ServerConfig struct {
	Port int `yaml:"port"`
}

type UpstreamConfig struct {
	BaseURL        string `yaml:"base_url"`
	TimeoutSeconds int    `yaml:"timeout_seconds"`
}

type RootDefaults struct {
	Thread        int    `yaml:"thread"`
	ShardLength   int    `yaml:"shard_length"`
	MaxTextLength int    `yaml:"max_text_length"`
}

type TokenDefaults struct {
	Voice         string `yaml:"voice"`
	Thread        int    `yaml:"thread"`
	ShardLength   int    `yaml:"shard_length"`
	MaxTextLength int    `yaml:"max_text_length"`
}

type TokenConfig struct {
	Name          string        `yaml:"name"`
	Token         string        `yaml:"token"`
	Enabled       bool          `yaml:"enabled"`
	Defaults      TokenDefaults `yaml:"defaults"`
	AllowedVoices []string      `yaml:"allowed_voices"`
}

func Load(path string) (*RuntimeConfig, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	var cfg RuntimeConfig
	decoder := yaml.NewDecoder(bytes.NewReader(body))
	decoder.KnownFields(true)
	if err := decoder.Decode(&cfg); err != nil {
		return nil, fmt.Errorf("decode yaml: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func (c *RuntimeConfig) Validate() error {
	if c.Server.Port <= 0 {
		return fmt.Errorf("server.port must be greater than zero")
	}

	if c.Upstream.BaseURL == "" {
		return fmt.Errorf("upstream.base_url is required")
	}

	if c.Upstream.TimeoutSeconds <= 0 {
		return fmt.Errorf("upstream.timeout_seconds must be greater than zero")
	}

	if c.Defaults.Thread <= 0 {
		return fmt.Errorf("defaults.thread must be greater than zero")
	}

	if c.Defaults.ShardLength <= 0 {
		return fmt.Errorf("defaults.shard_length must be greater than zero")
	}

	if c.Defaults.MaxTextLength <= 0 {
		return fmt.Errorf("defaults.max_text_length must be greater than zero")
	}

	if len(c.Tokens) == 0 {
		return fmt.Errorf("at least one token is required")
	}

	for i, token := range c.Tokens {
		if token.Name == "" {
			return fmt.Errorf("tokens[%d].name is required", i)
		}
		if token.Token == "" {
			return fmt.Errorf("tokens[%d].token is required", i)
		}
		if token.Defaults.Voice == "" {
			return fmt.Errorf("tokens[%d].defaults.voice is required", i)
		}
		if token.Defaults.Thread <= 0 {
			return fmt.Errorf("tokens[%d].defaults.thread must be greater than zero", i)
		}
		if token.Defaults.ShardLength <= 0 {
			return fmt.Errorf("tokens[%d].defaults.shard_length must be greater than zero", i)
		}
		if token.Defaults.MaxTextLength <= 0 {
			return fmt.Errorf("tokens[%d].defaults.max_text_length must be greater than zero", i)
		}
		if len(token.AllowedVoices) == 0 {
			return fmt.Errorf("tokens[%d].allowed_voices must contain at least one item", i)
		}
		if !contains(token.AllowedVoices, token.Defaults.Voice) {
			return fmt.Errorf("tokens[%d].defaults.voice must be included in allowed_voices", i)
		}
	}

	return nil
}

func contains(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}

	return false
}

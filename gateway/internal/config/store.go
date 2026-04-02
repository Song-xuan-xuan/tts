package config

import (
	"fmt"
	"sync/atomic"
)

type Store struct {
	path string
	ptr  atomic.Pointer[RuntimeConfig]
}

func NewStore(path string) (*Store, error) {
	cfg, err := Load(path)
	if err != nil {
		return nil, err
	}

	store := &Store{path: path}
	store.ptr.Store(cfg)

	return store, nil
}

func (s *Store) Current() *RuntimeConfig {
	cfg := s.ptr.Load()
	if cfg == nil {
		return nil
	}

	return cloneRuntimeConfig(cfg)
}

func (s *Store) Reload() error {
	cfg, err := Load(s.path)
	if err != nil {
		return fmt.Errorf("reload config: %w", err)
	}

	s.ptr.Store(cfg)

	return nil
}

func cloneRuntimeConfig(cfg *RuntimeConfig) *RuntimeConfig {
	clone := *cfg
	if len(cfg.Tokens) == 0 {
		return &clone
	}

	clone.Tokens = make([]TokenConfig, len(cfg.Tokens))
	for i, token := range cfg.Tokens {
		tokenClone := token
		if len(token.AllowedVoices) > 0 {
			tokenClone.AllowedVoices = append([]string(nil), token.AllowedVoices...)
		}
		clone.Tokens[i] = tokenClone
	}

	return &clone
}

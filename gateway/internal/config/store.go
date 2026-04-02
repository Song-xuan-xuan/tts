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
	return s.ptr.Load()
}

func (s *Store) Reload() error {
	cfg, err := Load(s.path)
	if err != nil {
		return fmt.Errorf("reload config: %w", err)
	}

	s.ptr.Store(cfg)

	return nil
}

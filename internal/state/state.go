package state

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const Version = 1

type State struct {
	Version      int                    `json:"version"`
	Actor        string                 `json:"actor"`
	LastSync     time.Time              `json:"last_sync,omitempty"`
	Roots        map[string]RootMapping `json:"roots"`
	ImportedURIs map[string]string      `json:"imported_uris"`
}

type RootMapping struct {
	Slug        string    `json:"slug"`
	ContentType string    `json:"content_type"`
	PostCount   int       `json:"post_count"`
	LastSeen    time.Time `json:"last_seen,omitempty"`
}

func New(actor string) *State {
	return &State{Version: Version, Actor: actor, Roots: map[string]RootMapping{}, ImportedURIs: map[string]string{}}
}

func Load(path, actor string) (*State, error) {
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return New(actor), nil
	}
	if err != nil {
		return nil, err
	}
	var s State
	if err := json.Unmarshal(b, &s); err != nil {
		return nil, fmt.Errorf("read state: %w", err)
	}
	if s.Version != Version {
		return nil, fmt.Errorf("unsupported state version %d", s.Version)
	}
	if s.Roots == nil {
		s.Roots = map[string]RootMapping{}
	}
	if s.ImportedURIs == nil {
		s.ImportedURIs = map[string]string{}
	}
	return &s, nil
}

func Save(path string, s *State) error {
	s.Version = Version
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	existing, err := os.ReadFile(path)
	if err == nil && string(existing) == string(b) {
		return nil
	}
	return os.WriteFile(path, b, 0o644)
}

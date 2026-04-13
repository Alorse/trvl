package providers

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Registry stores and manages provider configurations on disk.
type Registry struct {
	dir     string
	configs map[string]*ProviderConfig
	mu      sync.RWMutex
}

// NewRegistry creates a Registry backed by ~/.trvl/providers/.
// The directory is created if it does not exist, and all *.json files
// in that directory are loaded into memory.
func NewRegistry() (*Registry, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("providers: user home dir: %w", err)
	}
	dir := filepath.Join(home, ".trvl", "providers")
	return NewRegistryAt(dir)
}

// NewRegistryAt creates a Registry backed by the given directory.
// This is useful for testing with a temporary directory.
func NewRegistryAt(dir string) (*Registry, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("providers: create dir: %w", err)
	}

	r := &Registry{
		dir:     dir,
		configs: make(map[string]*ProviderConfig),
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("providers: read dir: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			return nil, fmt.Errorf("providers: read %s: %w", entry.Name(), err)
		}
		var cfg ProviderConfig
		if err := json.Unmarshal(data, &cfg); err != nil {
			return nil, fmt.Errorf("providers: parse %s: %w", entry.Name(), err)
		}
		r.configs[cfg.ID] = &cfg
	}

	return r, nil
}

// List returns all loaded provider configurations.
func (r *Registry) List() []*ProviderConfig {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]*ProviderConfig, 0, len(r.configs))
	for _, cfg := range r.configs {
		out = append(out, cfg)
	}
	return out
}

// Get returns the provider configuration with the given ID, or nil.
func (r *Registry) Get(id string) *ProviderConfig {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.configs[id]
}

// Save writes a provider configuration to disk and updates the in-memory map.
func (r *Registry) Save(config *ProviderConfig) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.saveLocked(config)
}

func (r *Registry) saveLocked(config *ProviderConfig) error {
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("providers: marshal %s: %w", config.ID, err)
	}
	path := filepath.Join(r.dir, config.ID+".json")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("providers: write %s: %w", config.ID, err)
	}
	r.configs[config.ID] = config
	return nil
}

// Delete removes a provider configuration from disk and memory.
// Returns an error if the provider does not exist.
func (r *Registry) Delete(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.configs[id]; !ok {
		return fmt.Errorf("providers: %s not found", id)
	}

	path := filepath.Join(r.dir, id+".json")
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("providers: delete %s: %w", id, err)
	}
	delete(r.configs, id)
	return nil
}

// ListByCategory returns all provider configurations with the given category.
func (r *Registry) ListByCategory(category string) []*ProviderConfig {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var out []*ProviderConfig
	for _, cfg := range r.configs {
		if cfg.Category == category {
			out = append(out, cfg)
		}
	}
	return out
}

// MarkSuccess records a successful request for the given provider.
func (r *Registry) MarkSuccess(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	cfg, ok := r.configs[id]
	if !ok {
		return
	}
	cfg.LastSuccess = time.Now()
	cfg.ErrorCount = 0
	_ = r.saveLocked(cfg)
}

// MarkError records a failed request for the given provider.
func (r *Registry) MarkError(id string, errMsg string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	cfg, ok := r.configs[id]
	if !ok {
		return
	}
	cfg.ErrorCount++
	cfg.LastError = errMsg
	_ = r.saveLocked(cfg)
}

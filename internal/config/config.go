// Package config handles the project's configuration in mobai.json, the file
// the MobAI app keeps at the project root. ios-builder reads its own keys
// from it (ios, profiles, defaultProfile, github, ...) and leaves the others
// (project, build, anything else) to MobAI.
package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"
)

// ConfigFileName is the configuration file name.
const ConfigFileName = "mobai.json"

// ErrConfigNotFound indicates mobai.json was not found.
var ErrConfigNotFound = errors.New("mobai.json not found")

// Manager handles configuration loading and persistence
type Manager struct {
	path string
}

// NewManager creates a new configuration manager for ./mobai.json.
func NewManager() *Manager {
	return &Manager{path: ConfigFileName}
}

// Load reads the configuration from the configured path
func (m *Manager) Load() (*Config, error) {
	data, err := os.ReadFile(m.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrConfigNotFound
		}
		return nil, err
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// Save writes the configuration into mobai.json, keeping every key this
// package does not know and adding no empty sections.
func (m *Manager) Save(cfg *Config) error {
	existing, _ := os.ReadFile(m.path)
	data, err := MergeInto(existing, cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(m.path, data, 0644)
}

// MergeInto writes cfg into existing, a JSON object (empty or nil for a new
// file), and returns the indented result. cfg is the whole of ios-builder's
// part, so start from what the file holds (Load, or unmarshal it): each key
// Config defines is set from cfg, and removed when cfg leaves it empty ({},
// "", [], false, null), so no empty section is written and a cleared one goes.
// Every other key is kept. "name" is MobAI's too: an empty cfg.Name keeps it.
func MergeInto(existing []byte, cfg *Config) ([]byte, error) {
	fields := map[string]json.RawMessage{}
	if len(bytes.TrimSpace(existing)) > 0 {
		if err := json.Unmarshal(existing, &fields); err != nil || fields == nil {
			return nil, fmt.Errorf("mobai.json is not a JSON object: %v", err)
		}
	}
	data, err := json.Marshal(cfg)
	if err != nil {
		return nil, err
	}
	var own map[string]json.RawMessage
	if err := json.Unmarshal(data, &own); err != nil {
		return nil, err
	}
	for _, k := range ownKeys() {
		if v, ok := own[k]; ok && !emptyJSON(v) {
			fields[k] = v
		} else if k != "name" {
			delete(fields, k)
		}
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(fields); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// ownKeys are the top-level mobai.json keys Config defines.
func ownKeys() []string {
	t := reflect.TypeOf(Config{})
	keys := make([]string, 0, t.NumField())
	for i := 0; i < t.NumField(); i++ {
		if name, _, _ := strings.Cut(t.Field(i).Tag.Get("json"), ","); name != "" && name != "-" {
			keys = append(keys, name)
		}
	}
	return keys
}

// emptyJSON reports a value that says nothing: null, "", false, {}, [], or
// an object of only empty values.
func emptyJSON(v json.RawMessage) bool {
	switch s := string(bytes.TrimSpace(v)); s {
	case "null", `""`, "{}", "[]", "false":
		return true
	}
	var obj map[string]json.RawMessage
	if json.Unmarshal(v, &obj) != nil || obj == nil {
		return false
	}
	for _, x := range obj {
		if !emptyJSON(x) {
			return false
		}
	}
	return true
}

package config

import (
	"encoding/json"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name     string
		config   Config
		wantErr  bool
		errField string
	}{
		{
			name: "valid config",
			config: Config{
				Name:     "MyApp",
				Platform: "ios",
				GitHub:   GitHubConfig{Owner: "user", Repo: "builder-myapp"},
			},
			wantErr: false,
		},
		{
			name: "no name is fine",
			config: Config{
				Platform: "ios",
				GitHub:   GitHubConfig{Owner: "user", Repo: "builder-myapp"},
			},
			wantErr: false,
		},
		{
			name: "missing github owner",
			config: Config{
				Name:     "MyApp",
				Platform: "ios",
				GitHub:   GitHubConfig{Repo: "builder-myapp"},
			},
			wantErr:  true,
			errField: "github.owner",
		},
		{
			name: "missing github repo",
			config: Config{
				Name:     "MyApp",
				Platform: "ios",
				GitHub:   GitHubConfig{Owner: "user"},
			},
			wantErr:  true,
			errField: "github.repo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && err != nil {
				valErr, ok := err.(*ValidationError)
				if !ok {
					t.Errorf("Validate() error type = %T, want *ValidationError", err)
					return
				}
				if valErr.Field != tt.errField {
					t.Errorf("Validate() error field = %q, want %q", valErr.Field, tt.errField)
				}
			}
		})
	}
}

func TestManager_SaveAndLoad(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "mobai.json")

	mgr := &Manager{path: configPath}

	// Create config
	cfg := &Config{
		Name:     "TestApp",
		Platform: "ios",
		GitHub: GitHubConfig{
			Owner: "testuser",
			Repo:  "builder-testapp",
		},
	}

	// Save
	if err := mgr.Save(cfg); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Error("Config file not created after Save()")
	}

	// Load
	loaded, err := mgr.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Verify fields
	if loaded.Name != cfg.Name {
		t.Errorf("Name = %q, want %q", loaded.Name, cfg.Name)
	}
	if loaded.Platform != cfg.Platform {
		t.Errorf("Platform = %q, want %q", loaded.Platform, cfg.Platform)
	}
	if loaded.GitHub.Owner != cfg.GitHub.Owner {
		t.Errorf("GitHub.Owner = %q, want %q", loaded.GitHub.Owner, cfg.GitHub.Owner)
	}
	if loaded.GitHub.Repo != cfg.GitHub.Repo {
		t.Errorf("GitHub.Repo = %q, want %q", loaded.GitHub.Repo, cfg.GitHub.Repo)
	}
}

func TestManager_Load_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "nonexistent.json")

	mgr := &Manager{path: configPath}

	_, err := mgr.Load()
	if err != ErrConfigNotFound {
		t.Errorf("Load() error = %v, want ErrConfigNotFound", err)
	}
}

// Saving into MobAI's mobai.json keeps its keys, adds no empty sections and
// removes a section ios-builder cleared.
func TestMergeIntoKeepsMobAIKeys(t *testing.T) {
	existing := []byte(`{"project": "0b7c-uuid", "name": "Shelf", "build": {"ios": {"debug": "make <ios>"}}, "signing": {"dir": "~/old"}, "kmp": {}}`)
	var cfg Config
	if err := json.Unmarshal(existing, &cfg); err != nil {
		t.Fatal(err)
	}
	cfg.Name = ""
	cfg.Signing = nil
	cfg.GitHub = GitHubConfig{Owner: "o", Repo: "r"}
	cfg.Profiles = map[string]Profile{"store": {Distribution: "store"}}
	out, err := MergeInto(existing, &cfg)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]json.RawMessage
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatal(err)
	}
	keys := slices.Sorted(maps.Keys(got))
	if want := []string{"build", "github", "name", "profiles", "project"}; !slices.Equal(keys, want) {
		t.Fatalf("keys = %v, want %v\n%s", keys, want, out)
	}
	if string(got["project"]) != `"0b7c-uuid"` || string(got["name"]) != `"Shelf"` || !strings.Contains(string(out), `"make <ios>"`) {
		t.Fatalf("MobAI keys changed:\n%s", out)
	}

	// A new file gets only what is set.
	out, err = MergeInto(nil, &Config{GitHub: GitHubConfig{Owner: "o", Repo: "r"}})
	if err != nil || string(out) != "{\n  \"github\": {\n    \"owner\": \"o\",\n    \"repo\": \"r\"\n  }\n}\n" {
		t.Fatalf("new file: %q, %v", out, err)
	}
	if _, err := MergeInto([]byte(`[1]`), &cfg); err == nil {
		t.Fatal("non-object mobai.json accepted")
	}
}

func TestNewManager(t *testing.T) {
	mgr := NewManager()
	if mgr.path != ConfigFileName {
		t.Errorf("NewManager().path = %q, want %q", mgr.path, ConfigFileName)
	}
}

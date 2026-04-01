package settings

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type ProviderPreset struct {
	ID               string   `json:"id"`
	Name             string   `json:"name"`
	Description      string   `json:"description"`
	SuggestedCommand string   `json:"suggestedCommand"`
	SuggestedArgs    []string `json:"suggestedArgs"`
}

type StudioSettings struct {
	PreferredPreset     string            `json:"preferredPreset"`
	AgentCommand        string            `json:"agentCommand"`
	AgentArgs           []string          `json:"agentArgs"`
	AgentEnv            map[string]string `json:"agentEnv"`
	PreferBundledASC    bool              `json:"preferBundledASC"`
	SystemASCPath       string            `json:"systemASCPath"`
	WorkspaceRoot       string            `json:"workspaceRoot"`
	Theme               string            `json:"theme"`
	WindowMaterial      string            `json:"windowMaterial"`
	ShowCommandPreviews bool              `json:"showCommandPreviews"`
}

type AgentLaunch struct {
	Command string
	Args    []string
	Env     []string
	Dir     string
}

type Store struct {
	path     string
	readFile func(string) ([]byte, error)
	write    func(string, []byte, os.FileMode) error
	mkdirAll func(string, os.FileMode) error
}

func DefaultRoot() (string, error) {
	root, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve user config dir: %w", err)
	}
	return filepath.Join(root, "asc-studio"), nil
}

func NewStore(root string) *Store {
	return &Store{
		path:     filepath.Join(root, "settings.json"),
		readFile: os.ReadFile,
		write:    os.WriteFile,
		mkdirAll: os.MkdirAll,
	}
}

func (s *Store) Load() (StudioSettings, error) {
	defaults := DefaultSettings()
	data, err := s.readFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return defaults, nil
		}
		return StudioSettings{}, fmt.Errorf("read settings: %w", err)
	}

	var cfg StudioSettings
	if err := json.Unmarshal(data, &cfg); err != nil {
		return StudioSettings{}, fmt.Errorf("decode settings: %w", err)
	}
	cfg.Normalize()
	return cfg.withDefaults(defaults), nil
}

func (s *Store) Save(cfg StudioSettings) error {
	cfg.Normalize()
	if err := s.mkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return fmt.Errorf("create settings directory: %w", err)
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("encode settings: %w", err)
	}
	data = append(data, '\n')
	if err := s.write(s.path, data, 0o600); err != nil {
		return fmt.Errorf("write settings: %w", err)
	}
	return nil
}

func (s *Store) Path() string {
	return s.path
}

func DefaultSettings() StudioSettings {
	return StudioSettings{
		PreferredPreset:     "codex",
		AgentEnv:            map[string]string{},
		PreferBundledASC:    true,
		Theme:               "system",
		WindowMaterial:      "translucent",
		ShowCommandPreviews: true,
	}
}

func DefaultPresets() []ProviderPreset {
	return []ProviderPreset{
		{
			ID:               "codex",
			Name:             "Codex",
			Description:      "Use a local Codex ACP adapter or wrapper command.",
			SuggestedCommand: "codex",
			SuggestedArgs:    []string{"agent", "acp"},
		},
		{
			ID:               "claude",
			Name:             "Claude",
			Description:      "Use a Claude ACP adapter command configured on this machine.",
			SuggestedCommand: "claude-acp",
			SuggestedArgs:    []string{},
		},
		{
			ID:               "custom",
			Name:             "Custom ACP Agent",
			Description:      "Point Studio at any local ACP-capable agent command.",
			SuggestedCommand: "",
			SuggestedArgs:    []string{},
		},
	}
}

func (s *StudioSettings) Normalize() {
	if strings.TrimSpace(s.PreferredPreset) == "" {
		s.PreferredPreset = DefaultSettings().PreferredPreset
	}
	if s.AgentEnv == nil {
		s.AgentEnv = map[string]string{}
	}
	if strings.TrimSpace(s.Theme) == "" {
		s.Theme = DefaultSettings().Theme
	}
	if strings.TrimSpace(s.WindowMaterial) == "" {
		s.WindowMaterial = DefaultSettings().WindowMaterial
	}
}

func (s StudioSettings) ResolveAgentLaunch() (AgentLaunch, error) {
	command := strings.TrimSpace(s.AgentCommand)
	args := append([]string(nil), s.AgentArgs...)
	if command == "" {
		for _, preset := range DefaultPresets() {
			if preset.ID == s.PreferredPreset {
				command = strings.TrimSpace(preset.SuggestedCommand)
				if len(args) == 0 {
					args = append(args, preset.SuggestedArgs...)
				}
				break
			}
		}
	}
	if command == "" {
		return AgentLaunch{}, errors.New("agent command is not configured")
	}

	env := make([]string, 0, len(s.AgentEnv))
	for key, value := range s.AgentEnv {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		env = append(env, key+"="+value)
	}

	return AgentLaunch{
		Command: command,
		Args:    args,
		Env:     env,
		Dir:     strings.TrimSpace(s.WorkspaceRoot),
	}, nil
}

func (s StudioSettings) withDefaults(defaults StudioSettings) StudioSettings {
	if strings.TrimSpace(s.PreferredPreset) == "" {
		s.PreferredPreset = defaults.PreferredPreset
	}
	if s.AgentEnv == nil {
		s.AgentEnv = defaults.AgentEnv
	}
	if strings.TrimSpace(s.Theme) == "" {
		s.Theme = defaults.Theme
	}
	if strings.TrimSpace(s.WindowMaterial) == "" {
		s.WindowMaterial = defaults.WindowMaterial
	}
	return s
}

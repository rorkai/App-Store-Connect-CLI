package environment

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/auth"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/config"
)

type Snapshot struct {
	ConfigPath        string `json:"configPath"`
	ConfigPresent     bool   `json:"configPresent"`
	DefaultAppID      string `json:"defaultAppId,omitempty"`
	KeychainAvailable bool   `json:"keychainAvailable"`
	KeychainBypassed  bool   `json:"keychainBypassed"`
	KeychainWarning   string `json:"keychainWarning,omitempty"`
	WorkflowPath      string `json:"workflowPath"`
}

type Service struct{}

var (
	configPathFunc        = config.Path
	configLoadFunc        = config.Load
	keychainAvailableFunc = auth.KeychainAvailable
	getwdFunc             = os.Getwd
)

func NewService() *Service {
	return &Service{}
}

func (s *Service) Snapshot() (Snapshot, error) {
	configPath, err := configPathFunc()
	if err != nil {
		return Snapshot{}, err
	}

	cfg, cfgErr := configLoadFunc()
	keychainAvailable, keychainErr := keychainAvailableFunc()
	if cfgErr != nil && !errors.Is(cfgErr, config.ErrNotFound) {
		return Snapshot{}, cfgErr
	}

	workflowPath := ""
	if cwd, err := getwdFunc(); err == nil {
		workflowPath = filepath.Join(cwd, ".asc", "workflow.json")
	}

	snapshot := Snapshot{
		ConfigPath:        configPath,
		ConfigPresent:     cfgErr == nil,
		KeychainAvailable: keychainAvailable,
		KeychainBypassed:  auth.ShouldBypassKeychain(),
		KeychainWarning:   keychainWarningMessage(keychainErr),
		WorkflowPath:      workflowPath,
	}
	if cfg != nil {
		snapshot.DefaultAppID = cfg.AppID
	}
	return snapshot, nil
}

func keychainWarningMessage(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

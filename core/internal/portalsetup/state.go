package portalsetup

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"github.com/duxlabs-dev/dux-files/core/internal/atomicfile"
)

type installState struct {
	ConfigPath     string `json:"configPath"`
	ConfigExisted  bool   `json:"configExisted"`
	OriginalLine   string `json:"originalLine"`
	HadLine        bool   `json:"hadLine"`
	WrittenLine    string `json:"writtenLine"`
	CreatedPortal  bool   `json:"createdPortal"`
	CreatedService bool   `json:"createdService"`
}

func stateDir(env Env) string {
	return filepath.Join(env.StateHome, "dfiles")
}

func statePath(env Env) string {
	return filepath.Join(stateDir(env), stateFile)
}

func readState(env Env) (installState, bool) {
	data, err := os.ReadFile(statePath(env))
	if err != nil {
		return installState{}, false
	}
	var state installState
	if err := json.Unmarshal(data, &state); err != nil {
		return installState{}, false
	}
	return state, true
}

func writeState(env Env, state installState) error {
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return atomicfile.WriteFileMkdir(statePath(env), append(data, '\n'), 0o644, 0o755)
}

func removeState(env Env) error {
	if err := os.Remove(statePath(env)); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	os.Remove(stateDir(env))
	return nil
}

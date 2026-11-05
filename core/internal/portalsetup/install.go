package portalsetup

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/duxlabs-dev/dux-files/core/internal/atomicfile"
)

func Install(env Env) (Status, error) {
	if env.PortalDir != "" {
		return Status{}, errors.New("the portal directory is set by the system; add dfiles to its portal list instead")
	}
	if !validExec(env.Exec) {
		return Status{}, errors.New("the dfiles binary path must be absolute without spaces, quotes or backslashes")
	}
	state, hasState := readState(env)
	createdPortal, err := ensurePortalFile(env)
	if err != nil {
		return Status{}, err
	}
	createdService, err := ensureServiceFile(env)
	if err != nil {
		return Status{}, err
	}
	path, existed := targetConfig(env)
	if err := refuseSymlink(path); err != nil {
		return Status{}, err
	}
	original, mode, err := readConfig(path, existed)
	if err != nil {
		return Status{}, err
	}
	value := PortalName + ";" + strings.Join(previousBackends(env), ";") + ";"
	updated := setPreferred(original, value)
	if err := atomicfile.WriteFileMkdir(path, []byte(updated), mode, 0o755); err != nil {
		return Status{}, err
	}
	written, _ := preferredLine(updated)
	switch {
	case hasState:
		state.WrittenLine = written
		state.CreatedPortal = state.CreatedPortal || createdPortal
		state.CreatedService = state.CreatedService || createdService
	default:
		originalLine, hadLine := preferredLine(original)
		state = installState{
			ConfigPath:     path,
			ConfigExisted:  existed,
			OriginalLine:   originalLine,
			HadLine:        hadLine,
			WrittenLine:    written,
			CreatedPortal:  createdPortal,
			CreatedService: createdService,
		}
	}
	if err := writeState(env, state); err != nil {
		return Status{}, err
	}
	return Inspect(env), nil
}

func Uninstall(env Env) (Status, error) {
	state, hasState := readState(env)
	if !hasState {
		if err := stripUnmanaged(env); err != nil {
			return Status{}, err
		}
		return Inspect(env), nil
	}
	if err := refuseSymlink(state.ConfigPath); err != nil {
		return Status{}, err
	}
	kept, err := restoreConfig(state)
	if err != nil {
		return Status{}, err
	}
	if state.CreatedPortal {
		if err := removeFile(filepath.Join(userPortalDir(env), PortalFile)); err != nil {
			return Status{}, err
		}
	}
	if state.CreatedService {
		if err := removeFile(filepath.Join(userServiceDir(env), ServiceFile)); err != nil {
			return Status{}, err
		}
	}
	if err := removeState(env); err != nil {
		return Status{}, err
	}
	status := Inspect(env)
	status.Kept = kept
	return status, nil
}

func validExec(exec string) bool {
	if !filepath.IsAbs(exec) {
		return false
	}
	return !strings.ContainsAny(exec, " \t\n\r\"'\\")
}

func ensurePortalFile(env Env) (bool, error) {
	if _, found := installedPortals(env)[PortalName]; found {
		return false, nil
	}
	content := "[portal]\nDBusName=" + BusName + "\nInterfaces=" + Interface + ";\n"
	return true, writeNew(filepath.Join(userPortalDir(env), PortalFile), content)
}

func ensureServiceFile(env Env) (bool, error) {
	content := "[D-BUS Service]\nName=" + BusName + "\nExec=" + serviceExec(env) + "\n"
	service := serviceStatus(env)
	switch {
	case !service.Found:
		return true, writeNew(service.Path, content)
	case !service.User:
		return false, nil
	}
	data, err := os.ReadFile(service.Path)
	if err != nil {
		return false, err
	}
	current, _ := lookupKey(string(data), "D-BUS Service", "Exec")
	if current == serviceExec(env) {
		return false, nil
	}
	return false, atomicfile.WriteFile(service.Path, []byte(content), 0o644)
}

func serviceExec(env Env) string {
	return env.Exec + " run --hidden --portal-activated"
}

func refuseSymlink(path string) error {
	info, err := os.Lstat(path)
	if err != nil || info.Mode()&os.ModeSymlink == 0 {
		return nil
	}
	return errors.New(path + " is a symlink; set the file chooser preference where that file is generated")
}

func previousBackends(env Env) []string {
	var previous []string
	for _, file := range configChain(env) {
		value, ok := lookupKey(file.content, preferredGroup, Interface)
		if !ok {
			value, ok = lookupKey(file.content, preferredGroup, "default")
		}
		if !ok {
			continue
		}
		for _, name := range backendList(value) {
			if name == PortalName {
				continue
			}
			previous = append(previous, name)
		}
		break
	}
	if len(previous) == 0 {
		return []string{"*"}
	}
	return previous
}

func readConfig(path string, exists bool) (string, os.FileMode, error) {
	if !exists {
		return "", 0o644, nil
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", 0, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", 0, err
	}
	return string(data), info.Mode().Perm(), nil
}

func stripUnmanaged(env Env) error {
	path, exists := targetConfig(env)
	if !exists {
		return nil
	}
	content, mode, err := readConfig(path, true)
	if err != nil {
		return err
	}
	line, found := preferredLine(content)
	if !found {
		return nil
	}
	_, value, _ := strings.Cut(line, "=")
	list := backendList(value)
	if len(list) == 0 || list[0] != PortalName {
		return nil
	}
	rest := list[1:]
	updated := removePreferred(content)
	if len(rest) > 0 {
		updated = setPreferred(content, strings.Join(rest, ";")+";")
	}
	return atomicfile.WriteFile(path, []byte(updated), mode)
}

func restoreConfig(state installState) (bool, error) {
	content, mode, err := readConfig(state.ConfigPath, isFile(state.ConfigPath))
	if err != nil {
		return false, err
	}
	current, found := preferredLine(content)
	switch {
	case !found:
		return false, nil
	case current != state.WrittenLine:
		return true, nil
	}
	updated := removePreferred(content)
	if state.HadLine {
		updated = setPreferredLine(content, state.OriginalLine)
	}
	if !state.ConfigExisted && onlyPreferredHeader(updated) {
		return false, removeFile(state.ConfigPath)
	}
	return false, atomicfile.WriteFile(state.ConfigPath, []byte(updated), mode)
}

func onlyPreferredHeader(content string) bool {
	for _, line := range splitLines(content) {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || trimmed == "["+preferredGroup+"]" {
			continue
		}
		return false
	}
	return true
}

func writeNew(path, content string) error {
	return atomicfile.WriteFileMkdir(path, []byte(content), 0o644, 0o755)
}

func removeFile(path string) error {
	err := os.Remove(path)
	if err == nil || errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

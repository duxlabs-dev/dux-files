package sandbox

import (
	"strings"

	"github.com/duxlabs-dev/dux-files/core/internal/ini"
)

const flatpakInfoPath = "/.flatpak-info"

// /.flatpak-info already has overrides applied.
type flatpakInfo struct {
	appID       string
	filesystems map[string]string
	sessionBus  map[string]string
	systemBus   map[string]string
}

func parseFlatpakInfo(data []byte) flatpakInfo {
	info := flatpakInfo{
		filesystems: map[string]string{},
		sessionBus:  map[string]string{},
		systemBus:   map[string]string{},
	}
	for line := range ini.Lines(data) {
		if line.Header {
			continue
		}
		switch line.Group {
		case "Application":
			if line.Key == "name" {
				info.appID = line.Value
			}
		case "Context":
			if line.Key == "filesystems" {
				info.filesystems = parseFilesystems(line.Value)
			}
		case "Session Bus Policy":
			info.sessionBus[line.Key] = line.Value
		case "System Bus Policy":
			info.systemBus[line.Key] = line.Value
		}
	}
	return info
}

func parseFilesystems(value string) map[string]string {
	out := map[string]string{}
	for _, entry := range strings.Split(value, ";") {
		path, mode := splitFilesystemMode(strings.TrimSpace(entry))
		switch {
		case path == "":
			continue
		case mode == "reset":
			clear(out)
		case strings.HasPrefix(path, "!"):
			delete(out, path[1:])
		default:
			out[path] = mode
		}
	}
	return out
}

func splitFilesystemMode(entry string) (string, string) {
	i := strings.LastIndex(entry, ":")
	if i < 0 {
		return entry, "rw"
	}
	switch mode := entry[i+1:]; mode {
	case "ro", "rw", "create", "reset":
		return entry[:i], mode
	}
	return entry, "rw"
}

func (fi flatpakInfo) hasFilesystem(path string) bool {
	_, ok := fi.filesystems[path]
	return ok
}

// Matched like xdg-dbus-proxy: exact name or a `prefix.*` wildcard at talk or own level.
func canTalk(policy map[string]string, name string) bool {
	for pattern, level := range policy {
		if level != "talk" && level != "own" {
			continue
		}
		if pattern == name {
			return true
		}
		prefix, ok := strings.CutSuffix(pattern, ".*")
		if !ok {
			continue
		}
		if strings.HasPrefix(name, prefix+".") {
			return true
		}
	}
	return false
}

package xdg

import (
	"os"
	"strings"

	"github.com/duxlabs-dev/dux-go/paths"
)

const (
	DefaultDataDirs   = "/usr/local/share:/usr/share"
	DefaultConfigDirs = "/etc/xdg"
)

func DataHome() string { return paths.XDGDataHome() }

func ConfigHome() string { return paths.XDGConfigHome() }

func StateHome() string { return paths.XDGStateHome() }

func DataDirs() []string {
	return SplitPathList(EnvOr(os.Getenv, "XDG_DATA_DIRS", DefaultDataDirs))
}

func ConfigDirs() []string {
	return SplitPathList(EnvOr(os.Getenv, "XDG_CONFIG_DIRS", DefaultConfigDirs))
}

func EnvOr(env func(string) string, key, fallback string) string {
	if v := env(key); v != "" {
		return v
	}
	return fallback
}

func SplitPathList(raw string) []string {
	var out []string
	for _, dir := range strings.Split(raw, ":") {
		if dir = strings.TrimSpace(dir); dir != "" {
			out = append(out, dir)
		}
	}
	return out
}

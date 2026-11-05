package sandbox

import (
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/duxlabs-dev/dux-files/core/internal/xdg"
)

type Dirs struct {
	ConfigHome string   `json:"configHome"`
	DataHome   string   `json:"dataHome"`
	CacheHome  string   `json:"cacheHome"`
	StateHome  string   `json:"stateHome"`
	ConfigDirs []string `json:"configDirs"`
	DataDirs   []string `json:"dataDirs"`
}

const hostOSRoot = "/run/host"

const (
	sandboxConfigDirs = "/app/etc/xdg:/etc/xdg"
	sandboxDataDirs   = "/app/share:/usr/share"
)

var hostConfigDirs = []string{hostOSRoot + "/etc/xdg"}

// Other Flatpak apps export desktop entries under the first two; /usr inside the sandbox is the runtime, the host's sits under /run/host.
func hostDataDirs(dataHome string) []string {
	return []string{
		filepath.Join(dataHome, "flatpak", "exports", "share"),
		"/var/lib/flatpak/exports/share",
		hostOSRoot + "/usr/local/share",
		hostOSRoot + "/usr/share",
	}
}

func nativeDirs(env func(string) string) Dirs {
	home := env("HOME")
	return Dirs{
		ConfigHome: xdg.EnvOr(env, "XDG_CONFIG_HOME", filepath.Join(home, ".config")),
		DataHome:   xdg.EnvOr(env, "XDG_DATA_HOME", filepath.Join(home, ".local", "share")),
		CacheHome:  xdg.EnvOr(env, "XDG_CACHE_HOME", filepath.Join(home, ".cache")),
		StateHome:  xdg.EnvOr(env, "XDG_STATE_HOME", filepath.Join(home, ".local", "state")),
		ConfigDirs: xdg.SplitPathList(xdg.EnvOr(env, "XDG_CONFIG_DIRS", "/etc/xdg")),
		DataDirs:   xdg.SplitPathList(xdg.EnvOr(env, "XDG_DATA_DIRS", "/usr/local/share:/usr/share")),
	}
}

// Flatpak exports HOST_XDG_*_HOME only when the host overrides it; the runtime's own XDG_*_DIRS stay first so the sandbox still finds its resources.
func hostDirs(env func(string) string, hostOS bool) Dirs {
	home := env("HOME")
	dirs := Dirs{
		ConfigHome: xdg.EnvOr(env, "HOST_XDG_CONFIG_HOME", filepath.Join(home, ".config")),
		DataHome:   xdg.EnvOr(env, "HOST_XDG_DATA_HOME", filepath.Join(home, ".local", "share")),
		CacheHome:  xdg.EnvOr(env, "HOST_XDG_CACHE_HOME", filepath.Join(home, ".cache")),
		StateHome:  xdg.EnvOr(env, "HOST_XDG_STATE_HOME", filepath.Join(home, ".local", "state")),
		ConfigDirs: xdg.SplitPathList(xdg.EnvOr(env, "XDG_CONFIG_DIRS", sandboxConfigDirs)),
		DataDirs:   xdg.SplitPathList(xdg.EnvOr(env, "XDG_DATA_DIRS", sandboxDataDirs)),
	}
	if !hostOS {
		return dirs
	}
	dirs.ConfigDirs = appendMissing(dirs.ConfigDirs, hostConfigDirs...)
	dirs.DataDirs = appendMissing(dirs.DataDirs, hostDataDirs(dirs.DataHome)...)
	return dirs
}

// Also reaches Qt in the UI child, which inherits the environment.
func ExportHostDirs(caps Capabilities) {
	if !caps.Sandboxed || !caps.UserFilesystem {
		return
	}
	exports := [][2]string{
		{"XDG_CONFIG_HOME", caps.Dirs.ConfigHome},
		{"XDG_DATA_HOME", caps.Dirs.DataHome},
		{"XDG_CACHE_HOME", caps.Dirs.CacheHome},
		{"XDG_STATE_HOME", caps.Dirs.StateHome},
		{"XDG_CONFIG_DIRS", strings.Join(caps.Dirs.ConfigDirs, ":")},
		{"XDG_DATA_DIRS", strings.Join(caps.Dirs.DataDirs, ":")},
	}
	for _, kv := range exports {
		_ = os.Setenv(kv[0], kv[1])
	}
}

func appendMissing(list []string, extra ...string) []string {
	for _, dir := range extra {
		if !slices.Contains(list, dir) {
			list = append(list, dir)
		}
	}
	return list
}

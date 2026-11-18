package portalsetup

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/duxlabs-dev/dux-files/core/internal/xdg"
)

const (
	BusName     = "org.freedesktop.impl.portal.desktop.dfiles"
	PortalName  = "dfiles"
	Interface   = "org.freedesktop.impl.portal.FileChooser"
	PortalFile  = "dfiles.portal"
	ServiceFile = BusName + ".service"
	stateFile   = "portal-install.json"
)

// USER_HZ is fixed at 100 on linux regardless of the kernel HZ
const clockTicks = 100

var desktopNamePattern = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

type Env struct {
	DataHome   string
	ConfigHome string
	StateHome  string
	ConfigDirs []string
	DataDirs   []string
	SysConfDir string
	DataDir    string
	Desktops   []string
	PortalDir  string
	Exec       string
}

func FromProcess() Env {
	exec, _ := os.Executable()
	if resolved, err := filepath.EvalSymlinks(exec); err == nil {
		exec = resolved
	}
	return Env{
		DataHome:   xdg.DataHome(),
		ConfigHome: xdg.ConfigHome(),
		StateHome:  xdg.StateHome(),
		ConfigDirs: xdg.ConfigDirs(),
		DataDirs:   xdg.DataDirs(),
		SysConfDir: "/etc",
		DataDir:    "/usr/share",
		Desktops:   xdg.SplitPathList(os.Getenv("XDG_CURRENT_DESKTOP")),
		PortalDir:  os.Getenv("XDG_DESKTOP_PORTAL_DIR"),
		Exec:       exec,
	}
}

func (e Env) WithEnviron(environ map[string]string) Env {
	if v, ok := environ["XDG_CURRENT_DESKTOP"]; ok {
		e.Desktops = xdg.SplitPathList(v)
	}
	if v, ok := environ["XDG_DESKTOP_PORTAL_DIR"]; ok {
		e.PortalDir = v
	}
	if v, ok := environ["XDG_DATA_DIRS"]; ok {
		e.DataDirs = xdg.SplitPathList(valueOr(v, xdg.DefaultDataDirs))
	}
	if v, ok := environ["XDG_CONFIG_DIRS"]; ok {
		e.ConfigDirs = xdg.SplitPathList(valueOr(v, xdg.DefaultConfigDirs))
	}
	if v := environ["XDG_DATA_HOME"]; v != "" {
		e.DataHome = v
	}
	if v := environ["XDG_CONFIG_HOME"]; v != "" {
		e.ConfigHome = v
	}
	return e
}

func ParseEnviron(data []byte) map[string]string {
	out := map[string]string{}
	for _, entry := range bytes.Split(data, []byte{0}) {
		if len(entry) == 0 {
			continue
		}
		key, value, _ := strings.Cut(string(entry), "=")
		if key == "" {
			continue
		}
		out[key] = value
	}
	return out
}

func ProcessStart(pid int) (time.Time, bool) {
	stat, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "stat"))
	if err != nil {
		return time.Time{}, false
	}
	btime, ok := bootTime()
	if !ok {
		return time.Time{}, false
	}
	return processStart(string(stat), btime, clockTicks)
}

func processStart(stat string, btime int64, hz int64) (time.Time, bool) {
	_, after, found := strings.Cut(stat, ") ")
	if !found || hz <= 0 {
		return time.Time{}, false
	}
	fields := strings.Fields(after)
	if len(fields) < 20 {
		return time.Time{}, false
	}
	ticks, err := strconv.ParseInt(fields[19], 10, 64)
	if err != nil {
		return time.Time{}, false
	}
	nanos := (ticks % hz) * (int64(time.Second) / hz)
	return time.Unix(btime+ticks/hz, nanos), true
}

func bootTime() (int64, bool) {
	data, err := os.ReadFile("/proc/stat")
	if err != nil {
		return 0, false
	}
	for _, line := range strings.Split(string(data), "\n") {
		if !strings.HasPrefix(line, "btime ") {
			continue
		}
		value, err := strconv.ParseInt(strings.TrimSpace(strings.TrimPrefix(line, "btime ")), 10, 64)
		return value, err == nil
	}
	return 0, false
}

func validDesktops(desktops []string) []string {
	var out []string
	for _, d := range desktops {
		if !desktopNamePattern.MatchString(d) {
			continue
		}
		out = append(out, strings.ToLower(d))
	}
	return out
}

func valueOr(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

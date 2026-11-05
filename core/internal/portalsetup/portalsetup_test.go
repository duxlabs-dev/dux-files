package portalsetup

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/duxlabs-dev/dux-files/core/internal/testfs"

	"github.com/duxlabs-dev/dux-files/core/internal/ini"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	gtkPortal    = "[portal]\nDBusName=org.freedesktop.impl.portal.desktop.gtk\nInterfaces=org.freedesktop.impl.portal.FileChooser;org.freedesktop.impl.portal.Print;\n"
	gnomePortal  = "[portal]\nDBusName=org.freedesktop.impl.portal.desktop.gnome\nInterfaces=org.freedesktop.impl.portal.Screenshot;\n"
	dfilesPortal = "[portal]\nDBusName=" + BusName + "\nInterfaces=" + Interface + ";\n"
)

func testEnv(t *testing.T) Env {
	t.Helper()
	root := t.TempDir()
	return Env{
		DataHome:   filepath.Join(root, "home", "share"),
		ConfigHome: filepath.Join(root, "home", "config"),
		StateHome:  filepath.Join(root, "home", "state"),
		ConfigDirs: []string{filepath.Join(root, "etc", "xdg")},
		DataDirs:   []string{filepath.Join(root, "usr", "local", "share"), filepath.Join(root, "usr", "share")},
		SysConfDir: filepath.Join(root, "etc"),
		DataDir:    filepath.Join(root, "usr", "share"),
		Desktops:   []string{"niri"},
		Exec:       "/tmp/x/dfiles",
	}
}

func sysPortal(env Env, name string) string {
	return filepath.Join(env.DataDir, "xdg-desktop-portal", "portals", name+".portal")
}

func sysConfig(env Env, name string) string {
	return filepath.Join(env.SysConfDir, "xdg-desktop-portal", name)
}

func userConfig(env Env, name string) string {
	return filepath.Join(env.ConfigHome, "xdg-desktop-portal", name)
}

func withoutPreferredLine(content string) string {
	var kept []string
	for _, line := range strings.Split(content, "\n") {
		if ini.Key(line) == Interface {
			continue
		}
		kept = append(kept, line)
	}
	return strings.Join(kept, "\n")
}

func TestInspectEmptyTree(t *testing.T) {
	env := testEnv(t)
	status := Inspect(env)
	assert.False(t, status.Managed)
	assert.False(t, status.SystemManaged)
	assert.False(t, status.PortalFile.Found)
	assert.False(t, status.Service.Found)
	assert.Equal(t, userConfig(env, "niri-portals.conf"), status.Config.Path)
	assert.False(t, status.Config.Exists)
	assert.Nil(t, status.Config.Backends)
	assert.Equal(t, "", status.Config.Effective)
	assert.False(t, status.Active)
}

func TestEffectiveResolution(t *testing.T) {
	t.Run("system default list picks first installed", func(t *testing.T) {
		env := testEnv(t)
		testfs.Write(t, sysPortal(env, "gtk"), gtkPortal)
		testfs.Write(t, sysConfig(env, "niri-portals.conf"), "[preferred]\ndefault=gnome;gtk;\n")
		assert.Equal(t, "gtk", Inspect(env).Config.Effective)
	})
	t.Run("user interface key wins", func(t *testing.T) {
		env := testEnv(t)
		testfs.Write(t, sysPortal(env, "gtk"), gtkPortal)
		testfs.Write(t, sysPortal(env, "dfiles"), dfilesPortal)
		testfs.Write(t, sysConfig(env, "niri-portals.conf"), "[preferred]\ndefault=gtk;\n")
		testfs.Write(t, userConfig(env, "niri-portals.conf"), "[preferred]\n"+Interface+"=dfiles;gtk;\n")
		status := Inspect(env)
		assert.Equal(t, "dfiles", status.Config.Effective)
		assert.Equal(t, []string{"dfiles", "gtk"}, status.Config.Backends)
		assert.True(t, status.Active)
	})
	t.Run("none stops", func(t *testing.T) {
		env := testEnv(t)
		testfs.Write(t, sysPortal(env, "gtk"), gtkPortal)
		testfs.Write(t, userConfig(env, "portals.conf"), "[preferred]\n"+Interface+"=none;\n")
		testfs.Write(t, sysConfig(env, "portals.conf"), "[preferred]\ndefault=gtk;\n")
		assert.Equal(t, "", Inspect(env).Config.Effective)
	})
	t.Run("star picks first installed by name", func(t *testing.T) {
		env := testEnv(t)
		testfs.Write(t, sysPortal(env, "zed"), gtkPortal)
		testfs.Write(t, sysPortal(env, "gtk"), gtkPortal)
		testfs.Write(t, sysPortal(env, "aaa"), gnomePortal)
		testfs.Write(t, sysConfig(env, "portals.conf"), "[preferred]\ndefault=*;\n")
		assert.Equal(t, "gtk", Inspect(env).Config.Effective)
	})
	t.Run("uninstalled backend falls through to the next file", func(t *testing.T) {
		env := testEnv(t)
		testfs.Write(t, sysPortal(env, "gtk"), gtkPortal)
		testfs.Write(t, userConfig(env, "niri-portals.conf"), "[preferred]\n"+Interface+"=kde;\n")
		testfs.Write(t, sysConfig(env, "niri-portals.conf"), "[preferred]\ndefault=gtk;\n")
		assert.Equal(t, "gtk", Inspect(env).Config.Effective)
	})
	t.Run("backend without the interface is skipped", func(t *testing.T) {
		env := testEnv(t)
		testfs.Write(t, sysPortal(env, "gnome"), gnomePortal)
		testfs.Write(t, sysPortal(env, "gtk"), gtkPortal)
		testfs.Write(t, sysConfig(env, "portals.conf"), "[preferred]\n"+Interface+"=gnome;gtk;\n")
		assert.Equal(t, "gtk", Inspect(env).Config.Effective)
	})
	t.Run("desktop file beats portals.conf in the same dir", func(t *testing.T) {
		env := testEnv(t)
		testfs.Write(t, sysPortal(env, "gtk"), gtkPortal)
		testfs.Write(t, sysPortal(env, "dfiles"), dfilesPortal)
		testfs.Write(t, sysConfig(env, "niri-portals.conf"), "[preferred]\ndefault=dfiles;\n")
		testfs.Write(t, sysConfig(env, "portals.conf"), "[preferred]\ndefault=gtk;\n")
		assert.Equal(t, "dfiles", Inspect(env).Config.Effective)
	})
	t.Run("file without preferred group is skipped", func(t *testing.T) {
		env := testEnv(t)
		testfs.Write(t, sysPortal(env, "gtk"), gtkPortal)
		testfs.Write(t, sysPortal(env, "dfiles"), dfilesPortal)
		testfs.Write(t, sysConfig(env, "niri-portals.conf"), "[other]\ndefault=dfiles;\n")
		testfs.Write(t, sysConfig(env, "portals.conf"), "[preferred]\ndefault=gtk;\n")
		assert.Equal(t, "gtk", Inspect(env).Config.Effective)
	})
	t.Run("invalid desktop entries skipped and names lowercased", func(t *testing.T) {
		env := testEnv(t)
		env.Desktops = []string{"bad name", "a/b", "Niri", "GNOME"}
		testfs.Write(t, sysPortal(env, "gtk"), gtkPortal)
		testfs.Write(t, sysPortal(env, "dfiles"), dfilesPortal)
		testfs.Write(t, sysConfig(env, "niri-portals.conf"), "[preferred]\ndefault=dfiles;\n")
		testfs.Write(t, sysConfig(env, "gnome-portals.conf"), "[preferred]\ndefault=gtk;\n")
		assert.Equal(t, "dfiles", Inspect(env).Config.Effective)
		assert.Equal(t, userConfig(env, "niri-portals.conf"), Inspect(env).Config.Path)
	})
	t.Run("no valid desktop targets portals.conf", func(t *testing.T) {
		env := testEnv(t)
		env.Desktops = []string{"bad name"}
		assert.Equal(t, userConfig(env, "portals.conf"), Inspect(env).Config.Path)
	})
	t.Run("user portal dir wins over system for the same name", func(t *testing.T) {
		env := testEnv(t)
		testfs.Write(t, filepath.Join(userPortalDir(env), "gtk.portal"), gnomePortal)
		testfs.Write(t, sysPortal(env, "gtk"), gtkPortal)
		testfs.Write(t, sysConfig(env, "portals.conf"), "[preferred]\ndefault=gtk;\n")
		assert.Equal(t, "", Inspect(env).Config.Effective)
	})
	t.Run("portal dir is exclusive", func(t *testing.T) {
		env := testEnv(t)
		env.PortalDir = filepath.Join(env.SysConfDir, "portal-dir")
		testfs.Write(t, sysPortal(env, "gtk"), gtkPortal)
		testfs.Write(t, filepath.Join(env.PortalDir, "dfiles.portal"), dfilesPortal)
		testfs.Write(t, filepath.Join(env.PortalDir, "portals.conf"), "[preferred]\ndefault=gtk;dfiles;\n")
		testfs.Write(t, userConfig(env, "niri-portals.conf"), "[preferred]\ndefault=gtk;\n")
		status := Inspect(env)
		assert.True(t, status.SystemManaged)
		assert.Equal(t, "dfiles", status.Config.Effective)
		assert.True(t, status.PortalFile.Found)
		assert.False(t, status.PortalFile.User)
	})
}

func TestInstallIntoExistingUserFile(t *testing.T) {
	env := testEnv(t)
	testfs.Write(t, sysPortal(env, "gtk"), gtkPortal)
	testfs.Write(t, sysConfig(env, "niri-portals.conf"), "[preferred]\ndefault=gnome;gtk;\n")
	path := userConfig(env, "niri-portals.conf")
	original := "# my portals\n[preferred]\ndefault=gtk;\norg.freedesktop.impl.portal.Screenshot=gnome;\n\n[other]\nk=v\n"
	testfs.Write(t, path, original)
	require.NoError(t, os.Chmod(path, 0o600))

	status, err := Install(env)
	require.NoError(t, err)

	updated := testfs.Read(t, path)
	assert.Equal(t, withoutPreferredLine(original), withoutPreferredLine(updated))
	line, found := preferredLine(updated)
	require.True(t, found)
	assert.Equal(t, Interface+"=dfiles;gtk;", line)
	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())

	assert.True(t, status.Managed)
	assert.True(t, status.Active)
	assert.Equal(t, "dfiles", status.Config.Effective)
	assert.Equal(t, []string{"dfiles", "gtk"}, status.Config.Backends)
	assert.True(t, status.PortalFile.User)
	assert.True(t, status.Service.User)
	assert.Equal(t, dfilesPortal, testfs.Read(t, filepath.Join(userPortalDir(env), PortalFile)))
	assert.Equal(t, "[D-BUS Service]\nName="+BusName+"\nExec=/tmp/x/dfiles run --hidden --portal-activated\n", testfs.Read(t, filepath.Join(userServiceDir(env), ServiceFile)))

	state, ok := readState(env)
	require.True(t, ok)
	assert.Equal(t, installState{ConfigPath: path, ConfigExisted: true, WrittenLine: Interface + "=dfiles;gtk;", CreatedPortal: true, CreatedService: true}, state)

	status, err = Uninstall(env)
	require.NoError(t, err)
	assert.False(t, status.Kept)
	assert.False(t, status.Managed)
	assert.False(t, status.Active)
	assert.Equal(t, "gtk", status.Config.Effective)
	assert.Equal(t, original, testfs.Read(t, path))
	assert.NoFileExists(t, filepath.Join(userPortalDir(env), PortalFile))
	assert.NoFileExists(t, filepath.Join(userServiceDir(env), ServiceFile))
	assert.NoFileExists(t, statePath(env))
	assert.NoDirExists(t, stateDir(env))
}

func TestInstallRestoresExistingLine(t *testing.T) {
	env := testEnv(t)
	testfs.Write(t, sysPortal(env, "gtk"), gtkPortal)
	path := userConfig(env, "portals.conf")
	original := "[preferred]\n" + Interface + " = gtk ; kde ;\n"
	testfs.Write(t, path, original)

	_, err := Install(env)
	require.NoError(t, err)
	assert.Equal(t, "[preferred]\n"+Interface+"=dfiles;gtk;kde;\n", testfs.Read(t, path))

	_, err = Uninstall(env)
	require.NoError(t, err)
	assert.Equal(t, original, testfs.Read(t, path))
}

func TestInstallWithoutUserFile(t *testing.T) {
	env := testEnv(t)
	path := userConfig(env, "niri-portals.conf")

	status, err := Install(env)
	require.NoError(t, err)
	assert.Equal(t, "[preferred]\n"+Interface+"=dfiles;*;\n", testfs.Read(t, path))
	assert.True(t, status.Active)
	assert.Equal(t, path, status.Config.Path)

	status, err = Uninstall(env)
	require.NoError(t, err)
	assert.NoFileExists(t, path)
	assert.False(t, status.Config.Exists)
	assert.False(t, status.Kept)
}

func TestInstallKeepsSystemPortal(t *testing.T) {
	env := testEnv(t)
	testfs.Write(t, sysPortal(env, "dfiles"), dfilesPortal)
	testfs.Write(t, filepath.Join(env.DataDirs[0], "dbus-1", "services", ServiceFile), "[D-BUS Service]\nName="+BusName+"\nExec=/usr/bin/dfiles run\n")

	status, err := Install(env)
	require.NoError(t, err)
	assert.NoFileExists(t, filepath.Join(userPortalDir(env), PortalFile))
	assert.NoFileExists(t, filepath.Join(userServiceDir(env), ServiceFile))
	assert.True(t, status.PortalFile.Found)
	assert.False(t, status.PortalFile.User)
	assert.False(t, status.Service.User)
	assert.True(t, status.Active)

	_, err = Uninstall(env)
	require.NoError(t, err)
	assert.FileExists(t, sysPortal(env, "dfiles"))
	assert.FileExists(t, filepath.Join(env.DataDirs[0], "dbus-1", "services", ServiceFile))
}

func TestInstallRewritesMovedServiceExec(t *testing.T) {
	env := testEnv(t)
	servicePath := filepath.Join(userServiceDir(env), ServiceFile)
	testfs.Write(t, servicePath, "[D-BUS Service]\nName="+BusName+"\nExec=/old/dfiles run --hidden --portal-activated\n")

	_, err := Install(env)
	require.NoError(t, err)
	assert.Contains(t, testfs.Read(t, servicePath), "Exec=/tmp/x/dfiles run --hidden --portal-activated\n")
	state, ok := readState(env)
	require.True(t, ok)
	assert.False(t, state.CreatedService)

	_, err = Uninstall(env)
	require.NoError(t, err)
	assert.FileExists(t, servicePath)
}

func TestUninstallAfterHandEdit(t *testing.T) {
	env := testEnv(t)
	testfs.Write(t, sysPortal(env, "gtk"), gtkPortal)
	path := userConfig(env, "niri-portals.conf")
	_, err := Install(env)
	require.NoError(t, err)

	edited := setPreferred(testfs.Read(t, path), "dfiles;kde;")
	testfs.Write(t, path, edited)

	status, err := Uninstall(env)
	require.NoError(t, err)
	assert.True(t, status.Kept)
	assert.Equal(t, edited, testfs.Read(t, path))
	assert.NoFileExists(t, filepath.Join(userPortalDir(env), PortalFile))
	assert.NoFileExists(t, filepath.Join(userServiceDir(env), ServiceFile))
	assert.NoFileExists(t, statePath(env))
}

func TestUninstallWithoutState(t *testing.T) {
	env := testEnv(t)
	path := userConfig(env, "niri-portals.conf")
	testfs.Write(t, path, "[preferred]\n"+Interface+"=dfiles;gtk;\n")
	status, err := Uninstall(env)
	require.NoError(t, err)
	assert.False(t, status.Kept)
	assert.Equal(t, "[preferred]\n"+Interface+"=gtk;\n", testfs.Read(t, path))

	testfs.Write(t, path, "[preferred]\ndefault=gtk;\n"+Interface+"=dfiles\n")
	_, err = Uninstall(env)
	require.NoError(t, err)
	assert.Equal(t, "[preferred]\ndefault=gtk;\n", testfs.Read(t, path))

	testfs.Write(t, path, "[preferred]\n"+Interface+"=gtk;dfiles;\n")
	_, err = Uninstall(env)
	require.NoError(t, err)
	assert.Equal(t, "[preferred]\n"+Interface+"=gtk;dfiles;\n", testfs.Read(t, path))
}

func TestInstallRefusesSymlinkedConfig(t *testing.T) {
	env := testEnv(t)
	require.NoError(t, os.MkdirAll(filepath.Join(env.ConfigHome, "xdg-desktop-portal"), 0o755))
	target := filepath.Join(env.ConfigHome, "generated.conf")
	testfs.Write(t, target, "[preferred]\ndefault=gtk;\n")
	require.NoError(t, os.Symlink(target, filepath.Join(env.ConfigHome, "xdg-desktop-portal", "niri-portals.conf")))

	_, err := Install(env)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "symlink")
	assert.Equal(t, "[preferred]\ndefault=gtk;\n", testfs.Read(t, target))
}

func TestInstallRefused(t *testing.T) {
	env := testEnv(t)
	env.PortalDir = filepath.Join(env.SysConfDir, "portal-dir")
	_, err := Install(env)
	assert.ErrorContains(t, err, "portal directory")

	env = testEnv(t)
	env.Exec = "/opt/my apps/dfiles"
	_, err = Install(env)
	assert.ErrorContains(t, err, "binary path")

	env.Exec = "dfiles"
	_, err = Install(env)
	assert.ErrorContains(t, err, "binary path")
	assert.NoFileExists(t, statePath(env))
}

func TestReinstallKeepsOriginalLine(t *testing.T) {
	env := testEnv(t)
	testfs.Write(t, sysPortal(env, "gtk"), gtkPortal)
	path := userConfig(env, "niri-portals.conf")
	original := "[preferred]\n" + Interface + "=gtk;\n"
	testfs.Write(t, path, original)

	_, err := Install(env)
	require.NoError(t, err)
	_, err = Install(env)
	require.NoError(t, err)
	assert.Equal(t, "[preferred]\n"+Interface+"=dfiles;gtk;\n", testfs.Read(t, path))
	state, ok := readState(env)
	require.True(t, ok)
	assert.Equal(t, Interface+"=gtk;", state.OriginalLine)
	assert.True(t, state.HadLine)

	_, err = Uninstall(env)
	require.NoError(t, err)
	assert.Equal(t, original, testfs.Read(t, path))
}

func TestParseEnviron(t *testing.T) {
	data := []byte("A=1\x00B=x=y\x00\x00NOEQ\x00=bare\x00XDG_CURRENT_DESKTOP=niri:GNOME\x00")
	got := ParseEnviron(data)
	assert.Equal(t, map[string]string{"A": "1", "B": "x=y", "NOEQ": "", "XDG_CURRENT_DESKTOP": "niri:GNOME"}, got)
	assert.Empty(t, ParseEnviron(nil))
}

func TestWithEnviron(t *testing.T) {
	env := testEnv(t)
	got := env.WithEnviron(map[string]string{
		"XDG_CURRENT_DESKTOP":    "Hyprland:wlroots",
		"XDG_DATA_DIRS":          "/a:/b",
		"XDG_CONFIG_DIRS":        "/c",
		"XDG_DESKTOP_PORTAL_DIR": "/portal",
		"XDG_DATA_HOME":          "/dh",
		"XDG_CONFIG_HOME":        "/ch",
	})
	assert.Equal(t, []string{"Hyprland", "wlroots"}, got.Desktops)
	assert.Equal(t, []string{"/a", "/b"}, got.DataDirs)
	assert.Equal(t, []string{"/c"}, got.ConfigDirs)
	assert.Equal(t, "/portal", got.PortalDir)
	assert.Equal(t, "/dh", got.DataHome)
	assert.Equal(t, "/ch", got.ConfigHome)
	assert.Equal(t, env.StateHome, got.StateHome)
	assert.Equal(t, env.Exec, got.Exec)

	same := env.WithEnviron(map[string]string{"HOME": "/x"})
	assert.Equal(t, env, same)

	defaults := env.WithEnviron(map[string]string{"XDG_DATA_DIRS": "", "XDG_CURRENT_DESKTOP": ""})
	assert.Equal(t, []string{"/usr/local/share", "/usr/share"}, defaults.DataDirs)
	assert.Nil(t, defaults.Desktops)
}

func TestProcessStart(t *testing.T) {
	stat := "1234 (xdg-desktop-por) S 1 1234 1234 0 -1 4194560 100 0 0 0 1 2 0 0 20 0 3 0 123450 1000 10 18446744073709551615 1 1 0 0 0 0 0 0 0 0 0 0 17 2 0 0 0 0 0 0 0 0 0 0 0 0 0"
	got, ok := processStart(stat, 1_700_000_000, 100)
	require.True(t, ok)
	assert.Equal(t, time.Unix(1_700_001_234, 500_000_000), got)

	_, ok = processStart("1234 (x) S 1", 0, 100)
	assert.False(t, ok)
	_, ok = processStart("garbage", 0, 100)
	assert.False(t, ok)
	_, ok = processStart(stat, 0, 0)
	assert.False(t, ok)
}

func TestStopFrontendEndsTheProcess(t *testing.T) {
	cmd := exec.Command("sleep", "30")
	require.NoError(t, cmd.Start())
	waited := make(chan error, 1)
	go func() { waited <- cmd.Wait() }()

	require.NoError(t, stopFrontend(cmd.Process.Pid))

	var exit *exec.ExitError
	require.ErrorAs(t, <-waited, &exit)
	assert.Equal(t, syscall.SIGTERM, exit.Sys().(syscall.WaitStatus).Signal())
}

func TestStopFrontendWithoutAProcess(t *testing.T) {
	assert.NoError(t, stopFrontend(0), "nothing runs, nothing to stop")
}

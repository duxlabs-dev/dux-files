package sandbox

import (
	"errors"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

type fakeProbe struct {
	env     map[string]string
	files   map[string]string
	exists  map[string]bool
	path    map[string]bool
	portals map[string]uint32
	calls   []string
}

func (f *fakeProbe) Getenv(key string) string { return f.env[key] }

func (f *fakeProbe) ReadFile(path string) ([]byte, error) {
	data, ok := f.files[path]
	if !ok {
		return nil, errors.New("missing " + path)
	}
	return []byte(data), nil
}

func (f *fakeProbe) Exists(path string) bool { return f.exists[path] }

func (f *fakeProbe) LookPath(name string) (string, error) {
	if f.path[name] {
		return "/usr/bin/" + name, nil
	}
	return "", errors.New("not found")
}

func (f *fakeProbe) PortalVersion(dest, objectPath, iface string) (uint32, bool) {
	f.calls = append(f.calls, dest+" "+objectPath+" "+iface)
	v, ok := f.portals[iface]
	return v, ok
}

const fullInfo = `[Application]
name=dev.duxlabs.files
runtime=runtime/org.kde.Platform/x86_64/6.11

[Context]
shared=ipc;
sockets=wayland;fallback-x11;
filesystems=/tmp;xdg-run/gvfsd;xdg-config/mimeapps.list:create;/var;host;xdg-run/gvfs;

[Session Bus Policy]
org.gtk.vfs.*=talk
org.freedesktop.Flatpak=talk
org.freedesktop.FileManager1=own

[System Bus Policy]
org.freedesktop.UDisks2=talk
`

func sandboxProbe(info string) *fakeProbe {
	return &fakeProbe{
		env:     map[string]string{"FLATPAK_ID": "dev.duxlabs.files", "HOME": "/home/u", "XDG_DATA_DIRS": "/app/share:/usr/share", "XDG_CONFIG_DIRS": "/app/etc/xdg:/etc/xdg", "XDG_CACHE_HOME": "/home/u/.var/app/x/cache"},
		files:   map[string]string{flatpakInfoPath: info},
		exists:  map[string]bool{hostOSRoot + "/usr": true},
		path:    map[string]bool{"flatpak-spawn": true},
		portals: map[string]uint32{"org.freedesktop.portal.Trash": 1, "org.freedesktop.portal.OpenURI": 5, "org.freedesktop.portal.FileTransfer": 1},
	}
}

func TestNativeHasEverythingAndSkipsPortals(t *testing.T) {
	p := &fakeProbe{env: map[string]string{"HOME": "/home/u", "XDG_CONFIG_HOME": "/cfg"}, portals: map[string]uint32{"org.freedesktop.portal.Trash": 1}}
	caps := Detect(p)

	assert.False(t, caps.Sandboxed)
	assert.True(t, caps.HostFilesystem)
	assert.True(t, caps.UDisks)
	assert.True(t, caps.Gvfs)
	assert.True(t, caps.HostSpawn)
	assert.Empty(t, caps.Portals)
	assert.Empty(t, p.calls)
	assert.Equal(t, "/cfg", caps.Dirs.ConfigHome)
	assert.Equal(t, "/home/u/.local/share", caps.Dirs.DataHome)
	assert.Equal(t, []string{"/usr/local/share", "/usr/share"}, caps.Dirs.DataDirs)
	for name, f := range caps.Features {
		assert.True(t, f.Available, name)
		assert.False(t, f.Degraded, name)
		assert.Empty(t, f.Reason, name)
	}
}

func TestSandboxWithHostAccess(t *testing.T) {
	caps := Detect(sandboxProbe(fullInfo))

	assert.True(t, caps.Sandboxed)
	assert.Equal(t, "dev.duxlabs.files", caps.AppID)
	assert.True(t, caps.HostFilesystem)
	assert.True(t, caps.UserFilesystem)
	assert.Equal(t, "/run/host", caps.HostOSRoot)
	assert.Equal(t, []string{"/tmp", "/var", "host", "xdg-config/mimeapps.list", "xdg-run/gvfs", "xdg-run/gvfsd"}, caps.Filesystems)
	assert.True(t, caps.UDisks)
	assert.True(t, caps.Gvfs)
	assert.True(t, caps.HostSpawn)
	assert.Equal(t, map[string]uint32{"Trash": 1, "OpenURI": 5, "FileTransfer": 1}, caps.Portals)

	assert.Equal(t, "/home/u/.cache", caps.Dirs.CacheHome, "the sandbox cache is not the host cache")
	assert.Equal(t, "/home/u/.config", caps.Dirs.ConfigHome)
	assert.Equal(t, []string{"/app/share", "/usr/share", "/home/u/.local/share/flatpak/exports/share", "/var/lib/flatpak/exports/share", "/run/host/usr/local/share", "/run/host/usr/share"}, caps.Dirs.DataDirs)
	assert.Equal(t, []string{"/app/etc/xdg", "/etc/xdg", "/run/host/etc/xdg"}, caps.Dirs.ConfigDirs)

	assert.Equal(t, Feature{Available: true}, caps.Features["trash"])
	assert.Equal(t, Feature{Available: true}, caps.Features["setDefault"])
	assert.Equal(t, Feature{Available: true}, caps.Features["devices"])
	assert.Equal(t, Feature{Available: true}, caps.Features["network"])
	assert.Equal(t, Feature{Available: true}, caps.Features["terminal"])
	assert.Equal(t, Feature{Available: true, Degraded: true, Reason: ReasonPortalChooser}, caps.Features["openWith"])
	assert.Equal(t, Feature{Reason: ReasonSandboxed}, caps.Features["elevation"])
	assert.Equal(t, Feature{Reason: ReasonSandboxed}, caps.Features["indexSearch"])
}

func TestHostXDGOverridesWin(t *testing.T) {
	p := sandboxProbe(fullInfo)
	p.env["HOST_XDG_CACHE_HOME"] = "/mnt/cache"
	p.env["HOST_XDG_DATA_HOME"] = "/mnt/data"
	caps := Detect(p)

	assert.Equal(t, "/mnt/cache", caps.Dirs.CacheHome)
	assert.Equal(t, "/mnt/data", caps.Dirs.DataHome)
	assert.Equal(t, "/home/u/.config", caps.Dirs.ConfigHome)
}

func TestSandboxWithoutHostAccess(t *testing.T) {
	info := `[Context]
filesystems=host;!host;xdg-download;

[Session Bus Policy]
org.gtk.vfs.*=see
`
	p := sandboxProbe(info)
	p.path = nil
	caps := Detect(p)

	assert.False(t, caps.HostFilesystem)
	assert.False(t, caps.UserFilesystem)
	assert.Empty(t, caps.HostOSRoot)
	assert.Equal(t, []string{"xdg-download"}, caps.Filesystems)
	assert.False(t, caps.UDisks)
	assert.False(t, caps.Gvfs)
	assert.False(t, caps.HostSpawn)
	assert.Equal(t, "/home/u/.var/app/x/cache", caps.Dirs.CacheHome, "private dirs stay when the host is out of reach")
	assert.Equal(t, []string{"/app/share", "/usr/share"}, caps.Dirs.DataDirs)

	assert.Equal(t, Feature{Reason: ReasonNoHostFS}, caps.Features["trash"])
	assert.Equal(t, Feature{Available: true, Degraded: true, Reason: ReasonNoHostFS}, caps.Features["thumbnails"])
	assert.Equal(t, Feature{Reason: ReasonNoUDisks}, caps.Features["devices"])
	assert.Equal(t, Feature{Reason: ReasonNoGvfs}, caps.Features["network"])
	assert.Equal(t, Feature{Reason: ReasonNoHostSpawn}, caps.Features["terminal"])
	assert.Equal(t, Feature{Reason: ReasonNoHostSpawn}, caps.Features["run"])
}

func TestHomeGrantReachesUserStateButNotTheHostOS(t *testing.T) {
	p := sandboxProbe(`[Context]
filesystems=home;
`)
	caps := Detect(p)

	assert.False(t, caps.HostFilesystem)
	assert.True(t, caps.UserFilesystem)
	assert.Empty(t, caps.HostOSRoot)
	assert.Equal(t, "/home/u/.cache", caps.Dirs.CacheHome)
	assert.Equal(t, []string{"/app/share", "/usr/share"}, caps.Dirs.DataDirs, "no /run/host without host-os")
	assert.Equal(t, Feature{Available: true}, caps.Features["trash"])
	assert.Equal(t, Feature{Available: true}, caps.Features["setDefault"])
	assert.Equal(t, Feature{Reason: ReasonNoHostFS}, caps.Features["hostFilesystem"])
}

func TestOverridesRemoveGrantsWithModes(t *testing.T) {
	negated := sandboxProbe(`[Context]
filesystems=host:ro;!host:ro;
`)
	assert.False(t, Detect(negated).HostFilesystem)

	reset := sandboxProbe(`[Context]
filesystems=host;xdg-run/gvfs;home:reset;
`)
	caps := Detect(reset)
	assert.False(t, caps.HostFilesystem)
	assert.Equal(t, []string{}, caps.Filesystems)
}

func TestMissingRuntimeDirsDefaultToTheSandboxOnes(t *testing.T) {
	p := sandboxProbe(fullInfo)
	delete(p.env, "XDG_DATA_DIRS")
	delete(p.env, "XDG_CONFIG_DIRS")
	caps := Detect(p)

	assert.Equal(t, []string{"/app/share", "/usr/share", "/home/u/.local/share/flatpak/exports/share", "/var/lib/flatpak/exports/share", "/run/host/usr/local/share", "/run/host/usr/share"}, caps.Dirs.DataDirs)
	assert.Equal(t, []string{"/app/etc/xdg", "/etc/xdg", "/run/host/etc/xdg"}, caps.Dirs.ConfigDirs)
}

func TestHostGrantNeedsTheHostMount(t *testing.T) {
	p := sandboxProbe(fullInfo)
	p.exists = nil
	caps := Detect(p)

	assert.False(t, caps.HostFilesystem)
	assert.Equal(t, Feature{Reason: ReasonNoHostFS}, caps.Features["hostFilesystem"])
}

func TestHostSpawnNeedsTalkNameAndBinary(t *testing.T) {
	withoutTalk := sandboxProbe(`[Session Bus Policy]
org.freedesktop.Flatpak=see
`)
	assert.False(t, Detect(withoutTalk).HostSpawn)

	withoutBinary := sandboxProbe(fullInfo)
	withoutBinary.path = nil
	assert.False(t, Detect(withoutBinary).HostSpawn)
}

func TestGvfsNeedsTalkNameAndBothRuntimeDirs(t *testing.T) {
	missingDir := sandboxProbe(`[Context]
filesystems=host;xdg-run/gvfs;

[Session Bus Policy]
org.gtk.vfs.*=talk
`)
	assert.False(t, Detect(missingDir).Gvfs)

	missingTalk := sandboxProbe(`[Context]
filesystems=host;xdg-run/gvfs;xdg-run/gvfsd;
`)
	assert.False(t, Detect(missingTalk).Gvfs)
}

func TestMissingInfoFileFallsBackToEnvID(t *testing.T) {
	p := sandboxProbe(fullInfo)
	p.files = nil
	caps := Detect(p)

	assert.True(t, caps.Sandboxed)
	assert.Equal(t, "dev.duxlabs.files", caps.AppID)
	assert.False(t, caps.HostFilesystem)
	assert.False(t, caps.UDisks)
}

func TestParseFilesystemsModes(t *testing.T) {
	fs := parseFilesystems("host:ro;xdg-config/kdeglobals:ro;/a:b/c;xdg-run/gvfs:create;:ro;")
	assert.Equal(t, map[string]string{"host": "ro", "xdg-config/kdeglobals": "ro", "/a:b/c": "rw", "xdg-run/gvfs": "create"}, fs)
}

func TestCanTalkWildcard(t *testing.T) {
	policy := map[string]string{"org.gtk.vfs.*": "talk", "org.kde.*": "see", "com.example.Exact": "own"}
	assert.True(t, canTalk(policy, "org.gtk.vfs.Daemon"))
	assert.False(t, canTalk(policy, "org.gtk.vfs"))
	assert.False(t, canTalk(policy, "org.gtk.vfsd"))
	assert.False(t, canTalk(policy, "org.kde.Foo"))
	assert.True(t, canTalk(policy, "com.example.Exact"))
	assert.False(t, canTalk(policy, "com.example.Exact.Sub"))
}

func TestExportHostDirs(t *testing.T) {
	for _, key := range []string{"XDG_CONFIG_HOME", "XDG_DATA_HOME", "XDG_CACHE_HOME", "XDG_STATE_HOME", "XDG_CONFIG_DIRS", "XDG_DATA_DIRS"} {
		t.Setenv(key, "private")
	}

	ExportHostDirs(Detect(sandboxProbe(fullInfo)))

	assert.Equal(t, "/home/u/.config", os.Getenv("XDG_CONFIG_HOME"))
	assert.Equal(t, "/home/u/.local/share", os.Getenv("XDG_DATA_HOME"))
	assert.Equal(t, "/home/u/.cache", os.Getenv("XDG_CACHE_HOME"))
	assert.Equal(t, "/home/u/.local/state", os.Getenv("XDG_STATE_HOME"))
	assert.Equal(t, "/app/etc/xdg:/etc/xdg:/run/host/etc/xdg", os.Getenv("XDG_CONFIG_DIRS"))
	assert.Equal(t, "/app/share:/usr/share:/home/u/.local/share/flatpak/exports/share:/var/lib/flatpak/exports/share:/run/host/usr/local/share:/run/host/usr/share", os.Getenv("XDG_DATA_DIRS"))
}

func TestExportHostDirsIsANoOpWithoutUserAccess(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", "private")
	p := sandboxProbe(fullInfo)
	p.exists = nil
	ExportHostDirs(Detect(p))
	assert.Equal(t, "private", os.Getenv("XDG_DATA_HOME"))

	ExportHostDirs(Detect(&fakeProbe{env: map[string]string{"HOME": "/home/u"}}))
	assert.Equal(t, "private", os.Getenv("XDG_DATA_HOME"))
}

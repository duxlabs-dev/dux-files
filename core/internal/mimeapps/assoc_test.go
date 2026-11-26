package mimeapps

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func idsOf(entries []*Entry) []string {
	out := make([]string, 0, len(entries))
	for _, entry := range entries {
		out = append(out, entry.ID)
	}
	return out
}

func TestDefaultSkipsMissingAndUnlaunchable(t *testing.T) {
	svc, _ := fixture(t)

	assert.Equal(t, "editor.desktop", svc.Default("text/plain"), "ghost.desktop has no file, so the next id wins")
	assert.Equal(t, "viewer.desktop", svc.Default("image/png"))
	assert.Equal(t, "system-viewer.desktop", svc.Default("image/jpeg"), "the system config supplies what the user config does not")
}

func TestDefaultHonoursDesktopVariant(t *testing.T) {
	svc, _ := fixture(t)
	assert.Equal(t, "editor.desktop", svc.Default("text/plain"))

	t.Setenv("XDG_CURRENT_DESKTOP", "GNOME:GNOME-Classic")
	assert.Equal(t, "viewer.desktop", svc.Default("text/plain"), "gnome-mimeapps.list outranks mimeapps.list in the same directory")
}

func TestDefaultFallsBackToAssociations(t *testing.T) {
	svc, _ := fixture(t)
	assert.Equal(t, "editor.desktop", svc.Default("text/markdown"), "no default is set, so the first association is used")
}

func TestDefaultFoldsAliases(t *testing.T) {
	svc, _ := fixture(t)
	assert.Equal(t, "viewer.desktop", svc.Default("image/x-png"))
}

func TestAssociationsOrdering(t *testing.T) {
	svc, _ := fixture(t)

	assert.Equal(t, []string{"editor.desktop", "viewer.desktop", "background.desktop"}, idsOf(svc.Associations("text/markdown")),
		"added associations come first in their listed order, then the rest")

	plain := idsOf(svc.Associations("text/plain"))
	assert.Equal(t, "editor.desktop", plain[0], "the default leads")
	assert.NotContains(t, plain, "broken.desktop", "TryExec that resolves to nothing is not launchable")
	assert.NotContains(t, plain, "hidden.desktop")
	assert.NotContains(t, plain, "settings.desktop", "a Type=Link entry is not an application")
}

func TestRemovedAssociationsBlockLowerPrecedence(t *testing.T) {
	svc, _ := fixture(t)

	png := idsOf(svc.Associations("image/png"))
	assert.Contains(t, png, "viewer.desktop")
	assert.NotContains(t, png, "system-viewer.desktop", "the user config removed it")
	assert.NotContains(t, png, "hidden.desktop")
}

func TestSubclassHandlersAreOffered(t *testing.T) {
	svc, _ := fixture(t)
	assert.Contains(t, idsOf(svc.Associations("application/x-shellscript")), "editor.desktop",
		"a text/plain handler also handles its subclasses")
}

func TestAppsExcludesNoDisplay(t *testing.T) {
	svc, _ := fixture(t)

	apps := idsOf(svc.Apps())
	assert.Equal(t, []string{"editor.desktop", "system-viewer.desktop", "vendor-terminalapp.desktop", "viewer.desktop"}, apps)
	assert.NotContains(t, apps, "background.desktop", "NoDisplay entries stay out of the picker")
	assert.Contains(t, idsOf(svc.Associations("text/markdown")), "background.desktop", "but they still handle their type")
}

func TestSymlinkedApplicationsDir(t *testing.T) {
	realRoot := t.TempDir()
	realApps := filepath.Join(realRoot, "applications")
	require.NoError(t, os.MkdirAll(realApps, 0o755))

	externalVendor := filepath.Join(t.TempDir(), "vendor")
	require.NoError(t, os.MkdirAll(externalVendor, 0o755))
	externalFile := filepath.Join(t.TempDir(), "linked.desktop")

	desktop := "[Desktop Entry]\nType=Application\nName=Zathura\nExec=true %f\nMimeType=application/pdf;\n"
	require.NoError(t, os.WriteFile(filepath.Join(realApps, "zathura.desktop"), []byte(desktop), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(externalVendor, "reader.desktop"), []byte(desktop), 0o644))
	require.NoError(t, os.WriteFile(externalFile, []byte(desktop), 0o644))
	require.NoError(t, os.Symlink(externalVendor, filepath.Join(realApps, "vendor")))
	require.NoError(t, os.Symlink(externalFile, filepath.Join(realApps, "linked.desktop")))
	require.NoError(t, os.Symlink(filepath.Join(realRoot, "missing.desktop"), filepath.Join(realApps, "broken.desktop")))
	require.NoError(t, os.Symlink(realApps, filepath.Join(realApps, "loop")))

	dataHome := filepath.Join(t.TempDir(), "share")
	require.NoError(t, os.MkdirAll(dataHome, 0o755))
	appsLink := filepath.Join(dataHome, "applications")
	require.NoError(t, os.Symlink(realApps, appsLink))

	t.Setenv("XDG_DATA_HOME", dataHome)
	t.Setenv("XDG_DATA_DIRS", dataHome)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	svc := NewServiceForDirs(nil)
	assert.Equal(t, []string{"linked.desktop", "vendor-reader.desktop", "zathura.desktop"}, idsOf(svc.Associations("application/pdf")))

	// Swapping the applications symlink to another directory with an identical mtime invalidates the cache.
	otherApps := filepath.Join(t.TempDir(), "applications-v2")
	require.NoError(t, os.MkdirAll(otherApps, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(otherApps, "okular.desktop"), []byte(desktop), 0o644))
	info, err := os.Stat(realApps)
	require.NoError(t, err)
	require.NoError(t, os.Chtimes(otherApps, info.ModTime(), info.ModTime()))
	require.NoError(t, os.Remove(appsLink))
	require.NoError(t, os.Symlink(otherApps, appsLink))

	assert.Equal(t, []string{"okular.desktop"}, idsOf(svc.Associations("application/pdf")))
}

func TestEntryFields(t *testing.T) {
	svc, _ := fixture(t)

	entry := svc.EntryByID("editor.desktop")
	require.NotNil(t, entry)
	assert.Equal(t, "Editor", entry.Name)
	assert.Equal(t, "Text Editor", entry.GenericName)
	assert.Equal(t, "accessories-text-editor", entry.Icon)
	assert.False(t, entry.Terminal)

	assert.True(t, svc.EntryByID("vendor-terminalapp.desktop").Terminal, "a subdirectory becomes a dash in the id")
	assert.False(t, svc.EntryByID("hidden.desktop").Launchable(), "Hidden=true entries are never launched")
}

func TestLocalizedNames(t *testing.T) {
	t.Cleanup(func() { localeCandidates = sync.OnceValue(sessionLocaleCandidates) })
	svc, _ := fixture(t)
	assert.Equal(t, "Editor", svc.EntryByID("editor.desktop").Name)

	t.Setenv("LC_ALL", "de_AT.UTF-8")
	localeCandidates = sync.OnceValue(sessionLocaleCandidates)
	svc2, _ := fixture(t)
	t.Setenv("LC_ALL", "de_AT.UTF-8")
	localeCandidates = sync.OnceValue(sessionLocaleCandidates)
	assert.Equal(t, "Schreibtisch", svc2.EntryByID("editor.desktop").Name)

	t.Setenv("LC_ALL", "de_CH.UTF-8")
	localeCandidates = sync.OnceValue(sessionLocaleCandidates)
	svc3, _ := fixture(t)
	t.Setenv("LC_ALL", "de_CH.UTF-8")
	localeCandidates = sync.OnceValue(sessionLocaleCandidates)
	assert.Equal(t, "Bearbeiter", svc3.EntryByID("editor.desktop").Name, "de_CH falls back to the de translation")

	t.Setenv("LANGUAGE", "de_AT:de")
	t.Setenv("LC_ALL", "en_US.UTF-8")
	localeCandidates = sync.OnceValue(sessionLocaleCandidates)
	svc4, _ := fixture(t)
	t.Setenv("LANGUAGE", "de_AT:de")
	localeCandidates = sync.OnceValue(sessionLocaleCandidates)
	assert.Equal(t, "Schreibtisch", svc4.EntryByID("editor.desktop").Name, "LANGUAGE outranks LC_ALL like g_get_language_names")
}

func TestSetDefaultRewritesUserList(t *testing.T) {
	svc, root := fixture(t)
	path := filepath.Join(root, "xdg", "config", "mimeapps.list")

	require.NoError(t, svc.SetDefault("image/png", "system-viewer.desktop"))
	updated, err := os.ReadFile(path)
	require.NoError(t, err)

	assert.Contains(t, string(updated), "image/png=system-viewer.desktop")
	assert.Contains(t, string(updated), "[Added Associations]", "other groups survive")
	assert.Contains(t, string(updated), "text/plain=ghost.desktop;editor.desktop;", "other keys survive")
	assert.Equal(t, "system-viewer.desktop", svc.Default("image/png"))
}

func TestSetDefaultAddsKeyAndGroup(t *testing.T) {
	svc, root := fixture(t)
	path := filepath.Join(root, "xdg", "config", "mimeapps.list")

	require.NoError(t, svc.SetDefault("text/markdown", "viewer.desktop"))
	assert.Equal(t, "viewer.desktop", svc.Default("text/markdown"))

	require.NoError(t, os.WriteFile(path, []byte("[Added Associations]\ntext/plain=editor.desktop;\n"), 0o644))
	require.NoError(t, svc.SetDefault("image/png", "viewer.desktop"))
	updated, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Contains(t, string(updated), "[Default Applications]\nimage/png=viewer.desktop")
	assert.Contains(t, string(updated), "[Added Associations]")
	assert.Contains(t, string(updated), "image/png=viewer.desktop;", "a new default is also an added association")
	assert.Contains(t, string(updated), "text/plain=editor.desktop;")
}

func TestSetDefaultRejectsUnknownEntry(t *testing.T) {
	svc, _ := fixture(t)

	require.Error(t, svc.SetDefault("image/png", "nope.desktop"))
	require.Error(t, svc.SetDefault("image/png", "broken.desktop"), "an entry whose TryExec is missing cannot become the default")
	require.Error(t, svc.SetDefault("image/png\n[Default Applications]", "viewer.desktop"))
}

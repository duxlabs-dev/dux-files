package icontheme

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// home prepares a fake home with its own config dir.
func home(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, "config"))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "config"), 0o755))
	return dir
}

func write(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}

func TestDetectedReadsTheDesktopConfig(t *testing.T) {
	dir := home(t)
	write(t, filepath.Join(dir, "config", "kdeglobals"), "[Icons]\nTheme=WhiteSur-dark\n")
	assert.Equal(t, "WhiteSur-dark", Detected(dir))
}

func TestDetectedFallsBackToQt6ctThenGSettings(t *testing.T) {
	dir := home(t)
	write(t, filepath.Join(dir, "config", "qt6ct", "qt6ct.conf"), "[Appearance]\nicon_theme=Bibata\n")
	assert.Equal(t, "Bibata", detected(dir, func() string { return "" }))

	other := home(t)
	assert.Equal(t, "Adwaita", detected(other, func() string { return "Adwaita" }))
}

func TestDetectedReadsTheGtkSettings(t *testing.T) {
	dir := home(t)
	write(t, filepath.Join(dir, "config", "gtk-3.0", "settings.ini"), "[Settings]\ngtk-icon-theme-name=Adwaita-dark\n")
	assert.Equal(t, "Adwaita-dark", detected(dir, func() string { return "" }))
}

func TestSettingRoundTrips(t *testing.T) {
	dir := home(t)
	assert.Equal(t, "auto", Setting(dir))

	require.NoError(t, Set(dir, "Breeze-dark"))
	assert.Equal(t, "Breeze-dark", Setting(dir))

	require.NoError(t, Set(dir, ""))
	assert.Equal(t, "auto", Setting(dir))
}

func TestEffectivePrefersThePickedTheme(t *testing.T) {
	dir := home(t)
	write(t, filepath.Join(dir, "config", "kdeglobals"), "[Icons]\nTheme=WhiteSur-dark\n")

	assert.Equal(t, "WhiteSur-dark", Effective(dir))

	require.NoError(t, Set(dir, "Breeze"))
	assert.Equal(t, "Breeze", Effective(dir))
}

func TestInstalledListsEveryThemeOnce(t *testing.T) {
	first, second := t.TempDir(), t.TempDir()
	for _, root := range []string{first, second} {
		for _, theme := range []string{"Adwaita", "hicolor"} {
			require.NoError(t, os.MkdirAll(filepath.Join(root, theme, "scalable"), 0o755))
			require.NoError(t, os.WriteFile(filepath.Join(root, theme, "index.theme"), []byte("[Icon Theme]\n"), 0o644))
		}
	}
	require.NoError(t, os.MkdirAll(filepath.Join(second, "Breeze", "scalable"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(second, "Breeze", "index.theme"), []byte("[Icon Theme]\n"), 0o644))

	assert.Equal(t, []string{"Adwaita", "Breeze", "hicolor"}, Installed([]string{first, second}))
}

func TestInstalledLeavesCursorThemesOut(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "Breeze", "cursors"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "Breeze", "index.theme"), []byte("[Icon Theme]\n"), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(root, "Breeze", "cursors_scalable"), 0o755))

	assert.Empty(t, Installed([]string{root}))
}

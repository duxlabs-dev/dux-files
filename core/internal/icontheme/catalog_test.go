package icontheme

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// icon writes one icon file into a theme tree under root.
func icon(t *testing.T, root, theme, dir, name string) {
	t.Helper()
	target := filepath.Join(root, theme, dir)
	require.NoError(t, os.MkdirAll(target, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(target, name), nil, 0o644))
}

func inherit(t *testing.T, root, theme, parents string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Join(root, theme), 0o755))
	body := "[Icon Theme]\nName=" + theme + "\nInherits=" + parents + "\n"
	require.NoError(t, os.WriteFile(filepath.Join(root, theme, "index.theme"), []byte(body), 0o644))
}

func TestScanSplitsLogosFromFolderArt(t *testing.T) {
	root := t.TempDir()
	icon(t, root, "Paper", "scalable/places", "folder.svg")
	icon(t, root, "Paper", "scalable/places", "folder-symbolic.svg")
	icon(t, root, "Paper", "scalable/emblems", "emblem-favorite.svg")
	icon(t, root, "Paper", "scalable/apps", "steam.png")
	icon(t, root, "Paper", "scalable/mimetypes", "text-plain.svg")

	catalog := Scan([]string{root}, "Paper")

	assert.Equal(t, []string{"emblem-favorite", "steam"}, catalog.Badges)
	assert.Equal(t, []string{"folder"}, catalog.Icons)
}

func TestScanDropsThePurposeVariantsOfAFolderFamily(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{
		"folder", "folder-blue", "folder_color", "folder_color_green", "folder_color_red",
		"folder_color_green_documents", "folder_color_green_pictures", "folder-color-red-desktop",
	} {
		icon(t, root, "Paper", "places/48", name+".svg")
	}

	catalog := Scan([]string{root}, "Paper")

	assert.Equal(t, []string{"folder", "folder-blue", "folder_color", "folder_color_green", "folder_color_red"}, catalog.Icons)
}

func TestScanReadsOnlyTheConfiguredTheme(t *testing.T) {
	root := t.TempDir()
	icon(t, root, "Paper", "places/scalable", "folder.svg")
	icon(t, root, "Adwaita", "apps", "firefox.svg")

	catalog := Scan([]string{root}, "Paper")

	assert.Equal(t, []string{"folder"}, catalog.Icons)
	assert.Empty(t, catalog.Badges)
}

func TestScanFallsBackToEveryThemeWhenTheSelectionIsGone(t *testing.T) {
	root := t.TempDir()
	icon(t, root, "Adwaita", "apps", "firefox.svg")
	icon(t, root, "Paper", "apps", "steam.svg")

	catalog := Scan([]string{root}, "Removed Theme")

	assert.Equal(t, []string{"firefox", "steam"}, catalog.Badges)
}

func TestScanSkipsWhatItCannotName(t *testing.T) {
	root := t.TempDir()
	icon(t, root, "Paper", "places/scalable", "folder.ico")
	icon(t, root, "Paper", "places/scalable", "folder red.svg")

	catalog := Scan([]string{root}, "Paper")

	assert.Empty(t, catalog.Badges)
	assert.Empty(t, catalog.Icons)
}

func TestThemeReadsTheDesktopSetting(t *testing.T) {
	home := t.TempDir()
	config := filepath.Join(home, ".config")
	require.NoError(t, os.MkdirAll(filepath.Join(config, "gtk-3.0"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(config, "kdeglobals"), []byte("[General]\nfoo=bar\n[Icons]\nTheme=WhiteSur-dark\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(config, "gtk-3.0", "settings.ini"), []byte("[Settings]\ngtk-icon-theme-name=Adwaita\n"), 0o644))

	assert.Equal(t, "WhiteSur-dark", Theme(home))
}

func TestThemeFallsBackToGtkWhenKdeHasNone(t *testing.T) {
	home := t.TempDir()
	config := filepath.Join(home, ".config")
	require.NoError(t, os.MkdirAll(filepath.Join(config, "gtk-4.0"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(config, "gtk-4.0", "settings.ini"), []byte("[Settings]\ngtk-icon-theme-name=Adwaita\n"), 0o644))

	assert.Equal(t, "Adwaita", Theme(home))
}

func TestRootsFollowsXDGOrder(t *testing.T) {
	roots := Roots("/home/me", "/home/me/.local/share", []string{"/usr/local/share", "/usr/share"})

	assert.Equal(t, []string{
		"/home/me/.icons",
		"/home/me/.local/share/icons",
		"/usr/local/share/icons",
		"/usr/share/icons",
	}, roots)
}

func TestScanKeepsOnlyFoldersDrawnAtALargeSize(t *testing.T) {
	root := t.TempDir()
	icon(t, root, "Paper", "places/scalable", "folder-blue.svg")
	icon(t, root, "Paper", "places/48", "folder-green.svg")
	icon(t, root, "Paper", "places/64@2x", "folder-red.png")
	icon(t, root, "Paper", "places/48x48", "folder-teal.png")
	icon(t, root, "Paper", "places/16", "folder-apple.svg")
	icon(t, root, "Paper", "places/22", "folder-new.svg")
	icon(t, root, "Paper", "places/24x24", "folder-add.png")

	catalog := Scan([]string{root}, "Paper")

	assert.Equal(t, []string{"folder-blue", "folder-green", "folder-red", "folder-teal"}, catalog.Icons)
}

func TestScanKeepsAFolderThatHasASmallAndALargeFile(t *testing.T) {
	root := t.TempDir()
	icon(t, root, "Paper", "places/16", "folder-book.svg")
	icon(t, root, "Paper", "places/scalable", "folder-book.svg")

	assert.Equal(t, []string{"folder-book"}, Scan([]string{root}, "Paper").Icons)
}

func TestSmallSizesDoNotFilterBadges(t *testing.T) {
	root := t.TempDir()
	icon(t, root, "Paper", "apps/16", "steam.svg")

	assert.Equal(t, []string{"steam"}, Scan([]string{root}, "Paper").Badges)
}

func TestInstalledListsAThemeOnceAcrossRoots(t *testing.T) {
	first, second := t.TempDir(), t.TempDir()
	for _, root := range []string{first, second} {
		for _, theme := range []string{"Adwaita", "hicolor"} {
			require.NoError(t, os.MkdirAll(filepath.Join(root, theme), 0o755))
		}
	}
	require.NoError(t, os.MkdirAll(filepath.Join(second, "Paper"), 0o755))

	assert.Equal(t, []string{"Adwaita", "hicolor", "Paper"}, installed([]string{first, second}))
}

func TestScanFollowsTheThemesAnOverlayInherits(t *testing.T) {
	user, system := t.TempDir(), t.TempDir()
	inherit(t, user, "Paper-dms-1a2b3c4d", "Paper,hicolor")
	icon(t, user, "Paper-dms-1a2b3c4d", "scalable/places", "folder.svg")
	inherit(t, system, "Paper", "Base, hicolor")
	icon(t, system, "Paper", "scalable/places", "folder-blue.svg")
	icon(t, system, "Paper", "scalable/emblems", "emblem-favorite.svg")
	icon(t, system, "Base", "apps/48", "kate.svg")
	icon(t, system, "hicolor", "48x48/apps", "steam.png")
	icon(t, system, "Adwaita", "scalable/apps", "firefox.svg")

	catalog := Scan([]string{user, system}, "Paper-dms-1a2b3c4d")

	assert.Equal(t, []string{"emblem-favorite", "kate", "steam"}, catalog.Badges)
	assert.Equal(t, []string{"folder", "folder-blue"}, catalog.Icons)
}

func TestScanStopsWhenThemesInheritEachOther(t *testing.T) {
	root := t.TempDir()
	inherit(t, root, "Paper", "Base")
	inherit(t, root, "Base", "Paper,Base")
	icon(t, root, "Paper", "apps", "steam.svg")
	icon(t, root, "Base", "apps", "kate.svg")

	assert.Equal(t, []string{"kate", "steam"}, Scan([]string{root}, "Paper").Badges)
}

func TestScanStopsFollowingAVeryLongLineOfThemes(t *testing.T) {
	root := t.TempDir()
	for level := range maxInheritDepth + 2 {
		name := fmt.Sprintf("level%02d", level)
		inherit(t, root, name, fmt.Sprintf("level%02d", level+1))
		icon(t, root, name, "apps", name+".svg")
	}

	badges := Scan([]string{root}, "level00").Badges

	assert.Len(t, badges, maxInheritDepth+1)
	assert.Equal(t, fmt.Sprintf("level%02d", maxInheritDepth), badges[len(badges)-1])
}

func TestScanTakesInheritsFromTheFirstRootThatDescribesTheTheme(t *testing.T) {
	user, system := t.TempDir(), t.TempDir()
	inherit(t, user, "Paper", "Base")
	inherit(t, system, "Paper", "Adwaita")
	icon(t, system, "Base", "apps", "kate.svg")
	icon(t, system, "Adwaita", "apps", "firefox.svg")

	assert.Equal(t, []string{"kate"}, Scan([]string{user, system}, "Paper").Badges)
}

package icontheme

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPathsSwitchThemesAndFollowInheritance(t *testing.T) {
	root := t.TempDir()
	put := func(path, text string) string {
		path = filepath.Join(root, path)
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0755))
		require.NoError(t, os.WriteFile(path, []byte(text), 0644))
		return path
	}
	a := put("A/scalable/places/folder.svg", "<svg/>")
	b := put("B/scalable/places/folder.svg", "<svg/>")
	parent := put("Parent/48x48/apps/app.png", "png")
	fallback := put("hicolor/scalable/apps/fallback.svg", "<svg/>")
	put("A/index.theme", "[Icon Theme]\nInherits=Parent\n")
	put("Parent/index.theme", "[Icon Theme]\nInherits=A\n")
	put("A/cursors/ignored.png", "cursor")
	require.Equal(t, a, Paths([]string{root}, "A")["folder"])
	require.Equal(t, b, Paths([]string{root}, "B")["folder"])
	require.Equal(t, parent, Paths([]string{root}, "A")["app"])
	require.Equal(t, fallback, Paths([]string{root}, "A")["fallback"])
	require.NotContains(t, Paths([]string{root}, "A"), "ignored")
}

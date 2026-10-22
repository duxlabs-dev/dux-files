package games

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func home(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	t.Setenv("XDG_DATA_HOME", filepath.Join(root, ".local", "share"))
	return root
}

func mkdir(t *testing.T, path string) string {
	t.Helper()
	require.NoError(t, os.MkdirAll(path, 0o755))
	return path
}

func write(t *testing.T, path, body string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(body), 0o644))
}

func keys(groups []Group) []string {
	out := make([]string, 0, len(groups))
	for _, group := range groups {
		out = append(out, group.Key)
	}
	return out
}

func TestDiscoverSkipsEmptyInstalls(t *testing.T) {
	root := home(t)
	mkdir(t, filepath.Join(root, ".local", "share", "Steam", "steamapps", "common"))
	mkdir(t, filepath.Join(root, ".config", "heroic", "GamesConfig"))
	mkdir(t, filepath.Join(root, ".local", "share", "lutris", "games"))
	mkdir(t, filepath.Join(root, "Games"))

	assert.Empty(t, Discover(root), "installed launchers without a game offer nothing")
}

func TestDiscoverSteamLibrary(t *testing.T) {
	root := home(t)
	library := filepath.Join(root, ".local", "share", "Steam")
	mkdir(t, filepath.Join(library, "steamapps", "common", "Supreme"))
	mkdir(t, filepath.Join(library, "steamapps", "compatdata", "1091500"))
	write(t, filepath.Join(library, "steamapps", "libraryfolders.vdf"), `
"libraryfolders"
{
	"0" { "path"		"`+library+`" }
	"1" { "path"		"/mnt/games/SteamLibrary" }
}`)

	groups := Discover(root)

	require.Equal(t, []string{"steam"}, keys(groups))
	assert.Equal(t, []Place{
		{Path: filepath.Join(library, "steamapps", "common"), Kind: KindLibrary, Detail: "Steam"},
		{Path: filepath.Join(library, "steamapps", "compatdata"), Kind: KindProton, Detail: "Steam"},
		{Path: "/mnt/games/SteamLibrary/steamapps/common", Kind: KindLibrary, Detail: "SteamLibrary", Missing: true},
	}, groups[0].Places)
}

func TestDiscoverHeroicPrefixes(t *testing.T) {
	root := home(t)
	named := mkdir(t, filepath.Join(root, "Games", "Heroic", "Prefixes", "Sugar"))
	untitled := mkdir(t, filepath.Join(root, "Games", "Heroic", "Prefixes", "Rocket League"))
	write(t, filepath.Join(root, ".config", "heroic", "GamesConfig", "sugar.json"),
		`{"sugar":{"title":"Sugar","winePrefix":"`+named+`"},"rocket":{"winePrefix":"`+untitled+`"},"gone":{"winePrefix":"/nope/missing"},"version":"v0","explicit":true}`)

	groups := Discover(root)

	require.Equal(t, []string{"heroic"}, keys(groups))
	assert.Equal(t, []Place{
		{Path: filepath.Join(root, ".config", "heroic"), Kind: KindConfig},
		{Path: untitled, Kind: KindPrefix, Detail: "Rocket League"},
		{Path: named, Kind: KindPrefix, Detail: "Sugar"},
		{Path: filepath.Join(root, "Games", "Heroic"), Kind: KindGames},
	}, groups[0].Places)
}

func TestDiscoverLutrisReadsDataHome(t *testing.T) {
	root := home(t)
	prefix := mkdir(t, filepath.Join(root, "Games", "pes"))
	write(t, filepath.Join(root, ".local", "share", "lutris", "games", "sp-football.yml"),
		"game:\n  exe: game.exe\n  prefix: "+prefix+"\nname: pes\n")

	groups := Discover(root)

	require.Equal(t, []string{"lutris"}, keys(groups))
	assert.Equal(t, []Place{
		{Path: filepath.Join(root, ".local", "share", "lutris"), Kind: KindConfig},
		{Path: prefix, Kind: KindPrefix, Detail: "pes"},
		{Path: filepath.Join(root, "Games"), Kind: KindGames},
	}, groups[0].Places)
}

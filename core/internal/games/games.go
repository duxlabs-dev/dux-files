package games

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/duxlabs-dev/dux-go/ipc"
)

const (
	KindLibrary = "library"
	KindProton  = "proton"
	KindConfig  = "config"
	KindPrefix  = "prefix"
	KindGames   = "games"
)

type Place struct {
	Path    string `json:"path"`
	Kind    string `json:"kind"`
	Detail  string `json:"detail,omitempty"`
	Missing bool   `json:"missing,omitempty"`
}

type Group struct {
	Key    string  `json:"key"`
	Places []Place `json:"places"`
}

// Discover returns the launchers that actually have a game on disk, so an empty or absent install offers nothing.
func Discover(home string) []Group {
	groups := make([]Group, 0, 3)
	for _, group := range []Group{steamGroup(home), heroicGroup(home), lutrisGroup(home)} {
		if len(group.Places) == 0 {
			continue
		}
		groups = append(groups, group)
	}
	return groups
}

func steamGroup(home string) Group {
	var places []Place
	for _, root := range steamRoots(home) {
		if !isDir(root) {
			continue
		}
		for _, library := range steamLibraries(root) {
			name := libraryName(library)
			common := filepath.Join(library, "steamapps", "common")
			switch {
			case hasEntries(common):
				places = append(places, Place{Path: common, Kind: KindLibrary, Detail: name})
				if prefixes := filepath.Join(library, "steamapps", "compatdata"); hasEntries(prefixes) {
					places = append(places, Place{Path: prefixes, Kind: KindProton, Detail: name})
				}
			case library != root && !isDir(common):
				places = append(places, Place{Path: common, Kind: KindLibrary, Detail: name, Missing: true})
			}
		}
	}
	return finish("steam", places)
}

func heroicGroup(home string) Group {
	var places []Place
	fallback := filepath.Join(home, "Games", "Heroic")
	hasGames := hasEntries(fallback)
	for _, root := range heroicRoots(home) {
		if !isDir(root) {
			continue
		}
		games := heroicGames(root)
		if len(games) == 0 {
			continue
		}
		hasGames = true
		places = append(places, Place{Path: root, Kind: KindConfig})
		places = append(places, games...)
	}
	if !hasGames {
		return Group{}
	}
	if isDir(fallback) {
		places = append(places, Place{Path: fallback, Kind: KindGames})
	}
	return finish("heroic", places)
}

func lutrisGroup(home string) Group {
	var places []Place
	for _, root := range lutrisRoots(home) {
		if !isDir(root) {
			continue
		}
		games := lutrisGames(root)
		if len(games) == 0 {
			continue
		}
		places = append(places, Place{Path: root, Kind: KindConfig})
		places = append(places, games...)
	}
	if len(places) == 0 {
		return Group{}
	}
	if folder := filepath.Join(home, "Games"); isDir(folder) {
		places = append(places, Place{Path: folder, Kind: KindGames})
	}
	return finish("lutris", places)
}

type heroicConfig struct {
	Title      string `json:"title"`
	WinePrefix string `json:"winePrefix"`
}

func heroicGames(root string) []Place {
	configDir := filepath.Join(root, "GamesConfig")
	entries, err := os.ReadDir(configDir)
	if err != nil {
		return nil
	}
	var places []Place
	seen := make(map[string]bool)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(configDir, entry.Name()))
		if err != nil {
			continue
		}
		var configs map[string]json.RawMessage
		if err := json.Unmarshal(data, &configs); err != nil {
			continue
		}
		for _, raw := range configs {
			var config heroicConfig
			if err := json.Unmarshal(raw, &config); err != nil {
				continue
			}
			if config.WinePrefix == "" || seen[config.WinePrefix] || !isDir(config.WinePrefix) {
				continue
			}
			seen[config.WinePrefix] = true
			title := config.Title
			if title == "" {
				title = filepath.Base(filepath.Clean(config.WinePrefix))
			}
			places = append(places, Place{Path: config.WinePrefix, Kind: KindPrefix, Detail: title})
		}
	}
	sortPlaces(places)
	return places
}

var (
	lutrisPrefix = regexp.MustCompile(`(?m)^\s*prefix:\s*(.+)$`)
	lutrisName   = regexp.MustCompile(`(?m)^name:\s*(.+)$`)
)

func lutrisGames(root string) []Place {
	entries, err := os.ReadDir(filepath.Join(root, "games"))
	if err != nil {
		return nil
	}
	var places []Place
	seen := make(map[string]bool)
	for _, entry := range entries {
		name := entry.Name()
		extension := filepath.Ext(name)
		if entry.IsDir() || (extension != ".yml" && extension != ".yaml") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(root, "games", name))
		if err != nil {
			continue
		}
		match := lutrisPrefix.FindStringSubmatch(string(data))
		if match == nil {
			continue
		}
		prefix := unquote(match[1])
		if !filepath.IsAbs(prefix) || seen[prefix] || !isDir(prefix) {
			continue
		}
		seen[prefix] = true
		title := strings.TrimSuffix(name, extension)
		if match := lutrisName.FindStringSubmatch(string(data)); match != nil {
			if named := unquote(match[1]); named != "" {
				title = named
			}
		}
		places = append(places, Place{Path: prefix, Kind: KindPrefix, Detail: title})
	}
	sortPlaces(places)
	return places
}

func unquote(value string) string {
	return strings.Trim(strings.TrimSpace(value), `"'`)
}

func sortPlaces(places []Place) {
	sort.Slice(places, func(i, j int) bool {
		return strings.ToLower(places[i].Detail) < strings.ToLower(places[j].Detail)
	})
}

// finish drops repeated paths, the flatpak and the native install often share one.
func finish(key string, places []Place) Group {
	group := Group{Key: key, Places: make([]Place, 0, len(places))}
	seen := make(map[string]bool, len(places))
	for _, place := range places {
		resolved := place.Path
		if path, err := filepath.EvalSymlinks(place.Path); err == nil {
			resolved = path
		}
		if seen[resolved] {
			continue
		}
		seen[resolved] = true
		group.Places = append(group.Places, place)
	}
	return group
}

var steamLibraryPath = regexp.MustCompile(`"path"\s+"((?:\\.|[^"\\])*)"`)

func steamLibraries(root string) []string {
	libraries := []string{root}
	data, err := os.ReadFile(filepath.Join(root, "steamapps", "libraryfolders.vdf"))
	if err != nil {
		return libraries
	}
	unescape := strings.NewReplacer(`\\`, `\`, `\"`, `"`)
	for _, match := range steamLibraryPath.FindAllStringSubmatch(string(data), -1) {
		if path := unescape.Replace(match[1]); filepath.IsAbs(path) {
			libraries = append(libraries, path)
		}
	}
	return libraries
}

func steamRoots(home string) []string {
	return []string{
		filepath.Join(dataHome(home), "Steam"),
		filepath.Join(home, ".steam", "steam"),
		filepath.Join(home, ".var", "app", "com.valvesoftware.Steam", ".local", "share", "Steam"),
	}
}

func heroicRoots(home string) []string {
	return []string{
		filepath.Join(home, ".config", "heroic"),
		filepath.Join(home, ".var", "app", "com.heroicgameslauncher.hgl", "config", "heroic"),
	}
}

func lutrisRoots(home string) []string {
	return []string{
		filepath.Join(home, ".config", "lutris"),
		filepath.Join(dataHome(home), "lutris"),
		filepath.Join(home, ".var", "app", "net.lutris.Lutris", "config", "lutris"),
		filepath.Join(home, ".var", "app", "net.lutris.Lutris", "data", "lutris"),
	}
}

func dataHome(home string) string {
	if dir := os.Getenv("XDG_DATA_HOME"); dir != "" {
		return dir
	}
	return filepath.Join(home, ".local", "share")
}

func libraryName(path string) string {
	switch name := filepath.Base(filepath.Clean(path)); name {
	case "steam":
		return "Steam"
	case ".", "/":
		return path
	default:
		return name
	}
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func hasEntries(path string) bool {
	entries, err := os.ReadDir(path)
	return err == nil && len(entries) > 0
}

func Handles(method string) bool {
	return strings.HasPrefix(method, "games.")
}

func Handle(w *ipc.ConnWriter, req ipc.Request) {
	if req.Method != "games.list" {
		w.RespondError(req.ID, "unknown games method: "+req.Method)
		return
	}
	home, err := os.UserHomeDir()
	if err != nil {
		w.RespondError(req.ID, err.Error())
		return
	}
	w.Respond(req.ID, map[string]any{"groups": Discover(home)})
}

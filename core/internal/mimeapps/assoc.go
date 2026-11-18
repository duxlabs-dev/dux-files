package mimeapps

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/duxlabs-dev/dux-files/core/internal/ini"

	"github.com/duxlabs-dev/dux-files/core/internal/xdg"

	"github.com/duxlabs-dev/dux-files/core/internal/atomicfile"
)

const (
	groupDefaults = "Default Applications"
	groupAdded    = "Added Associations"
	groupRemoved  = "Removed Associations"
	groupCache    = "MIME Cache"
)

type assocFile struct {
	path     string
	defaults map[string][]string
	added    map[string][]string
	removed  map[string][]string
}

func assocPaths() []string {
	desktops := currentDesktops()
	var paths []string

	add := func(dir string) {
		for _, desktop := range desktops {
			paths = append(paths, filepath.Join(dir, desktop+"-mimeapps.list"))
		}
		paths = append(paths, filepath.Join(dir, "mimeapps.list"))
	}

	add(xdg.ConfigHome())
	for _, dir := range xdg.ConfigDirs() {
		add(dir)
	}
	add(filepath.Join(xdg.DataHome(), "applications"))
	for _, dir := range xdg.DataDirs() {
		add(filepath.Join(dir, "applications"))
	}
	return paths
}

func currentDesktops() []string {
	raw := os.Getenv("XDG_CURRENT_DESKTOP")
	if raw == "" {
		return nil
	}
	var out []string
	for _, name := range strings.Split(raw, ":") {
		if name = strings.ToLower(strings.TrimSpace(name)); name != "" {
			out = append(out, name)
		}
	}
	return out
}

func writePath() string { return filepath.Join(xdg.ConfigHome(), "mimeapps.list") }

func loadAssocFiles() []assocFile {
	var files []assocFile
	for _, path := range assocPaths() {
		groups := readGroups(path)
		if groups == nil {
			continue
		}
		files = append(files, assocFile{
			path:     path,
			defaults: listGroup(groups[groupDefaults]),
			added:    listGroup(groups[groupAdded]),
			removed:  listGroup(groups[groupRemoved]),
		})
	}
	return files
}

func listGroup(g *iniGroup) map[string][]string {
	out := map[string][]string{}
	if g == nil {
		return out
	}
	for key, value := range g.keys {
		out[key] = splitList(value)
	}
	return out
}

func loadMimeinfoCaches() map[string][]string {
	out := map[string][]string{}
	for _, dir := range ApplicationDirs() {
		groups := readGroups(filepath.Join(dir, "mimeinfo.cache"))
		if groups[groupCache] == nil {
			continue
		}
		for mimeType, value := range groups[groupCache].keys {
			out[mimeType] = append(out[mimeType], splitList(value)...)
		}
	}
	return out
}

func (s *Service) Default(mimeType string) string {
	candidates := s.Associations(mimeType)
	if len(candidates) == 0 {
		return ""
	}
	return candidates[0].ID
}

func removedBefore(files []assocFile, mimeType, id string) bool {
	for _, file := range files {
		if slices.Contains(file.removed[mimeType], id) {
			return true
		}
	}
	return false
}

func (s *Service) Associations(mimeType string) []*Entry {
	files := loadAssocFiles()
	cache := loadMimeinfoCaches()

	seen := map[string]bool{}
	var out []*Entry

	add := func(m, id string, blockers []assocFile) {
		if id == "" || seen[id] || removedBefore(blockers, m, id) {
			return
		}
		entry := s.entries.byIdentifier(id)
		if !entry.Launchable() {
			return
		}
		seen[id] = true
		out = append(out, entry)
	}

	for _, m := range s.detector.db.Chain(mimeType) {
		for i, file := range files {
			for _, id := range file.defaults[m] {
				add(m, id, files[:i])
			}
		}
		for i, file := range files {
			for _, id := range file.added[m] {
				add(m, id, files[:i])
			}
		}
		for _, id := range cache[m] {
			add(m, id, files)
		}
		for _, entry := range s.entries.all() {
			if !claimsMime(entry, m, s.detector.db) {
				continue
			}
			add(m, entry.ID, files)
		}
	}
	return out
}

func claimsMime(entry *Entry, mimeType string, db *DB) bool {
	for _, claimed := range entry.MimeTypes {
		if db.Canonical(claimed) == mimeType {
			return true
		}
	}
	return false
}

var writeMu sync.Mutex

func (s *Service) SetDefault(mimeType, desktopID string) error {
	if !safeIniField(mimeType) || !safeIniField(desktopID) {
		return fmt.Errorf("invalid mime type or desktop id")
	}
	if !s.entries.byIdentifier(desktopID).Launchable() {
		return fmt.Errorf("desktop entry %q is not launchable", desktopID)
	}

	writeMu.Lock()
	defer writeMu.Unlock()

	path := writePath()
	original, _ := os.ReadFile(path)
	canonical := s.detector.db.Canonical(mimeType)
	updated := setGroupLine(original, groupDefaults, canonical, desktopID)
	updated = setGroupLine(updated, groupAdded, canonical, addedLine(updated, canonical, desktopID))
	return atomicfile.WriteFileMkdir(path, updated, 0o644, 0o755)
}

func safeIniField(s string) bool {
	return s != "" && !strings.ContainsAny(s, "\n\r[]=")
}

func setGroupLine(original []byte, group, key, value string) []byte {
	line := key + "=" + value
	lines := strings.Split(strings.TrimRight(string(original), "\n"), "\n")
	if len(lines) == 1 && lines[0] == "" {
		lines = nil
	}

	start, end := -1, -1
	for i, raw := range lines {
		name, ok := ini.Header(raw)
		switch {
		case !ok:
		case name == group:
			start = i
		case start >= 0 && end < 0:
			end = i
		}
	}

	if start < 0 {
		if len(lines) > 0 {
			lines = append(lines, "")
		}
		return joinLines(append(lines, "["+group+"]", line))
	}
	if end < 0 {
		end = len(lines)
	}

	for i := start + 1; i < end; i++ {
		if ini.Key(lines[i]) == key {
			lines[i] = line
			return joinLines(lines)
		}
	}

	insert := end
	for insert > start+1 && strings.TrimSpace(lines[insert-1]) == "" {
		insert--
	}
	return joinLines(slices.Insert(lines, insert, line))
}

func joinLines(lines []string) []byte {
	return []byte(strings.Join(lines, "\n") + "\n")
}

func writableConfig() bool {
	path := writePath()
	if f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0o644); err == nil {
		f.Close()
		return true
	}
	probe, err := os.CreateTemp(filepath.Dir(path), ".mimeapps-probe-*")
	if err != nil {
		return false
	}
	probe.Close()
	os.Remove(probe.Name())
	return true
}

package mimeapps

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type Entry struct {
	ID            string   `json:"id"`
	Path          string   `json:"-"`
	Name          string   `json:"name"`
	GenericName   string   `json:"genericName,omitempty"`
	Comment       string   `json:"comment,omitempty"`
	Icon          string   `json:"icon,omitempty"`
	Exec          string   `json:"-"`
	TryExec       string   `json:"-"`
	WorkingDir    string   `json:"-"`
	MimeTypes     []string `json:"-"`
	Categories    []string `json:"-"`
	Keywords      []string `json:"-"`
	Terminal      bool     `json:"terminal"`
	NoDisplay     bool     `json:"noDisplay"`
	Hidden        bool     `json:"-"`
	StartupNotify bool     `json:"-"`
}

func (e *Entry) Launchable() bool {
	if e == nil || e.Hidden || e.Exec == "" {
		return false
	}
	if e.TryExec == "" {
		return true
	}
	if strings.ContainsRune(e.TryExec, os.PathSeparator) {
		info, err := os.Stat(e.TryExec)
		return err == nil && info.Mode()&0o111 != 0
	}
	_, err := exec.LookPath(e.TryExec)
	return err == nil
}

type parsedEntry struct {
	entry   *Entry
	modTime time.Time
	size    int64
}

type entryIndex struct {
	mu       sync.Mutex
	parsed   map[string]parsedEntry
	byID     map[string]*Entry
	ordered  []*Entry
	dirStamp string
}

func newEntryIndex() *entryIndex {
	return &entryIndex{parsed: map[string]parsedEntry{}, byID: map[string]*Entry{}}
}

func ApplicationDirs() []string {
	return appendXDG(nil, "applications")
}

func (x *entryIndex) all() []*Entry {
	x.mu.Lock()
	defer x.mu.Unlock()
	x.refresh()
	return x.ordered
}

func (x *entryIndex) byIdentifier(id string) *Entry {
	x.mu.Lock()
	defer x.mu.Unlock()
	x.refresh()
	return x.byID[id]
}

// An in-place edit of a .desktop file leaves the directory mtime alone and is not seen until the next rename there.
func (x *entryIndex) refresh() {
	dirs := ApplicationDirs()
	stamp := dirStamp(dirs)
	if stamp == x.dirStamp {
		return
	}

	byID := map[string]*Entry{}
	var ordered []*Entry
	for _, dir := range dirs {
		for _, path := range desktopFiles(dir) {
			id := entryID(dir, path)
			if _, ok := byID[id]; ok {
				continue
			}
			entry := x.parse(path, id)
			if entry == nil {
				continue
			}
			byID[id] = entry
			ordered = append(ordered, entry)
		}
	}

	x.byID = byID
	x.ordered = ordered
	x.dirStamp = stamp
}

func (x *entryIndex) parse(path, id string) *Entry {
	info, err := os.Stat(path)
	if err != nil {
		return nil
	}
	if cached, ok := x.parsed[path]; ok && cached.modTime.Equal(info.ModTime()) && cached.size == info.Size() {
		return cached.entry
	}

	entry := parseDesktopFile(path, id)
	x.parsed[path] = parsedEntry{entry: entry, modTime: info.ModTime(), size: info.Size()}
	return entry
}

func dirStamp(dirs []string) string {
	var b strings.Builder
	for _, dir := range dirs {
		real, err := filepath.EvalSymlinks(dir)
		if err != nil {
			continue
		}
		info, err := os.Stat(real)
		if err != nil {
			continue
		}
		b.WriteString(dir)
		b.WriteByte('=')
		b.WriteString(real)
		b.WriteByte('@')
		b.WriteString(info.ModTime().UTC().Format(time.RFC3339Nano))
		b.WriteByte(';')
	}
	return b.String()
}

func desktopFiles(dir string) []string {
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return nil
	}
	var out []string
	walkDesktopFiles(dir, map[string]bool{}, &out)
	return out
}

func walkDesktopFiles(dir string, visited map[string]bool, out *[]string) {
	real, err := filepath.EvalSymlinks(dir)
	if err != nil || visited[real] {
		return
	}
	visited[real] = true

	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, d := range entries {
		path := filepath.Join(dir, d.Name())
		if d.IsDir() {
			walkDesktopFiles(path, visited, out)
			continue
		}
		if d.Type()&os.ModeSymlink != 0 {
			info, err := os.Stat(path)
			if err != nil {
				continue
			}
			if info.IsDir() {
				walkDesktopFiles(path, visited, out)
				continue
			}
		}
		if strings.HasSuffix(d.Name(), ".desktop") {
			*out = append(*out, path)
		}
	}
}

func entryID(root, path string) string {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return filepath.Base(path)
	}
	return strings.ReplaceAll(rel, string(filepath.Separator), "-")
}

func parseDesktopFile(path, id string) *Entry {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	group := parseGroups(data)["Desktop Entry"]
	if group == nil {
		return nil
	}
	if kind := group.value("Type"); kind != "" && kind != "Application" {
		return nil
	}

	return &Entry{
		ID:            id,
		Path:          path,
		Name:          group.localized("Name"),
		GenericName:   group.localized("GenericName"),
		Comment:       group.localized("Comment"),
		Icon:          group.value("Icon"),
		Exec:          group.value("Exec"),
		TryExec:       group.value("TryExec"),
		WorkingDir:    group.value("Path"),
		MimeTypes:     splitList(group.value("MimeType")),
		Categories:    splitList(group.value("Categories")),
		Keywords:      splitList(group.localized("Keywords")),
		Terminal:      group.value("Terminal") == "true",
		NoDisplay:     group.value("NoDisplay") == "true",
		Hidden:        group.value("Hidden") == "true",
		StartupNotify: group.value("StartupNotify") == "true",
	}
}

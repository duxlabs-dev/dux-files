// Package recent edits $XDG_DATA_HOME/recently-used.xbel, the list GTK's GtkRecentManager keeps.
package recent

import (
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/duxlabs-dev/dux-files/core/internal/fileuri"

	"github.com/duxlabs-dev/dux-files/core/internal/atomicfile"
)

const (
	FileName    = "recently-used.xbel"
	timeLayout  = "2006-01-02T15:04:05.000000Z07:00"
	nsBookmark  = "http://www.freedesktop.org/standards/desktop-bookmarks"
	nsMime      = "http://www.freedesktop.org/standards/shared-mime-info"
	ownerDesk   = "http://freedesktop.org"
	defaultMode = os.FileMode(0o600)
)

type Application struct {
	Name     string
	Exec     string
	Modified time.Time
	Count    int
}

type Icon struct {
	Href string
	Type string
}

type Item struct {
	URI      string
	Added    time.Time
	Modified time.Time
	Visited  time.Time
	Title    string
	Desc     string
	MimeType string
	Private  bool
	Groups   []string
	Icon     Icon
	Apps     []Application
}

func (i Item) Used() time.Time {
	used := i.Modified
	for _, when := range []time.Time{i.Visited, i.Added} {
		if when.After(used) {
			used = when
		}
	}
	return used
}

func (i Item) Path() (string, bool) { return PathOf(i.URI) }

type Store struct {
	Path string
	mu   sync.Mutex
}

func NewStore(dataHome string) *Store {
	return &Store{Path: filepath.Join(dataHome, FileName)}
}

func (s *Store) Stamp() string {
	info, err := os.Stat(s.Path)
	if err != nil {
		return ""
	}
	return strconv.FormatInt(info.ModTime().UnixNano(), 10) + ":" + strconv.FormatInt(info.Size(), 10)
}

func (s *Store) Load() ([]Item, string, error) {
	stamp := s.Stamp()
	data, err := os.ReadFile(s.Path)
	switch {
	case os.IsNotExist(err):
		return nil, "", nil
	case err != nil:
		return nil, "", err
	}
	items, err := Parse(data)
	if err != nil {
		return nil, "", err
	}
	return items, stamp, nil
}

type Use struct {
	Path     string
	MimeType string
}

func (s *Store) Add(path, mimeType, appName, appExec string, now time.Time) error {
	return s.Record([]Use{{Path: path, MimeType: mimeType}}, appName, appExec, now)
}

func (s *Store) Record(uses []Use, appName, appExec string, now time.Time) error {
	if len(uses) == 0 {
		return nil
	}
	return s.edit(func(items []Item) []Item {
		for _, use := range uses {
			items = add(items, use.Path, use.MimeType, Application{Name: appName, Exec: appExec, Modified: now, Count: 1}, now)
		}
		return items
	})
}

func (s *Store) Remove(paths []string) error {
	drop := map[string]bool{}
	for _, path := range paths {
		drop[filepath.Clean(path)] = true
	}
	return s.edit(func(items []Item) []Item {
		kept := make([]Item, 0, len(items))
		for _, item := range items {
			if path, ok := item.Path(); ok && drop[path] {
				continue
			}
			kept = append(kept, item)
		}
		return kept
	})
}

func (s *Store) Clear() error {
	return s.edit(func([]Item) []Item { return nil })
}

func (s *Store) edit(change func([]Item) []Item) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	items, _, err := s.Load()
	if err != nil {
		return err
	}
	mode := defaultMode
	if info, err := os.Stat(s.Path); err == nil {
		mode = info.Mode().Perm()
	}
	return atomicfile.WriteFileMkdir(s.Path, Marshal(change(items)), mode, 0o700)
}

func add(items []Item, path, mimeType string, app Application, now time.Time) []Item {
	path = filepath.Clean(path)
	for i := range items {
		existing, ok := items[i].Path()
		if !ok || existing != path {
			continue
		}
		items[i].Modified = now
		if mimeType != "" {
			items[i].MimeType = mimeType
		}
		items[i].Apps = register(items[i].Apps, app)
		return items
	}
	return append(items, Item{URI: FileURI(path), Added: now, Modified: now, Visited: now, MimeType: mimeType, Apps: []Application{app}})
}

func register(apps []Application, app Application) []Application {
	for i := range apps {
		if apps[i].Name != app.Name {
			continue
		}
		apps[i].Exec = app.Exec
		apps[i].Modified = app.Modified
		apps[i].Count++
		return apps
	}
	return append(apps, app)
}

const uriKept = "!$&'()*+,-./:=@_~"

func FileURI(path string) string {
	var out strings.Builder
	out.WriteString("file://")
	for _, b := range []byte(path) {
		switch {
		case b >= 'a' && b <= 'z', b >= 'A' && b <= 'Z', b >= '0' && b <= '9', strings.IndexByte(uriKept, b) >= 0:
			out.WriteByte(b)
		default:
			fmt.Fprintf(&out, "%%%02X", b)
		}
	}
	return out.String()
}

func PathOf(uri string) (string, bool) {
	return fileuri.Parse(uri)
}

type xbel struct {
	Bookmarks []bookmark `xml:"bookmark"`
}

type bookmark struct {
	Href     string `xml:"href,attr"`
	Added    string `xml:"added,attr"`
	Modified string `xml:"modified,attr"`
	Visited  string `xml:"visited,attr"`
	Title    string `xml:"title"`
	Desc     string `xml:"desc"`
	Info     struct {
		Metadata []metadata `xml:"metadata"`
	} `xml:"info"`
}

type metadata struct {
	Owner    string `xml:"owner,attr"`
	MimeType *struct {
		Type string `xml:"type,attr"`
	} `xml:"mime-type"`
	Groups *struct {
		Group []string `xml:"group"`
	} `xml:"groups"`
	Applications *struct {
		Application []application `xml:"application"`
	} `xml:"applications"`
	Private *struct{} `xml:"private"`
	Icon    *struct {
		Href string `xml:"href,attr"`
		Type string `xml:"type,attr"`
	} `xml:"icon"`
}

type application struct {
	Name     string `xml:"name,attr"`
	Exec     string `xml:"exec,attr"`
	Modified string `xml:"modified,attr"`
	Count    int    `xml:"count,attr"`
}

func Parse(data []byte) ([]Item, error) {
	if len(strings.TrimSpace(string(data))) == 0 {
		return nil, nil
	}
	var root xbel
	if err := xml.Unmarshal(data, &root); err != nil {
		return nil, fmt.Errorf("%s: %w", FileName, err)
	}
	items := make([]Item, 0, len(root.Bookmarks))
	for _, b := range root.Bookmarks {
		if b.Href == "" {
			continue
		}
		item := Item{URI: b.Href, Added: parseTime(b.Added), Modified: parseTime(b.Modified), Visited: parseTime(b.Visited), Title: b.Title, Desc: b.Desc}
		for _, meta := range b.Info.Metadata {
			if meta.Owner != ownerDesk {
				continue
			}
			item.applyMetadata(meta)
		}
		items = append(items, item)
	}
	return items, nil
}

func (i *Item) applyMetadata(meta metadata) {
	if meta.MimeType != nil {
		i.MimeType = meta.MimeType.Type
	}
	if meta.Groups != nil {
		i.Groups = meta.Groups.Group
	}
	if meta.Applications != nil {
		for _, app := range meta.Applications.Application {
			i.Apps = append(i.Apps, Application{Name: app.Name, Exec: app.Exec, Modified: parseTime(app.Modified), Count: max(app.Count, 1)})
		}
	}
	i.Private = meta.Private != nil
	if meta.Icon != nil {
		i.Icon = Icon{Href: meta.Icon.Href, Type: meta.Icon.Type}
	}
}

func parseTime(text string) time.Time {
	when, err := time.Parse(time.RFC3339Nano, text)
	if err != nil {
		return time.Time{}
	}
	return when
}

func formatTime(when time.Time) string {
	if when.IsZero() {
		when = time.Now()
	}
	return when.UTC().Format(timeLayout)
}

func Marshal(items []Item) []byte {
	var out strings.Builder
	out.WriteString("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<xbel version=\"1.0\"\n      xmlns:bookmark=\"" + nsBookmark + "\"\n      xmlns:mime=\"" + nsMime + "\"\n>\n")
	for _, item := range items {
		writeItem(&out, item)
	}
	out.WriteString("</xbel>")
	return []byte(out.String())
}

func writeItem(out *strings.Builder, item Item) {
	fmt.Fprintf(out, "  <bookmark href=\"%s\" added=\"%s\" modified=\"%s\" visited=\"%s\">\n", escape(item.URI), formatTime(item.Added), formatTime(item.Modified), formatTime(item.Visited))
	if item.Title != "" {
		fmt.Fprintf(out, "    <title>%s</title>\n", escape(item.Title))
	}
	if item.Desc != "" {
		fmt.Fprintf(out, "    <desc>%s</desc>\n", escape(item.Desc))
	}
	out.WriteString("    <info>\n      <metadata owner=\"" + ownerDesk + "\">\n")
	if item.MimeType != "" {
		fmt.Fprintf(out, "        <mime:mime-type type=\"%s\"/>\n", escape(item.MimeType))
	}
	if len(item.Groups) > 0 {
		out.WriteString("        <bookmark:groups>\n")
		for _, group := range item.Groups {
			fmt.Fprintf(out, "          <bookmark:group>%s</bookmark:group>\n", escape(group))
		}
		out.WriteString("        </bookmark:groups>\n")
	}
	if len(item.Apps) > 0 {
		out.WriteString("        <bookmark:applications>\n")
		for _, app := range item.Apps {
			fmt.Fprintf(out, "          <bookmark:application name=\"%s\" exec=\"%s\" modified=\"%s\" count=\"%d\"/>\n", escape(app.Name), escape(app.Exec), formatTime(app.Modified), app.Count)
		}
		out.WriteString("        </bookmark:applications>\n")
	}
	if item.Private {
		out.WriteString("        <bookmark:private/>\n")
	}
	if item.Icon.Href != "" {
		fmt.Fprintf(out, "        <bookmark:icon href=\"%s\" type=\"%s\"/>\n", escape(item.Icon.Href), escape(item.Icon.Type))
	}
	out.WriteString("      </metadata>\n    </info>\n  </bookmark>\n")
}

func escape(text string) string {
	var out strings.Builder
	_ = xml.EscapeText(&out, []byte(text))
	return out.String()
}

func SortByUsed(items []Item) {
	used := make([]time.Time, len(items))
	for i := range items {
		used[i] = items[i].Used()
	}
	sort.Stable(byUsed{items: items, used: used})
}

type byUsed struct {
	items []Item
	used  []time.Time
}

func (b byUsed) Len() int           { return len(b.items) }
func (b byUsed) Less(i, j int) bool { return b.used[i].After(b.used[j]) }
func (b byUsed) Swap(i, j int) {
	b.items[i], b.items[j] = b.items[j], b.items[i]
	b.used[i], b.used[j] = b.used[j], b.used[i]
}

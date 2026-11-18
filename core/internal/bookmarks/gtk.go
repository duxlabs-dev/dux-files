package bookmarks

import (
	"os"
	"strings"

	"github.com/duxlabs-dev/dux-files/core/internal/fileuri"
)

// One `file:///path Label` per line, the label optional, the path percent-encoded.
func ParseGTK(data []byte) []Bookmark {
	var out []Bookmark
	for line := range strings.SplitSeq(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		uri, label, _ := strings.Cut(line, " ")
		path, ok := fileuri.Parse(uri)
		if !ok {
			continue
		}
		out = append(out, Bookmark{Path: path, Label: strings.TrimSpace(label)})
	}
	return out
}

func (s *Store) GTKImportAvailable() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.gtkImport
}

func (s *Store) gtkImportAvailable() bool {
	if s.gtkFile == "" || s.Exists() {
		return false
	}
	data, err := os.ReadFile(s.gtkFile)
	if err != nil {
		return false
	}
	return len(ParseGTK(data)) > 0
}

func (s *Store) ImportGTK() ([]Bookmark, int, error) {
	data, err := os.ReadFile(s.gtkFile)
	if err != nil {
		return nil, 0, err
	}
	added := 0
	items, err := s.update(func(items []Bookmark) ([]Bookmark, error) {
		for _, b := range ParseGTK(data) {
			if indexOf(items, b.Path) >= 0 {
				continue
			}
			if b.Label == "" {
				b.Label = labelOf(b.Path)
			}
			items = append(items, b)
			added++
		}
		return items, nil
	})
	return items, added, err
}

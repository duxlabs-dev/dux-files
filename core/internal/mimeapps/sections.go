package mimeapps

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/duxlabs-dev/dux-files/core/internal/atomicfile"
)

const (
	selfDesktopID = "dev.duxlabs.files.desktop"
	recentLimit   = 5
)

type Sections struct {
	MimeTypes   []string
	DefaultID   string
	Recent      []*Entry
	Recommended []*Entry
	Other       []*Entry
}

func (s *Service) RecentFor(mimeType string) []*Entry {
	groups := readGroups(writePath())
	if groups[groupAdded] == nil {
		return nil
	}
	var out []*Entry
	for _, id := range splitList(groups[groupAdded].value(s.detector.db.Canonical(mimeType))) {
		entry := s.entries.byIdentifier(id)
		if !entry.Launchable() || containsID(out, id) {
			continue
		}
		out = append(out, entry)
	}
	return out
}

func (s *Service) RecordUse(mimeType, desktopID string) error {
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
	updated := setGroupLine(original, groupAdded, canonical, addedLine(original, canonical, desktopID))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return atomicfile.WriteFile(path, updated, 0o644)
}

// GLib's update_mimeapps_list also records a new default under Added Associations, as the spec wants a default to be associated.
func addedLine(original []byte, canonical, desktopID string) string {
	ids := []string{desktopID}
	if added := parseGroups(original)[groupAdded]; added != nil {
		for _, id := range splitList(added.value(canonical)) {
			if id != desktopID && !slices.Contains(ids, id) {
				ids = append(ids, id)
			}
		}
	}
	return strings.Join(ids, ";") + ";"
}

func (s *Service) RecordUseForPaths(paths []string, desktopID string) error {
	var seen []string
	for _, path := range paths {
		mimeType, err := s.MimeOf(path)
		if err != nil || slices.Contains(seen, mimeType) {
			continue
		}
		seen = append(seen, mimeType)
		if err := s.RecordUse(mimeType, desktopID); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) Sections(mimeTypes []string) Sections {
	var types []string
	for _, mimeType := range mimeTypes {
		canonical := s.detector.db.Canonical(StripParams(mimeType))
		if canonical != "" && !slices.Contains(types, canonical) {
			types = append(types, canonical)
		}
	}
	out := Sections{MimeTypes: types}
	if len(types) == 0 {
		return out
	}
	if preferred := s.Default(types[0]); len(types) == 1 && preferred != selfDesktopID {
		out.DefaultID = preferred
	}

	recommended := shown(s.Associations(types[0]), out.DefaultID)
	recent := shown(s.RecentFor(types[0]), out.DefaultID)
	for _, mimeType := range types[1:] {
		recommended = intersectEntries(recommended, s.Associations(mimeType))
		recent = intersectEntries(recent, s.RecentFor(mimeType))
	}

	taken := map[string]bool{selfDesktopID: true}
	for _, entry := range recent {
		if taken[entry.ID] || entry.ID == out.DefaultID || len(out.Recent) == recentLimit {
			continue
		}
		taken[entry.ID] = true
		out.Recent = append(out.Recent, entry)
	}
	out.Recommended = takeEntries(recommended, taken)
	out.Other = takeEntries(s.Apps(), taken)
	return out
}

func takeEntries(entries []*Entry, taken map[string]bool) []*Entry {
	out := []*Entry{}
	for _, entry := range entries {
		if taken[entry.ID] {
			continue
		}
		taken[entry.ID] = true
		out = append(out, entry)
	}
	return out
}

func intersectEntries(ordered, other []*Entry) []*Entry {
	var out []*Entry
	for _, entry := range ordered {
		if containsID(other, entry.ID) {
			out = append(out, entry)
		}
	}
	return out
}

func containsID(entries []*Entry, id string) bool {
	for _, entry := range entries {
		if entry.ID == id {
			return true
		}
	}
	return false
}

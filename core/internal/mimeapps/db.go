package mimeapps

import (
	"bufio"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/duxlabs-dev/dux-files/core/internal/xdg"
)

type DB struct {
	globs        *globSet
	magic        []magicRule
	maxExtent    int
	aliases      map[string]string
	subclasses   map[string][]string
	children     map[string][]string
	genericIcons map[string]string
	icons        map[string]string
}

func MimeDirs() []string {
	return appendXDG(nil, "mime")
}

func appendXDG(dirs []string, leaf string) []string {
	seen := map[string]bool{}
	add := func(dir string) {
		if dir == "" || seen[dir] {
			return
		}
		seen[dir] = true
		dirs = append(dirs, dir)
	}

	add(filepath.Join(xdg.DataHome(), leaf))
	for _, dir := range xdg.DataDirs() {
		add(filepath.Join(dir, leaf))
	}
	return dirs
}

func LoadDB(dirs []string) *DB {
	db := &DB{
		globs:        newGlobSet(),
		aliases:      map[string]string{},
		subclasses:   map[string][]string{},
		children:     map[string][]string{},
		genericIcons: map[string]string{},
		icons:        map[string]string{},
	}

	for _, dir := range dirs {
		db.globs.load(filepath.Join(dir, "globs2"))
		db.loadMagic(filepath.Join(dir, "magic"))
		readPairs(filepath.Join(dir, "aliases"), " ", func(alias, target string) {
			if _, ok := db.aliases[alias]; !ok {
				db.aliases[alias] = target
			}
		})
		readPairs(filepath.Join(dir, "subclasses"), " ", func(child, parent string) {
			db.subclasses[child] = append(db.subclasses[child], parent)
			db.children[parent] = append(db.children[parent], child)
		})
		readPairs(filepath.Join(dir, "generic-icons"), ":", func(mimeType, icon string) {
			if _, ok := db.genericIcons[mimeType]; !ok {
				db.genericIcons[mimeType] = icon
			}
		})
		readPairs(filepath.Join(dir, "icons"), ":", func(mimeType, icon string) {
			if _, ok := db.icons[mimeType]; !ok {
				db.icons[mimeType] = icon
			}
		})
	}

	db.globs.sort()
	return db
}

func readPairs(path, sep string, fn func(key, value string)) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, sep)
		if !ok {
			continue
		}
		fn(strings.TrimSpace(key), strings.TrimSpace(value))
	}
}

func (db *DB) Canonical(mimeType string) string {
	mimeType = StripParams(mimeType)
	if target, ok := db.aliases[mimeType]; ok {
		return target
	}
	return mimeType
}

func (db *DB) Chain(mimeType string) []string {
	root := db.Canonical(mimeType)
	if root == "" {
		return nil
	}

	seen := map[string]bool{root: true}
	chain := []string{root}
	for i := 0; i < len(chain); i++ {
		for _, parent := range db.subclasses[chain[i]] {
			parent = db.Canonical(parent)
			if seen[parent] {
				continue
			}
			seen[parent] = true
			chain = append(chain, parent)
		}
	}
	if strings.HasPrefix(root, "text/") && !seen[TypeText] {
		chain = append(chain, TypeText)
	}
	if !strings.HasPrefix(root, "inode/") && !seen[TypeBinary] {
		chain = append(chain, TypeBinary)
	}
	return chain
}

func (db *DB) IsSubclassOf(mimeType, ancestor string) bool {
	ancestor = db.Canonical(ancestor)
	for _, m := range db.Chain(mimeType) {
		if m == ancestor {
			return true
		}
	}
	return false
}

func (db *DB) MimeIcon(mimeType string) (specific, generic string) {
	canonical := db.Canonical(mimeType)
	return db.icons[canonical], db.genericIcons[canonical]
}

func (db *DB) Globs(mimeType string) []string {
	mimeType = db.Canonical(strings.ToLower(mimeType))
	patterns := map[string]bool{}
	visited := map[string]bool{}
	switch {
	case strings.HasSuffix(mimeType, "/*"):
		for _, t := range db.typesWithPrefix(strings.TrimSuffix(mimeType, "*")) {
			db.collectGlobs(t, patterns, visited)
		}
	default:
		db.collectGlobs(mimeType, patterns, visited)
	}
	if len(patterns) == 0 {
		return nil
	}
	return slices.Sorted(maps.Keys(patterns))
}

func (db *DB) collectGlobs(mimeType string, patterns, visited map[string]bool) {
	if visited[mimeType] {
		return
	}
	visited[mimeType] = true
	for _, pattern := range db.globs.byMime[mimeType] {
		patterns[pattern] = true
	}
	for _, child := range db.children[mimeType] {
		db.collectGlobs(child, patterns, visited)
	}
}

func (db *DB) typesWithPrefix(prefix string) []string {
	set := map[string]bool{}
	for mimeType := range db.globs.byMime {
		if strings.HasPrefix(mimeType, prefix) {
			set[mimeType] = true
		}
	}
	for alias, canonical := range db.aliases {
		if strings.HasPrefix(alias, prefix) {
			set[canonical] = true
		}
	}
	for child, parents := range db.subclasses {
		if strings.HasPrefix(child, prefix) {
			set[child] = true
		}
		for _, parent := range parents {
			if strings.HasPrefix(parent, prefix) {
				set[parent] = true
			}
		}
	}
	return slices.Sorted(maps.Keys(set))
}

func StripParams(mimeType string) string {
	if cut := strings.IndexByte(mimeType, ';'); cut >= 0 {
		mimeType = mimeType[:cut]
	}
	return strings.TrimSpace(mimeType)
}

package search

import (
	"bufio"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/duxlabs-dev/dux-files/core/internal/xdg"

	"github.com/go-git/go-billy/v5/osfs"
	"github.com/go-git/go-git/v5/plumbing/format/gitignore"
)

// Mirrors danksearch's DefaultExcludeDirs so both backends skip the same folders.
var builtinSkips = []string{
	".git", ".hg", ".svn",
	"node_modules", "bower_components", ".npm", ".yarn",
	"site-packages", "__pycache__", ".venv", "venv", ".tox", ".pytest_cache", ".eggs",
	"dist", "build", "out", "bin", "obj",
	"target", "vendor",
	".gradle", ".m2", "bundle",
	".cache", ".parcel-cache", ".next", ".nuxt", ".serverless",
	"Library", ".Trash-1000",
	".postgresql", ".postgres", ".mysql", ".mongodb", ".redis", "pgdata", "pg_data",
	"go", ".cargo", ".pyenv", ".rbenv", ".nvm", ".rustup", ".composer", ".gem",
	".idea", ".vscode",
}

var ignoreFiles = []string{".gitignore", ".ignore"}

type ignorePattern struct {
	pattern  gitignore.Pattern
	nameOnly bool
}

type ignoreRules struct {
	enabled   bool
	skipNames map[string]bool
	skipPaths []string
	global    []ignorePattern
}

func newIgnoreRules(excludes []string, includeIgnored bool) ignoreRules {
	if includeIgnored {
		return ignoreRules{}
	}
	rules := newSkipRules(excludes)
	rules.global = globalPatterns()
	return rules
}

func newSkipRules(excludes []string) ignoreRules {
	rules := ignoreRules{enabled: true, skipNames: make(map[string]bool, len(builtinSkips)+len(excludes))}
	for _, name := range builtinSkips {
		rules.skipNames[name] = true
	}
	home, _ := os.UserHomeDir()
	for _, raw := range excludes {
		entry := strings.TrimSpace(raw)
		switch {
		case entry == "":
		case strings.HasPrefix(entry, "~/") && home != "":
			rules.skipPaths = append(rules.skipPaths, filepath.Clean(filepath.Join(home, entry[2:])))
		case strings.HasPrefix(entry, "/"):
			rules.skipPaths = append(rules.skipPaths, filepath.Clean(entry))
		default:
			rules.skipNames[entry] = true
		}
	}
	return rules
}

func globalPatterns() []ignorePattern {
	loaded, _ := gitignore.LoadGlobalPatterns(osfs.New("/"))
	if len(loaded) > 0 {
		patterns := make([]ignorePattern, 0, len(loaded))
		for _, p := range loaded {
			patterns = append(patterns, ignorePattern{pattern: p})
		}
		return patterns
	}
	return readPatterns(filepath.Join(xdg.ConfigHome(), "git", "ignore"), nil)
}

func (r ignoreRules) skipsDir(name, full string) bool {
	if !r.enabled {
		return false
	}
	if r.skipNames[name] {
		return true
	}
	for _, path := range r.skipPaths {
		if path == full {
			return true
		}
	}
	return false
}

func (r ignoreRules) skipsPath(path, root string) bool {
	if !r.enabled {
		return false
	}
	dir := filepath.Dir(path)
	for dir != root && len(dir) > len(root) {
		if r.skipsDir(filepath.Base(dir), dir) {
			return true
		}
		dir = filepath.Dir(dir)
	}
	return false
}

func (r ignoreRules) rootPatterns(root string) []ignorePattern {
	if !r.enabled {
		return nil
	}
	patterns := r.global
	var dirs []string
	for dir := filepath.Dir(root); ; dir = filepath.Dir(dir) {
		dirs = append(dirs, dir)
		if dir == filepath.Dir(dir) {
			break
		}
	}
	for i := len(dirs) - 1; i >= 0; i-- {
		patterns = r.load(dirs[i], patterns)
	}
	return patterns
}

func (r ignoreRules) loadIf(dir string, children []os.DirEntry, inherited []ignorePattern) []ignorePattern {
	if !r.enabled {
		return nil
	}
	for _, child := range children {
		if slices.Contains(ignoreFiles, child.Name()) {
			return r.load(dir, inherited)
		}
	}
	return inherited
}

func (r ignoreRules) load(dir string, inherited []ignorePattern) []ignorePattern {
	if !r.enabled {
		return nil
	}
	domain := components(dir)
	out := inherited
	for _, file := range ignoreFiles {
		found := readPatterns(filepath.Join(dir, file), domain)
		if len(found) == 0 {
			continue
		}
		if len(out) == len(inherited) {
			out = append(inherited[:len(inherited):len(inherited)], found...)
			continue
		}
		out = append(out, found...)
	}
	return out
}

func readPatterns(path string, domain []string) []ignorePattern {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	var out []ignorePattern
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "#") || strings.TrimSpace(line) == "" {
			continue
		}
		body := strings.TrimSuffix(strings.TrimPrefix(line, "!"), "/")
		nameOnly := !strings.Contains(body, "/")
		scope := domain
		if nameOnly {
			scope = nil
		}
		out = append(out, ignorePattern{pattern: gitignore.ParsePattern(line, scope), nameOnly: nameOnly})
	}
	return out
}

func ignored(patterns []ignorePattern, full string, isDir bool) bool {
	if len(patterns) == 0 {
		return false
	}
	name := []string{filepath.Base(full)}
	var path []string
	for i := len(patterns) - 1; i >= 0; i-- {
		p := patterns[i]
		subject := name
		if !p.nameOnly {
			if path == nil {
				path = components(full)
			}
			subject = path
		}
		if match := p.pattern.Match(subject, isDir); match > gitignore.NoMatch {
			return match == gitignore.Exclude
		}
	}
	return false
}

func components(path string) []string {
	trimmed := strings.Trim(filepath.ToSlash(path), "/")
	if trimmed == "" {
		return nil
	}
	return strings.Split(trimmed, "/")
}

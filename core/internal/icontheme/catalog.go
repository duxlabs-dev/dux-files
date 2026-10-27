// Package icontheme lists the icon names a folder badge or icon can use.
package icontheme

import (
	"bufio"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// Catalog holds the icon names the appearance picker offers, sorted.
type Catalog struct {
	Badges []string `json:"badges"`
	Icons  []string `json:"icons"`
}

const (
	maxNames      = 1500
	minFolderSize = 48
	badgeDirs     = "emblems,apps"
	iconDirs      = "places"
)

// Roots lists the icon theme directories XDG points at.
func Roots(home, dataHome string, dataDirs []string) []string {
	roots := make([]string, 0, len(dataDirs)+2)
	roots = append(roots, filepath.Join(home, ".icons"))
	if dataHome != "" {
		roots = append(roots, filepath.Join(dataHome, "icons"))
	}
	for _, dir := range dataDirs {
		if dir != "" {
			roots = append(roots, filepath.Join(dir, "icons"))
		}
	}
	return roots
}

// Theme names the icon theme the desktop is set to use.
func Theme(home string) string {
	for _, candidate := range []struct{ path, section, key string }{
		{filepath.Join(home, ".config", "kdeglobals"), "Icons", "Theme"},
		{filepath.Join(home, ".config", "gtk-4.0", "settings.ini"), "Settings", "gtk-icon-theme-name"},
		{filepath.Join(home, ".config", "gtk-3.0", "settings.ini"), "Settings", "gtk-icon-theme-name"},
	} {
		if name := iniValue(candidate.path, candidate.section, candidate.key); name != "" {
			return name
		}
	}
	return ""
}

func iniValue(path, section, key string) string {
	file, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer file.Close()
	current := ""
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			current = strings.TrimSuffix(strings.TrimPrefix(line, "["), "]")
			continue
		}
		if current != section {
			continue
		}
		name, value, found := strings.Cut(line, "=")
		if found && strings.TrimSpace(name) == key {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

// Scan collects badge and folder icon names, from every installed theme when
// the configured one is unknown.
func Scan(roots []string, theme string) Catalog {
	badges := map[string]bool{}
	icons := map[string]bool{}
	themes := []string{theme}
	if theme == "" || !exists(roots, theme) {
		themes = Installed(roots)
	}
	for _, name := range themes {
		for _, root := range roots {
			dir := filepath.Join(root, name)
			if info, err := os.Stat(dir); err != nil || !info.IsDir() {
				continue
			}
			walk(dir, badgeDirs, badges, nil)
			walk(dir, iconDirs, nil, icons)
		}
	}
	return Catalog{Badges: sorted(badges), Icons: plainVariants(sorted(icons))}
}

func exists(roots []string, theme string) bool {
	for _, root := range roots {
		if info, err := os.Stat(filepath.Join(root, theme)); err == nil && info.IsDir() {
			return true
		}
	}
	return false
}

// walk adds the icon names under dir to badges or icons, whichever is non-nil.
func walk(dir, families string, badges, icons map[string]bool) {
	_ = filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if entry.IsDir() {
			if path != dir && !isFamily(strings.Split(entry.Name(), "@")[0], families) {
				return filepath.SkipDir
			}
			return nil
		}
		icon, ok := iconName(entry.Name())
		if !ok || strings.HasSuffix(icon, "-symbolic") {
			return nil
		}
		if badges != nil && len(badges) < maxNames {
			badges[icon] = true
		}
		if icons != nil && strings.HasPrefix(icon, "folder") && isLarge(path) && len(icons) < maxNames {
			icons[icon] = true
		}
		return nil
	})
}

// isLarge reports whether an icon file is scalable or at least minFolderSize,
// which tells a drawn folder from a small symbol.
func isLarge(path string) bool {
	for _, part := range strings.Split(filepath.ToSlash(path), "/") {
		size, _, _ := strings.Cut(strings.Split(part, "@")[0], "x")
		switch number, err := strconv.Atoi(size); {
		case part == "scalable":
			return true
		case err == nil && number >= minFolderSize:
			return true
		}
	}
	return false
}

// isFamily reports whether a directory holds a wanted icon family or its sizes.
func isFamily(dir, families string) bool {
	if dir == "scalable" || dir == "symbolic" {
		return true
	}
	if size, _, _ := strings.Cut(dir, "x"); size != "" {
		if _, err := strconv.Atoi(size); err == nil {
			return true
		}
	}
	for _, name := range strings.Split(families, ",") {
		if dir == name {
			return true
		}
	}
	return false
}

// plainVariants drops purpose variants like folder_color_green_documents.
func plainVariants(names []string) []string {
	plain := make([]string, 0, len(names))
	for _, name := range names {
		rest := strings.TrimPrefix(name, "folder")
		if strings.Count(rest, "-")+strings.Count(rest, "_") > 2 {
			continue
		}
		plain = append(plain, name)
	}
	return plain
}

// iconName strips the extension from an svg or png file name.
func iconName(file string) (string, bool) {
	extension := filepath.Ext(file)
	if extension != ".svg" && extension != ".png" {
		return "", false
	}
	name := strings.TrimSuffix(file, extension)
	if name == "" || strings.ContainsAny(name, "/ ") {
		return "", false
	}
	return name, true
}

func sorted(set map[string]bool) []string {
	names := make([]string, 0, len(set))
	for name := range set {
		names = append(names, name)
	}
	sort.Slice(names, func(a, b int) bool {
		left, right := strings.ToLower(names[a]), strings.ToLower(names[b])
		if left == right {
			return names[a] < names[b]
		}
		return left < right
	})
	return names
}

// Default scans the icon theme this user has configured.
func Default() Catalog {
	home, _ := os.UserHomeDir()
	return Scan(Roots(home, os.Getenv("XDG_DATA_HOME"), filepath.SplitList(os.Getenv("XDG_DATA_DIRS"))), Effective(home))
}

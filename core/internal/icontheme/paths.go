package icontheme

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Paths resolves names to files, including inherited themes and hicolor.
func Paths(roots []string, theme string) map[string]string {
	paths := map[string]string{}
	seen := map[string]bool{}
	var visit func(string)
	visit = func(name string) {
		if name == "" || seen[name] || filepath.Base(name) != name {
			return
		}
		seen[name] = true
		inherited := []string{}
		for _, root := range roots {
			dir := filepath.Join(root, name)
			resolved, err := filepath.EvalSymlinks(dir)
			if err != nil {
				continue
			}
			local := map[string]string{}
			_ = filepath.WalkDir(resolved, func(path string, entry fs.DirEntry, err error) error {
				if err != nil {
					return nil
				}
				if entry.IsDir() {
					if entry.Name() == "cursors" || entry.Name() == "cursors_scalable" {
						return filepath.SkipDir
					}
					return nil
				}
				icon, ok := iconName(entry.Name())
				if !ok {
					return nil
				}
				// ponytail: one large/scalable file per name; pass render size if raster sizing needs tuning.
				previous := local[icon]
				if previous == "" || (isLarge(path) && !isLarge(previous)) || (strings.Contains(path, "/scalable/") && !strings.Contains(previous, "/scalable/")) {
					if info, err := os.Stat(path); err == nil && info.Mode().IsRegular() {
						local[icon] = path
					}
				}
				return nil
			})
			for icon, path := range local {
				if paths[icon] == "" {
					paths[icon] = path
				}
			}
			inherited = append(inherited, strings.Split(iniValue(filepath.Join(dir, "index.theme"), "Icon Theme", "Inherits"), ",")...)
		}
		for _, parent := range inherited {
			visit(strings.TrimSpace(parent))
		}
	}
	visit(theme)
	visit("hicolor")
	return paths
}

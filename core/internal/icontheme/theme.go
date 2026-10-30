package icontheme

import (
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// The icon theme the user picked lives in its own file next to the app
// settings. The settings file is rewritten by the UI, so a value written
// there from the terminal would be dropped from it.
const (
	settingFile = "icon-theme"
	autoTheme   = "auto"
)

// Setting returns the theme the user picked, or "auto" when they left it to
// the app.
func Setting(home string) string {
	name := strings.TrimSpace(readFile(settingPath(home)))
	if name == "" {
		return autoTheme
	}
	return name
}

// Set stores the theme choice. An empty name means the app decides.
func Set(home, name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		name = autoTheme
	}
	path := settingPath(home)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(name+"\n"), 0o600)
}

// Effective returns the theme to draw with: the picked one when it names a
// theme, otherwise the one the desktop is set to use.
func Effective(home string) string {
	if picked := Setting(home); picked != autoTheme {
		return picked
	}
	return Detected(home)
}

// Detected names the icon theme the desktop shows, looking where the desktop
// tools write it.
func Detected(home string) string {
	return detected(home, gsettingsTheme)
}

func detected(home string, gsettings func() string) string {
	config := configDir(home)
	for _, candidate := range []struct{ path, section, key string }{
		{filepath.Join(config, "kdeglobals"), "Icons", "Theme"},
		{filepath.Join(config, "qt6ct", "qt6ct.conf"), "Appearance", "icon_theme"},
		{filepath.Join(config, "gtk-4.0", "settings.ini"), "Settings", "gtk-icon-theme-name"},
		{filepath.Join(config, "gtk-3.0", "settings.ini"), "Settings", "gtk-icon-theme-name"},
	} {
		if name := iniValue(candidate.path, candidate.section, candidate.key); name != "" {
			return name
		}
	}
	return gsettings()
}

// gsettingsTheme asks GNOME for the icon theme, which is where a GNOME or
// GTK desktop keeps it.
func gsettingsTheme() string {
	out, err := exec.Command("gsettings", "get", "org.gnome.desktop.interface", "icon-theme").Output()
	if err != nil {
		return ""
	}
	return strings.Trim(strings.TrimSpace(string(out)), "'")
}

// Installed lists the icon themes found in the icon roots. A theme holds icons
// when it has a directory of them, which is what keeps the cursor themes out.
func Installed(roots []string) []string {
	seen := map[string]bool{}
	names := []string{}
	for _, root := range roots {
		entries, err := os.ReadDir(root)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if !entry.IsDir() || seen[entry.Name()] {
				continue
			}
			if !holdsIcons(filepath.Join(root, entry.Name())) {
				continue
			}
			seen[entry.Name()] = true
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)
	return names
}

// holdsIcons reports whether a theme directory carries icons rather than only
// cursors.
func holdsIcons(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, entry := range entries {
		if entry.IsDir() && entry.Name() != "cursors" && entry.Name() != "cursors_scalable" {
			return true
		}
	}
	return false
}

func settingPath(home string) string {
	return filepath.Join(configDir(home), "dfiles", settingFile)
}

// configDir is where the desktop keeps its configuration.
func configDir(home string) string {
	if dir := os.Getenv("XDG_CONFIG_HOME"); dir != "" {
		return dir
	}
	return filepath.Join(home, ".config")
}

func readFile(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(data)
}

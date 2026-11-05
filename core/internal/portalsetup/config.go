package portalsetup

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/duxlabs-dev/dux-files/core/internal/ini"
)

const preferredGroup = "preferred"

func targetConfig(env Env) (string, bool) {
	dir := filepath.Join(env.ConfigHome, "xdg-desktop-portal")
	names := configFileNames(env)
	for _, name := range names {
		path := filepath.Join(dir, name)
		if isFile(path) {
			return path, true
		}
	}
	return filepath.Join(dir, names[0]), false
}

func configFileNames(env Env) []string {
	var names []string
	for _, desktop := range validDesktops(env.Desktops) {
		names = append(names, desktop+"-portals.conf")
	}
	return append(names, "portals.conf")
}

func preferredLine(content string) (string, bool) {
	lines := splitLines(content)
	_, key := locatePreferred(lines)
	if key < 0 {
		return "", false
	}
	return lines[key], true
}

func setPreferred(content, value string) string {
	return setPreferredLine(content, Interface+"="+value)
}

func setPreferredLine(content, line string) string {
	lines := splitLines(content)
	group, key := locatePreferred(lines)
	switch {
	case key >= 0:
		lines[key] = line
	case group >= 0:
		lines = insertAt(lines, lastContentLine(lines, group)+1, line)
	default:
		lines = appendGroup(lines, "["+preferredGroup+"]", line)
	}
	return joinLines(lines)
}

func removePreferred(content string) string {
	lines := splitLines(content)
	_, key := locatePreferred(lines)
	if key < 0 {
		return content
	}
	lines = append(lines[:key], lines[key+1:]...)
	return joinLines(lines)
}

func backendList(value string) []string {
	var out []string
	for _, part := range strings.Split(value, ";") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		out = append(out, part)
	}
	return out
}

func lookupKey(content, group, key string) (string, bool) {
	lines := splitLines(content)
	header := groupIndex(lines, group)
	if header < 0 {
		return "", false
	}
	idx := keyIndex(lines, header, key)
	if idx < 0 {
		return "", false
	}
	_, value, _ := strings.Cut(lines[idx], "=")
	return strings.TrimSpace(value), true
}

func hasGroup(content, group string) bool {
	return groupIndex(splitLines(content), group) >= 0
}

func locatePreferred(lines []string) (int, int) {
	group := groupIndex(lines, preferredGroup)
	if group < 0 {
		return -1, -1
	}
	return group, keyIndex(lines, group, Interface)
}

func groupIndex(lines []string, group string) int {
	for i, line := range lines {
		if name, ok := ini.Header(line); ok && name == group {
			return i
		}
	}
	return -1
}

func keyIndex(lines []string, header int, key string) int {
	for i := header + 1; i < len(lines); i++ {
		if _, ok := ini.Header(lines[i]); ok {
			return -1
		}
		if ini.Key(lines[i]) == key {
			return i
		}
	}
	return -1
}

func lastContentLine(lines []string, header int) int {
	last := header
	for i := header + 1; i < len(lines); i++ {
		if _, ok := ini.Header(lines[i]); ok {
			return last
		}
		if strings.TrimSpace(lines[i]) != "" {
			last = i
		}
	}
	return last
}

func insertAt(lines []string, at int, line string) []string {
	lines = append(lines, "")
	copy(lines[at+1:], lines[at:])
	lines[at] = line
	return lines
}

func appendGroup(lines []string, header, line string) []string {
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	if len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) != "" {
		lines = append(lines, "")
	}
	return append(lines, header, line)
}

func splitLines(content string) []string {
	if content == "" {
		return nil
	}
	return strings.Split(content, "\n")
}

func joinLines(lines []string) string {
	if len(lines) == 0 {
		return ""
	}
	out := strings.Join(lines, "\n")
	if !strings.HasSuffix(out, "\n") {
		out += "\n"
	}
	if strings.TrimSpace(out) == "" {
		return ""
	}
	return out
}

func isFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}

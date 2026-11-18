package pathutil

import "strings"

func Within(path, dir string) bool {
	return path == dir || Under(path, dir)
}

func Under(path, dir string) bool {
	if path == dir {
		return false
	}
	return strings.HasPrefix(path, strings.TrimSuffix(dir, "/")+"/")
}

func ValidName(name string) bool {
	switch {
	case name == "", name == ".", name == "..":
		return false
	case strings.ContainsAny(name, "/\x00"):
		return false
	case len(name) > 255:
		return false
	}
	return true
}

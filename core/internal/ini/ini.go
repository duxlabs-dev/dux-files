package ini

import (
	"iter"
	"strings"
)

type Line struct {
	Group  string
	Key    string
	Value  string
	Header bool
}

func Lines(data []byte) iter.Seq[Line] {
	return func(yield func(Line) bool) {
		group := ""
		for _, raw := range strings.Split(string(data), "\n") {
			line := strings.TrimSpace(raw)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			if name, ok := Header(line); ok {
				group = name
				if !yield(Line{Group: group, Header: true}) {
					return
				}
				continue
			}
			key, value, ok := strings.Cut(line, "=")
			if !ok {
				continue
			}
			if !yield(Line{Group: group, Key: strings.TrimSpace(key), Value: strings.TrimSpace(value)}) {
				return
			}
		}
	}
}

func Header(line string) (string, bool) {
	trimmed := strings.TrimSpace(line)
	if len(trimmed) < 2 || trimmed[0] != '[' || trimmed[len(trimmed)-1] != ']' {
		return "", false
	}
	return trimmed[1 : len(trimmed)-1], true
}

func Key(line string) string {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || strings.HasPrefix(trimmed, "#") {
		return ""
	}
	key, _, found := strings.Cut(trimmed, "=")
	if !found {
		return ""
	}
	return strings.TrimSpace(key)
}

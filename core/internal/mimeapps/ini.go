package mimeapps

import (
	"os"
	"strings"
	"sync"

	"github.com/duxlabs-dev/dux-files/core/internal/ini"
)

type iniGroup struct {
	keys map[string]string
}

func (g *iniGroup) value(key string) string {
	if g == nil {
		return ""
	}
	return unescapeValue(g.keys[key])
}

func (g *iniGroup) localized(key string) string {
	if g == nil {
		return ""
	}
	for _, tag := range localeCandidates() {
		if v, ok := g.keys[key+"["+tag+"]"]; ok {
			return unescapeValue(v)
		}
	}
	return g.value(key)
}

func parseGroups(data []byte) map[string]*iniGroup {
	groups := map[string]*iniGroup{}
	for line := range ini.Lines(data) {
		switch {
		case line.Header:
			if groups[line.Group] == nil {
				groups[line.Group] = &iniGroup{keys: map[string]string{}}
			}
		case groups[line.Group] == nil:
		default:
			keys := groups[line.Group].keys
			if _, exists := keys[line.Key]; !exists {
				keys[line.Key] = line.Value
			}
		}
	}
	return groups
}

func readGroups(path string) map[string]*iniGroup {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	return parseGroups(data)
}

func splitList(value string) []string {
	var out []string
	for _, item := range strings.Split(unescapeValue(value), ";") {
		if item = strings.TrimSpace(item); item != "" {
			out = append(out, item)
		}
	}
	return out
}

func unescapeValue(value string) string {
	if !strings.ContainsRune(value, '\\') {
		return value
	}
	var b strings.Builder
	for i := 0; i < len(value); i++ {
		if value[i] != '\\' || i+1 >= len(value) {
			b.WriteByte(value[i])
			continue
		}
		i++
		switch value[i] {
		case 's':
			b.WriteByte(' ')
		case 'n':
			b.WriteByte('\n')
		case 't':
			b.WriteByte('\t')
		case 'r':
			b.WriteByte('\r')
		case ';':
			b.WriteByte(';')
		case '\\':
			b.WriteByte('\\')
		default:
			b.WriteByte('\\')
			b.WriteByte(value[i])
		}
	}
	return b.String()
}

var localeCandidates = sync.OnceValue(sessionLocaleCandidates)

func sessionLocaleCandidates() []string {
	return expandLocale(sessionLocale())
}

func sessionLocale() string {
	if first, _, _ := strings.Cut(os.Getenv("LANGUAGE"), ":"); first != "" {
		return first
	}
	for _, key := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		if value := os.Getenv(key); value != "" {
			return value
		}
	}
	return ""
}

func expandLocale(locale string) []string {
	locale = StripParams(locale)
	if locale == "" || locale == "C" || locale == "POSIX" {
		return nil
	}

	modifier := ""
	if base, mod, ok := strings.Cut(locale, "@"); ok {
		locale, modifier = base, mod
	}
	if base, _, ok := strings.Cut(locale, "."); ok {
		locale = base
	}
	lang, country, hasCountry := strings.Cut(locale, "_")

	var tags []string
	switch {
	case hasCountry && modifier != "":
		tags = append(tags, lang+"_"+country+"@"+modifier, lang+"_"+country, lang+"@"+modifier, lang)
	case hasCountry:
		tags = append(tags, lang+"_"+country, lang)
	case modifier != "":
		tags = append(tags, lang+"@"+modifier, lang)
	default:
		tags = append(tags, lang)
	}
	return tags
}

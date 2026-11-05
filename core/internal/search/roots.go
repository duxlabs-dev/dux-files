package search

import (
	"os"
	"path/filepath"
	"slices"
	"sort"

	"github.com/duxlabs-dev/dux-files/core/internal/pathutil"
)

func roots(scope, root string, mounts func() []string) []string {
	switch scope {
	case ScopeHome:
		return []string{homeOr(root)}
	case ScopeEverywhere:
		all := []string{homeOr(root)}
		if mounts != nil {
			all = append(all, mounts()...)
		}
		return dedupeRoots(all)
	}
	return []string{root}
}

func homeOr(fallback string) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return fallback
	}
	return filepath.Clean(home)
}

func dedupeRoots(paths []string) []string {
	cleaned := make([]string, 0, len(paths))
	for _, p := range paths {
		if p == "" || !filepath.IsAbs(p) {
			continue
		}
		cleaned = append(cleaned, filepath.Clean(p))
	}
	sort.Strings(cleaned)
	var out []string
	for _, p := range cleaned {
		if slices.ContainsFunc(out, func(kept string) bool { return pathutil.Within(p, kept) }) {
			continue
		}
		out = append(out, p)
	}
	return out
}

package search

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/duxlabs-dev/dux-files/core/internal/testfs"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIgnoreRulesSkipHeavyFoldersAndUserExclusions(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	rules := newIgnoreRules([]string{"Steam", "~/big", "/mnt/data/old", " "}, false)
	assert.True(t, rules.skipsDir("node_modules", "/x/node_modules"))
	assert.True(t, rules.skipsDir("Steam", "/x/Steam"))
	assert.True(t, rules.skipsDir("big", filepath.Join(os.Getenv("HOME"), "big")))
	assert.True(t, rules.skipsDir("old", "/mnt/data/old"))
	assert.False(t, rules.skipsDir("old", "/mnt/other/old"))
	assert.False(t, rules.skipsDir("src", "/x/src"))

	off := newIgnoreRules(nil, true)
	assert.False(t, off.skipsDir("node_modules", "/x/node_modules"))
	assert.Nil(t, off.rootPatterns("/x"))
}

func TestIgnoreFilesNestNegateAndMatchDirectories(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	root := t.TempDir()
	testfs.Write(t, filepath.Join(root, ".gitignore"), "*.log\n!keep.log\nbuild/\n/top-only\n")
	testfs.Write(t, filepath.Join(root, "sub", ".ignore"), "secret*\n")
	rules := newIgnoreRules(nil, false)

	top := rules.load(root, rules.rootPatterns(root))
	assert.True(t, ignored(top, filepath.Join(root, "a.log"), false))
	assert.True(t, ignored(top, filepath.Join(root, "deep", "b.log"), false))
	assert.False(t, ignored(top, filepath.Join(root, "keep.log"), false))
	assert.True(t, ignored(top, filepath.Join(root, "x", "build"), true))
	assert.False(t, ignored(top, filepath.Join(root, "x", "build"), false))
	assert.True(t, ignored(top, filepath.Join(root, "top-only"), false))
	assert.False(t, ignored(top, filepath.Join(root, "x", "top-only"), false))
	assert.False(t, ignored(top, filepath.Join(root, "sub", "secret.txt"), false))

	sub := rules.load(filepath.Join(root, "sub"), top)
	assert.True(t, ignored(sub, filepath.Join(root, "sub", "secret.txt"), false))
	assert.True(t, ignored(sub, filepath.Join(root, "sub", "c.log"), false))
	assert.Len(t, top, 4, "loading a child never grows the parent's slice")
}

func TestIgnoreRulesHonourAncestorsAndTheGlobalFile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	testfs.Write(t, filepath.Join(home, ".config", "git", "ignore"), "*.swp\n")
	repo := filepath.Join(home, "repo")
	testfs.Write(t, filepath.Join(repo, ".gitignore"), "generated/\n")
	rules := newIgnoreRules(nil, false)

	patterns := rules.rootPatterns(filepath.Join(repo, "core"))
	assert.True(t, ignored(patterns, filepath.Join(repo, "core", "x.swp"), false))
	entries, err := os.ReadDir(repo)
	require.NoError(t, err)
	assert.Equal(t, len(patterns), len(rules.loadIf(filepath.Join(repo, "core"), nil, patterns)), "no ignore file listed, nothing read")
	assert.Greater(t, len(rules.loadIf(repo, entries, patterns)), len(patterns))
	assert.True(t, ignored(patterns, filepath.Join(repo, "core", "generated"), true))
	assert.False(t, ignored(patterns, filepath.Join(repo, "core", "main.go"), false))
}

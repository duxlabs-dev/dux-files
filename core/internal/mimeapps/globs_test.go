package mimeapps

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeMime(t *testing.T, dir, name, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644))
}

func globsFixture(t *testing.T) *DB {
	t.Helper()
	root := t.TempDir()
	home := filepath.Join(root, "home", "mime")
	sys := filepath.Join(root, "sys", "mime")
	writeMime(t, home, "globs2", `# comment line
50:image/png:*.png
50:image/jpeg:*.jpg
50:image/jpeg:*.jpeg
50:text/plain:*.txt
50:text/x-python:*.py:cs
50:text/x-makefile:Makefile
50:application/x-shellscript:*.sh
50:text/x-csrc:*.[ch]
10:application/gzip:*.gz
garbage line
50:nocolon
50:text/plain:

50:cycle/a:*.ca
50:cycle/b:*.cb
`)
	writeMime(t, home, "aliases", `image/x-png image/png
application/x-gzip application/gzip
broken
`)
	writeMime(t, home, "subclasses", `text/x-python text/plain
application/x-shellscript text/plain
text/x-csrc text/plain
text/x-makefile text/plain
cycle/a cycle/b
cycle/b cycle/a
`)
	writeMime(t, sys, "globs2", `50:image/png:*.PNG
50:image/webp:*.webp
50:text/x-python:*.pyw
`)
	return LoadDB([]string{home, sys, filepath.Join(root, "missing", "mime")})
}

func TestGlobsDirectLookup(t *testing.T) {
	db := globsFixture(t)
	assert.Equal(t, []string{"*.jpeg", "*.jpg"}, db.Globs("image/jpeg"))
	assert.Equal(t, []string{"*.gz"}, db.Globs(" Application/GZIP "))
}

func TestGlobsFollowAliases(t *testing.T) {
	db := globsFixture(t)
	assert.Equal(t, []string{"*.png"}, db.Globs("image/x-png"))
	assert.Equal(t, []string{"*.gz"}, db.Globs("application/x-gzip"))
}

func TestGlobsWildcardCoversTheMediaType(t *testing.T) {
	db := globsFixture(t)
	assert.Equal(t, []string{"*.jpeg", "*.jpg", "*.png", "*.webp"}, db.Globs("image/*"))
}

func TestGlobsIncludeTransitiveSubclasses(t *testing.T) {
	db := globsFixture(t)
	assert.Equal(t, []string{"*.[ch]", "*.py", "*.pyw", "*.sh", "*.txt", "makefile"}, db.Globs("text/plain"))
}

func TestGlobsUnknownTypesAreNil(t *testing.T) {
	db := globsFixture(t)
	assert.Nil(t, db.Globs("application/x-unknown"))
	assert.Nil(t, db.Globs("nope/*"))
	assert.Nil(t, db.Globs(""))
}

func TestGlobsDedupeAcrossDirs(t *testing.T) {
	db := globsFixture(t)
	assert.Equal(t, []string{"*.png"}, db.Globs("image/png"))
}

func TestGlobsIgnoreMalformedLines(t *testing.T) {
	db := globsFixture(t)
	assert.Nil(t, db.Globs("garbage line"))
	assert.Nil(t, db.Globs("nocolon"))
	assert.Nil(t, db.Globs("broken"))
	assert.Equal(t, []string{"*.txt"}, db.globs.byMime["text/plain"])
}

func TestGlobsSubclassCycleTerminates(t *testing.T) {
	db := globsFixture(t)
	assert.Equal(t, []string{"*.ca", "*.cb"}, db.Globs("cycle/a"))
	assert.Equal(t, []string{"*.ca", "*.cb"}, db.Globs("cycle/*"))
}

package mimeapps

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/duxlabs-dev/dux-files/core/internal/testfs"

	"github.com/duxlabs-dev/dux-go/files"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGlobPriorityAndCase(t *testing.T) {
	svc, _ := fixture(t)
	d := svc.Detector()

	assert.Equal(t, "application/x-compressed-tar", d.FromName("backup.tar.gz"), "the longer suffix wins")
	assert.Equal(t, "application/gzip", d.FromName("backup.gz"))
	assert.Equal(t, "image/png", d.FromName("SHOT.PNG"), "globs are case insensitive by default")
	assert.Equal(t, "application/x-cd-image", d.FromName("disc.iso"))
	assert.Empty(t, d.FromName("disc.ISO"), "a cs glob only matches the exact case")
	assert.Equal(t, "text/x-changelog", d.FromName("ChangeLog"), "a literal name beats every pattern")
	assert.Equal(t, "text/x-csrc", d.FromName("main.c"), "character classes are matched")
	assert.Equal(t, "text/x-makefile", d.FromName("Makefile"))
	assert.Empty(t, d.FromName("nothing.unknownext"))
}

func TestMagicSniffing(t *testing.T) {
	svc, root := fixture(t)
	d := svc.Detector()

	png := testfs.WriteBytes(t, filepath.Join(root, "files", "blob"), []byte("\x89PNG\r\n\x1a\n rest"))
	assert.Equal(t, "image/png", d.Sniff(png))

	pdf := testfs.WriteBytes(t, filepath.Join(root, "files", "doc"), []byte("%PDF-1.7 trailing"))
	assert.Equal(t, "application/pdf", d.Sniff(pdf))

	wav := testfs.WriteBytes(t, filepath.Join(root, "files", "sound"), []byte("RIFF....WAVEfmt "))
	assert.Equal(t, "audio/x-wav", d.Sniff(wav), "the nested matchlet picks between two RIFF types")

	avi := testfs.WriteBytes(t, filepath.Join(root, "files", "movie"), []byte("RIFF....AVI LIST"))
	assert.Equal(t, "video/x-msvideo", d.Sniff(avi))

	riff := testfs.WriteBytes(t, filepath.Join(root, "files", "riff"), []byte("RIFF\x00\x00\x00\x00NOPE\x00"))
	assert.Equal(t, "application/octet-stream", d.Sniff(riff), "a parent without a matching child does not match")

	script := testfs.WriteBytes(t, filepath.Join(root, "files", "run"), []byte("#!  /bin/sh\necho hi\n"))
	assert.Equal(t, "application/x-shellscript", d.Sniff(script), "the child matchlet searches its range")

	prose := testfs.WriteBytes(t, filepath.Join(root, "files", "prose"), []byte("héllo wörld\n"))
	assert.Equal(t, "text/plain", d.Sniff(prose))

	blob := testfs.WriteBytes(t, filepath.Join(root, "files", "binary"), []byte{0x00, 0x01, 0x02, 0x7f})
	assert.Equal(t, "application/octet-stream", d.Sniff(blob))

	assert.Empty(t, d.Sniff(filepath.Join(root, "files", "missing")))
	assert.Empty(t, d.Sniff(testfs.WriteBytes(t, filepath.Join(root, "files", "empty"), nil)))
}

func TestSniffHonoursMaxExtent(t *testing.T) {
	svc, root := fixture(t)
	d := svc.Detector()

	body := append([]byte(strings.Repeat(".", 600)), []byte("DEEPMARK")...)
	deep := testfs.WriteBytes(t, filepath.Join(root, "files", "deep"), body)
	assert.Equal(t, "text/x-deep", d.Sniff(deep), "the read reaches the database's furthest rule")
}

func TestDetectCombinesGlobAndMagic(t *testing.T) {
	svc, root := fixture(t)
	d := svc.Detector()

	trap := testfs.WriteBytes(t, filepath.Join(root, "files", "trap.txt"), []byte("\x89PNG\r\n\x1a\n rest"))
	assert.Equal(t, "text/plain", d.Detect(trap, false), "a low priority magic match does not beat the glob, as gio does it")
	assert.Equal(t, "image/png", d.Sniff(trap), "an explicit sniff always reports the content type")

	deep := testfs.WriteBytes(t, filepath.Join(root, "files", "deep.txt"), append([]byte(strings.Repeat(".", 600)), []byte("DEEPMARK")...))
	assert.Equal(t, "text/plain", d.Detect(deep, false), "a magic subtype of the glob type does not beat the glob either, per the spec's checking order")

	wins := testfs.WriteBytes(t, filepath.Join(root, "files", "wins.txt"), []byte("MAGICWIN and more"))
	assert.Equal(t, "application/x-magic-wins", d.Detect(wins, false), "priority 90 outranks the glob")

	nameless := testfs.WriteBytes(t, filepath.Join(root, "files", "nameless"), []byte("%PDF-1.4"))
	assert.Equal(t, "application/pdf", d.Detect(nameless, false), "with no glob match the magic type stands")

	assert.Equal(t, TypeDirectory, d.Detect(root, true), "directories never read content")
}

func TestAliasesAndSubclasses(t *testing.T) {
	svc, _ := fixture(t)
	db := svc.Detector().DB()

	assert.Equal(t, "image/png", db.Canonical("image/x-png"))
	assert.Equal(t, "application/gzip", db.Canonical("application/x-gzip; charset=binary"))
	assert.Equal(t, []string{"text/markdown", "text/plain", "application/octet-stream"}, db.Chain("text/markdown"))
	assert.Equal(t, []string{"text/x-unknown", "text/plain", "application/octet-stream"}, db.Chain("text/x-unknown"), "text/* implicitly descends from text/plain")
	assert.Equal(t, []string{"inode/directory"}, db.Chain("inode/directory"), "inode types are not streamable")
	assert.True(t, db.IsSubclassOf("application/x-shellscript", "text/plain"))
	assert.False(t, db.IsSubclassOf("image/png", "text/plain"))
}

func TestMimeIconTables(t *testing.T) {
	svc, _ := fixture(t)
	db := svc.Detector().DB()

	specific, generic := db.MimeIcon("application/x-desktop")
	assert.Equal(t, "application-x-executable", specific)
	assert.Empty(t, generic)

	specific, generic = db.MimeIcon("text/markdown")
	assert.Empty(t, specific)
	assert.Equal(t, "text-x-generic", generic)
}

func TestMimeOfReportsMissingPaths(t *testing.T) {
	svc, root := fixture(t)

	_, err := svc.MimeOf(filepath.Join(root, "gone"))
	require.Error(t, err)
	assert.Equal(t, files.CodeNotFound, files.CodeOf(err))
}

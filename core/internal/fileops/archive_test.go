package fileops

import (
	"archive/tar"
	"archive/zip"
	"os"
	"path/filepath"
	"testing"

	"github.com/duxlabs-dev/dux-files/core/internal/testfs"

	"github.com/duxlabs-dev/dux-go/ipc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func sampleTree(t *testing.T) (root, project string) {
	t.Helper()
	root = t.TempDir()
	project = filepath.Join(root, "project")
	testfs.Write(t, filepath.Join(project, "README.md"), "hello")
	testfs.Write(t, filepath.Join(project, "src", "main.go"), "package main")
	require.NoError(t, os.Chmod(filepath.Join(project, "src", "main.go"), 0o755))
	require.NoError(t, os.Symlink("README.md", filepath.Join(project, "link")))
	require.NoError(t, os.MkdirAll(filepath.Join(project, "empty"), 0o755))
	return root, project
}

func TestCompressAndExtractRoundTrip(t *testing.T) {
	for _, format := range compressFormats {
		t.Run(format, func(t *testing.T) {
			root, project := sampleTree(t)
			manager, bus := newHarness(t)

			view := run(t, manager, bus, spec{kind: KindCompress, sources: []string{project, filepath.Join(project, "README.md")}, dest: root, name: "bundle", format: format})
			require.Equal(t, StateDone, view.State, view.Error)
			archive := filepath.Join(root, "bundle."+format)
			require.FileExists(t, archive)
			assert.True(t, view.Undoable)
			assert.Equal(t, 0, view.Progress.FilesTotal-view.Progress.FilesDone)
			assert.Empty(t, tempsIn(t, root))

			out := filepath.Join(root, "out")
			require.NoError(t, os.Mkdir(out, 0o755))
			view = run(t, manager, bus, spec{kind: KindExtract, sources: []string{archive}, dest: out})
			require.Equal(t, StateDone, view.State, view.Error)

			base := filepath.Join(out, "bundle")
			assert.Equal(t, "hello", testfs.Read(t, filepath.Join(base, "project", "README.md")))
			assert.Equal(t, "package main", testfs.Read(t, filepath.Join(base, "project", "src", "main.go")))
			assert.Equal(t, "hello", testfs.Read(t, filepath.Join(base, "README.md")), "a second top-level source lands beside the first")
			info, err := os.Stat(filepath.Join(base, "project", "src", "main.go"))
			require.NoError(t, err)
			assert.Equal(t, os.FileMode(0o755), info.Mode().Perm())
			assert.DirExists(t, filepath.Join(base, "project", "empty"))
			target, err := os.Readlink(filepath.Join(base, "project", "link"))
			require.NoError(t, err)
			assert.Equal(t, "README.md", target)
			assert.True(t, view.Undoable)

			undo, err := manager.Undo("")
			require.NoError(t, err)
			bus.wait(t, undo.ID)
			assert.NoDirExists(t, base)
		})
	}
}

func TestCompressKeepsBothWhenTheNameIsTaken(t *testing.T) {
	root, project := sampleTree(t)
	testfs.Write(t, filepath.Join(root, "project.zip"), "old")
	manager, bus := newHarness(t)

	view := run(t, manager, bus, spec{kind: KindCompress, sources: []string{project}, dest: root, name: "project", format: FormatZip})
	require.Equal(t, StateDone, view.State, view.Error)
	assert.Equal(t, "old", testfs.Read(t, filepath.Join(root, "project.zip")))
	assert.FileExists(t, filepath.Join(root, "project (copy).zip"))
}

func TestExtractIntoAnExistingFolderNameKeepsBoth(t *testing.T) {
	root, project := sampleTree(t)
	manager, bus := newHarness(t)
	run(t, manager, bus, spec{kind: KindCompress, sources: []string{project}, dest: root, name: "project", format: FormatTarGz})
	require.NoError(t, os.Mkdir(filepath.Join(root, "out"), 0o755))
	require.NoError(t, os.Mkdir(filepath.Join(root, "out", "project"), 0o755))

	view := run(t, manager, bus, spec{kind: KindExtract, sources: []string{filepath.Join(root, "project.tar.gz")}, dest: filepath.Join(root, "out")})
	require.Equal(t, StateDone, view.State, view.Error)
	assert.FileExists(t, filepath.Join(root, "out", "project (copy)", "README.md"))
}

func TestExtractRefusesEntriesThatEscape(t *testing.T) {
	root := t.TempDir()
	archive := filepath.Join(root, "evil.zip")
	f, err := os.Create(archive)
	require.NoError(t, err)
	w := zip.NewWriter(f)
	entry, err := w.Create("../escaped.txt")
	require.NoError(t, err)
	_, err = entry.Write([]byte("nope"))
	require.NoError(t, err)
	require.NoError(t, w.Close())
	require.NoError(t, f.Close())

	manager, bus := newHarness(t)
	out := filepath.Join(root, "out")
	require.NoError(t, os.Mkdir(out, 0o755))
	view := run(t, manager, bus, spec{kind: KindExtract, sources: []string{archive}, dest: out})
	assert.Equal(t, StateFailed, view.State)
	assert.NoFileExists(t, filepath.Join(root, "escaped.txt"))
	assert.NoDirExists(t, filepath.Join(out, "evil"), "a failed extraction leaves nothing behind")
}

func TestExtractDetectsFormatWithoutAnExtension(t *testing.T) {
	root, project := sampleTree(t)
	manager, bus := newHarness(t)
	run(t, manager, bus, spec{kind: KindCompress, sources: []string{project}, dest: root, name: "project", format: FormatZip})
	bare := filepath.Join(root, "bare")
	require.NoError(t, os.Rename(filepath.Join(root, "project.zip"), bare))
	out := filepath.Join(root, "out")
	require.NoError(t, os.Mkdir(out, 0o755))

	view := run(t, manager, bus, spec{kind: KindExtract, sources: []string{bare}, dest: out})
	require.Equal(t, StateDone, view.State, view.Error)
	assert.FileExists(t, filepath.Join(out, "bare", "project", "README.md"))

	testfs.Write(t, filepath.Join(root, "plain.bin"), "not an archive")
	view = run(t, manager, bus, spec{kind: KindExtract, sources: []string{filepath.Join(root, "plain.bin")}, dest: out})
	assert.Equal(t, StateFailed, view.State)
	assert.NoDirExists(t, filepath.Join(out, "plain.bin"))
}

func TestArchiveStem(t *testing.T) {
	assert.Equal(t, "a", archiveStem("a.tar.gz"))
	assert.Equal(t, "a.b", archiveStem("a.b.tgz"))
	assert.Equal(t, "A", archiveStem("A.ZIP"))
	assert.Equal(t, "a", archiveStem("a.tar"))
	assert.Equal(t, "a", archiveStem("a.tar.xz"))
	assert.Equal(t, "a", archiveStem("a.tar.zst"))
	assert.Equal(t, "a", archiveStem("a.tbz2"))
	assert.Equal(t, "a", archiveStem("a.7z"))
	assert.Equal(t, "a", archiveStem("a.rar"))
	assert.Equal(t, "a", archiveStem("a.cpio"))
	assert.Equal(t, "notes.txt", archiveStem("notes.txt.gz"))
	assert.Equal(t, "notes.txt", archiveStem("notes.txt.xz"))
	assert.Equal(t, "plain", archiveStem("plain"))
	assert.Equal(t, ".zip", archiveStem(".zip"))
}

func fixture(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	require.NoError(t, err)
	path := filepath.Join(t.TempDir(), name)
	require.NoError(t, os.WriteFile(path, data, 0o644))
	return path
}

func extractFixture(t *testing.T, archive string) (string, View) {
	t.Helper()
	manager, bus := newHarness(t)
	out := filepath.Join(t.TempDir(), "out")
	require.NoError(t, os.Mkdir(out, 0o755))
	view := run(t, manager, bus, spec{kind: KindExtract, sources: []string{archive}, dest: out})
	return out, view
}

func assertSampleTree(t *testing.T, base string) {
	t.Helper()
	assert.Equal(t, "hello", testfs.Read(t, filepath.Join(base, "README.md")))
	assert.Equal(t, "package main", testfs.Read(t, filepath.Join(base, "src", "main.go")))
	info, err := os.Stat(filepath.Join(base, "src", "main.go"))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o755), info.Mode().Perm())
	assert.DirExists(t, filepath.Join(base, "empty"))
	target, err := os.Readlink(filepath.Join(base, "link"))
	require.NoError(t, err)
	assert.Equal(t, "README.md", target)
}

// The fixtures were packed from the sampleTree layout with the system tools
// (tar+bzip2/xz/zstd/lzma, 7z -snl, cpio -H newc).
func TestExtractForeignFormats(t *testing.T) {
	for _, name := range []string{"sample.tar.bz2", "sample.tar.xz", "sample.tar.zst", "sample.tar.lzma", "sample.7z", "sample.cpio"} {
		t.Run(name, func(t *testing.T) {
			out, view := extractFixture(t, fixture(t, name))
			require.Equal(t, StateDone, view.State, view.Error)
			assert.Equal(t, []Created{{Path: filepath.Join(out, "sample"), Dir: true}}, view.Created)
			assertSampleTree(t, filepath.Join(out, "sample"))
			assert.True(t, view.Undoable)
			assert.Equal(t, 0, view.Progress.FilesTotal-view.Progress.FilesDone)
		})
	}
}

// libarchive.rar is test_read_format_rar.rar from libarchive's test suite (BSD-2-Clause).
func TestExtractRar(t *testing.T) {
	out, view := extractFixture(t, fixture(t, "libarchive.rar"))
	require.Equal(t, StateDone, view.State, view.Error)
	base := filepath.Join(out, "libarchive")
	assert.Equal(t, "test text document\r\n", testfs.Read(t, filepath.Join(base, "test.txt")))
	assert.Equal(t, "test text document\r\n", testfs.Read(t, filepath.Join(base, "testdir", "test.txt")))
	assert.DirExists(t, filepath.Join(base, "testemptydir"))
	target, err := os.Readlink(filepath.Join(base, "testlink"))
	require.NoError(t, err)
	assert.Equal(t, "test.txt", target)
}

func TestExtractLoneCompressedFileBesideItself(t *testing.T) {
	for _, name := range []string{"hello.txt.gz", "hello.txt.xz", "hello.txt.bz2"} {
		t.Run(name, func(t *testing.T) {
			out, view := extractFixture(t, fixture(t, name))
			require.Equal(t, StateDone, view.State, view.Error)
			assert.Equal(t, "hello", testfs.Read(t, filepath.Join(out, "hello.txt")))
			assert.NoDirExists(t, filepath.Join(out, "hello.txt"))
			assert.Equal(t, []Created{{Path: filepath.Join(out, "hello.txt"), Dir: false}}, view.Created)
			assert.True(t, view.Undoable)
		})
	}
}

func TestDetectFormatByMagic(t *testing.T) {
	cases := map[string]archiveFormat{
		"sample.tar.bz2": {compressBzip2, containerNone},
		"sample.tar.xz":  {compressXz, containerNone},
		"sample.tar.zst": {compressZstd, containerNone},
		"sample.7z":      {compressNone, container7z},
		"libarchive.rar": {compressNone, containerRar},
		"sample.cpio":    {compressNone, containerCpio},
	}
	for name, want := range cases {
		t.Run(name, func(t *testing.T) {
			bare := filepath.Join(t.TempDir(), "bare")
			require.NoError(t, os.Rename(fixture(t, name), bare))
			got, err := detectFormat(bare)
			require.NoError(t, err)
			assert.Equal(t, want, got)
			assert.True(t, IsArchive(bare))
		})
	}
	plain := filepath.Join(t.TempDir(), "plain")
	testfs.Write(t, plain, "not an archive")
	assert.False(t, IsArchive(plain))
	assert.False(t, IsArchive(filepath.Join(t.TempDir(), "missing")))
}

func TestExtractCompressedTarWithoutAnExtensionStillUnpacksTheTree(t *testing.T) {
	bare := filepath.Join(t.TempDir(), "bare")
	require.NoError(t, os.Rename(fixture(t, "sample.tar.zst"), bare))
	out, view := extractFixture(t, bare)
	require.Equal(t, StateDone, view.State, view.Error)
	assertSampleTree(t, filepath.Join(out, "bare", "sample"))
}

func TestCompressHandlerValidation(t *testing.T) {
	manager, _ := newHarness(t)
	cases := []map[string]any{
		{"paths": "/tmp/a", "dest": "/tmp", "name": "x", "format": "rar"},
		{"paths": "/tmp/a", "dest": "/tmp", "name": "x/y"},
		{"paths": "/tmp/a", "dest": "relative", "name": "x"},
	}
	for _, params := range cases {
		out := call(t, manager, ipcRequest("files.op.compress", params))
		require.Contains(t, out, "error")
	}
	out := call(t, manager, ipcRequest("files.op.extract", map[string]any{"paths": "/tmp/a"}))
	require.Contains(t, out, "error")
	require.Empty(t, manager.Jobs())
}

func ipcRequest(method string, params map[string]any) ipc.Request {
	return ipc.Request{Method: method, Params: params}
}

func TestExtractRefusesWritingThroughAPlantedSymlink(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(root, "outside")
	require.NoError(t, os.Mkdir(outside, 0o755))
	archive := filepath.Join(root, "sneaky.zip")
	f, err := os.Create(archive)
	require.NoError(t, err)
	w := zip.NewWriter(f)
	link := &zip.FileHeader{Name: "evil"}
	link.SetMode(os.ModeSymlink | 0o777)
	entry, err := w.CreateHeader(link)
	require.NoError(t, err)
	_, err = entry.Write([]byte(outside))
	require.NoError(t, err)
	entry, err = w.Create("evil/planted.txt")
	require.NoError(t, err)
	_, err = entry.Write([]byte("nope"))
	require.NoError(t, err)
	require.NoError(t, w.Close())
	require.NoError(t, f.Close())

	manager, bus := newHarness(t)
	out := filepath.Join(root, "out")
	require.NoError(t, os.Mkdir(out, 0o755))
	view := run(t, manager, bus, spec{kind: KindExtract, sources: []string{archive}, dest: out})
	assert.Equal(t, StateFailed, view.State)
	assert.NoFileExists(t, filepath.Join(outside, "planted.txt"))
}

func TestCompressSkipsAnUnreadableSubdirectory(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root reads everything")
	}
	root, project := sampleTree(t)
	locked := filepath.Join(project, "locked")
	require.NoError(t, os.Mkdir(locked, 0o755))
	testfs.Write(t, filepath.Join(locked, "secret.txt"), "x")
	require.NoError(t, os.Chmod(locked, 0o000))
	t.Cleanup(func() { _ = os.Chmod(locked, 0o755) })

	manager, bus := newHarness(t)
	view := run(t, manager, bus, spec{kind: KindCompress, sources: []string{project}, dest: root, name: "project", format: FormatZip})
	require.Equal(t, StateDone, view.State, view.Error)
	assert.FileExists(t, filepath.Join(root, "project.zip"))
	assert.Equal(t, 1, view.Skipped)
}

func TestExtractSkipsTheRootEntryOfATar(t *testing.T) {
	archive := filepath.Join(t.TempDir(), "rooted.tar")
	f, err := os.Create(archive)
	require.NoError(t, err)
	w := tar.NewWriter(f)
	require.NoError(t, w.WriteHeader(&tar.Header{Name: "./", Typeflag: tar.TypeDir, Mode: 0o755}))
	require.NoError(t, w.WriteHeader(&tar.Header{Name: "./sample/", Typeflag: tar.TypeDir, Mode: 0o755}))
	require.NoError(t, w.WriteHeader(&tar.Header{Name: "./sample/hello.txt", Typeflag: tar.TypeReg, Mode: 0o644, Size: 5}))
	_, err = w.Write([]byte("hello"))
	require.NoError(t, err)
	require.NoError(t, w.Close())
	require.NoError(t, f.Close())

	out, view := extractFixture(t, archive)
	require.Equal(t, StateDone, view.State, view.Error)
	assert.Equal(t, "hello", testfs.Read(t, filepath.Join(out, "rooted", "sample", "hello.txt")))
}

func zipWith(t *testing.T, path string, entries map[string]string) string {
	t.Helper()
	f, err := os.Create(path)
	require.NoError(t, err)
	w := zip.NewWriter(f)
	for name, body := range entries {
		entry, err := w.Create(name)
		require.NoError(t, err)
		_, err = entry.Write([]byte(body))
		require.NoError(t, err)
	}
	require.NoError(t, w.Close())
	require.NoError(t, f.Close())
	return path
}

func TestExtractLiftsASoleEntryNamedLikeTheArchive(t *testing.T) {
	root := t.TempDir()
	out := filepath.Join(root, "out")
	require.NoError(t, os.Mkdir(out, 0o755))
	manager, bus := newHarness(t)
	extract := func(archive string) View {
		view := run(t, manager, bus, spec{kind: KindExtract, sources: []string{archive}, dest: out})
		require.Equal(t, StateDone, view.State, view.Error)
		assert.Empty(t, tempsIn(t, out))
		return view
	}

	project := zipWith(t, filepath.Join(root, "project.zip"), map[string]string{"project/README.md": "hello", "project/src/main.go": "package main"})
	view := extract(project)
	assert.Equal(t, []Created{{Path: filepath.Join(out, "project"), Dir: true}}, view.Created)
	assert.Equal(t, "hello", testfs.Read(t, filepath.Join(out, "project", "README.md")))
	assert.Equal(t, "package main", testfs.Read(t, filepath.Join(out, "project", "src", "main.go")))

	view = extract(project)
	assert.Equal(t, []Created{{Path: filepath.Join(out, "project (copy)"), Dir: true}}, view.Created)
	assert.Equal(t, "hello", testfs.Read(t, filepath.Join(out, "project (copy)", "README.md")))

	release := zipWith(t, filepath.Join(root, "release-1.0.zip"), map[string]string{"release/": "", "release/bin": "x"})
	extract(release)
	assert.Equal(t, "x", testfs.Read(t, filepath.Join(out, "release-1.0", "release", "bin")), "a differently named folder keeps the archive folder")

	notes := zipWith(t, filepath.Join(root, "notes.zip"), map[string]string{"notes.txt": "n"})
	view = extract(notes)
	assert.Equal(t, []Created{{Path: filepath.Join(out, "notes.txt"), Dir: false}}, view.Created)
	assert.Equal(t, "n", testfs.Read(t, filepath.Join(out, "notes.txt")))
	view = extract(notes)
	assert.Equal(t, "n", testfs.Read(t, filepath.Join(out, "notes (copy).txt")))

	readme := zipWith(t, filepath.Join(root, "docs.zip"), map[string]string{"README.md": "r"})
	extract(readme)
	assert.Equal(t, "r", testfs.Read(t, filepath.Join(out, "docs", "README.md")), "a differently named file keeps the archive folder")

	pair := zipWith(t, filepath.Join(root, "pair.zip"), map[string]string{"pair/x": "1", "other/y": "2"})
	extract(pair)
	assert.Equal(t, "1", testfs.Read(t, filepath.Join(out, "pair", "pair", "x")))
	assert.Equal(t, "2", testfs.Read(t, filepath.Join(out, "pair", "other", "y")))

	undo, err := manager.Undo("")
	require.NoError(t, err)
	bus.wait(t, undo.ID)
	assert.NoDirExists(t, filepath.Join(out, "pair"))
}

func TestSameStem(t *testing.T) {
	assert.True(t, sameStem("project", "project"))
	assert.True(t, sameStem("notes.txt", "notes"))
	assert.True(t, sameStem("inner.tar.gz", "inner"))
	assert.False(t, sameStem("release", "release-1.0"))
	assert.False(t, sameStem("Project", "project"))
}

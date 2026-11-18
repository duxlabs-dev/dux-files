package helper

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/duxlabs-dev/dux-files/core/internal/testfs"

	"github.com/duxlabs-dev/dux-go/files"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

func runOp(t *testing.T, stdin []byte, args ...string) (string, error) {
	t.Helper()
	var out bytes.Buffer
	err := Run(args, bytes.NewReader(stdin), &out)
	return out.String(), err
}

func frames(chunks ...string) []byte {
	var buf bytes.Buffer
	for _, chunk := range chunks {
		var header [4]byte
		binary.BigEndian.PutUint32(header[:], uint32(len(chunk)))
		buf.Write(header[:])
		buf.WriteString(chunk)
	}
	return buf.Bytes()
}

func committed(chunks ...string) []byte {
	return append(frames(chunks...), 0, 0, 0, 0)
}

func TestRunRefusesBadArguments(t *testing.T) {
	root := t.TempDir()
	cases := map[string][]string{
		"no operation":     {},
		"unknown op":       {"fly", root},
		"relative path":    {OpLstat, "relative/a"},
		"unclean path":     {OpLstat, root + "//a"},
		"dotdot path":      {OpLstat, root + "/a/.."},
		"root path":        {OpRemoveAll, "/"},
		"missing mode":     {OpMkdir, root + "/d"},
		"bad mode":         {OpMkdir, root + "/d", "9x"},
		"mode too wide":    {OpChmod, root, "17777"},
		"special bits":     {OpChmod, root, "4755"},
		"bad flag":         {OpChmod, root, "755", "deep"},
		"bad uid":          {OpChown, root, "x", "0"},
		"uid sentinel":     {OpChown, root, "4294967295", "0"},
		"rename flag":      {OpRename, root, root + "/b", "replace"},
		"symlink arity":    {OpSymlink, root},
		"rename arity":     {OpRename, root},
		"too many paths":   {OpRead, root, root},
		"nul in path":      {OpLstat, root + "/a\x00b"},
		"chown arity":      {OpChown, root, "0"},
		"write needs mode": {OpWrite, root + "/f"},
	}
	for name, args := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := runOp(t, nil, args...)
			require.Error(t, err)
			assert.Equal(t, files.CodeInvalid, files.CodeOf(err), err.Error())
		})
	}
}

func TestSymlinkComponentsAreRefused(t *testing.T) {
	root := t.TempDir()
	real := filepath.Join(root, "real")
	require.NoError(t, os.Mkdir(real, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(real, "f"), []byte("x"), 0o644))
	link := filepath.Join(root, "link")
	require.NoError(t, os.Symlink(real, link))
	fileLink := filepath.Join(root, "flink")
	require.NoError(t, os.Symlink(filepath.Join(real, "f"), fileLink))

	for name, args := range map[string][]string{
		"write through a symlinked dir": {OpWrite, filepath.Join(link, "new"), "644"},
		"readdir through a symlink":     {OpReadDir, link},
		"read a symlink leaf":           {OpRead, fileLink},
		"mkdir under a symlink":         {OpMkdir, filepath.Join(link, "d"), "755"},
		"chmod through a symlink":       {OpChmod, filepath.Join(link, "f"), "600"},
		"remove through a symlink":      {OpRemove, filepath.Join(link, "f")},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := runOp(t, committed("data"), args...)
			require.ErrorIs(t, err, errSymlink)
		})
	}

	assert.Equal(t, "x", string(testfs.ReadBytes(t, filepath.Join(real, "f"))))
	_, err := os.Lstat(filepath.Join(real, "new"))
	assert.True(t, os.IsNotExist(err), "nothing was written through the symlink")

	out, err := runOp(t, nil, OpLstat, fileLink)
	require.NoError(t, err)
	var info Info
	require.NoError(t, json.Unmarshal([]byte(out), &info))
	assert.NotZero(t, fs.FileMode(info.Mode)&fs.ModeSymlink, "lstat describes the link itself")
}

func TestLstatReadDirReadlinkDescribeTheTree(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "d")
	require.NoError(t, os.Mkdir(dir, 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.txt"), []byte("abc"), 0o640))
	require.NoError(t, os.Symlink("a.txt", filepath.Join(dir, "l")))

	out, err := runOp(t, nil, OpLstat, filepath.Join(dir, "a.txt"))
	require.NoError(t, err)
	var info Info
	require.NoError(t, json.Unmarshal([]byte(out), &info))
	assert.Equal(t, "a.txt", info.Name)
	assert.Equal(t, int64(3), info.Size)
	assert.Equal(t, fs.FileMode(0o640), fs.FileMode(info.Mode))
	assert.Equal(t, uint32(os.Getuid()), info.UID)
	assert.NotZero(t, info.MtimeNs)

	out, err = runOp(t, nil, OpLstat, dir)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal([]byte(out), &info))
	assert.True(t, fs.FileMode(info.Mode).IsDir())

	out, err = runOp(t, nil, OpReadDir, dir)
	require.NoError(t, err)
	var entries []Entry
	require.NoError(t, json.Unmarshal([]byte(out), &entries))
	names := map[string]fs.FileMode{}
	for _, entry := range entries {
		names[entry.Name] = fs.FileMode(entry.Mode)
	}
	assert.Equal(t, fs.FileMode(0), names["a.txt"])
	assert.Equal(t, fs.ModeSymlink, names["l"])

	out, err = runOp(t, nil, OpReadlink, filepath.Join(dir, "l"))
	require.NoError(t, err)
	assert.JSONEq(t, `{"target":"a.txt"}`, out)

	_, err = runOp(t, nil, OpLstat, filepath.Join(dir, "missing"))
	assert.Equal(t, files.CodeNotFound, files.CodeFromOS(err))
}

func TestReadStreamsRegularFilesOnly(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "f")
	require.NoError(t, os.WriteFile(path, bytes.Repeat([]byte("z"), 70000), 0o600))

	out, err := runOp(t, nil, OpRead, path)
	require.NoError(t, err)
	assert.Len(t, out, 70000)

	fifo := filepath.Join(root, "fifo")
	require.NoError(t, unix.Mkfifo(fifo, 0o600))
	_, err = runOp(t, nil, OpRead, fifo)
	require.Error(t, err)
	assert.Equal(t, files.CodeInvalid, files.CodeOf(err))

	_, err = runOp(t, nil, OpRead, root)
	require.Error(t, err)
}

func TestWriteCommitsFramesAndDropsAbortedFiles(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "out")

	_, err := runOp(t, committed("hello ", "world"), OpWrite, path, "640")
	require.NoError(t, err)
	assert.Equal(t, "hello world", string(testfs.ReadBytes(t, path)))
	info, err := os.Lstat(path)
	require.NoError(t, err)
	assert.Equal(t, fs.FileMode(0o640), info.Mode().Perm()&^fs.FileMode(umask(t)))

	_, err = runOp(t, committed("again"), OpWrite, path, "644")
	require.Error(t, err)
	assert.Equal(t, files.CodeExists, files.CodeFromOS(err))
	assert.Equal(t, "hello world", string(testfs.ReadBytes(t, path)), "an existing file is never replaced")

	aborted := filepath.Join(root, "aborted")
	_, err = runOp(t, frames("partial"), OpWrite, aborted, "644")
	require.Error(t, err)
	_, statErr := os.Lstat(aborted)
	assert.True(t, os.IsNotExist(statErr), "an uncommitted write leaves nothing behind")

	empty := filepath.Join(root, "empty")
	_, err = runOp(t, committed(), OpWrite, empty, "644")
	require.NoError(t, err)
	assert.Empty(t, testfs.ReadBytes(t, empty))
}

func TestMkdirSymlinkRenameRemove(t *testing.T) {
	root := t.TempDir()

	_, err := runOp(t, nil, OpMkdirAll, filepath.Join(root, "a", "b", "c"), "755")
	require.NoError(t, err)
	_, err = runOp(t, nil, OpMkdirAll, filepath.Join(root, "a", "b", "c"), "755")
	require.NoError(t, err, "mkdirall tolerates existing directories")
	_, err = runOp(t, nil, OpMkdir, filepath.Join(root, "a"), "755")
	assert.Equal(t, files.CodeExists, files.CodeFromOS(err))

	_, err = runOp(t, nil, OpSymlink, "/nowhere", filepath.Join(root, "a", "link"))
	require.NoError(t, err)
	target, err := os.Readlink(filepath.Join(root, "a", "link"))
	require.NoError(t, err)
	assert.Equal(t, "/nowhere", target)

	require.NoError(t, os.WriteFile(filepath.Join(root, "a", "b", "c", "f"), []byte("f"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "taken"), []byte("t"), 0o644))
	_, err = runOp(t, nil, OpRename, filepath.Join(root, "a", "b", "c", "f"), filepath.Join(root, "taken"), FlagNoReplace)
	assert.Equal(t, files.CodeExists, files.CodeFromOS(err), "noreplace keeps the existing target")
	assert.Equal(t, "t", string(testfs.ReadBytes(t, filepath.Join(root, "taken"))))
	_, err = runOp(t, nil, OpRename, filepath.Join(root, "a"), filepath.Join(root, "moved"), "overwrite")
	assert.Equal(t, files.CodeInvalid, files.CodeOf(err))
	_, err = runOp(t, nil, OpRename, filepath.Join(root, "a"), filepath.Join(root, "moved"))
	require.NoError(t, err)
	assert.Equal(t, "f", string(testfs.ReadBytes(t, filepath.Join(root, "moved", "b", "c", "f"))))

	_, err = runOp(t, nil, OpRemove, filepath.Join(root, "moved"))
	assert.Equal(t, files.CodeExists, files.CodeFromOS(err), "remove refuses a non-empty directory")
	_, err = runOp(t, nil, OpRemove, filepath.Join(root, "moved", "link"))
	require.NoError(t, err)

	outside := filepath.Join(root, "outside")
	require.NoError(t, os.Mkdir(outside, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(outside, "keep"), []byte("k"), 0o644))
	require.NoError(t, os.Symlink(outside, filepath.Join(root, "moved", "escape")))

	_, err = runOp(t, nil, OpRemoveAll, filepath.Join(root, "moved"))
	require.NoError(t, err)
	_, statErr := os.Lstat(filepath.Join(root, "moved"))
	assert.True(t, os.IsNotExist(statErr))
	assert.Equal(t, "k", string(testfs.ReadBytes(t, filepath.Join(outside, "keep"))), "removeall never follows symlinks out of the tree")

	_, err = runOp(t, nil, OpRemoveAll, filepath.Join(root, "moved"))
	require.NoError(t, err, "removeall of a missing path succeeds like os.RemoveAll")
}

func TestRenameatIfAbsentKeepsAnExistingTarget(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "a"), []byte("a"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "b"), []byte("b"), 0o644))
	dir, err := unix.Open(root, unix.O_PATH|unix.O_DIRECTORY, 0)
	require.NoError(t, err)
	defer unix.Close(dir)

	require.ErrorIs(t, renameatIfAbsent(dir, "a", dir, "b"), fs.ErrExist)
	assert.Equal(t, "b", string(testfs.ReadBytes(t, filepath.Join(root, "b"))))

	require.NoError(t, renameatIfAbsent(dir, "a", dir, "c"))
	assert.Equal(t, "a", string(testfs.ReadBytes(t, filepath.Join(root, "c"))))
	assert.NoFileExists(t, filepath.Join(root, "a"))
}

func TestUtimesSetsTheModificationTime(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "f")
	require.NoError(t, os.WriteFile(path, []byte("f"), 0o644))
	link := filepath.Join(root, "l")
	require.NoError(t, os.Symlink(path, link))

	_, err := runOp(t, nil, OpUtimes, path, "1700000000000000000")
	require.NoError(t, err)
	info, err := os.Lstat(path)
	require.NoError(t, err)
	assert.Equal(t, int64(1700000000), info.ModTime().Unix())

	_, err = runOp(t, nil, OpUtimes, link, "1600000000000000000")
	require.NoError(t, err)
	info, err = os.Lstat(path)
	require.NoError(t, err)
	assert.Equal(t, int64(1700000000), info.ModTime().Unix(), "the link itself is stamped, not its target")

	_, err = runOp(t, nil, OpUtimes, path, "soon")
	assert.Equal(t, files.CodeInvalid, files.CodeOf(err))
}

func TestWriteAnnouncesTheCreatedFile(t *testing.T) {
	root := t.TempDir()
	out, err := runOp(t, committed("x"), OpWrite, filepath.Join(root, "f"), "644")
	require.NoError(t, err)
	assert.Equal(t, Ready+"\n", out)

	out, err = runOp(t, committed("x"), OpWrite, filepath.Join(root, "f"), "644")
	require.Error(t, err)
	assert.Empty(t, out, "nothing is announced when the create fails")
}

func TestChmodAndChownWalkWithoutFollowingLinks(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "tree")
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "sub"), 0o755))
	inner := filepath.Join(dir, "sub", "f")
	require.NoError(t, os.WriteFile(inner, []byte("f"), 0o644))
	outside := filepath.Join(root, "outside")
	require.NoError(t, os.WriteFile(outside, []byte("o"), 0o644))
	require.NoError(t, os.Symlink(outside, filepath.Join(dir, "link")))

	_, err := runOp(t, nil, OpChmod, dir, "700", "recursive")
	require.NoError(t, err)
	for _, path := range []string{dir, filepath.Join(dir, "sub"), inner} {
		info, err := os.Lstat(path)
		require.NoError(t, err)
		assert.Equal(t, fs.FileMode(0o700), info.Mode().Perm(), path)
	}
	info, err := os.Lstat(outside)
	require.NoError(t, err)
	assert.Equal(t, fs.FileMode(0o644), info.Mode().Perm(), "a symlink target outside the tree is untouched")

	_, err = runOp(t, nil, OpChmod, inner, "600")
	require.NoError(t, err)
	info, err = os.Lstat(inner)
	require.NoError(t, err)
	assert.Equal(t, fs.FileMode(0o600), info.Mode().Perm())

	_, err = runOp(t, nil, OpChown, dir, "-1", "-1", "recursive")
	require.NoError(t, err, "chown to the current owner walks the tree")
	_, err = runOp(t, nil, OpChown, inner, "-1", "-1")
	require.NoError(t, err)
}

func TestMainRefusesOutsidePkexec(t *testing.T) {
	t.Setenv("PKEXEC_UID", "")
	var stderr bytes.Buffer
	assert.Equal(t, ExitRefused, Main([]string{OpLstat, "/"}, nil, nil, &stderr))
	assert.Contains(t, stderr.String(), "pkexec")

	if os.Getuid() == 0 {
		t.Skip("running as root")
	}
	t.Setenv("PKEXEC_UID", "1000")
	stderr.Reset()
	assert.Equal(t, ExitRefused, Main([]string{OpLstat, "/"}, nil, nil, &stderr))
	assert.Contains(t, stderr.String(), "root")
}

func umask(t *testing.T) int {
	t.Helper()
	mask := unix.Umask(0)
	unix.Umask(mask)
	return mask
}

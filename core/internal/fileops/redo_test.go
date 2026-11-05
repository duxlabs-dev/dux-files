package fileops

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/duxlabs-dev/dux-files/core/internal/testfs"

	"github.com/duxlabs-dev/dux-go/ipc"
	"github.com/stretchr/testify/require"
)

func redo(t *testing.T, m *Manager, bus *testBus, scope string) View {
	t.Helper()
	view, err := m.Redo(scope)
	require.NoError(t, err)
	return bus.wait(t, view.ID)
}

func TestRedoMoveAndRenameReplayTheRecordedPairs(t *testing.T) {
	root := t.TempDir()
	src := testfs.Write(t, filepath.Join(root, "src", "a.txt"), "a")
	dest := filepath.Join(root, "dest")
	require.NoError(t, os.Mkdir(dest, 0o755))
	manager, bus := newHarness(t)

	run(t, manager, bus, spec{kind: KindMove, scope: "w1", sources: []string{src}, dest: dest})
	undone := undo(t, manager, bus, "w1")
	require.Equal(t, "undo", undone.Replay)
	require.True(t, undone.Redoable)
	require.False(t, undone.Undoable)

	redone := redo(t, manager, bus, "w1")
	require.Equal(t, StateDone, redone.State)
	require.Equal(t, "redo", redone.Replay)
	require.True(t, redone.Undoable)
	require.NoFileExists(t, src)
	require.Equal(t, "a", testfs.Read(t, filepath.Join(dest, "a.txt")))

	undo(t, manager, bus, "w1")
	require.FileExists(t, src)

	run(t, manager, bus, spec{kind: KindRename, scope: "w1", sources: []string{src}, name: "b.txt"})
	undo(t, manager, bus, "w1")
	require.FileExists(t, src)
	redo(t, manager, bus, "w1")
	require.NoFileExists(t, src)
	require.Equal(t, "a", testfs.Read(t, filepath.Join(root, "src", "b.txt")))
}

func TestRedoRecreatesWhatAnUndoDeleted(t *testing.T) {
	root := t.TempDir()
	src := testfs.Write(t, filepath.Join(root, "src", "a.txt"), "a")
	template := testfs.Write(t, filepath.Join(root, "templates", "Sheet.csv"), "a,b\n")
	dest := filepath.Join(root, "dest")
	require.NoError(t, os.Mkdir(dest, 0o755))
	manager, bus := newHarness(t)

	cases := []struct {
		name string
		spec spec
		made string
	}{
		{"copy", spec{kind: KindCopy, scope: "w1", sources: []string{src}, dest: dest}, filepath.Join(dest, "a.txt")},
		{"link", spec{kind: KindLink, scope: "w1", sources: []string{src}, dest: dest}, filepath.Join(dest, "a.txt")},
		{"newFolder", spec{kind: KindNewFolder, scope: "w1", dest: dest, name: "Fresh", unique: true}, filepath.Join(dest, "Fresh")},
		{"newFile", spec{kind: KindNewFile, scope: "w1", dest: dest, name: "note.txt", unique: true}, filepath.Join(dest, "note.txt")},
		{"newFile from template", spec{kind: KindNewFile, scope: "w1", dest: dest, name: "Budget.csv", template: template}, filepath.Join(dest, "Budget.csv")},
		{"compress", spec{kind: KindCompress, scope: "w1", sources: []string{src}, dest: dest, name: "bundle", format: FormatZip}, filepath.Join(dest, "bundle.zip")},
		{"duplicate", spec{kind: KindDuplicate, scope: "w1", sources: []string{src}, pairs: []pair{{src: src, dst: src}}}, filepath.Join(root, "src", "a (copy).txt")},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, StateDone, run(t, manager, bus, tc.spec).State)
			_, err := os.Lstat(tc.made)
			require.NoError(t, err)

			undone := undo(t, manager, bus, "w1")
			require.Equal(t, StateDone, undone.State)
			require.True(t, undone.Redoable)
			require.NoFileExists(t, tc.made)

			redone := redo(t, manager, bus, "w1")
			require.Equal(t, StateDone, redone.State)
			_, err = os.Lstat(tc.made)
			require.NoError(t, err)
			require.True(t, redone.Undoable)

			undo(t, manager, bus, "w1")
			require.NoFileExists(t, tc.made)
		})
	}
	require.Equal(t, "a,b\n", testfs.Read(t, template))
}

func TestRedoExtractUnpacksAgain(t *testing.T) {
	root := t.TempDir()
	testfs.Write(t, filepath.Join(root, "project", "README.md"), "hi")
	out := filepath.Join(root, "out")
	require.NoError(t, os.Mkdir(out, 0o755))
	manager, bus := newHarness(t)

	run(t, manager, bus, spec{kind: KindCompress, sources: []string{filepath.Join(root, "project")}, dest: root, name: "project", format: FormatZip})
	archive := filepath.Join(root, "project.zip")
	run(t, manager, bus, spec{kind: KindExtract, scope: "w1", sources: []string{archive}, dest: out})
	unpacked := filepath.Join(out, "project", "README.md")
	require.FileExists(t, unpacked)

	undo(t, manager, bus, "w1")
	require.NoDirExists(t, filepath.Join(out, "project"))

	require.Equal(t, StateDone, redo(t, manager, bus, "w1").State)
	require.Equal(t, "hi", testfs.Read(t, unpacked))
}

func TestRedoTrashAndRestore(t *testing.T) {
	trashHome(t)
	root := t.TempDir()
	src := testfs.Write(t, filepath.Join(root, "a.txt"), "a")
	manager, bus := newHarness(t)

	run(t, manager, bus, spec{kind: KindTrash, scope: "w1", sources: []string{src}})
	undo(t, manager, bus, "w1")
	require.FileExists(t, src)

	require.Equal(t, StateDone, redo(t, manager, bus, "w1").State)
	require.NoFileExists(t, src)

	undo(t, manager, bus, "w1")
	require.FileExists(t, src)

	trashed := run(t, manager, bus, spec{kind: KindTrash, scope: "w2", sources: []string{src}})
	require.Equal(t, StateDone, trashed.State)
	entries, err := listTrash()
	require.NoError(t, err)
	require.Len(t, entries, 1)

	run(t, manager, bus, spec{kind: KindRestore, scope: "w1", sources: []string{entries[0].Name}, trashDir: entries[0].TrashDir})
	require.FileExists(t, src)
	undo(t, manager, bus, "w1")
	require.NoFileExists(t, src)
	require.Equal(t, StateDone, redo(t, manager, bus, "w1").State)
	require.Equal(t, "a", testfs.Read(t, src))
}

func TestRedoIsClearedByANewOperation(t *testing.T) {
	root := t.TempDir()
	src := testfs.Write(t, filepath.Join(root, "src", "a.txt"), "a")
	dest := filepath.Join(root, "dest")
	require.NoError(t, os.Mkdir(dest, 0o755))
	manager, bus := newHarness(t)

	run(t, manager, bus, spec{kind: KindMove, scope: "w1", sources: []string{src}, dest: dest})
	undo(t, manager, bus, "w1")
	_, err := manager.Redo("w2")
	require.Error(t, err)

	run(t, manager, bus, spec{kind: KindNewFolder, scope: "w1", dest: dest, name: "Fresh"})
	_, err = manager.Redo("w1")
	require.Error(t, err)

	undo(t, manager, bus, "w1")
	require.NoDirExists(t, filepath.Join(dest, "Fresh"))
	require.Equal(t, StateDone, redo(t, manager, bus, "w1").State)
	require.DirExists(t, filepath.Join(dest, "Fresh"))
	require.FileExists(t, src)
	_, err = manager.Redo("w1")
	require.Error(t, err)
}

func TestACancelledOrSkippedUndoPushesNoRedo(t *testing.T) {
	root := t.TempDir()
	src := testfs.Write(t, filepath.Join(root, "src", "a.txt"), "a")
	dest := filepath.Join(root, "dest")
	require.NoError(t, os.Mkdir(dest, 0o755))
	manager, bus := newHarness(t)
	bus.onFailure = func(f Failure) Answer { return Answer{ID: f.ID, Action: ActionSkip} }

	run(t, manager, bus, spec{kind: KindCopy, scope: "w1", sources: []string{src}, dest: dest})
	copied := filepath.Join(dest, "a.txt")
	testfs.Write(t, copied, "edited")
	require.NoError(t, os.Chtimes(copied, time.Now().Add(time.Hour), time.Now().Add(time.Hour)))

	undone := undo(t, manager, bus, "w1")
	require.Equal(t, StateFailed, undone.State)
	require.False(t, undone.Redoable)
	_, err := manager.Redo("w1")
	require.Error(t, err)
	require.Equal(t, "edited", testfs.Read(t, copied))

	require.NoError(t, os.Chmod(dest, 0o555))
	t.Cleanup(func() { os.Chmod(dest, 0o755) })
	run(t, manager, bus, spec{kind: KindCopy, scope: "w1", sources: []string{src}, dest: root})
	require.NoError(t, os.Chmod(root, 0o555))
	skipped := undo(t, manager, bus, "w1")
	require.NoError(t, os.Chmod(root, 0o755))
	require.Equal(t, 1, skipped.Skipped)
	require.False(t, skipped.Redoable)
	_, err = manager.Redo("w1")
	require.Error(t, err)
}

func TestARedoFromABeforeANewerOperationIsDropped(t *testing.T) {
	root := t.TempDir()
	src := testfs.Write(t, filepath.Join(root, "src", "a.txt"), "a")
	dest := filepath.Join(root, "dest")
	require.NoError(t, os.Mkdir(dest, 0o755))
	manager, bus := newHarness(t)

	run(t, manager, bus, spec{kind: KindMove, scope: "w1", sources: []string{src}, dest: dest})
	manager.mu.Lock()
	entry, ok := pop(manager.undo, "w1")
	manager.mu.Unlock()
	require.True(t, ok)
	stale := entry.inverse
	stale.scope = "w1"
	stale.replay = replayUndo
	stale.origin = entry.origin
	stale.epoch = 0

	run(t, manager, bus, spec{kind: KindNewFolder, scope: "w1", dest: dest, name: "Fresh"})
	undone := bus.wait(t, manager.enqueue(stale).id)
	require.Equal(t, StateDone, undone.State)
	require.FileExists(t, src)
	require.False(t, undone.Redoable)
	_, err := manager.Redo("w1")
	require.Error(t, err)
}

func TestForgetDropsAScope(t *testing.T) {
	root := t.TempDir()
	src := testfs.Write(t, filepath.Join(root, "src", "a.txt"), "a")
	dest := filepath.Join(root, "dest")
	require.NoError(t, os.Mkdir(dest, 0o755))
	manager, bus := newHarness(t)

	run(t, manager, bus, spec{kind: KindMove, scope: "w1", sources: []string{src}, dest: dest})
	run(t, manager, bus, spec{kind: KindMove, scope: "w2", sources: []string{filepath.Join(dest, "a.txt")}, dest: root})
	undo(t, manager, bus, "w2")

	out := call(t, manager, ipc.Request{Method: "files.op.forget", Params: map[string]any{"scope": "w2"}})
	require.Equal(t, true, out["result"].(map[string]any)["forgotten"])
	_, err := manager.Redo("w2")
	require.Error(t, err)
	_, err = manager.Undo("w2")
	require.Error(t, err)
	require.Equal(t, StateDone, undo(t, manager, bus, "w1").State)
}

func TestProtectedArchiveIsNotRedone(t *testing.T) {
	root := t.TempDir()
	src := testfs.Write(t, filepath.Join(root, "a.txt"), "a")
	manager, bus := newHarness(t)

	run(t, manager, bus, spec{kind: KindCompress, scope: "w1", sources: []string{src}, dest: root, name: "secret", format: FormatZip, password: "pw"})
	manager.mu.Lock()
	require.Nil(t, manager.undo["w1"][0].origin, "the password must not sit in the undo stack")
	manager.mu.Unlock()

	undone := undo(t, manager, bus, "w1")
	require.False(t, undone.Redoable)
	_, err := manager.Redo("w1")
	require.Error(t, err)
}

func TestDuplicateNamesCopiesBesideTheOriginal(t *testing.T) {
	root := t.TempDir()
	file := testfs.Write(t, filepath.Join(root, "a.txt"), "a")
	dir := filepath.Join(root, "docs")
	testfs.Write(t, filepath.Join(dir, "inner.txt"), "i")
	manager, bus := newHarness(t)

	dupe := func() View {
		return run(t, manager, bus, spec{kind: KindDuplicate, scope: "w1", sources: []string{file, dir}, pairs: []pair{{src: file, dst: file}, {src: dir, dst: dir}}})
	}

	first := dupe()
	require.Equal(t, StateDone, first.State)
	require.Equal(t, []Created{{Path: filepath.Join(root, "a (copy).txt")}, {Path: filepath.Join(root, "docs (copy)"), Dir: true}}, first.Created)
	require.Equal(t, "a", testfs.Read(t, filepath.Join(root, "a (copy).txt")))
	require.Equal(t, "i", testfs.Read(t, filepath.Join(root, "docs (copy)", "inner.txt")))
	require.Empty(t, bus.kinds("conflict"))

	second := dupe()
	require.Equal(t, filepath.Join(root, "a (copy 2).txt"), second.Created[0].Path)
	require.Equal(t, filepath.Join(root, "docs (copy 2)"), second.Created[1].Path)

	undo(t, manager, bus, "w1")
	require.NoFileExists(t, filepath.Join(root, "a (copy 2).txt"))
	require.NoDirExists(t, filepath.Join(root, "docs (copy 2)"))
	require.FileExists(t, filepath.Join(root, "a (copy).txt"))
	require.Equal(t, "a", testfs.Read(t, file))
}

func TestNewFolderWithSelectionIsOneUndoStep(t *testing.T) {
	root := t.TempDir()
	a := testfs.Write(t, filepath.Join(root, "a.txt"), "a")
	b := testfs.Write(t, filepath.Join(root, "b.txt"), "b")
	require.NoError(t, os.Mkdir(filepath.Join(root, "Untitled folder"), 0o755))
	manager, bus := newHarness(t)

	view := run(t, manager, bus, spec{kind: KindNewFolderWith, scope: "w1", sources: []string{a, b}, dest: root, name: "Untitled folder", unique: true})
	require.Equal(t, StateDone, view.State)
	folder := filepath.Join(root, "Untitled folder 2")
	require.Equal(t, []Created{{Path: folder, Dir: true}}, view.Created)
	require.True(t, view.Undoable)
	require.Equal(t, "a", testfs.Read(t, filepath.Join(folder, "a.txt")))
	require.Equal(t, "b", testfs.Read(t, filepath.Join(folder, "b.txt")))
	require.NoFileExists(t, a)

	undone := undo(t, manager, bus, "w1")
	require.Equal(t, StateDone, undone.State)
	require.Equal(t, "a", testfs.Read(t, a))
	require.Equal(t, "b", testfs.Read(t, b))
	require.NoDirExists(t, folder)
	_, err := manager.Undo("w1")
	require.Error(t, err)

	redone := redo(t, manager, bus, "w1")
	require.Equal(t, StateDone, redone.State)
	require.Equal(t, folder, redone.Created[0].Path)
	require.Equal(t, "a", testfs.Read(t, filepath.Join(folder, "a.txt")))
	require.NoFileExists(t, b)
}

func TestNewFolderWithSelectionKeepsAFolderSomeoneFilled(t *testing.T) {
	root := t.TempDir()
	a := testfs.Write(t, filepath.Join(root, "a.txt"), "a")
	manager, bus := newHarness(t)
	bus.onFailure = func(f Failure) Answer { return Answer{ID: f.ID, Action: ActionSkip} }

	run(t, manager, bus, spec{kind: KindNewFolderWith, scope: "w1", sources: []string{a}, dest: root, name: "Group"})
	folder := filepath.Join(root, "Group")
	testfs.Write(t, filepath.Join(folder, "later.txt"), "kept")

	undone := undo(t, manager, bus, "w1")
	require.Equal(t, StateDone, undone.State)
	require.Equal(t, "a", testfs.Read(t, a))
	require.Equal(t, "kept", testfs.Read(t, filepath.Join(folder, "later.txt")))
	require.Equal(t, 1, undone.Skipped)
}

type fakeImages struct {
	data []byte
	mime string
}

func (f fakeImages) Image() ([]byte, string, bool) {
	return f.data, f.mime, f.data != nil
}

func TestPasteImageWritesTheClipboardImage(t *testing.T) {
	root := t.TempDir()
	manager, bus := newHarness(t)
	manager.SetImageSource(fakeImages{data: []byte("png-bytes"), mime: "image/png"})

	paste := func() map[string]any {
		out := call(t, manager, ipc.Request{Method: "files.op.pasteImage", Params: map[string]any{"path": root, "name": "Pasted image", "scope": "w1"}})
		return out["result"].(map[string]any)
	}

	out := paste()
	require.Equal(t, true, out["image"])
	job := out["job"].(map[string]any)
	first := bus.wait(t, job["id"].(string))
	require.Equal(t, StateDone, first.State)
	require.Equal(t, []Created{{Path: filepath.Join(root, "Pasted image.png")}}, first.Created)
	require.Equal(t, "png-bytes", testfs.Read(t, filepath.Join(root, "Pasted image.png")))
	require.True(t, first.Undoable)
	require.Empty(t, tempsIn(t, root))

	second := bus.wait(t, paste()["job"].(map[string]any)["id"].(string))
	require.Equal(t, filepath.Join(root, "Pasted image 2.png"), second.Created[0].Path)

	undone := undo(t, manager, bus, "w1")
	require.Equal(t, StateDone, undone.State)
	require.NoFileExists(t, filepath.Join(root, "Pasted image 2.png"))
	require.False(t, undone.Redoable)
	_, err := manager.Redo("w1")
	require.Error(t, err)

	manager.SetImageSource(fakeImages{data: []byte("jpg"), mime: "image/jpeg"})
	third := bus.wait(t, paste()["job"].(map[string]any)["id"].(string))
	require.Equal(t, filepath.Join(root, "Pasted image.jpg"), third.Created[0].Path)

	manager.SetImageSource(fakeImages{data: []byte("ico"), mime: "image/x-icon"})
	fourth := bus.wait(t, paste()["job"].(map[string]any)["id"].(string))
	require.Equal(t, filepath.Join(root, "Pasted image.ico"), fourth.Created[0].Path)
}

func TestPasteImageWithoutAnImageStartsNoJob(t *testing.T) {
	root := t.TempDir()
	manager, _ := newHarness(t)
	manager.SetImageSource(fakeImages{})

	out := call(t, manager, ipc.Request{Method: "files.op.pasteImage", Params: map[string]any{"path": root}})["result"].(map[string]any)
	require.Equal(t, false, out["image"])
	require.NotContains(t, out, "job")
	require.Empty(t, manager.Jobs())

	bad := call(t, manager, ipc.Request{Method: "files.op.pasteImage", Params: map[string]any{"path": "relative"}})
	require.Contains(t, bad, "error")

	unsupported, _ := newHarness(t)
	none := call(t, unsupported, ipc.Request{Method: "files.op.pasteImage", Params: map[string]any{"path": root}})
	require.Contains(t, none, "error")
}

func TestNewMethodsRejectBadParams(t *testing.T) {
	manager, _ := newHarness(t)
	root := t.TempDir()

	cases := []ipc.Request{
		{Method: "files.op.duplicate", Params: map[string]any{"paths": "relative"}},
		{Method: "files.op.duplicate"},
		{Method: "files.op.duplicate", Params: map[string]any{"paths": "/"}},
		{Method: "files.op.newFolderWith", Params: map[string]any{"paths": root + "/a", "path": root}},
		{Method: "files.op.newFolderWith", Params: map[string]any{"paths": root, "path": root + "/inside", "name": "x"}},
		{Method: "files.op.newFolderWith", Params: map[string]any{"paths": root + "/a", "path": "rel", "name": "x"}},
		{Method: "files.op.redo", Params: map[string]any{"scope": "nothing"}},
	}
	for _, req := range cases {
		t.Run(req.Method, func(t *testing.T) {
			out := call(t, manager, req)
			require.Contains(t, out, "error")
			require.Empty(t, manager.Jobs())
		})
	}
}

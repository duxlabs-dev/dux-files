package fileops

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/duxlabs-dev/dux-files/core/internal/testfs"

	"github.com/stretchr/testify/require"
)

func undo(t *testing.T, m *Manager, bus *testBus, scope string) View {
	t.Helper()
	view, err := m.Undo(scope)
	require.NoError(t, err)
	return bus.wait(t, view.ID)
}

func TestUndoMovePutsFilesBack(t *testing.T) {
	root := t.TempDir()
	src := testfs.Write(t, filepath.Join(root, "src", "a.txt"), "a")
	dest := filepath.Join(root, "dest")
	require.NoError(t, os.Mkdir(dest, 0o755))

	manager, bus := newHarness(t)
	run(t, manager, bus, spec{kind: KindMove, scope: "w1", sources: []string{src}, dest: dest})
	require.NoFileExists(t, src)

	view := undo(t, manager, bus, "w1")
	require.Equal(t, StateDone, view.State)
	require.Equal(t, "a", testfs.Read(t, src))
	require.NoFileExists(t, filepath.Join(dest, "a.txt"))
}

func TestUndoCopyDeletesTheCopies(t *testing.T) {
	root := t.TempDir()
	src := testfs.Write(t, filepath.Join(root, "src", "a.txt"), "a")
	dest := filepath.Join(root, "dest")
	require.NoError(t, os.Mkdir(dest, 0o755))

	manager, bus := newHarness(t)
	run(t, manager, bus, spec{kind: KindCopy, scope: "w1", sources: []string{src}, dest: dest})

	view := undo(t, manager, bus, "w1")
	require.Equal(t, StateDone, view.State)
	require.NoFileExists(t, filepath.Join(dest, "a.txt"))
	require.Equal(t, "a", testfs.Read(t, src))
}

func TestUndoTrashRestores(t *testing.T) {
	trashHome(t)
	root := t.TempDir()
	src := testfs.Write(t, filepath.Join(root, "a.txt"), "a")

	manager, bus := newHarness(t)
	run(t, manager, bus, spec{kind: KindTrash, scope: "w1", sources: []string{src}})
	require.NoFileExists(t, src)

	view := undo(t, manager, bus, "w1")
	require.Equal(t, StateDone, view.State)
	require.Equal(t, "a", testfs.Read(t, src))
}

func TestUndoStopsWhenTargetChanged(t *testing.T) {
	root := t.TempDir()
	src := testfs.Write(t, filepath.Join(root, "src", "a.txt"), "a")
	dest := filepath.Join(root, "dest")
	require.NoError(t, os.Mkdir(dest, 0o755))

	manager, bus := newHarness(t)
	run(t, manager, bus, spec{kind: KindMove, scope: "w1", sources: []string{src}, dest: dest})

	moved := filepath.Join(dest, "a.txt")
	testfs.Write(t, moved, "edited since the move")
	require.NoError(t, os.Chtimes(moved, time.Now().Add(time.Hour), time.Now().Add(time.Hour)))

	view := undo(t, manager, bus, "w1")
	require.Equal(t, StateFailed, view.State)
	require.Contains(t, view.Error, "changed since")
	require.Equal(t, "edited since the move", testfs.Read(t, moved))
	require.NoFileExists(t, src)
}

func TestUndoIsPerScopeAndEmptiesOut(t *testing.T) {
	root := t.TempDir()
	src := testfs.Write(t, filepath.Join(root, "src", "a.txt"), "a")
	dest := filepath.Join(root, "dest")
	require.NoError(t, os.Mkdir(dest, 0o755))

	manager, bus := newHarness(t)
	run(t, manager, bus, spec{kind: KindMove, scope: "w1", sources: []string{src}, dest: dest})

	_, err := manager.Undo("w2")
	require.Error(t, err)

	undo(t, manager, bus, "w1")
	_, err = manager.Undo("w1")
	require.Error(t, err)
}

func TestUndoOfAnUndoIsNotStacked(t *testing.T) {
	root := t.TempDir()
	src := testfs.Write(t, filepath.Join(root, "src", "a.txt"), "a")
	dest := filepath.Join(root, "dest")
	require.NoError(t, os.Mkdir(dest, 0o755))

	manager, bus := newHarness(t)
	run(t, manager, bus, spec{kind: KindMove, scope: "w1", sources: []string{src}, dest: dest})
	undone := undo(t, manager, bus, "w1")
	require.False(t, undone.Undoable)

	_, err := manager.Undo("w1")
	require.Error(t, err)
}

func TestNewFolderRenameLinkAndPermissions(t *testing.T) {
	root := t.TempDir()
	manager, bus := newHarness(t)

	run(t, manager, bus, spec{kind: KindNewFolder, scope: "w1", dest: root, name: "Fresh"})
	require.DirExists(t, filepath.Join(root, "Fresh"))

	run(t, manager, bus, spec{kind: KindNewFile, scope: "w1", dest: root, name: "note.txt"})
	require.FileExists(t, filepath.Join(root, "note.txt"))

	run(t, manager, bus, spec{kind: KindRename, scope: "w1", sources: []string{filepath.Join(root, "note.txt")}, name: "renamed.txt"})
	require.FileExists(t, filepath.Join(root, "renamed.txt"))
	require.NoFileExists(t, filepath.Join(root, "note.txt"))

	undo(t, manager, bus, "w1")
	require.FileExists(t, filepath.Join(root, "note.txt"))

	run(t, manager, bus, spec{kind: KindLink, sources: []string{filepath.Join(root, "note.txt")}, dest: filepath.Join(root, "Fresh")})
	target, err := os.Readlink(filepath.Join(root, "Fresh", "note.txt"))
	require.NoError(t, err)
	require.Equal(t, filepath.Join(root, "note.txt"), target)

	run(t, manager, bus, spec{kind: KindSetPermissions, sources: []string{filepath.Join(root, "note.txt")}, mode: 0o600})
	info, err := os.Lstat(filepath.Join(root, "note.txt"))
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o600), info.Mode().Perm())
}

func TestNewFileFromTemplate(t *testing.T) {
	root := t.TempDir()
	template := testfs.Write(t, filepath.Join(root, "templates", "Sheet.csv"), "a,b\n")

	manager, bus := newHarness(t)
	view := run(t, manager, bus, spec{kind: KindNewFile, dest: root, name: "Budget.csv", template: template})

	require.Equal(t, StateDone, view.State)
	require.Equal(t, "a,b\n", testfs.Read(t, filepath.Join(root, "Budget.csv")))
}

func TestRenameRefusesAnExistingName(t *testing.T) {
	root := t.TempDir()
	testfs.Write(t, filepath.Join(root, "a.txt"), "a")
	testfs.Write(t, filepath.Join(root, "b.txt"), "b")

	manager, bus := newHarness(t)
	view := run(t, manager, bus, spec{kind: KindRename, sources: []string{filepath.Join(root, "a.txt")}, name: "b.txt"})

	require.Equal(t, StateFailed, view.State)
	require.Equal(t, "b", testfs.Read(t, filepath.Join(root, "b.txt")))
}

func TestPermanentDeleteRemovesTrees(t *testing.T) {
	root := t.TempDir()
	testfs.Write(t, filepath.Join(root, "dir", "a.txt"), "a")

	manager, bus := newHarness(t)
	view := run(t, manager, bus, spec{kind: KindDelete, scope: "w1", sources: []string{filepath.Join(root, "dir")}})

	require.Equal(t, StateDone, view.State)
	require.NoDirExists(t, filepath.Join(root, "dir"))

	_, err := manager.Undo("w1")
	require.Error(t, err, "permanent delete is not undoable")
}

func TestNewFileRefusesAnExistingName(t *testing.T) {
	root := t.TempDir()
	testfs.Write(t, filepath.Join(root, "a.txt"), "keep")

	manager, bus := newHarness(t)
	view := run(t, manager, bus, spec{kind: KindNewFile, dest: root, name: "a.txt"})

	require.Equal(t, StateFailed, view.State)
	require.Equal(t, "keep", testfs.Read(t, filepath.Join(root, "a.txt")))
}

func TestNewItemsReportWhatTheyCreated(t *testing.T) {
	root := t.TempDir()
	template := testfs.Write(t, filepath.Join(root, "templates", "Sheet.csv"), "a,b\n")
	manager, bus := newHarness(t)

	folder := run(t, manager, bus, spec{kind: KindNewFolder, scope: "w1", dest: root, name: "Fresh"})
	require.Equal(t, []Created{{Path: filepath.Join(root, "Fresh"), Dir: true}}, folder.Created)

	file := run(t, manager, bus, spec{kind: KindNewFile, scope: "w1", dest: root, name: "note.txt"})
	require.Equal(t, []Created{{Path: filepath.Join(root, "note.txt")}}, file.Created)

	copied := run(t, manager, bus, spec{kind: KindNewFile, scope: "w1", dest: root, name: "Budget.csv", template: template})
	require.Equal(t, []Created{{Path: filepath.Join(root, "Budget.csv")}}, copied.Created)
	require.True(t, copied.Undoable)
}

func TestUniqueNewItemsAreNumbered(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(root, "Untitled folder"), 0o755))
	require.NoError(t, os.Mkdir(filepath.Join(root, "Untitled folder 2"), 0o755))
	testfs.Write(t, filepath.Join(root, "notes.txt"), "kept")
	testfs.Write(t, filepath.Join(root, ".env"), "kept")
	manager, bus := newHarness(t)

	folder := run(t, manager, bus, spec{kind: KindNewFolder, dest: root, name: "Untitled folder", unique: true})
	require.Equal(t, StateDone, folder.State)
	require.Equal(t, filepath.Join(root, "Untitled folder 3"), folder.Created[0].Path)

	file := run(t, manager, bus, spec{kind: KindNewFile, dest: root, name: "notes.txt", unique: true})
	require.Equal(t, filepath.Join(root, "notes 2.txt"), file.Created[0].Path)
	require.Equal(t, "kept", testfs.Read(t, filepath.Join(root, "notes.txt")))

	dotfile := run(t, manager, bus, spec{kind: KindNewFile, dest: root, name: ".env", unique: true})
	require.Equal(t, filepath.Join(root, ".env 2"), dotfile.Created[0].Path)

	free := run(t, manager, bus, spec{kind: KindNewFolder, dest: root, name: "Other", unique: true})
	require.Equal(t, filepath.Join(root, "Other"), free.Created[0].Path)
}

func TestNewFileWithoutUniqueStillRefusesATakenName(t *testing.T) {
	root := t.TempDir()
	testfs.Write(t, filepath.Join(root, "notes.txt"), "kept")
	manager, bus := newHarness(t)

	view := run(t, manager, bus, spec{kind: KindNewFile, dest: root, name: "notes.txt"})
	require.Equal(t, StateFailed, view.State)
	require.Empty(t, view.Created)
	require.NoFileExists(t, filepath.Join(root, "notes 2.txt"))
}

func TestUniqueNameMovesOnWhenAnotherJobTookIt(t *testing.T) {
	root := t.TempDir()
	r := &runner{job: &job{spec: spec{dest: root, name: "Untitled folder", unique: true}}}
	path := filepath.Join(root, "Untitled folder")

	raced := false
	err := r.createUnique(&path, func(target string) error {
		if !raced {
			raced = true
			require.NoError(t, os.Mkdir(target, 0o755))
		}
		return os.Mkdir(target, 0o755)
	})

	require.NoError(t, err)
	require.Equal(t, filepath.Join(root, "Untitled folder 2"), path)
	require.DirExists(t, path)
}

func TestUndoIsNotOfferedAfterReplacing(t *testing.T) {
	root := t.TempDir()
	src := testfs.Write(t, filepath.Join(root, "src", "a.txt"), "new")
	dest := filepath.Join(root, "dest")
	testfs.Write(t, filepath.Join(dest, "a.txt"), "irreplaceable")

	manager, bus := newHarness(t)
	bus.onConflict = func(c Conflict) Answer { return Answer{ID: c.ID, Action: ActionReplace} }
	view := run(t, manager, bus, spec{kind: KindCopy, scope: "w1", sources: []string{src}, dest: dest})

	require.Equal(t, StateDone, view.State)
	require.False(t, view.Undoable)
	require.Equal(t, "new", testfs.Read(t, filepath.Join(dest, "a.txt")))

	_, err := manager.Undo("w1")
	require.Error(t, err, "undoing would delete the copy and the replaced file is already gone")
}

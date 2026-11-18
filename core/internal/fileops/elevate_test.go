package fileops

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/duxlabs-dev/dux-files/core/internal/testfs"

	"github.com/duxlabs-dev/dux-go/files"
	"github.com/duxlabs-dev/dux-go/ipc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeElevator stands in for the pkexec helper: it lifts the mode bits the
// test locked down, then runs the plain os call, and counts what it did.
type fakeElevator struct {
	localFS
	unavailable error
	opens       int
	creates     int
	chowns      []string
	failOpen    error
}

func (f *fakeElevator) Route() string    { return "fake" }
func (f *fakeElevator) Available() error { return f.unavailable }

func (f *fakeElevator) Open(path string) (io.ReadCloser, error) {
	f.opens++
	if f.failOpen != nil {
		return nil, f.failOpen
	}
	_ = os.Chmod(path, 0o600)
	return os.Open(path)
}

func (f *fakeElevator) Create(path string, perm os.FileMode) (Writer, error) {
	f.creates++
	_ = os.Chmod(filepath.Dir(path), 0o700)
	return f.localFS.Create(path, perm)
}

func (f *fakeElevator) Lchown(path string, uid, gid int, recursive bool) error {
	f.chowns = append(f.chowns, path+" "+strconv.Itoa(uid)+":"+strconv.Itoa(gid)+" "+strconv.FormatBool(recursive))
	return nil
}

func denied() error {
	return &fs.PathError{Op: "open", Path: "x", Err: fs.ErrPermission}
}

func TestDeniedErrorOffersElevationOnlyWhenARouteExists(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root ignores mode bits")
	}
	for name, elevator := range map[string]*fakeElevator{
		"no route":          nil,
		"route unavailable": {unavailable: &Unavailable{Reason: "NOPOLICY"}},
	} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			src := testfs.Write(t, filepath.Join(root, "a.txt"), "a")
			require.NoError(t, os.Chmod(src, 0o000))

			manager, bus := newHarness(t)
			if elevator != nil {
				manager.SetElevator(elevator)
			}
			var seen Failure
			bus.onFailure = func(f Failure) Answer {
				seen = f
				return Answer{ID: f.ID, Action: ActionCancel}
			}
			run(t, manager, bus, spec{kind: KindCopy, sources: []string{src}, dest: filepath.Join(root, "dest")})

			assert.Equal(t, "EACCES", seen.Code)
			assert.False(t, seen.Escalatable)
			assert.NotContains(t, seen.Actions, ActionRetryElevated)
		})
	}
}

func TestRetryElevatedRunsTheStepThroughTheElevator(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root ignores mode bits")
	}
	root := t.TempDir()
	src := testfs.Write(t, filepath.Join(root, "a.txt"), "secret")
	require.NoError(t, os.Chmod(src, 0o000))
	dest := filepath.Join(root, "dest")
	require.NoError(t, os.Mkdir(dest, 0o755))

	manager, bus := newHarness(t)
	elevator := &fakeElevator{}
	manager.SetElevator(elevator)
	var seen []Failure
	bus.onFailure = func(f Failure) Answer {
		seen = append(seen, f)
		return Answer{ID: f.ID, Action: ActionRetryElevated}
	}
	view := run(t, manager, bus, spec{kind: KindCopy, sources: []string{src}, dest: dest})

	require.Equal(t, StateDone, view.State, view.Error)
	require.Len(t, seen, 1)
	assert.True(t, seen[0].Escalatable)
	assert.Equal(t, []Action{ActionRetryElevated, ActionRetry, ActionSkip, ActionCancel}, seen[0].Actions)
	assert.Equal(t, 1, elevator.opens)
	copied := filepath.Join(dest, "a.txt")
	info, err := os.Lstat(copied)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0), info.Mode().Perm(), "the source mode is preserved")
	require.NoError(t, os.Chmod(copied, 0o600))
	assert.Equal(t, "secret", testfs.Read(t, copied))
	assert.Empty(t, tempsIn(t, dest))
	assert.True(t, view.Undoable, "an elevated read into a normal folder is undone like any copy")
}

func TestElevatedWriteIntoALockedFolder(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root ignores mode bits")
	}
	root := t.TempDir()
	src := testfs.Write(t, filepath.Join(root, "a.txt"), "payload")
	dest := filepath.Join(root, "locked")
	require.NoError(t, os.Mkdir(dest, 0o500))
	t.Cleanup(func() { _ = os.Chmod(dest, 0o700) })

	manager, bus := newHarness(t)
	elevator := &fakeElevator{}
	manager.SetElevator(elevator)
	bus.onFailure = func(f Failure) Answer {
		return Answer{ID: f.ID, Action: ActionRetryElevated}
	}
	view := run(t, manager, bus, spec{kind: KindCopy, sources: []string{src}, dest: dest})

	require.Equal(t, StateDone, view.State, view.Error)
	assert.Equal(t, 1, elevator.creates, "the temp file was created through the route")
	assert.Equal(t, "payload", testfs.Read(t, filepath.Join(dest, "a.txt")))
	assert.Equal(t, 0, view.Warnings, "mode and mtime are preserved through the route")
	assert.Empty(t, tempsIn(t, dest))
}

func TestUnverifiedTargetIsNeverReplaced(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root ignores mode bits")
	}
	root := t.TempDir()
	src := testfs.Write(t, filepath.Join(root, "a.txt"), "new")
	dest := filepath.Join(root, "unlistable")
	testfs.Write(t, filepath.Join(dest, "a.txt"), "old")
	require.NoError(t, os.Chmod(dest, 0o200))
	t.Cleanup(func() { _ = os.Chmod(dest, 0o700) })

	manager, bus := newHarness(t)
	manager.SetElevator(&fakeElevator{})
	var codes []string
	bus.onFailure = func(f Failure) Answer {
		codes = append(codes, f.Code)
		if f.Code == "EACCES" {
			return Answer{ID: f.ID, Action: ActionRetryElevated}
		}
		return Answer{ID: f.ID, Action: ActionSkip}
	}
	view := run(t, manager, bus, spec{kind: KindCopy, sources: []string{src}, dest: dest})

	require.Equal(t, StateDone, view.State, view.Error)
	assert.Equal(t, []string{"EACCES", "EEXIST"}, codes)
	assert.Equal(t, "old", testfs.Read(t, filepath.Join(dest, "a.txt")))
	assert.Equal(t, 1, view.Skipped)
	assert.Empty(t, tempsIn(t, dest))
	assert.False(t, view.Undoable)
}

func TestOneElevatedAnswerCoversTheRestOfTheJob(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root ignores mode bits")
	}
	root := t.TempDir()
	first := testfs.Write(t, filepath.Join(root, "a.txt"), "a")
	second := testfs.Write(t, filepath.Join(root, "b.txt"), "b")
	require.NoError(t, os.Chmod(first, 0o000))
	require.NoError(t, os.Chmod(second, 0o000))
	dest := filepath.Join(root, "dest")
	require.NoError(t, os.Mkdir(dest, 0o755))

	manager, bus := newHarness(t)
	elevator := &fakeElevator{}
	manager.SetElevator(elevator)
	asked := 0
	bus.onFailure = func(f Failure) Answer {
		asked++
		return Answer{ID: f.ID, Action: ActionRetryElevated}
	}
	view := run(t, manager, bus, spec{kind: KindCopy, sources: []string{first, second}, dest: dest})

	require.Equal(t, StateDone, view.State, view.Error)
	assert.Equal(t, 1, asked, "the choice is per job")
	assert.Equal(t, 2, elevator.opens)
	assert.Equal(t, 0, view.Skipped)
}

func TestStickyRetryElevatedDoesNotLoopWhenTheElevatedStepFails(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root ignores mode bits")
	}
	root := t.TempDir()
	first := testfs.Write(t, filepath.Join(root, "a.txt"), "a")
	second := testfs.Write(t, filepath.Join(root, "b.txt"), "b")
	require.NoError(t, os.Chmod(first, 0o000))
	require.NoError(t, os.Chmod(second, 0o000))
	dest := filepath.Join(root, "dest")
	require.NoError(t, os.Mkdir(dest, 0o755))

	manager, bus := newHarness(t)
	elevator := &fakeElevator{failOpen: denied()}
	manager.SetElevator(elevator)
	var seen []Failure
	bus.onFailure = func(f Failure) Answer {
		seen = append(seen, f)
		if len(seen) == 1 {
			return Answer{ID: f.ID, Action: ActionRetryElevated, ApplyToAll: true}
		}
		return Answer{ID: f.ID, Action: ActionSkip, ApplyToAll: true}
	}
	view := run(t, manager, bus, spec{kind: KindCopy, sources: []string{first, second}, dest: dest})

	require.Equal(t, StateDone, view.State, view.Error)
	require.Len(t, seen, 2, "the elevated failure is shown once, then the sticky skip covers the rest")
	assert.True(t, seen[1].Escalatable, "a dismissed prompt can be retried as administrator again")
	assert.Equal(t, 1, elevator.opens, "the sticky skip replaced the sticky elevation")
	assert.Equal(t, 2, view.Skipped)
	assert.Empty(t, tempsIn(t, dest))
}

func TestTrashHasNoElevatedForm(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root ignores mode bits")
	}
	trashHome(t)
	root := t.TempDir()
	locked := filepath.Join(root, "locked")
	victim := testfs.Write(t, filepath.Join(locked, "a.txt"), "a")
	require.NoError(t, os.Chmod(locked, 0o500))
	t.Cleanup(func() { _ = os.Chmod(locked, 0o700) })

	manager, bus := newHarness(t)
	manager.SetElevator(&fakeElevator{})
	var seen Failure
	bus.onFailure = func(f Failure) Answer {
		seen = f
		return Answer{ID: f.ID, Action: ActionCancel}
	}
	run(t, manager, bus, spec{kind: KindTrash, sources: []string{victim}})

	assert.Equal(t, "EACCES", seen.Code)
	assert.False(t, seen.Escalatable)
}

func TestSetOwnerResolvesNamesAndNumbers(t *testing.T) {
	root := t.TempDir()
	target := testfs.Write(t, filepath.Join(root, "a.txt"), "a")
	me, err := user.Current()
	require.NoError(t, err)
	group, err := user.LookupGroupId(me.Gid)
	require.NoError(t, err)

	manager, bus := newHarness(t)
	out := call(t, manager, ipc.Request{ID: 1, Method: "files.op.setOwner", Params: map[string]any{
		"paths": []any{target},
		"owner": me.Username,
		"group": group.Name,
	}})
	job := out["result"].(map[string]any)["job"].(map[string]any)
	view := bus.wait(t, job["id"].(string))
	assert.Equal(t, StateDone, view.State, view.Error)
	assert.Equal(t, KindSetOwner, view.Kind)
	assert.False(t, view.Undoable)

	out = call(t, manager, ipc.Request{ID: 2, Method: "files.op.setOwner", Params: map[string]any{
		"paths": []any{target},
		"owner": float64(os.Getuid()),
		"group": float64(os.Getgid()),
	}})
	job = out["result"].(map[string]any)["job"].(map[string]any)
	view = bus.wait(t, job["id"].(string))
	assert.Equal(t, StateDone, view.State, "numeric ids are accepted as JSON numbers")

	for name, params := range map[string]map[string]any{
		"no owner or group": {"paths": []any{target}},
		"unknown owner":     {"paths": []any{target}, "owner": "nobody-such-user-dfiles"},
		"relative path":     {"paths": "a.txt", "owner": "0"},
		"uid sentinel":      {"paths": []any{target}, "owner": "4294967295"},
		"uid overflow":      {"paths": []any{target}, "group": "99999999999"},
	} {
		t.Run(name, func(t *testing.T) {
			out := call(t, manager, ipc.Request{Method: "files.op.setOwner", Params: params})
			assert.Contains(t, out, "error")
			assert.Equal(t, string(files.CodeInvalid), out["code"])
		})
	}
}

func TestSetOwnerToAnotherUserIsElevated(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root can chown freely")
	}
	root := t.TempDir()
	target := testfs.Write(t, filepath.Join(root, "a.txt"), "a")

	manager, bus := newHarness(t)
	elevator := &fakeElevator{}
	manager.SetElevator(elevator)
	bus.onFailure = func(f Failure) Answer {
		return Answer{ID: f.ID, Action: ActionRetryElevated}
	}
	view := run(t, manager, bus, spec{kind: KindSetOwner, sources: []string{target}, uid: 0, gid: -1, recursive: true})

	require.Equal(t, StateDone, view.State, view.Error)
	assert.Equal(t, []string{target + " 0:-1 true"}, elevator.chowns)
}

func TestRespondAcceptsRetryElevated(t *testing.T) {
	manager, _ := newHarness(t)
	out := call(t, manager, ipc.Request{Method: "files.op.respond", Params: map[string]any{"jobId": "j1", "id": "e1", "action": "retryElevated"}})
	assert.NotContains(t, out["error"], "unknown action")

	assert.Equal(t, Elevation{Reason: "NOROUTE"}, manager.Elevation())
	manager.SetElevator(&fakeElevator{unavailable: &Unavailable{Reason: "NOPOLICY"}})
	assert.Equal(t, Elevation{Route: "fake", Reason: "NOPOLICY"}, manager.Elevation())
	manager.SetElevator(&fakeElevator{unavailable: errors.New("odd")})
	assert.Equal(t, Elevation{Route: "fake", Reason: "odd"}, manager.Elevation())
	manager.SetElevator(&fakeElevator{})
	assert.Equal(t, Elevation{Available: true, Route: "fake"}, manager.Elevation())
}

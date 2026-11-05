package fileops

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/duxlabs-dev/dux-files/core/internal/testfs"

	"github.com/duxlabs-dev/dux-files/core/internal/ipctest"

	"github.com/duxlabs-dev/dux-go/ipc"
	"github.com/stretchr/testify/require"
)

func call(t *testing.T, m *Manager, req ipc.Request) map[string]any {
	t.Helper()
	return ipctest.Call(t, func(w *ipc.ConnWriter) { m.Handle(context.Background(), w, req) })
}

func TestHandleRejectsBadParams(t *testing.T) {
	manager, _ := newHarness(t)

	cases := []struct {
		name   string
		req    ipc.Request
		reason string
	}{
		{"relative source", ipc.Request{Method: "files.op.copy", Params: map[string]any{"paths": "relative/a.txt", "dest": "/tmp"}}, "paths"},
		{"relative dest", ipc.Request{Method: "files.op.move", Params: map[string]any{"paths": "/tmp/a", "dest": "dest"}}, "dest"},
		{"rename with a slash", ipc.Request{Method: "files.op.rename", Params: map[string]any{"path": "/tmp/a", "name": "b/c"}}, "name"},
		{"rename to dotdot", ipc.Request{Method: "files.op.rename", Params: map[string]any{"path": "/tmp/a", "name": ".."}}, "name"},
		{"mode that is not octal", ipc.Request{Method: "files.op.setPermissions", Params: map[string]any{"paths": "/tmp/a", "mode": "9x9"}}, "mode"},
		{"unknown method", ipc.Request{Method: "files.op.nope"}, "unknown"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out := call(t, manager, tc.req)
			require.Contains(t, out, "error")
			require.Contains(t, out, "code")
			require.Empty(t, manager.Jobs())
		})
	}
}

func TestHandleStartsAJobAndLists(t *testing.T) {
	root := t.TempDir()
	src := testfs.Write(t, filepath.Join(root, "a.txt"), "a")
	dest := filepath.Join(root, "dest")
	testfs.Write(t, filepath.Join(dest, "keep"), "")

	manager, bus := newHarness(t)
	out := call(t, manager, ipc.Request{ID: 3, Method: "files.op.copy", Params: map[string]any{
		"paths": []any{src},
		"dest":  dest,
		"scope": "w1",
	}})

	result := out["result"].(map[string]any)
	job := result["job"].(map[string]any)
	require.Equal(t, "copy", job["kind"])
	bus.wait(t, job["id"].(string))

	listed := call(t, manager, ipc.Request{Method: "files.op.jobs"})["result"].(map[string]any)["jobs"].([]any)
	require.Len(t, listed, 1)
	require.Equal(t, "done", listed[0].(map[string]any)["state"])
	require.Equal(t, "a", testfs.Read(t, filepath.Join(dest, "a.txt")))
}

func TestHandleRespondNeedsTheCurrentAnswerID(t *testing.T) {
	root := t.TempDir()
	src := testfs.Write(t, filepath.Join(root, "src", "a.txt"), "new")
	dest := filepath.Join(root, "dest")
	testfs.Write(t, filepath.Join(dest, "a.txt"), "old")

	manager, bus := newHarness(t)
	paused := make(chan struct{}, 1)
	bus.onConflict = func(c Conflict) Answer {
		require.Error(t, manager.Respond(c.ID, Answer{ID: c.ID, Action: ActionSkip}), "unknown job id")
		paused <- struct{}{}
		return Answer{ID: "stale", Action: ActionSkip}
	}

	job := manager.enqueue(spec{kind: KindCopy, sources: []string{src}, dest: dest})
	<-paused
	require.Equal(t, StatePausedConflict, job.view().State)

	require.NoError(t, manager.Cancel(job.id))
	view := bus.wait(t, job.id)
	require.True(t, view.Cancelled)
	require.Equal(t, "old", testfs.Read(t, filepath.Join(dest, "a.txt")))
}

func TestHandleCancelAndUndoErrors(t *testing.T) {
	manager, _ := newHarness(t)

	out := call(t, manager, ipc.Request{Method: "files.op.cancel", Params: map[string]any{"jobId": "j404"}})
	require.Contains(t, out, "error")

	out = call(t, manager, ipc.Request{Method: "files.op.undo", Params: map[string]any{"scope": "w1"}})
	require.Contains(t, out, "error")

	out = call(t, manager, ipc.Request{Method: "files.op.respond", Params: map[string]any{"jobId": "j404", "id": "c1", "action": "banana"}})
	require.Contains(t, out, "error")
}

func TestJobsListingIsSafeWhileAJobRecords(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(root, "src")
	for i := range 40 {
		testfs.Write(t, filepath.Join(src, "f"+strconv.Itoa(i)+".txt"), "body")
	}
	dest := filepath.Join(root, "dest")
	require.NoError(t, os.Mkdir(dest, 0o755))

	manager, bus := newHarness(t)
	sources := make([]string, 0, 40)
	for i := range 40 {
		sources = append(sources, filepath.Join(src, "f"+strconv.Itoa(i)+".txt"))
	}
	job := manager.enqueue(spec{kind: KindCopy, scope: "w1", sources: sources, dest: dest})

	done := make(chan struct{})
	go func() {
		defer close(done)
		for range 200 {
			manager.Jobs()
		}
	}()

	bus.wait(t, job.id)
	<-done
	require.FileExists(t, filepath.Join(dest, "f39.txt"))
}

func TestDropActionOnTheSameDeviceIsAMove(t *testing.T) {
	manager, _ := newHarness(t)
	root := t.TempDir()
	src := filepath.Join(root, "a.txt")
	require.NoError(t, os.WriteFile(src, []byte("a"), 0o644))
	dest := filepath.Join(root, "sub")
	require.NoError(t, os.Mkdir(dest, 0o755))

	result := call(t, manager, ipc.Request{Method: "files.op.dropAction", Params: map[string]any{"paths": src, "dest": dest}})["result"].(map[string]any)
	require.Equal(t, "move", result["action"])
	require.Equal(t, true, result["sameDevice"])
}

func TestDropActionRejectsBadParams(t *testing.T) {
	manager, _ := newHarness(t)
	for _, params := range []map[string]any{
		{"paths": "relative/a.txt", "dest": t.TempDir()},
		{"paths": filepath.Join(t.TempDir(), "a"), "dest": "relative"},
	} {
		out := call(t, manager, ipc.Request{Method: "files.op.dropAction", Params: params})
		require.NotEmpty(t, out["error"])
	}
}

func TestHandleNewFolderPassesUnique(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(root, "Untitled folder"), 0o755))

	manager, bus := newHarness(t)
	out := call(t, manager, ipc.Request{Method: "files.op.newFolder", Params: map[string]any{
		"path":   root,
		"name":   "Untitled folder",
		"unique": true,
	}})

	job := out["result"].(map[string]any)["job"].(map[string]any)
	view := bus.wait(t, job["id"].(string))
	require.Equal(t, StateDone, view.State)
	require.DirExists(t, filepath.Join(root, "Untitled folder 2"))
}

func TestDuplicateAnswerDoesNotLeakIntoTheNextPrompt(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(root, "src")
	testfs.Write(t, filepath.Join(src, "a.txt"), "new")
	testfs.Write(t, filepath.Join(src, "b.txt"), "new")
	dest := filepath.Join(root, "dest")
	testfs.Write(t, filepath.Join(dest, "a.txt"), "old")
	testfs.Write(t, filepath.Join(dest, "b.txt"), "old")

	manager, bus := newHarness(t)
	var asked []string
	bus.onConflict = func(c Conflict) Answer {
		asked = append(asked, c.ID)
		var wg sync.WaitGroup
		for range 4 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_ = manager.Respond(c.ID, Answer{ID: c.ID, Action: ActionReplace})
			}()
		}
		wg.Wait()
		return Answer{ID: c.ID, Action: ActionSkip}
	}

	view := run(t, manager, bus, spec{
		kind:    KindCopy,
		sources: []string{filepath.Join(src, "a.txt"), filepath.Join(src, "b.txt")},
		dest:    dest,
	})

	require.Equal(t, StateDone, view.State)
	require.Len(t, asked, 2, "both files must raise their own prompt")
	require.Equal(t, "old", testfs.Read(t, filepath.Join(dest, "a.txt")))
	require.Equal(t, "old", testfs.Read(t, filepath.Join(dest, "b.txt")))
}

func TestAnswerForAnotherJobIsRefused(t *testing.T) {
	root := t.TempDir()
	src := testfs.Write(t, filepath.Join(root, "src", "a.txt"), "new")
	dest := filepath.Join(root, "dest")
	testfs.Write(t, filepath.Join(dest, "a.txt"), "old")

	manager, bus := newHarness(t)
	bus.onConflict = func(c Conflict) Answer {
		require.Error(t, manager.Respond("j404", Answer{ID: c.ID, Action: ActionReplace}))
		require.Error(t, manager.Respond("j1", Answer{ID: "c99", Action: ActionReplace}))
		return Answer{ID: c.ID, Action: ActionSkip}
	}
	view := run(t, manager, bus, spec{kind: KindCopy, sources: []string{src}, dest: dest})

	require.Equal(t, StateDone, view.State)
	require.Equal(t, "old", testfs.Read(t, filepath.Join(dest, "a.txt")))
}

func TestCancelAfterTheJobFinishedKeepsTheTerminalState(t *testing.T) {
	root := t.TempDir()
	src := testfs.Write(t, filepath.Join(root, "src", "a.txt"), "a")
	dest := filepath.Join(root, "dest")
	require.NoError(t, os.Mkdir(dest, 0o755))

	manager, bus := newHarness(t)
	job := manager.enqueue(spec{kind: KindCopy, sources: []string{src}, dest: dest})

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if err := manager.Cancel(job.id); err != nil {
			break
		}
	}
	view := bus.wait(t, job.id)

	listed := manager.Jobs()
	require.Len(t, listed, 1)
	require.Contains(t, []State{StateDone, StateFailed}, listed[0].State)
	require.Equal(t, view.State, listed[0].State)
}

func TestCopyIntoItsOwnSubdirectoryIsRefused(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(root, "src")
	testfs.Write(t, filepath.Join(src, "a.txt"), "a")
	inner := filepath.Join(src, "inner")
	require.NoError(t, os.Mkdir(inner, 0o755))

	manager, _ := newHarness(t)
	for _, method := range []string{"files.op.copy", "files.op.move"} {
		out := call(t, manager, ipc.Request{Method: method, Params: map[string]any{
			"paths": []any{src},
			"dest":  inner,
		}})
		require.Contains(t, out, "error", method)
		require.Equal(t, "EINVAL", out["code"], method)
	}
	require.Empty(t, manager.Jobs())
}

func TestRelativePathInAListIsRefused(t *testing.T) {
	manager, _ := newHarness(t)

	out := call(t, manager, ipc.Request{Method: "files.op.trash", Params: map[string]any{
		"paths": []any{"/home/someone/a.txt", "b.txt"},
	}})
	require.Contains(t, out, "error")
	require.Empty(t, manager.Jobs(), "nothing is trashed when one entry is relative")
}

func TestRestoreNamesCannotEscapeTheTrash(t *testing.T) {
	trashDir := trashHome(t)
	manager, _ := newHarness(t)

	for _, name := range []string{"../../../etc/passwd", "sub/file", "..", ""} {
		out := call(t, manager, ipc.Request{Method: "files.op.restore", Params: map[string]any{
			"names":    []any{name},
			"trashDir": trashDir,
		}})
		require.Contains(t, out, "error", name)
	}
	require.Empty(t, manager.Jobs())
}

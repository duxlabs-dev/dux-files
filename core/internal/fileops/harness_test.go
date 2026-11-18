package fileops

import (
	"os"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type testBus struct {
	mu         sync.Mutex
	events     []event
	finished   map[string]View
	waiters    map[string]chan View
	onConflict func(Conflict) Answer
	onFailure  func(Failure) Answer
	onPassword func(PasswordPrompt) Answer
	manager    *Manager
}

func newHarness(t *testing.T) (*Manager, *testBus) {
	t.Helper()
	bus := &testBus{finished: map[string]View{}, waiters: map[string]chan View{}}
	manager := NewManager(bus, nil)
	bus.manager = manager
	return manager, bus
}

func (b *testBus) Publish(_ string, data any) {
	payload, ok := data.(event)
	if !ok {
		return
	}

	b.mu.Lock()
	b.events = append(b.events, payload)
	conflict := b.onConflict
	failure := b.onFailure
	password := b.onPassword
	b.mu.Unlock()

	switch {
	case payload.Kind == "conflict" && conflict != nil:
		_ = b.manager.Respond(payload.JobID, conflict(*payload.Conflict))
	case payload.Kind == "error" && failure != nil:
		_ = b.manager.Respond(payload.JobID, failure(*payload.Failure))
	case payload.Kind == "password" && password != nil:
		_ = b.manager.Respond(payload.JobID, password(*payload.PasswordPrompt))
	case payload.Kind == "job" && (payload.Job.State == StateDone || payload.Job.State == StateFailed):
		b.complete(*payload.Job)
	}
}

func (b *testBus) complete(view View) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.finished[view.ID] = view
	waiter, ok := b.waiters[view.ID]
	if !ok {
		return
	}
	delete(b.waiters, view.ID)
	waiter <- view
}

func (b *testBus) wait(t *testing.T, id string) View {
	t.Helper()

	b.mu.Lock()
	if view, ok := b.finished[id]; ok {
		b.mu.Unlock()
		return view
	}
	waiter := make(chan View, 1)
	b.waiters[id] = waiter
	b.mu.Unlock()

	select {
	case view := <-waiter:
		return view
	case <-time.After(20 * time.Second):
		t.Fatalf("job %s did not finish", id)
		return View{}
	}
}

func (b *testBus) kinds(kind string) []event {
	b.mu.Lock()
	defer b.mu.Unlock()
	var out []event
	for _, e := range b.events {
		if e.Kind == kind {
			out = append(out, e)
		}
	}
	return out
}

func run(t *testing.T, m *Manager, bus *testBus, s spec) View {
	t.Helper()
	return bus.wait(t, m.enqueue(s).id)
}

func tempsIn(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	var leftovers []string
	for _, entry := range entries {
		if len(entry.Name()) > 12 && entry.Name()[:12] == ".dfiles-tmp-" {
			leftovers = append(leftovers, entry.Name())
		}
	}
	return leftovers
}

func otherDevice(t *testing.T, reference string) string {
	t.Helper()
	for _, parent := range []string{"/dev/shm", "/run/user/" + strconv.Itoa(os.Getuid()), os.TempDir()} {
		dir, err := os.MkdirTemp(parent, "dfiles-xdev-")
		if err != nil {
			continue
		}
		if sameDevice(reference, dir) {
			os.RemoveAll(dir)
			continue
		}
		t.Cleanup(func() { os.RemoveAll(dir) })
		return dir
	}
	if os.Getenv("CI") != "" {
		t.Fatal("no second filesystem for the cross-device tests")
	}
	t.Skip("no second filesystem available")
	return ""
}

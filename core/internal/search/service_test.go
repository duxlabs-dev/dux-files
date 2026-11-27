package search

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/duxlabs-dev/dux-go/files"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testBus struct {
	mu      sync.Mutex
	events  map[string][]event
	done    chan event
	results chan event
}

func newBus() *testBus {
	return &testBus{events: map[string][]event{}, done: make(chan event, 16), results: make(chan event, 1)}
}

func (b *testBus) Publish(topic string, data any) {
	ev, ok := data.(event)
	if !ok {
		return
	}
	b.mu.Lock()
	b.events[topic] = append(b.events[topic], ev)
	b.mu.Unlock()
	switch ev.Kind {
	case "done":
		b.done <- ev
	case "results":
		select {
		case b.results <- ev:
		default:
		}
	}
}

func (b *testBus) waitResults(t *testing.T) {
	t.Helper()
	select {
	case <-b.results:
	case <-time.After(10 * time.Second):
		t.Fatal("no results arrived")
	}
}

func (b *testBus) waitDone(t *testing.T) event {
	t.Helper()
	select {
	case ev := <-b.done:
		return ev
	case <-time.After(10 * time.Second):
		t.Fatal("search did not finish")
		return event{}
	}
}

func (b *testBus) names(topic string) []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	var out []string
	for _, ev := range b.events[topic] {
		for _, e := range ev.Entries {
			out = append(out, e.Name)
		}
	}
	return out
}

func newTestService(t *testing.T, bus Publisher) *Service {
	t.Helper()
	svc := walkOnly(NewService(bus, files.NewService(nil, t.TempDir(), nil)))
	svc.AttachOnOpen()
	return svc
}

func walkOnly(svc *Service) *Service {
	svc.tools.lookPath = func(string) (string, error) { return "", exec.ErrNotFound }
	return svc
}

func tree(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, path := range []string{
		"Report.pdf", "notes.txt", "photo.jpg", "song.mp3",
		"docs/report-2024.txt", "docs/deep/Another Report.md",
		".hidden/report-secret.txt", "docs/.report-dot.txt",
		"archive.zip", "clip.mp4",
	} {
		full := filepath.Join(root, path)
		require.NoError(t, os.MkdirAll(filepath.Dir(full), 0o755))
		require.NoError(t, os.WriteFile(full, []byte("x"), 0o644))
	}
	require.NoError(t, os.MkdirAll(filepath.Join(root, "Reports"), 0o755))
	return root
}

func runSearch(t *testing.T, root, query string, opts Options) (*testBus, event, string) {
	t.Helper()
	bus := newBus()
	svc := newTestService(t, bus)
	id, topic, err := svc.Start(context.Background(), root, query, opts)
	require.NoError(t, err)
	assert.Equal(t, "search:"+id, topic)
	return bus, bus.waitDone(t), topic
}

func TestMatchesNameSubstringCaseInsensitively(t *testing.T) {
	root := tree(t)
	bus, done, topic := runSearch(t, root, "REPORT", Options{})

	assert.ElementsMatch(t, []string{"Report.pdf", "report-2024.txt", "Another Report.md", "Reports"}, bus.names(topic))
	assert.Equal(t, 4, done.Count)
	assert.False(t, done.Truncated)
	assert.False(t, done.Cancelled)
}

func TestHiddenEntriesAreSkippedUnlessAsked(t *testing.T) {
	root := tree(t)
	bus, _, topic := runSearch(t, root, "report", Options{IncludeHidden: true})
	assert.ElementsMatch(t, []string{"Report.pdf", "report-2024.txt", "Another Report.md", "Reports", "report-secret.txt", ".report-dot.txt"}, bus.names(topic))
}

func TestTypeFilters(t *testing.T) {
	root := tree(t)
	tests := []struct {
		kind string
		want []string
	}{
		{"documents", []string{"Report.pdf", "notes.txt", "report-2024.txt", "Another Report.md"}},
		{"images", []string{"photo.jpg"}},
		{"audio", []string{"song.mp3"}},
		{"video", []string{"clip.mp4"}},
		{"archives", []string{"archive.zip"}},
		{"folders", []string{"docs", "deep", "Reports"}},
	}
	for _, tt := range tests {
		t.Run(tt.kind, func(t *testing.T) {
			bus, _, topic := runSearch(t, root, "", Options{Types: []string{tt.kind}})
			assert.ElementsMatch(t, tt.want, bus.names(topic))
		})
	}
}

func TestModifiedAndSizeFilters(t *testing.T) {
	root := t.TempDir()
	old := filepath.Join(root, "old.txt")
	fresh := filepath.Join(root, "fresh.txt")
	big := filepath.Join(root, "big.txt")
	require.NoError(t, os.WriteFile(old, []byte("1"), 0o644))
	require.NoError(t, os.WriteFile(fresh, []byte("22"), 0o644))
	require.NoError(t, os.WriteFile(big, make([]byte, 5000), 0o644))
	require.NoError(t, os.Chtimes(old, time.Now().Add(-48*time.Hour), time.Now().Add(-48*time.Hour)))

	bus, _, topic := runSearch(t, root, "txt", Options{ModifiedAfterMs: time.Now().Add(-time.Hour).UnixMilli()})
	assert.ElementsMatch(t, []string{"fresh.txt", "big.txt"}, bus.names(topic))

	bus, _, topic = runSearch(t, root, "txt", Options{MinSize: 2, MaxSize: 100})
	assert.ElementsMatch(t, []string{"fresh.txt"}, bus.names(topic))

	bus, _, topic = runSearch(t, root, "txt", Options{MinSize: 1000})
	assert.ElementsMatch(t, []string{"big.txt"}, bus.names(topic))
}

func TestResultsAreCappedAndReportedTruncated(t *testing.T) {
	root := t.TempDir()
	for i := range 30 {
		require.NoError(t, os.WriteFile(filepath.Join(root, "file-"+string(rune('a'+i%26))+string(rune('a'+i/26))+".txt"), []byte("x"), 0o644))
	}
	bus, done, topic := runSearch(t, root, "file", Options{Limit: 12})
	assert.Len(t, bus.names(topic), 12)
	assert.Equal(t, 12, done.Count)
	assert.True(t, done.Truncated)
	assert.False(t, done.Cancelled)
}

func TestSymlinkedDirectoriesAreNotFollowed(t *testing.T) {
	root := t.TempDir()
	real := filepath.Join(root, "real")
	require.NoError(t, os.MkdirAll(real, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(real, "inside.txt"), []byte("x"), 0o644))
	require.NoError(t, os.Symlink(real, filepath.Join(root, "linked")))

	bus, _, topic := runSearch(t, root, "", Options{Types: []string{"documents"}})
	assert.ElementsMatch(t, []string{"inside.txt"}, bus.names(topic))

	bus, _, topic = runSearch(t, root, "linked", Options{})
	assert.ElementsMatch(t, []string{"linked"}, bus.names(topic))
}

func TestUnreadableDirectoriesAreCounted(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root reads everything")
	}
	root := t.TempDir()
	locked := filepath.Join(root, "locked")
	require.NoError(t, os.MkdirAll(locked, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(locked, "match.txt"), []byte("x"), 0o644))
	require.NoError(t, os.Chmod(locked, 0o000))
	t.Cleanup(func() { _ = os.Chmod(locked, 0o755) })

	bus, done, topic := runSearch(t, root, "match", Options{})
	assert.Empty(t, bus.names(topic))
	assert.Equal(t, 1, done.Unreadable)
}

func TestCancelStopsTheWalk(t *testing.T) {
	root := t.TempDir()
	for i := range 400 {
		dir := filepath.Join(root, "d"+string(rune('a'+i%26)), "n"+string(rune('a'+i%26))+string(rune('a'+i/26)))
		require.NoError(t, os.MkdirAll(dir, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "match.txt"), []byte("x"), 0o644))
	}
	bus := newBus()
	svc := walkOnly(NewService(bus, slowStater{files.NewService(nil, t.TempDir(), nil)}))
	svc.AttachOnOpen()
	svc.workers = 1

	id, _, err := svc.Start(context.Background(), root, "match", Options{})
	require.NoError(t, err)
	bus.waitResults(t)
	assert.True(t, svc.Cancel(id))

	done := bus.waitDone(t)
	assert.True(t, done.Cancelled)
	assert.Less(t, done.Count, 400)
	assert.False(t, svc.Cancel(id), "a finished search is gone")
}

type slowStater struct{ inner *files.Service }

func (s slowStater) Stat(path string) (files.Entry, error) {
	time.Sleep(2 * time.Millisecond)
	return s.inner.Stat(path)
}

func TestClientDisconnectCancels(t *testing.T) {
	root := tree(t)
	bus := newBus()
	svc := walkOnly(NewService(bus, slowStater{files.NewService(nil, t.TempDir(), nil)}))
	svc.AttachOnOpen()
	ctx, disconnect := context.WithCancel(context.Background())
	disconnect()

	_, _, err := svc.Start(ctx, root, "report", Options{})
	require.NoError(t, err)
	done := bus.waitDone(t)
	assert.True(t, done.Cancelled)
}

func TestEventsBufferUntilTheTopicIsSubscribed(t *testing.T) {
	root := tree(t)
	bus := newBus()
	svc := walkOnly(NewService(bus, files.NewService(nil, t.TempDir(), nil)))

	id, topic, err := svc.Start(context.Background(), root, "report", Options{})
	require.NoError(t, err)
	sc, _ := svc.byID(id)
	<-sc.finished
	bus.mu.Lock()
	assert.Empty(t, bus.events[topic])
	bus.mu.Unlock()

	svc.Attach([]string{topic})
	done := bus.waitDone(t)
	assert.Equal(t, 4, done.Count)
	assert.Len(t, bus.names(topic), 4)
}

func TestValidation(t *testing.T) {
	svc := newTestService(t, newBus())
	tests := []struct {
		name  string
		root  string
		query string
		opts  Options
	}{
		{"relative root", "relative", "x", Options{}},
		{"empty query without filters", t.TempDir(), "  ", Options{}},
		{"unknown type", t.TempDir(), "x", Options{Types: []string{"spreadsheets"}}},
		{"inverted size range", t.TempDir(), "x", Options{MinSize: 10, MaxSize: 5}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := svc.Start(context.Background(), tt.root, tt.query, tt.opts)
			require.Error(t, err)
			assert.Equal(t, files.CodeInvalid, files.CodeOf(err))
		})
	}
}

func TestCloseWaitsForEverySearch(t *testing.T) {
	root := tree(t)
	bus := newBus()
	svc := walkOnly(NewService(bus, slowStater{files.NewService(nil, t.TempDir(), nil)}))
	svc.AttachOnOpen()
	for range 3 {
		_, _, err := svc.Start(context.Background(), root, "report", Options{})
		require.NoError(t, err)
	}

	closed := make(chan struct{})
	go func() {
		svc.Close()
		close(closed)
	}()
	select {
	case <-closed:
	case <-time.After(10 * time.Second):
		t.Fatal("Close did not return")
	}
	for range 3 {
		bus.waitDone(t)
	}
	svc.mu.Lock()
	defer svc.mu.Unlock()
	assert.Empty(t, svc.searches)
}

func TestUnattachedFinishedSearchIsDroppedByCancelOrDisconnect(t *testing.T) {
	root := tree(t)
	svc := walkOnly(NewService(newBus(), files.NewService(nil, t.TempDir(), nil)))

	id, _, err := svc.Start(context.Background(), root, "report", Options{})
	require.NoError(t, err)
	sc, _ := svc.byID(id)
	<-sc.finished
	_, still := svc.byID(id)
	assert.True(t, still, "kept for a late subscriber")
	assert.True(t, svc.Cancel(id))
	_, still = svc.byID(id)
	assert.False(t, still, "cancel drops a finished search")

	ctx, disconnect := context.WithCancel(context.Background())
	id, _, err = svc.Start(ctx, root, "report", Options{})
	require.NoError(t, err)
	sc, _ = svc.byID(id)
	<-sc.finished
	disconnect()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if _, still = svc.byID(id); !still {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	assert.False(t, still, "disconnect drops a finished search")
}

func TestConcurrentSearchesAreCapped(t *testing.T) {
	root := tree(t)
	svc := walkOnly(NewService(newBus(), files.NewService(nil, t.TempDir(), nil)))
	for range MaxConcurrent {
		_, _, err := svc.Start(context.Background(), root, "report", Options{})
		require.NoError(t, err)
	}
	_, _, err := svc.Start(context.Background(), root, "report", Options{})
	require.Error(t, err)
	assert.Equal(t, files.CodeBusy, files.CodeOf(err))
	svc.Close()
}

func TestStartAfterCloseIsRefused(t *testing.T) {
	svc := walkOnly(NewService(newBus(), files.NewService(nil, t.TempDir(), nil)))
	svc.Close()
	_, _, err := svc.Start(context.Background(), tree(t), "report", Options{})
	require.Error(t, err)
}

package search

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/duxlabs-dev/dux-files/core/internal/testfs"

	"github.com/duxlabs-dev/dux-go/ipc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeIndex struct {
	mu       sync.Mutex
	info     map[string]any
	infoErr  string
	hits     []map[string]any
	dirHits  []map[string]any
	total    uint64
	delay    time.Duration
	searches []map[string]any
	started  chan struct{}
}

func (f *fakeIndex) handle(_ context.Context, w *ipc.ConnWriter, req ipc.Request, _ *ipc.Subscriber) {
	f.mu.Lock()
	defer f.mu.Unlock()
	switch req.Method {
	case "info":
		if f.infoErr != "" {
			w.RespondError(req.ID, f.infoErr)
			return
		}
		w.Respond(req.ID, f.info)
	case "search":
		f.searches = append(f.searches, req.Params)
		if f.started != nil {
			f.started <- struct{}{}
		}
		time.Sleep(f.delay)
		w.Respond(req.ID, map[string]any{"hits": f.hits, "directory_hits": f.dirHits, "total_hits": f.total})
	default:
		w.RespondError(req.ID, "unknown method: "+req.Method)
	}
}

func (f *fakeIndex) params(t *testing.T) map[string]any {
	t.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()
	require.Len(t, f.searches, 1)
	return f.searches[0]
}

func startFakeIndex(t *testing.T, fake *fakeIndex) {
	t.Helper()
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
	srv := ipc.NewServer(ipc.Config{AppName: indexApp, APIVersion: 1}, fake.handle)
	require.NoError(t, srv.Listen())
	ctx, cancel := context.WithCancel(context.Background())
	go func() { _ = srv.Serve(ctx) }()
	t.Cleanup(func() {
		cancel()
		_ = srv.Close()
	})
}

func indexedService(t *testing.T, bus Publisher) *Service {
	t.Helper()
	svc := newTestService(t, bus)
	svc.SetIndexEnabled(true)
	svc.index.infoWait = 200 * time.Millisecond
	svc.index.searchWait = 200 * time.Millisecond
	return svc
}

func indexed(path string, score float64) map[string]any {
	return map[string]any{"id": path, "score": score}
}

func (b *testBus) kinds(topic string) []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	var out []string
	for _, ev := range b.events[topic] {
		out = append(out, ev.Kind)
	}
	return out
}

func (b *testBus) backend(topic string) event {
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, ev := range b.events[topic] {
		if ev.Kind == "backend" {
			return ev
		}
	}
	return event{}
}

func (b *testBus) scores(topic string) map[string]int {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := map[string]int{}
	for _, ev := range b.events[topic] {
		for _, h := range ev.Entries {
			out[h.Name] = h.Score
		}
	}
	return out
}

func homeInfo(home string, schema int) map[string]any {
	return map[string]any{
		"schema_version":          schema,
		"expected_schema_version": schema,
		"phase":                   "idle",
		"index_paths":             []map[string]any{{"path": home, "max_depth": 6, "exclude_hidden": true, "exclude_dirs": []string{"node_modules"}}},
	}
}

func TestIndexServesACoveredSearchInRankOrder(t *testing.T) {
	root := tree(t)
	t.Setenv("HOME", root)
	fake := &fakeIndex{
		info: homeInfo(root, 4),
		hits: []map[string]any{
			indexed(filepath.Join(root, "docs", "report-2024.txt"), 0.9),
			indexed(filepath.Join(root, "gone", "report.txt"), 0.8),
			indexed(filepath.Join(root, "Report.pdf"), 0.7),
			indexed("/elsewhere/report.txt", 0.6),
			indexed(filepath.Join(root, "notes.txt"), 0.5),
		},
		dirHits: []map[string]any{indexed(filepath.Join(root, "Reports"), 0.4)},
		total:   5,
	}
	startFakeIndex(t, fake)
	bus := newBus()
	svc := indexedService(t, bus)
	id, topic, err := svc.Start(context.Background(), filepath.Join(root, "docs"), "report", Options{Scope: ScopeHome})
	require.NoError(t, err)
	done := bus.waitDone(t)

	assert.Equal(t, "search:"+id, topic)
	assert.Equal(t, []string{"Report.pdf", "Reports", "report-2024.txt"}, bus.names(topic), "stale and foreign hits dropped, exact name first, depth last")
	assert.Equal(t, 3, done.Count)
	assert.False(t, done.Partial)
	backend := bus.backend(topic)
	assert.Equal(t, "index", backend.Backend)
	assert.Equal(t, "idle", backend.Phase)
	assert.Equal(t, 6, backend.DepthLimit)

	params := fake.params(t)
	assert.Equal(t, "report", params["query"])
	assert.Equal(t, "all", params["type"])
	assert.EqualValues(t, MaxResults, params["limit"])
	assert.NotContains(t, params, "folder", "schema 4 filters client-side")
	assert.NotContains(t, params, "fuzzy")
}

func TestIndexSendsFolderAndFiltersOnSchemaFive(t *testing.T) {
	root := tree(t)
	fake := &fakeIndex{info: homeInfo(root, 5), hits: []map[string]any{indexed(filepath.Join(root, "docs", "report-2024.txt"), 1)}, total: 1}
	startFakeIndex(t, fake)
	bus := newBus()
	svc := indexedService(t, bus)
	_, topic, err := svc.Start(context.Background(), filepath.Join(root, "docs"), "report", Options{Fuzzy: true, Contents: true, MinSize: 1, MaxSize: 10, ModifiedAfterMs: 86400000})
	require.NoError(t, err)
	bus.waitDone(t)

	params := fake.params(t)
	assert.Equal(t, filepath.Join(root, "docs"), params["folder"])
	assert.Equal(t, true, params["fuzzy"])
	assert.Equal(t, "body", params["field"])
	assert.EqualValues(t, 1, params["min_size"])
	assert.EqualValues(t, 10, params["max_size"])
	assert.Equal(t, "1970-01-02T00:00:00Z", params["modified_after"])
	assert.Equal(t, []string{"report-2024.txt"}, bus.names(topic))
	assert.Equal(t, 5, bus.backend(topic).DepthLimit)
}

func TestIndexReportsPartialWhenBleveCutTheCandidates(t *testing.T) {
	root := tree(t)
	fake := &fakeIndex{info: homeInfo(root, 4), hits: []map[string]any{indexed(filepath.Join(root, "Report.pdf"), 1)}, total: 5000}
	startFakeIndex(t, fake)
	bus := newBus()
	svc := indexedService(t, bus)
	_, _, err := svc.Start(context.Background(), root, "report", Options{})
	require.NoError(t, err)
	assert.True(t, bus.waitDone(t).Partial)
}

func TestIndexFallsBackToTheWalkOnTimeout(t *testing.T) {
	root := tree(t)
	fake := &fakeIndex{info: homeInfo(root, 4), delay: 600 * time.Millisecond}
	startFakeIndex(t, fake)
	bus := newBus()
	svc := indexedService(t, bus)
	_, topic, err := svc.Start(context.Background(), root, "report", Options{})
	require.NoError(t, err)
	done := bus.waitDone(t)

	backend := bus.backend(topic)
	assert.Equal(t, "walk", backend.Backend)
	assert.Equal(t, reasonError, backend.Reason)
	assert.Equal(t, 4, done.Count)
	assert.True(t, svc.index.logged.Load())
}

func TestIndexFallsBackWhenInfoIsUnknown(t *testing.T) {
	root := tree(t)
	fake := &fakeIndex{infoErr: "unknown method: info"}
	startFakeIndex(t, fake)
	bus := newBus()
	svc := indexedService(t, bus)
	_, topic, err := svc.Start(context.Background(), root, "report", Options{})
	require.NoError(t, err)
	bus.waitDone(t)
	assert.Equal(t, reasonError, bus.backend(topic).Reason)
	assert.Empty(t, fake.searches)
}

func TestIndexIsSkippedWhenTheSearchIsNotCovered(t *testing.T) {
	root := tree(t)
	tests := []struct {
		name   string
		query  string
		opts   Options
		info   map[string]any
		reason string
	}{
		{"outside the index", "report", Options{}, homeInfo(filepath.Join(root, "docs"), 4), reasonNotIndexed},
		{"hidden chip against a hidden-excluding index", "report", Options{IncludeHidden: true}, homeInfo(root, 4), reasonHidden},
		{"old schema", "report", Options{}, homeInfo(root, 1), reasonSchema},
		{"no query", "", Options{Types: []string{"images"}}, homeInfo(root, 4), reasonNoQuery},
		{"direct", "report", Options{Direct: true}, homeInfo(root, 4), reasonDirect},
		{"not running", "report", Options{}, nil, reasonNoIndex},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := &fakeIndex{info: tt.info}
			if tt.info != nil {
				startFakeIndex(t, fake)
			} else {
				t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
			}
			bus := newBus()
			svc := indexedService(t, bus)
			_, topic, err := svc.Start(context.Background(), root, tt.query, tt.opts)
			require.NoError(t, err)
			bus.waitDone(t)
			assert.Equal(t, "walk", bus.backend(topic).Backend)
			assert.Equal(t, tt.reason, bus.backend(topic).Reason)
			assert.Empty(t, fake.searches)
			assert.False(t, svc.index.logged.Load(), "not covered is not an error")
		})
	}
}

func TestIndexLocationIsCachedBriefly(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
	now := time.Now()
	b := indexBackend{now: func() time.Time { return now }}
	_, found := b.locate()
	assert.False(t, found)
	fake := &fakeIndex{}
	startFakeIndex(t, fake)
	_, found = b.locate()
	assert.False(t, found, "the miss is remembered")
	now = now.Add(locateTTL)
	_, found = b.locate()
	assert.True(t, found)
}

func TestCoverageTable(t *testing.T) {
	home := "/home/u"
	info := indexInfo{Schema: 4, Paths: []indexPath{
		{Path: home, MaxDepth: 3, ExcludeHidden: true, ExcludeDirs: []string{"node_modules", "/^build-.*/"}},
		{Path: "/srv/data", MaxDepth: 0, ExcludeHidden: false},
	}}
	tests := []struct {
		name      string
		roots     []string
		hidden    bool
		depthLeft int
		reason    string
	}{
		{"index root", []string{home}, false, 3, ""},
		{"one level down", []string{home + "/docs"}, false, 2, ""},
		{"at the depth limit", []string{home + "/a/b/c"}, false, 0, reasonDepth},
		{"unlimited path", []string{"/srv/data/deep/er"}, false, 0, ""},
		{"unlimited path keeps hidden", []string{"/srv/data"}, true, 0, ""},
		{"hidden against an excluding path", []string{home}, true, 0, reasonHidden},
		{"inside an excluded folder", []string{home + "/node_modules/x"}, false, 0, reasonExcluded},
		{"inside a regex excluded folder", []string{home + "/build-1"}, false, 0, reasonExcluded},
		{"inside a hidden folder", []string{home + "/.config"}, false, 0, reasonExcluded},
		{"outside every path", []string{"/mnt/usb"}, false, 0, reasonNotIndexed},
		{"everywhere with one unindexed mount", []string{home, "/mnt/usb"}, false, 0, reasonNotIndexed},
		{"everywhere fully indexed takes the tightest depth", []string{home + "/x", "/srv/data"}, false, 2, ""},
		{"prefix that is not a parent", []string{"/home/user2"}, false, 0, reasonNotIndexed},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			depthLeft, reason := coverage(tt.roots, tt.hidden, info)
			assert.Equal(t, tt.reason, reason)
			assert.Equal(t, tt.depthLeft, depthLeft)
		})
	}
	_, reason := coverage([]string{home}, false, indexInfo{Schema: 1, Paths: info.Paths})
	assert.Equal(t, reasonSchema, reason)
}

func TestEverywhereWalksHomeAndMountedDrives(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	mount := t.TempDir()
	nested := filepath.Join(home, "inner")
	for _, path := range []string{filepath.Join(home, "home-match.txt"), filepath.Join(mount, "drive-match.txt"), filepath.Join(nested, "nested-match.txt")} {
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, []byte("x"), 0o644))
	}
	bus := newBus()
	svc := newTestService(t, bus)
	svc.SetMounts(func() []string { return []string{mount, nested, "relative", ""} })

	_, topic, err := svc.Start(context.Background(), "/unused", "match", Options{Scope: ScopeEverywhere})
	require.NoError(t, err)
	bus.waitDone(t)
	assert.ElementsMatch(t, []string{"home-match.txt", "drive-match.txt", "nested-match.txt"}, bus.names(topic))

	_, topic, err = svc.Start(context.Background(), mount, "match", Options{Scope: ScopeHome})
	require.NoError(t, err)
	bus.waitDone(t)
	assert.ElementsMatch(t, []string{"home-match.txt", "nested-match.txt"}, bus.names(topic))
}

func TestWalkSkipsHeavyFoldersIgnoreFilesAndUserExclusions(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	root := t.TempDir()
	testfs.Write(t, filepath.Join(root, ".gitignore"), "*.log\n")
	testfs.Write(t, filepath.Join(root, "node_modules", "match-a.txt"), "x")
	testfs.Write(t, filepath.Join(root, "Steam", "match-b.txt"), "x")
	testfs.Write(t, filepath.Join(root, "match-c.log"), "x")
	testfs.Write(t, filepath.Join(root, "src", "match-d.txt"), "x")
	bus := newBus()
	svc := newTestService(t, bus)
	svc.SetExcludes(func() []string { return []string{"Steam"} })

	_, topic, err := svc.Start(context.Background(), root, "match", Options{})
	require.NoError(t, err)
	bus.waitDone(t)
	assert.ElementsMatch(t, []string{"match-d.txt"}, bus.names(topic))

	_, topic, err = svc.Start(context.Background(), root, "match", Options{IncludeIgnored: true})
	require.NoError(t, err)
	bus.waitDone(t)
	assert.ElementsMatch(t, []string{"match-a.txt", "match-b.txt", "match-c.log", "match-d.txt"}, bus.names(topic))
}

func TestWalkScoresResultsAndHoldsFuzzyMatchesForLast(t *testing.T) {
	root := t.TempDir()
	testfs.Write(t, filepath.Join(root, "z", "r-e-p-o-r-t.txt"), "x")
	testfs.Write(t, filepath.Join(root, "z", "y", "report.txt"), "x")
	testfs.Write(t, filepath.Join(root, "my-report.txt"), "x")
	bus := newBus()
	svc := newTestService(t, bus)
	svc.workers = 1

	_, topic, err := svc.Start(context.Background(), root, "report", Options{Fuzzy: true})
	require.NoError(t, err)
	bus.waitDone(t)
	names := bus.names(topic)
	assert.Len(t, names, 3)
	assert.Equal(t, "r-e-p-o-r-t.txt", names[2], "fuzzy-only matches come after the walk")
	scores := bus.scores(topic)
	assert.Greater(t, scores["report.txt"], scores["my-report.txt"])
	assert.Greater(t, scores["my-report.txt"], scores["r-e-p-o-r-t.txt"])

	_, topic, err = svc.Start(context.Background(), root, "report", Options{})
	require.NoError(t, err)
	bus.waitDone(t)
	assert.ElementsMatch(t, []string{"report.txt", "my-report.txt"}, bus.names(topic))
	assert.Equal(t, "walk", bus.backend(topic).Backend)
}

func TestWalkMatchesTermsAgainstThePath(t *testing.T) {
	root := t.TempDir()
	testfs.Write(t, filepath.Join(root, "search", "walk.go"), "x")
	testfs.Write(t, filepath.Join(root, "search", "other.go"), "x")
	testfs.Write(t, filepath.Join(root, "walk", "readme.md"), "x")
	bus := newBus()
	svc := newTestService(t, bus)
	_, topic, err := svc.Start(context.Background(), root, "search walk", Options{})
	require.NoError(t, err)
	bus.waitDone(t)
	assert.Equal(t, []string{"walk.go"}, bus.names(topic))
}

func TestWalkGlobTerms(t *testing.T) {
	root := tree(t)
	bus := newBus()
	svc := newTestService(t, bus)
	_, topic, err := svc.Start(context.Background(), root, "*.txt", Options{})
	require.NoError(t, err)
	bus.waitDone(t)
	assert.ElementsMatch(t, []string{"notes.txt", "report-2024.txt"}, bus.names(topic))
}

func TestIndexCancelledMidRoundTripKeepsTheSocket(t *testing.T) {
	root := tree(t)
	fake := &fakeIndex{info: homeInfo(root, 4), delay: 400 * time.Millisecond, started: make(chan struct{}, 1)}
	startFakeIndex(t, fake)
	bus := newBus()
	svc := indexedService(t, bus)
	svc.index.searchWait = 2 * time.Second
	ctx, cancel := context.WithCancel(context.Background())
	_, topic, err := svc.Start(ctx, root, "report", Options{})
	require.NoError(t, err)
	select {
	case <-fake.started:
	case <-time.After(5 * time.Second):
		t.Fatal("the index never saw the search")
	}
	cancel()
	done := bus.waitDone(t)

	assert.True(t, done.Cancelled)
	assert.Equal(t, event{}, bus.backend(topic), "no backend event, no walk")
	assert.False(t, svc.index.logged.Load())
	_, found := svc.index.locate()
	assert.True(t, found, "the socket stays cached")
}

func TestIndexHonoursCleanedRootsAndTheIgnoreChip(t *testing.T) {
	root := tree(t)
	fake := &fakeIndex{info: homeInfo(root+"/", 5), hits: []map[string]any{
		indexed(filepath.Join(root, "docs", "report-2024.txt"), 1),
		indexed(filepath.Join(root, "Steam", "report.txt"), 1),
		indexed(filepath.Join(root, "node_modules", "report.txt"), 1),
	}, total: 3}
	startFakeIndex(t, fake)
	bus := newBus()
	svc := indexedService(t, bus)
	svc.SetExcludes(func() []string { return []string{"Steam"} })

	_, topic, err := svc.Start(context.Background(), root+"/docs/../docs/", "report", Options{})
	require.NoError(t, err)
	bus.waitDone(t)
	assert.Equal(t, "index", bus.backend(topic).Backend)
	assert.Equal(t, 5, bus.backend(topic).DepthLimit)
	assert.Equal(t, filepath.Join(root, "docs"), fake.params(t)["folder"])
	assert.Equal(t, []string{"report-2024.txt"}, bus.names(topic))

	_, topic, err = svc.Start(context.Background(), root, "report", Options{})
	require.NoError(t, err)
	bus.waitDone(t)
	assert.Equal(t, []string{"report-2024.txt"}, bus.names(topic), "excluded and heavy folders dropped from index hits")

	_, topic, err = svc.Start(context.Background(), root, "report", Options{IncludeIgnored: true})
	require.NoError(t, err)
	bus.waitDone(t)
	assert.Equal(t, reasonIgnored, bus.backend(topic).Reason)
}

func TestIndexQueryFieldAndGlobTerms(t *testing.T) {
	root := tree(t)
	fake := &fakeIndex{info: homeInfo(root, 4), hits: []map[string]any{indexed(filepath.Join(root, "docs", "report-2024.txt"), 1), indexed(filepath.Join(root, "notes.txt"), 1)}, total: 2}
	startFakeIndex(t, fake)
	bus := newBus()
	svc := indexedService(t, bus)
	_, topic, err := svc.Start(context.Background(), root, "report *.txt", Options{})
	require.NoError(t, err)
	bus.waitDone(t)
	params := fake.params(t)
	assert.Equal(t, "*.txt", params["query"])
	assert.Equal(t, "filename", params["field"])
	assert.Equal(t, []string{"report-2024.txt"}, bus.names(topic))
}

func TestRootsFoldNestedMountsBehindSiblings(t *testing.T) {
	assert.Equal(t, []string{"/home/u", "/home/u-old"}, dedupeRoots([]string{"/home/u", "/home/u-old", "/home/u/mnt", "/home/u/"}))
}

func TestGlobAndBoundariesAreRuneAware(t *testing.T) {
	assert.True(t, wildcard("café", "caf?"))
	assert.True(t, atBoundary("é-x", len("é-")))
	assert.False(t, atBoundary("éx", len("é")))
}

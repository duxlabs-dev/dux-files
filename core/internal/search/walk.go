package search

import (
	"container/heap"
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/duxlabs-dev/dux-go/files"
)

const (
	batchSize      = 200
	flushInterval  = 100 * time.Millisecond
	fuzzyReservoir = 2000
)

type hit struct {
	files.Entry
	Score   int    `json:"score"`
	Line    int    `json:"line,omitempty"`
	Snippet string `json:"snippet,omitempty"`
}

type event struct {
	SearchID   string `json:"searchId"`
	Kind       string `json:"kind"`
	Entries    []hit  `json:"entries,omitempty"`
	Count      int    `json:"count"`
	Truncated  bool   `json:"truncated,omitempty"`
	Partial    bool   `json:"partial,omitempty"`
	Cancelled  bool   `json:"cancelled,omitempty"`
	Unreadable int    `json:"unreadable,omitempty"`
	Backend    string `json:"backend,omitempty"`
	Phase      string `json:"phase,omitempty"`
	DepthLimit int    `json:"depthLimit,omitempty"`
	Reason     string `json:"reason,omitempty"`
	Contents   bool   `json:"contents,omitempty"`
	Code       string `json:"code,omitempty"`
	Error      string `json:"error,omitempty"`
}

type search struct {
	id, topic string
	svc       *Service
	ctx       context.Context
	cancel    context.CancelFunc
	finished  chan struct{}

	mu       sync.Mutex
	attached bool
	done     bool
	pending  []event
	stopWait func() bool
}

func newSearch(svc *Service) *search {
	ctx, cancel := context.WithCancel(context.Background())
	return &search{svc: svc, ctx: ctx, cancel: cancel, finished: make(chan struct{})}
}

func (sc *search) follow(parent context.Context) {
	sc.stopWait = context.AfterFunc(parent, func() {
		sc.cancel()
		sc.svc.unregister(sc.id)
	})
}

func (sc *search) run(root, query string, opts Options) {
	defer close(sc.finished)
	defer sc.retire()

	targets := roots(opts.Scope, root, sc.svc.mounts)
	out := &sink{sc: sc, em: newEmitter(sc, opts.limit()), opts: opts, reservoir: reservoir{limit: fuzzyReservoir}}
	reason := sc.svc.index.serve(sc, out, targets, query)
	if reason == "" {
		return
	}
	found := sc.svc.tools.locate()
	if opts.Contents && found.rg != "" && strings.TrimSpace(query) != "" {
		sc.grep(out, found.rg, targets, query, reason)
		return
	}
	m, err := newMatcher(query, opts)
	if err != nil {
		out.em.fail(files.CodeInvalid, err.Error())
		return
	}
	backend := event{Kind: "backend", Reason: reason, Contents: found.rg != ""}
	if found.fd != "" && m.selective() {
		sc.find(out, found.fd, targets, m, backend)
		return
	}
	sc.walk(out, targets, m, backend)
}

func (sc *search) excludes() []string {
	if sc.svc.excludes == nil {
		return nil
	}
	return sc.svc.excludes()
}

func (sc *search) skipRules(includeIgnored bool) ignoreRules {
	if includeIgnored {
		return ignoreRules{}
	}
	return newSkipRules(sc.excludes())
}

func (sc *search) walk(out *sink, targets []string, m matcher, backend event) {
	backend.Backend = "walk"
	sc.publish(backend)
	unreadable := sc.walkInto(out, targets, m)
	out.flush()
	out.em.finish(false, unreadable)
}

func (sc *search) walkInto(out *sink, targets []string, m matcher) int {
	opts := out.opts
	w := &walker{
		sc:    sc,
		out:   out,
		match: m,
		opts:  opts,
		rules: newIgnoreRules(sc.excludes(), opts.IncludeIgnored),
	}
	for _, target := range targets {
		info := &rootInfo{path: target}
		if !opts.CrossDevice {
			if dev, ok := deviceOf(target); ok {
				info.device, info.sameDevice = dev, true
			}
		}
		w.queue.push(dirItem{path: target, root: info, patterns: w.rules.rootPatterns(target)})
	}

	var workers sync.WaitGroup
	for range sc.svc.workers {
		workers.Add(1)
		go func() {
			defer workers.Done()
			w.work()
		}()
	}
	go func() {
		w.queue.wait()
		w.queue.close()
	}()
	workers.Wait()
	return int(w.unreadable.Load())
}

func (sc *search) publish(ev event) {
	ev.SearchID = sc.id
	sc.mu.Lock()
	if !sc.attached && !sc.svc.attachOnOpen.Load() {
		sc.pending = append(sc.pending, ev)
		sc.mu.Unlock()
		return
	}
	sc.mu.Unlock()
	sc.svc.publish(sc.topic, ev)
}

func (sc *search) attach() {
	sc.mu.Lock()
	sc.attached = true
	for _, ev := range sc.pending {
		sc.svc.publish(sc.topic, ev)
	}
	sc.pending = nil
	done := sc.done
	sc.mu.Unlock()
	if done {
		sc.drop()
	}
}

// Must run before the done event, or a client that saw the search finish could cancel a still registered id.
func (sc *search) retire() {
	sc.mu.Lock()
	if sc.done {
		sc.mu.Unlock()
		return
	}
	sc.done = true
	flushed := sc.attached || sc.svc.attachOnOpen.Load()
	sc.mu.Unlock()
	if flushed {
		sc.drop()
	}
}

func (sc *search) drop() {
	if sc.stopWait != nil {
		sc.stopWait()
	}
	sc.svc.unregister(sc.id)
}

type rootInfo struct {
	path       string
	device     uint64
	sameDevice bool
}

type walker struct {
	sc         *search
	out        *sink
	match      matcher
	opts       Options
	rules      ignoreRules
	queue      dirQueue
	unreadable atomic.Int64
}

func (w *walker) work() {
	for {
		item, ok := w.queue.pop()
		if !ok {
			return
		}
		w.readDir(item)
		w.queue.done()
	}
}

func (w *walker) readDir(item dirItem) {
	if w.sc.ctx.Err() != nil {
		return
	}
	children, err := os.ReadDir(item.path)
	if err != nil {
		w.unreadable.Add(1)
		return
	}
	patterns := w.rules.loadIf(item.path, children, item.patterns)
	for _, child := range children {
		if w.sc.ctx.Err() != nil {
			return
		}
		name := child.Name()
		if !w.opts.IncludeHidden && strings.HasPrefix(name, ".") {
			continue
		}
		full := filepath.Join(item.path, name)
		isDir := child.Type()&fs.ModeSymlink == 0 && child.IsDir()
		if isDir && w.rules.skipsDir(name, full) {
			continue
		}
		if ignored(patterns, full, isDir) {
			continue
		}
		nameLower := strings.ToLower(name)
		relLower := nameLower
		if item.relLower != "" {
			relLower = item.relLower + "/" + nameLower
		}
		if isDir && w.descends(full, item.root) {
			w.queue.push(dirItem{
				path:     full,
				relLower: relLower,
				depth:    item.depth + 1,
				root:     item.root,
				patterns: patterns,
			})
		}
		points, tier := w.match.score(name, nameLower, relLower, item.depth)
		if !w.out.offer(full, points, tier) {
			return
		}
	}
}

func (w *walker) descends(dir string, root *rootInfo) bool {
	if !root.sameDevice {
		return true
	}
	dev, ok := deviceOf(dir)
	return !ok || dev == root.device
}

type sink struct {
	sc        *search
	em        *emitter
	opts      Options
	reservoir reservoir
}

func (s *sink) offer(path string, points, tier int) bool {
	switch tier {
	case tierNone:
		return true
	case tierFuzzy:
		s.reservoir.offer(path, points)
		return true
	}
	return s.emit(path, hit{Score: points})
}

func (s *sink) emit(path string, found hit) bool {
	entry, err := s.sc.svc.stater.Stat(path)
	if err != nil || !s.opts.accepts(entry) {
		return true
	}
	found.Entry = entry
	return s.em.emit(found)
}

func (s *sink) flush() {
	for _, f := range s.reservoir.drain() {
		if s.sc.ctx.Err() != nil {
			return
		}
		if !s.emit(f.path, hit{Score: f.score}) {
			return
		}
	}
}

type fuzzyHit struct {
	path  string
	score int
}

type fuzzyHeap []fuzzyHit

func (h fuzzyHeap) Len() int           { return len(h) }
func (h fuzzyHeap) Less(i, j int) bool { return h[i].score < h[j].score }
func (h fuzzyHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }
func (h *fuzzyHeap) Push(x any)        { *h = append(*h, x.(fuzzyHit)) }
func (h *fuzzyHeap) Pop() any          { old := *h; x := old[len(old)-1]; *h = old[:len(old)-1]; return x }

type reservoir struct {
	mu    sync.Mutex
	limit int
	items fuzzyHeap
}

func (r *reservoir) offer(path string, score int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	switch {
	case len(r.items) < r.limit:
		heap.Push(&r.items, fuzzyHit{path, score})
	case score > r.items[0].score:
		r.items[0] = fuzzyHit{path, score}
		heap.Fix(&r.items, 0)
	}
}

func (r *reservoir) drain() []fuzzyHit {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := r.items
	r.items = nil
	sort.SliceStable(out, func(i, j int) bool { return out[i].score > out[j].score })
	return out
}

type emitter struct {
	sc      *search
	limit   int
	matches chan hit
	done    chan event
}

func newEmitter(sc *search, limit int) *emitter {
	em := &emitter{sc: sc, limit: limit, matches: make(chan hit, batchSize), done: make(chan event, 1)}
	go em.collect()
	return em
}

func (em *emitter) emit(h hit) bool {
	select {
	case em.matches <- h:
		return true
	case <-em.sc.ctx.Done():
		return false
	}
}

func (em *emitter) finish(partial bool, unreadable int) {
	em.end(event{Partial: partial, Unreadable: unreadable})
}

func (em *emitter) fail(code files.Code, message string) {
	em.end(event{Code: string(code), Error: message})
}

func (em *emitter) end(outcome event) {
	close(em.matches)
	done := <-em.done
	done.Cancelled = em.sc.ctx.Err() != nil && !done.Truncated
	done.Partial = outcome.Partial
	done.Unreadable = outcome.Unreadable
	done.Code = outcome.Code
	done.Error = outcome.Error
	em.sc.retire()
	em.sc.publish(done)
}

func (em *emitter) collect() {
	var batch []hit
	count := 0
	truncated := false
	var timer *time.Timer
	var due <-chan time.Time

	flush := func() {
		if len(batch) == 0 {
			return
		}
		em.sc.publish(event{Kind: "results", Entries: batch, Count: count})
		batch = nil
	}
	arm := func() {
		if timer == nil {
			timer = time.NewTimer(flushInterval)
			due = timer.C
			return
		}
		timer.Reset(flushInterval)
	}

	for {
		select {
		case h, ok := <-em.matches:
			if !ok {
				flush()
				if timer != nil {
					timer.Stop()
				}
				em.done <- event{Kind: "done", Count: count, Truncated: truncated}
				return
			}
			if truncated {
				continue
			}
			batch = append(batch, h)
			count++
			switch {
			case count >= em.limit:
				truncated = true
				em.sc.cancel()
				flush()
			case len(batch) == 1:
				arm()
			case len(batch) >= batchSize:
				flush()
			}
		case <-due:
			flush()
		}
	}
}

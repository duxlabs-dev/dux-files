package search

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"net"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/duxlabs-dev/dux-files/core/internal/pathutil"
	"github.com/duxlabs-dev/dux-go/ipc"
	"github.com/duxlabs-dev/dux-go/log"
)

const (
	indexApp      = "danksearch"
	locateTTL     = 10 * time.Second
	infoTimeout   = time.Second
	searchTimeout = 3 * time.Second
	minSchema     = 2
	folderSchema  = 5
)

const (
	reasonNoIndex    = "NOINDEX"
	reasonError      = "ERROR"
	reasonNoQuery    = "NOQUERY"
	reasonSchema     = "SCHEMA"
	reasonNotIndexed = "NOTINDEXED"
	reasonDepth      = "DEPTH"
	reasonHidden     = "HIDDEN"
	reasonExcluded   = "EXCLUDED"
	reasonIgnored    = "IGNORED"
	reasonDirect     = "DIRECT"
)

type indexInfo struct {
	Schema         int         `json:"schema_version"`
	ExpectedSchema int         `json:"expected_schema_version"`
	Phase          string      `json:"phase"`
	Paths          []indexPath `json:"index_paths"`
}

type indexPath struct {
	Path          string   `json:"path"`
	MaxDepth      int      `json:"max_depth"`
	ExcludeHidden bool     `json:"exclude_hidden"`
	ExcludeDirs   []string `json:"exclude_dirs"`
}

type indexHit struct {
	ID    string  `json:"id"`
	Score float64 `json:"score"`
}

type indexResult struct {
	Hits          []indexHit `json:"hits"`
	DirectoryHits []indexHit `json:"directory_hits"`
	TotalHits     uint64     `json:"total_hits"`
}

type indexBackend struct {
	enabled    atomic.Bool
	logged     atomic.Bool
	now        func() time.Time
	infoWait   time.Duration
	searchWait time.Duration

	mu      sync.Mutex
	socket  string
	checked time.Time
}

func (b *indexBackend) locate() (string, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	now := b.now()
	if !b.checked.IsZero() && now.Sub(b.checked) < locateTTL {
		return b.socket, b.socket != ""
	}
	b.checked = now
	socket, err := ipc.FindRunningSocket(indexApp)
	if err != nil {
		b.socket = ""
		return "", false
	}
	b.socket = socket
	return socket, true
}

func (b *indexBackend) forget() {
	b.mu.Lock()
	b.socket = ""
	b.checked = b.now()
	b.mu.Unlock()
}

func (b *indexBackend) fallback(err error) string {
	b.forget()
	if !b.logged.Swap(true) {
		log.Warnf("danksearch unavailable, searching with the walk instead: %v", err)
	}
	return reasonError
}

func (b *indexBackend) serve(sc *search, em *emitter, targets []string, query string, terms []term, opts Options) string {
	if opts.Direct {
		return reasonDirect
	}
	if !b.enabled.Load() {
		return reasonNoIndex
	}
	if strings.TrimSpace(query) == "" {
		return reasonNoQuery
	}
	if opts.IncludeIgnored {
		return reasonIgnored
	}
	socket, ok := b.locate()
	if !ok {
		return reasonNoIndex
	}
	info, err := b.info(sc.ctx, socket)
	if err != nil {
		return b.failed(sc, em, err)
	}
	depthLeft, reason := coverage(targets, opts.IncludeHidden, info)
	if reason != "" {
		return reason
	}
	result, err := b.search(sc.ctx, socket, indexParams(targets, query, terms, opts, info))
	if err != nil {
		return b.failed(sc, em, err)
	}
	sc.publish(event{Kind: "backend", Backend: "index", Phase: info.Phase, DepthLimit: depthLeft})
	var excludes []string
	if sc.svc.excludes != nil {
		excludes = sc.svc.excludes()
	}
	ranked := rankHits(result, targets, terms, opts, newSkipRules(excludes))
	for _, c := range ranked {
		if sc.ctx.Err() != nil {
			break
		}
		entry, err := sc.svc.stater.Stat(c.path)
		if err != nil || !opts.accepts(entry) {
			continue
		}
		if !em.emit(hit{Entry: entry, Score: c.score}) {
			break
		}
	}
	em.finish(result.partial(opts.limit()), 0)
	return ""
}

func (b *indexBackend) failed(sc *search, em *emitter, err error) string {
	if sc.ctx.Err() != nil {
		em.finish(false, 0)
		return ""
	}
	return b.fallback(err)
}

func (b *indexBackend) info(ctx context.Context, socket string) (indexInfo, error) {
	raw, err := b.call(ctx, socket, "info", nil, b.infoWait)
	if err != nil {
		return indexInfo{}, err
	}
	var info indexInfo
	if err := json.Unmarshal(raw, &info); err != nil {
		return indexInfo{}, err
	}
	return info, nil
}

func (b *indexBackend) search(ctx context.Context, socket string, params map[string]any) (indexResult, error) {
	raw, err := b.call(ctx, socket, "search", params, b.searchWait)
	if err != nil {
		return indexResult{}, err
	}
	var result indexResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return indexResult{}, err
	}
	return result, nil
}

func (b *indexBackend) call(ctx context.Context, socket, method string, params map[string]any, timeout time.Duration) (json.RawMessage, error) {
	conn, err := net.DialTimeout("unix", socket, timeout)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	_ = conn.SetDeadline(b.now().Add(timeout))
	stop := context.AfterFunc(ctx, func() { _ = conn.SetDeadline(time.Unix(0, 0)) })
	defer stop()

	reader := bufio.NewReaderSize(conn, 64*1024)
	if _, err := reader.ReadBytes('\n'); err != nil {
		return nil, err
	}
	body, err := json.Marshal(ipc.Request{ID: 1, Method: method, Params: params})
	if err != nil {
		return nil, err
	}
	if _, err := conn.Write(append(body, '\n')); err != nil {
		return nil, err
	}
	line, err := reader.ReadBytes('\n')
	if err != nil {
		return nil, err
	}
	var resp struct {
		Result json.RawMessage `json:"result"`
		Error  string          `json:"error"`
	}
	if err := json.Unmarshal(line, &resp); err != nil {
		return nil, err
	}
	if resp.Error != "" {
		return nil, errors.New(resp.Error)
	}
	return resp.Result, nil
}

// bleve treats a query holding * or ? as one wildcard over the whole name, so a glob term goes alone.
func indexParams(targets []string, query string, terms []term, opts Options, info indexInfo) map[string]any {
	params := map[string]any{"query": query, "limit": opts.limit(), "type": "all", "field": "filename"}
	for _, t := range terms {
		if t.glob {
			params["query"] = t.text
			break
		}
	}
	if opts.Fuzzy {
		params["fuzzy"] = true
	}
	if opts.Contents {
		params["field"] = "body"
	}
	if opts.MinSize > 0 {
		params["min_size"] = opts.MinSize
	}
	if opts.MaxSize > 0 {
		params["max_size"] = opts.MaxSize
	}
	if opts.ModifiedAfterMs > 0 {
		params["modified_after"] = time.UnixMilli(opts.ModifiedAfterMs).UTC().Format(time.RFC3339)
	}
	if info.Schema >= folderSchema && len(targets) == 1 {
		params["folder"] = targets[0]
	}
	return params
}

func (r indexResult) partial(limit int) bool {
	return r.TotalHits > uint64(len(r.Hits)) || len(r.DirectoryHits) >= limit
}

type candidate struct {
	path  string
	score int
	bleve float64
}

func rankHits(result indexResult, targets []string, terms []term, opts Options, rules ignoreRules) []candidate {
	seen := make(map[string]bool, len(result.Hits)+len(result.DirectoryHits))
	var out []candidate
	consider := func(h indexHit) {
		path := filepath.Clean(h.ID)
		if seen[path] {
			return
		}
		seen[path] = true
		root, ok := rootOf(path, targets)
		if !ok || rules.skipsPath(path, root) {
			return
		}
		rel := strings.ToLower(path[len(root)+1:])
		name := rel[strings.LastIndexByte(rel, '/')+1:]
		depth := strings.Count(rel, "/")
		points := 0
		if !opts.Contents {
			var tier int
			points, tier = score(name, rel, depth, terms, opts.Fuzzy)
			if tier == tierNone {
				return
			}
		}
		out = append(out, candidate{path: path, score: points, bleve: h.Score})
	}
	for _, h := range result.Hits {
		consider(h)
	}
	for _, h := range result.DirectoryHits {
		consider(h)
	}
	sort.SliceStable(out, func(i, j int) bool {
		switch {
		case out[i].score != out[j].score:
			return out[i].score > out[j].score
		case out[i].bleve != out[j].bleve:
			return out[i].bleve > out[j].bleve
		}
		return out[i].path < out[j].path
	})
	return out
}

func rootOf(path string, targets []string) (string, bool) {
	for _, root := range targets {
		if path != root && pathutil.Within(path, root) {
			return strings.TrimSuffix(root, "/"), true
		}
	}
	return "", false
}

// depthLeft is how many levels below the root the index reaches, 0 for all.
func coverage(targets []string, includeHidden bool, info indexInfo) (depthLeft int, reason string) {
	if info.Schema < minSchema {
		return 0, reasonSchema
	}
	for _, root := range targets {
		ip := longestIndexPath(root, info.Paths)
		if ip == nil {
			return 0, reasonNotIndexed
		}
		if includeHidden && ip.ExcludeHidden {
			return 0, reasonHidden
		}
		rel, err := filepath.Rel(filepath.Clean(ip.Path), root)
		if err != nil {
			return 0, reasonNotIndexed
		}
		comps := components(rel)
		if rel == "." {
			comps = nil
		}
		for _, comp := range comps {
			if ip.ExcludeHidden && strings.HasPrefix(comp, ".") {
				return 0, reasonExcluded
			}
			if ip.excludes(comp) {
				return 0, reasonExcluded
			}
		}
		if ip.MaxDepth <= 0 {
			continue
		}
		if len(comps) >= ip.MaxDepth {
			return 0, reasonDepth
		}
		left := ip.MaxDepth - len(comps)
		if depthLeft == 0 || left < depthLeft {
			depthLeft = left
		}
	}
	return depthLeft, ""
}

func longestIndexPath(root string, paths []indexPath) *indexPath {
	var best *indexPath
	for i := range paths {
		ip := &paths[i]
		if ip.Path == "" || !pathutil.Within(root, filepath.Clean(ip.Path)) {
			continue
		}
		if best == nil || len(ip.Path) > len(best.Path) {
			best = ip
		}
	}
	return best
}

func (ip *indexPath) excludes(comp string) bool {
	for _, rule := range ip.ExcludeDirs {
		if len(rule) >= 3 && rule[0] == '/' && rule[len(rule)-1] == '/' {
			if re, err := regexp.Compile(rule[1 : len(rule)-1]); err == nil && re.MatchString(comp) {
				return true
			}
			continue
		}
		if rule == comp {
			return true
		}
	}
	return false
}

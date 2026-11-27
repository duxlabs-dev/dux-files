package ipc

import (
	"cmp"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/duxlabs-dev/dux-files/core/internal/fileops"
	"github.com/duxlabs-dev/dux-files/core/internal/wire"
	"github.com/duxlabs-dev/dux-go/files"
	"github.com/duxlabs-dev/dux-go/ipc/params"
)

const (
	archiveWatchPrefix = "archive:"
	genericFileIcon    = "text-x-generic"
)

type archiveListing struct {
	Path        string        `json:"path"`
	WatchID     string        `json:"watchId"`
	Entries     []files.Entry `json:"entries"`
	Total       int           `json:"total"`
	Hidden      int           `json:"hidden"`
	Cursor      string        `json:"cursor,omitempty"`
	Topic       string        `json:"topic"`
	Watching    bool          `json:"watching"`
	PollOnFocus bool          `json:"pollOnFocus"`
}

type archiveView struct {
	includeHidden bool
	filters       []string
	sortKey       files.SortKey
	desc          bool
	dirsFirst     bool
	hiddenLast    bool
	limit         int
	cursor        int
}

func archiveWatchID(archive, inner string) string {
	return archiveWatchPrefix + strconv.Itoa(len(archive)) + ":" + filepath.Join(archive, inner)
}

func parseArchiveWatchID(id string) (archive, inner string, ok bool) {
	rest, found := strings.CutPrefix(id, archiveWatchPrefix)
	if !found {
		return "", "", false
	}
	rawLength, path, found := strings.Cut(rest, ":")
	length, err := strconv.Atoi(rawLength)
	if !found || err != nil || length <= 0 || length > len(path) {
		return "", "", false
	}
	return path[:length], strings.TrimPrefix(path[length:], "/"), true
}

func archiveTarget(req Request) (archive, inner string, ok bool) {
	if id := params.StringOpt(req.Params, "watchId", ""); id != "" {
		return parseArchiveWatchID(id)
	}
	path := params.StringOpt(req.Params, "path", "")
	if !filepath.IsAbs(path) {
		return "", "", false
	}
	return fileops.LocateArchive(path)
}

func handleArchiveFiles(w *ConnWriter, req Request, deps Deps) bool {
	if deps.FileOps == nil {
		return false
	}
	switch req.Method {
	case "files.watch", "files.list":
		archive, inner, ok := archiveTarget(req)
		if !ok {
			return false
		}
		respondArchiveListing(w, req, deps, archive, inner)
	case "files.stat":
		if _, err := os.Lstat(params.StringOpt(req.Params, "path", "")); err == nil {
			return false
		}
		archive, inner, ok := archiveTarget(req)
		if !ok || inner == "" {
			return false
		}
		respondArchiveStat(w, req, deps, archive, inner)
	case "files.unwatch":
		id := params.StringOpt(req.Params, "watchId", "")
		if !strings.HasPrefix(id, archiveWatchPrefix) {
			return false
		}
		w.Respond(req.ID, map[string]any{"watchId": id, "closed": true})
	case "files.thumbnail":
		if !strings.HasPrefix(params.StringOpt(req.Params, "watchId", ""), archiveWatchPrefix) {
			return false
		}
		w.Respond(req.ID, map[string]any{"size": params.StringOpt(req.Params, "size", "normal"), "results": []any{}})
	case "files.count":
		paths := wire.StringList(req.Params, "paths")
		if len(paths) == 0 || !filepath.IsAbs(paths[0]) {
			return false
		}
		if _, _, ok := fileops.LocateArchive(paths[0]); !ok {
			return false
		}
		w.Respond(req.ID, map[string]any{"counts": archiveCounts(deps, paths, params.BoolLoose(req.Params, "includeHidden"))})
	default:
		return false
	}
	return true
}

func respondArchiveListing(w *ConnWriter, req Request, deps Deps, archive, inner string) {
	children, err := deps.FileOps.ArchiveChildren(archive, inner)
	if err != nil {
		wire.RespondError(w, req.ID, err)
		return
	}
	view := archiveViewOf(req)
	dir := filepath.Join(archive, inner)
	entries := make([]files.Entry, 0, len(children))
	hidden := 0
	for _, child := range children {
		entry := archiveEntry(deps, dir, child)
		if !view.passes(entry) {
			continue
		}
		if entry.Hidden && !view.includeHidden {
			hidden++
			continue
		}
		entries = append(entries, entry)
	}
	slices.SortStableFunc(entries, view.compare)

	start := min(view.cursor, len(entries))
	end := min(start+view.limit, len(entries))
	listing := archiveListing{Path: dir, WatchID: archiveWatchID(archive, inner), Entries: entries[start:end], Total: len(entries), Hidden: hidden}
	if end < len(entries) {
		listing.Cursor = strconv.Itoa(end)
	}
	w.Respond(req.ID, listing)
}

func respondArchiveStat(w *ConnWriter, req Request, deps Deps, archive, inner string) {
	children, err := deps.FileOps.ArchiveChildren(archive, filepath.Dir(inner))
	if err != nil {
		wire.RespondError(w, req.ID, err)
		return
	}
	at := slices.IndexFunc(children, func(child fileops.ArchiveEntry) bool { return child.Name == filepath.Base(inner) })
	if at < 0 {
		wire.RespondError(w, req.ID, &files.Error{Code: files.CodeNotFound, Path: filepath.Join(archive, inner), Err: fs.ErrNotExist})
		return
	}
	w.Respond(req.ID, map[string]any{"entry": archiveEntry(deps, filepath.Join(archive, filepath.Dir(inner)), children[at])})
}

func archiveCounts(deps Deps, paths []string, includeHidden bool) map[string]files.Count {
	counts := make(map[string]files.Count, len(paths))
	for _, path := range paths {
		archive, inner, ok := fileops.LocateArchive(path)
		if !ok {
			counts[path] = files.Count{Code: string(files.CodeNotDir)}
			continue
		}
		children, err := deps.FileOps.ArchiveChildren(archive, inner)
		if err != nil {
			counts[path] = files.Count{Code: string(files.CodeOf(err))}
			continue
		}
		count := 0
		for _, child := range children {
			if includeHidden || !strings.HasPrefix(child.Name, ".") {
				count++
			}
		}
		counts[path] = files.Count{Count: count}
	}
	return counts
}

func archiveEntry(deps Deps, dir string, child fileops.ArchiveEntry) files.Entry {
	entry := files.Entry{
		Name:          child.Name,
		Path:          filepath.Join(dir, child.Name),
		IsDir:         child.Dir,
		IsSymlink:     child.Symlink,
		SymlinkTarget: child.Link,
		Size:          child.Size,
		Mode:          child.Perm.String(),
		Hidden:        strings.HasPrefix(child.Name, "."),
		Extension:     strings.ToLower(strings.TrimPrefix(filepath.Ext(child.Name), ".")),
	}
	if !child.Modified.IsZero() {
		entry.MtimeMs = child.Modified.UnixMilli()
	}
	if child.Dir {
		entry.Size = -1
		entry.Mode = (child.Perm | fs.ModeDir).String()
		entry.Mime = files.MimeDirectory
		entry.IconName = "folder"
		return entry
	}
	entry.Executable = !child.Symlink && child.Perm&0o111 != 0
	typeArchiveFile(deps, &entry)
	if child.Symlink {
		entry.Size = -1
		entry.Mode = (child.Perm | fs.ModeSymlink).String()
		entry.IconName = "inode-symlink"
	}
	return entry
}

func typeArchiveFile(deps Deps, entry *files.Entry) {
	entry.IconName = genericFileIcon
	if deps.Apps == nil {
		return
	}
	detector := deps.Apps.Detector()
	entry.Mime = detector.FromName(entry.Name)
	if entry.Mime == "" {
		return
	}
	entry.IconName = strings.ReplaceAll(entry.Mime, "/", "-")
	if specific, _ := detector.MimeIcon(entry.Mime); specific != "" {
		entry.IconName = specific
	}
}

func archiveViewOf(req Request) archiveView {
	view := archiveView{
		includeHidden: params.BoolLoose(req.Params, "includeHidden"),
		sortKey:       files.ParseSortKey(params.StringOpt(req.Params, "sort", "")),
		desc:          params.BoolLoose(req.Params, "desc"),
		dirsFirst:     req.Params["dirsFirst"] == nil || params.BoolLoose(req.Params, "dirsFirst"),
		hiddenLast:    params.BoolLoose(req.Params, "hiddenLast"),
		limit:         looseInt(req.Params, "limit"),
		cursor:        max(looseInt(req.Params, "cursor"), 0),
	}
	if view.limit <= 0 {
		view.limit = files.DefaultPageSize
	}
	for _, pattern := range wire.StringList(req.Params, "filters") {
		pattern = strings.ToLower(strings.TrimSpace(pattern))
		switch pattern {
		case "":
			continue
		case "*", "*.*":
			view.filters = nil
			return view
		}
		view.filters = append(view.filters, pattern)
	}
	return view
}

func looseInt(p map[string]any, key string) int {
	switch value := p[key].(type) {
	case float64:
		return int(value)
	case string:
		parsed, _ := strconv.Atoi(value)
		return parsed
	}
	return 0
}

func (v archiveView) passes(entry files.Entry) bool {
	if len(v.filters) == 0 || entry.IsDir {
		return true
	}
	name := strings.ToLower(entry.Name)
	return slices.ContainsFunc(v.filters, func(pattern string) bool {
		matched, _ := filepath.Match(pattern, name)
		return matched
	})
}

func (v archiveView) compare(a, b files.Entry) int {
	if v.dirsFirst && a.IsDir != b.IsDir {
		return cmp.Compare(boolRank(b.IsDir), boolRank(a.IsDir))
	}
	if v.hiddenLast && a.Hidden != b.Hidden {
		return cmp.Compare(boolRank(a.Hidden), boolRank(b.Hidden))
	}
	order := 0
	switch v.sortKey {
	case files.SortSize:
		order = cmp.Compare(a.Size, b.Size)
	case files.SortMtime:
		order = cmp.Compare(a.MtimeMs, b.MtimeMs)
	case files.SortType:
		order = cmp.Compare(a.Mime, b.Mime)
	default:
		order = naturalCompare(a.Name, b.Name)
	}
	if v.desc {
		order = -order
	}
	return cmp.Or(order, naturalCompare(a.Name, b.Name))
}

func boolRank(value bool) int {
	if value {
		return 1
	}
	return 0
}

func naturalCompare(a, b string) int {
	left, right := a, b
	for left != "" && right != "" {
		if isDigit(left[0]) && isDigit(right[0]) {
			leftRun, leftRest := digitRun(left)
			rightRun, rightRest := digitRun(right)
			if order := cmp.Or(cmp.Compare(len(leftRun), len(rightRun)), strings.Compare(leftRun, rightRun)); order != 0 {
				return order
			}
			left, right = leftRest, rightRest
			continue
		}
		leftRune, leftSize := utf8.DecodeRuneInString(left)
		rightRune, rightSize := utf8.DecodeRuneInString(right)
		if order := cmp.Compare(unicode.ToLower(leftRune), unicode.ToLower(rightRune)); order != 0 {
			return order
		}
		left, right = left[leftSize:], right[rightSize:]
	}
	return cmp.Or(cmp.Compare(len(left), len(right)), strings.Compare(a, b))
}

func isDigit(char byte) bool { return char >= '0' && char <= '9' }

func digitRun(text string) (run, rest string) {
	end := 0
	for end < len(text) && isDigit(text[end]) {
		end++
	}
	return strings.TrimLeft(text[:end], "0"), text[end:]
}

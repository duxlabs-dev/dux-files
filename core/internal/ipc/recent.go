package ipc

import (
	"context"

	"github.com/duxlabs-dev/dux-files/core/internal/recent"
	"github.com/duxlabs-dev/dux-files/core/internal/wire"
	"github.com/duxlabs-dev/dux-go/files"
	"github.com/duxlabs-dev/dux-go/ipc/params"
)

const recentLimit = 1000

type RecentItem struct {
	files.Entry
	URI    string `json:"uri"`
	UsedMs int64  `json:"usedMs"`
}

func HandleRecent(_ context.Context, w *ConnWriter, req Request, deps Deps) {
	store := deps.Recent
	if store == nil {
		wire.RespondError(w, req.ID, wire.Fail(files.CodeNotSupported, "recent list unavailable"))
		return
	}
	switch req.Method {
	case "recent.entries":
		if deps.Files == nil {
			wire.RespondError(w, req.ID, wire.Fail(files.CodeNotSupported, "file service unavailable"))
			return
		}
		if known := params.StringOpt(req.Params, "stamp", ""); known != "" && known == store.Stamp() {
			w.Respond(req.ID, map[string]any{"stamp": known, "unchanged": true})
			return
		}
		items, stamp, err := store.Load()
		if err != nil {
			wire.RespondError(w, req.ID, err)
			return
		}
		w.Respond(req.ID, map[string]any{"stamp": stamp, "entries": recentItems(items, deps.Files.Stat)})
	case "recent.remove":
		targets := wire.StringList(req.Params, "paths")
		if len(targets) == 0 {
			wire.RespondError(w, req.ID, wire.Invalid("recent.remove requires paths"))
			return
		}
		if err := store.Remove(targets); err != nil {
			wire.RespondError(w, req.ID, err)
			return
		}
		w.Respond(req.ID, map[string]any{"ok": true, "stamp": store.Stamp()})
	case "recent.clear":
		if err := store.Clear(); err != nil {
			wire.RespondError(w, req.ID, err)
			return
		}
		w.Respond(req.ID, map[string]any{"ok": true, "stamp": store.Stamp()})
	default:
		w.RespondError(req.ID, "unknown recent method: "+req.Method)
	}
}

func recentItems(items []recent.Item, stat func(string) (files.Entry, error)) []RecentItem {
	recent.SortByUsed(items)
	out := make([]RecentItem, 0, min(len(items), recentLimit))
	seen := map[string]bool{}
	for _, item := range items {
		if len(out) == recentLimit {
			break
		}
		path, ok := item.Path()
		if !ok || item.Private || seen[path] {
			continue
		}
		seen[path] = true
		entry, err := stat(path)
		if err != nil {
			continue
		}
		out = append(out, RecentItem{Entry: entry, URI: item.URI, UsedMs: item.Used().UnixMilli()})
	}
	newestFirst(out, func(i RecentItem) (int64, string) { return i.UsedMs, i.Name })
	return out
}

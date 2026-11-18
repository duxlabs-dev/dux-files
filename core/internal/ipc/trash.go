package ipc

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/duxlabs-dev/dux-files/core/internal/wire"

	"github.com/duxlabs-dev/dux-files/core/internal/fileops"
	"github.com/duxlabs-dev/dux-go/files"
	"github.com/duxlabs-dev/dux-go/trash"
)

const trashDateLayout = "2006-01-02T15:04:05"

type TrashItem struct {
	files.Entry
	TrashName    string `json:"trashName"`
	TrashDir     string `json:"trashDir"`
	OriginalPath string `json:"originalPath"`
	DeletedMs    int64  `json:"deletedMs"`
}

var listTrash = trash.List

func HandleTrash(_ context.Context, w *ConnWriter, req Request, deps Deps) {
	switch req.Method {
	case "trash.list":
		entries, err := listTrash()
		if err != nil {
			wire.RespondError(w, req.ID, err)
			return
		}
		if entries == nil {
			entries = []trash.Entry{}
		}
		w.Respond(req.ID, map[string]any{"entries": entries})
	case "trash.entries":
		if deps.Files == nil {
			wire.RespondError(w, req.ID, wire.Fail(files.CodeNotSupported, "file service unavailable"))
			return
		}
		entries, err := listTrash()
		if err != nil {
			wire.RespondError(w, req.ID, err)
			return
		}
		w.Respond(req.ID, map[string]any{"entries": trashItems(entries, deps.Files.Stat)})
	case "trash.count":
		count, err := trash.Count()
		if err != nil {
			wire.RespondError(w, req.ID, err)
			return
		}
		w.Respond(req.ID, map[string]any{"count": count, "stamp": trashStamp(count)})
	default:
		w.RespondError(req.ID, "unknown trash method: "+req.Method)
	}
}

func trashItems(entries []trash.Entry, stat func(string) (files.Entry, error)) []TrashItem {
	items := make([]TrashItem, 0, len(entries))
	for _, e := range entries {
		entry, err := stat(e.FilesPath)
		if err != nil {
			continue
		}
		original, _ := fileops.RestorePath(e.TrashDir, e.OriginalPath)
		if original != "" {
			entry.Name = filepath.Base(original)
		}
		items = append(items, TrashItem{
			Entry:        entry,
			TrashName:    e.Name,
			TrashDir:     e.TrashDir,
			OriginalPath: original,
			DeletedMs:    deletedMs(e),
		})
	}
	newestFirst(items, func(i TrashItem) (int64, string) { return i.DeletedMs, i.Name })
	return items
}

func trashStamp(count int) string {
	stamp := strconv.Itoa(count)
	info, err := os.Stat(filepath.Join(fileops.HomeTrash(), "info"))
	if err != nil {
		return stamp
	}
	return stamp + ":" + strconv.FormatInt(info.ModTime().UnixNano(), 10)
}

func deletedMs(e trash.Entry) int64 {
	if when, err := time.ParseInLocation(trashDateLayout, e.DeletionDate, time.Local); err == nil {
		return when.UnixMilli()
	}
	info, err := os.Lstat(e.InfoPath)
	if err != nil {
		return 0
	}
	return info.ModTime().UnixMilli()
}

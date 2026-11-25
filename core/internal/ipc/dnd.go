package ipc

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/duxlabs-dev/dux-files/core/internal/wire"
	"github.com/duxlabs-dev/dux-go/files"
	"github.com/duxlabs-dev/dux-go/ipc/params"
	"github.com/godbus/dbus/v5"
)

const (
	arkDndInterface = "org.kde.ark.DndExtract"
	arkDndMethod    = arkDndInterface + ".extractSelectedFilesTo"
	arkDndPath      = "/DndExtract/"
)

var dndSessionBus = dbus.SessionBus

func HandleDndExtract(ctx context.Context, w *ConnWriter, req Request) {
	service := params.StringOpt(req.Params, "service", "")
	path := params.StringOpt(req.Params, "path", "")
	dest := params.StringOpt(req.Params, "dest", "")

	switch {
	case !strings.HasPrefix(service, ":") || len(service) == 1:
		wire.RespondError(w, req.ID, wire.Invalid("files.dndExtract requires Ark's unique bus name"))
		return
	case !strings.HasPrefix(path, arkDndPath) || !dbus.ObjectPath(path).IsValid():
		wire.RespondError(w, req.ID, wire.Invalid("files.dndExtract requires an Ark DndExtract object path"))
		return
	case !filepath.IsAbs(dest):
		wire.RespondError(w, req.ID, wire.Invalid("files.dndExtract requires an absolute dest"))
		return
	}

	info, err := os.Stat(dest)
	if err != nil || !info.IsDir() {
		wire.RespondError(w, req.ID, wire.Invalid("files.dndExtract dest is not a directory"))
		return
	}

	conn, err := dndSessionBus()
	if err != nil {
		wire.RespondError(w, req.ID, wire.Fail(files.CodeIO, "session bus unavailable: %v", err))
		return
	}

	callCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	call := conn.Object(service, dbus.ObjectPath(path)).
		CallWithContext(callCtx, arkDndMethod, 0, dest)
	if call.Err != nil {
		wire.RespondError(w, req.ID, wire.Fail(files.CodeIO, "ark drag extraction failed: %v", call.Err))
		return
	}

	w.Respond(req.ID, map[string]any{"ok": true})
}

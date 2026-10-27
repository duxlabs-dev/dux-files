package ipc

import (
	"context"
	"strings"

	"github.com/duxlabs-dev/dux-files/core/internal/bookmarks"
	"github.com/duxlabs-dev/dux-files/core/internal/clipboard"
	"github.com/duxlabs-dev/dux-files/core/internal/devices"
	"github.com/duxlabs-dev/dux-files/core/internal/games"
	"github.com/duxlabs-dev/dux-files/core/internal/mimeapps"
	"github.com/duxlabs-dev/dux-files/core/internal/netmount"
	"github.com/duxlabs-dev/dux-files/core/internal/search"
	"github.com/duxlabs-dev/dux-go/ipc/params"
)

func Route(ctx context.Context, w *ConnWriter, req Request, deps Deps) {
	switch req.Method {
	case "version":
		w.Respond(req.ID, map[string]any{"version": deps.Version, "apiVersion": APIVersion})
		return
	case "describe":
		w.Respond(req.ID, map[string]any{"apiVersion": APIVersion, "methods": Methods})
		return
	}

	switch {
	case devices.Handles(req.Method):
		if deps.Devices == nil {
			w.RespondError(req.ID, "device service unavailable")
			return
		}
		deps.Devices.Handle(w, req)
	case netmount.Handles(req.Method):
		if deps.Network == nil {
			w.RespondError(req.ID, "network service unavailable")
			return
		}
		deps.Network.Handle(w, req)
	case clipboard.Handles(req.Method):
		if deps.Clipboard == nil {
			w.RespondError(req.ID, "clipboard unavailable")
			return
		}
		deps.Clipboard.Handle(w, req)
	case mimeapps.Handles(req.Method):
		if deps.Apps == nil {
			w.RespondError(req.ID, "application service unavailable")
			return
		}
		deps.Apps.Handle(ctx, w, withTerminalDefault(req, deps))
	case search.Handles(req.Method):
		if deps.Search == nil {
			w.RespondError(req.ID, "search unavailable")
			return
		}
		deps.Search.Handle(ctx, w, req)
	case bookmarks.Handles(req.Method):
		if deps.Bookmarks == nil {
			w.RespondError(req.ID, "bookmarks unavailable")
			return
		}
		deps.Bookmarks.Handle(w, req)
	case games.Handles(req.Method):
		games.Handle(w, req)
	case strings.HasPrefix(req.Method, "files.op."):
		if deps.FileOps == nil {
			w.RespondError(req.ID, "file operations unavailable")
			return
		}
		deps.FileOps.Handle(ctx, w, req)
	case strings.HasPrefix(req.Method, "ui."):
		HandleUI(ctx, w, req, deps)
	case strings.HasPrefix(req.Method, "system."):
		HandleSystem(ctx, w, req, deps)
	case strings.HasPrefix(req.Method, "portal."):
		HandlePortal(ctx, w, req, deps)
	case strings.HasPrefix(req.Method, "trash."):
		HandleTrash(ctx, w, req, deps)
	case strings.HasPrefix(req.Method, "recent."):
		HandleRecent(ctx, w, req, deps)
	case strings.HasPrefix(req.Method, "location."):
		HandleLocation(ctx, w, req)
	case req.Method == "files.preview":
		HandlePreview(ctx, w, req)
	case req.Method == "files.imageSize":
		HandleImageSize(ctx, w, req)
	case req.Method == "files.iconCatalog":
		HandleIconCatalog(ctx, w, req)
	case req.Method == "files.iconThemes":
		HandleIconThemes(ctx, w, req)
	case req.Method == "files.iconThemePaths":
		HandleIconThemePaths(ctx, w, req)
	case req.Method == "files.setIconTheme":
		HandleSetIconTheme(ctx, w, req, deps)
	case strings.HasPrefix(req.Method, "files."):
		if deps.Files == nil {
			w.RespondError(req.ID, "file service unavailable")
			return
		}
		deps.Files.Handle(ctx, w, req)
	default:
		w.RespondError(req.ID, "unknown method: "+req.Method)
	}
}

func withTerminalDefault(req Request, deps Deps) Request {
	if deps.Settings == nil || params.StringOpt(req.Params, "terminal", "") != "" {
		return req
	}
	terminal := deps.Settings().Terminal
	if terminal == "" {
		return req
	}
	params := make(map[string]any, len(req.Params)+1)
	for key, value := range req.Params {
		params[key] = value
	}
	params["terminal"] = terminal
	req.Params = params
	return req
}

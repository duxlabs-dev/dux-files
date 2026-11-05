package ipc

import (
	"context"
	"strings"

	"github.com/duxlabs-dev/dux-files/core/internal/wire"
	"github.com/duxlabs-dev/dux-go/files"

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

	for _, service := range services {
		if !service.handles(req.Method) {
			continue
		}
		if !service.available(deps) {
			wire.RespondError(w, req.ID, wire.Fail(files.CodeNotSupported, "%s unavailable", service.name))
			return
		}
		service.handle(ctx, w, req, deps)
		return
	}

	switch {
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
	case strings.HasPrefix(req.Method, "files."):
		if deps.Files == nil {
			wire.RespondError(w, req.ID, wire.Fail(files.CodeNotSupported, "file service unavailable"))
			return
		}
		deps.Files.Handle(ctx, w, req)
	default:
		w.RespondError(req.ID, "unknown method: "+req.Method)
	}
}

type service struct {
	name      string
	handles   func(method string) bool
	available func(deps Deps) bool
	handle    func(ctx context.Context, w *ConnWriter, req Request, deps Deps)
}

var services = []service{
	{"device service", devices.Handles, func(d Deps) bool { return d.Devices != nil }, func(_ context.Context, w *ConnWriter, req Request, d Deps) { d.Devices.Handle(w, req) }},
	{"network service", netmount.Handles, func(d Deps) bool { return d.Network != nil }, func(_ context.Context, w *ConnWriter, req Request, d Deps) { d.Network.Handle(w, req) }},
	{"clipboard", clipboard.Handles, func(d Deps) bool { return d.Clipboard != nil }, func(_ context.Context, w *ConnWriter, req Request, d Deps) { d.Clipboard.Handle(w, req) }},
	{"application service", mimeapps.Handles, func(d Deps) bool { return d.Apps != nil }, func(ctx context.Context, w *ConnWriter, req Request, d Deps) {
		d.Apps.Handle(ctx, w, withTerminalDefault(req, d))
	}},
	{"search", search.Handles, func(d Deps) bool { return d.Search != nil }, func(ctx context.Context, w *ConnWriter, req Request, d Deps) { d.Search.Handle(ctx, w, req) }},
	{"bookmarks", bookmarks.Handles, func(d Deps) bool { return d.Bookmarks != nil }, func(_ context.Context, w *ConnWriter, req Request, d Deps) { d.Bookmarks.Handle(w, req) }},
	{"games", games.Handles, func(Deps) bool { return true }, func(_ context.Context, w *ConnWriter, req Request, _ Deps) { games.Handle(w, req) }},
	{"file operations", func(m string) bool { return strings.HasPrefix(m, "files.op.") }, func(d Deps) bool { return d.FileOps != nil }, func(ctx context.Context, w *ConnWriter, req Request, d Deps) { d.FileOps.Handle(ctx, w, req) }},
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

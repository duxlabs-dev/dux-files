package ipc

import (
	"context"

	"github.com/duxlabs-dev/dux-files/core/internal/wire"
	"github.com/duxlabs-dev/dux-go/files"

	"github.com/duxlabs-dev/dux-files/core/internal/filemanager1"
	"github.com/duxlabs-dev/dux-files/core/internal/portal"
	"github.com/duxlabs-dev/dux-files/core/internal/sandbox"
	"github.com/duxlabs-dev/dux-go/ipc/params"
)

func HandleSystem(ctx context.Context, w *ConnWriter, req Request, deps Deps) {
	switch req.Method {
	case "system.colorScheme.get":
		if deps.ColorScheme == nil {
			w.Respond(req.ID, map[string]any{"available": false, "colorScheme": uint32(0)})
			return
		}
		w.Respond(req.ID, deps.ColorScheme.State())
	case "system.capabilities":
		w.Respond(req.ID, capabilities(ctx, deps))
	case "system.elevation":
		if deps.FileOps == nil {
			w.Respond(req.ID, map[string]any{"available": false, "reason": "NOROUTE"})
			return
		}
		w.Respond(req.ID, deps.FileOps.Elevation())
	case "system.fileManager.get":
		if deps.FileManager == nil {
			w.Respond(req.ID, filemanager1.Status{Reason: "only the window process registers"})
			return
		}
		w.Respond(req.ID, deps.FileManager.Status())
	case "system.fileManager.set":
		if deps.FileManager == nil {
			w.RespondError(req.ID, "only the window process registers as the file manager")
			return
		}
		w.Respond(req.ID, deps.FileManager.SetEnabled(params.BoolLoose(req.Params, "enabled")))
	case "system.portal.get":
		if deps.Portal == nil {
			w.Respond(req.ID, map[string]any{"available": false, "reason": portal.ReasonNoWindow})
			return
		}
		w.Respond(req.ID, deps.Portal.Status())
	case "system.portal.install", "system.portal.uninstall":
		if deps.Portal == nil {
			w.RespondError(req.ID, "only the window process serves the portal")
			return
		}
		change := deps.Portal.Install
		if req.Method == "system.portal.uninstall" {
			change = deps.Portal.Uninstall
		}
		report, err := change()
		if err != nil {
			w.RespondError(req.ID, err.Error())
			return
		}
		w.Respond(req.ID, report)
	case "system.portal.restart":
		if deps.Portal == nil {
			w.RespondError(req.ID, "only the window process serves the portal")
			return
		}
		if err := deps.Portal.Restart(); err != nil {
			w.RespondError(req.ID, err.Error())
			return
		}
		w.Respond(req.ID, map[string]any{"restarted": true})
	case "system.openUri":
		if deps.Apps == nil {
			wire.RespondError(w, req.ID, wire.Fail(files.CodeNotSupported, "uri opener unavailable"))
			return
		}
		uri := params.StringOpt(req.Params, "uri", "")
		if uri == "" {
			wire.RespondError(w, req.ID, wire.Invalid("uri is required"))
			return
		}
		if err := deps.Apps.OpenURI(ctx, uri); err != nil {
			w.RespondError(req.ID, err.Error())
			return
		}
		w.Respond(req.ID, map[string]any{"opened": true})
	case "system.setWallpaper":
		if deps.Wallpaper == nil {
			wire.RespondError(w, req.ID, wire.Fail(files.CodeNotSupported, "wallpaper service unavailable"))
			return
		}
		path := params.StringOpt(req.Params, "path", "")
		screen := params.StringOpt(req.Params, "screen", "")
		if screen == "" {
			screen = params.StringOpt(req.Params, "screenName", "")
		}
		if err := deps.Wallpaper.Set(ctx, path, screen); err != nil {
			wire.RespondError(w, req.ID, err)
			return
		}
		w.Respond(req.ID, map[string]any{"set": true})
	default:
		w.RespondError(req.ID, "unknown system method: "+req.Method)
	}
}

type capabilitiesResponse struct {
	sandbox.Capabilities
	Thumbnailers []string `json:"thumbnailers"`
	SearchTools  []string `json:"searchTools"`
}

func capabilities(ctx context.Context, deps Deps) capabilitiesResponse {
	caps := deps.Sandbox
	if deps.FileOps != nil {
		e := deps.FileOps.Elevation()
		caps = caps.WithFeature("elevation", sandbox.Feature{Available: e.Available, Reason: e.Reason})
	}
	if deps.Wallpaper != nil {
		caps = caps.WithFeature("setWallpaper", deps.Wallpaper.Feature(ctx))
	}
	if caps.Portals == nil {
		caps.Portals = map[string]uint32{}
	}
	if caps.Features == nil {
		caps.Features = map[string]sandbox.Feature{}
	}
	if caps.Filesystems == nil {
		caps.Filesystems = []string{}
	}
	out := capabilitiesResponse{Capabilities: caps, Thumbnailers: []string{}, SearchTools: []string{}}
	if deps.Files != nil {
		out.Thumbnailers = append(out.Thumbnailers, deps.Files.Capabilities()...)
	}
	if deps.Search != nil {
		out.SearchTools = append(out.SearchTools, deps.Search.Tools()...)
	}
	return out
}

package mimeapps

import (
	"context"

	"github.com/duxlabs-dev/dux-files/core/internal/wire"
	"github.com/duxlabs-dev/dux-go/files"
	"github.com/duxlabs-dev/dux-go/ipc"
	"github.com/duxlabs-dev/dux-go/ipc/params"
)

type appView struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	GenericName string `json:"genericName,omitempty"`
	Comment     string `json:"comment,omitempty"`
	Icon        string `json:"icon,omitempty"`
	Terminal    bool   `json:"terminal"`
	IsDefault   bool   `json:"isDefault"`
}

type sectionsResult struct {
	MimeTypes   []string  `json:"mimeTypes"`
	DefaultID   string    `json:"defaultId,omitempty"`
	Mixed       bool      `json:"mixed"`
	Recent      []appView `json:"recent"`
	Recommended []appView `json:"recommended"`
	Other       []appView `json:"other"`
}

type appsResult struct {
	MimeType  string    `json:"mimeType,omitempty"`
	DefaultID string    `json:"defaultId,omitempty"`
	Apps      []appView `json:"apps"`
}

func Handles(method string) bool {
	switch method {
	case "files.open", "files.openWith", "files.openTerminal", "files.run",
		"files.apps.forMime", "files.apps.sections", "files.apps.all", "files.apps.setDefault", "files.apps.terminals":
		return true
	}
	return false
}

func (s *Service) Handle(ctx context.Context, w *ipc.ConnWriter, req ipc.Request) {
	switch req.Method {
	case "files.open":
		s.handleOpen(ctx, w, req)
	case "files.openWith":
		s.handleOpenWith(ctx, w, req)
	case "files.run":
		s.handleRun(w, req)
	case "files.openTerminal":
		s.handleOpenTerminal(w, req)
	case "files.apps.forMime":
		s.handleForMime(w, req)
	case "files.apps.sections":
		s.handleSections(w, req)
	case "files.apps.all":
		w.Respond(req.ID, appsResult{Apps: viewsOf(s.Apps(), "")})
	case "files.apps.setDefault":
		s.handleSetDefault(w, req)
	case "files.apps.terminals":
		w.Respond(req.ID, map[string]any{"terminals": Terminals()})
	default:
		wire.RespondError(w, req.ID, fail(files.CodeNotSupported, "unknown method: %s", req.Method))
	}
}

func (s *Service) handleOpen(ctx context.Context, w *ipc.ConnWriter, req ipc.Request) {
	paths := params.StringSlice(req.Params, "paths")
	if len(paths) == 0 {
		wire.RespondError(w, req.ID, wire.Invalid("files.open requires paths"))
		return
	}
	if err := s.Open(ctx, paths, launchOptions(req)); err != nil {
		wire.RespondError(w, req.ID, err)
		return
	}
	w.Respond(req.ID, map[string]any{"opened": true})
}

func (s *Service) handleOpenWith(ctx context.Context, w *ipc.ConnWriter, req ipc.Request) {
	paths := params.StringSlice(req.Params, "paths")
	desktopID := params.StringOpt(req.Params, "desktopId", "")
	if len(paths) == 0 || desktopID == "" {
		wire.RespondError(w, req.ID, wire.Invalid("files.openWith requires paths and a desktopId"))
		return
	}
	if err := s.OpenWith(ctx, desktopID, paths, launchOptions(req)); err != nil {
		wire.RespondError(w, req.ID, err)
		return
	}
	recorded := (!Sandboxed() || writableConfig()) && params.BoolOpt(req.Params, "record", true) && s.RecordUseForPaths(paths, desktopID) == nil
	w.Respond(req.ID, map[string]any{"opened": true, "recorded": recorded})
}

func (s *Service) handleRun(w *ipc.ConnWriter, req ipc.Request) {
	path := params.StringOpt(req.Params, "path", "")
	if path == "" {
		wire.RespondError(w, req.ID, wire.Invalid("files.run requires a path"))
		return
	}
	if err := s.Run(path, launchOptions(req)); err != nil {
		wire.RespondError(w, req.ID, err)
		return
	}
	w.Respond(req.ID, map[string]any{"started": true})
}

func (s *Service) handleOpenTerminal(w *ipc.ConnWriter, req ipc.Request) {
	path := params.StringOpt(req.Params, "path", "")
	if path == "" {
		wire.RespondError(w, req.ID, wire.Invalid("files.openTerminal requires a path"))
		return
	}
	if err := s.OpenTerminal(path, params.StringOpt(req.Params, "terminal", "")); err != nil {
		wire.RespondError(w, req.ID, err)
		return
	}
	w.Respond(req.ID, map[string]any{"opened": true})
}

func (s *Service) handleForMime(w *ipc.ConnWriter, req ipc.Request) {
	mimeType := params.StringOpt(req.Params, "mime", "")
	if path := params.StringOpt(req.Params, "path", ""); mimeType == "" && path != "" {
		resolved, err := s.MimeOf(path)
		if err != nil {
			wire.RespondError(w, req.ID, err)
			return
		}
		mimeType = resolved
	}
	if mimeType == "" {
		wire.RespondError(w, req.ID, wire.Invalid("files.apps.forMime requires a mime or a path"))
		return
	}

	defaultID := s.Default(mimeType)
	w.Respond(req.ID, appsResult{
		MimeType:  mimeType,
		DefaultID: defaultID,
		Apps:      viewsOf(shown(s.Associations(mimeType), defaultID), defaultID),
	})
}

func (s *Service) handleSections(w *ipc.ConnWriter, req ipc.Request) {
	mimeTypes := params.StringSlice(req.Params, "mimes")
	for _, path := range params.StringSlice(req.Params, "paths") {
		resolved, err := s.MimeOf(path)
		if err != nil {
			wire.RespondError(w, req.ID, err)
			return
		}
		mimeTypes = append(mimeTypes, resolved)
	}
	if len(mimeTypes) == 0 {
		wire.RespondError(w, req.ID, wire.Invalid("files.apps.sections requires mimes or paths"))
		return
	}

	sections := s.Sections(mimeTypes)
	w.Respond(req.ID, sectionsResult{
		MimeTypes:   sections.MimeTypes,
		DefaultID:   sections.DefaultID,
		Mixed:       len(sections.MimeTypes) > 1,
		Recent:      viewsOf(sections.Recent, sections.DefaultID),
		Recommended: viewsOf(sections.Recommended, sections.DefaultID),
		Other:       viewsOf(sections.Other, sections.DefaultID),
	})
}

func (s *Service) handleSetDefault(w *ipc.ConnWriter, req ipc.Request) {
	mimeType := params.StringOpt(req.Params, "mime", "")
	desktopID := params.StringOpt(req.Params, "desktopId", "")
	if mimeType == "" || desktopID == "" {
		wire.RespondError(w, req.ID, wire.Invalid("files.apps.setDefault requires a mime and a desktopId"))
		return
	}
	if Sandboxed() && !writableConfig() {
		wire.RespondError(w, req.ID, fail(files.CodeSandboxed, "the sandbox cannot write mimeapps.list"))
		return
	}
	if err := s.SetDefault(mimeType, desktopID); err != nil {
		wire.RespondError(w, req.ID, &files.Error{Code: files.CodeAccess, Err: err})
		return
	}
	w.Respond(req.ID, map[string]any{"mimeType": mimeType, "desktopId": desktopID})
}

func launchOptions(req ipc.Request) LaunchOptions {
	return LaunchOptions{
		WorkingDir:      params.StringOpt(req.Params, "cwd", ""),
		ActivationToken: params.StringOpt(req.Params, "activationToken", ""),
		Terminal:        params.StringOpt(req.Params, "terminal", ""),
		Opener:          params.StringOpt(req.Params, "opener", ""),
	}
}

func viewsOf(entries []*Entry, defaultID string) []appView {
	out := make([]appView, 0, len(entries))
	for _, entry := range entries {
		out = append(out, appView{
			ID:          entry.ID,
			Name:        entry.Name,
			GenericName: entry.GenericName,
			Comment:     entry.Comment,
			Icon:        entry.Icon,
			Terminal:    entry.Terminal,
			IsDefault:   entry.ID == defaultID,
		})
	}
	return out
}

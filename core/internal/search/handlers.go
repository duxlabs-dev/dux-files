package search

import (
	"context"
	"errors"
	"strings"

	"github.com/duxlabs-dev/dux-files/core/internal/wire"
	"github.com/duxlabs-dev/dux-go/files"
	"github.com/duxlabs-dev/dux-go/ipc"
	"github.com/duxlabs-dev/dux-go/ipc/params"
)

func Handles(method string) bool {
	return strings.HasPrefix(method, "search.")
}

func (s *Service) Handle(ctx context.Context, w *ipc.ConnWriter, req ipc.Request) {
	switch req.Method {
	case "search.start":
		s.handleStart(ctx, w, req)
	case "search.cancel":
		id := params.StringOpt(req.Params, "searchId", "")
		w.Respond(req.ID, map[string]any{"searchId": id, "cancelled": s.Cancel(id)})
	default:
		wire.RespondError(w, req.ID, &files.Error{Code: files.CodeNotSupported, Err: errors.New("unknown search method: " + req.Method)})
	}
}

func (s *Service) handleStart(ctx context.Context, w *ipc.ConnWriter, req ipc.Request) {
	p := req.Params
	opts := Options{
		Scope:           params.StringOpt(p, "scope", ""),
		IncludeHidden:   params.BoolLoose(p, "includeHidden"),
		IncludeIgnored:  params.BoolLoose(p, "includeIgnored"),
		CrossDevice:     params.BoolLoose(p, "crossDevice"),
		Fuzzy:           params.BoolLoose(p, "fuzzy"),
		Contents:        params.BoolLoose(p, "contents"),
		Regex:           params.BoolLoose(p, "regex"),
		Direct:          params.BoolLoose(p, "direct"),
		Types:           wire.StringList(p, "types"),
		ModifiedAfterMs: int64(params.IntOpt(p, "modifiedAfterMs", 0)),
		MinSize:         int64(params.IntOpt(p, "minSize", 0)),
		MaxSize:         int64(params.IntOpt(p, "maxSize", 0)),
		Limit:           params.IntOpt(p, "limit", 0),
	}
	id, topic, err := s.Start(ctx, params.StringOpt(p, "root", ""), params.StringOpt(p, "query", ""), opts)
	if err != nil {
		wire.RespondError(w, req.ID, err)
		return
	}
	w.Respond(req.ID, map[string]any{"searchId": id, "topic": topic, "limit": opts.limit()})
}

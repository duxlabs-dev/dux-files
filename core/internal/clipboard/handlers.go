package clipboard

import (
	"github.com/duxlabs-dev/dux-files/core/internal/wire"
	"github.com/duxlabs-dev/dux-go/ipc"
	"github.com/duxlabs-dev/dux-go/ipc/params"
)

func (s *Service) Handle(w *ipc.ConnWriter, req ipc.Request) {
	switch req.Method {
	case "clipboard.copy":
		s.handleCopy(w, req)
	case "clipboard.paths":
		s.handlePaths(w, req)
	case "clipboard.clear":
		s.Clear()
		w.Respond(req.ID, map[string]any{"cleared": true})
	default:
		w.RespondError(req.ID, "unknown clipboard method: "+req.Method)
	}
}

func (s *Service) handleCopy(w *ipc.ConnWriter, req ipc.Request) {
	paths := wire.StringList(req.Params, "paths")
	if len(paths) == 0 {
		wire.RespondError(w, req.ID, wire.Invalid("clipboard.copy requires paths"))
		return
	}
	cut := params.BoolLoose(req.Params, "cut")
	if err := s.Copy(paths, cut); err != nil {
		w.RespondError(req.ID, err.Error())
		return
	}
	w.Respond(req.ID, map[string]any{"paths": paths, "cut": cut})
}

func (s *Service) handlePaths(w *ipc.ConnWriter, req ipc.Request) {
	paths, cut, source := s.Paths()
	w.Respond(req.ID, map[string]any{"paths": paths, "cut": cut, "source": source})
}

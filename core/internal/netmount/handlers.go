package netmount

import (
	"github.com/duxlabs-dev/dux-files/core/internal/wire"
	"github.com/duxlabs-dev/dux-go/files"
	"github.com/duxlabs-dev/dux-go/ipc"
	"github.com/duxlabs-dev/dux-go/ipc/params"
)

func Handles(method string) bool {
	switch method {
	case "network.mounts", "network.connect", "network.disconnect", "network.answer",
		"network.recent", "network.forget", "network.mountPortable":
		return true
	}
	return false
}

func (s *Service) Handle(w *ipc.ConnWriter, req ipc.Request) {
	switch req.Method {
	case "network.mounts":
		snapshot := s.Snapshot()
		w.Respond(req.ID, map[string]any{
			"available": snapshot.Available,
			"reason":    snapshot.Reason,
			"schemes":   snapshot.Schemes,
			"mounts":    snapshot.Mounts,
			"portables": s.Portables(),
		})
	case "network.connect":
		s.handleConnect(w, req)
	case "network.disconnect":
		s.handleDisconnect(w, req)
	case "network.answer":
		s.handleAnswer(w, req)
	case "network.recent":
		w.Respond(req.ID, map[string]any{"servers": s.Servers()})
	case "network.forget":
		s.Forget(params.StringOpt(req.Params, "uri", ""))
		w.Respond(req.ID, map[string]any{"servers": s.Servers()})
	case "network.mountPortable":
		s.handleMountPortable(w, req)
	default:
		wire.RespondError(w, req.ID, wire.Fail(files.CodeNotSupported, "unknown network method: %s", req.Method))
	}
}

func (s *Service) handleConnect(w *ipc.ConnWriter, req ipc.Request) {
	uri := params.StringOpt(req.Params, "uri", "")
	if uri == "" {
		wire.RespondError(w, req.ID, wire.Invalid("network.connect requires a uri"))
		return
	}
	connected, err := s.Connect(uri, params.StringOpt(req.Params, "password", ""))
	if err != nil {
		wire.RespondError(w, req.ID, err)
		return
	}
	w.Respond(req.ID, connected)
}

func (s *Service) handleDisconnect(w *ipc.ConnWriter, req ipc.Request) {
	id := params.StringOpt(req.Params, "id", "")
	if id == "" {
		wire.RespondError(w, req.ID, wire.Invalid("network.disconnect requires an id"))
		return
	}
	if err := s.Disconnect(id); err != nil {
		wire.RespondError(w, req.ID, err)
		return
	}
	w.Respond(req.ID, map[string]any{"id": id, "mounted": false})
}

func (s *Service) handleAnswer(w *ipc.ConnWriter, req ipc.Request) {
	id := params.StringOpt(req.Params, "id", "")
	if id == "" {
		wire.RespondError(w, req.ID, wire.Invalid("network.answer requires an id"))
		return
	}

	answer := Answer{
		Abort:     params.BoolLoose(req.Params, "abort"),
		Password:  params.StringOpt(req.Params, "password", ""),
		User:      params.StringOpt(req.Params, "user", ""),
		Domain:    params.StringOpt(req.Params, "domain", ""),
		Anonymous: params.BoolLoose(req.Params, "anonymous"),
		Save:      uint32(params.IntOpt(req.Params, "save", 0)),
		Choice:    uint32(params.IntOpt(req.Params, "choice", 0)),
	}
	if err := s.Answer(id, answer); err != nil {
		wire.RespondError(w, req.ID, err)
		return
	}
	w.Respond(req.ID, map[string]any{"id": id, "answered": true})
}

func (s *Service) handleMountPortable(w *ipc.ConnWriter, req ipc.Request) {
	monitor := params.StringOpt(req.Params, "monitor", "")
	id := params.StringOpt(req.Params, "id", "")
	if monitor == "" || id == "" {
		wire.RespondError(w, req.ID, wire.Invalid("network.mountPortable requires a monitor and an id"))
		return
	}
	if err := s.MountPortable(monitor, id); err != nil {
		wire.RespondError(w, req.ID, err)
		return
	}
	w.Respond(req.ID, map[string]any{"id": id, "mounted": true})
}

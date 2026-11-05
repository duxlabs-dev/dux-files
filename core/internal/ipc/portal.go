package ipc

import (
	"context"
	"encoding/json"

	"github.com/duxlabs-dev/dux-files/core/internal/wire"
	"github.com/duxlabs-dev/dux-go/files"

	"github.com/duxlabs-dev/dux-files/core/internal/filechooser"
	"github.com/duxlabs-dev/dux-go/ipc/params"
)

type ChooserUI struct{ Deps Deps }

func (u ChooserUI) ShowChooser(req filechooser.Request) {
	publishUI(u.Deps, map[string]any{"action": "chooser", "request": req})
}

func (u ChooserUI) CloseChooser(id string) {
	publishUI(u.Deps, map[string]any{"action": "chooserClose", "id": id})
}

func republishChoosers(deps Deps) {
	if deps.Portal == nil {
		return
	}
	for _, req := range deps.Portal.Pending() {
		deps.Bus.Publish("ui", map[string]any{"action": "chooser", "request": req})
	}
}

func HandlePortal(_ context.Context, w *ConnWriter, req Request, deps Deps) {
	switch req.Method {
	case "portal.respond":
		if deps.Portal == nil {
			wire.RespondError(w, req.ID, wire.Fail(files.CodeNotSupported, "portal backend unavailable"))
			return
		}
		id := params.StringOpt(req.Params, "id", "")
		answer, err := answerFrom(req.Params)
		switch {
		case id == "":
			wire.RespondError(w, req.ID, wire.Invalid("id is required"))
		case err != nil:
			wire.RespondError(w, req.ID, err)
		case !deps.Portal.Answer(id, answer):
			wire.RespondError(w, req.ID, wire.Fail(files.CodeNotFound, "no open request %s", id))
		default:
			w.Respond(req.ID, map[string]any{"ok": true})
		}
	default:
		w.RespondError(req.ID, "unknown portal method: "+req.Method)
	}
}

func answerFrom(params map[string]any) (filechooser.Answer, error) {
	data, err := json.Marshal(params)
	if err != nil {
		return filechooser.Answer{}, err
	}
	answer := filechooser.Answer{CurrentFilter: -1}
	if err := json.Unmarshal(data, &answer); err != nil {
		return filechooser.Answer{}, err
	}
	if answer.Paths == nil {
		answer.Paths = []string{}
	}
	return answer, nil
}

package ipc

import (
	"context"
	"errors"
	"strings"

	"github.com/duxlabs-dev/dux-files/core/internal/fileops"
	"github.com/duxlabs-dev/dux-files/core/internal/location"
	"github.com/duxlabs-dev/dux-files/core/internal/wire"
	"github.com/duxlabs-dev/dux-go/ipc/params"
)

func hostEnv() location.Env {
	return location.HomeEnv(fileops.IsArchive)
}

func publishUI(deps Deps, payload map[string]any) {
	if deps.Pending == nil {
		deps.Bus.Publish("ui", payload)
		return
	}
	deps.Pending.Deliver(deps.Bus, payload)
}

func OpenLocations(deps Deps, open, sel []string, newWindow bool) error {
	if len(open)+len(sel) == 0 && !newWindow {
		return errors.New("nothing to open")
	}
	locations, err := location.Resolve(open, sel, hostEnv())
	if err != nil {
		return err
	}
	if locations == nil {
		locations = []location.Location{}
	}
	payload := map[string]any{"action": "open", "locations": locations}
	if newWindow {
		payload["newWindow"] = true
	}
	publishUI(deps, payload)
	return nil
}

func ShowProperties(deps Deps, targets []string) error {
	if len(targets) == 0 {
		return errors.New("nothing to show")
	}
	resolved, err := location.LocalPaths(targets, hostEnv())
	if err != nil {
		return err
	}
	publishUI(deps, map[string]any{"action": "properties", "paths": resolved})
	return nil
}

func HandleUI(_ context.Context, w *ConnWriter, req Request, deps Deps) {
	switch req.Method {
	case "ui.show", "ui.hide", "ui.toggle":
		action := strings.TrimPrefix(req.Method, "ui.")
		deps.Bus.Publish("ui", map[string]any{"action": action})
		w.Respond(req.ID, map[string]any{"ok": true})
	case "ui.open":
		open := wire.StringList(req.Params, "paths")
		if path := params.StringOpt(req.Params, "path", ""); path != "" {
			open = append([]string{path}, open...)
		}
		if err := OpenLocations(deps, open, wire.StringList(req.Params, "select"), params.BoolLoose(req.Params, "newWindow")); err != nil {
			wire.RespondError(w, req.ID, err)
			return
		}
		w.Respond(req.ID, map[string]any{"ok": true})
	case "ui.properties":
		if err := ShowProperties(deps, wire.StringList(req.Params, "paths")); err != nil {
			wire.RespondError(w, req.ID, err)
			return
		}
		w.Respond(req.ID, map[string]any{"ok": true})
	case "ui.quit":
		w.Respond(req.ID, map[string]any{"ok": true})
		if deps.Quit != nil {
			deps.Quit()
		}
	default:
		w.RespondError(req.ID, "unknown ui method: "+req.Method)
	}
}

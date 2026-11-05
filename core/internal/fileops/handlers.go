package fileops

import (
	"context"
	"errors"
	"os/user"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/duxlabs-dev/dux-files/core/internal/pathutil"

	"github.com/duxlabs-dev/dux-files/core/internal/helper"
	"github.com/duxlabs-dev/dux-files/core/internal/wire"
	"github.com/duxlabs-dev/dux-go/files"
	"github.com/duxlabs-dev/dux-go/ipc"
	"github.com/duxlabs-dev/dux-go/ipc/params"
)

func (m *Manager) Handle(_ context.Context, w *ipc.ConnWriter, req ipc.Request) {
	switch req.Method {
	case "files.op.jobs":
		w.Respond(req.ID, map[string]any{"jobs": m.Jobs()})
		return
	case "files.op.cancel":
		m.handleCancel(w, req)
		return
	case "files.op.respond":
		m.handleRespond(w, req)
		return
	case "files.op.undo":
		m.handleUndo(w, req)
		return
	case "files.op.redo":
		m.handleRedo(w, req)
		return
	case "files.op.forget":
		m.Forget(params.StringOpt(req.Params, "scope", ""))
		w.Respond(req.ID, map[string]any{"forgotten": true})
		return
	case "files.op.pasteImage":
		m.handlePasteImage(w, req)
		return
	case "files.op.dropAction":
		handleDropAction(w, req)
		return
	}

	s, err := specOf(req)
	if err != nil {
		wire.RespondError(w, req.ID, err)
		return
	}
	w.Respond(req.ID, map[string]any{"job": m.enqueue(s).view()})
}

func handleDropAction(w *ipc.ConnWriter, req ipc.Request) {
	sources, err := absPaths(req, "paths")
	if err != nil {
		wire.RespondError(w, req.ID, err)
		return
	}
	dest := params.StringOpt(req.Params, "dest", "")
	if len(sources) == 0 || !filepath.IsAbs(dest) {
		wire.RespondError(w, req.ID, wire.Invalid("files.op.dropAction requires paths and an absolute dest"))
		return
	}

	destDev, same := deviceOf(dest)
	for _, src := range sources {
		if !same {
			break
		}
		srcDev, found := deviceOf(src)
		same = found && srcDev == destDev
	}
	action := "copy"
	if same {
		action = "move"
	}
	w.Respond(req.ID, map[string]any{"action": action, "sameDevice": same})
}

func specOf(req ipc.Request) (spec, error) {
	s := spec{scope: params.StringOpt(req.Params, "scope", "")}

	switch req.Method {
	case "files.op.copy", "files.op.move", "files.op.link":
		s.kind = Kind(strings.TrimPrefix(req.Method, "files.op."))
		sources, err := absPaths(req, "paths")
		if err != nil {
			return spec{}, err
		}
		s.sources = sources
		s.dest = params.StringOpt(req.Params, "dest", "")
		if len(s.sources) == 0 || !filepath.IsAbs(s.dest) {
			return spec{}, wire.Invalid(req.Method + " requires paths and an absolute dest")
		}
		if nested := nestedSource(s.sources, s.dest); nested != "" {
			return spec{}, wire.Invalid("cannot put " + nested + " inside itself")
		}
	case "files.op.rename":
		s.kind = KindRename
		sources, err := absPaths(req, "path")
		if err != nil {
			return spec{}, err
		}
		s.sources = sources
		s.name = params.StringOpt(req.Params, "name", "")
		if len(s.sources) != 1 || !pathutil.ValidName(s.name) {
			return spec{}, wire.Invalid("files.op.rename requires an absolute path and a name")
		}
	case "files.op.newFolder", "files.op.newFile":
		s.kind = Kind(strings.TrimPrefix(req.Method, "files.op."))
		s.dest = params.StringOpt(req.Params, "path", "")
		s.name = params.StringOpt(req.Params, "name", "")
		s.template = params.StringOpt(req.Params, "template", "")
		s.unique = params.BoolLoose(req.Params, "unique")
		if !filepath.IsAbs(s.dest) || !pathutil.ValidName(s.name) {
			return spec{}, wire.Invalid(req.Method + " requires an absolute path and a name")
		}
	case "files.op.duplicate":
		s.kind = KindDuplicate
		sources, err := absPaths(req, "paths")
		if err != nil {
			return spec{}, err
		}
		s.sources = sources
		if len(s.sources) == 0 {
			return spec{}, wire.Invalid("files.op.duplicate requires absolute paths")
		}
		for _, src := range s.sources {
			if filepath.Dir(src) == src {
				return spec{}, wire.Invalid("cannot duplicate " + src)
			}
			s.pairs = append(s.pairs, pair{src: src, dst: src})
		}
	case "files.op.newFolderWith":
		s.kind = KindNewFolderWith
		sources, err := absPaths(req, "paths")
		if err != nil {
			return spec{}, err
		}
		s.sources = sources
		s.dest = params.StringOpt(req.Params, "path", "")
		s.name = params.StringOpt(req.Params, "name", "")
		s.unique = params.BoolLoose(req.Params, "unique")
		if len(s.sources) == 0 || !filepath.IsAbs(s.dest) || !pathutil.ValidName(s.name) {
			return spec{}, wire.Invalid("files.op.newFolderWith requires paths, an absolute path and a name")
		}
		if nested := nestedSource(s.sources, s.dest); nested != "" {
			return spec{}, wire.Invalid("cannot put " + nested + " inside itself")
		}
	case "files.op.trash", "files.op.delete":
		s.kind = Kind(strings.TrimPrefix(req.Method, "files.op."))
		sources, err := absPaths(req, "paths")
		if err != nil {
			return spec{}, err
		}
		s.sources = sources
		if len(s.sources) == 0 {
			return spec{}, wire.Invalid(req.Method + " requires absolute paths")
		}
	case "files.op.restore":
		s.kind = KindRestore
		s.sources = wire.StringList(req.Params, "names")
		s.trashDir = params.StringOpt(req.Params, "trashDir", "")
		if len(s.sources) == 0 || !filepath.IsAbs(s.trashDir) {
			return spec{}, wire.Invalid("files.op.restore requires names and an absolute trashDir")
		}
		for _, name := range s.sources {
			if !pathutil.ValidName(name) {
				return spec{}, wire.Invalid("not a trash entry name: " + name)
			}
		}
	case "files.op.emptyTrash":
		s.kind = KindEmptyTrash
	case "files.op.compress":
		s.kind = KindCompress
		sources, err := absPaths(req, "paths")
		if err != nil {
			return spec{}, err
		}
		s.sources = sources
		s.dest = params.StringOpt(req.Params, "dest", "")
		s.name = params.StringOpt(req.Params, "name", "")
		s.format = params.StringOpt(req.Params, "format", FormatZip)
		if len(s.sources) == 0 || !filepath.IsAbs(s.dest) || !pathutil.ValidName(s.name) {
			return spec{}, wire.Invalid("files.op.compress requires paths, an absolute dest and a name")
		}
		if !slices.Contains(compressFormats, s.format) {
			return spec{}, wire.Invalid("files.op.compress format must be one of " + strings.Join(compressFormats, ", "))
		}
		s.password = params.StringOpt(req.Params, "password", "")
		if s.password != "" && s.format != FormatZip {
			return spec{}, wire.Invalid("files.op.compress only protects zip archives with a password")
		}
	case "files.op.extract":
		s.kind = KindExtract
		sources, err := absPaths(req, "paths")
		if err != nil {
			return spec{}, err
		}
		s.sources = sources
		s.dest = params.StringOpt(req.Params, "dest", "")
		if len(s.sources) == 0 || !filepath.IsAbs(s.dest) {
			return spec{}, wire.Invalid("files.op.extract requires paths and an absolute dest")
		}
	case "files.op.setOwner":
		s.kind = KindSetOwner
		sources, err := absPaths(req, "paths")
		if err != nil {
			return spec{}, err
		}
		s.sources = sources
		s.recursive = params.BoolLoose(req.Params, "recursive")
		s.uid, err = lookupID(textParam(req.Params, "owner"), user.Lookup, func(u *user.User) string { return u.Uid })
		if err != nil {
			return spec{}, err
		}
		s.gid, err = lookupID(textParam(req.Params, "group"), user.LookupGroup, func(g *user.Group) string { return g.Gid })
		if err != nil {
			return spec{}, err
		}
		if len(s.sources) == 0 || (s.uid < 0 && s.gid < 0) {
			return spec{}, wire.Invalid("files.op.setOwner requires paths and an owner or a group")
		}
	case "files.op.setPermissions":
		s.kind = KindSetPermissions
		sources, err := absPaths(req, "paths")
		if err != nil {
			return spec{}, err
		}
		s.sources = sources
		s.recursive = params.BoolLoose(req.Params, "recursive")
		mode, parseErr := strconv.ParseUint(params.StringOpt(req.Params, "mode", ""), 8, 32)
		if parseErr != nil || len(s.sources) == 0 {
			return spec{}, wire.Invalid("files.op.setPermissions requires paths and an octal mode")
		}
		s.mode = uint32(mode)
	default:
		return spec{}, &files.Error{Code: files.CodeNotSupported, Err: errors.New("unknown files.op method: " + req.Method)}
	}
	return s, nil
}

func (m *Manager) handleCancel(w *ipc.ConnWriter, req ipc.Request) {
	id := params.StringOpt(req.Params, "jobId", "")
	if err := m.Cancel(id); err != nil {
		wire.RespondError(w, req.ID, wire.Invalid(err.Error()))
		return
	}
	w.Respond(req.ID, map[string]any{"jobId": id, "cancelling": true})
}

func (m *Manager) handleRespond(w *ipc.ConnWriter, req ipc.Request) {
	jobID := params.StringOpt(req.Params, "jobId", "")
	answer := Answer{
		ID:         params.StringOpt(req.Params, "id", ""),
		Action:     Action(params.StringOpt(req.Params, "action", "")),
		ApplyToAll: params.BoolLoose(req.Params, "applyToAll"),
		Password:   params.StringOpt(req.Params, "password", ""),
	}
	if !knownAction(answer.Action) {
		wire.RespondError(w, req.ID, wire.Invalid("unknown action: "+string(answer.Action)))
		return
	}
	if err := m.Respond(jobID, answer); err != nil {
		wire.RespondError(w, req.ID, wire.Invalid(err.Error()))
		return
	}
	w.Respond(req.ID, map[string]any{"jobId": jobID, "accepted": true})
}

func (m *Manager) handleUndo(w *ipc.ConnWriter, req ipc.Request) {
	view, err := m.Undo(params.StringOpt(req.Params, "scope", ""))
	if err != nil {
		wire.RespondError(w, req.ID, wire.Invalid(err.Error()))
		return
	}
	w.Respond(req.ID, map[string]any{"job": view})
}

func (m *Manager) handleRedo(w *ipc.ConnWriter, req ipc.Request) {
	view, err := m.Redo(params.StringOpt(req.Params, "scope", ""))
	if err != nil {
		wire.RespondError(w, req.ID, wire.Invalid(err.Error()))
		return
	}
	w.Respond(req.ID, map[string]any{"job": view})
}

func (m *Manager) handlePasteImage(w *ipc.ConnWriter, req ipc.Request) {
	dest := params.StringOpt(req.Params, "path", "")
	stem := params.StringOpt(req.Params, "name", "Pasted image")
	if !filepath.IsAbs(dest) || !pathutil.ValidName(stem) {
		wire.RespondError(w, req.ID, wire.Invalid("files.op.pasteImage requires an absolute path and a name"))
		return
	}
	images := m.imageSource()
	if images == nil {
		wire.RespondError(w, req.ID, &files.Error{Code: files.CodeNotSupported, Err: errors.New("no clipboard image source")})
		return
	}
	data, mime, ok := images.Image()
	if !ok {
		w.Respond(req.ID, map[string]any{"image": false})
		return
	}
	s := spec{
		kind:   KindPasteImage,
		scope:  params.StringOpt(req.Params, "scope", ""),
		dest:   dest,
		name:   stem + imageExt(mime),
		data:   data,
		unique: true,
	}
	w.Respond(req.ID, map[string]any{"image": true, "job": m.enqueue(s).view()})
}

func lookupID[T any](raw string, lookup func(string) (*T, error), id func(*T) string) (int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return -1, nil
	}
	if number, err := strconv.Atoi(raw); err == nil && number >= 0 {
		if number >= helper.MaxID {
			return -1, wire.Invalid("owner or group id out of range: " + raw)
		}
		return number, nil
	}
	found, err := lookup(raw)
	if err != nil {
		return -1, wire.Invalid("unknown owner or group: " + raw)
	}
	number, err := strconv.Atoi(id(found))
	if err != nil || number < 0 || number >= helper.MaxID {
		return -1, wire.Invalid("unknown owner or group: " + raw)
	}
	return number, nil
}

func textParam(p map[string]any, key string) string {
	switch value := p[key].(type) {
	case string:
		return value
	case float64:
		return strconv.FormatInt(int64(value), 10)
	case int:
		return strconv.Itoa(value)
	}
	return ""
}

func knownAction(action Action) bool {
	switch action {
	case ActionSkip, ActionReplace, ActionKeepBoth, ActionMerge, ActionRetry, ActionRetryElevated, ActionCancel, ActionUnlock:
		return true
	}
	return false
}

func absPaths(req ipc.Request, key string) ([]string, error) {
	values := wire.StringList(req.Params, key)
	if len(values) == 0 {
		values = wire.StringList(req.Params, "path")
	}

	out := make([]string, 0, len(values))
	for _, value := range values {
		if !filepath.IsAbs(value) {
			return nil, wire.Invalid("not an absolute path: " + value)
		}
		out = append(out, filepath.Clean(value))
	}
	return out, nil
}

func nestedSource(sources []string, dest string) string {
	for _, src := range sources {
		if dest == src || pathutil.Under(dest, src) {
			return src
		}
	}
	return ""
}

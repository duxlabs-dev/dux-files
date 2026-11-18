package devices

import (
	"github.com/duxlabs-dev/dux-files/core/internal/wire"
	"github.com/duxlabs-dev/dux-go/files"
	"github.com/duxlabs-dev/dux-go/ipc"
	"github.com/duxlabs-dev/dux-go/ipc/params"
	"github.com/godbus/dbus/v5"
)

func Handles(method string) bool {
	switch method {
	case "devices.list", "devices.usage", "devices.mount", "devices.unmount",
		"devices.eject", "devices.powerOff", "devices.unlock", "devices.lock":
		return true
	}
	return false
}

func (s *Service) Handle(w *ipc.ConnWriter, req ipc.Request) {
	switch req.Method {
	case "devices.list":
		w.Respond(req.ID, s.Snapshot())
	case "devices.usage":
		w.Respond(req.ID, map[string]any{"usage": s.Usage(params.StringSlice(req.Params, "paths"), params.BoolLoose(req.Params, "force"))})
	case "devices.mount":
		s.handleMount(w, req)
	case "devices.unmount":
		s.handleUnmount(w, req)
	case "devices.eject":
		s.handleEject(w, req)
	case "devices.powerOff":
		s.handlePowerOff(w, req)
	case "devices.unlock":
		s.handleUnlock(w, req)
	case "devices.lock":
		s.handleLock(w, req)
	default:
		wire.RespondError(w, req.ID, wire.Fail(files.CodeNotSupported, "unknown devices method: %s", req.Method))
	}
}

func (s *Service) handleMount(w *ipc.ConnWriter, req ipc.Request) {
	id, ok := deviceID(w, req)
	if !ok {
		return
	}
	mountPoint, err := s.Mount(id)
	if err != nil {
		wire.RespondError(w, req.ID, err)
		return
	}
	w.Respond(req.ID, map[string]any{"id": id, "mountPoint": mountPoint})
}

func (s *Service) handleUnmount(w *ipc.ConnWriter, req ipc.Request) {
	id, ok := deviceID(w, req)
	if !ok {
		return
	}
	if err := s.Unmount(id, params.BoolLoose(req.Params, "force")); err != nil {
		wire.RespondError(w, req.ID, err)
		return
	}
	w.Respond(req.ID, map[string]any{"id": id, "mounted": false})
}

func (s *Service) handleEject(w *ipc.ConnWriter, req ipc.Request) {
	id, ok := deviceID(w, req)
	if !ok {
		return
	}
	if err := s.Eject(id); err != nil {
		wire.RespondError(w, req.ID, err)
		return
	}
	w.Respond(req.ID, map[string]any{"id": id, "ejected": true})
}

func (s *Service) handlePowerOff(w *ipc.ConnWriter, req ipc.Request) {
	id, ok := deviceID(w, req)
	if !ok {
		return
	}
	if err := s.PowerOff(id); err != nil {
		wire.RespondError(w, req.ID, err)
		return
	}
	w.Respond(req.ID, map[string]any{"id": id, "poweredOff": true})
}

func (s *Service) handleUnlock(w *ipc.ConnWriter, req ipc.Request) {
	id, ok := deviceID(w, req)
	if !ok {
		return
	}
	passphrase := params.StringOpt(req.Params, "passphrase", "")
	if passphrase == "" {
		wire.RespondError(w, req.ID, wire.Invalid("devices.unlock requires a passphrase"))
		return
	}

	cleartext, err := s.Unlock(id, passphrase)
	if err != nil {
		wire.RespondError(w, req.ID, err)
		return
	}

	result := map[string]any{"id": id, "cleartext": cleartext}
	if params.BoolLoose(req.Params, "mount") {
		mountPoint, err := s.mountPath(dbus.ObjectPath(cleartext))
		switch err {
		case nil:
			result["mountPoint"] = mountPoint
		default:
			result["mountError"] = err.Error()
			result["mountCode"] = files.CodeOf(err)
		}
	}
	w.Respond(req.ID, result)
}

func (s *Service) handleLock(w *ipc.ConnWriter, req ipc.Request) {
	id, ok := deviceID(w, req)
	if !ok {
		return
	}
	if err := s.Lock(id); err != nil {
		wire.RespondError(w, req.ID, err)
		return
	}
	w.Respond(req.ID, map[string]any{"id": id, "unlocked": false})
}

// The object table has not seen the cleartext device yet right after an unlock.
func (s *Service) mountPath(path dbus.ObjectPath) (string, error) {
	var mountPoint string
	if err := s.call(path, FsIface+".Mount", noOptions).Store(&mountPoint); err != nil {
		return "", wrapDBus(err)
	}
	s.usage.forget(mountPoint)
	return mountPoint, nil
}

func deviceID(w *ipc.ConnWriter, req ipc.Request) (string, bool) {
	id := params.StringOpt(req.Params, "id", "")
	if id == "" {
		wire.RespondError(w, req.ID, wire.Invalid(req.Method+" requires a device id"))
		return "", false
	}
	return id, true
}

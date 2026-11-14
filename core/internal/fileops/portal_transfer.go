package fileops

import (
	"context"
	"syscall"
	"time"

	"github.com/duxlabs-dev/dux-files/core/internal/wire"
	"github.com/duxlabs-dev/dux-go/ipc"
	"github.com/duxlabs-dev/dux-go/log"
	"github.com/godbus/dbus/v5"
)

const (
	portalBusName       = "org.freedesktop.portal.Documents"
	portalObjectPath    = "/org/freedesktop/portal/documents"
	portalTransferIface = "org.freedesktop.portal.FileTransfer"
	portalBatchSize     = 16
	portalTimeout       = 2 * time.Second
)

type portalTransferClient interface {
	StartTransfer(ctx context.Context) (string, error)
	AddFiles(ctx context.Context, key string, fds []dbus.UnixFD) error
	StopTransfer(ctx context.Context, key string)
}

type dbusPortalTransfer struct {
	obj dbus.BusObject
}

func (c dbusPortalTransfer) StartTransfer(ctx context.Context) (string, error) {
	opts := map[string]dbus.Variant{
		"writable": dbus.MakeVariant(false),
		"autostop": dbus.MakeVariant(true),
	}
	var key string
	err := c.obj.CallWithContext(ctx, portalTransferIface+".StartTransfer", 0, opts).Store(&key)
	return key, err
}

func (c dbusPortalTransfer) AddFiles(ctx context.Context, key string, fds []dbus.UnixFD) error {
	return c.obj.CallWithContext(ctx, portalTransferIface+".AddFiles", 0, key, fds, map[string]dbus.Variant{}).Err
}

func (c dbusPortalTransfer) StopTransfer(ctx context.Context, key string) {
	_ = c.obj.CallWithContext(ctx, portalTransferIface+".StopTransfer", 0, key).Err
}

func (m *Manager) handlePortalTransfer(ctx context.Context, w *ipc.ConnWriter, req ipc.Request) {
	sources, err := absPaths(req, "paths")
	if err != nil {
		wire.RespondError(w, req.ID, err)
		return
	}
	if len(sources) == 0 {
		wire.RespondError(w, req.ID, wire.Invalid("files.op.portalTransfer requires paths"))
		return
	}

	bus, err := dbus.SessionBus()
	if err != nil {
		log.Debugf("portalTransfer: session bus: %v", err)
		w.Respond(req.ID, map[string]any{"key": ""})
		return
	}

	callCtx, cancel := context.WithTimeout(ctx, portalTimeout)
	defer cancel()

	client := dbusPortalTransfer{obj: bus.Object(portalBusName, portalObjectPath)}
	key, err := m.exportPortalTransfer(callCtx, client, sources)
	if err != nil {
		log.Debugf("portalTransfer: %v", err)
		w.Respond(req.ID, map[string]any{"key": ""})
		return
	}
	w.Respond(req.ID, map[string]any{"key": key})
}

func (m *Manager) exportPortalTransfer(ctx context.Context, client portalTransferClient, paths []string) (string, error) {
	m.mu.Lock()
	prev := m.portalKey
	m.portalKey = ""
	m.mu.Unlock()

	if prev != "" {
		client.StopTransfer(ctx, prev)
	}

	key, err := exportPortalTransfer(ctx, client, paths)
	if err != nil || key == "" {
		return "", err
	}

	m.mu.Lock()
	if m.portalKey == "" {
		m.portalKey = key
		m.mu.Unlock()
		return key, nil
	}
	m.mu.Unlock()

	client.StopTransfer(ctx, key)
	return "", nil
}

func exportPortalTransfer(ctx context.Context, client portalTransferClient, paths []string) (string, error) {
	key, err := client.StartTransfer(ctx)
	if err != nil || key == "" {
		return "", err
	}

	for i := 0; i < len(paths); i += portalBatchSize {
		end := min(i+portalBatchSize, len(paths))
		if err := sendPortalBatch(ctx, client, key, paths[i:end]); err != nil {
			client.StopTransfer(ctx, key)
			return "", err
		}
	}
	return key, nil
}

func sendPortalBatch(ctx context.Context, client portalTransferClient, key string, paths []string) error {
	fds := make([]dbus.UnixFD, 0, len(paths))
	defer func() {
		for _, fd := range fds {
			_ = syscall.Close(int(fd))
		}
	}()

	for _, p := range paths {
		fd, err := syscall.Open(p, syscall.O_RDONLY|syscall.O_NONBLOCK|syscall.O_CLOEXEC, 0)
		if err != nil {
			return err
		}
		fds = append(fds, dbus.UnixFD(fd))
	}
	return client.AddFiles(ctx, key, fds)
}

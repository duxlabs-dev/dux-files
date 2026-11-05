package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/duxlabs-dev/dux-files/core/internal/bookmarks"
	"github.com/duxlabs-dev/dux-files/core/internal/clipboard"
	"github.com/duxlabs-dev/dux-files/core/internal/colorscheme"
	"github.com/duxlabs-dev/dux-files/core/internal/devices"
	"github.com/duxlabs-dev/dux-files/core/internal/elevate"
	"github.com/duxlabs-dev/dux-files/core/internal/filechooser"
	"github.com/duxlabs-dev/dux-files/core/internal/filemanager1"
	"github.com/duxlabs-dev/dux-files/core/internal/fileops"
	"github.com/duxlabs-dev/dux-files/core/internal/ipc"
	"github.com/duxlabs-dev/dux-files/core/internal/mimeapps"
	"github.com/duxlabs-dev/dux-files/core/internal/netmount"
	"github.com/duxlabs-dev/dux-files/core/internal/portal"
	"github.com/duxlabs-dev/dux-files/core/internal/sandbox"
	"github.com/duxlabs-dev/dux-files/core/internal/search"
	"github.com/duxlabs-dev/dux-files/core/internal/settings"
	"github.com/duxlabs-dev/dux-go/files"
	"github.com/duxlabs-dev/dux-go/log"
	dankpaths "github.com/duxlabs-dev/dux-go/paths"
	"github.com/godbus/dbus/v5"
)

var daemonCmd = &cobra.Command{
	Use:   "daemon",
	Short: "Run the dfiles daemon (IPC only, no UI)",
	RunE:  runDaemon,
}

type daemonServices struct {
	ipc       *ipc.Server
	ipcErrCh  <-chan error
	files     *files.Service
	devices   *devices.Service
	network   *netmount.Service
	clipboard *clipboard.Service
	search    *search.Service
	bookmarks *bookmarks.Store
	fm1       *filemanager1.Service
	chooser   *filechooser.Service
	colors    *colorscheme.Watcher
}

func (s *daemonServices) SocketPath() string {
	if s.ipc == nil {
		return ""
	}
	return s.ipc.SocketPath()
}

func (s *daemonServices) Done() <-chan error { return s.ipcErrCh }

func (s *daemonServices) Close() {
	if s.ipc != nil {
		s.ipc.Close()
	}
	if s.fm1 != nil {
		s.fm1.Close()
	}
	if s.chooser != nil {
		s.chooser.Close()
	}
	if s.files != nil {
		s.files.Close()
	}
	if s.devices != nil {
		s.devices.Close()
	}
	if s.network != nil {
		s.network.Close()
	}
	if s.clipboard != nil {
		s.clipboard.Close()
	}
	if s.search != nil {
		s.search.Close()
	}
	if s.bookmarks != nil {
		s.bookmarks.Close()
	}
	if s.colors != nil {
		s.colors.Close()
	}
}

func bootDaemonServices(ctx context.Context, ownsWindow bool) (*daemonServices, error) {
	caps := sandbox.Current()
	if caps.Sandboxed {
		log.Infof("sandboxed as %s (host filesystem: %v)", caps.AppID, caps.HostFilesystem)
	}
	sandbox.ExportHostDirs(caps)

	bus := ipc.NewEventBus()
	pending := &ipc.PendingOpen{}
	var chooser *filechooser.Service
	var portalController *portal.Controller
	if ownsWindow {
		chooser, portalController = startPortalChooser(ipc.Deps{Bus: bus, Pending: pending}, caps)
	}

	appService := mimeapps.NewService()
	fileService := files.NewService(bus, cacheHome(), appService.Detector())
	opsManager := fileops.NewManager(bus, fileService)
	opsManager.SetElevator(elevate.NewClient())

	deviceService := startDevices(bus, log.Warnf)

	config := configHome()
	recentStore := wireRecent(appService, chooser, config)
	networkService := netmount.NewService(bus, config)
	if err := networkService.Start(); err != nil {
		log.Warnf("gvfs unavailable: %v", err)
	}

	clipboardService := clipboard.NewService()
	opsManager.SetImageSource(clipboardService)
	searchService := newSearch(bus, fileService, deviceService, config, caps)
	bookmarkStore := bookmarks.New(bus, config, filepath.Join(filepath.Dir(config), "gtk-3.0", "bookmarks"))
	if err := bookmarkStore.Start(); err != nil {
		log.Warnf("bookmarks unavailable: %v", err)
		bookmarkStore = nil
	}

	colorSchemeWatcher := colorscheme.New(bus.Publish)
	if err := colorSchemeWatcher.Start(); err != nil {
		log.Warnf("color-scheme portal watcher unavailable: %v", err)
	}

	deps := ipc.Deps{
		Bus:         bus,
		Pending:     pending,
		Portal:      portalController,
		Version:     Version,
		ColorScheme: colorSchemeWatcher,
		Files:       fileService,
		FileOps:     opsManager,
		Apps:        appService,
		Devices:     deviceService,
		Network:     networkService,
		Clipboard:   clipboardService,
		Search:      searchService,
		Bookmarks:   bookmarkStore,
		Recent:      recentStore,
		Settings:    func() settings.UISettings { return settings.Load(config) },
		Sandbox:     caps,
		Quit:        requestQuit,
	}

	var fm1 *filemanager1.Service
	if ownsWindow {
		fm1 = startFileManager1(deps, dbusActivated || settings.Load(config).RegisterFileManager)
	}
	if fm1 != nil {
		deps.FileManager = fm1
	}

	svc := &daemonServices{files: fileService, devices: deviceService, network: networkService, clipboard: clipboardService, search: searchService, bookmarks: bookmarkStore, fm1: fm1, chooser: chooser, colors: colorSchemeWatcher}
	ipcSrv, ipcErrCh, err := startIPC(ctx, deps)
	if err != nil {
		svc.Close()
		return nil, err
	}
	svc.ipc = ipcSrv
	svc.ipcErrCh = ipcErrCh
	return svc, nil
}

// The system device is left out so scope Everywhere never walks the whole root filesystem.
func localMounts(snapshot devices.Snapshot) []string {
	var out []string
	for _, device := range snapshot.Devices {
		if !device.Mounted || device.System || device.Hidden {
			continue
		}
		if len(device.MountPoints) == 0 {
			out = append(out, device.MountPoint)
			continue
		}
		out = append(out, device.MountPoints...)
	}
	return out
}

type fileManagerRequests struct{ deps ipc.Deps }

func (r fileManagerRequests) ShowFolders(uris []string) error {
	return ipc.OpenLocations(r.deps, uris, nil, false)
}

func (r fileManagerRequests) ShowItems(uris []string) error {
	return ipc.OpenLocations(r.deps, nil, uris, false)
}

func (r fileManagerRequests) ShowItemProperties(uris []string) error {
	return ipc.ShowProperties(r.deps, uris)
}

func startFileManager1(deps ipc.Deps, enabled bool) *filemanager1.Service {
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		log.Warnf("filemanager1 unavailable: %v", err)
		return nil
	}
	svc, err := filemanager1.New(conn, fileManagerRequests{deps: deps})
	if err != nil {
		conn.Close()
		log.Warnf("filemanager1 unavailable: %v", err)
		return nil
	}
	svc.SetEnabled(enabled)
	return svc
}

func startPortalChooser(deps ipc.Deps, caps sandbox.Capabilities) (*filechooser.Service, *portal.Controller) {
	if caps.Sandboxed {
		return nil, portal.Unavailable(sandbox.ReasonSandboxed)
	}
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		log.Warnf("portal file chooser unavailable: %v", err)
		return nil, portal.NewController(nil, keepRunning)
	}
	svc, err := filechooser.New(conn, ipc.ChooserUI{Deps: deps}, func(mime string) []string { return mimeDB().Globs(mime) })
	if err != nil {
		conn.Close()
		log.Warnf("portal file chooser unavailable: %v", err)
		return nil, portal.NewController(nil, keepRunning)
	}
	svc.SetEnabled(true)
	return svc, portal.NewController(svc, keepRunning)
}

var mimeDB = sync.OnceValue(func() *mimeapps.DB { return mimeapps.LoadDB(mimeapps.MimeDirs()) })

var appPaths = dankpaths.New("dfiles")

func configHome() string {
	dir, err := appPaths.ConfigDir()
	if err != nil {
		log.Warnf("config dir unavailable: %v", err)
		return os.TempDir()
	}
	return dir
}

func cacheHome() string {
	dir, err := appPaths.CacheDir()
	if err != nil {
		log.Warnf("cache dir unavailable: %v", err)
		return os.TempDir()
	}
	return filepath.Dir(dir)
}

func runDaemon(_ *cobra.Command, _ []string) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	svc, err := bootDaemonServices(ctx, false)
	if err != nil {
		return err
	}
	defer svc.Close()

	log.Infof("dfiles daemon ready (ipc=%s)", svc.SocketPath())

	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-signalCh:
		log.Infof("received %s, shutting down", sig)
	case err := <-svc.Done():
		if err != nil {
			return fmt.Errorf("ipc server: %w", err)
		}
	}
	return nil
}

func requestQuit() {
	_ = syscall.Kill(os.Getpid(), syscall.SIGTERM)
}

func startIPC(ctx context.Context, deps ipc.Deps) (*ipc.Server, <-chan error, error) {
	srv := ipc.NewServer(deps)
	if err := srv.Listen(); err != nil {
		return nil, nil, err
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.Serve(ctx)
	}()
	return srv, errCh, nil
}

func startDevices(bus *ipc.EventBus, logf func(string, ...any)) *devices.Service {
	deviceService := devices.NewService(bus, uint32(os.Getuid()))
	if err := deviceService.Start(); err != nil {
		logf("udisks2 unavailable: %v", err)
	}
	return deviceService
}

func newSearch(bus *ipc.EventBus, fileService *files.Service, deviceService *devices.Service, config string, caps sandbox.Capabilities) *search.Service {
	searchService := search.NewService(bus, fileService)
	searchService.SetIndexEnabled(!caps.Sandboxed)
	searchService.SetMounts(func() []string { return localMounts(deviceService.Snapshot()) })
	searchService.SetExcludes(func() []string { return settings.Load(config).SearchExcludes })
	return searchService
}

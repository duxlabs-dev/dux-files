package ipc

import (
	"github.com/duxlabs-dev/dux-files/core/internal/bookmarks"
	"github.com/duxlabs-dev/dux-files/core/internal/clipboard"
	"github.com/duxlabs-dev/dux-files/core/internal/colorscheme"
	"github.com/duxlabs-dev/dux-files/core/internal/devices"
	"github.com/duxlabs-dev/dux-files/core/internal/filemanager1"
	"github.com/duxlabs-dev/dux-files/core/internal/fileops"
	"github.com/duxlabs-dev/dux-files/core/internal/mimeapps"
	"github.com/duxlabs-dev/dux-files/core/internal/netmount"
	"github.com/duxlabs-dev/dux-files/core/internal/portal"
	"github.com/duxlabs-dev/dux-files/core/internal/recent"
	"github.com/duxlabs-dev/dux-files/core/internal/sandbox"
	"github.com/duxlabs-dev/dux-files/core/internal/search"
	"github.com/duxlabs-dev/dux-files/core/internal/settings"
	"github.com/duxlabs-dev/dux-files/core/internal/wallpaper"
	"github.com/duxlabs-dev/dux-go/files"
)

type Deps struct {
	Bus         *EventBus
	Pending     *PendingOpen
	Version     string
	ColorScheme *colorscheme.Watcher
	Files       *files.Service
	FileOps     *fileops.Manager
	Apps        *mimeapps.Service
	Devices     *devices.Service
	Network     *netmount.Service
	Clipboard   *clipboard.Service
	Search      *search.Service
	Bookmarks   *bookmarks.Store
	Recent      *recent.Store
	Settings    func() settings.UISettings
	Sandbox     sandbox.Capabilities
	FileManager *filemanager1.Service
	Portal      *portal.Controller
	Wallpaper   *wallpaper.Service
	Quit        func()
}

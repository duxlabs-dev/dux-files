package portal

import (
	"errors"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/duxlabs-dev/dux-files/core/internal/filechooser"
	"github.com/duxlabs-dev/dux-files/core/internal/portalsetup"
	"github.com/godbus/dbus/v5"
)

const (
	ReasonNoBus    = "NOBUS"
	ReasonNoWindow = "NOWINDOW"
)

type Frontend struct {
	Running       bool `json:"running"`
	PID           int  `json:"pid"`
	RestartNeeded bool `json:"restartNeeded"`
	UnitKnown     bool `json:"unitKnown"`
}

type Report struct {
	portalsetup.Status
	Available bool               `json:"available"`
	Reason    string             `json:"reason,omitempty"`
	Backend   filechooser.Status `json:"backend"`
	Frontend  Frontend           `json:"frontend"`
	Resident  bool               `json:"resident"`
}

type Controller struct {
	mu          sync.Mutex
	service     *filechooser.Service
	unavailable string
	resident    bool
	changed     atomic.Pointer[time.Time]
}

func NewController(service *filechooser.Service, resident bool) *Controller {
	return &Controller{service: service, resident: resident}
}

func Unavailable(reason string) *Controller {
	return &Controller{unavailable: reason}
}

func (c *Controller) Env() portalsetup.Env {
	env := portalsetup.FromProcess()
	pid, ok := c.frontendPID()
	if !ok {
		return env
	}
	data, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/environ")
	if err != nil {
		return env
	}
	return env.WithEnviron(portalsetup.ParseEnviron(data))
}

func (c *Controller) frontendPID() (int, bool) {
	if c.service != nil {
		return c.service.FrontendPID()
	}
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return 0, false
	}
	defer conn.Close()
	var pid uint32
	if err := conn.BusObject().Call("org.freedesktop.DBus.GetConnectionUnixProcessID", 0, filechooser.FrontendName).Store(&pid); err != nil {
		return 0, false
	}
	return int(pid), true
}

func (c *Controller) Pending() []filechooser.Request {
	if c.service == nil {
		return nil
	}
	return c.service.Pending()
}

func (c *Controller) Status() Report {
	return c.report(portalsetup.Inspect(c.Env()))
}

func (c *Controller) Install() (Report, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.unavailable != "" {
		return c.report(portalsetup.Inspect(c.Env())), errors.New("the portal backend is unavailable: " + c.unavailable)
	}
	status, err := portalsetup.Install(c.Env())
	if err != nil {
		return c.report(status), err
	}
	c.markChanged()
	c.reloadBus()
	return c.report(status), nil
}

func (c *Controller) markChanged() {
	now := time.Now()
	c.changed.Store(&now)
}

func (c *Controller) reloadBus() {
	if c.service != nil {
		_ = c.service.ReloadBus()
		return
	}
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return
	}
	defer conn.Close()
	_ = conn.BusObject().Call("org.freedesktop.DBus.ReloadConfig", 0).Err
}

func (c *Controller) Uninstall() (Report, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	status, err := portalsetup.Uninstall(c.Env())
	if err != nil {
		return c.report(status), err
	}
	c.markChanged()
	c.reloadBus()
	return c.report(status), nil
}

func (c *Controller) Restart() error {
	pid, _ := c.frontendPID()
	return portalsetup.RestartFrontend(pid)
}

func (c *Controller) Answer(id string, answer filechooser.Answer) bool {
	if c.service == nil {
		return false
	}
	return c.service.Answer(id, answer)
}

func (c *Controller) report(status portalsetup.Status) Report {
	out := Report{Status: status, Available: c.unavailable == "", Reason: c.unavailable, Resident: c.resident, Backend: filechooser.Status{Reason: ReasonNoBus}}
	if c.service != nil {
		out.Backend = c.service.Status()
	}
	out.Frontend = c.frontend(status)
	return out
}

func (c *Controller) frontend(status portalsetup.Status) Frontend {
	out := Frontend{UnitKnown: portalsetup.FrontendUnitKnown()}
	pid, ok := c.frontendPID()
	if !ok {
		return out
	}
	out.Running = true
	out.PID = pid
	started, ok := portalsetup.ProcessStart(pid)
	if !ok {
		return out
	}
	out.RestartNeeded = c.newestChange(status).After(started)
	return out
}

func (c *Controller) newestChange(status portalsetup.Status) time.Time {
	var newest time.Time
	if changed := c.changed.Load(); changed != nil {
		newest = *changed
	}
	if !status.Managed && !status.Active {
		return newest
	}
	for _, path := range []string{status.PortalFile.Path, status.Service.Path, status.Config.Path} {
		if path == "" {
			continue
		}
		info, err := os.Stat(path)
		if err != nil {
			continue
		}
		if info.ModTime().After(newest) {
			newest = info.ModTime()
		}
	}
	return newest
}

package filemanager1

import (
	"errors"
	"sync"

	"github.com/duxlabs-dev/dux-files/core/internal/dbusutil"

	"github.com/godbus/dbus/v5"
	"github.com/godbus/dbus/v5/introspect"
)

const (
	Name      = "org.freedesktop.FileManager1"
	Path      = dbus.ObjectPath("/org/freedesktop/FileManager1")
	Interface = "org.freedesktop.FileManager1"
)

// nautilus data/freedesktop-dbus-interfaces.xml minus its two property extensions.
const introspection = `<node>
  <interface name="org.freedesktop.FileManager1">
    <method name="ShowFolders">
      <arg type="as" name="URIs" direction="in"/>
      <arg type="s" name="StartupId" direction="in"/>
    </method>
    <method name="ShowItems">
      <arg type="as" name="URIs" direction="in"/>
      <arg type="s" name="StartupId" direction="in"/>
    </method>
    <method name="ShowItemProperties">
      <arg type="as" name="URIs" direction="in"/>
      <arg type="s" name="StartupId" direction="in"/>
    </method>
  </interface>` + introspect.IntrospectDataString + `</node>`

type Handler interface {
	ShowFolders(uris []string) error
	ShowItems(uris []string) error
	ShowItemProperties(uris []string) error
}

type Status struct {
	Available bool   `json:"available"`
	Enabled   bool   `json:"enabled"`
	Owned     bool   `json:"owned"`
	Queued    bool   `json:"queued"`
	Owner     string `json:"owner"`
	Reason    string `json:"reason,omitempty"`
}

type Service struct {
	mu    sync.Mutex
	conn  *dbus.Conn
	owner *dbusutil.NameOwner
}

type object struct{ handler Handler }

func (o object) ShowFolders(uris []string, _ string) *dbus.Error {
	return failed(o.handler.ShowFolders(uris))
}

func (o object) ShowItems(uris []string, _ string) *dbus.Error {
	return failed(o.handler.ShowItems(uris))
}

func (o object) ShowItemProperties(uris []string, _ string) *dbus.Error {
	return failed(o.handler.ShowItemProperties(uris))
}

func failed(err error) *dbus.Error {
	if err == nil {
		return nil
	}
	return dbus.MakeFailedError(err)
}

func New(conn *dbus.Conn, handler Handler) (*Service, error) {
	if conn == nil {
		return nil, errors.New("no session bus")
	}
	if err := conn.Export(object{handler: handler}, Path, Interface); err != nil {
		return nil, err
	}
	if err := conn.Export(introspect.Introspectable(introspection), Path, "org.freedesktop.DBus.Introspectable"); err != nil {
		return nil, err
	}
	return &Service{conn: conn, owner: dbusutil.NewNameOwner(conn, Name, "filemanager1")}, nil
}

func (s *Service) SetEnabled(enabled bool) Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.owner.SetEnabled(enabled)
	return s.statusLocked()
}

func (s *Service) Status() Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.statusLocked()
}

func (s *Service) statusLocked() Status {
	owned, holder := s.owner.Owner()
	return Status{
		Available: true,
		Enabled:   s.owner.Enabled(),
		Owned:     owned,
		Queued:    s.owner.Enabled() && !owned,
		Owner:     holder,
		Reason:    s.owner.Reason(),
	}
}

func (s *Service) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.owner.Release()
	s.conn.Close()
}

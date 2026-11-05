package dbusutil

import (
	"os"
	"strconv"
	"strings"

	"github.com/duxlabs-dev/dux-go/log"
	"github.com/godbus/dbus/v5"
)

// Callers serialize access; the owner keeps no lock of its own.
type NameOwner struct {
	conn    *dbus.Conn
	name    string
	prefix  string
	enabled bool
	reason  string
}

func NewNameOwner(conn *dbus.Conn, name, logPrefix string) *NameOwner {
	return &NameOwner{conn: conn, name: name, prefix: logPrefix}
}

func (o *NameOwner) Enabled() bool { return o.enabled }

func (o *NameOwner) Reason() string { return o.reason }

func (o *NameOwner) SetEnabled(enabled bool) {
	switch {
	case enabled && !o.enabled:
		o.request()
	case !enabled && o.enabled:
		o.Release()
	}
}

func (o *NameOwner) request() {
	reply, err := o.conn.RequestName(o.name, 0)
	if err != nil {
		o.reason = err.Error()
		log.Warnf("%s: request %s: %v", o.prefix, o.name, err)
		return
	}
	o.enabled = true
	o.reason = ""
	if reply != dbus.RequestNameReplyInQueue {
		return
	}
	_, holder := o.Owner()
	log.Infof("%s: %s is held by %s; queued until it lets go", o.prefix, o.name, holder)
}

func (o *NameOwner) Release() {
	if !o.enabled {
		return
	}
	if _, err := o.conn.ReleaseName(o.name); err != nil {
		log.Warnf("%s: release %s: %v", o.prefix, o.name, err)
	}
	o.enabled = false
}

func (o *NameOwner) Owned() bool {
	owned, _ := o.Owner()
	return owned
}

func (o *NameOwner) Owner() (owned bool, holder string) {
	var unique string
	if err := o.conn.BusObject().Call("org.freedesktop.DBus.GetNameOwner", 0, o.name).Store(&unique); err != nil {
		return false, ""
	}
	if names := o.conn.Names(); len(names) > 0 && names[0] == unique {
		return true, ""
	}
	pid, ok := ProcessID(o.conn, unique)
	if !ok {
		return false, unique
	}
	comm, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/comm")
	if err != nil {
		return false, unique
	}
	return false, strings.TrimSpace(string(comm))
}

func ProcessID(conn *dbus.Conn, name string) (int, bool) {
	var pid uint32
	if err := conn.BusObject().Call("org.freedesktop.DBus.GetConnectionUnixProcessID", 0, name).Store(&pid); err != nil {
		return 0, false
	}
	return int(pid), true
}

func ReloadConfig(conn *dbus.Conn) error {
	return conn.BusObject().Call("org.freedesktop.DBus.ReloadConfig", 0).Err
}

func WithSessionBus(fn func(conn *dbus.Conn)) bool {
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return false
	}
	defer conn.Close()
	fn(conn)
	return true
}

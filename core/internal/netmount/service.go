package netmount

import (
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/duxlabs-dev/dux-files/core/internal/location"
	"github.com/duxlabs-dev/dux-go/files"
	"github.com/duxlabs-dev/dux-go/log"
	"github.com/godbus/dbus/v5"
)

const (
	Topic         = "network"
	coalesceDelay = 150 * time.Millisecond
)

type Publisher interface {
	Publish(topic string, data any)
}

type Service struct {
	bus       Publisher
	servers   *serverStore
	opCounter atomic.Uint32
	stop      chan struct{}
	stopped   sync.Once

	mu         sync.RWMutex
	conn       *dbus.Conn
	mountables []Mountable
	mounts     []Mount
	available  bool
	reason     string
	ops        map[dbus.ObjectPath]*mountOp
	listers    map[dbus.ObjectPath]*enumerator
}

type Snapshot struct {
	Kind      string   `json:"kind"`
	Available bool     `json:"available"`
	Reason    string   `json:"reason,omitempty"`
	Schemes   []string `json:"schemes"`
	Mounts    []Mount  `json:"mounts"`
}

func NewService(bus Publisher, configDir string) *Service {
	return &Service{
		bus:     bus,
		servers: newServerStore(configDir),
		stop:    make(chan struct{}),
		reason:  "gvfs is not running",
		ops:     map[dbus.ObjectPath]*mountOp{},
		listers: map[dbus.ObjectPath]*enumerator{},
	}
}

func (s *Service) Start() error {
	conn, err := dbus.ConnectSessionBus(dbus.WithIncomingInterceptor(s.intercept))
	if err != nil {
		s.setReason("the session bus is unavailable")
		return err
	}

	s.mu.Lock()
	s.conn = conn
	s.mu.Unlock()

	if err := conn.AddMatchSignal(dbus.WithMatchInterface(TrackerIface)); err != nil {
		log.Debugf("network: gvfs signals unavailable: %v", err)
	}
	signals := make(chan *dbus.Signal, 16)
	conn.Signal(signals)

	s.reload(conn)
	go s.loop(conn, signals)
	return nil
}

func (s *Service) Close() {
	s.stopped.Do(func() { close(s.stop) })
}

func (s *Service) loop(conn *dbus.Conn, signals chan *dbus.Signal) {
	defer conn.Close()
	var flush <-chan time.Time

	for {
		select {
		case <-s.stop:
			return
		case sig, ok := <-signals:
			if !ok {
				return
			}
			if sig.Name != TrackerIface+".Mounted" && sig.Name != TrackerIface+".Unmounted" {
				continue
			}
			if flush == nil {
				flush = time.After(coalesceDelay)
			}
		case <-flush:
			flush = nil
			s.reload(s.connection())
			s.publish(s.Snapshot())
		}
	}
}

func (s *Service) reload(conn *dbus.Conn) {
	if conn == nil {
		return
	}
	tracker := conn.Object(VfsBus, TrackerPath)

	var mountables []Mountable
	if err := tracker.Call(TrackerIface+".ListMountableInfo", 0).Store(&mountables); err != nil {
		s.setReason(reasonFor(err))
		log.Debugf("network: ListMountableInfo failed: %v", err)
		return
	}

	var rawMounts []wireMount
	if err := tracker.Call(TrackerIface+".ListMounts", 0).Store(&rawMounts); err != nil {
		s.setReason(reasonFor(err))
		log.Debugf("network: ListMounts failed: %v", err)
		return
	}

	mounts := make([]Mount, 0, len(rawMounts))
	for _, raw := range rawMounts {
		if !raw.UserVisible {
			continue
		}
		mounts = append(mounts, decodeMount(raw, mountables))
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.mountables = mountables
	s.mounts = mounts
	s.available = true
	s.reason = ""
}

func (s *Service) Snapshot() Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return Snapshot{Kind: "mounts", Available: s.available, Reason: s.reason, Schemes: availableSchemes(s.mountables), Mounts: s.mounts}
}

type Connected struct {
	Mount    *Mount  `json:"mount,omitempty"`
	Location string  `json:"location,omitempty"`
	Server   string  `json:"server,omitempty"`
	Shares   []Share `json:"shares"`
}

func (s *Service) Connect(uri, password string) (Connected, error) {
	conn := s.connection()
	if conn == nil {
		return Connected{}, fail(files.CodeNotSupported, "gvfs is not running")
	}

	remote := location.SplitPassword(strings.TrimSpace(uri))
	if password == "" {
		password = remote.Password
	}
	spec, err := ParseURI(remote.URI, s.mountableList())
	if err != nil {
		return Connected{}, err
	}
	if spec.Type() == "smb-server" {
		return s.browse(conn, remote.URI, password, spec)
	}
	mount, err := s.mount(conn, remote.URI, password, spec)
	if err != nil {
		return Connected{}, err
	}
	return Connected{Mount: &mount, Location: LocationIn(mount, URIOf(spec, s.mountableList()))}, nil
}

func (s *Service) mount(conn *dbus.Conn, uri, password string, spec Spec) (Mount, error) {
	if mount, found := s.mountedAt(spec); found {
		return mount, nil
	}
	if err := s.mountLocation(conn, uri, password, spec); err != nil {
		return Mount{}, err
	}

	s.reload(conn)
	mount, found := s.findBySpec(spec)
	if !found {
		return Mount{}, fail(files.CodeNotSupported, "%s is mounted but gvfs offers no folder for it", uri)
	}
	s.remember(Server{URI: mount.URI, Name: mount.Name, Scheme: mount.Scheme})
	s.publish(s.Snapshot())
	return mount, nil
}

// gvfsd-smb-browse keeps a server mount hidden and without a FUSE folder, so its shares are read over D-Bus.
func (s *Service) browse(conn *dbus.Conn, uri, password string, spec Spec) (Connected, error) {
	if err := s.mountLocation(conn, uri, password, spec); err != nil {
		return Connected{}, err
	}

	var raw wireMount
	if err := conn.Object(VfsBus, TrackerPath).Call(TrackerIface+".LookupMount", 0, encodeSpec(spec)).Store(&raw); err != nil {
		return Connected{}, wrapDBus(err)
	}
	server := URIOf(spec, s.mountableList())
	shares, err := s.listShares(conn, raw, server)
	if err != nil {
		return Connected{}, err
	}
	s.remember(Server{URI: server, Name: raw.DisplayName, Scheme: SchemeOf(spec, s.mountableList())})
	return Connected{Server: server, Shares: shares}, nil
}

func (s *Service) mountLocation(conn *dbus.Conn, uri, password string, spec Spec) error {
	op := s.newOp(uri)
	op.password = password
	if err := op.export(conn); err != nil {
		return &files.Error{Code: files.CodeIO, Err: err}
	}
	s.trackOp(op)
	defer s.releaseOp(op, conn)

	source := wireSource{DBusID: conn.Names()[0], Path: op.path}
	err := conn.Object(VfsBus, TrackerPath).Call(TrackerIface+".MountLocation", 0, encodeSpec(spec), source).Store()
	if err != nil && !alreadyMounted(err) {
		return wrapDBus(err)
	}
	return nil
}

func (s *Service) remember(server Server) {
	if err := s.servers.remember(server); err != nil {
		log.Warnf("network: saving recent servers: %v", err)
	}
}

func (s *Service) Disconnect(id string) error {
	conn := s.connection()
	if conn == nil {
		return fail(files.CodeNotSupported, "gvfs is not running")
	}

	mount, found := s.find(id)
	if !found {
		return fail(files.CodeNotFound, "unknown mount: %s", id)
	}

	op := s.newOp(mount.URI)
	if err := op.export(conn); err != nil {
		return &files.Error{Code: files.CodeIO, Err: err}
	}
	s.trackOp(op)
	defer s.releaseOp(op, conn)

	if err := conn.Object(mount.dbusID, mount.object).Call(MountIface+".Unmount", 0, conn.Names()[0], op.path, uint32(0)).Store(); err != nil {
		return wrapDBus(err)
	}

	s.reload(conn)
	s.publish(s.Snapshot())
	return nil
}

func (s *Service) Answer(id string, answer Answer) error {
	path, _, ok := cutLast(id, "/")
	if !ok {
		return fail(files.CodeInvalid, "unknown question: %s", id)
	}

	s.mu.RLock()
	op := s.ops[dbus.ObjectPath(path)]
	s.mu.RUnlock()

	if op == nil || !op.answer(id, answer) {
		return fail(files.CodeInvalid, "question %s is no longer waiting", id)
	}
	return nil
}

func (s *Service) Servers() []Server { return s.servers.list() }

func (s *Service) Forget(uri string) {
	if err := s.servers.forget(uri); err != nil {
		log.Warnf("network: saving recent servers: %v", err)
	}
}

func (s *Service) find(id string) (Mount, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, mount := range s.mounts {
		if mount.ID == id || mount.URI == id || mount.Path == id {
			return mount, true
		}
	}
	return Mount{}, false
}

func (s *Service) mountedAt(spec Spec) (Mount, bool) {
	uri := URIOf(spec, s.mountableList())

	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, mount := range s.mounts {
		rest, ok := strings.CutPrefix(uri, strings.TrimSuffix(mount.URI, "/"))
		if ok && (rest == "" || strings.HasPrefix(rest, "/")) {
			return mount, true
		}
	}
	return Mount{}, false
}

func (s *Service) findBySpec(spec Spec) (Mount, bool) {
	uri := URIOf(spec, s.mountableList())

	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, mount := range s.mounts {
		if mount.URI == uri {
			return mount, true
		}
	}
	for _, mount := range s.mounts {
		if strings.HasPrefix(uri, mount.URI) {
			return mount, true
		}
	}
	return Mount{}, false
}

func (s *Service) mountableList() []Mountable {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.mountables
}

func (s *Service) connection() *dbus.Conn {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.conn
}

func (s *Service) trackOp(op *mountOp) {
	s.mu.Lock()
	s.ops[op.path] = op
	s.mu.Unlock()
}

func (s *Service) releaseOp(op *mountOp, conn *dbus.Conn) {
	s.mu.Lock()
	delete(s.ops, op.path)
	s.mu.Unlock()
	op.release(conn)
}

func (s *Service) setReason(reason string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.available = false
	s.reason = reason
}

func (s *Service) publish(data any) {
	if s.bus == nil {
		return
	}
	s.bus.Publish(Topic, data)
}

func reasonFor(err error) string {
	if call, ok := err.(dbus.Error); ok && call.Name == "org.freedesktop.DBus.Error.ServiceUnknown" {
		return "gvfs is not installed"
	}
	return "gvfs is not running"
}

func cutLast(value, sep string) (string, string, bool) {
	index := strings.LastIndex(value, sep)
	if index < 0 {
		return value, "", false
	}
	return value[:index], value[index+len(sep):], true
}

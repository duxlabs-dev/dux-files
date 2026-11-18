package filechooser

import (
	"errors"
	"path/filepath"
	"strings"
	"sync"

	"github.com/duxlabs-dev/dux-files/core/internal/fileuri"

	"github.com/duxlabs-dev/dux-files/core/internal/dbusutil"

	"github.com/duxlabs-dev/dux-go/log"
	"github.com/godbus/dbus/v5"
	"github.com/godbus/dbus/v5/introspect"
)

const (
	BusName          = "org.freedesktop.impl.portal.desktop.dfiles"
	Path             = dbus.ObjectPath("/org/freedesktop/portal/desktop")
	Interface        = "org.freedesktop.impl.portal.FileChooser"
	RequestInterface = "org.freedesktop.impl.portal.Request"
	FrontendName     = "org.freedesktop.portal.Desktop"
	requestPrefix    = "/org/freedesktop/portal/desktop/request/"
)

const chooserIntrospection = `<node>
  <interface name="org.freedesktop.impl.portal.FileChooser">
    <method name="OpenFile">
      <arg type="o" name="handle" direction="in"/>
      <arg type="s" name="app_id" direction="in"/>
      <arg type="s" name="parent_window" direction="in"/>
      <arg type="s" name="title" direction="in"/>
      <arg type="a{sv}" name="options" direction="in"/>
      <arg type="u" name="response" direction="out"/>
      <arg type="a{sv}" name="results" direction="out"/>
    </method>
    <method name="SaveFile">
      <arg type="o" name="handle" direction="in"/>
      <arg type="s" name="app_id" direction="in"/>
      <arg type="s" name="parent_window" direction="in"/>
      <arg type="s" name="title" direction="in"/>
      <arg type="a{sv}" name="options" direction="in"/>
      <arg type="u" name="response" direction="out"/>
      <arg type="a{sv}" name="results" direction="out"/>
    </method>
    <method name="SaveFiles">
      <arg type="o" name="handle" direction="in"/>
      <arg type="s" name="app_id" direction="in"/>
      <arg type="s" name="parent_window" direction="in"/>
      <arg type="s" name="title" direction="in"/>
      <arg type="a{sv}" name="options" direction="in"/>
      <arg type="u" name="response" direction="out"/>
      <arg type="a{sv}" name="results" direction="out"/>
    </method>
  </interface>` + introspect.IntrospectDataString + `</node>`

const requestIntrospection = `<node>
  <interface name="org.freedesktop.impl.portal.Request">
    <method name="Close"/>
  </interface>` + introspect.IntrospectDataString + `</node>`

type UI interface {
	ShowChooser(req Request)
	CloseChooser(id string)
}

type Status struct {
	Enabled bool   `json:"enabled"`
	Owned   bool   `json:"owned"`
	Reason  string `json:"reason,omitempty"`
}

type Service struct {
	mu      sync.Mutex
	conn    *dbus.Conn
	ui      UI
	globs   func(mime string) []string
	owner   *dbusutil.NameOwner
	pending map[string]*pending
	watched map[string]int
	signals chan *dbus.Signal
	Chosen  func(req Request, paths []string)
}

type pending struct {
	sender  string
	request Request
	done    chan Answer
	once    sync.Once
}

func (p *pending) resolve(a Answer) {
	p.once.Do(func() { p.done <- a })
}

type object struct{ service *Service }

func (o object) OpenFile(sender dbus.Sender, handle dbus.ObjectPath, appID, parent, title string, options map[string]dbus.Variant) (uint32, map[string]dbus.Variant, *dbus.Error) {
	return o.service.serve(MethodOpen, sender, handle, appID, parent, title, options)
}

func (o object) SaveFile(sender dbus.Sender, handle dbus.ObjectPath, appID, parent, title string, options map[string]dbus.Variant) (uint32, map[string]dbus.Variant, *dbus.Error) {
	return o.service.serve(MethodSave, sender, handle, appID, parent, title, options)
}

func (o object) SaveFiles(sender dbus.Sender, handle dbus.ObjectPath, appID, parent, title string, options map[string]dbus.Variant) (uint32, map[string]dbus.Variant, *dbus.Error) {
	return o.service.serve(MethodSaveFiles, sender, handle, appID, parent, title, options)
}

type requestObject struct {
	service *Service
	id      string
}

func (r requestObject) Close() *dbus.Error {
	r.service.cancel(r.id)
	return nil
}

func New(conn *dbus.Conn, ui UI, globs func(mime string) []string) (*Service, error) {
	if conn == nil {
		return nil, errors.New("no session bus")
	}
	if ui == nil {
		return nil, errors.New("no chooser ui")
	}
	s := &Service{conn: conn, ui: ui, globs: globs, owner: dbusutil.NewNameOwner(conn, BusName, "filechooser"), pending: map[string]*pending{}, watched: map[string]int{}, signals: make(chan *dbus.Signal, 16)}
	if err := conn.Export(object{service: s}, Path, Interface); err != nil {
		return nil, err
	}
	if err := conn.Export(introspect.Introspectable(chooserIntrospection), Path, "org.freedesktop.DBus.Introspectable"); err != nil {
		return nil, err
	}
	conn.Signal(s.signals)
	go s.watchOwners()
	return s, nil
}

func (s *Service) serve(method string, sender dbus.Sender, handle dbus.ObjectPath, appID, parent, title string, options map[string]dbus.Variant) (uint32, map[string]dbus.Variant, *dbus.Error) {
	id := string(handle)
	if !handle.IsValid() || !strings.HasPrefix(id, requestPrefix) {
		return ResponseOther, map[string]dbus.Variant{}, dbus.NewError(errInvalidArgument, []any{errBadHandle.Error()})
	}
	req, derr := parseRequest(method, id, appID, parent, title, options, s.globs)
	if derr != nil {
		return ResponseOther, map[string]dbus.Variant{}, derr
	}
	p, err := s.begin(id, string(sender), *req)
	if err != nil {
		return ResponseOther, map[string]dbus.Variant{}, dbus.MakeFailedError(err)
	}
	s.ui.ShowChooser(*req)
	answer := <-p.done
	s.finish(id, p.sender)
	if answer.Response == ResponseOK && s.Chosen != nil {
		s.Chosen(*req, chosenPaths(answer.Paths))
	}
	return answer.Response, results(req, answer), nil
}

func chosenPaths(paths []string) []string {
	out := make([]string, 0, len(paths))
	for _, path := range paths {
		if filepath.IsAbs(path) {
			out = append(out, filepath.Clean(path))
		}
	}
	return out
}

func (s *Service) begin(id, sender string, req Request) (*pending, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, taken := s.pending[id]; taken {
		return nil, errors.New("request already open")
	}
	if err := s.conn.Export(requestObject{service: s, id: id}, dbus.ObjectPath(id), RequestInterface); err != nil {
		return nil, err
	}
	if err := s.conn.Export(introspect.Introspectable(requestIntrospection), dbus.ObjectPath(id), "org.freedesktop.DBus.Introspectable"); err != nil {
		_ = s.conn.Export(nil, dbus.ObjectPath(id), RequestInterface)
		return nil, err
	}
	p := &pending{sender: sender, request: req, done: make(chan Answer, 1)}
	s.pending[id] = p
	s.watchLocked(sender)
	return p, nil
}

func (s *Service) finish(id, sender string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.pending, id)
	_ = s.conn.Export(nil, dbus.ObjectPath(id), RequestInterface)
	_ = s.conn.Export(nil, dbus.ObjectPath(id), "org.freedesktop.DBus.Introspectable")
	s.unwatchLocked(sender)
}

func (s *Service) Answer(id string, answer Answer) bool {
	s.mu.Lock()
	p, ok := s.pending[id]
	s.mu.Unlock()
	if !ok {
		return false
	}
	p.resolve(answer)
	return true
}

func (s *Service) cancel(id string) {
	if s.Answer(id, Answer{Response: ResponseCancelled}) {
		s.ui.CloseChooser(id)
	}
}

func (s *Service) Pending() []Request {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Request, 0, len(s.pending))
	for _, p := range s.pending {
		out = append(out, p.request)
	}
	return out
}

func ownerMatch(sender string) []dbus.MatchOption {
	return []dbus.MatchOption{
		dbus.WithMatchSender("org.freedesktop.DBus"),
		dbus.WithMatchInterface("org.freedesktop.DBus"),
		dbus.WithMatchMember("NameOwnerChanged"),
		dbus.WithMatchArg(0, sender),
	}
}

func (s *Service) watchLocked(sender string) {
	if sender == "" {
		return
	}
	s.watched[sender]++
	if s.watched[sender] > 1 {
		return
	}
	if err := s.conn.AddMatchSignal(ownerMatch(sender)...); err != nil {
		log.Debugf("filechooser: watch %s: %v", sender, err)
	}
}

func (s *Service) unwatchLocked(sender string) {
	if sender == "" || s.watched[sender] == 0 {
		return
	}
	s.watched[sender]--
	if s.watched[sender] > 0 {
		return
	}
	delete(s.watched, sender)
	_ = s.conn.RemoveMatchSignal(ownerMatch(sender)...)
}

func (s *Service) watchOwners() {
	for sig := range s.signals {
		if sig.Name != "org.freedesktop.DBus.NameOwnerChanged" || len(sig.Body) < 3 {
			continue
		}
		name, _ := sig.Body[0].(string)
		newOwner, _ := sig.Body[2].(string)
		if newOwner != "" {
			continue
		}
		for _, id := range s.pendingFrom(name) {
			s.cancel(id)
		}
	}
}

func (s *Service) pendingFrom(sender string) []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	var ids []string
	for id, p := range s.pending {
		if p.sender == sender {
			ids = append(ids, id)
		}
	}
	return ids
}

func results(req *Request, answer Answer) map[string]dbus.Variant {
	out := map[string]dbus.Variant{}
	if answer.Response != ResponseOK {
		return out
	}
	uris := []string{}
	for _, path := range answer.Paths {
		if !filepath.IsAbs(path) {
			continue
		}
		uris = append(uris, fileuri.Build(filepath.Clean(path)))
	}
	if req.Method == MethodSave && len(uris) > 1 {
		uris = uris[:1]
	}
	out["uris"] = dbus.MakeVariant(uris)
	if len(req.Choices) > 0 {
		pairs := make([]wireChoiceValue, 0, len(answer.Choices))
		for _, c := range answer.Choices {
			pairs = append(pairs, wireChoiceValue(c))
		}
		out["choices"] = dbus.MakeVariant(pairs)
	}
	if answer.CurrentFilter >= 0 && answer.CurrentFilter < len(req.Filters) {
		out["current_filter"] = dbus.MakeVariant(toWire(req.Filters[answer.CurrentFilter]))
	}
	return out
}

func toWire(f Filter) wireFilter {
	w := wireFilter{Name: f.Name, Patterns: make([]wirePattern, 0, len(f.Patterns))}
	for _, p := range f.Patterns {
		kind := uint32(0)
		if p.Kind == KindMime {
			kind = 1
		}
		w.Patterns = append(w.Patterns, wirePattern{Kind: kind, Pattern: p.Pattern})
	}
	return w
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
	return Status{Enabled: s.owner.Enabled(), Owned: s.owner.Owned(), Reason: s.owner.Reason()}
}

func (s *Service) ReloadBus() error {
	return dbusutil.ReloadConfig(s.conn)
}

func (s *Service) FrontendPID() (int, bool) {
	return dbusutil.ProcessID(s.conn, FrontendName)
}

func (s *Service) Close() {
	s.mu.Lock()
	for id, p := range s.pending {
		p.resolve(Answer{Response: ResponseCancelled})
		s.ui.CloseChooser(id)
	}
	s.owner.Release()
	s.mu.Unlock()
	s.conn.RemoveSignal(s.signals)
	close(s.signals)
	s.conn.Close()
}

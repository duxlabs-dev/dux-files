package netmount

import (
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/duxlabs-dev/dux-files/core/internal/dbusutil"
	"github.com/duxlabs-dev/dux-go/files"
	"github.com/duxlabs-dev/dux-go/log"
	"github.com/godbus/dbus/v5"
)

const (
	EnumeratorIface = "org.gtk.vfs.Enumerator"
	EnumeratorRoot  = "/dev/duxlabs/files/enumerator/"

	shareAttributes      = "standard::name,standard::display-name,standard::type,standard::target-uri,smb::comment"
	gioFileTypeMountable = 6
)

var browseTimeout = 2 * time.Minute

type Share struct {
	Name    string `json:"name"`
	URI     string `json:"uri"`
	Comment string `json:"comment,omitempty"`
}

type wireAttribute struct {
	Name   string
	Status uint32
	Value  dbus.Variant
}

// godbus runs each incoming call in its own goroutine, so Done can overtake GotInfo.
// The interceptor sees the calls in bus order and does the collecting; the exported methods only acknowledge.
type enumerator struct {
	path   dbus.ObjectPath
	server string
	done   chan struct{}
	once   sync.Once

	mu    sync.Mutex
	infos [][]wireAttribute
}

func (e *enumerator) GotInfo(infos [][]wireAttribute) *dbus.Error { return nil }

func (e *enumerator) Done() *dbus.Error { return nil }

func (e *enumerator) receive(member string, body []any) {
	switch member {
	case "GotInfo":
		var infos [][]wireAttribute
		if err := dbus.Store(body, &infos); err != nil {
			log.Debugf("network: enumerator info from gvfs: %v", err)
			return
		}
		e.mu.Lock()
		e.infos = append(e.infos, infos...)
		e.mu.Unlock()
	case "Done":
		e.once.Do(func() { close(e.done) })
	}
}

func (s *Service) intercept(msg *dbus.Message) {
	if msg.Type != dbus.TypeMethodCall {
		return
	}
	path, _ := msg.Headers[dbus.FieldPath].Value().(dbus.ObjectPath)
	member, _ := msg.Headers[dbus.FieldMember].Value().(string)

	s.mu.RLock()
	lister := s.listers[path]
	s.mu.RUnlock()
	if lister == nil {
		return
	}
	lister.receive(member, msg.Body)
}

func (s *Service) listShares(conn *dbus.Conn, mount wireMount, server string) ([]Share, error) {
	lister := &enumerator{
		path:   dbus.ObjectPath(EnumeratorRoot + strconv.FormatUint(uint64(s.opCounter.Add(1)), 10)),
		server: server,
		done:   make(chan struct{}),
	}
	if err := conn.Export(lister, lister.path, EnumeratorIface); err != nil {
		return nil, &files.Error{Code: files.CodeIO, Err: err}
	}
	s.mu.Lock()
	s.listers[lister.path] = lister
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		delete(s.listers, lister.path)
		s.mu.Unlock()
		_ = conn.Export(nil, lister.path, EnumeratorIface)
	}()

	err := conn.Object(mount.DBusID, mount.ObjectPath).Call(MountIface+".Enumerate", 0, byteStringOf("/"), string(lister.path), shareAttributes, uint32(0), server).Store()
	if err != nil {
		return nil, wrapDBus(err)
	}

	select {
	case <-lister.done:
	case <-time.After(browseTimeout):
		return nil, fail(files.CodeIO, "%s did not finish listing its shares", server)
	}

	lister.mu.Lock()
	defer lister.mu.Unlock()
	return sharesOf(server, lister.infos), nil
}

func sharesOf(server string, infos [][]wireAttribute) []Share {
	shares := []Share{}
	for _, info := range infos {
		share, ok := shareOf(server, info)
		if !ok {
			continue
		}
		shares = append(shares, share)
	}
	sort.SliceStable(shares, func(i, j int) bool {
		return strings.ToLower(shares[i].Name) < strings.ToLower(shares[j].Name)
	})
	return shares
}

// gvfsd-smb-browse writes standard::target-uri without the port, so the uri is built from the server.
func shareOf(server string, info []wireAttribute) (Share, bool) {
	var share Share
	var name string
	mountable := false
	for _, attribute := range info {
		switch attribute.Name {
		case "standard::name":
			name = attributeString(attribute.Value)
		case "standard::display-name":
			share.Name = attributeString(attribute.Value)
		case "smb::comment":
			share.Comment = attributeString(attribute.Value)
		case "standard::type":
			fileType, _ := attribute.Value.Value().(uint32)
			mountable = fileType == gioFileTypeMountable
		}
	}
	if share.Name == "" {
		share.Name = name
	}
	if !mountable || name == "" {
		return Share{}, false
	}
	share.URI = strings.TrimSuffix(server, "/") + "/" + url.PathEscape(name)
	return share, true
}

func attributeString(value dbus.Variant) string {
	switch v := value.Value().(type) {
	case string:
		return v
	case []byte:
		return dbusutil.ByteString(v)
	}
	return ""
}

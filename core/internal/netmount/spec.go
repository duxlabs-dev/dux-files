package netmount

import (
	"net/url"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/duxlabs-dev/dux-go/files"
)

type Mountable struct {
	Type        string
	Scheme      string
	Aliases     []string
	DefaultPort int32
	HostIsInet  bool
}

type Spec struct {
	Prefix string
	Items  map[string]string
}

func (s Spec) Type() string { return s.Items["type"] }

var connectSchemes = []string{"sftp", "smb", "ftp", "dav", "davs", "nfs"}

func availableSchemes(mountables []Mountable) []string {
	schemes := []string{}
	for _, scheme := range connectSchemes {
		if _, ok := FindMountable(scheme, mountables); ok {
			schemes = append(schemes, scheme)
		}
	}
	return schemes
}

func FindMountable(scheme string, mountables []Mountable) (Mountable, bool) {
	scheme = strings.ToLower(scheme)
	for _, m := range mountables {
		if m.Scheme == scheme || m.Type == scheme {
			return m, true
		}
		for _, alias := range m.Aliases {
			if alias == scheme {
				return m, true
			}
		}
	}
	return Mountable{}, false
}

func ParseURI(raw string, mountables []Mountable) (Spec, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return Spec{}, fail(files.CodeInvalid, "%s is not a valid address", raw)
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return Spec{}, fail(files.CodeInvalid, "%s needs a scheme and a host, for example sftp://host/path", raw)
	}

	mountable, ok := FindMountable(parsed.Scheme, mountables)
	if !ok {
		return Spec{}, fail(files.CodeNotSupported, "no gvfs backend handles %s://", parsed.Scheme)
	}

	spec := Spec{Prefix: "/", Items: map[string]string{"type": mountable.Type}}
	if user := parsed.User.Username(); user != "" {
		spec.Items["user"] = user
	}
	if port := parsed.Port(); port != "" {
		if number, err := strconv.Atoi(port); err == nil && int32(number) != mountable.DefaultPort {
			spec.Items["port"] = port
		}
	}
	if path := strings.Trim(parsed.Path, "/"); path != "" {
		spec.Prefix = "/" + path
	}

	if mountable.Scheme == "smb" {
		return smbSpec(spec, parsed.Hostname()), nil
	}
	spec.Items["host"] = parsed.Hostname()
	return davSpec(spec, parsed.Scheme), nil
}

// gvfsd-dav registers no davs backend; davs:// is type dav with ssl=true, as in gvfs's httpuri.c.
func davSpec(spec Spec, scheme string) Spec {
	switch scheme {
	case "davs":
		spec.Items["type"] = "dav"
	case "davs+sd":
		spec.Items["type"] = "dav+sd"
	default:
		return spec
	}
	spec.Items["ssl"] = "true"
	return spec
}

func smbSpec(spec Spec, host string) Spec {
	spec.Items["server"] = host
	segments := strings.Split(strings.Trim(spec.Prefix, "/"), "/")
	if segments[0] == "" {
		spec.Items["type"] = "smb-server"
		spec.Prefix = "/"
		return spec
	}

	spec.Items["type"] = "smb-share"
	spec.Items["share"] = segments[0]
	spec.Prefix = "/" + strings.Join(segments[1:], "/")
	if user, ok := spec.Items["user"]; ok {
		domain, name, split := strings.Cut(user, ";")
		if split {
			spec.Items["domain"] = domain
			spec.Items["user"] = name
		}
	}
	return spec
}

func URIOf(spec Spec, mountables []Mountable) string {
	scheme := spec.Type()
	if mountable, ok := FindMountable(spec.Type(), mountables); ok {
		scheme = mountable.Scheme
	}
	if scheme == "dav" && spec.Items["ssl"] == "true" {
		scheme = "davs"
	}

	host := spec.Items["host"]
	path := spec.Prefix
	if server := spec.Items["server"]; server != "" {
		host = server
		path = "/" + strings.Trim(spec.Items["share"]+strings.TrimSuffix(path, "/"), "/")
	}
	if host == "" {
		return scheme + ":" + strings.TrimSuffix(path, "/")
	}

	authority := host
	if port := spec.Items["port"]; port != "" {
		authority += ":" + port
	}
	if user := spec.Items["user"]; user != "" {
		authority = url.User(user).String() + "@" + authority
	}
	if path == "" {
		path = "/"
	}
	return scheme + "://" + authority + path
}

func SchemeOf(spec Spec, mountables []Mountable) string {
	if mountable, ok := FindMountable(spec.Type(), mountables); ok {
		return mountable.Scheme
	}
	return spec.Type()
}

func LocationIn(mount Mount, uri string) string {
	rest, ok := strings.CutPrefix(uri, mount.URI)
	switch {
	case mount.Path == "", !ok:
		return mount.Path
	case rest != "" && !strings.HasSuffix(mount.URI, "/") && !strings.HasPrefix(rest, "/"):
		return mount.Path
	}
	return filepath.Join(mount.Path, filepath.Clean("/"+rest))
}

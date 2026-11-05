package location

import (
	"errors"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/duxlabs-dev/dux-go/paths"

	"github.com/duxlabs-dev/dux-files/core/internal/wire"

	"github.com/duxlabs-dev/dux-files/core/internal/fileuri"

	"github.com/duxlabs-dev/dux-go/files"
)

type Kind string

const (
	KindFolder  Kind = "folder"
	KindArchive Kind = "archive"
	KindRemote  Kind = "remote"
	KindServers Kind = "servers"
	KindRecent  Kind = "recent"
)

type Location struct {
	Kind     Kind     `json:"kind"`
	Path     string   `json:"path,omitempty"`
	URI      string   `json:"uri,omitempty"`
	Password string   `json:"password,omitempty"`
	Select   []string `json:"select"`
}

const CodeOtherHome files.Code = "EOTHERHOME"

type Env struct {
	TrashFiles string
	Stat       func(path string) (fs.FileInfo, error)
	Lstat      func(path string) (fs.FileInfo, error)
	IsArchive  func(path string) bool
}

var remoteSchemes = []string{"sftp", "ssh", "smb", "ftp", "ftps", "dav", "davs", "nfs", "afp"}

var uriPrefix = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9+.-]*:/`)

func Absolute(raw, cwd string) string {
	switch {
	case uriPrefix.MatchString(raw):
		return raw
	case filepath.IsAbs(raw):
		return filepath.Clean(raw)
	}
	return filepath.Join(cwd, raw)
}

func Resolve(open, sel []string, env Env) ([]Location, error) {
	var out []Location
	folderAt := map[string]int{}

	addFolder := func(path string) int {
		if index, ok := folderAt[path]; ok {
			return index
		}
		out = append(out, Location{Kind: KindFolder, Path: path, Select: []string{}})
		folderAt[path] = len(out) - 1
		return len(out) - 1
	}
	addSelection := func(path string) {
		if path == "/" {
			addFolder(path)
			return
		}
		index := addFolder(filepath.Dir(path))
		if !slices.Contains(out[index].Select, path) {
			out[index].Select = append(out[index].Select, path)
		}
	}

	for _, raw := range open {
		loc, err := Parse(raw, env)
		if err != nil {
			return nil, err
		}
		if loc.Kind != KindFolder {
			out = append(out, loc)
			continue
		}
		if isFolder(loc.Path, env) {
			addFolder(loc.Path)
			continue
		}
		if _, err := env.Lstat(loc.Path); err != nil {
			return nil, describe(loc.Path, err)
		}
		if env.IsArchive != nil && env.IsArchive(loc.Path) {
			out = append(out, Location{Kind: KindArchive, Path: loc.Path, Select: []string{}})
			continue
		}
		addSelection(loc.Path)
	}

	for _, raw := range sel {
		loc, err := Parse(raw, env)
		if err != nil {
			return nil, err
		}
		if loc.Kind != KindFolder {
			return nil, wire.Fail(files.CodeInvalid, "%s cannot be selected, only opened", redact(raw))
		}
		if _, err := env.Lstat(loc.Path); err != nil {
			return nil, describe(loc.Path, err)
		}
		addSelection(loc.Path)
	}
	return out, nil
}

func LocalPaths(targets []string, env Env) ([]string, error) {
	out := make([]string, 0, len(targets))
	for _, raw := range targets {
		loc, err := Parse(raw, env)
		if err != nil {
			return nil, err
		}
		if loc.Kind != KindFolder {
			return nil, wire.Fail(files.CodeInvalid, "%s is not a local path", redact(raw))
		}
		if _, err := env.Lstat(loc.Path); err != nil {
			return nil, describe(loc.Path, err)
		}
		out = append(out, loc.Path)
	}
	return out, nil
}

func Typed(text, cwd, home string, env Env) (Location, error) {
	raw := strings.TrimSpace(text)
	switch {
	case raw == "":
		return Location{}, wire.Fail(files.CodeInvalid, "empty location")
	case raw == "~", strings.HasPrefix(raw, "~/"):
		if !filepath.IsAbs(home) {
			return Location{}, wire.Fail(files.CodeInvalid, "the home folder is unknown")
		}
		raw = filepath.Join(home, raw[1:])
	case strings.HasPrefix(raw, "~"):
		user, _, _ := strings.Cut(raw, "/")
		return Location{}, wire.Fail(CodeOtherHome, "%s names another user's home folder; use ~ for yours or type the full path", user)
	case !uriPrefix.MatchString(raw) && !filepath.IsAbs(raw):
		if !filepath.IsAbs(cwd) {
			return Location{}, wire.Fail(files.CodeInvalid, "%s is relative and there is no folder to resolve it against", raw)
		}
		raw = filepath.Join(cwd, raw)
	}

	loc, err := Parse(raw, env)
	if err != nil || loc.Kind != KindFolder {
		return loc, err
	}
	if isFolder(loc.Path, env) {
		return loc, nil
	}
	if _, err := env.Lstat(loc.Path); err != nil {
		return Location{}, describe(loc.Path, err)
	}
	return Location{Kind: KindFolder, Path: filepath.Dir(loc.Path), Select: []string{loc.Path}}, nil
}

func Parse(raw string, env Env) (Location, error) {
	switch {
	case raw == "":
		return Location{}, wire.Fail(files.CodeInvalid, "empty location")
	case strings.HasPrefix(raw, "/"):
		return folder(filepath.Clean(raw)), nil
	}

	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" {
		return Location{}, wire.Fail(files.CodeInvalid, "%s is not an absolute path or a supported URI", redact(raw))
	}

	scheme := strings.ToLower(u.Scheme)
	switch scheme {
	case "file":
		if !fileuri.LocalHost(u.Host) {
			return Location{}, wire.Fail(files.CodeNotSupported, "%s is on another host", redact(raw))
		}
		return absolute(raw, u.Path)
	case "admin":
		return absolute(raw, u.Path)
	case "trash":
		if env.TrashFiles == "" {
			return Location{}, wire.Fail(files.CodeNotSupported, "the trash location is unknown")
		}
		return folder(filepath.Join(env.TrashFiles, filepath.Clean("/"+u.Path))), nil
	case "computer":
		return folder("/"), nil
	case "network":
		return Location{Kind: KindServers, Select: []string{}}, nil
	case "recent":
		return Location{Kind: KindRecent, Select: []string{}}, nil
	}

	if !slices.Contains(remoteSchemes, scheme) {
		return Location{}, wire.Fail(files.CodeNotSupported, "%s:// locations are not supported", scheme)
	}
	if u.Host == "" {
		return Location{}, wire.Fail(files.CodeInvalid, "%s needs a host, for example %s://host/path", redact(raw), scheme)
	}
	return SplitPassword(raw), nil
}

func SplitPassword(raw string) Location {
	remote := Location{Kind: KindRemote, URI: raw, Select: []string{}}
	u, err := url.Parse(raw)
	if err != nil || u.User == nil {
		return remote
	}
	password, ok := u.User.Password()
	if !ok {
		return remote
	}
	name := u.User.Username()
	u.User = nil
	if name != "" {
		u.User = url.User(name)
	}
	remote.URI = u.String()
	remote.Password = password
	return remote
}

func redact(raw string) string {
	scheme, rest, ok := strings.Cut(raw, "://")
	if !ok {
		return raw
	}
	end := strings.IndexAny(rest, "/?#")
	if end < 0 {
		end = len(rest)
	}
	at := strings.LastIndex(rest[:end], "@")
	if at < 0 {
		return raw
	}
	user, _, _ := strings.Cut(rest[:at], ":")
	if user != "" {
		user += "@"
	}
	return scheme + "://" + user + rest[at+1:]
}

func absolute(raw, path string) (Location, error) {
	if !filepath.IsAbs(path) {
		return Location{}, wire.Fail(files.CodeInvalid, "%s does not name an absolute path", redact(raw))
	}
	return folder(filepath.Clean(path)), nil
}

func isFolder(path string, env Env) bool {
	if env.TrashFiles != "" && path == env.TrashFiles {
		return true
	}
	info, err := env.Stat(path)
	return err == nil && info.IsDir()
}

func folder(path string) Location {
	return Location{Kind: KindFolder, Path: path, Select: []string{}}
}

func describe(path string, err error) error {
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return wire.Fail(files.CodeNotFound, "%s does not exist", path)
	case errors.Is(err, fs.ErrPermission):
		return wire.Fail(files.CodeAccess, "%s is not accessible", path)
	}
	return wire.Fail(files.CodeIO, "%s: %s", path, err)
}

func HostEnv(dataHome string, isArchive func(string) bool) Env {
	return Env{TrashFiles: filepath.Join(dataHome, "Trash", "files"), Stat: os.Stat, Lstat: os.Lstat, IsArchive: isArchive}
}

func HomeEnv(isArchive func(string) bool) Env {
	return HostEnv(paths.XDGDataHome(), isArchive)
}

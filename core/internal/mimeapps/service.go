package mimeapps

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/duxlabs-dev/dux-go/files"
)

func fail(code files.Code, format string, args ...any) error {
	return &files.Error{Code: code, Err: fmt.Errorf(format, args...)}
}

type Service struct {
	detector *Detector
	entries  *entryIndex
	Opened   func(paths, mimes []string, app *Entry)
}

func NewService() *Service { return NewServiceForDirs(MimeDirs()) }

func NewServiceForDirs(mimeDirs []string) *Service {
	return &Service{detector: NewDetector(LoadDB(mimeDirs)), entries: newEntryIndex()}
}

func (s *Service) Detector() *Detector { return s.detector }

func (s *Service) EntryByID(id string) *Entry { return s.entries.byIdentifier(id) }

func (s *Service) Apps() []*Entry {
	var out []*Entry
	for _, entry := range s.entries.all() {
		if entry.NoDisplay || !entry.Launchable() {
			continue
		}
		out = append(out, entry)
	}
	sort.SliceStable(out, func(i, j int) bool {
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})
	return out
}

func shown(entries []*Entry, defaultID string) []*Entry {
	var out []*Entry
	for _, entry := range entries {
		if entry.NoDisplay && entry.ID != defaultID {
			continue
		}
		out = append(out, entry)
	}
	return out
}

func (s *Service) MimeOf(path string) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", files.Wrap(path, err)
	}
	return s.detector.Detect(path, info.IsDir()), nil
}

func (s *Service) Open(ctx context.Context, paths []string, opts LaunchOptions) error {
	if len(paths) == 0 {
		return fail(files.CodeInvalid, "no paths given")
	}
	if opts.Opener != "" {
		return s.openViaCommand(paths, opts)
	}
	if Sandboxed() {
		return openViaPortal(ctx, paths, false)
	}

	groups, unhandled, err := s.groupByApp(paths)
	if err != nil {
		return err
	}

	for _, group := range groups {
		batch := opts
		batch.Paths = group.paths
		entry := s.EntryByID(group.app)
		if err := s.Launch(entry, batch); err != nil {
			return &files.Error{Code: files.CodeIO, Err: err}
		}
		s.noteOpened(entry, group.paths, group.mimes)
	}
	if len(unhandled) > 0 {
		return fail(files.CodeNotSupported, "no application is registered for %s", strings.Join(unhandled, ", "))
	}
	return nil
}

func (s *Service) openViaCommand(paths []string, opts LaunchOptions) error {
	command := strings.Fields(opts.Opener)
	if len(command) == 0 {
		return fail(files.CodeInvalid, "empty opener command")
	}
	for _, path := range paths {
		dir := cmp.Or(opts.WorkingDir, filepath.Dir(path))
		argv := append(slices.Clone(command), path)
		var err error
		if Sandboxed() {
			err = hostSpawn(argv, dir)
		} else {
			err = spawn(argv, dir, opts.ActivationToken, false)
		}
		switch {
		case err == nil:
		case errors.Is(err, exec.ErrNotFound):
			return fail(files.CodeNotFound, "opener %q is not installed", command[0])
		default:
			return fail(files.CodeIO, "opener %s: %v", command[0], err)
		}
	}
	s.noteOpened(nil, paths, nil)
	return nil
}

func (s *Service) OpenWith(ctx context.Context, desktopID string, paths []string, opts LaunchOptions) error {
	if len(paths) == 0 {
		return fail(files.CodeInvalid, "no paths given")
	}
	if Sandboxed() {
		return openViaPortal(ctx, paths, true)
	}

	entry := s.EntryByID(desktopID)
	if !entry.Launchable() {
		return fail(files.CodeNotFound, "desktop entry %q is not installed", desktopID)
	}
	opts.Paths = paths
	if err := s.Launch(entry, opts); err != nil {
		return &files.Error{Code: files.CodeIO, Err: err}
	}
	s.noteOpened(entry, paths, nil)
	return nil
}

func (s *Service) noteOpened(entry *Entry, paths, mimes []string) {
	if s.Opened != nil {
		s.Opened(paths, mimes, entry)
	}
}

// The execute bit is also what marks a desktop file trusted.
func (s *Service) Run(path string, opts LaunchOptions) error {
	info, err := os.Stat(path)
	if err != nil {
		return files.Wrap(path, err)
	}
	if info.IsDir() {
		return fail(files.CodeInvalid, "%s is a directory", path)
	}
	if info.Mode()&0o111 == 0 {
		return fail(files.CodeAccess, "%s is not marked executable", path)
	}

	if s.detector.Detect(path, false) == TypeDesktop {
		entry := parseDesktopFile(path, baseName(path))
		if entry == nil {
			return fail(files.CodeInvalid, "%s is not a valid desktop entry", path)
		}
		opts.Paths = nil
		return s.Launch(entry, opts)
	}

	if Sandboxed() {
		return hostSpawn([]string{path}, opts.WorkingDir)
	}
	return spawn([]string{path}, opts.WorkingDir, opts.ActivationToken, false)
}

func (s *Service) OpenURI(ctx context.Context, uri string) error {
	scheme, _, ok := strings.Cut(uri, ":")
	if !ok || scheme == "" {
		return fail(files.CodeInvalid, "invalid uri %q", uri)
	}
	if scheme == "file" {
		path := fileURIPath(uri)
		if path == "" {
			return fail(files.CodeInvalid, "invalid file uri %q", uri)
		}
		return s.Open(ctx, []string{path}, LaunchOptions{})
	}
	if Sandboxed() {
		return openURIViaPortal(ctx, uri)
	}

	id := s.Default("x-scheme-handler/" + strings.ToLower(scheme))
	if id == "" {
		return fail(files.CodeNotSupported, "no handler registered for %q", scheme)
	}
	if err := s.Launch(s.EntryByID(id), LaunchOptions{Paths: []string{uri}}); err != nil {
		return &files.Error{Code: files.CodeIO, Err: err}
	}
	return nil
}

type appGroup struct {
	app   string
	paths []string
	mimes []string
}

func (s *Service) groupByApp(paths []string) ([]appGroup, []string, error) {
	var groups []appGroup
	var unhandled []string
	at := map[string]int{}
	defaults := map[string]string{}

	for _, path := range paths {
		mimeType, err := s.MimeOf(path)
		if err != nil {
			return nil, nil, err
		}
		app, known := defaults[mimeType]
		if !known {
			app = s.Default(mimeType)
			defaults[mimeType] = app
		}
		if app == "" {
			if !slices.Contains(unhandled, mimeType) {
				unhandled = append(unhandled, mimeType)
			}
			continue
		}
		index, ok := at[app]
		if !ok {
			index = len(groups)
			at[app] = index
			groups = append(groups, appGroup{app: app})
		}
		groups[index].paths = append(groups[index].paths, path)
		groups[index].mimes = append(groups[index].mimes, mimeType)
	}
	return groups, unhandled, nil
}

func fileURIPath(uri string) string {
	parsed, err := url.Parse(uri)
	if err != nil || parsed.Scheme != "file" {
		return ""
	}
	return parsed.Path
}

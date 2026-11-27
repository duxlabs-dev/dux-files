package fileops

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/duxlabs-dev/dux-go/files"
)

const (
	CodePasswordNeeded files.Code = "PASSWORD"
	CodeBadPassword    files.Code = "BADPASSWORD"

	archiveIndexLimit = 4
	archiveIndexIdle  = 10 * time.Minute
	memberCacheAge    = 24 * time.Hour
)

type ArchiveEntry struct {
	Name     string
	Dir      bool
	Symlink  bool
	Link     string
	Size     int64
	Modified time.Time
	Perm     fs.FileMode
}

type archiveIndex struct {
	size     int64
	modified time.Time
	children map[string][]ArchiveEntry
	slots    map[string]int
}

func (i *archiveIndex) add(path string, entry ArchiveEntry) {
	parent := filepath.Dir(path)
	if parent == "." {
		parent = ""
	}
	if _, listed := i.children[path]; entry.Dir && !listed {
		i.children[path] = nil
	}
	if at, known := i.slots[path]; known {
		i.children[parent][at] = entry
		return
	}
	if parent != "" {
		i.ensureDir(parent)
	}
	i.slots[path] = len(i.children[parent])
	i.children[parent] = append(i.children[parent], entry)
}

func (i *archiveIndex) ensureDir(path string) {
	if _, known := i.slots[path]; known {
		return
	}
	i.add(path, ArchiveEntry{Name: filepath.Base(path), Dir: true, Perm: 0o755})
}

func (i *archiveIndex) matches(info fs.FileInfo) bool {
	return i.size == info.Size() && i.modified.Equal(info.ModTime())
}

// LocateArchive splits a path at the archive file it runs through; a path that exists on disk is never inside one.
func LocateArchive(path string) (archive, inner string, ok bool) {
	path = filepath.Clean(path)
	for at := path; at != "/" && at != "."; at = filepath.Dir(at) {
		info, err := os.Stat(at)
		switch {
		case errors.Is(err, syscall.ENOTDIR):
			continue
		case err != nil, !info.Mode().IsRegular(), !IsArchive(at):
			return "", "", false
		}
		return at, strings.TrimPrefix(path[len(at):], "/"), true
	}
	return "", "", false
}

func archiveMembers(sources []string) (archive string, members []string, err error) {
	archive, first, ok := LocateArchive(sources[0])
	if !ok || first == "" {
		return "", nil, nil
	}
	prefix := archive + "/"
	for _, src := range sources {
		if !strings.HasPrefix(src, prefix) {
			return "", nil, &files.Error{Code: files.CodeInvalid, Err: errors.New("cannot mix archive entries with other paths: " + src)}
		}
		members = append(members, src[len(prefix):])
	}
	return archive, members, nil
}

func buildArchiveIndex(archive string, info fs.FileInfo, password string) (*archiveIndex, error) {
	format, err := detectFormat(archive)
	if err != nil {
		return nil, &files.Error{Code: files.CodeNotSupported, Err: err}
	}
	in, err := os.Open(archive)
	if err != nil {
		return nil, files.Wrap(archive, err)
	}
	defer in.Close()

	stream, closeStream, err := openStream(in, format.compression)
	if err != nil {
		return nil, err
	}
	defer closeStream()

	kind := format.container
	if kind == containerNone && streamHoldsTar(stream) {
		kind = containerTar
	}
	if kind == containerNone {
		return nil, &files.Error{Code: files.CodeNotSupported, Err: errors.New("a compressed file has no folders to browse")}
	}
	var source io.Reader = stream
	if format.compression == compressNone {
		if _, err := in.Seek(0, io.SeekStart); err != nil {
			return nil, err
		}
		source = in
	}
	reader, err := newArchiveReader(kind, in, source, password)
	if err != nil {
		return nil, passwordCoded(err)
	}

	index := &archiveIndex{
		size:     info.Size(),
		modified: info.ModTime(),
		children: map[string][]ArchiveEntry{"": nil},
		slots:    map[string]int{},
	}
	for {
		m, err := reader.next()
		switch {
		case errors.Is(err, io.EOF):
			return index, nil
		case err != nil:
			return nil, passwordCoded(err)
		}
		name, err := entryName(m.name)
		if err != nil || m.kind == memberOther {
			continue
		}
		entry := ArchiveEntry{
			Name:     filepath.Base(name),
			Dir:      m.kind == memberDir,
			Symlink:  m.kind == memberSymlink,
			Size:     m.size,
			Modified: m.modified,
			Perm:     m.perm,
		}
		if entry.Symlink {
			entry.Link = m.link
		}
		index.add(name, entry)
	}
}

func passwordCoded(err error) error {
	switch {
	case errors.Is(err, errPasswordNeeded):
		return &files.Error{Code: CodePasswordNeeded, Err: err}
	case isBadPassword(err):
		return &files.Error{Code: CodeBadPassword, Err: err}
	}
	return err
}

type archiveSlot struct {
	ready chan struct{}
	index *archiveIndex
	err   error
}

func (s *archiveSlot) built() bool {
	select {
	case <-s.ready:
		return true
	default:
		return false
	}
}

type archiveCache struct {
	mu        sync.Mutex
	slots     map[string]*archiveSlot
	recent    []string
	passwords map[string]string
	idle      *time.Timer
}

func newArchiveCache() *archiveCache {
	return &archiveCache{slots: map[string]*archiveSlot{}, passwords: map[string]string{}}
}

func (c *archiveCache) password(archive string) string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.passwords[archive]
}

func (c *archiveCache) keepPassword(archive, password string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.passwords[archive] = password
	c.touch(archive)
}

func (c *archiveCache) touch(archive string) {
	c.recent = append(slices.DeleteFunc(c.recent, func(known string) bool { return known == archive }), archive)
	for len(c.recent) > archiveIndexLimit {
		delete(c.slots, c.recent[0])
		delete(c.passwords, c.recent[0])
		c.recent = c.recent[1:]
	}
	if c.idle == nil {
		c.idle = time.AfterFunc(archiveIndexIdle, c.clear)
		return
	}
	c.idle.Reset(archiveIndexIdle)
}

func (c *archiveCache) clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.slots = map[string]*archiveSlot{}
	c.passwords = map[string]string{}
	c.recent = nil
}

func (c *archiveCache) index(archive string) (*archiveIndex, error) {
	info, err := os.Stat(archive)
	if err != nil {
		return nil, files.Wrap(archive, err)
	}

	c.mu.Lock()
	slot := c.slots[archive]
	if slot != nil && slot.built() && !slot.index.matches(info) {
		slot = nil
	}
	if slot != nil {
		c.touch(archive)
		c.mu.Unlock()
		<-slot.ready
		return slot.index, slot.err
	}
	slot = &archiveSlot{ready: make(chan struct{})}
	c.slots[archive] = slot
	c.touch(archive)
	password := c.passwords[archive]
	c.mu.Unlock()

	slot.index, slot.err = buildArchiveIndex(archive, info, password)
	close(slot.ready)
	if slot.err == nil {
		return slot.index, nil
	}
	c.mu.Lock()
	if c.slots[archive] == slot {
		delete(c.slots, archive)
	}
	c.mu.Unlock()
	return nil, slot.err
}

func (c *archiveCache) unlock(archive, password string) error {
	info, err := os.Stat(archive)
	if err != nil {
		return files.Wrap(archive, err)
	}
	index, err := buildArchiveIndex(archive, info, password)
	if err != nil {
		return err
	}
	slot := &archiveSlot{ready: make(chan struct{}), index: index}
	close(slot.ready)

	c.mu.Lock()
	defer c.mu.Unlock()
	c.slots[archive] = slot
	c.passwords[archive] = password
	c.touch(archive)
	return nil
}

func (m *Manager) ArchiveChildren(archive, inner string) ([]ArchiveEntry, error) {
	index, err := m.archives.index(archive)
	if err != nil {
		return nil, err
	}
	children, found := index.children[filepath.Clean("/" + inner)[1:]]
	if !found {
		return nil, &files.Error{Code: files.CodeNotFound, Path: filepath.Join(archive, inner), Err: fs.ErrNotExist}
	}
	return children, nil
}

func (r *runner) unlockArchive() error {
	archive := r.job.spec.sources[0]
	return skipToNil(r.unlock(archive, func(password string) error {
		return r.job.mgr.archives.unlock(archive, password)
	}))
}

func (m *Manager) SetMemberCache(dir string) {
	m.mu.Lock()
	m.memberCache = dir
	m.mu.Unlock()
	pruneMemberCache(dir)
}

func pruneMemberCache(dir string) {
	entries, _ := os.ReadDir(dir)
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil || time.Since(info.ModTime()) < memberCacheAge {
			continue
		}
		_ = os.RemoveAll(filepath.Join(dir, entry.Name()))
	}
}

func (m *Manager) memberDir(archive, member string) (string, error) {
	m.mu.Lock()
	base := m.memberCache
	m.mu.Unlock()
	if base == "" {
		return "", &files.Error{Code: files.CodeNotSupported, Err: errors.New("no cache folder for opening archive entries")}
	}
	info, err := os.Stat(archive)
	if err != nil {
		return "", files.Wrap(archive, err)
	}
	key := sha256.Sum256([]byte(archive + "\x00" + strconv.FormatInt(info.Size(), 10) + "\x00" + strconv.FormatInt(info.ModTime().UnixNano(), 10)))
	return filepath.Join(base, hex.EncodeToString(key[:8]), filepath.Dir(member)), nil
}

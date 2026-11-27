package fileops

import (
	"bufio"
	"compress/bzip2"
	"compress/gzip"
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/duxlabs-dev/dux-go/files"
	"github.com/klauspost/compress/zstd"
	"github.com/sorairolake/lzip-go"
	"github.com/ulikunitz/xz"
	"github.com/ulikunitz/xz/lzma"
)

const (
	LayoutAuto   = "auto"
	LayoutFolder = "folder"
	LayoutFlat   = "flat"
)

var extractLayouts = []string{LayoutAuto, LayoutFolder, LayoutFlat}

type placed struct {
	Created
	undoable bool
}

func (r *runner) extract() error {
	r.setTotals(0, 0, true)
	if r.job.spec.toCache {
		return r.extractToCache()
	}

	pick := newPicker(r.job.spec.entries, false)
	return eachSource(r, r.job.spec.sources, func(archive string) error {
		items, err := r.unpack(archive, pick)
		for _, item := range items {
			if item.undoable {
				r.remember(KindDelete, item.Path, "")
			}
			r.noteCreated(item.Path, item.Dir)
		}
		return err
	})
}

func (r *runner) extractToCache() error {
	spec := r.job.spec
	target := filepath.Join(spec.dest, filepath.Base(spec.entries[0]))
	if _, err := os.Lstat(target); err == nil {
		r.noteCreated(target, false)
		return nil
	}
	if err := os.MkdirAll(spec.dest, 0o700); err != nil {
		return err
	}
	items, err := r.unpack(spec.sources[0], newPicker(spec.entries, true))
	for _, item := range items {
		r.noteCreated(item.Path, item.Dir)
	}
	return skipToNil(err)
}

func (r *runner) unpack(archive string, pick *picker) (items []placed, err error) {
	format, err := detectFormat(archive)
	if err != nil {
		return nil, err
	}
	err = r.unlock(archive, func(password string) error {
		items, err = r.unpackWith(archive, format, pick, password)
		return err
	})
	return items, err
}

func (r *runner) layout() string {
	switch {
	case r.job.spec.layout != "":
		return r.job.spec.layout
	case len(r.job.spec.entries) > 0:
		return LayoutFlat
	}
	return LayoutAuto
}

func (r *runner) unpackWith(archive string, format archiveFormat, pick *picker, password string) ([]placed, error) {
	var in *os.File
	if err := r.retryLocal(archive, func() error {
		opened, err := os.Open(archive)
		in = opened
		return err
	}); err != nil {
		return nil, err
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
		target, err := r.unpackFile(archive, stream)
		if err != nil {
			return nil, err
		}
		return []placed{{Created{Path: target}, true}}, nil
	}

	reader, err := newArchiveReader(kind, in, stream, password)
	if err != nil {
		return nil, err
	}
	layout := r.layout()
	holding := r.targetPath(archive)
	if layout == LayoutFlat {
		holding = r.tempPath(holding)
	}
	if err := os.Mkdir(holding, 0o755); err != nil {
		return nil, err
	}
	before := r.progress()
	fail := func(leftover string, err error) ([]placed, error) {
		_ = os.RemoveAll(leftover)
		r.restoreProgress(before)
		return nil, err
	}
	root, err := os.OpenRoot(holding)
	if err != nil {
		return fail(holding, err)
	}
	err = r.unpackMembers(reader, root, pick, password)
	_ = root.Close()
	if err != nil {
		return fail(holding, err)
	}

	switch layout {
	case LayoutFlat:
		return r.settle(holding)
	case LayoutFolder:
		return []placed{{Created{Path: holding, Dir: true}, true}}, nil
	}
	created, dir, err := r.hoistSoleEntry(holding, archiveStem(filepath.Base(archive)))
	if err != nil {
		return fail(created, err)
	}
	return []placed{{Created{Path: created, Dir: dir}, true}}, nil
}

func (r *runner) settle(staging string) ([]placed, error) {
	entries, err := os.ReadDir(staging)
	if err != nil {
		_ = os.RemoveAll(staging)
		return nil, err
	}
	r.growTotals(len(entries))

	var items []placed
	for _, entry := range entries {
		done, err := r.transfer(filepath.Join(staging, entry.Name()), filepath.Join(r.job.spec.dest, entry.Name()), true, false)
		switch {
		case errors.Is(err, errSkipped):
			continue
		case err != nil:
			_ = os.RemoveAll(staging)
			return items, err
		}
		items = append(items, placed{Created{Path: done.dst, Dir: done.dir}, done.undoable})
	}
	return items, os.RemoveAll(staging)
}

func (r *runner) hoistSoleEntry(target, stem string) (string, bool, error) {
	entries, err := os.ReadDir(target)
	if err != nil || len(entries) != 1 || !sameStem(entries[0].Name(), stem) {
		return target, true, nil
	}
	name, dir := entries[0].Name(), entries[0].IsDir()
	staging := r.tempPath(target)
	if err := os.Rename(target, staging); err != nil {
		return target, true, err
	}
	final := filepath.Join(r.job.spec.dest, name)
	if _, err := os.Lstat(final); err == nil {
		final = uniquePath(final)
	}
	if err := os.Rename(filepath.Join(staging, name), final); err != nil {
		return staging, true, err
	}
	_ = os.Remove(staging)
	return final, dir, nil
}

func sameStem(name, stem string) bool {
	if name == stem {
		return true
	}
	inner, _ := splitName(name)
	return inner == stem
}

func (r *runner) progress() Progress {
	r.job.mu.Lock()
	defer r.job.mu.Unlock()
	return r.job.progress
}

func (r *runner) restoreProgress(p Progress) {
	r.job.mu.Lock()
	r.job.progress.FilesDone, r.job.progress.FilesTotal = p.FilesDone, p.FilesTotal
	r.job.progress.BytesDone, r.job.progress.BytesTotal = p.BytesDone, p.BytesTotal
	r.job.mu.Unlock()
}

func (r *runner) targetPath(archive string) string {
	target := filepath.Join(r.job.spec.dest, archiveStem(filepath.Base(archive)))
	if _, err := os.Lstat(target); err == nil {
		return uniquePath(target)
	}
	return target
}

func openStream(in *os.File, kind compression) (*bufio.Reader, func(), error) {
	switch kind {
	case compressGzip:
		reader, err := gzip.NewReader(in)
		if err != nil {
			return nil, nil, err
		}
		return bufio.NewReader(reader), func() { _ = reader.Close() }, nil
	case compressBzip2:
		return bufio.NewReader(bzip2.NewReader(in)), func() {}, nil
	case compressXz:
		reader, err := xz.NewReader(in)
		if err != nil {
			return nil, nil, err
		}
		return bufio.NewReader(reader), func() {}, nil
	case compressLzma:
		reader, err := lzma.NewReader(in)
		if err != nil {
			return nil, nil, err
		}
		return bufio.NewReader(reader), func() {}, nil
	case compressLzip:
		reader, err := lzip.NewReader(in)
		if err != nil {
			return nil, nil, err
		}
		return bufio.NewReader(reader), func() {}, nil
	case compressZstd:
		reader, err := zstd.NewReader(in)
		if err != nil {
			return nil, nil, err
		}
		return bufio.NewReader(reader), reader.Close, nil
	}
	return bufio.NewReader(in), func() {}, nil
}

func streamHoldsTar(stream *bufio.Reader) bool {
	head, _ := stream.Peek(tarMagicOffset + len(tarMagic))
	return looksLikeTar(head)
}

func (r *runner) unpackFile(archive string, stream io.Reader) (string, error) {
	base := filepath.Base(archive)
	name := archiveStem(base)
	if name == base {
		name = base + ".out"
	}
	target := filepath.Join(r.job.spec.dest, name)
	if _, err := os.Lstat(target); err == nil {
		target = uniquePath(target)
	}
	out, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return "", err
	}
	r.growTotals(1)
	if err := r.stream(target, stream, out); err != nil {
		_ = out.Close()
		_ = os.Remove(target)
		return "", err
	}
	if err := out.Close(); err != nil {
		_ = os.Remove(target)
		return "", err
	}
	r.advance(target, 1, 0)
	return target, nil
}

type picker struct {
	picked map[string]struct{}
	single bool
	hits   int
}

func newPicker(entries []string, single bool) *picker {
	p := &picker{single: single}
	if len(entries) == 0 {
		return p
	}
	p.picked = make(map[string]struct{}, len(entries))
	for _, entry := range entries {
		p.picked[filepath.Clean(entry)] = struct{}{}
	}
	return p
}

func (p *picker) place(raw string) (string, error) {
	if rootEntry(raw) {
		return "", nil
	}
	name, err := entryName(raw)
	if err != nil {
		return "", err
	}
	if p.picked == nil {
		return name, nil
	}
	for at := name; at != "."; at = filepath.Dir(at) {
		if _, ok := p.picked[at]; !ok {
			continue
		}
		p.hits++
		parent := filepath.Dir(at)
		if parent == "." {
			return name, nil
		}
		return name[len(parent)+1:], nil
	}
	return "", nil
}

func (p *picker) measure(members []*member, password string) (count int, bytes int64, err error) {
	for _, m := range members {
		name, err := p.place(m.name)
		if err != nil {
			return 0, 0, err
		}
		if name == "" || m.kind == memberOther {
			continue
		}
		if m.encrypted && password == "" {
			return 0, 0, errPasswordNeeded
		}
		count++
		bytes += m.size
	}
	return count, bytes, nil
}

func (p *picker) missed() error {
	if p.picked == nil || p.hits > 0 {
		return nil
	}
	return &files.Error{Code: files.CodeNotFound, Err: errors.New("not found in the archive")}
}

func (r *runner) unpackMembers(reader archiveReader, root *os.Root, pick *picker, password string) error {
	listed, sized := reader.(*listedReader)
	if sized {
		count, bytes, err := pick.measure(listed.members, password)
		if err != nil {
			return err
		}
		r.addTotals(count, bytes)
	}
	for {
		if r.job.closed() {
			return context.Canceled
		}
		m, err := reader.next()
		switch {
		case errors.Is(err, io.EOF):
			return pick.missed()
		case err != nil:
			return err
		}
		name, err := pick.place(m.name)
		if err != nil {
			return err
		}
		if name == "" {
			continue
		}
		if m.encrypted && password == "" {
			return errPasswordNeeded
		}
		done, err := r.unpackMember(root, m, name, pick)
		if err != nil {
			return err
		}
		if !done {
			r.skip()
			continue
		}
		if !sized {
			r.growTotals(1)
		}
		r.advance(filepath.Join(root.Name(), name), 1, 0)
		if pick.single {
			return nil
		}
	}
}

func (r *runner) unpackMember(root *os.Root, m *member, name string, pick *picker) (bool, error) {
	switch m.kind {
	case memberDir:
		return true, root.MkdirAll(name, dirPerm(m.perm))
	case memberSymlink:
		target, err := m.linkTarget()
		if err != nil {
			return false, err
		}
		return true, makeLink(root, name, target)
	case memberHardLink:
		source, err := pick.place(m.link)
		if err != nil || source == "" {
			return false, err
		}
		if err := root.MkdirAll(filepath.Dir(name), 0o755); err != nil {
			return false, err
		}
		return true, root.Link(source, name)
	case memberFile:
		body, err := m.open()
		if err != nil {
			return false, err
		}
		err = r.writeEntry(root, name, body, filePerm(m.perm))
		_ = body.Close()
		if err != nil {
			return false, err
		}
		_ = root.Chtimes(name, m.modified, m.modified)
		return true, nil
	}
	return false, nil
}

func (r *runner) addTotals(files int, bytes int64) {
	r.growTotals(files)
	r.job.mu.Lock()
	r.job.progress.BytesTotal += bytes
	r.job.mu.Unlock()
}

func makeLink(root *os.Root, name, target string) error {
	if err := root.MkdirAll(filepath.Dir(name), 0o755); err != nil {
		return err
	}
	return root.Symlink(target, name)
}

func (r *runner) writeEntry(root *os.Root, name string, body io.Reader, perm fs.FileMode) error {
	if err := root.MkdirAll(filepath.Dir(name), 0o755); err != nil {
		return err
	}
	out, err := root.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm)
	if err != nil {
		return err
	}
	if err := r.stream(filepath.Join(root.Name(), name), body, out); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}

func rootEntry(name string) bool {
	return filepath.Clean(filepath.FromSlash(name)) == "."
}

func entryName(name string) (string, error) {
	clean := filepath.Clean(filepath.FromSlash(name))
	if clean == "." || !filepath.IsLocal(clean) {
		return "", &archiveEntryError{name}
	}
	return clean, nil
}

type archiveEntryError struct{ name string }

func (e *archiveEntryError) Error() string {
	return "archive entry escapes the target folder: " + e.name
}

func dirPerm(mode fs.FileMode) fs.FileMode {
	perm := mode.Perm() | 0o700
	if perm == 0o700 {
		return 0o755
	}
	return perm
}

func filePerm(mode fs.FileMode) fs.FileMode {
	perm := mode.Perm() | 0o600
	if perm == 0o600 {
		return 0o644
	}
	return perm
}

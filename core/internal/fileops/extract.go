package fileops

import (
	"archive/tar"
	"archive/zip"
	"bufio"
	"compress/bzip2"
	"compress/gzip"
	"context"
	"errors"
	"hash"
	"hash/crc32"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/bodgit/sevenzip"
	"github.com/cavaliergopher/cpio"
	"github.com/klauspost/compress/zstd"
	"github.com/nwaples/rardecode/v2"
	"github.com/sorairolake/lzip-go"
	"github.com/ulikunitz/xz"
	"github.com/ulikunitz/xz/lzma"
)

const unixSymlinkAttribute = 0xA000

func (r *runner) extract() error {
	r.setTotals(0, 0, true)

	return eachSource(r, r.job.spec.sources, func(archive string) error {
		created, dir, err := r.unpack(archive)
		if err != nil {
			return err
		}
		r.remember(KindDelete, created, "")
		r.noteCreated(created, dir)
		return nil
	})
}

func (r *runner) unpack(archive string) (created string, dir bool, err error) {
	format, err := detectFormat(archive)
	if err != nil {
		return "", false, err
	}
	err = r.unlock(archive, func(password string) error {
		created, dir, err = r.unpackWith(archive, format, password)
		return err
	})
	return created, dir, err
}

func (r *runner) unpackWith(archive string, format archiveFormat, password string) (string, bool, error) {
	var in *os.File
	if err := r.retryLocal(archive, func() error {
		opened, err := os.Open(archive)
		in = opened
		return err
	}); err != nil {
		return "", false, err
	}
	defer in.Close()

	stream, closeStream, err := openStream(in, format.compression)
	if err != nil {
		return "", false, err
	}
	defer closeStream()

	kind := format.container
	if kind == containerNone && streamHoldsTar(stream) {
		kind = containerTar
	}
	if kind == containerNone {
		target, err := r.unpackFile(archive, stream)
		return target, false, err
	}

	target := r.targetPath(archive)
	if err := os.Mkdir(target, 0o755); err != nil {
		return "", false, err
	}
	root, err := os.OpenRoot(target)
	if err != nil {
		_ = os.RemoveAll(target)
		return "", false, err
	}
	before := r.progress()
	fail := func(holding string, err error) (string, bool, error) {
		_ = os.RemoveAll(holding)
		r.restoreProgress(before)
		return "", false, err
	}
	err = r.unpackInto(kind, in, stream, root, password)
	_ = root.Close()
	if err != nil {
		return fail(target, err)
	}
	created, dir, err := r.hoistSoleEntry(target, archiveStem(filepath.Base(archive)))
	if err != nil {
		return fail(created, err)
	}
	return created, dir, nil
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

func (r *runner) unpackInto(kind container, in *os.File, stream *bufio.Reader, root *os.Root, password string) error {
	switch kind {
	case containerZip:
		return r.unzip(in, root, password)
	case container7z:
		return r.un7z(in, root, password)
	case containerRar:
		return r.unrar(stream, root, password)
	case containerCpio:
		return r.uncpio(stream, root)
	default:
		return r.untar(stream, root)
	}
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

func (r *runner) unzip(in *os.File, root *os.Root, password string) error {
	info, err := in.Stat()
	if err != nil {
		return err
	}
	reader, err := zip.NewReader(in, info.Size())
	if err != nil {
		return err
	}
	var bytes int64
	for _, f := range reader.File {
		bytes += int64(f.UncompressedSize64)
		if f.Flags&zipFlagEncrypted != 0 && password == "" {
			return errPasswordNeeded
		}
	}
	r.addTotals(len(reader.File), bytes)

	for _, f := range reader.File {
		if r.job.closed() {
			return context.Canceled
		}
		open := f.Open
		if f.Flags&zipFlagEncrypted != 0 {
			open = func() (io.ReadCloser, error) { return openEncryptedZipEntry(f, password) }
		}
		if err := r.unzipEntry(root, f, open); err != nil {
			return err
		}
	}
	return nil
}

func (r *runner) unzipEntry(root *os.Root, f *zip.File, open func() (io.ReadCloser, error)) error {
	if rootEntry(f.Name) {
		return nil
	}
	name, err := entryName(f.Name)
	if err != nil {
		return err
	}
	mode := f.Mode()
	switch {
	case strings.HasSuffix(f.Name, "/") || mode.IsDir():
		if err := root.MkdirAll(name, dirPerm(mode)); err != nil {
			return err
		}
	case mode&fs.ModeSymlink != 0:
		if err := linkFromBody(root, name, open); err != nil {
			return err
		}
	default:
		body, err := open()
		if err != nil {
			return err
		}
		err = r.writeEntry(root, name, body, filePerm(mode))
		_ = body.Close()
		if err != nil {
			return err
		}
		_ = root.Chtimes(name, f.Modified, f.Modified)
	}
	r.advance(filepath.Join(root.Name(), name), 1, 0)
	return nil
}

func (r *runner) un7z(in *os.File, root *os.Root, password string) error {
	info, err := in.Stat()
	if err != nil {
		return err
	}
	reader, err := sevenzip.NewReaderWithPassword(in, info.Size(), password)
	if err != nil {
		return sevenZipPasswordError(err, password)
	}
	var bytes int64
	for _, f := range reader.File {
		bytes += int64(f.UncompressedSize)
	}
	r.addTotals(len(reader.File), bytes)

	for _, f := range reader.File {
		if r.job.closed() {
			return context.Canceled
		}
		if rootEntry(f.Name) {
			continue
		}
		name, err := entryName(f.Name)
		if err != nil {
			return err
		}
		open := func() (io.ReadCloser, error) {
			body, err := f.Open()
			if err != nil {
				return nil, sevenZipPasswordError(err, password)
			}
			return &sevenZipEntry{inner: body, password: password, crc: crc32.NewIEEE(), want: f.CRC32}, nil
		}
		mode := sevenZipMode(f)
		switch {
		case mode.IsDir():
			if err := root.MkdirAll(name, dirPerm(mode)); err != nil {
				return err
			}
		case mode&fs.ModeSymlink != 0:
			if err := linkFromBody(root, name, open); err != nil {
				return err
			}
		default:
			body, err := open()
			if err != nil {
				return err
			}
			err = r.writeEntry(root, name, body, filePerm(mode))
			_ = body.Close()
			if err != nil {
				return err
			}
			_ = root.Chtimes(name, f.Modified, f.Modified)
		}
		r.advance(filepath.Join(root.Name(), name), 1, 0)
	}
	return nil
}

// an empty password still derives a key, so a protected entry can decode to garbage only the CRC catches
func sevenZipPasswordError(err error, password string) error {
	var readErr *sevenzip.ReadError
	encrypted := errors.As(err, &readErr) && readErr.Encrypted
	if !encrypted && !errors.Is(err, errDamagedEntry) {
		return err
	}
	if password == "" {
		return errPasswordNeeded
	}
	return &badPasswordError{err}
}

// sevenZipEntry checks the entry CRC itself; the library does not.
type sevenZipEntry struct {
	inner    io.ReadCloser
	password string
	crc      hash.Hash32
	want     uint32
}

func (e *sevenZipEntry) Read(p []byte) (int, error) {
	n, err := e.inner.Read(p)
	e.crc.Write(p[:n])
	switch {
	case err == nil:
		return n, nil
	case errors.Is(err, io.EOF):
		if e.want != 0 && e.crc.Sum32() != e.want {
			return n, e.fail(errDamagedEntry)
		}
		return n, io.EOF
	default:
		return n, e.fail(err)
	}
}

func (e *sevenZipEntry) fail(err error) error {
	return sevenZipPasswordError(err, e.password)
}

func (e *sevenZipEntry) Close() error { return e.inner.Close() }

// the unix mode sits in the high attribute bits; the library maps everything but links
func sevenZipMode(f *sevenzip.File) fs.FileMode {
	mode := f.Mode()
	if f.Attributes&0x8000 != 0 && (f.Attributes>>16)&0xF000 == unixSymlinkAttribute {
		mode |= fs.ModeSymlink
	}
	return mode
}

func (r *runner) unrar(stream io.Reader, root *os.Root, password string) error {
	reader, err := rardecode.NewReader(stream, rardecode.Password(password))
	if err != nil {
		return rarPasswordError(err, password)
	}
	for {
		if r.job.closed() {
			return context.Canceled
		}
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return rarPasswordError(err, password)
		}
		if (header.Encrypted || header.HeaderEncrypted) && password == "" {
			return errPasswordNeeded
		}
		if rootEntry(header.Name) {
			continue
		}
		name, err := entryName(header.Name)
		if err != nil {
			return err
		}
		body := io.Reader(reader)
		if header.Encrypted {
			body = &rarEntry{inner: reader}
		}
		mode := header.Mode()
		switch {
		case header.IsDir:
			if err := root.MkdirAll(name, dirPerm(mode)); err != nil {
				return err
			}
		case header.LinkTarget != "":
			if err := makeLink(root, name, header.LinkTarget); err != nil {
				return err
			}
		case mode&fs.ModeSymlink != 0:
			target, err := io.ReadAll(body)
			if err != nil {
				return err
			}
			if err := makeLink(root, name, string(target)); err != nil {
				return err
			}
		default:
			r.growTotals(1)
			if err := r.writeEntry(root, name, body, filePerm(mode)); err != nil {
				return err
			}
			_ = root.Chtimes(name, header.ModificationTime, header.ModificationTime)
		}
		r.advance(filepath.Join(root.Name(), name), 1, 0)
	}
}

func rarPasswordError(err error, password string) error {
	switch {
	case errors.Is(err, rardecode.ErrArchiveEncrypted), errors.Is(err, rardecode.ErrArchivedFileEncrypted):
		if password == "" {
			return errPasswordNeeded
		}
		return &badPasswordError{err}
	case errors.Is(err, rardecode.ErrBadPassword):
		return &badPasswordError{err}
	}
	return err
}

// RAR3 has no password check value, so a wrong key shows up as corrupt data
type rarEntry struct{ inner io.Reader }

func (e *rarEntry) Read(p []byte) (int, error) {
	n, err := e.inner.Read(p)
	if err == nil || errors.Is(err, io.EOF) {
		return n, err
	}
	return n, &badPasswordError{err}
}

func (r *runner) uncpio(stream io.Reader, root *os.Root) error {
	reader := cpio.NewReader(stream)
	for {
		if r.job.closed() {
			return context.Canceled
		}
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		if rootEntry(header.Name) {
			continue
		}
		name, err := entryName(header.Name)
		if err != nil {
			return err
		}
		perm := fs.FileMode(header.Mode.Perm())
		switch header.Mode & cpio.ModeType {
		case cpio.TypeDir:
			if err := root.MkdirAll(name, dirPerm(perm)); err != nil {
				return err
			}
		case cpio.TypeSymlink:
			target := header.Linkname
			if target == "" {
				body, err := io.ReadAll(reader)
				if err != nil {
					return err
				}
				target = string(body)
			}
			if err := makeLink(root, name, target); err != nil {
				return err
			}
		case cpio.TypeReg:
			r.growTotals(1)
			if err := r.writeEntry(root, name, reader, filePerm(perm)); err != nil {
				return err
			}
			_ = root.Chtimes(name, header.ModTime, header.ModTime)
		default:
			r.skip()
			continue
		}
		r.advance(filepath.Join(root.Name(), name), 1, 0)
	}
}

func (r *runner) untar(in io.Reader, root *os.Root) error {
	reader := tar.NewReader(in)
	for {
		if r.job.closed() {
			return context.Canceled
		}
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		if rootEntry(header.Name) {
			continue
		}
		name, err := entryName(header.Name)
		if err != nil {
			return err
		}
		mode := header.FileInfo().Mode()
		switch header.Typeflag {
		case tar.TypeDir:
			if err := root.MkdirAll(name, dirPerm(mode)); err != nil {
				return err
			}
		case tar.TypeSymlink:
			if err := makeLink(root, name, header.Linkname); err != nil {
				return err
			}
		case tar.TypeLink:
			old, err := entryName(header.Linkname)
			if err != nil {
				return err
			}
			if err := root.MkdirAll(filepath.Dir(name), 0o755); err != nil {
				return err
			}
			if err := root.Link(old, name); err != nil {
				return err
			}
		case tar.TypeReg:
			r.growTotals(1)
			if err := r.writeEntry(root, name, reader, filePerm(mode)); err != nil {
				return err
			}
			_ = root.Chtimes(name, header.ModTime, header.ModTime)
		default:
			r.skip()
			continue
		}
		r.advance(filepath.Join(root.Name(), name), 1, 0)
	}
}

func (r *runner) addTotals(files int, bytes int64) {
	r.growTotals(files)
	r.job.mu.Lock()
	r.job.progress.BytesTotal += bytes
	r.job.mu.Unlock()
}

func linkFromBody(root *os.Root, name string, open func() (io.ReadCloser, error)) error {
	body, err := open()
	if err != nil {
		return err
	}
	target, err := io.ReadAll(body)
	_ = body.Close()
	if err != nil {
		return err
	}
	return makeLink(root, name, string(target))
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

package fileops

import (
	"archive/tar"
	"archive/zip"
	"errors"
	"hash"
	"hash/crc32"
	"io"
	"io/fs"
	"os"
	"strings"
	"time"

	"github.com/bodgit/sevenzip"
	"github.com/cavaliergopher/cpio"
	"github.com/nwaples/rardecode/v2"
)

const unixSymlinkAttribute = 0xA000

type memberKind int

const (
	memberFile memberKind = iota
	memberDir
	memberSymlink
	memberHardLink
	memberOther
)

type member struct {
	name      string
	kind      memberKind
	perm      fs.FileMode
	size      int64
	modified  time.Time
	link      string
	encrypted bool
	open      func() (io.ReadCloser, error)
}

func (m *member) linkTarget() (string, error) {
	if m.link != "" || m.open == nil {
		return m.link, nil
	}
	body, err := m.open()
	if err != nil {
		return "", err
	}
	target, err := io.ReadAll(body)
	_ = body.Close()
	return string(target), err
}

type archiveReader interface {
	next() (*member, error)
}

type listedReader struct {
	members []*member
	at      int
}

func (l *listedReader) next() (*member, error) {
	if l.at >= len(l.members) {
		return nil, io.EOF
	}
	l.at++
	return l.members[l.at-1], nil
}

func newArchiveReader(kind container, in *os.File, stream io.Reader, password string) (archiveReader, error) {
	switch kind {
	case containerZip:
		return readZip(in, password)
	case container7z:
		return read7z(in, password)
	case containerRar:
		reader, err := rardecode.NewReader(stream, rardecode.Password(password))
		if err != nil {
			return nil, rarPasswordError(err, password)
		}
		return &rarStream{inner: reader, password: password}, nil
	case containerCpio:
		return &cpioStream{inner: cpio.NewReader(stream)}, nil
	case containerTar:
		return &tarStream{inner: tar.NewReader(stream)}, nil
	}
	return nil, errUnsupportedArchive
}

func kindOf(mode fs.FileMode, dir bool) memberKind {
	switch {
	case dir || mode.IsDir():
		return memberDir
	case mode&fs.ModeSymlink != 0:
		return memberSymlink
	case mode.IsRegular():
		return memberFile
	}
	return memberOther
}

func readZip(in *os.File, password string) (*listedReader, error) {
	info, err := in.Stat()
	if err != nil {
		return nil, err
	}
	reader, err := zip.NewReader(in, info.Size())
	if err != nil {
		return nil, err
	}
	members := make([]*member, 0, len(reader.File))
	for _, f := range reader.File {
		mode := f.Mode()
		members = append(members, &member{
			name:      f.Name,
			kind:      kindOf(mode, strings.HasSuffix(f.Name, "/")),
			perm:      mode.Perm(),
			size:      int64(f.UncompressedSize64),
			modified:  f.Modified,
			encrypted: f.Flags&zipFlagEncrypted != 0,
			open:      func() (io.ReadCloser, error) { return openZipEntry(f, password) },
		})
	}
	return &listedReader{members: members}, nil
}

func read7z(in *os.File, password string) (*listedReader, error) {
	info, err := in.Stat()
	if err != nil {
		return nil, err
	}
	reader, err := sevenzip.NewReaderWithPassword(in, info.Size(), password)
	if err != nil {
		return nil, sevenZipPasswordError(err, password)
	}
	members := make([]*member, 0, len(reader.File))
	for _, f := range reader.File {
		mode := sevenZipMode(f)
		members = append(members, &member{
			name:     f.Name,
			kind:     kindOf(mode, false),
			perm:     mode.Perm(),
			size:     int64(f.UncompressedSize),
			modified: f.Modified,
			open: func() (io.ReadCloser, error) {
				body, err := f.Open()
				if err != nil {
					return nil, sevenZipPasswordError(err, password)
				}
				return &sevenZipEntry{inner: body, password: password, crc: crc32.NewIEEE(), want: f.CRC32}, nil
			},
		})
	}
	return &listedReader{members: members}, nil
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

type tarStream struct{ inner *tar.Reader }

func (t *tarStream) next() (*member, error) {
	header, err := t.inner.Next()
	if err != nil {
		return nil, err
	}
	m := &member{
		name:     header.Name,
		kind:     memberOther,
		perm:     header.FileInfo().Mode().Perm(),
		size:     header.Size,
		modified: header.ModTime,
		link:     header.Linkname,
	}
	switch header.Typeflag {
	case tar.TypeDir:
		m.kind = memberDir
	case tar.TypeSymlink:
		m.kind = memberSymlink
	case tar.TypeLink:
		m.kind = memberHardLink
	case tar.TypeReg:
		m.kind = memberFile
		m.open = func() (io.ReadCloser, error) { return io.NopCloser(t.inner), nil }
	}
	return m, nil
}

type rarStream struct {
	inner    *rardecode.Reader
	password string
}

func (s *rarStream) next() (*member, error) {
	header, err := s.inner.Next()
	if err != nil {
		return nil, rarPasswordError(err, s.password)
	}
	mode := header.Mode()
	m := &member{
		name:      header.Name,
		kind:      kindOf(mode, header.IsDir),
		perm:      mode.Perm(),
		size:      header.UnPackedSize,
		modified:  header.ModificationTime,
		link:      header.LinkTarget,
		encrypted: header.Encrypted || header.HeaderEncrypted,
	}
	if header.LinkTarget != "" && m.kind != memberDir {
		m.kind = memberSymlink
	}
	if m.kind == memberDir {
		return m, nil
	}
	m.open = func() (io.ReadCloser, error) {
		if header.Encrypted {
			return io.NopCloser(&rarEntry{inner: s.inner}), nil
		}
		return io.NopCloser(s.inner), nil
	}
	return m, nil
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

type cpioStream struct{ inner *cpio.Reader }

func (c *cpioStream) next() (*member, error) {
	header, err := c.inner.Next()
	if err != nil {
		return nil, err
	}
	m := &member{
		name:     header.Name,
		kind:     memberOther,
		perm:     fs.FileMode(header.Mode.Perm()),
		size:     header.Size,
		modified: header.ModTime,
		link:     header.Linkname,
		open:     func() (io.ReadCloser, error) { return io.NopCloser(c.inner), nil },
	}
	switch header.Mode & cpio.ModeType {
	case cpio.TypeDir:
		m.kind = memberDir
	case cpio.TypeSymlink:
		m.kind = memberSymlink
	case cpio.TypeReg:
		m.kind = memberFile
	}
	return m, nil
}

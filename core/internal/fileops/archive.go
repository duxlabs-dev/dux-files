package fileops

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/klauspost/compress/zstd"
	"github.com/ulikunitz/xz"
)

type archiveWriter interface {
	dir(name string, info fs.FileInfo) error
	symlink(name, target string, info fs.FileInfo) error
	file(name string, info fs.FileInfo) (io.Writer, error)
	Close() error
}

func (r *runner) compress() error {
	sources := r.job.spec.sources
	count, bytes, complete := r.scan(sources)
	r.setTotals(count, bytes, !complete)

	target := filepath.Join(r.job.spec.dest, r.job.spec.name+"."+r.job.spec.format)
	if _, err := os.Lstat(target); err == nil {
		target = uniquePath(target)
	}
	tmp := r.tempPath(target)

	var out *os.File
	if err := r.retryLocal(tmp, func() error {
		created, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		out = created
		return err
	}); err != nil {
		return skipToNil(err)
	}
	discard := func() {
		_ = out.Close()
		_ = os.Remove(tmp)
	}

	writer, err := newArchiveWriter(out, r.job.spec.format, r.job.spec.password)
	if err != nil {
		discard()
		return err
	}
	for _, src := range sources {
		if err := r.pack(writer, src); err != nil {
			discard()
			return skipToNil(err)
		}
	}
	if err := writer.Close(); err != nil {
		discard()
		return err
	}
	if err := out.Close(); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if err := renameNoReplace(tmp, target); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	r.remember(KindDelete, target, "")
	return nil
}

func newArchiveWriter(out io.Writer, format, password string) (archiveWriter, error) {
	switch format {
	case FormatZip:
		return newZipWriter(out, password), nil
	case FormatTarGz:
		return newTarWriter(gzip.NewWriter(out)), nil
	case FormatTarXz:
		compressor, err := xz.NewWriter(out)
		if err != nil {
			return nil, err
		}
		return newTarWriter(compressor), nil
	case FormatTarZst:
		compressor, err := zstd.NewWriter(out)
		if err != nil {
			return nil, err
		}
		return newTarWriter(compressor), nil
	}
	return nil, errUnsupportedArchive
}

func newTarWriter(compressor io.WriteCloser) *tarWriter {
	return &tarWriter{inner: tar.NewWriter(compressor), compressor: compressor}
}

func (r *runner) pack(w archiveWriter, src string) error {
	base := filepath.Dir(src)
	return filepath.WalkDir(src, func(path string, entry fs.DirEntry, walkErr error) error {
		if r.job.closed() {
			return context.Canceled
		}
		if walkErr != nil {
			if entry == nil {
				return walkErr
			}
			r.warn(path, walkErr)
			r.skip()
			if entry.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		name, _ := filepath.Rel(base, path)
		name = filepath.ToSlash(name)
		info, err := entry.Info()
		if err != nil {
			return err
		}

		switch {
		case info.IsDir():
			if err := w.dir(name+"/", info); err != nil {
				return err
			}
			r.advance(path, 1, 0)
			return nil
		case info.Mode()&fs.ModeSymlink != 0:
			target, err := os.Readlink(path)
			if err != nil {
				return err
			}
			if err := w.symlink(name, target, info); err != nil {
				return err
			}
			r.advance(path, 1, 0)
			return nil
		case !info.Mode().IsRegular():
			r.skip()
			return nil
		}

		var in *os.File
		if err := r.retryLocal(path, func() error {
			opened, err := os.Open(path)
			in = opened
			return err
		}); err != nil {
			if errors.Is(err, errSkipped) {
				return nil
			}
			return err
		}
		defer in.Close()

		dst, err := w.file(name, info)
		if err != nil {
			return err
		}
		if err := r.stream(path, in, dst); err != nil {
			return err
		}
		r.advance(path, 1, 0)
		return nil
	})
}

func (r *runner) stream(path string, in io.Reader, out io.Writer) error {
	chunk := borrowChunk()
	defer chunks.Put(chunk)
	buf := *chunk
	for {
		if r.job.closed() {
			return context.Canceled
		}
		n, readErr := in.Read(buf)
		if n > 0 {
			if _, err := out.Write(buf[:n]); err != nil {
				return err
			}
			r.advance(path, 0, int64(n))
		}
		if readErr == io.EOF {
			return nil
		}
		if readErr != nil {
			return readErr
		}
	}
}

type zipWriter struct {
	inner    *zip.Writer
	password string
}

func newZipWriter(out io.Writer, password string) *zipWriter {
	inner := zip.NewWriter(out)
	if password != "" {
		inner.RegisterCompressor(zipMethodAES, aesCompressor(password))
	}
	return &zipWriter{inner: inner, password: password}
}

func (z *zipWriter) header(name string, info fs.FileInfo) *zip.FileHeader {
	header := &zip.FileHeader{Name: name, Method: zip.Deflate, Modified: info.ModTime()}
	header.SetMode(info.Mode())
	return header
}

func (z *zipWriter) protected(header *zip.FileHeader) *zip.FileHeader {
	if z.password != "" {
		encryptedZipHeader(header)
	}
	return header
}

func (z *zipWriter) dir(name string, info fs.FileInfo) error {
	header := z.header(name, info)
	header.Method = zip.Store
	_, err := z.inner.CreateHeader(header)
	return err
}

func (z *zipWriter) symlink(name, target string, info fs.FileInfo) error {
	header := z.header(name, info)
	header.Method = zip.Store
	w, err := z.inner.CreateHeader(z.protected(header))
	if err != nil {
		return err
	}
	_, err = io.WriteString(w, target)
	return err
}

func (z *zipWriter) file(name string, info fs.FileInfo) (io.Writer, error) {
	return z.inner.CreateHeader(z.protected(z.header(name, info)))
}

func (z *zipWriter) Close() error { return z.inner.Close() }

type tarWriter struct {
	inner      *tar.Writer
	compressor io.WriteCloser
}

func (t *tarWriter) dir(name string, info fs.FileInfo) error {
	header, err := tar.FileInfoHeader(info, "")
	if err != nil {
		return err
	}
	header.Name = name
	return t.inner.WriteHeader(header)
}

func (t *tarWriter) symlink(name, target string, info fs.FileInfo) error {
	header, err := tar.FileInfoHeader(info, target)
	if err != nil {
		return err
	}
	header.Name = name
	return t.inner.WriteHeader(header)
}

func (t *tarWriter) file(name string, info fs.FileInfo) (io.Writer, error) {
	header, err := tar.FileInfoHeader(info, "")
	if err != nil {
		return nil, err
	}
	header.Name = name
	if err := t.inner.WriteHeader(header); err != nil {
		return nil, err
	}
	return t.inner, nil
}

func (t *tarWriter) Close() error {
	if err := t.inner.Close(); err != nil {
		return err
	}
	return t.compressor.Close()
}

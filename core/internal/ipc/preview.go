package ipc

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"os"
	"path/filepath"
	"syscall"
	"unicode/utf8"

	"github.com/duxlabs-dev/dux-files/core/internal/wire"

	"github.com/duxlabs-dev/dux-go/ipc/params"
	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/tiff"
	_ "golang.org/x/image/webp"
)

const (
	previewDefaultBytes = 16 * 1024
	previewMaxBytes     = 256 * 1024
	imageHeaderBytes    = 4 << 20
)

type previewResult struct {
	Text      string `json:"text"`
	Binary    bool   `json:"binary"`
	Truncated bool   `json:"truncated"`
	Size      int64  `json:"size"`
	Lines     int    `json:"lines"`
}

type imageSizeResult struct {
	Width  int    `json:"width"`
	Height int    `json:"height"`
	Format string `json:"format"`
}

func HandlePreview(_ context.Context, w *ConnWriter, req Request) {
	path := params.StringOpt(req.Params, "path", "")
	if !filepath.IsAbs(path) {
		wire.RespondError(w, req.ID, wire.Invalid("path is not an absolute path"))
		return
	}
	limit := params.IntOpt(req.Params, "maxBytes", 0)
	switch {
	case limit <= 0:
		limit = previewDefaultBytes
	case limit > previewMaxBytes:
		limit = previewMaxBytes
	}
	result, err := readPreview(path, limit)
	if err != nil {
		w.RespondError(req.ID, err.Error())
		return
	}
	w.Respond(req.ID, result)
}

func HandleImageSize(_ context.Context, w *ConnWriter, req Request) {
	path := params.StringOpt(req.Params, "path", "")
	if !filepath.IsAbs(path) {
		wire.RespondError(w, req.ID, wire.Invalid("path is not an absolute path"))
		return
	}
	result, err := readImageSize(path)
	if err != nil {
		w.RespondError(req.ID, err.Error())
		return
	}
	w.Respond(req.ID, result)
}

func openRegular(path string) (*os.File, os.FileInfo, error) {
	file, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, nil, err
	}
	info, err := file.Stat()
	switch {
	case err != nil:
	case info.IsDir():
		err = errors.New("path is a directory")
	case !info.Mode().IsRegular():
		err = errors.New("path is not a regular file")
	default:
		return file, info, nil
	}
	file.Close()
	return nil, nil, err
}

func readImageSize(path string) (imageSizeResult, error) {
	file, _, err := openRegular(path)
	if err != nil {
		return imageSizeResult{}, err
	}
	defer file.Close()

	config, format, err := image.DecodeConfig(bufio.NewReader(io.LimitReader(file, imageHeaderBytes)))
	if err != nil {
		return imageSizeResult{}, err
	}
	return imageSizeResult{Width: config.Width, Height: config.Height, Format: format}, nil
}

func readPreview(path string, limit int) (previewResult, error) {
	file, info, err := openRegular(path)
	if err != nil {
		return previewResult{}, err
	}
	defer file.Close()

	buf := make([]byte, limit+1)
	n, err := io.ReadFull(file, buf)
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
		return previewResult{}, err
	}
	data := buf[:n]
	result := previewResult{Size: info.Size(), Truncated: n > limit}
	if result.Truncated {
		data = data[:limit]
	}
	if bytes.IndexByte(data, 0) >= 0 {
		result.Binary = true
		return result, nil
	}
	if result.Truncated {
		data = trimPartialRune(data)
	}
	if !utf8.Valid(data) {
		result.Binary = true
		return result, nil
	}
	result.Text = string(data)
	result.Lines = bytes.Count(data, []byte{'\n'})
	if len(data) > 0 && data[len(data)-1] != '\n' {
		result.Lines++
	}
	return result, nil
}

func trimPartialRune(data []byte) []byte {
	for cut := 0; cut < utf8.UTFMax && cut < len(data); cut++ {
		if utf8.Valid(data[len(data)-1-cut:]) {
			return data
		}
		if utf8.RuneStart(data[len(data)-1-cut]) {
			return data[:len(data)-1-cut]
		}
	}
	return data
}

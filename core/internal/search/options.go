package search

import (
	"errors"
	"path/filepath"
	"strings"

	"github.com/duxlabs-dev/dux-go/files"
)

const (
	ScopeFolder     = "folder"
	ScopeHome       = "home"
	ScopeEverywhere = "everywhere"
)

type Options struct {
	Scope           string
	IncludeHidden   bool
	IncludeIgnored  bool
	CrossDevice     bool
	Fuzzy           bool
	Contents        bool
	Regex           bool
	Direct          bool
	Types           []string
	ModifiedAfterMs int64
	MinSize         int64
	MaxSize         int64
	Limit           int
}

var knownTypes = map[string]bool{
	"documents": true,
	"images":    true,
	"video":     true,
	"audio":     true,
	"archives":  true,
	"folders":   true,
}

func validate(root, query string, opts Options) error {
	switch {
	case !filepath.IsAbs(root):
		return &files.Error{Code: files.CodeInvalid, Path: root, Err: errors.New("root must be absolute")}
	case opts.Scope != "" && opts.Scope != ScopeFolder && opts.Scope != ScopeHome && opts.Scope != ScopeEverywhere:
		return &files.Error{Code: files.CodeInvalid, Err: errors.New("unknown scope: " + opts.Scope)}
	case strings.TrimSpace(query) == "" && !opts.filtered():
		return &files.Error{Code: files.CodeInvalid, Err: errors.New("search needs a query or a filter")}
	case opts.MinSize > 0 && opts.MaxSize > 0 && opts.MaxSize < opts.MinSize:
		return &files.Error{Code: files.CodeInvalid, Err: errors.New("maxSize is below minSize")}
	}
	for _, kind := range opts.Types {
		if !knownTypes[kind] {
			return &files.Error{Code: files.CodeInvalid, Err: errors.New("unknown type filter: " + kind)}
		}
	}
	return nil
}

func (o Options) filtered() bool {
	return len(o.Types) > 0 || o.ModifiedAfterMs > 0 || o.MinSize > 0 || o.MaxSize > 0
}

func (o Options) limit() int {
	if o.Limit <= 0 || o.Limit > MaxResults {
		return MaxResults
	}
	return o.Limit
}

func (o Options) accepts(e files.Entry) bool {
	if o.ModifiedAfterMs > 0 && e.MtimeMs < o.ModifiedAfterMs {
		return false
	}
	if (o.MinSize > 0 || o.MaxSize > 0) && e.IsDir {
		return false
	}
	if o.MinSize > 0 && e.Size < o.MinSize {
		return false
	}
	if o.MaxSize > 0 && e.Size > o.MaxSize {
		return false
	}
	if len(o.Types) == 0 {
		return true
	}
	for _, kind := range o.Types {
		if isType(e, kind) {
			return true
		}
	}
	return false
}

func isType(e files.Entry, kind string) bool {
	if kind == "folders" {
		return e.IsDir
	}
	if e.IsDir {
		return false
	}
	media, sub, _ := strings.Cut(e.Mime, "/")
	switch kind {
	case "images":
		return media == "image"
	case "video":
		return media == "video"
	case "audio":
		return media == "audio"
	case "archives":
		return archiveMimes[e.Mime] || strings.HasSuffix(sub, "-compressed") || strings.HasSuffix(sub, "-compressed-tar")
	case "documents":
		return media == "text" || documentMimes[e.Mime] || strings.HasPrefix(sub, "vnd.oasis.opendocument") || strings.HasPrefix(sub, "vnd.openxmlformats-officedocument") || strings.HasPrefix(sub, "vnd.ms-")
	}
	return false
}

var archiveMimes = map[string]bool{
	"application/zip":             true,
	"application/x-tar":           true,
	"application/gzip":            true,
	"application/x-gzip":          true,
	"application/x-xz":            true,
	"application/x-bzip2":         true,
	"application/zstd":            true,
	"application/x-rar":           true,
	"application/vnd.rar":         true,
	"application/x-cpio":          true,
	"application/x-lzip":          true,
	"application/x-archive":       true,
	"application/x-iso9660-image": true,
}

var documentMimes = map[string]bool{
	"application/pdf":      true,
	"application/rtf":      true,
	"application/msword":   true,
	"application/epub+zip": true,
	"application/json":     true,
	"application/xml":      true,
	"application/toml":     true,
	"application/x-yaml":   true,
}

package clipboard

import (
	"strings"

	"github.com/duxlabs-dev/dux-files/core/internal/fileuri"
)

// Only the GNOME flavour carries cut vs copy (nautilus src/nautilus-clipboard.c); KDE uses a separate one-byte flavour (kio src/widgets/paste.cpp).
const (
	MimeGnomeCopiedFiles = "x-special/gnome-copied-files"
	MimeUriList          = "text/uri-list"
	MimeKdeCutSelection  = "application/x-kde-cutselection"
	MimeTextUTF8         = "text/plain;charset=utf-8"
)

func GnomePayload(paths []string, cut bool) []byte {
	action := "copy"
	if cut {
		action = "cut"
	}
	out := strings.Builder{}
	out.WriteString(action)
	out.WriteByte('\n')
	for _, path := range paths {
		out.WriteString(FileURI(path))
		out.WriteByte('\n')
	}
	return []byte(out.String())
}

// RFC 2483 wants CRLF between URIs.
func UriList(paths []string) []byte {
	out := strings.Builder{}
	for _, path := range paths {
		out.WriteString(FileURI(path))
		out.WriteString("\r\n")
	}
	return []byte(out.String())
}

func PlainText(paths []string) []byte {
	return []byte(strings.Join(paths, "\n"))
}

func FileURI(path string) string {
	return fileuri.Build(path)
}

// Every flavour but GNOME reads as a copy.
func Parse(data []byte, mime string) ([]string, bool) {
	switch {
	case strings.HasPrefix(mime, MimeGnomeCopiedFiles):
		return parseGnome(data)
	case strings.HasPrefix(mime, MimeUriList):
		return parseLines(data), false
	default:
		return parseLines(data), false
	}
}

func parseGnome(data []byte) ([]string, bool) {
	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	if len(lines) == 0 {
		return nil, false
	}
	action := strings.TrimSpace(lines[0])
	if action != "copy" && action != "cut" {
		return parseLines(data), false
	}
	return collect(lines[1:]), action == "cut"
}

func parseLines(data []byte) []string {
	return collect(strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n"))
}

func collect(lines []string) []string {
	paths := make([]string, 0, len(lines))
	for _, line := range lines {
		path := toPath(strings.TrimSpace(line))
		if path == "" {
			continue
		}
		paths = append(paths, path)
	}
	if len(paths) == 0 {
		return nil
	}
	return paths
}

// A remote URI or a # comment line must never become an operand.
func toPath(line string) string {
	switch {
	case line == "" || strings.HasPrefix(line, "#"):
		return ""
	case strings.HasPrefix(line, "/"):
		return line
	case !strings.HasPrefix(line, "file:"):
		return ""
	}
	path, _ := fileuri.Parse(line)
	return path
}

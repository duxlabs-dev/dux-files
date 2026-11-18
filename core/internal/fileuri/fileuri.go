package fileuri

import (
	"net/url"
	"path/filepath"
	"strings"
)

func Build(path string) string {
	return (&url.URL{Scheme: "file", Path: path}).String()
}

func Parse(uri string) (string, bool) {
	parsed, err := url.Parse(uri)
	if err != nil || !strings.EqualFold(parsed.Scheme, "file") || !LocalHost(parsed.Host) || !strings.HasPrefix(parsed.Path, "/") {
		return "", false
	}
	return filepath.Clean(parsed.Path), true
}

func LocalHost(host string) bool {
	return host == "" || host == "localhost"
}

package fileuri

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBuildEscapes(t *testing.T) {
	assert.Equal(t, "file:///home/me/a%20b%23c", Build("/home/me/a b#c"))
}

func TestParseAcceptsLocalFileURIsOnly(t *testing.T) {
	path, ok := Parse("file:///home/me/a%20b/")
	assert.True(t, ok)
	assert.Equal(t, "/home/me/a b", path)
	path, ok = Parse("FILE://localhost/x")
	assert.True(t, ok)
	assert.Equal(t, "/x", path)
	for _, bad := range []string{"file://nas/share", "http://x/y", "file:relative", "/plain"} {
		_, ok := Parse(bad)
		assert.False(t, ok, bad)
	}
}

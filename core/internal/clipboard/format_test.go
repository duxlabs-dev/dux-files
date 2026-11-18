package clipboard

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGnomePayloadMatchesNautilus(t *testing.T) {
	assert.Equal(t, "copy\nfile:///home/me/a.txt\nfile:///home/me/b%20c.txt\n",
		string(GnomePayload([]string{"/home/me/a.txt", "/home/me/b c.txt"}, false)))
	assert.Equal(t, "cut\nfile:///home/me/a.txt\n",
		string(GnomePayload([]string{"/home/me/a.txt"}, true)))
}

func TestUriListIsCrlfSeparated(t *testing.T) {
	assert.Equal(t, "file:///a\r\nfile:///b\r\n", string(UriList([]string{"/a", "/b"})))
}

func TestParseRoundTripsOurOwnPayload(t *testing.T) {
	paths := []string{"/home/me/a.txt", "/home/me/b c.txt"}
	for _, cut := range []bool{false, true} {
		got, gotCut := Parse(GnomePayload(paths, cut), MimeGnomeCopiedFiles)
		assert.Equal(t, paths, got)
		assert.Equal(t, cut, gotCut)
	}
}

func TestParseUriList(t *testing.T) {
	got, cut := Parse([]byte("#comment\r\nfile:///a\r\nfile:///b%20c\r\n"), MimeUriList)
	assert.Equal(t, []string{"/a", "/b c"}, got)
	assert.False(t, cut)
}

func TestParseRejectsRemoteAndRelativeEntries(t *testing.T) {
	got, _ := Parse([]byte("file://otherhost/a\r\nsftp://host/b\r\nrelative/c\r\nfile:///d\r\n"), MimeUriList)
	assert.Equal(t, []string{"/d"}, got)
}

func TestParsePlainTextAcceptsBarePaths(t *testing.T) {
	got, cut := Parse([]byte("/home/me/a\n/home/me/b\n"), MimeTextUTF8)
	assert.Equal(t, []string{"/home/me/a", "/home/me/b"}, got)
	assert.False(t, cut)
}

func TestParseGnomeWithUnknownActionFallsBackToLines(t *testing.T) {
	got, cut := Parse([]byte("link\nfile:///a\n"), MimeGnomeCopiedFiles)
	assert.Equal(t, []string{"/a"}, got)
	assert.False(t, cut)
}

func TestParseEmptySelection(t *testing.T) {
	got, cut := Parse(nil, MimeUriList)
	require.Nil(t, got)
	assert.False(t, cut)
}

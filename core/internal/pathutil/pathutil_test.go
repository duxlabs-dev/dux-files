package pathutil

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWithinAndUnder(t *testing.T) {
	assert.True(t, Within("/a/b", "/a"))
	assert.True(t, Within("/a", "/a"))
	assert.True(t, Within("/a", "/"))
	assert.False(t, Within("/ab", "/a"))
	assert.False(t, Under("/a", "/a"))
	assert.True(t, Under("/a/b", "/a/"))
	assert.True(t, Under("/x", "/"))
}

func TestValidName(t *testing.T) {
	assert.True(t, ValidName("notes.txt"))
	for _, bad := range []string{"", ".", "..", "a/b", "a\x00", string(make([]byte, 256))} {
		assert.False(t, ValidName(bad), "%q", bad)
	}
}

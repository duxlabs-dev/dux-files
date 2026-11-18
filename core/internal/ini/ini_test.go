package ini

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLinesWalksGroupsAndKeys(t *testing.T) {
	data := []byte("# comment\nloose=1\n[One]\na = x\nnot a pair\n\n[Two]\nb=y=z\n")
	var got []Line
	for line := range Lines(data) {
		got = append(got, line)
	}
	assert.Equal(t, []Line{
		{Key: "loose", Value: "1"},
		{Group: "One", Header: true},
		{Group: "One", Key: "a", Value: "x"},
		{Group: "Two", Header: true},
		{Group: "Two", Key: "b", Value: "y=z"},
	}, got)
}

func TestHeaderAndKey(t *testing.T) {
	name, ok := Header("  [Desktop Entry] ")
	assert.True(t, ok)
	assert.Equal(t, "Desktop Entry", name)
	_, ok = Header("[x")
	assert.False(t, ok)
	assert.Equal(t, "Exec", Key(" Exec = true"))
	assert.Equal(t, "", Key("# Exec=true"))
	assert.Equal(t, "", Key("plain"))
}

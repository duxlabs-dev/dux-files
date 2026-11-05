package testfs

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func Write(t *testing.T, path, body string) string {
	t.Helper()
	return WriteBytes(t, path, []byte(body))
}

func WriteBytes(t *testing.T, path string, data []byte) string {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, data, 0o644))
	return path
}

func Read(t *testing.T, path string) string {
	t.Helper()
	return string(ReadBytes(t, path))
}

func ReadBytes(t *testing.T, path string) []byte {
	t.Helper()
	body, err := os.ReadFile(path)
	require.NoError(t, err)
	return body
}

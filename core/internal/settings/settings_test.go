package settings

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    UISettings
	}{
		{"missing file", "", Defaults()},
		{"invalid json", "{not json", Defaults()},
		{"partial file keeps other defaults", `{"themeMode": "dark"}`, Defaults()},
		{"file manager registration off", `{"registerFileManager": false}`, UISettings{RegisterFileManager: false, RecentFiles: true}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			if tt.content != "" {
				require.NoError(t, os.WriteFile(filepath.Join(dir, FileName), []byte(tt.content), 0o600))
			}
			assert.Equal(t, tt.want, Load(dir))
		})
	}
}

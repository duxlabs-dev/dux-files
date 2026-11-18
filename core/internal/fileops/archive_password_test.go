package fileops

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/duxlabs-dev/dux-files/core/internal/testfs"

	"github.com/sorairolake/lzip-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// secret-*.zip and secret*.7z were packed from the sampleTree layout with
// zip -P secret --symlinks and 7z -psecret (-mem=AES256, -mhe=on). The
// rar fixtures are from the rarfile test suite (ISC), password "password".
var protectedFixtures = []struct {
	name     string
	password string
	check    func(t *testing.T, base string)
}{
	{"secret-zipcrypto.zip", "secret", func(t *testing.T, base string) { assertSampleTree(t, filepath.Join(base, "sample")) }},
	{"secret-aes256.zip", "secret", func(t *testing.T, base string) { assertSampleTree(t, filepath.Join(base, "sample")) }},
	{"secret.7z", "secret", func(t *testing.T, base string) { assertSampleTree(t, filepath.Join(base, "sample")) }},
	{"secret-header.7z", "secret", func(t *testing.T, base string) { assertSampleTree(t, filepath.Join(base, "sample")) }},
	{"rar5-psw.rar", "password", assertRarNumbers},
	{"rar5-hpsw.rar", "password", assertRarNumbers},
	{"rar3-comment-psw.rar", "password", func(t *testing.T, base string) {
		assert.Equal(t, "", testfs.Read(t, filepath.Join(base, "file1.txt")))
		assert.Equal(t, "", testfs.Read(t, filepath.Join(base, "file2.txt")))
	}},
}

func assertRarNumbers(t *testing.T, base string) {
	t.Helper()
	var want strings.Builder
	for i := range 512 {
		fmt.Fprintf(&want, "%03d\n", i)
	}
	assert.Equal(t, want.String(), testfs.Read(t, filepath.Join(base, "stest1.txt")))
	assert.Equal(t, want.String(), testfs.Read(t, filepath.Join(base, "stest2.txt")))
}

func unlockWith(passwords ...string) func(PasswordPrompt) Answer {
	return func(p PasswordPrompt) Answer {
		password := passwords[0]
		if len(passwords) > 1 {
			passwords = passwords[1:]
		}
		return Answer{ID: p.ID, Action: ActionUnlock, Password: password}
	}
}

func extractProtected(t *testing.T, archives []string, answer func(PasswordPrompt) Answer) (string, View, *testBus) {
	t.Helper()
	manager, bus := newHarness(t)
	bus.onPassword = answer
	out := filepath.Join(t.TempDir(), "out")
	require.NoError(t, os.Mkdir(out, 0o755))
	view := run(t, manager, bus, spec{kind: KindExtract, sources: archives, dest: out})
	return out, view, bus
}

func prompts(bus *testBus) []PasswordPrompt {
	var out []PasswordPrompt
	for _, e := range bus.kinds("password") {
		out = append(out, *e.PasswordPrompt)
	}
	return out
}

func TestExtractProtectedArchivesAskForThePassword(t *testing.T) {
	for _, tc := range protectedFixtures {
		t.Run(tc.name, func(t *testing.T) {
			archive := fixture(t, tc.name)
			out, view, bus := extractProtected(t, []string{archive}, unlockWith(tc.password))
			require.Equal(t, StateDone, view.State, view.Error)

			asked := prompts(bus)
			require.Len(t, asked, 1)
			assert.Equal(t, archive, asked[0].Path)
			assert.False(t, asked[0].Wrong)
			assert.Equal(t, []Action{ActionUnlock, ActionSkip, ActionCancel}, asked[0].Actions)

			base := filepath.Join(out, archiveStem(tc.name))
			assert.Equal(t, []Created{{Path: base, Dir: true}}, view.Created)
			tc.check(t, base)
			assert.Equal(t, 0, view.Progress.FilesTotal-view.Progress.FilesDone)
		})
	}
}

func TestExtractAsksAgainAfterAWrongPassword(t *testing.T) {
	for _, name := range []string{"secret-zipcrypto.zip", "secret-aes256.zip", "secret.7z", "secret-header.7z", "rar5-psw.rar", "rar5-hpsw.rar"} {
		t.Run(name, func(t *testing.T) {
			password := "secret"
			if strings.HasSuffix(name, ".rar") {
				password = "password"
			}
			out, view, bus := extractProtected(t, []string{fixture(t, name)}, unlockWith("nope", password))
			require.Equal(t, StateDone, view.State, view.Error)

			asked := prompts(bus)
			require.Len(t, asked, 2)
			assert.False(t, asked[0].Wrong)
			assert.True(t, asked[1].Wrong)
			assert.DirExists(t, filepath.Join(out, archiveStem(name)))
			entries, err := os.ReadDir(out)
			require.NoError(t, err)
			assert.Len(t, entries, 1, "the failed attempt leaves nothing behind")
		})
	}
}

func TestExtractSkipsAProtectedArchiveWhenDeclined(t *testing.T) {
	archive := fixture(t, "secret-aes256.zip")
	plain := fixture(t, "hello.txt.gz")
	out, view, bus := extractProtected(t, []string{archive, plain}, func(p PasswordPrompt) Answer {
		return Answer{ID: p.ID, Action: ActionSkip}
	})
	require.Equal(t, StateDone, view.State, view.Error)
	assert.Len(t, prompts(bus), 1)
	assert.Equal(t, 1, view.Skipped)
	assert.NoDirExists(t, filepath.Join(out, "secret-aes256"))
	assert.Equal(t, "hello", testfs.Read(t, filepath.Join(out, "hello.txt")), "the rest of the job still runs")
}

func TestExtractCancelsFromThePasswordPrompt(t *testing.T) {
	out, view, _ := extractProtected(t, []string{fixture(t, "secret.7z")}, func(p PasswordPrompt) Answer {
		return Answer{ID: p.ID, Action: ActionCancel}
	})
	require.Equal(t, StateDone, view.State, view.Error)
	assert.True(t, view.Cancelled)
	assert.Empty(t, view.Created)
	assert.NoDirExists(t, filepath.Join(out, "secret"))
}

func TestExtractReusesThePasswordForTheNextArchive(t *testing.T) {
	zipFile := fixture(t, "secret-aes256.zip")
	sevenZip := fixture(t, "secret.7z")
	out, view, bus := extractProtected(t, []string{zipFile, sevenZip}, unlockWith("secret"))
	require.Equal(t, StateDone, view.State, view.Error)
	assert.Len(t, prompts(bus), 1)
	assertSampleTree(t, filepath.Join(out, "secret-aes256", "sample"))
	assertSampleTree(t, filepath.Join(out, "secret", "sample"))
}

func TestExtractAsksFreshWhenTheRememberedPasswordFails(t *testing.T) {
	sevenZip := fixture(t, "secret.7z")
	rar := fixture(t, "rar5-psw.rar")
	out, view, bus := extractProtected(t, []string{sevenZip, rar}, unlockWith("secret", "password"))
	require.Equal(t, StateDone, view.State, view.Error)

	asked := prompts(bus)
	require.Len(t, asked, 2)
	assert.Equal(t, rar, asked[1].Path)
	assert.False(t, asked[1].Wrong, "the user never typed a password for this archive")
	assertRarNumbers(t, filepath.Join(out, "rar5-psw"))
}

func TestCompressWithAPasswordMakesAnAESZip(t *testing.T) {
	root, project := sampleTree(t)
	manager, bus := newHarness(t)

	view := run(t, manager, bus, spec{kind: KindCompress, sources: []string{project}, dest: root, name: "locked", format: FormatZip, password: "hunter2"})
	require.Equal(t, StateDone, view.State, view.Error)
	archive := filepath.Join(root, "locked.zip")

	reader, err := zip.OpenReader(archive)
	require.NoError(t, err)
	defer reader.Close()
	for _, f := range reader.File {
		if strings.HasSuffix(f.Name, "/") {
			assert.Zero(t, f.Flags&zipFlagEncrypted, f.Name)
			continue
		}
		assert.NotZero(t, f.Flags&zipFlagEncrypted, f.Name)
		assert.Equal(t, uint16(zipMethodAES), f.Method, f.Name)
		params, err := parseWinZipAES(f.Extra)
		require.NoError(t, err, f.Name)
		assert.Equal(t, byte(3), params.strength)
	}

	out, extracted, bus := extractProtected(t, []string{archive}, unlockWith("wrong", "hunter2"))
	require.Equal(t, StateDone, extracted.State, extracted.Error)
	asked := prompts(bus)
	require.Len(t, asked, 2)
	assert.True(t, asked[1].Wrong)
	assertSampleTree(t, filepath.Join(out, "locked", "project"))
}

func TestCompressRefusesAPasswordOutsideZip(t *testing.T) {
	manager, _ := newHarness(t)
	out := call(t, manager, ipcRequest("files.op.compress", map[string]any{"paths": "/tmp/a", "dest": "/tmp", "name": "x", "format": FormatTarGz, "password": "x"}))
	require.Contains(t, out, "error")
	require.Empty(t, manager.Jobs())
}

func TestUnlockAnswerCarriesThePassword(t *testing.T) {
	manager, bus := newHarness(t)
	archive := fixture(t, "secret-aes256.zip")
	out := filepath.Join(t.TempDir(), "out")
	require.NoError(t, os.Mkdir(out, 0o755))
	bus.onPassword = func(p PasswordPrompt) Answer {
		result := call(t, manager, ipcRequest("files.op.respond", map[string]any{"jobId": currentJobID(manager), "id": p.ID, "action": "unlock", "password": "secret"}))
		require.NotContains(t, result, "error", result)
		return Answer{}
	}
	job := manager.enqueue(spec{kind: KindExtract, sources: []string{archive}, dest: out})
	view := bus.wait(t, job.id)
	require.Equal(t, StateDone, view.State, view.Error)
	assertSampleTree(t, filepath.Join(out, "secret-aes256", "sample"))
}

func currentJobID(m *Manager) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.order[0]
}

func TestExtractLzip(t *testing.T) {
	root := t.TempDir()
	var tarball bytes.Buffer
	tw := tar.NewWriter(&tarball)
	body := []byte("hello")
	require.NoError(t, tw.WriteHeader(&tar.Header{Name: "sample/", Typeflag: tar.TypeDir, Mode: 0o755}))
	require.NoError(t, tw.WriteHeader(&tar.Header{Name: "sample/README.md", Typeflag: tar.TypeReg, Mode: 0o644, Size: int64(len(body))}))
	_, err := tw.Write(body)
	require.NoError(t, err)
	require.NoError(t, tw.Close())

	tarLz := filepath.Join(root, "sample.tar.lz")
	writeLzip(t, tarLz, tarball.Bytes())
	fileLz := filepath.Join(root, "hello.txt.lz")
	writeLzip(t, fileLz, body)
	bare := filepath.Join(root, "bare")
	writeLzip(t, bare, tarball.Bytes())

	format, err := detectFormat(bare)
	require.NoError(t, err)
	assert.Equal(t, archiveFormat{compressLzip, containerNone}, format)

	out, view := extractFixture(t, tarLz)
	require.Equal(t, StateDone, view.State, view.Error)
	assert.Equal(t, "hello", testfs.Read(t, filepath.Join(out, "sample", "README.md")))

	out, view = extractFixture(t, fileLz)
	require.Equal(t, StateDone, view.State, view.Error)
	assert.Equal(t, "hello", testfs.Read(t, filepath.Join(out, "hello.txt")))

	out, view = extractFixture(t, bare)
	require.Equal(t, StateDone, view.State, view.Error)
	assert.Equal(t, "hello", testfs.Read(t, filepath.Join(out, "bare", "sample", "README.md")))
}

func writeLzip(t *testing.T, path string, body []byte) {
	t.Helper()
	f, err := os.Create(path)
	require.NoError(t, err)
	w := lzip.NewWriter(f)
	_, err = w.Write(body)
	require.NoError(t, err)
	require.NoError(t, w.Close())
	require.NoError(t, f.Close())
}

func TestADamagedAESEntryIsNotReportedAsAWrongPassword(t *testing.T) {
	root, project := sampleTree(t)
	manager, bus := newHarness(t)
	view := run(t, manager, bus, spec{kind: KindCompress, sources: []string{project}, dest: root, name: "locked", format: FormatZip, password: "hunter2"})
	require.Equal(t, StateDone, view.State, view.Error)
	archive := filepath.Join(root, "locked.zip")

	reader, err := zip.OpenReader(archive)
	require.NoError(t, err)
	var offset int64
	for _, f := range reader.File {
		if f.Name == "project/src/main.go" {
			offset, err = f.DataOffset()
			require.NoError(t, err)
		}
	}
	require.NoError(t, reader.Close())
	data, err := os.ReadFile(archive)
	require.NoError(t, err)
	data[offset+18] ^= 0xff
	require.NoError(t, os.WriteFile(archive, data, 0o644))

	_, view, bus = extractProtected(t, []string{archive}, unlockWith("hunter2"))
	require.Equal(t, StateFailed, view.State)
	assert.NotContains(t, strings.ToLower(view.Error), "password")
	require.Len(t, prompts(bus), 1, "the right password is not asked for again")
}

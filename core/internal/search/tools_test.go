package search

import (
	"bufio"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/duxlabs-dev/dux-files/core/internal/testfs"
	"github.com/duxlabs-dev/dux-go/files"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func toolService(t *testing.T, bus Publisher, tool string) *Service {
	t.Helper()
	if _, err := exec.LookPath(tool); err != nil {
		t.Skip(tool + " is not installed")
	}
	svc := newTestService(t, bus)
	svc.tools.lookPath = func(name string) (string, error) {
		if name != tool {
			return "", exec.ErrNotFound
		}
		return exec.LookPath(name)
	}
	return svc
}

func search1(t *testing.T, svc *Service, bus *testBus, root, query string, opts Options) (event, string) {
	t.Helper()
	_, topic, err := svc.Start(context.Background(), root, query, opts)
	require.NoError(t, err)
	return bus.waitDone(t), topic
}

func (b *testBus) hits(topic string) map[string]hit {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := map[string]hit{}
	for _, ev := range b.events[topic] {
		for _, h := range ev.Entries {
			out[h.Name] = h
		}
	}
	return out
}

func TestToolVersionGate(t *testing.T) {
	tests := []struct {
		spec   toolSpec
		banner string
		want   bool
	}{
		{fdSpec, "fd 10.5.0", true},
		{fdSpec, "fd 8.7.0", true},
		{fdSpec, "fd 8.6.9", false},
		{fdSpec, "fdfind 9.0.0", false},
		{fdSpec, "", false},
		{rgSpec, "ripgrep 15.2.0 (rev abc)", true},
		{rgSpec, "ripgrep 12.0.0", true},
		{rgSpec, "ripgrep 11.0.2", false},
		{rgSpec, "ripgrep 14.1.1-dev", true},
		{rgSpec, "ripgrep fourteen", false},
		{rgSpec, "rg 14.0.0", false},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, tt.spec.accepts(tt.banner), "%q", tt.banner)
	}
}

func TestToolboxPicksUsableToolsAndCachesTheLookup(t *testing.T) {
	now := time.Unix(1000, 0)
	lookups := 0
	banners := map[string]string{"/bin/fd": "fd 7.0.0", "/bin/fdfind": "fd 9.0.0", "/bin/rg": "ripgrep 14.0.0"}
	svc := NewService(nil, nil)
	box := &svc.tools
	box.now = func() time.Time { return now }
	box.lookPath = func(name string) (string, error) {
		lookups++
		return "/bin/" + name, nil
	}
	box.banner = func(path string) string { return banners[path] }

	assert.Equal(t, tools{fd: "/bin/fdfind", rg: "/bin/rg"}, box.locate())
	assert.Equal(t, []string{"fd", "rg"}, svc.Tools())
	seen := lookups
	box.locate()
	assert.Equal(t, seen, lookups, "a second search inside the ttl does not look again")

	now = now.Add(locateTTL + time.Second)
	banners["/bin/rg"] = "ripgrep 1.0.0"
	box.lookPath = func(name string) (string, error) {
		if name == "fdfind" {
			return "", exec.ErrNotFound
		}
		return "/bin/" + name, nil
	}
	assert.Equal(t, tools{rg: "/bin/rg"}, box.locate(), "a known path is trusted, a vanished one is dropped")
}

func TestToolArguments(t *testing.T) {
	t.Setenv("HOME", "/home/me")
	rules := newSkipRules([]string{"Steam", "we[ird]", "~/big", "/mnt/skip", "a/b"})
	rules.skipNames = map[string]bool{"Steam": true, "we[ird]": true, "a/b": true}
	m, err := newMatcher("report *.TXT", Options{})
	require.NoError(t, err)

	assert.Equal(t, []string{
		"--color", "never", "--print0", "--full-path", "--ignore-case", "--no-require-git", "--show-errors",
		"--hidden", "--one-file-system",
		"--exclude", "Steam/", "--exclude", `we\[ird\]/`, "--exclude", "/big/",
		"--", "report", "/home/me",
	}, fdArgs("/home/me", m, Options{IncludeHidden: true}, rules))
	assert.Equal(t, []string{
		"--color", "never", "--print0", "--full-path", "--ignore-case", "--no-require-git", "--show-errors",
		"--no-ignore", "--", "report", "/",
	}, fdArgs("/", m, Options{IncludeIgnored: true, CrossDevice: true}, ignoreRules{}))

	assert.Equal(t, []string{
		"--no-config", "--color", "never", "--no-messages", "--no-heading", "--with-filename", "--line-number", "--column", "--null",
		"--max-count", "1", "--no-require-git", "--ignore-case",
		"--fixed-strings", "--one-file-system", "--max-filesize", "1000",
		"--glob", "!Steam/", "--glob", `!we\[ird\]/`, "--glob", "!/home/me/big/", "--glob", "!/mnt/skip/",
		"--regexp", "-v needle", "--", "/home/me", "/mnt/usb",
	}, rgArgs([]string{"/home/me", "/mnt/usb"}, " -v needle ", Options{MaxSize: 1000}, rules))
	assert.Contains(t, rgArgs([]string{"/"}, "a.+b ", Options{Regex: true, CrossDevice: true}, ignoreRules{}), "a.+b ")
	assert.NotContains(t, rgArgs([]string{"/"}, "a.+b", Options{Regex: true}, ignoreRules{}), "--fixed-strings")
}

func TestFdPatternIsALiteralEveryMatchContains(t *testing.T) {
	tests := []struct {
		query string
		opts  Options
		want  string
	}{
		{"report", Options{}, "report"},
		{"a.b final", Options{}, "f[iİ]nal"},
		{"*.tar.gz x", Options{}, `\.tar\.gz`},
		{"café", Options{}, "caf"},
		{"日本", Options{}, "."},
		{"*", Options{}, "."},
		{"report", Options{Fuzzy: true}, "."},
		{"^re.*t$", Options{Regex: true}, "."},
	}
	for _, tt := range tests {
		m, err := newMatcher(tt.query, tt.opts)
		require.NoError(t, err)
		assert.Equal(t, tt.want, m.fdPattern(), "%q", tt.query)
	}
}

func TestRegexMatchesNamesAndRejectsABadPattern(t *testing.T) {
	root := tree(t)
	bus, _, topic := runSearch(t, root, `^report.*\.(pdf|txt)$`, Options{Regex: true})
	assert.ElementsMatch(t, []string{"Report.pdf", "report-2024.txt"}, bus.names(topic))

	bus, _, topic = runSearch(t, root, `(?-i)^Report`, Options{Regex: true})
	assert.ElementsMatch(t, []string{"Report.pdf", "Reports"}, bus.names(topic))

	bus, done, topic := runSearch(t, root, `report(`, Options{Regex: true})
	assert.Empty(t, bus.names(topic))
	assert.Equal(t, string(files.CodeInvalid), done.Code)
	assert.NotEmpty(t, done.Error)
}

func TestFdFindsWhatTheWalkFinds(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	root := t.TempDir()
	for _, path := range []string{
		"report.txt", "docs/Report Final.md", "docs/deep/quarterly-report.pdf", "reports/summary.txt",
		".hidden/report-secret.txt", "docs/.report-dot.txt", "node_modules/report-dep.js", "Steam/report-game.txt",
		"we[ird]/report-odd.txt", "big/report-big.txt", "ignored-report.log", "İstanbul-report.txt", "Kelvin-report.txt",
		"r-e-p-o-r-t.txt", "go/report-inside-go.txt", "src/go", "src/search/walk.go", "src/walk/readme.md", "notes.txt",
	} {
		testfs.Write(t, filepath.Join(root, path), "x")
	}
	testfs.Write(t, filepath.Join(root, ".gitignore"), "*.log\n")
	require.NoError(t, os.Symlink(filepath.Join(root, "docs"), filepath.Join(root, "report-link")))
	excludes := func() []string { return []string{"Steam", "we[ird]", filepath.Join(root, "big")} }

	tests := []struct {
		name  string
		query string
		opts  Options
	}{
		{"plain term", "report", Options{}},
		{"upper case query", "REPORT", Options{}},
		{"hidden", "report", Options{IncludeHidden: true}},
		{"ignored", "report", Options{IncludeIgnored: true}},
		{"two terms against the path", "search walk", Options{}},
		{"glob", "*.txt", Options{}},
		{"glob with a second term", "re*.txt docs", Options{IncludeIgnored: true}},
		{"folded letters", "istanbul", Options{}},
		{"kelvin sign", "kelvin", Options{}},
		{"fuzzy", "report", Options{Fuzzy: true}},
		{"regex", `report.*\.(txt|md)$`, Options{Regex: true}},
		{"a file named like a skipped folder", "go", Options{}},
		{"type filter", "report", Options{Types: []string{"folders"}}},
		{"nothing", "zzqq", Options{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			walkBus := newBus()
			walked := newTestService(t, walkBus)
			walked.SetExcludes(excludes)
			_, walkTopic := search1(t, walked, walkBus, root, tt.query, tt.opts)

			fdBus := newBus()
			found := toolService(t, fdBus, "fd")
			found.SetExcludes(excludes)
			_, fdTopic := search1(t, found, fdBus, root, tt.query, tt.opts)

			assert.Equal(t, "fd", fdBus.backend(fdTopic).Backend)
			assert.False(t, found.tools.logged.Load(), "fd ran without falling back")
			assert.Equal(t, walkBus.scores(walkTopic), fdBus.scores(fdTopic))
		})
	}
}

func TestFdCountsUnreadableFoldersAndStopsAtTheLimit(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root reads everything")
	}
	root := t.TempDir()
	locked := filepath.Join(root, "locked")
	testfs.Write(t, filepath.Join(locked, "match.txt"), "x")
	for _, name := range []string{"match-a.txt", "match-b.txt", "match-c.txt"} {
		testfs.Write(t, filepath.Join(root, name), "x")
	}
	require.NoError(t, os.Chmod(locked, 0o000))
	t.Cleanup(func() { _ = os.Chmod(locked, 0o755) })
	bus := newBus()
	svc := toolService(t, bus, "fd")

	done, topic := search1(t, svc, bus, root, "match", Options{})
	assert.Len(t, bus.names(topic), 3)
	assert.Equal(t, 1, done.Unreadable)

	done, _ = search1(t, svc, bus, root, "match", Options{Limit: 2})
	assert.True(t, done.Truncated)
	assert.Equal(t, 2, done.Count)
}

func TestCancelStopsFdAndRg(t *testing.T) {
	root := t.TempDir()
	for i := range 400 {
		testfs.Write(t, filepath.Join(root, "d"+strconv.Itoa(i%20), "match-"+strconv.Itoa(i)+".txt"), "match\n")
	}
	tests := []struct {
		tool string
		opts Options
	}{
		{"fd", Options{}},
		{"rg", Options{Contents: true}},
	}
	for _, tt := range tests {
		t.Run(tt.tool, func(t *testing.T) {
			bus := newBus()
			svc := toolService(t, bus, tt.tool)
			svc.stater = slowStater{files.NewService(nil, t.TempDir(), nil)}

			id, topic, err := svc.Start(context.Background(), root, "match", tt.opts)
			require.NoError(t, err)
			bus.waitResults(t)
			assert.True(t, svc.Cancel(id))
			done := bus.waitDone(t)
			assert.Equal(t, tt.tool, bus.backend(topic).Backend)
			assert.True(t, done.Cancelled)
			assert.Less(t, done.Count, 400)
			assert.Empty(t, done.Error)

			gone, disconnect := context.WithCancel(context.Background())
			disconnect()
			_, _, err = svc.Start(gone, root, "match", tt.opts)
			require.NoError(t, err)
			done = bus.waitDone(t)
			assert.True(t, done.Cancelled)
			assert.Empty(t, done.Error)
			assert.False(t, svc.tools.logged.Load(), "a cancelled tool is not a broken tool")
			svc.Close()
		})
	}
}

func TestABrokenFdFallsBackToTheWalk(t *testing.T) {
	root := tree(t)
	broken := filepath.Join(t.TempDir(), "fd")
	require.NoError(t, os.WriteFile(broken, []byte("#!/bin/sh\necho 'error: unexpected argument' >&2\nexit 2\n"), 0o755))
	bus := newBus()
	svc := newTestService(t, bus)
	svc.tools.lookPath = func(name string) (string, error) {
		if name != "fd" {
			return "", exec.ErrNotFound
		}
		return broken, nil
	}
	svc.tools.banner = func(string) string { return "fd 10.0.0" }

	done, topic := search1(t, svc, bus, root, "report", Options{})
	assert.ElementsMatch(t, []string{"Report.pdf", "report-2024.txt", "Another Report.md", "Reports"}, bus.names(topic))
	assert.Empty(t, done.Error)
}

func TestRgSearchesFileContents(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	root := t.TempDir()
	testfs.Write(t, filepath.Join(root, "notes.txt"), "first line\n  the Needle is here  \nneedle again\n")
	testfs.Write(t, filepath.Join(root, "docs", "deep.md"), "# needle heading\n")
	testfs.Write(t, filepath.Join(root, "needle-in-name-only.txt"), "nothing\n")
	testfs.Write(t, filepath.Join(root, ".hidden", "secret.txt"), "needle\n")
	testfs.Write(t, filepath.Join(root, "node_modules", "dep.js"), "needle\n")
	testfs.Write(t, filepath.Join(root, "Steam", "game.txt"), "needle\n")
	testfs.Write(t, filepath.Join(root, "big", "huge.txt"), "needle\n")
	testfs.Write(t, filepath.Join(root, "ignored.log"), "needle\n")
	testfs.Write(t, filepath.Join(root, "odd\nname.txt"), "needle\n")
	testfs.Write(t, filepath.Join(root, "version.txt"), "v1.2\nv1x2\n")
	testfs.Write(t, filepath.Join(root, ".gitignore"), "*.log\n")
	bus := newBus()
	svc := toolService(t, bus, "rg")
	svc.SetExcludes(func() []string { return []string{"Steam", filepath.Join(root, "big")} })

	done, topic := search1(t, svc, bus, root, " needle ", Options{Contents: true})
	hits := bus.hits(topic)
	assert.ElementsMatch(t, []string{"notes.txt", "deep.md", "odd\nname.txt"}, bus.names(topic))
	assert.Equal(t, event{Kind: "backend", SearchID: done.SearchID, Backend: "rg", Reason: reasonNoIndex, Contents: true}, bus.backend(topic))
	assert.Equal(t, 2, hits["notes.txt"].Line)
	assert.Equal(t, "the Needle is here", hits["notes.txt"].Snippet)
	assert.Equal(t, filepath.Join(root, "odd\nname.txt"), hits["odd\nname.txt"].Path)
	assert.Empty(t, done.Error)

	_, topic = search1(t, svc, bus, root, "needle", Options{Contents: true, IncludeHidden: true})
	assert.ElementsMatch(t, []string{"notes.txt", "deep.md", "odd\nname.txt", "secret.txt"}, bus.names(topic))

	_, topic = search1(t, svc, bus, root, "needle", Options{Contents: true, IncludeIgnored: true})
	assert.ElementsMatch(t, []string{"notes.txt", "deep.md", "odd\nname.txt", "dep.js", "game.txt", "huge.txt", "ignored.log"}, bus.names(topic))

	_, topic = search1(t, svc, bus, root, "needle", Options{Contents: true, MaxSize: 20})
	assert.ElementsMatch(t, []string{"deep.md", "odd\nname.txt"}, bus.names(topic))

	done, _ = search1(t, svc, bus, root, "needle", Options{Contents: true, Limit: 1})
	assert.True(t, done.Truncated)
	assert.Equal(t, 1, done.Count)

	_, topic = search1(t, svc, bus, root, "v1.2", Options{Contents: true})
	assert.Equal(t, 1, bus.hits(topic)["version.txt"].Line, "a plain query is not a pattern")
	_, topic = search1(t, svc, bus, root, `v1x\d`, Options{Contents: true, Regex: true})
	assert.Equal(t, 2, bus.hits(topic)["version.txt"].Line)

	done, topic = search1(t, svc, bus, root, "needle(", Options{Contents: true, Regex: true})
	assert.Empty(t, bus.names(topic))
	assert.Equal(t, string(files.CodeInvalid), done.Code)
	assert.Contains(t, done.Error, "regex parse error")

	done, topic = search1(t, svc, bus, root, "zzqq-not-there", Options{Contents: true})
	assert.Empty(t, bus.names(topic))
	assert.Empty(t, done.Error)
}

func TestContentsWithoutRgMatchesNamesAndSaysSo(t *testing.T) {
	root := t.TempDir()
	testfs.Write(t, filepath.Join(root, "needle.txt"), "nothing\n")
	testfs.Write(t, filepath.Join(root, "other.txt"), "needle\n")
	bus, _, topic := runSearch(t, root, "needle", Options{Contents: true})
	assert.Equal(t, []string{"needle.txt"}, bus.names(topic))
	assert.False(t, bus.backend(topic).Contents)
}

func TestMatchesAreReadWithASnippetAroundTheHit(t *testing.T) {
	long := strings.Repeat("x", 3000) + " needle " + strings.Repeat("y", 3000)
	unicode := strings.Repeat("é", 200) + "needle"
	reader := bufio.NewReaderSize(strings.NewReader(
		"/a/short.txt\x002:9:\t  the needle is here  \n"+
			"/a/odd\nname.txt\x001:1:needle\n"+
			"/a/long.js\x007:3002:"+long+"\n"+
			"/a/unicode.txt\x001:401:"+unicode+"\n"+
			"/a/bad.txt\x003:5:bad \xff needle",
	), 64)
	var got []match
	for {
		found, ok := readMatch(reader)
		if !ok {
			break
		}
		got = append(got, found)
	}
	require.Len(t, got, 5)
	assert.Equal(t, match{path: "/a/short.txt", line: 2, snippet: "the needle is here"}, got[0])
	assert.Equal(t, match{path: "/a/odd\nname.txt", line: 1, snippet: "needle"}, got[1])
	assert.Equal(t, 7, got[2].line)
	assert.Equal(t, "…"+strings.Repeat("x", snippetLead-1)+" needle "+strings.Repeat("y", snippetBytes-snippetLead-7)+"…", got[2].snippet)
	assert.Equal(t, "…"+strings.Repeat("é", snippetLead/2)+"needle", got[3].snippet)
	assert.Equal(t, match{path: "/a/bad.txt", line: 3, snippet: "bad needle"}, got[4])
}

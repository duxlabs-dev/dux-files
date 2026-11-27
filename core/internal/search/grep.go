package search

import (
	"bufio"
	"bytes"
	"context"
	"os/exec"
	"strconv"
	"strings"

	"github.com/duxlabs-dev/dux-go/files"
)

const (
	snippetLead  = 24
	snippetBytes = 240
	stderrKept   = 4096
)

func (sc *search) grep(out *sink, rg string, targets []string, query, reason string) {
	sc.publish(event{Kind: "backend", Backend: "rg", Reason: reason, Contents: true})
	ctx, stop := context.WithCancel(sc.ctx)
	defer stop()
	cmd := exec.CommandContext(ctx, rg, rgArgs(targets, query, out.opts, sc.skipRules(out.opts.IncludeIgnored))...)
	cmd.Dir = "/"
	cmd.WaitDelay = toolWaitDelay
	complaint := cappedBuffer{limit: stderrKept}
	cmd.Stderr = &complaint
	stdout, err := cmd.StdoutPipe()
	if err == nil {
		err = cmd.Start()
	}
	switch {
	case err == nil:
	case sc.ctx.Err() != nil:
		out.em.finish(false, 0)
		return
	default:
		out.em.fail(files.CodeIO, err.Error())
		return
	}

	matched := 0
	reader := bufio.NewReaderSize(stdout, 64*1024)
	for {
		found, ok := readMatch(reader)
		if !ok {
			break
		}
		matched++
		if !out.emit(found.path, hit{Line: found.line, Snippet: found.snippet}) {
			stop()
			break
		}
	}
	failure := toolFailure(sc.ctx, cmd.Wait(), matched)
	if failure == nil || complaint.Len() == 0 {
		out.em.finish(false, 0)
		return
	}
	out.em.fail(files.CodeInvalid, complaint.String())
}

func rgArgs(targets []string, query string, opts Options, rules ignoreRules) []string {
	args := []string{
		"--no-config", "--color", "never", "--no-messages", "--no-heading", "--with-filename", "--line-number", "--column", "--null",
		"--max-count", "1", "--no-require-git", "--ignore-case",
	}
	pattern := query
	if !opts.Regex {
		args = append(args, "--fixed-strings")
		pattern = strings.TrimSpace(query)
	}
	if opts.IncludeHidden {
		args = append(args, "--hidden")
	}
	if opts.IncludeIgnored {
		args = append(args, "--no-ignore")
	}
	if !opts.CrossDevice {
		args = append(args, "--one-file-system")
	}
	if opts.MaxSize > 0 {
		args = append(args, "--max-filesize", strconv.FormatInt(opts.MaxSize, 10))
	}
	for _, glob := range rules.globs("") {
		args = append(args, "--glob", "!"+glob)
	}
	args = append(args, "--regexp", pattern, "--")
	return append(args, targets...)
}

type match struct {
	path    string
	line    int
	snippet string
}

func readMatch(reader *bufio.Reader) (match, bool) {
	path, err := reader.ReadString(0)
	if err != nil {
		return match{}, false
	}
	line := readNumber(reader)
	column := readNumber(reader)
	return match{path: strings.TrimSuffix(path, "\x00"), line: line, snippet: readSnippet(reader, column)}, true
}

func readNumber(reader *bufio.Reader) int {
	digits, _ := reader.ReadString(':')
	number, _ := strconv.Atoi(strings.TrimSuffix(digits, ":"))
	return number
}

func readSnippet(reader *bufio.Reader, column int) string {
	from := max(0, column-1-snippetLead)
	var head, window []byte
	length := 0
	for {
		chunk, err := reader.ReadSlice('\n')
		body := bytes.TrimSuffix(chunk, []byte{'\n'})
		head = append(head, span(body, -length, snippetBytes-length)...)
		window = append(window, span(body, from-length, from+snippetBytes-length)...)
		length += len(body)
		if err != bufio.ErrBufferFull {
			break
		}
	}
	if length <= snippetBytes {
		return tidy(head)
	}
	text := tidy(window)
	if from > 0 {
		text = "…" + text
	}
	if from+snippetBytes < length {
		text += "…"
	}
	return text
}

func span(body []byte, from, to int) []byte {
	return body[min(max(from, 0), len(body)):min(max(to, 0), len(body))]
}

func tidy(text []byte) string {
	return strings.Join(strings.Fields(strings.ToValidUTF8(string(text), "")), " ")
}

type cappedBuffer struct {
	strings.Builder
	limit int
}

func (b *cappedBuffer) Write(p []byte) (int, error) {
	if room := b.limit - b.Len(); room > 0 {
		b.Builder.Write(p[:min(room, len(p))])
	}
	return len(p), nil
}

func (b *cappedBuffer) String() string {
	return strings.TrimSpace(strings.ToValidUTF8(b.Builder.String(), "�"))
}

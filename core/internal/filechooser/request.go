package filechooser

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/duxlabs-dev/dux-files/core/internal/pathutil"

	"github.com/duxlabs-dev/dux-go/dbusutil"

	"github.com/godbus/dbus/v5"
)

const (
	MethodOpen      = "open"
	MethodSave      = "save"
	MethodSaveFiles = "saveFiles"

	KindGlob = "glob"
	KindMime = "mime"

	ResponseOK        uint32 = 0
	ResponseCancelled uint32 = 1
	ResponseOther     uint32 = 2

	errInvalidArgument = "org.freedesktop.portal.Error.InvalidArgument"

	maxTitle    = 256
	maxLabel    = 64
	maxFilters  = 1024
	maxPatterns = 256
	maxText     = 256
	maxChoices  = 32
	maxOptions  = 64
	maxFiles    = 1024
)

var mimePattern = regexp.MustCompile(`^[a-z0-9!#$&^_.+-]+/([a-z0-9!#$&^_.+-]+|\*)$`)

type Pattern struct {
	Kind    string `json:"kind"`
	Pattern string `json:"pattern"`
}

type Filter struct {
	Name     string    `json:"name"`
	Patterns []Pattern `json:"patterns"`
	Globs    []string  `json:"globs"`
}

type Option struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

type Choice struct {
	ID      string   `json:"id"`
	Label   string   `json:"label"`
	Options []Option `json:"options"`
	Initial string   `json:"initial"`
}

type Request struct {
	ID            string   `json:"id"`
	Method        string   `json:"method"`
	AppID         string   `json:"appId"`
	ParentWindow  string   `json:"parentWindow"`
	Title         string   `json:"title"`
	AcceptLabel   string   `json:"acceptLabel"`
	Modal         bool     `json:"modal"`
	Multiple      bool     `json:"multiple"`
	Directory     bool     `json:"directory"`
	Filters       []Filter `json:"filters"`
	CurrentFilter int      `json:"currentFilter"`
	Choices       []Choice `json:"choices"`
	CurrentFolder string   `json:"currentFolder"`
	CurrentFile   string   `json:"currentFile"`
	CurrentName   string   `json:"currentName"`
	Files         []string `json:"files"`
}

type ChoiceValue struct {
	ID    string `json:"id"`
	Value string `json:"value"`
}

type Answer struct {
	Response      uint32        `json:"response"`
	Paths         []string      `json:"paths"`
	Choices       []ChoiceValue `json:"choices"`
	CurrentFilter int           `json:"currentFilter"`
}

type wirePattern struct {
	Kind    uint32
	Pattern string
}

type wireFilter struct {
	Name     string
	Patterns []wirePattern
}

type wireOption struct {
	ID    string
	Label string
}

type wireChoice struct {
	ID      string
	Label   string
	Options []wireOption
	Initial string
}

type wireChoiceValue struct {
	ID    string
	Value string
}

type parser struct {
	options map[string]dbus.Variant
	globs   func(mime string) []string
	err     error
}

func parseRequest(method, id, appID, parent, title string, options map[string]dbus.Variant, globs func(string) []string) (*Request, *dbus.Error) {
	p := &parser{options: options, globs: globs}
	req := &Request{
		ID:            id,
		Method:        method,
		AppID:         appID,
		ParentWindow:  parent,
		Title:         clip(strings.TrimSpace(title), maxTitle),
		Modal:         p.bool("modal", true),
		AcceptLabel:   clip(stripMnemonic(p.string("accept_label")), maxLabel),
		Choices:       p.choices(),
		CurrentFilter: -1,
		Files:         []string{},
		Filters:       []Filter{},
	}
	switch method {
	case MethodOpen:
		req.Multiple = p.bool("multiple", false)
		req.Directory = p.bool("directory", false)
		req.Filters, req.CurrentFilter = p.filters()
		req.CurrentFolder = p.folder("current_folder")
	case MethodSave:
		req.Filters, req.CurrentFilter = p.filters()
		req.CurrentName = sanitizedName(clip(p.string("current_name"), maxText))
		req.CurrentFolder = p.folder("current_folder")
		req.CurrentFile = p.file("current_file")
		if req.CurrentFile != "" {
			req.CurrentFolder = filepath.Dir(req.CurrentFile)
			if req.CurrentName == "" {
				req.CurrentName = filepath.Base(req.CurrentFile)
			}
		}
	case MethodSaveFiles:
		req.CurrentFolder = p.folder("current_folder")
		req.Files = p.files()
	}
	if p.err != nil {
		return nil, dbus.NewError(errInvalidArgument, []any{p.err.Error()})
	}
	if req.Choices == nil {
		req.Choices = []Choice{}
	}
	return req, nil
}

func (p *parser) fail(format string, args ...any) {
	if p.err != nil {
		return
	}
	p.err = fmt.Errorf(format, args...)
}

func (p *parser) string(key string) string {
	v, ok := p.options[key]
	if !ok {
		return ""
	}
	s, ok := dbusutil.As[string](v)
	if !ok {
		p.fail("option %s must be a string", key)
		return ""
	}
	return s
}

func (p *parser) bool(key string, def bool) bool {
	v, ok := p.options[key]
	if !ok {
		return def
	}
	b, ok := dbusutil.As[bool](v)
	if !ok {
		p.fail("option %s must be a boolean", key)
		return def
	}
	return b
}

func (p *parser) bytesPath(key string) string {
	v, ok := p.options[key]
	if !ok {
		return ""
	}
	raw, ok := dbusutil.As[[]byte](v)
	if !ok {
		p.fail("option %s must be a byte array", key)
		return ""
	}
	raw = bytes.TrimRight(raw, "\x00")
	if bytes.ContainsRune(raw, 0) {
		return ""
	}
	path := string(raw)
	if !filepath.IsAbs(path) {
		return ""
	}
	return filepath.Clean(path)
}

func (p *parser) folder(key string) string {
	path := p.bytesPath(key)
	if path == "" {
		return ""
	}
	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		return ""
	}
	return path
}

func (p *parser) file(key string) string {
	path := p.bytesPath(key)
	if path == "" || sanitizedName(filepath.Base(path)) == "" {
		return ""
	}
	info, err := os.Stat(filepath.Dir(path))
	if err != nil || !info.IsDir() {
		return ""
	}
	return path
}

func (p *parser) filters() ([]Filter, int) {
	filters := p.filterList("filters")
	current := -1
	v, ok := p.options["current_filter"]
	if !ok {
		return filters, current
	}
	var wire wireFilter
	if err := v.Store(&wire); err != nil {
		p.fail("option current_filter must be (sa(us))")
		return filters, current
	}
	wanted, ok := p.filter(wire)
	if !ok {
		return filters, current
	}
	if len(filters) == 0 {
		return []Filter{wanted}, 0
	}
	for i, f := range filters {
		if sameFilter(f, wanted) {
			return filters, i
		}
	}
	p.fail("option current_filter is not one of filters")
	return filters, current
}

func (p *parser) filterList(key string) []Filter {
	v, ok := p.options[key]
	if !ok {
		return []Filter{}
	}
	var wire []wireFilter
	if err := v.Store(&wire); err != nil {
		p.fail("option %s must be a(sa(us))", key)
		return []Filter{}
	}
	if len(wire) > maxFilters {
		p.fail("option %s lists too many filters", key)
		return []Filter{}
	}
	out := make([]Filter, 0, len(wire))
	for _, w := range wire {
		f, ok := p.filter(w)
		if !ok {
			return []Filter{}
		}
		out = append(out, f)
	}
	return out
}

func (p *parser) filter(w wireFilter) (Filter, bool) {
	name := clip(strings.TrimSpace(w.Name), maxText)
	switch {
	case name == "":
		p.fail("a filter has no name")
		return Filter{}, false
	case len(w.Patterns) == 0:
		p.fail("filter %q has no patterns", name)
		return Filter{}, false
	case len(w.Patterns) > maxPatterns:
		p.fail("filter %q has too many patterns", name)
		return Filter{}, false
	}
	f := Filter{Name: name, Patterns: make([]Pattern, 0, len(w.Patterns)), Globs: []string{}}
	seen := map[string]bool{}
	for _, wp := range w.Patterns {
		pattern := strings.TrimSpace(wp.Pattern)
		if pattern == "" || len(pattern) > maxText {
			p.fail("filter %q has an empty or oversized pattern", name)
			return Filter{}, false
		}
		switch wp.Kind {
		case 0:
			if _, err := filepath.Match(pattern, ""); err != nil || strings.ContainsRune(pattern, '/') {
				p.fail("filter %q has an invalid glob %q", name, pattern)
				return Filter{}, false
			}
			f.Patterns = append(f.Patterns, Pattern{Kind: KindGlob, Pattern: pattern})
			f.Globs = appendGlob(f.Globs, seen, strings.ToLower(pattern))
		case 1:
			mime := strings.ToLower(pattern)
			if !mimePattern.MatchString(mime) {
				p.fail("filter %q has an invalid mime type %q", name, pattern)
				return Filter{}, false
			}
			f.Patterns = append(f.Patterns, Pattern{Kind: KindMime, Pattern: mime})
			for _, glob := range p.mimeGlobs(mime) {
				f.Globs = appendGlob(f.Globs, seen, glob)
			}
		default:
			p.fail("filter %q has an unknown pattern type %d", name, wp.Kind)
			return Filter{}, false
		}
	}
	return f, true
}

func (p *parser) mimeGlobs(mime string) []string {
	if p.globs == nil {
		return nil
	}
	return p.globs(mime)
}

func appendGlob(globs []string, seen map[string]bool, glob string) []string {
	if seen[glob] {
		return globs
	}
	seen[glob] = true
	return append(globs, glob)
}

func sameFilter(a, b Filter) bool {
	if a.Name != b.Name || len(a.Patterns) != len(b.Patterns) {
		return false
	}
	for i := range a.Patterns {
		if a.Patterns[i] != b.Patterns[i] {
			return false
		}
	}
	return true
}

func (p *parser) choices() []Choice {
	v, ok := p.options["choices"]
	if !ok {
		return []Choice{}
	}
	var wire []wireChoice
	if err := v.Store(&wire); err != nil {
		p.fail("option choices must be a(ssa(ss)s)")
		return []Choice{}
	}
	if len(wire) > maxChoices {
		p.fail("option choices lists too many entries")
		return []Choice{}
	}
	out := make([]Choice, 0, len(wire))
	for _, w := range wire {
		c, ok := p.choice(w)
		if !ok {
			return []Choice{}
		}
		out = append(out, c)
	}
	return out
}

func (p *parser) choice(w wireChoice) (Choice, bool) {
	c := Choice{ID: clip(w.ID, maxText), Label: clip(strings.TrimSpace(w.Label), maxText), Options: []Option{}, Initial: clip(w.Initial, maxText)}
	switch {
	case c.ID == "" || c.Label == "":
		p.fail("a choice has an empty id or label")
		return Choice{}, false
	case len(w.Options) > maxOptions:
		p.fail("choice %q has too many options", c.ID)
		return Choice{}, false
	case len(w.Options) == 0:
		switch c.Initial {
		case "", "true", "false":
			return c, true
		}
		p.fail("choice %q has a non-boolean initial value", c.ID)
		return Choice{}, false
	}
	ids := map[string]bool{}
	for _, o := range w.Options {
		id, label := clip(o.ID, maxText), clip(strings.TrimSpace(o.Label), maxText)
		if id == "" || label == "" {
			p.fail("choice %q has an option with an empty id or label", c.ID)
			return Choice{}, false
		}
		ids[id] = true
		c.Options = append(c.Options, Option{ID: id, Label: label})
	}
	if c.Initial != "" && !ids[c.Initial] {
		p.fail("choice %q starts on an unknown option", c.ID)
		return Choice{}, false
	}
	return c, true
}

func (p *parser) files() []string {
	v, ok := p.options["files"]
	if !ok {
		return []string{}
	}
	var wire [][]byte
	if err := v.Store(&wire); err != nil {
		p.fail("option files must be aay")
		return []string{}
	}
	if len(wire) > maxFiles {
		p.fail("option files lists too many names")
		return []string{}
	}
	out := make([]string, 0, len(wire))
	for _, raw := range wire {
		name := sanitizedName(string(bytes.TrimRight(raw, "\x00")))
		if name == "" {
			p.fail("option files holds an invalid file name")
			return []string{}
		}
		out = append(out, name)
	}
	return out
}

func stripMnemonic(label string) string {
	var b strings.Builder
	runes := []rune(strings.TrimSpace(label))
	for i := 0; i < len(runes); i++ {
		if runes[i] != '_' {
			b.WriteRune(runes[i])
			continue
		}
		if i+1 < len(runes) && runes[i+1] == '_' {
			b.WriteRune('_')
			i++
		}
	}
	return b.String()
}

func clip(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max])
}

var errBadHandle = errors.New("handle is not a portal request path")

func sanitizedName(name string) string {
	if !pathutil.ValidName(name) {
		return ""
	}
	return name
}

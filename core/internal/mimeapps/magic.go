package mimeapps

import (
	"bytes"
	"encoding/binary"
	"os"
	"sort"
	"strconv"
	"strings"
)

const magicHeader = "MIME-Magic\x00\n"

type matchlet struct {
	offset   int
	rangeLen int
	value    []byte
	mask     []byte
	children []matchlet
}

type magicRule struct {
	priority  int
	mime      string
	matchlets []matchlet
}

func (db *DB) loadMagic(path string) {
	data, err := os.ReadFile(path)
	if err != nil || !bytes.HasPrefix(data, []byte(magicHeader)) {
		return
	}
	db.appendRules(parseMagic(data[len(magicHeader):]))
}

func (db *DB) appendRules(rules []magicRule) {
	db.magic = append(db.magic, rules...)
	for _, rule := range rules {
		db.maxExtent = max(db.maxExtent, ruleExtent(rule.matchlets))
	}
	sort.SliceStable(db.magic, func(i, j int) bool { return db.magic[i].priority > db.magic[j].priority })
}

func ruleExtent(matchlets []matchlet) int {
	extent := 0
	for _, m := range matchlets {
		extent = max(extent, m.offset+m.rangeLen+len(m.value))
		extent = max(extent, ruleExtent(m.children))
	}
	return extent
}

type magicParser struct {
	data []byte
	pos  int
}

func parseMagic(data []byte) []magicRule {
	p := &magicParser{data: data}
	var rules []magicRule

	for p.pos < len(p.data) {
		if p.data[p.pos] != '[' {
			p.skipLine()
			continue
		}
		priority, mime, ok := p.section()
		if !ok {
			break
		}
		flat, ok := p.matchlets()
		if !ok {
			break
		}
		if len(flat) > 0 {
			rules = append(rules, magicRule{priority: priority, mime: mime, matchlets: nest(flat, 0)})
		}
	}
	return rules
}

func (p *magicParser) skipLine() {
	if nl := bytes.IndexByte(p.data[p.pos:], '\n'); nl >= 0 {
		p.pos += nl + 1
		return
	}
	p.pos = len(p.data)
}

func (p *magicParser) section() (int, string, bool) {
	end := bytes.IndexByte(p.data[p.pos:], ']')
	if end < 0 {
		return 0, "", false
	}
	head := string(p.data[p.pos+1 : p.pos+end])
	p.pos += end + 1
	if p.pos < len(p.data) && p.data[p.pos] == '\n' {
		p.pos++
	}

	priorityText, mime, ok := strings.Cut(head, ":")
	if !ok {
		return 0, "", false
	}
	priority, err := strconv.Atoi(priorityText)
	if err != nil || priority < 0 {
		return 0, "", false
	}
	return priority, mime, true
}

type flatMatchlet struct {
	indent int
	m      matchlet
}

func (p *magicParser) matchlets() ([]flatMatchlet, bool) {
	var out []flatMatchlet
	for p.pos < len(p.data) && p.data[p.pos] != '[' {
		m, indent, ok := p.matchlet()
		if !ok {
			return out, false
		}
		out = append(out, flatMatchlet{indent: indent, m: m})
	}
	return out, true
}

func (p *magicParser) matchlet() (matchlet, int, bool) {
	indent := p.digits()
	if p.pos >= len(p.data) || p.data[p.pos] != '>' {
		return matchlet{}, 0, false
	}
	p.pos++

	m := matchlet{offset: p.digits()}
	if p.pos >= len(p.data) || p.data[p.pos] != '=' {
		return matchlet{}, 0, false
	}
	p.pos++

	if p.pos+2 > len(p.data) {
		return matchlet{}, 0, false
	}
	length := int(binary.BigEndian.Uint16(p.data[p.pos:]))
	p.pos += 2
	if p.pos+length > len(p.data) {
		return matchlet{}, 0, false
	}
	m.value = p.data[p.pos : p.pos+length]
	p.pos += length

	if p.pos < len(p.data) && p.data[p.pos] == '&' {
		p.pos++
		if p.pos+length > len(p.data) {
			return matchlet{}, 0, false
		}
		m.mask = p.data[p.pos : p.pos+length]
		p.pos += length
	}
	if p.pos < len(p.data) && p.data[p.pos] == '~' {
		p.pos++
		wordSize := p.digits()
		m.value = hostOrder(m.value, wordSize)
		m.mask = hostOrder(m.mask, wordSize)
	}
	if p.pos < len(p.data) && p.data[p.pos] == '+' {
		p.pos++
		m.rangeLen = p.digits()
	}
	if p.pos < len(p.data) && p.data[p.pos] == '\n' {
		p.pos++
	}
	return m, indent, true
}

func (p *magicParser) digits() int {
	value := 0
	for p.pos < len(p.data) && p.data[p.pos] >= '0' && p.data[p.pos] <= '9' {
		value = value*10 + int(p.data[p.pos]-'0')
		p.pos++
	}
	return value
}

func hostOrder(value []byte, wordSize int) []byte {
	if len(value) == 0 || wordSize < 2 || len(value)%wordSize != 0 || !littleEndian() {
		return value
	}
	out := make([]byte, len(value))
	for i := 0; i < len(value); i += wordSize {
		for j := 0; j < wordSize; j++ {
			out[i+j] = value[i+wordSize-1-j]
		}
	}
	return out
}

func littleEndian() bool {
	return binary.NativeEndian.Uint16([]byte{1, 0}) == 1
}

func nest(flat []flatMatchlet, indent int) []matchlet {
	var out []matchlet
	for i := 0; i < len(flat); i++ {
		if flat[i].indent != indent {
			continue
		}
		end := i + 1
		for end < len(flat) && flat[end].indent > indent {
			end++
		}
		m := flat[i].m
		m.children = nest(flat[i+1:end], indent+1)
		out = append(out, m)
		i = end - 1
	}
	return out
}

func (db *DB) sniffData(data []byte) (string, int) {
	for _, rule := range db.magic {
		if matchAny(rule.matchlets, data) {
			return rule.mime, rule.priority
		}
	}
	return "", 0
}

func matchAny(matchlets []matchlet, data []byte) bool {
	for i := range matchlets {
		m := &matchlets[i]
		if !m.matches(data) {
			continue
		}
		if len(m.children) == 0 || matchAny(m.children, data) {
			return true
		}
	}
	return false
}

func (m *matchlet) matches(data []byte) bool {
	if m.offset >= len(data) {
		return false
	}
	if m.rangeLen == 0 && m.mask == nil && len(m.value) > 0 {
		end := m.offset + len(m.value)
		return end <= len(data) && data[m.offset] == m.value[0] && bytes.Equal(data[m.offset:end], m.value)
	}
	window := data[m.offset:min(m.offset+m.rangeLen+len(m.value), len(data))]
	if len(window) < len(m.value) {
		return false
	}
	if m.mask == nil {
		return bytes.Contains(window, m.value)
	}
	for off := 0; off+len(m.value) <= len(window); off++ {
		if equalMasked(window[off:off+len(m.value)], m.value, m.mask) {
			return true
		}
	}
	return false
}

func equalMasked(got, want, mask []byte) bool {
	if mask == nil {
		return bytes.Equal(got, want)
	}
	for i := range want {
		if got[i]&mask[i] != want[i]&mask[i] {
			return false
		}
	}
	return true
}

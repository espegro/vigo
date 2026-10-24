// Package buffer stores the text being edited and the undo history.
package buffer

import (
	"bytes"
	"strings"
	"unicode/utf8"
)

// Pos is a position in the buffer. Line is a line index and Col is a rune
// index within that line.
type Pos struct {
	Line, Col int
}

// Before reports whether p comes before o.
func (p Pos) Before(o Pos) bool {
	if p.Line != o.Line {
		return p.Line < o.Line
	}
	return p.Col < o.Col
}

type snapshot struct {
	lines      [][]rune
	cur        Pos
	missingEOL bool
	seq        int
	bytes      int
}

// Buffer is the text of one file plus its undo history.
type Buffer struct {
	lines      [][]rune
	path       string
	eol        string
	missingEOL bool
	version    uint64
	seq        int
	savedSeq   int
	undo       []snapshot
	redo       []snapshot
	inBlock    bool
	UndoLimit  int
	readOnly   bool
}

// New returns an empty buffer.
func New() *Buffer {
	return &Buffer{lines: [][]rune{{}}, eol: "\n", missingEOL: true, UndoLimit: 1000}
}

// SetDefaultEOL sets the line ending used for new content.
func (b *Buffer) SetDefaultEOL(eol string) {
	if eol != "" {
		b.eol = eol
	}
}

// FromString builds a buffer from text.
func FromString(s string) *Buffer {
	return FromBytes([]byte(s))
}

// FromBytes builds a buffer from raw file contents, detecting the line ending.
func FromBytes(data []byte) *Buffer {
	b := New()
	b.eol = "\n"
	if bytes.Contains(data, []byte("\r\n")) {
		b.eol = "\r\n"
	}
	norm := strings.ReplaceAll(string(data), "\r\n", "\n")
	norm = strings.ReplaceAll(norm, "\r", "\n")
	if norm == "" {
		b.lines = [][]rune{{}}
		b.missingEOL = true
		return b
	}
	b.missingEOL = !strings.HasSuffix(norm, "\n")
	norm = strings.TrimSuffix(norm, "\n")
	parts := strings.Split(norm, "\n")
	lines := make([][]rune, len(parts))
	for i, p := range parts {
		lines[i] = []rune(p)
	}
	b.lines = lines
	return b
}

// Path returns the file name associated with the buffer.
func (b *Buffer) Path() string { return b.path }

// SetPath changes the file name associated with the buffer.
func (b *Buffer) SetPath(p string) { b.path = p }

// Modified reports whether the buffer has unsaved changes.
func (b *Buffer) Modified() bool { return b.seq != b.savedSeq }

// MarkSaved records that the buffer on disk matches the buffer in memory.
func (b *Buffer) MarkSaved() { b.savedSeq = b.seq }

// MarkModified forces the buffer into the modified state.
func (b *Buffer) MarkModified() {
	if b.seq == b.savedSeq {
		b.seq++
	}
}

// ReadOnly reports whether writes to the file are refused.
func (b *Buffer) ReadOnly() bool { return b.readOnly }

// SetReadOnly sets the read only flag.
func (b *Buffer) SetReadOnly(v bool) { b.readOnly = v }

// EOL returns the line ending used when saving.
func (b *Buffer) EOL() string { return b.eol }

// SetEOL sets the line ending used when saving.
func (b *Buffer) SetEOL(eol string) { b.eol = eol }

// MissingEOL reports whether the last line lacks a trailing line ending.
func (b *Buffer) MissingEOL() bool { return b.missingEOL }

// SetMissingEOL sets whether the last line is unterminated.
func (b *Buffer) SetMissingEOL(v bool) { b.missingEOL = v }

// Version increases on every change and is used to invalidate caches.
func (b *Buffer) Version() uint64 { return b.version }

// Len returns the number of lines.
func (b *Buffer) Len() int { return len(b.lines) }

// Line returns the runes of line i. The slice is owned by the buffer and must
// not be modified by the caller.
func (b *Buffer) Line(i int) []rune {
	if i < 0 || i >= len(b.lines) {
		return nil
	}
	return b.lines[i]
}

// LineString returns line i as a string.
func (b *Buffer) LineString(i int) string { return string(b.Line(i)) }

// LineLen returns the number of runes in line i.
func (b *Buffer) LineLen(i int) int { return len(b.Line(i)) }

// LastLine returns the index of the final line.
func (b *Buffer) LastLine() int { return len(b.lines) - 1 }

// RuneAt returns the rune at p, or 0 when p is out of range.
func (b *Buffer) RuneAt(p Pos) rune {
	line := b.Line(p.Line)
	if p.Col < 0 || p.Col >= len(line) {
		return 0
	}
	return line[p.Col]
}

// Clamp constrains p to a valid insert position.
func (b *Buffer) Clamp(p Pos) Pos {
	if p.Line < 0 {
		p.Line = 0
	}
	if p.Line > b.LastLine() {
		p.Line = b.LastLine()
	}
	if p.Col < 0 {
		p.Col = 0
	}
	if n := b.LineLen(p.Line); p.Col > n {
		p.Col = n
	}
	return p
}

// ClampNormal constrains p to a valid normal mode position, where the cursor
// rests on a character unless the line is empty.
func (b *Buffer) ClampNormal(p Pos) Pos {
	p = b.Clamp(p)
	if n := b.LineLen(p.Line); n > 0 && p.Col >= n {
		p.Col = n - 1
	} else if n == 0 {
		p.Col = 0
	}
	return p
}

// EndOfLine returns the position after the last rune of line i.
func (b *Buffer) EndOfLine(i int) Pos { return Pos{Line: i, Col: b.LineLen(i)} }

// LastCharPos returns the position of the final character of line i.
func (b *Buffer) LastCharPos(i int) Pos {
	n := b.LineLen(i)
	if n == 0 {
		return Pos{Line: i, Col: 0}
	}
	return Pos{Line: i, Col: n - 1}
}

// FirstNonBlank returns the position of the first non blank rune of line i.
func (b *Buffer) FirstNonBlank(i int) Pos {
	for col, r := range b.Line(i) {
		if r != ' ' && r != '\t' {
			return Pos{Line: i, Col: col}
		}
	}
	return Pos{Line: i, Col: 0}
}

// String renders the buffer as it would be saved.
func (b *Buffer) String() string { return string(b.Bytes()) }

// Bytes renders the buffer contents using the buffer's line ending.
func (b *Buffer) Bytes() []byte {
	var buf bytes.Buffer
	for i, line := range b.lines {
		buf.WriteString(string(line))
		if i < len(b.lines)-1 {
			buf.WriteString(b.eol)
		}
	}
	if !b.missingEOL && len(b.lines) > 0 {
		buf.WriteString(b.eol)
	}
	return buf.Bytes()
}

// SetContents replaces all text without touching the undo history.
func (b *Buffer) SetContents(data []byte) {
	tmp := FromBytes(data)
	b.lines = tmp.lines
	b.eol = tmp.eol
	b.missingEOL = tmp.missingEOL
	b.version++
	b.savedSeq = b.seq
}

// Insert inserts runes at p and returns the position after the inserted text.
func (b *Buffer) Insert(p Pos, runes []rune) Pos {
	if len(runes) == 0 {
		return p
	}
	p = b.Clamp(p)
	line := b.Line(p.Line)
	rest := append([]rune{}, line[p.Col:]...)
	newLine := append(line[:p.Col:p.Col], runes...)
	b.lines[p.Line] = append(newLine, rest...)
	b.touch()
	return Pos{Line: p.Line, Col: p.Col + len(runes)}
}

// InsertText inserts text that may contain newlines.
func (b *Buffer) InsertText(p Pos, text string) Pos {
	if !strings.Contains(text, "\n") {
		return b.Insert(p, []rune(text))
	}
	parts := strings.Split(text, "\n")
	pos := b.Insert(p, []rune(parts[0]))
	for _, part := range parts[1:] {
		pos = b.SplitLine(pos)
		pos = b.Insert(pos, []rune(part))
	}
	return pos
}

// SplitLine inserts a newline at p and returns the position of the new line.
func (b *Buffer) SplitLine(p Pos) Pos {
	p = b.Clamp(p)
	line := b.Line(p.Line)
	left := append([]rune{}, line[:p.Col]...)
	right := append([]rune{}, line[p.Col:]...)
	b.lines[p.Line] = left
	b.insertLineAt(p.Line+1, right)
	b.touch()
	return Pos{Line: p.Line + 1, Col: 0}
}

// JoinLines joins line i and i+1 inserting spaces between them.
func (b *Buffer) JoinLines(i int, spaces bool) {
	if i < 0 || i >= b.LastLine() {
		return
	}
	cur := b.Line(i)
	next := b.Line(i + 1)
	trimmed := append([]rune{}, next...)
	for len(trimmed) > 0 && (trimmed[0] == ' ' || trimmed[0] == '\t') {
		trimmed = trimmed[1:]
	}
	joined := append(append([]rune{}, cur...), trimmed...)
	if spaces && len(cur) > 0 && len(joined) > len(cur) {
		last := cur[len(cur)-1]
		if last != ' ' && last != '\t' {
			insert := []rune{' '}
			if i < b.LastLine() {
				joined = append(append(append([]rune{}, cur...), insert...), trimmed...)
			}
		}
	}
	b.lines[i] = joined
	b.lines = append(b.lines[:i+1], b.lines[i+2:]...)
	b.touch()
}

// SetLine replaces line i.
func (b *Buffer) SetLine(i int, runes []rune) {
	if i < 0 || i >= b.Len() {
		return
	}
	b.lines[i] = runes
	b.touch()
}

func (b *Buffer) insertLineAt(i int, runes []rune) {
	if i < 0 {
		i = 0
	}
	if i > len(b.lines) {
		i = len(b.lines)
	}
	b.lines = append(b.lines, nil)
	copy(b.lines[i+1:], b.lines[i:])
	b.lines[i] = runes
}

// InsertLines inserts lines before line index at.
func (b *Buffer) InsertLines(at int, lines [][]rune) {
	if len(lines) == 0 {
		return
	}
	if at < 0 {
		at = 0
	}
	if at > len(b.lines) {
		at = len(b.lines)
	}
	out := make([][]rune, 0, len(b.lines)+len(lines))
	out = append(out, b.lines[:at]...)
	out = append(out, lines...)
	out = append(out, b.lines[at:]...)
	b.lines = out
	b.touch()
}

// DeleteRange removes the runes between a and b, which must be ordered. When
// the range spans lines the lines in between are removed. The removed text is
// returned split per line.
func (b *Buffer) DeleteRange(a, c Pos) [][]rune {
	a, c = b.Clamp(a), b.Clamp(c)
	if c.Before(a) {
		a, c = c, a
	}
	if a == c {
		return nil
	}
	if a.Line == c.Line {
		line := b.Line(a.Line)
		removed := append([]rune{}, line[a.Col:c.Col]...)
		b.lines[a.Line] = append(append([]rune{}, line[:a.Col]...), line[c.Col:]...)
		b.touch()
		return [][]rune{removed}
	}
	first := b.Line(a.Line)
	last := b.Line(c.Line)
	removed := make([][]rune, 0, c.Line-a.Line+1)
	removed = append(removed, append([]rune{}, first[a.Col:]...))
	for i := a.Line + 1; i < c.Line; i++ {
		removed = append(removed, append([]rune{}, b.Line(i)...))
	}
	removed = append(removed, append([]rune{}, last[:c.Col]...))
	merged := append(append([]rune{}, first[:a.Col]...), last[c.Col:]...)
	b.lines[a.Line] = merged
	b.lines = append(b.lines[:a.Line+1], b.lines[c.Line+1:]...)
	b.touch()
	return removed
}

// DeleteLines removes lines from..to inclusive.
func (b *Buffer) DeleteLines(from, to int) [][]rune {
	if from < 0 {
		from = 0
	}
	if to > b.LastLine() {
		to = b.LastLine()
	}
	if from > to {
		return nil
	}
	removed := make([][]rune, 0, to-from+1)
	for i := from; i <= to; i++ {
		removed = append(removed, append([]rune{}, b.Line(i)...))
	}
	b.lines = append(b.lines[:from], b.lines[to+1:]...)
	if len(b.lines) == 0 {
		b.lines = [][]rune{{}}
	}
	b.touch()
	return removed
}

func (b *Buffer) touch() {
	b.version++
	b.seq++
}

func indexRunes(haystack, needle []rune, from int) int {
	if from < 0 {
		from = 0
	}
	if from > len(haystack) {
		return -1
	}
	sub := haystack[from:]
	idx := strings.Index(string(sub), string(needle))
	if idx < 0 {
		return -1
	}
	return from + utf8.RuneCountInString(string(sub[:idx]))
}

func lastIndexRunes(haystack, needle []rune, limit int) int {
	if limit > len(haystack) {
		limit = len(haystack)
	}
	sub := string(haystack[:limit])
	idx := strings.LastIndex(sub, string(needle))
	if idx < 0 {
		return -1
	}
	return len([]rune(sub[:idx]))
}

// ReplaceLines replaces the lines from..to inclusive with new content. Unlike
// DeleteLines followed by InsertLines it keeps the buffer at one line when
// everything is replaced.
func (b *Buffer) ReplaceLines(from, to int, lines [][]rune) {
	from = clampInt(from, 0, b.LastLine())
	to = clampInt(to, from, b.LastLine())
	out := make([][]rune, 0, len(b.lines)-(to-from+1)+len(lines))
	out = append(out, b.lines[:from]...)
	out = append(out, lines...)
	if to+1 <= len(b.lines) {
		out = append(out, b.lines[to+1:]...)
	}
	if len(out) == 0 {
		out = [][]rune{{}}
	}
	b.lines = out
	b.touch()
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

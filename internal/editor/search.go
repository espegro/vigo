package editor

import (
	"regexp"
	"unicode"
	"unicode/utf8"

	"vigo/internal/buffer"
)

// match is one highlighted search hit.
type match struct {
	line       int
	start, end int
}

func runeIndex(s string, byteIdx int) int {
	return utf8.RuneCountInString(s[:byteIdx])
}

func byteIndex(s string, runeIdx int) int {
	if runeIdx <= 0 {
		return 0
	}
	n := 0
	for i := range s {
		if n == runeIdx {
			return i
		}
		n++
	}
	return len(s)
}

// compileSearch compiles a vim pattern honouring ignorecase and smartcase.
func (e *Editor) compileSearch(pat string) (*regexp.Regexp, error) {
	ignoreCase := e.opts.IgnoreCase
	if e.opts.SmartCase && patternHasUpper(pat) {
		ignoreCase = false
	}
	expr, err := patternToRegexp(pat, ignoreCase)
	if err != nil {
		return nil, err
	}
	return regexp.Compile(expr)
}

func patternHasUpper(pat string) bool {
	escaped := false
	for _, r := range pat {
		if escaped {
			escaped = false
			continue
		}
		if r == '\\' {
			escaped = true
			continue
		}
		if unicode.IsUpper(r) {
			return true
		}
	}
	return false
}

// findMatch looks for the next or previous match of re starting at from.
func (e *Editor) findMatch(re *regexp.Regexp, from buffer.Pos, forward bool) (buffer.Pos, bool) {
	if forward {
		for i := clamp(from.Line, 0, e.buf.LastLine()); i <= e.buf.LastLine(); i++ {
			s := e.buf.LineString(i)
			locs := re.FindAllStringSubmatchIndex(s, -1)
			for _, l := range locs {
				col := runeIndex(s, l[0])
				if i == from.Line && col < from.Col {
					continue
				}
				return buffer.Pos{Line: i, Col: col}, true
			}
		}
		return buffer.Pos{}, false
	}
	for i := clamp(from.Line, 0, e.buf.LastLine()); i >= 0; i-- {
		s := e.buf.LineString(i)
		locs := re.FindAllStringSubmatchIndex(s, -1)
		for j := len(locs) - 1; j >= 0; j-- {
			col := runeIndex(s, locs[j][0])
			if i == from.Line && col >= from.Col {
				continue
			}
			return buffer.Pos{Line: i, Col: col}, true
		}
	}
	return buffer.Pos{}, false
}

// searchFrom searches for count matches from a starting position.
func (e *Editor) searchFrom(re *regexp.Regexp, from buffer.Pos, forward bool, count int) (buffer.Pos, bool) {
	pos := from
	var last buffer.Pos
	found := false
	for i := 0; i < count; i++ {
		p, ok := e.findMatch(re, pos, forward)
		if !ok {
			if forward {
				p, ok = e.findMatch(re, buffer.Pos{Line: 0, Col: 0}, true)
			} else {
				p, ok = e.findMatch(re, buffer.Pos{Line: e.buf.LastLine(), Col: 1 << 20}, false)
			}
			if !ok {
				return buffer.Pos{}, false
			}
		}
		last = p
		found = true
		if forward {
			pos = buffer.Pos{Line: p.Line, Col: p.Col + 1}
		} else if p.Col > 0 {
			pos = buffer.Pos{Line: p.Line, Col: p.Col - 1}
		} else if p.Line > 0 {
			pos = buffer.Pos{Line: p.Line - 1, Col: 1 << 20}
		} else {
			return buffer.Pos{}, false
		}
	}
	if !found {
		return buffer.Pos{}, false
	}
	return last, true
}

// doSearch moves the cursor to the next match of pat, in direction dir.
func (e *Editor) doSearch(pat string, forward bool, count int) (buffer.Pos, bool) {
	re, err := e.compileSearch(pat)
	if err != nil {
		e.errorf("E383: Invalid search string: %v", err)
		return buffer.Pos{}, false
	}
	from := e.cur
	if forward {
		from.Col++
	} else if from.Col > 0 {
		from.Col--
	} else {
		from.Line--
		from.Col = 1 << 20
	}
	return e.searchFrom(re, from, forward, count)
}

// searchNext implements n and N.
func (e *Editor) searchNext(dir int, count int) (buffer.Pos, bool) {
	if e.lastSearch == "" {
		e.errorf("E35: No previous regular expression")
		return buffer.Pos{}, false
	}
	forward := dir > 0
	if e.lastSearchDir < 0 {
		forward = !forward
	}
	return e.doSearch(e.lastSearch, forward, count)
}

// searchWordUnderCursor implements * and #.
func (e *Editor) searchWordUnderCursor(forward bool, count int) (buffer.Pos, bool) {
	word := e.wordUnderCursor()
	if word == "" {
		return buffer.Pos{}, false
	}
	pat := `\<` + regexp.QuoteMeta(word) + `\>`
	e.lastSearch = pat
	if forward {
		e.lastSearchDir = 1
	} else {
		e.lastSearchDir = -1
	}
	return e.doSearch(pat, forward, count)
}

func (e *Editor) wordUnderCursor() string {
	line := e.buf.Line(e.cur.Line)
	if e.cur.Col >= len(line) {
		return ""
	}
	start, end := e.cur.Col, e.cur.Col
	for start > 0 && charClass(line[start-1], false) == cKeyword {
		start--
	}
	for end < len(line) && charClass(line[end], false) == cKeyword {
		end++
	}
	return string(line[start:end])
}

// execSearch runs a search typed on the command line.
func (e *Editor) execSearch(pat string, forward bool) {
	if pat == "" {
		pat = e.lastSearch
	} else {
		e.lastSearch = pat
	}
	if pat == "" {
		e.errorf("E35: No previous regular expression")
		return
	}
	if forward {
		e.lastSearchDir = 1
	} else {
		e.lastSearchDir = -1
	}
	e.searchHistoryAdd(pat)
	c := e.cache()
	c.hlVersion = 0
	c.hlOff = false
	e.pushJump(e.cur)
	dest, ok := e.doSearch(pat, forward, e.count())
	if !ok {
		e.errorf("E486: Pattern not found: %s", pat)
		return
	}
	e.setCursor(dest)
	e.desiredCol = dest.Col
}

// updateSearchHighlight recomputes the highlight matches when needed.
func (e *Editor) updateSearchHighlight() {
	c := e.cache()
	if c.hlOff || e.lastSearch == "" || !e.opts.HlSearch {
		c.hlMatches = nil
		return
	}
	version := e.buf.Version()*31 + uint64(len(e.lastSearch)) + boolToUint(e.opts.IgnoreCase)
	if c.hlVersion == version {
		return
	}
	c.hlVersion = version
	re, err := e.compileSearch(e.lastSearch)
	if err != nil {
		c.hlMatches = nil
		return
	}
	var matches []match
	const limit = 2000
	for i := 0; i <= e.buf.LastLine() && len(matches) < limit; i++ {
		s := e.buf.LineString(i)
		for _, l := range re.FindAllStringSubmatchIndex(s, -1) {
			start, end := runeIndex(s, l[0]), runeIndex(s, l[1])
			if end <= start {
				end = start + 1
			}
			matches = append(matches, match{line: i, start: start, end: end})
			if len(matches) >= limit {
				break
			}
		}
	}
	c.hlMatches = matches
}

// searchRangeIn returns the highlight spans on one line.
func (e *Editor) searchRangeIn(line int) []match {
	var out []match
	for _, m := range e.cache().hlMatches {
		if m.line == line {
			out = append(out, m)
		}
		if m.line > line {
			break
		}
	}
	return out
}

func (e *Editor) searchHistoryAdd(pat string) {
	if len(e.searchHistory) == 0 || e.searchHistory[len(e.searchHistory)-1] != pat {
		e.searchHistory = append(e.searchHistory, pat)
	}
	if len(e.searchHistory) > 100 {
		e.searchHistory = e.searchHistory[1:]
	}
}

// previewSearch shows the first match while the user types an incremental
// search, and returns whether anything matched.
func (e *Editor) previewSearch(pat string) bool {
	if !e.opts.IncSearch || pat == "" {
		return false
	}
	re, err := e.compileSearch(pat)
	if err != nil {
		return false
	}
	forward := e.cmd.mode == ModeSearchForward
	from := e.cmd.searchOrigin
	if forward {
		from.Col++
	} else if from.Col > 0 {
		from.Col--
	} else {
		from.Line--
		from.Col = 1 << 20
	}
	p, ok := e.searchFrom(re, from, forward, 1)
	if !ok {
		return false
	}
	e.setCursor(p)
	return true
}

func boolToUint(v bool) uint64 {
	if v {
		return 1
	}
	return 0
}

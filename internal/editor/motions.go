package editor

import (
	"strings"

	"vigo/internal/buffer"
)

type motionKind uint8

const (
	mExclusive motionKind = iota
	mInclusive
	mLine
)

// rangeInfo is a half open rune range plus how the operator range should be
// interpreted.
type rangeInfo struct {
	from, to buffer.Pos
	kind     motionKind
	block    bool
}

func (r rangeInfo) empty() bool { return r.from == r.to }

// motionRes is the outcome of a motion: where the cursor goes and which range
// an operator would act on.
type motionRes struct {
	dest buffer.Pos
	r    rangeInfo
	ok   bool
}

func rangeOf(a, b buffer.Pos, kind motionKind) rangeInfo {
	if b.Before(a) {
		a, b = b, a
	}
	return rangeInfo{from: a, to: b, kind: kind}
}

func lineRange(a, b buffer.Pos) rangeInfo {
	if b.Line < a.Line {
		a, b = b, a
	}
	return rangeInfo{from: buffer.Pos{Line: a.Line}, to: buffer.Pos{Line: b.Line + 1}, kind: mLine}
}

// Character classes used by the word motions.
const (
	cWhite = iota
	cKeyword
	cPunct
)

func charClass(r rune, big bool) int {
	switch {
	case r == 0, r == ' ', r == '\t':
		return cWhite
	case r == '_' || r >= '0' && r <= '9' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z':
		return cKeyword
	case r > 0x7f:
		return cKeyword
	case big:
		return cPunct
	}
	return cPunct
}

func (e *Editor) classAt(p buffer.Pos, big bool) int {
	if p.Line >= e.buf.Len() {
		return cWhite
	}
	if p.Col >= e.buf.LineLen(p.Line) {
		return cWhite
	}
	return charClass(e.buf.RuneAt(p), big)
}

func (e *Editor) atBufferEnd(p buffer.Pos) bool {
	return p.Line >= e.buf.Len()-1 && p.Col >= e.buf.LineLen(p.Line)
}

func (e *Editor) advance(p buffer.Pos) buffer.Pos {
	if p.Col < e.buf.LineLen(p.Line) {
		return buffer.Pos{Line: p.Line, Col: p.Col + 1}
	}
	if p.Line < e.buf.LastLine() {
		return buffer.Pos{Line: p.Line + 1, Col: 0}
	}
	return p
}

func (e *Editor) retreat(p buffer.Pos) buffer.Pos {
	if p.Col > 0 {
		return buffer.Pos{Line: p.Line, Col: p.Col - 1}
	}
	if p.Line > 0 {
		return buffer.Pos{Line: p.Line - 1, Col: e.buf.LineLen(p.Line - 1)}
	}
	return p
}

func (e *Editor) wordForward(p buffer.Pos, count int, big bool) buffer.Pos {
	for i := 0; i < count; i++ {
		if e.atBufferEnd(p) {
			break
		}
		cls := e.classAt(p, big)
		if cls != cWhite {
			for !e.atBufferEnd(p) && e.classAt(p, big) == cls {
				p = e.advance(p)
			}
		}
		for !e.atBufferEnd(p) && e.classAt(p, big) == cWhite {
			p = e.advance(p)
		}
	}
	return p
}

func (e *Editor) wordBackward(p buffer.Pos, count int, big bool) buffer.Pos {
	for i := 0; i < count; i++ {
		if p.Line == 0 && p.Col == 0 {
			break
		}
		p = e.retreat(p)
		for !(p.Line == 0 && p.Col == 0) && e.classAt(p, big) == cWhite {
			p = e.retreat(p)
		}
		cls := e.classAt(p, big)
		for {
			prev := e.retreat(p)
			if prev == p || e.classAt(prev, big) != cls {
				break
			}
			p = prev
		}
	}
	return p
}

func (e *Editor) wordEnd(p buffer.Pos, count int, big bool) buffer.Pos {
	for i := 0; i < count; i++ {
		if e.atBufferEnd(p) {
			break
		}
		q := e.advance(p)
		for !e.atBufferEnd(q) && e.classAt(q, big) == cWhite {
			q = e.advance(q)
		}
		cls := e.classAt(q, big)
		for {
			n := e.advance(q)
			if n == q || e.classAt(n, big) != cls {
				break
			}
			q = n
		}
		p = q
		if e.atBufferEnd(p) {
			p = e.buf.LastCharPos(e.buf.LastLine())
			break
		}
	}
	return p
}

func (e *Editor) paragraphForward(p buffer.Pos, count int) buffer.Pos {
	line := p.Line
	for i := 0; i < count; i++ {
		line++
		for line <= e.buf.LastLine() && strings.TrimSpace(e.buf.LineString(line)) != "" {
			line++
		}
	}
	if line > e.buf.LastLine() {
		line = e.buf.LastLine()
		for line > 0 && strings.TrimSpace(e.buf.LineString(line)) == "" {
			line--
		}
	}
	return buffer.Pos{Line: line}
}

func (e *Editor) paragraphBackward(p buffer.Pos, count int) buffer.Pos {
	line := p.Line
	for i := 0; i < count; i++ {
		if line == 0 {
			break
		}
		line--
		for line > 0 && strings.TrimSpace(e.buf.LineString(line)) != "" {
			line--
		}
	}
	return buffer.Pos{Line: line}
}

func (e *Editor) sentenceForward(p buffer.Pos, count int) buffer.Pos {
	for i := 0; i < count; i++ {
		found := false
		for !e.atBufferEnd(p) {
			r := e.buf.RuneAt(p)
			p = e.advance(p)
			if r == '.' || r == '!' || r == '?' {
				for !e.atBufferEnd(p) && e.classAt(p, false) == cWhite {
					p = e.advance(p)
				}
				if p.Col < e.buf.LineLen(p.Line) {
					found = true
					break
				}
			}
		}
		if !found {
			break
		}
	}
	return p
}

func (e *Editor) sentenceBackward(p buffer.Pos, count int) buffer.Pos {
	for i := 0; i < count; i++ {
		q := e.retreat(p)
		for q.Line > 0 || q.Col > 0 {
			r := e.buf.RuneAt(q)
			if (r == '.' || r == '!' || r == '?') && q.Col+1 < e.buf.LineLen(q.Line) {
				n := buffer.Pos{Line: q.Line, Col: q.Col + 1}
				if charClass(e.buf.RuneAt(n), false) == cWhite {
					p = n
					for p.Col < e.buf.LineLen(p.Line) && e.classAt(p, false) == cWhite {
						p = e.advance(p)
					}
					break
				}
			}
			q = e.retreat(q)
			if q.Line == 0 && q.Col == 0 {
				p = q
			}
		}
	}
	return p
}

var bracketPairs = map[rune]rune{'(': ')', '[': ']', '{': '}', ')': '(', ']': '[', '}': '{'}

func (e *Editor) matchBracket(p buffer.Pos) (buffer.Pos, bool) {
	line := []rune(e.buf.LineString(p.Line))
	col := -1
	for i := p.Col; i < len(line); i++ {
		if _, ok := bracketPairs[line[i]]; ok {
			col = i
			break
		}
	}
	if col < 0 {
		return p, false
	}
	open := line[col]
	close_, isOpen := bracketPairs[open]
	if isOpen {
		if open == ')' || open == ']' || open == '}' {
			isOpen = false
			close_ = bracketPairs[open]
		}
	} else {
		isOpen = true
	}
	depth := 0
	pos := buffer.Pos{Line: p.Line, Col: col}
	if isOpen {
		fwd := close_
		for {
			r := e.buf.RuneAt(pos)
			if r == open {
				depth++
			} else if r == fwd {
				depth--
				if depth == 0 {
					return pos, true
				}
			}
			if e.atBufferEnd(pos) {
				return p, false
			}
			pos = e.advance(pos)
		}
	}
	back := close_
	for {
		r := e.buf.RuneAt(pos)
		if r == open {
			depth++
		} else if r == back {
			depth--
			if depth == 0 {
				return pos, true
			}
		}
		if pos.Line == 0 && pos.Col == 0 {
			return p, false
		}
		pos = e.retreat(pos)
	}
}

// findState remembers the last f/t/F/T search so ; and , can repeat it.
type findState struct {
	kind rune
	char rune
	set  bool
}

func (e *Editor) findMotion(p buffer.Pos, kind rune, ch rune, count int) (buffer.Pos, bool) {
	line := []rune(e.buf.LineString(p.Line))
	if len(line) == 0 {
		return p, false
	}
	pos := p.Col
	for i := 0; i < count; i++ {
		found := -1
		switch kind {
		case 'f', 't':
			for c := pos + 1; c < len(line); c++ {
				if line[c] == ch {
					found = c
					break
				}
			}
		case 'F', 'T':
			for c := pos - 1; c >= 0; c-- {
				if line[c] == ch {
					found = c
					break
				}
			}
		}
		if found < 0 {
			return p, false
		}
		pos = found
		if kind == 't' {
			pos = found - 1
		} else if kind == 'T' {
			pos = found + 1
		}
	}
	if pos == p.Col || pos < 0 || pos >= len(line) {
		return p, false
	}
	return buffer.Pos{Line: p.Line, Col: pos}, true
}

// motion resolves one motion key. n is the count, countSet tells whether the
// user typed an explicit count (gg and G need that distinction).
func (e *Editor) motion(name string, n int, countSet bool) motionRes {
	p := e.cur
	excl := func(dest buffer.Pos) motionRes {
		return motionRes{dest: dest, r: rangeOf(p, dest, mExclusive), ok: true}
	}
	incl := func(dest buffer.Pos) motionRes {
		return motionRes{dest: dest, r: rangeInfo{from: p, to: dest, kind: mInclusive}, ok: true}
	}
	linewise := func(dest buffer.Pos) motionRes {
		return motionRes{dest: dest, r: lineRange(p, dest), ok: true}
	}
	switch name {
	case "h":
		return excl(e.clampCol(p, p.Col-n))
	case "l", " ":
		return excl(e.clampCol(p, p.Col+n))
	case "backspace":
		return excl(e.clampCol(p, p.Col-n))
	case "j":
		d := e.vertical(p, n)
		return linewise(d)
	case "k":
		d := e.vertical(p, -n)
		return linewise(d)
	case "cr":
		d := e.vertical(p, n)
		return linewise(buffer.Pos{Line: d.Line, Col: e.buf.FirstNonBlank(d.Line).Col})
	case "-":
		d := e.vertical(p, -n)
		return linewise(buffer.Pos{Line: d.Line, Col: e.buf.FirstNonBlank(d.Line).Col})
	case "+":
		d := e.vertical(p, n)
		return linewise(buffer.Pos{Line: d.Line, Col: e.buf.FirstNonBlank(d.Line).Col})
	case "_":
		d := e.vertical(p, n-1)
		return linewise(buffer.Pos{Line: d.Line, Col: e.buf.FirstNonBlank(d.Line).Col})
	case "0":
		return excl(buffer.Pos{Line: p.Line, Col: 0})
	case "^":
		return excl(e.buf.FirstNonBlank(p.Line))
	case "$":
		last := e.buf.LastCharPos(p.Line)
		if e.buf.LineLen(p.Line) == 0 {
			return motionRes{dest: last, r: rangeInfo{from: p, to: last, kind: mExclusive}, ok: true}
		}
		return motionRes{dest: last, r: rangeInfo{from: p, to: last, kind: mInclusive}, ok: true}
	case "w":
		return excl(e.wordForward(p, n, false))
	case "W":
		return excl(e.wordForward(p, n, true))
	case "b":
		return excl(e.wordBackward(p, n, false))
	case "B":
		return excl(e.wordBackward(p, n, true))
	case "e":
		return incl(e.wordEnd(p, n, false))
	case "E":
		return incl(e.wordEnd(p, n, true))
	case "}":
		return linewise(e.paragraphForward(p, n))
	case "{":
		return linewise(e.paragraphBackward(p, n))
	case ")":
		return excl(e.sentenceForward(p, n))
	case "(":
		return excl(e.sentenceBackward(p, n))
	case "gg":
		line := 0
		if countSet {
			line = n - 1
		}
		if line > e.buf.LastLine() {
			line = e.buf.LastLine()
		}
		d := buffer.Pos{Line: line, Col: e.buf.FirstNonBlank(line).Col}
		return linewise(d)
	case "G":
		line := e.buf.LastLine()
		if countSet {
			line = n - 1
		}
		if line > e.buf.LastLine() {
			line = e.buf.LastLine()
		}
		d := buffer.Pos{Line: line, Col: e.buf.FirstNonBlank(line).Col}
		return linewise(d)
	case "|":
		return excl(e.clampCol(p, n-1))
	case "%":
		if dest, ok := e.matchBracket(p); ok {
			return motionRes{dest: dest, r: rangeInfo{from: p, to: dest, kind: mInclusive}, ok: true}
		}
		return motionRes{ok: false}
	case "f", "F", "t", "T":
		if !e.lastFind.set {
			return motionRes{ok: false}
		}
		dest, ok := e.findMotion(p, rune(name[0]), e.lastFind.char, n)
		if !ok {
			return motionRes{ok: false}
		}
		if name == "f" || name == "t" {
			return incl(dest)
		}
		return excl(dest)
	case ";":
		if !e.lastFind.set {
			return motionRes{ok: false}
		}
		dest, ok := e.findMotion(p, e.lastFind.kind, e.lastFind.char, n)
		if !ok {
			return motionRes{ok: false}
		}
		if e.lastFind.kind == 'f' || e.lastFind.kind == 't' {
			return incl(dest)
		}
		return excl(dest)
	case ",":
		if !e.lastFind.set {
			return motionRes{ok: false}
		}
		k := e.lastFind.kind
		flip := map[rune]rune{'f': 'F', 'F': 'f', 't': 'T', 'T': 't'}[k]
		dest, ok := e.findMotion(p, flip, e.lastFind.char, n)
		if !ok {
			return motionRes{ok: false}
		}
		if flip == 'f' || flip == 't' {
			return incl(dest)
		}
		return excl(dest)
	case "H":
		e.computeRows()
		line := e.cur.Line
		if len(e.rows) > 0 {
			line = e.rows[0].line
		}
		return linewise(buffer.Pos{Line: line, Col: e.buf.FirstNonBlank(line).Col})
	case "M":
		e.computeRows()
		line := e.cur.Line
		if len(e.rows) > 0 {
			line = e.rows[len(e.rows)/2].line
		}
		return linewise(buffer.Pos{Line: line, Col: e.buf.FirstNonBlank(line).Col})
	case "L":
		e.computeRows()
		line := e.cur.Line
		if len(e.rows) > 0 {
			line = e.rows[len(e.rows)-1].line
		}
		return linewise(buffer.Pos{Line: line, Col: e.buf.FirstNonBlank(line).Col})
	case "n":
		dest, ok := e.searchNext(1, n)
		if !ok {
			return motionRes{ok: false}
		}
		return excl(dest)
	case "N":
		dest, ok := e.searchNext(-1, n)
		if !ok {
			return motionRes{ok: false}
		}
		return excl(dest)
	case "*", "#":
		dest, ok := e.searchWordUnderCursor(name == "*", n)
		if !ok {
			return motionRes{ok: false}
		}
		return excl(dest)
	}
	return motionRes{ok: false}
}

func (e *Editor) clampCol(p buffer.Pos, col int) buffer.Pos {
	if col < 0 {
		col = 0
	}
	if n := e.buf.LineLen(p.Line); col > n {
		col = n
	}
	return buffer.Pos{Line: p.Line, Col: col}
}

// vertical moves n lines up or down, keeping the desired column.
func (e *Editor) vertical(p buffer.Pos, n int) buffer.Pos {
	line := p.Line + n
	if line < 0 {
		line = 0
	}
	if line > e.buf.LastLine() {
		line = e.buf.LastLine()
	}
	col := e.desiredCol
	maxCol := e.buf.LineLen(line)
	if maxCol > 0 {
		maxCol--
	}
	if col > maxCol {
		col = maxCol
	}
	if col < 0 {
		col = 0
	}
	return buffer.Pos{Line: line, Col: col}
}

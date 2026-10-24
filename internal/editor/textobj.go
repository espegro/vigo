package editor

import (
	"strings"

	"vigo/internal/buffer"
	"vigo/internal/keys"
)

// textObject returns the range for an i/a text object.
func (e *Editor) textObject(name rune, inner bool, count int) (rangeInfo, bool) {
	if count < 1 {
		count = 1
	}
	switch name {
	case 'w', 'W':
		return e.wordObject(inner, count, name == 'W'), true
	case 'p':
		return e.paragraphObject(inner, count), true
	case 's':
		return e.sentenceObject(inner, count), true
	case '(', ')', 'b':
		return e.pairObject('(', ')', inner)
	case '[', ']':
		return e.pairObject('[', ']', inner)
	case '{', '}', 'B':
		return e.pairObject('{', '}', inner)
	case '<', '>':
		return e.pairObject('<', '>', inner)
	case '"', '\'', '`':
		return e.quoteObject(name, inner)
	case 't':
		return rangeInfo{}, false
	}
	return rangeInfo{}, false
}

func (e *Editor) wordObject(inner bool, count int, big bool) rangeInfo {
	line := []rune(e.buf.LineString(e.cur.Line))
	if len(line) == 0 {
		return rangeInfo{from: e.cur, to: e.cur}
	}
	col := e.cur.Col
	if col >= len(line) {
		col = len(line) - 1
	}
	cls := charClass(line[col], big)
	start, end := col, col
	for start > 0 && charClass(line[start-1], big) == cls {
		start--
	}
	for end < len(line) && charClass(line[end], big) == cls {
		end++
	}
	for i := 1; i < count; i++ {
		next := e.wordForward(buffer.Pos{Line: e.cur.Line, Col: end}, 1, big)
		if next.Line != e.cur.Line || next.Col >= len(line) {
			break
		}
		cls2 := charClass(line[next.Col], big)
		f := next.Col
		for f < len(line) && charClass(line[f], big) == cls2 {
			f++
		}
		end = f
	}
	if !inner {
		f := end
		for f < len(line) && charClass(line[f], big) == cWhite {
			f++
		}
		if f > end {
			end = f
		} else {
			for start > 0 && charClass(line[start-1], big) == cWhite {
				start--
			}
		}
	}
	return rangeInfo{from: buffer.Pos{Line: e.cur.Line, Col: start}, to: buffer.Pos{Line: e.cur.Line, Col: end}, kind: mExclusive}
}

func (e *Editor) paragraphObject(inner bool, count int) rangeInfo {
	from := e.cur.Line
	for from > 0 && strings.TrimSpace(e.buf.LineString(from-1)) != "" {
		from--
	}
	to := e.cur.Line
	for to < e.buf.LastLine() && strings.TrimSpace(e.buf.LineString(to+1)) != "" {
		to++
	}
	if !inner {
		t := to
		for t < e.buf.LastLine() && strings.TrimSpace(e.buf.LineString(t+1)) == "" {
			t++
		}
		if t > to {
			to = t
		} else {
			for from > 0 && strings.TrimSpace(e.buf.LineString(from-1)) == "" {
				from--
			}
		}
	}
	return rangeInfo{from: buffer.Pos{Line: from}, to: buffer.Pos{Line: to + 1}, kind: mLine}
}

func (e *Editor) sentenceObject(inner bool, count int) rangeInfo {
	start := e.cur
	for start.Col > 0 {
		prev := e.retreat(start)
		r := e.buf.RuneAt(prev)
		if (r == '.' || r == '!' || r == '?') && start.Col > prev.Col+1 {
			break
		}
		start = prev
	}
	for start.Col < e.buf.LineLen(start.Line) {
		r := e.buf.RuneAt(start)
		if r == ' ' || r == '\t' {
			start.Col++
			continue
		}
		break
	}
	end := e.sentenceForward(start, 1)
	kind := mInclusive
	if !inner {
		e2 := end
		for e2.Col < e.buf.LineLen(e2.Line) {
			r := e.buf.RuneAt(e2)
			if r == ' ' || r == '\t' {
				e2.Col++
				continue
			}
			break
		}
		end = e2
	}
	return rangeInfo{from: start, to: end, kind: kind}
}

// pairObject finds the innermost bracket pair around the cursor.
func (e *Editor) pairObject(open, close_ rune, inner bool) (rangeInfo, bool) {
	if p, ok := e.findEnclosingPair(open, close_); ok {
		if inner {
			return rangeInfo{from: buffer.Pos{Line: p[0].Line, Col: p[0].Col + 1}, to: p[1], kind: mExclusive}, true
		}
		return rangeInfo{from: p[0], to: buffer.Pos{Line: p[1].Line, Col: p[1].Col + 1}, kind: mExclusive}, true
	}
	return rangeInfo{}, false
}

// findEnclosingPair returns the innermost pair of brackets containing the
// cursor, scanning the whole buffer from the top.
func (e *Editor) findEnclosingPair(open, close_ rune) ([2]buffer.Pos, bool) {
	var stack []buffer.Pos
	var best [2]buffer.Pos
	found := false
	for line := 0; line <= e.buf.LastLine(); line++ {
		for col, r := range e.buf.Line(line) {
			pos := buffer.Pos{Line: line, Col: col}
			switch r {
			case open:
				stack = append(stack, pos)
			case close_:
				if len(stack) == 0 {
					continue
				}
				top := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				contains := (top.Before(e.cur) || top == e.cur) && (e.cur.Before(pos) || e.cur == pos)
				if contains {
					best, found = [2]buffer.Pos{top, pos}, true
				}
			}
		}
		if found && len(stack) == 0 && line >= e.cur.Line {
			break
		}
	}
	return best, found
}

func (e *Editor) quoteObject(quote rune, inner bool) (rangeInfo, bool) {
	line := []rune(e.buf.LineString(e.cur.Line))
	positions := []int{}
	escaped := false
	for i, r := range line {
		if escaped {
			escaped = false
			continue
		}
		if r == '\\' {
			escaped = true
			continue
		}
		if r == quote {
			positions = append(positions, i)
		}
	}
	for i := 0; i+1 < len(positions); i += 2 {
		start, end := positions[i], positions[i+1]
		if e.cur.Col < start {
			continue
		}
		if end < e.cur.Col {
			continue
		}
		if inner {
			return rangeInfo{from: buffer.Pos{Line: e.cur.Line, Col: start + 1}, to: buffer.Pos{Line: e.cur.Line, Col: end}, kind: mExclusive}, true
		}
		return rangeInfo{from: buffer.Pos{Line: e.cur.Line, Col: start}, to: buffer.Pos{Line: e.cur.Line, Col: end + 1}, kind: mExclusive}, true
	}
	return rangeInfo{}, false
}

// operatorTextObject applies the pending operator to a text object.
func (e *Editor) operatorTextObject(k keys.Key, marker keys.Key) {
	if !k.IsRune() {
		e.abortPending()
		return
	}
	inner := marker.IsRune() && marker.R == 'i'
	count := 1
	if e.pending.countSet {
		count = e.pending.count
	}
	if e.pending.opCount > 0 {
		count *= e.pending.opCount
	}
	r, ok := e.textObject(k.R, inner, count)
	if !ok {
		e.abortPending()
		return
	}
	e.applyOperator(e.pending.op, r, count)
}

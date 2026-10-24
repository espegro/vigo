package editor

import (
	"strconv"
	"strings"
	"unicode"

	"vigo/internal/buffer"
)

// normalizeRange makes a range half open and orders its ends.
func (e *Editor) normalizeRange(r rangeInfo) rangeInfo {
	if r.to.Before(r.from) {
		r.from, r.to = r.to, r.from
	}
	r.block = false
	if r.kind == mInclusive && !r.empty() {
		to := r.to
		if to.Col < e.buf.LineLen(to.Line) {
			to.Col++
		}
		r.to = to
		r.kind = mExclusive
	}
	if r.kind == mLine {
		r.from.Col = 0
		if r.to.Line <= e.buf.LastLine() {
			r.to.Col = 0
		}
	}
	return r
}

// rangeText returns the text of a character wise range, split per line.
func (e *Editor) rangeText(r rangeInfo) []string {
	r = e.normalizeRange(r)
	if r.from.Line == r.to.Line {
		line := e.buf.Line(r.from.Line)
		start := clamp(r.from.Col, 0, len(line))
		end := clamp(r.to.Col, 0, len(line))
		if end < start {
			end = start
		}
		return []string{string(line[start:end])}
	}
	out := []string{}
	first := e.buf.Line(r.from.Line)
	out = append(out, string(first[clamp(r.from.Col, 0, len(first)):]))
	for i := r.from.Line + 1; i < r.to.Line && i <= e.buf.LastLine(); i++ {
		out = append(out, e.buf.LineString(i))
	}
	last := e.buf.Line(r.to.Line)
	if last != nil {
		out = append(out, string(last[:clamp(r.to.Col, 0, len(last))]))
	}
	return out
}

func stringsOf(lines [][]rune) []string {
	out := make([]string, len(lines))
	for i, l := range lines {
		out[i] = string(l)
	}
	return out
}

// rangeLines returns the lines of a line wise range.
func (e *Editor) rangeLines(r rangeInfo) ([]string, int, int) {
	if r.to.Before(r.from) {
		r.from, r.to = r.to, r.from
	}
	from := clamp(r.from.Line, 0, e.buf.LastLine())
	to := clamp(r.to.Line-1, 0, e.buf.LastLine())
	if r.kind != mLine {
		to = r.to.Line
	}
	out := []string{}
	for i := from; i <= to; i++ {
		out = append(out, e.buf.LineString(i))
	}
	return out, from, to
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func (e *Editor) registerForRange(r rangeInfo, reg rune) Register {
	if r.kind == mLine {
		lines, _, _ := e.rangeLines(r)
		return LineRegister(lines)
	}
	return Register{Lines: e.rangeText(r)}
}

// applyOperator runs the pending operator on a range.
func (e *Editor) applyOperator(op string, r rangeInfo, count int) {
	reg := e.defaultRegister()
	switch op {
	case "d":
		e.deleteRange(r, reg)
	case "y":
		e.yankRange(r, reg)
	case "c":
		e.changeRange(r, reg)
	case ">":
		e.shiftRange(r, count, true)
	case "<":
		e.shiftRange(r, count, false)
	case "=":
		e.indentBlock(r)
	case "g~":
		e.caseRange(r, '~')
	case "gu":
		e.caseRange(r, 'l')
	case "gU":
		e.caseRange(r, 'u')
	case "gq":
		e.formatRange(r)
	case "!":
		e.askFilter(r)
	}
	e.pending.clearOp()
}

func (p *pendingState) clearOp() {
	p.op = ""
	p.count = 0
	p.countSet = false
	p.opCount = 0
	p.regSet = false
	p.reg = 0
}

func (e *Editor) deleteRange(r rangeInfo, reg rune) {
	e.buf.Checkpoint(e.cur)
	if r.kind == mLine {
		lines, from, to := e.rangeLines(r)
		e.storeRegister(reg, LineRegister(lines), false, false)
		e.buf.DeleteLines(from, to)
		line := clamp(from, 0, e.buf.LastLine())
		e.setCursor(buffer.Pos{Line: line, Col: e.buf.FirstNonBlank(line).Col})
		return
	}
	nr := e.normalizeRange(r)
	if nr.empty() {
		return
	}
	lines := e.buf.DeleteRange(nr.from, nr.to)
	e.storeRegister(reg, Register{Lines: stringsOf(lines)}, false, len(lines) == 1)
	e.setCursor(nr.from)
}

func (e *Editor) yankRange(r rangeInfo, reg rune) {
	regContent := e.registerForRange(r, reg)
	e.storeRegister(reg, regContent, true, false)
	if r.kind == mLine {
		line := clamp(r.from.Line, 0, e.buf.LastLine())
		e.setCursor(buffer.Pos{Line: line, Col: e.buf.FirstNonBlank(line).Col})
		return
	}
	e.setCursor(e.normalizeRange(r).from)
}

func (e *Editor) changeRange(r rangeInfo, reg rune) {
	if r.kind == mLine {
		lines, from, to := e.rangeLines(r)
		e.buf.Checkpoint(e.cur)
		e.storeRegister(reg, LineRegister(lines), false, false)
		indent := indentOf(e.buf.LineString(from))
		e.buf.ReplaceLines(from, to, [][]rune{[]rune(indent)})
		line := clamp(from, 0, e.buf.LastLine())
		e.setCursor(buffer.Pos{Line: line, Col: len(indent)})
		e.enterInsert()
		return
	}
	nr := e.normalizeRange(r)
	if nr.empty() {
		e.enterInsert()
		return
	}
	e.buf.Checkpoint(e.cur)
	lines := e.buf.DeleteRange(nr.from, nr.to)
	e.storeRegister(reg, Register{Lines: stringsOf(lines)}, false, false)
	e.cur = e.buf.Clamp(nr.from)
	e.desiredCol = e.cur.Col
	e.enterInsert()
}

func (e *Editor) shiftRange(r rangeInfo, count int, right bool) {
	if r.kind != mLine {
		r = rangeInfo{from: buffer.Pos{Line: r.from.Line}, to: buffer.Pos{Line: r.to.Line + 1}, kind: mLine}
	}
	nr := e.normalizeRange(r)
	_, from, to := e.rangeLines(nr)
	e.buf.Checkpoint(e.cur)
	for i := from; i <= to; i++ {
		line := e.buf.Line(i)
		if right {
			e.buf.SetLine(i, append([]rune(e.indentUnit(count)), line...))
		} else {
			e.buf.SetLine(i, unindent(line, e.shiftUnits(count)))
		}
	}
	line := clamp(from, 0, e.buf.LastLine())
	e.setCursor(buffer.Pos{Line: line, Col: e.buf.FirstNonBlank(line).Col})
}

func (e *Editor) shiftUnits(count int) int {
	unit := e.opts.ShiftWidth
	if unit <= 0 {
		unit = e.opts.TabStop
	}
	if unit <= 0 {
		unit = 8
	}
	return unit * count
}

func (e *Editor) indentUnit(count int) string {
	if e.opts.ExpandTab {
		return strings.Repeat(" ", e.shiftUnits(count))
	}
	unit := e.opts.ShiftWidth
	if unit <= 0 {
		unit = e.opts.TabStop
	}
	ts := e.opts.TabStop
	if ts <= 0 {
		ts = 8
	}
	width := unit * count
	tabs := width / ts
	spaces := width % ts
	return strings.Repeat("\t", tabs) + strings.Repeat(" ", spaces)
}

func indentOf(line string) string {
	for i, r := range line {
		if r != ' ' && r != '\t' {
			return line[:i]
		}
	}
	return line
}

func unindent(line []rune, width int) []rune {
	removed := 0
	i := 0
	for i < len(line) && removed < width {
		if line[i] == '\t' {
			removed += 4
			i++
		} else if line[i] == ' ' {
			removed++
			i++
		} else {
			break
		}
	}
	return append([]rune{}, line[i:]...)
}

func (e *Editor) caseRange(r rangeInfo, mode rune) {
	nr := e.normalizeRange(r)
	lines := e.rangeText(nr)
	if len(lines) == 0 {
		return
	}
	e.buf.Checkpoint(e.cur)
	if nr.kind == mLine {
		from := nr.from.Line
		to := clamp(nr.to.Line-1, 0, e.buf.LastLine())
		for i := from; i <= to; i++ {
			e.buf.SetLine(i, mapRunes(e.buf.Line(i), mode))
		}
		e.setCursor(buffer.Pos{Line: from, Col: e.buf.FirstNonBlank(from).Col})
		return
	}
	if nr.from.Line == nr.to.Line {
		line := append([]rune{}, e.buf.Line(nr.from.Line)...)
		for i := nr.from.Col; i < nr.to.Col && i < len(line); i++ {
			line[i] = shiftRune(line[i], mode)
		}
		e.buf.SetLine(nr.from.Line, line)
		e.setCursor(nr.from)
		return
	}
	first := append([]rune{}, e.buf.Line(nr.from.Line)...)
	for i := nr.from.Col; i < len(first); i++ {
		first[i] = shiftRune(first[i], mode)
	}
	e.buf.SetLine(nr.from.Line, first)
	for i := nr.from.Line + 1; i < nr.to.Line && i <= e.buf.LastLine(); i++ {
		e.buf.SetLine(i, mapRunes(e.buf.Line(i), mode))
	}
	last := append([]rune{}, e.buf.Line(nr.to.Line)...)
	for i := 0; i < nr.to.Col && i < len(last); i++ {
		last[i] = shiftRune(last[i], mode)
	}
	e.buf.SetLine(nr.to.Line, last)
	e.setCursor(nr.from)
}

func mapRunes(line []rune, mode rune) []rune {
	out := make([]rune, len(line))
	for i, r := range line {
		out[i] = shiftRune(r, mode)
	}
	return out
}

func shiftRune(r rune, mode rune) rune {
	switch mode {
	case 'u':
		return unicode.ToUpper(r)
	case 'l':
		return unicode.ToLower(r)
	case '~':
		if unicode.IsUpper(r) {
			return unicode.ToLower(r)
		}
		if unicode.IsLower(r) {
			return unicode.ToUpper(r)
		}
	}
	return r
}

func (e *Editor) formatRange(r rangeInfo) {
	nr := e.normalizeRange(r)
	from := clamp(nr.from.Line, 0, e.buf.LastLine())
	to := clamp(nr.to.Line-1, 0, e.buf.LastLine())
	if from > to {
		return
	}
	width := 79
	indent := indentOf(e.buf.LineString(from))
	words := strings.Fields(strings.Join(e.lineStrings(from, to), " "))
	e.buf.Checkpoint(e.cur)
	wrapped := wrapWords(words, indent, width)
	e.buf.ReplaceLines(from, to, wrapped)
	e.setCursor(buffer.Pos{Line: from, Col: e.buf.FirstNonBlank(from).Col})
}

func (e *Editor) lineStrings(from, to int) []string {
	out := make([]string, 0, to-from+1)
	for i := from; i <= to && i <= e.buf.LastLine(); i++ {
		out = append(out, e.buf.LineString(i))
	}
	return out
}

func wrapWords(words []string, indent string, width int) [][]rune {
	var out [][]rune
	cur := indent
	for _, w := range words {
		switch {
		case cur == indent:
			cur += w
		case len([]rune(cur))+1+len([]rune(w)) <= width:
			cur += " " + w
		default:
			out = append(out, []rune(cur))
			cur = indent + w
		}
	}
	out = append(out, []rune(cur))
	return out
}

// deleteChars implements x and X.
func (e *Editor) deleteChars(count int, backward bool) {
	line := e.buf.Line(e.cur.Line)
	col := e.cur.Col
	if len(line) == 0 {
		return
	}
	var from, to int
	if backward {
		to = col
		from = col - count
		if from < 0 {
			from = 0
		}
	} else {
		from = col
		to = col + count
		if to > len(line) {
			to = len(line)
		}
	}
	if from >= to {
		return
	}
	e.buf.Checkpoint(e.cur)
	removed := e.buf.DeleteRange(buffer.Pos{Line: e.cur.Line, Col: from}, buffer.Pos{Line: e.cur.Line, Col: to})
	reg := Register{Lines: []string{string(removed[0])}}
	e.storeRegister(e.defaultRegister(), reg, false, true)
	e.setCursor(buffer.Pos{Line: e.cur.Line, Col: from})
}

func (e *Editor) deleteToEndOfLine() {
	line := e.buf.Line(e.cur.Line)
	if len(line) == 0 {
		return
	}
	r := rangeInfo{from: e.cur, to: buffer.Pos{Line: e.cur.Line, Col: len(line)}, kind: mExclusive}
	e.deleteRange(r, e.defaultRegister())
}

func (e *Editor) yankToEndOfLine() {
	line := e.buf.Line(e.cur.Line)
	if len(line) == 0 {
		return
	}
	r := rangeInfo{from: e.cur, to: buffer.Pos{Line: e.cur.Line, Col: len(line)}, kind: mExclusive}
	e.yankRange(r, e.defaultRegister())
}

func (e *Editor) toggleCase(count int) {
	line := e.buf.Line(e.cur.Line)
	if len(line) == 0 {
		return
	}
	e.buf.Checkpoint(e.cur)
	col := e.cur.Col
	for i := 0; i < count && col < e.buf.LineLen(e.cur.Line); i++ {
		l := append([]rune{}, e.buf.Line(e.cur.Line)...)
		l[col] = shiftRune(l[col], '~')
		e.buf.SetLine(e.cur.Line, l)
		col++
	}
	e.setCursor(buffer.Pos{Line: e.cur.Line, Col: col})
}

func (e *Editor) replaceChar(ch rune, count int) {
	e.buf.Checkpoint(e.cur)
	if ch == '\n' {
		e.buf.SplitLine(buffer.Pos{Line: e.cur.Line, Col: e.cur.Col + count})
		e.setCursor(e.cur)
		return
	}
	line := append([]rune{}, e.buf.Line(e.cur.Line)...)
	for i := 0; i < count; i++ {
		c := e.cur.Col + i
		if c >= len(line) {
			break
		}
		line[c] = ch
	}
	e.buf.SetLine(e.cur.Line, line)
	e.setCursor(buffer.Pos{Line: e.cur.Line, Col: e.cur.Col + count - 1})
}

func (e *Editor) joinLines(count int, spaces bool) {
	if e.cur.Line >= e.buf.LastLine() {
		return
	}
	e.buf.Checkpoint(e.cur)
	for i := 0; i < count; i++ {
		if e.cur.Line >= e.buf.LastLine() {
			break
		}
		e.buf.JoinLines(e.cur.Line, spaces)
	}
	line := e.buf.Line(e.cur.Line)
	col := e.cur.Col
	if col >= len(line) {
		col = len(line) - 1
	}
	if col < 0 {
		col = 0
	}
	e.setCursor(buffer.Pos{Line: e.cur.Line, Col: col})
}

// insertTextAt inserts text that may contain newlines.
func (e *Editor) insertTextAt(p buffer.Pos, text string) buffer.Pos {
	return e.buf.InsertText(p, text)
}

// paste implements p and P.
func (e *Editor) paste(count int, before bool) {
	reg := e.register(e.defaultRegister())
	if reg.Empty() {
		e.errorf("E353: Nothing in register %c", e.defaultRegister())
		return
	}
	e.buf.Checkpoint(e.cur)
	switch {
	case reg.Linewise:
		at := e.cur.Line + 1
		col := 0
		if before {
			at = e.cur.Line
		}
		lines := make([][]rune, 0, len(reg.Lines)*count)
		for i := 0; i < count; i++ {
			for _, l := range reg.Lines {
				lines = append(lines, []rune(l))
			}
		}
		e.buf.InsertLines(at, lines)
		line := clamp(at, 0, e.buf.LastLine())
		col = e.buf.FirstNonBlank(line).Col
		e.setCursor(buffer.Pos{Line: line, Col: col})
	case reg.Blockwise:
		e.pasteBlock(reg, count, before)
	default:
		text := strings.Repeat(strings.Join(reg.Lines, "\n"), count)
		pos := e.cur
		if !before {
			if pos.Col < e.buf.LineLen(pos.Line) {
				pos.Col++
			}
		}
		end := e.insertTextAt(pos, text)
		e.setCursor(e.retreat(end))
	}
}

func (e *Editor) pasteBlock(reg Register, count int, before bool) {
	col := e.cur.Col
	if !before {
		col++
	}
	width := reg.Width
	if width == 0 && len(reg.Lines) > 0 {
		width = len([]rune(reg.Lines[0]))
	}
	for i := 0; i < count; i++ {
		line := e.cur.Line + i*len(reg.Lines)
		for j, text := range reg.Lines {
			target := line + j
			for target > e.buf.LastLine() {
				e.buf.InsertLines(e.buf.Len(), [][]rune{{}})
			}
			runes := []rune(text)
			for len(runes) < width {
				runes = append(runes, ' ')
			}
			l := append([]rune{}, e.buf.Line(target)...)
			if col > len(l) {
				for len(l) < col {
					l = append(l, ' ')
				}
			}
			tail := append([]rune{}, l[col:]...)
			l = append(l[:col], runes...)
			l = append(l, tail...)
			e.buf.SetLine(target, l)
		}
	}
	e.setCursor(buffer.Pos{Line: e.cur.Line, Col: col + width - 1})
}

func (e *Editor) undo(n int) {
	if n < 1 {
		n = 1
	}
	for i := 0; i < n; i++ {
		cur, ok := e.buf.Undo(e.cur)
		if !ok {
			e.errorf("E32: No previous undo")
			return
		}
		e.setCursor(cur)
	}
}

func (e *Editor) redo() {
	e.buf.BreakBlock()
	n := e.count()
	for i := 0; i < n; i++ {
		cur, ok := e.buf.Redo(e.cur)
		if !ok {
			e.errorf("E33: No later undo")
			return
		}
		e.setCursor(cur)
	}
}

// incrementNumber implements ctrl-a and ctrl-x.
func (e *Editor) incrementNumber(delta int) {
	if delta == 0 {
		return
	}
	line := []rune(e.buf.LineString(e.cur.Line))
	if len(line) == 0 {
		return
	}
	start, end := e.cur.Col, e.cur.Col
	isDigit := func(r rune) bool { return r >= '0' && r <= '9' }
	isHex := func(r rune) bool {
		return isDigit(r) || r >= 'a' && r <= 'f' || r >= 'A' && r <= 'F'
	}
	hex := false
	find := func(pred func(rune) bool) (int, int) {
		s, t := e.cur.Col, e.cur.Col
		for s > 0 && pred(line[s-1]) {
			s--
		}
		for t < len(line) && pred(line[t]) {
			t++
		}
		return s, t
	}
	if s, t := find(isDigit); s < t {
		start, end = s, t
	} else if s, t := find(isHex); s < t {
		start, end, hex = s, t, true
		if s == 0 && line[s] == '-' {
			hex = false
		}
	} else {
		return
	}
	text := string(line[start:end])
	base := 10
	if hex {
		base = 16
	}
	val, err := strconv.ParseInt(text, base, 64)
	if err != nil {
		return
	}
	val += int64(delta)
	width := end - start
	rep := strconv.FormatInt(val, base)
	if len(rep) < width {
		rep = strings.Repeat("0", width-len(rep)) + rep
	}
	e.buf.Checkpoint(e.cur)
	out := append(append([]rune{}, line[:start]...), []rune(rep)...)
	out = append(out, line[end:]...)
	e.buf.SetLine(e.cur.Line, out)
	e.setCursor(buffer.Pos{Line: e.cur.Line, Col: start})
}

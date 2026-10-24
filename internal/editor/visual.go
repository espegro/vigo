package editor

import (
	"strings"

	"vigo/internal/buffer"
	"vigo/internal/keys"
)

func (e *Editor) exitVisual() {
	e.saveLastVisual()
	e.mode = ModeNormal
}

func (e *Editor) saveLastVisual() {
	if !e.mode.visual() {
		return
	}
	a, b := e.visualStart, e.cur
	if b.Before(a) {
		a, b = b, a
	}
	e.lastVisual = [2]buffer.Pos{a, b}
	e.hasLastVisual = true
	e.marks['<'] = a
	e.marks['>'] = b
}

func (e *Editor) reselectVisual() {
	if !e.hasLastVisual {
		return
	}
	e.enterVisual(ModeVisual)
	e.visualStart = e.lastVisual[0]
	e.setCursor(e.lastVisual[1])
}

// visualRange returns the range the current visual selection covers.
func (e *Editor) visualRange() rangeInfo {
	switch e.mode {
	case ModeVisualLine:
		return lineRange(e.visualStart, e.cur)
	case ModeVisualBlock:
		c1, c2 := e.visualStart.Col, e.cur.Col
		if c2 < c1 {
			c1, c2 = c2, c1
		}
		lo, hi := e.visualStart.Line, e.cur.Line
		if hi < lo {
			lo, hi = hi, lo
		}
		return rangeInfo{
			from:  buffer.Pos{Line: lo, Col: c1},
			to:    buffer.Pos{Line: hi, Col: c2 + 1},
			kind:  mExclusive,
			block: true,
		}
	default:
		return rangeInfo{from: e.visualStart, to: e.cur, kind: mInclusive}
	}
}

func (e *Editor) visualKey(k keys.Key) {
	if k.IsSpecial(keys.Esc) || (k.IsCtrl() && k.R == 'C') {
		e.exitVisual()
		return
	}
	if e.pending.awaiting != "" {
		e.visualAwait(k)
		return
	}
	if e.tryRegister(k) {
		return
	}
	if k.IsCtrl() {
		switch k.R {
		case 'V':
			e.blockCol = e.cur.Col
			e.mode = ModeVisualBlock
		case 'G':
			e.showPosition()
		}
		return
	}
	if !k.IsRune() {
		switch k.Name {
		case keys.Up, keys.Down, keys.Left, keys.Right, keys.Home, keys.End, keys.PgUp, keys.PgDn:
			name := map[string]string{
				keys.Up: "k", keys.Down: "j", keys.Left: "h", keys.Right: "l",
				keys.Home: "0", keys.End: "$",
			}[k.Name]
			if name == "" {
				if k.Name == keys.PgUp {
					e.scrollPages(-e.count())
				} else {
					e.scrollPages(e.count())
				}
				return
			}
			e.doMotion(name)
		}
		return
	}
	r := k.R
	switch {
	case r >= '1' && r <= '9':
		e.addCount(int(r - '0'))
		return
	case r == '0' && e.pending.countSet:
		e.addCount(0)
		return
	}
	switch r {
	case 'h', 'j', 'k', 'l', 'w', 'W', 'b', 'B', 'e', 'E', '0', '^', '$',
		'{', '}', '(', ')', 'G', '|', '_', '+', '-', ' ', '%', ';', ',', 'n', 'N', '*', '#':
		if e.mode == ModeVisualBlock && r == '$' {
			e.blockEOL = true
		} else if e.mode == ModeVisualBlock {
			e.blockEOL = false
		}
		e.doMotion(string(r))
	case 'g':
		e.pending.awaiting = "g"
	case 'o':
		e.visualSwap(false)
	case 'O':
		e.visualSwap(true)
	case 'v':
		if e.mode == ModeVisual {
			e.exitVisual()
		} else {
			e.mode = ModeVisual
		}
	case 'V':
		if e.mode == ModeVisualLine {
			e.exitVisual()
		} else {
			e.mode = ModeVisualLine
		}
	case 'd', 'x':
		e.visualOperator("d")
	case 'y':
		e.visualOperator("y")
	case 'c', 's':
		e.visualOperator("c")
	case 'p', 'P':
		e.visualPaste()
	case '>':
		e.visualOperator(">")
	case '<':
		e.visualOperator("<")
	case '=':
		e.visualOperator("=")
	case '~':
		e.visualOperator("g~")
	case 'u':
		e.visualOperator("gu")
	case 'U':
		e.visualOperator("gU")
	case 'J':
		e.visualJoin()
	case ':':
		e.saveLastVisual()
		e.startCmdline(ModeCmdline, "'<,'>")
	case 'I':
		e.blockInsertAt(true)
	case 'A':
		e.blockInsertAt(false)
	case 'r':
		e.pending.awaiting = "replace"
	case 'm':
		e.pending.awaiting = "mark"
	case '`', '\'':
		e.pending.awaitKey = keys.Rune(r)
		e.pending.awaiting = "jumpmark"
	case 'q':
		e.pending.awaiting = "record"
	case '@':
		e.pending.awaiting = "play"
	}
}

func (e *Editor) visualAwait(k keys.Key) {
	state := e.pending.awaiting
	e.pending.awaiting = ""
	switch state {
	case "g":
		if !k.IsRune() {
			return
		}
		switch k.R {
		case 'J':
			e.visualJoin()
		case 'u':
			e.visualOperator("gu")
		case 'U':
			e.visualOperator("gU")
		case '~':
			e.visualOperator("g~")
		case 'q':
			e.visualOperator("gq")
		case 'v':
			e.reselectVisual()
		}
	case "replace":
		if k.IsRune() {
			e.visualReplace(k.R)
		}
	case "register":
		if k.IsRune() {
			e.pending.reg = k.R
			e.pending.regSet = true
		}
	case "mark":
		if k.IsRune() {
			e.marks[k.R] = e.cur
		}
	case "jumpmark":
		e.jumpToMark(k, e.pending.awaitKey)
	}
}

func (e *Editor) visualSwap(corner bool) {
	anchor := e.visualStart
	e.visualStart = e.cur
	if corner {
		e.setCursor(buffer.Pos{Line: anchor.Line, Col: e.blockCol})
		e.blockCol = anchor.Col
		return
	}
	e.setCursor(anchor)
}

func (e *Editor) visualOperator(op string) {
	count := 1
	if e.pending.countSet {
		count = e.pending.count
	}
	r := e.visualRange()
	e.pending.count = 0
	e.pending.countSet = false
	e.saveLastVisual()
	if r.block && (op == "d" || op == "y" || op == "c") {
		e.blockOperator(op, r)
		e.mode = ModeNormal
		return
	}
	e.applyOperator(op, r, count)
	if op == "c" {
		return
	}
	e.mode = ModeNormal
}

func (e *Editor) blockOperator(op string, r rangeInfo) {
	from, to, c1, c2 := e.blockBounds(r)
	reg := e.defaultRegister()
	lines := e.blockText(r, from, to, c1, c2)
	e.storeRegister(reg, Register{Lines: lines, Blockwise: true, Width: c2 - c1}, op == "y", false)
	if op == "y" {
		e.setCursor(buffer.Pos{Line: from, Col: c1})
		return
	}
	e.buf.Checkpoint(e.cur)
	for i := from; i <= to; i++ {
		line := e.buf.Line(i)
		s := clamp(c1, 0, len(line))
		t := clamp(c2, 0, len(line))
		if s >= t {
			continue
		}
		e.buf.DeleteRange(buffer.Pos{Line: i, Col: s}, buffer.Pos{Line: i, Col: t})
	}
	e.setCursor(buffer.Pos{Line: from, Col: c1})
	if op == "c" {
		e.enterInsert()
		return
	}
	e.mode = ModeNormal
}

func (e *Editor) blockBounds(r rangeInfo) (from, to, c1, c2 int) {
	from, to = r.from.Line, r.to.Line
	c1, c2 = r.from.Col, r.to.Col
	if c2 < c1 {
		c1, c2 = c2, c1
	}
	if e.blockEOL {
		c2 = c1 + 1
		for i := from; i <= to && i <= e.buf.LastLine(); i++ {
			if n := e.buf.LineLen(i); n > c2 {
				c2 = n
			}
		}
	}
	c1 = clamp(c1, 0, 1<<20)
	c2 = clamp(c2, 0, 1<<20)
	return
}

func (e *Editor) blockText(r rangeInfo, from, to, c1, c2 int) []string {
	out := make([]string, 0, to-from+1)
	for i := from; i <= to && i <= e.buf.LastLine(); i++ {
		line := e.buf.Line(i)
		s := clamp(c1, 0, len(line))
		t := clamp(c2, 0, len(line))
		text := string(line[s:t])
		for len([]rune(text)) < c2-c1 {
			text += " "
		}
		out = append(out, text)
	}
	return out
}

func (e *Editor) visualPaste() {
	reg := e.register(e.defaultRegister())
	if reg.Empty() {
		e.errorf("E353: Nothing in register %c", e.defaultRegister())
		return
	}
	r := e.visualRange()
	e.saveLastVisual()
	if r.block && reg.Blockwise {
		e.blockOperator("d", r)
		e.mode = ModeNormal
		e.setCursor(buffer.Pos{Line: r.from.Line, Col: r.from.Col})
		e.paste(1, false)
		return
	}
	e.pending.count = 0
	e.pending.countSet = false
	e.applyOperator("d", r, 1)
	e.mode = ModeNormal
	e.paste(1, false)
}

func (e *Editor) visualReplace(ch rune) {
	r := e.visualRange()
	nr := e.normalizeRange(r)
	e.saveLastVisual()
	e.buf.Checkpoint(e.cur)
	if e.mode == ModeVisualLine {
		from, to := nr.from.Line, clamp(nr.to.Line-1, 0, e.buf.LastLine())
		for i := from; i <= to; i++ {
			line := e.buf.Line(i)
			repl := make([]rune, len(line))
			for j := range repl {
				repl[j] = ch
			}
			e.buf.SetLine(i, repl)
		}
		e.setCursor(buffer.Pos{Line: from, Col: 0})
		e.mode = ModeNormal
		return
	}
	if nr.from.Line != nr.to.Line {
		e.mode = ModeNormal
		return
	}
	line := e.buf.Line(nr.from.Line)
	s := clamp(nr.from.Col, 0, len(line))
	t := clamp(nr.to.Col, 0, len(line))
	for i := s; i < t; i++ {
		line[i] = ch
	}
	e.buf.SetLine(nr.from.Line, line)
	e.setCursor(nr.from)
	e.mode = ModeNormal
}

func (e *Editor) visualJoin() {
	r := e.visualRange()
	nr := e.normalizeRange(r)
	from := nr.from.Line
	to := clamp(nr.to.Line-1, 0, e.buf.LastLine())
	e.saveLastVisual()
	e.buf.Checkpoint(e.cur)
	for i := from; i < to; i++ {
		e.buf.JoinLines(from, true)
	}
	e.setCursor(buffer.Pos{Line: from, Col: e.buf.FirstNonBlank(from).Col})
	e.mode = ModeNormal
}

// blockInsertAt implements I and A in block visual mode.
func (e *Editor) blockInsertAt(atStart bool) {
	if e.mode != ModeVisualBlock {
		e.enterInsert()
		return
	}
	r := e.visualRange()
	from, to, c1, _ := e.blockBounds(r)
	e.blockInsertActive = true
	e.blockTextTyped = nil
	e.mode = ModeInsert
	if atStart {
		e.blockInsertTop = buffer.Pos{Line: from, Col: c1}
		e.blockInsertBot = buffer.Pos{Line: to, Col: c1}
		e.cur = e.buf.Clamp(buffer.Pos{Line: from, Col: c1})
	} else {
		e.blockInsertTop = buffer.Pos{Line: from, Col: -1}
		e.blockInsertBot = buffer.Pos{Line: to, Col: -1}
		e.cur = e.buf.EndOfLine(from)
	}
	e.desiredCol = e.cur.Col
	e.noteInsert()
}

// applyBlockInsert writes the text typed during a block insert to every line.
func (e *Editor) applyBlockInsert() {
	e.blockInsertActive = false
	text := string(e.blockTextTyped)
	e.blockTextTyped = nil
	if text == "" {
		return
	}
	from := e.blockInsertTop
	to := e.blockInsertBot
	e.buf.Checkpoint(e.cur)
	curLine := e.cur.Line
	for line := from.Line; line <= to.Line; line++ {
		if line == curLine {
			continue
		}
		col := from.Col
		l := append([]rune{}, e.buf.Line(line)...)
		if col < 0 {
			col = len(l)
		}
		if col > len(l) {
			for len(l) < col {
				l = append(l, ' ')
			}
		}
		tail := append([]rune{}, l[col:]...)
		l = append(l[:col], []rune(text)...)
		l = append(l, tail...)
		e.buf.SetLine(line, l)
	}
	e.setCursor(from)
}

func (e *Editor) visualString() string {
	r := e.visualRange()
	if r.block {
		from, to, c1, c2 := e.blockBounds(r)
		return strings.Join(e.blockText(r, from, to, c1, c2), "\n")
	}
	return strings.Join(e.rangeText(r), "\n")
}

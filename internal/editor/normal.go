package editor

import (
	"strconv"
	"strings"

	"vigo/internal/buffer"
	"vigo/internal/keys"
)

func (e *Editor) addCount(d int) {
	p := &e.pending
	p.count = p.count*10 + d
	p.countSet = true
}

// tryRegister consumes a " register selector.
func (e *Editor) tryRegister(k keys.Key) bool {
	if !k.IsRune() || k.R != '"' {
		return false
	}
	e.pending.awaiting = "register"
	return true
}

// normalKey is the entry point for normal mode keys.
func (e *Editor) normalKey(k keys.Key) {
	if k.IsSpecial(keys.Esc) {
		e.abortPending()
		e.cache().hlVersion = 0
		return
	}
	if e.pending.awaiting != "" {
		e.awaitKey(k)
		return
	}
	if e.tryRegister(k) {
		return
	}
	if k.IsCtrl() {
		e.ctrlKey(k)
		return
	}
	if k.IsRune() {
		r := k.R
		switch {
		case r >= '1' && r <= '9':
			e.addCount(int(r - '0'))
			return
		case r == '0' && e.pending.countSet:
			e.addCount(0)
			return
		}
		if e.pending.op != "" {
			e.operatorMotion(k)
			return
		}
		e.normalRune(k)
		return
	}
	if e.pending.op != "" {
		e.operatorMotion(k)
		return
	}
	e.normalSpecial(k)
}

func (e *Editor) normalSpecial(k keys.Key) {
	switch k.Name {
	case keys.Up:
		e.doMotion("k")
	case keys.Down:
		e.doMotion("j")
	case keys.Left:
		e.doMotion("h")
	case keys.Right:
		e.doMotion("l")
	case keys.Home:
		e.doMotion("0")
	case keys.End:
		e.doMotion("$")
	case keys.PgUp:
		e.scrollPages(-e.count())
	case keys.PgDn:
		e.scrollPages(e.count())
	case keys.Del:
		e.normalRune(keys.Rune('x'))
	case keys.Ins:
		e.normalRune(keys.Rune('i'))
	}
}

// ctrlKey handles control modified normal mode keys.
func (e *Editor) ctrlKey(k keys.Key) {
	switch k.R {
	case 'D':
		n, set := e.takeCount()
		if !set {
			n = e.viewHeight() / 2
			if n < 1 {
				n = 1
			}
		}
		e.scrollWithCursor(n)
	case 'U':
		n, set := e.takeCount()
		if !set {
			n = e.viewHeight() / 2
			if n < 1 {
				n = 1
			}
		}
		e.scrollWithCursor(-n)
	case 'F':
		e.scrollPages(e.count())
	case 'B':
		e.scrollPages(-e.count())
	case 'E':
		e.scrollWithCursor(e.count())
	case 'Y':
		e.scrollWithCursor(-e.count())
	case 'R':
		e.redo()
	case 'G':
		e.showPosition()
	case 'L':
		e.forceRedraw = true
		e.scr.Resize(e.scr.Size())
	case 'O', 'I':
		if k.R == 'O' {
			e.jumpOlder()
		} else {
			e.jumpNewer()
		}
	case 'V':
		e.enterVisual(ModeVisualBlock)
	case 'A':
		e.incrementNumber(e.count())
	case 'X':
		e.incrementNumber(-e.count())
	case 'C':
		e.errorf("Type  :quit<Enter>  to exit vigo")
	case 'W':
		e.pending.awaiting = "ctrlw"
	}
}

func (e *Editor) normalRune(k keys.Key) {
	r := k.R
	switch r {
	case 'h', 'j', 'k', 'l', 'w', 'W', 'b', 'B', 'e', 'E', '0', '^', '$',
		'{', '}', '(', ')', 'G', '|', '_', '+', '-', ' ', '%', ';', ',':
		e.doMotion(string(r))
	case 'g':
		e.pending.awaiting = "g"
	case 'z':
		e.pending.awaiting = "z"
	case 'Z':
		e.pending.awaiting = "Z"
	case 'f', 'F', 't', 'T':
		e.pending.awaiting = "find"
		e.pending.awaitKey = k
	case 'i':
		e.cmdInsert('i')
	case 'I':
		e.cmdInsert('I')
	case 'a':
		e.cmdInsert('a')
	case 'A':
		e.cmdInsert('A')
	case 'o':
		e.cmdInsert('o')
	case 'O':
		e.cmdInsert('O')
	case 's':
		e.cmdInsert('s')
	case 'S':
		e.cmdInsert('S')
	case 'C':
		e.cmdInsert('C')
	case 'R':
		e.cmdInsert('R')
	case 'x':
		e.deleteChars(e.count(), false)
	case 'X':
		e.deleteChars(e.count(), true)
	case 'D':
		e.deleteToEndOfLine()
	case 'Y':
		e.yankToEndOfLine()
	case 'J':
		e.joinLines(e.count(), true)
	case 'd', 'y', 'c', '>', '<', '=':
		e.pending.op = string(r)
	case '!':
		e.pending.op = "!"
	case '~':
		e.toggleCase(e.count())
	case 'p':
		e.paste(e.count(), false)
	case 'P':
		e.paste(e.count(), true)
	case 'r':
		e.pending.awaiting = "replace"
	case 'u':
		e.undo(e.count())
	case '.':
		e.repeatChange()
	case 'v':
		e.enterVisual(ModeVisual)
	case 'V':
		e.enterVisual(ModeVisualLine)
	case ':':
		e.startCmdline(ModeCmdline, "")
	case '/':
		e.startCmdline(ModeSearchForward, "/")
	case '?':
		e.startCmdline(ModeSearchBackward, "?")
	case 'n':
		e.doMotion("n")
	case 'N':
		e.doMotion("N")
	case '*', '#':
		e.doMotion(string(r))
	case 'm':
		e.pending.awaiting = "mark"
	case '\'', '`':
		e.pending.awaitKey = keys.Rune(r)
		e.pending.awaiting = "jumpmark"
	case 'q':
		if e.recording {
			e.stopRecording()
		} else {
			e.pending.awaiting = "record"
		}
	case '@':
		e.pending.awaiting = "play"
	case '&':
		e.repeatSubstitute()
	case 'Q':
		e.mode = ModeReplace
	case 'U':
		e.errorf("U is not supported yet")
	}
}

// awaitKey handles the second key of a multi key command.
func (e *Editor) awaitKey(k keys.Key) {
	state := e.pending.awaiting
	e.pending.awaiting = ""
	switch state {
	case "register":
		if k.IsRune() || k.IsCtrl() {
			e.pending.reg = k.R
			if k.IsRune() && k.R >= 'A' && k.R <= 'Z' {
				e.pending.reg = k.R
			}
			e.pending.regSet = true
		}
	case "g":
		e.awaitG(k)
	case "z":
		e.awaitZ(k)
	case "Z":
		if k.IsRune() && k.R == 'Z' {
			e.writeCurrent()
			e.quit = true
		} else if k.IsRune() && k.R == 'Q' {
			e.quit = true
		}
	case "find":
		if k.IsRune() {
			kind := 'f'
			if e.pending.awaitKey.R != 0 {
				kind = e.pending.awaitKey.R
			}
			e.lastFind = findState{kind: kind, char: k.R, set: true}
			name := string(kind)
			if e.pending.op != "" {
				n, _ := e.opMotionCount()
				res := e.motion(name, n, false)
				if res.ok {
					e.applyOperator(e.pending.op, res.r, n)
				} else {
					e.abortPending()
				}
				return
			}
			e.doMotion(name)
		}
	case "gmotion":
		if k.IsRune() && k.R == 'g' {
			n, set := e.opMotionCount()
			res := e.motion("gg", n, set)
			if res.ok {
				e.applyOperator(e.pending.op, res.r, n)
			} else {
				e.abortPending()
			}
			return
		}
		e.abortPending()
	case "textobj":
		e.operatorTextObject(k, e.pending.awaitKey)
	case "replace":
		if k.IsRune() {
			e.replaceChar(k.R, e.count())
		} else if k.IsSpecial(keys.CR) {
			e.replaceChar('\n', e.count())
		}
	case "mark":
		if k.IsRune() {
			e.marks[k.R] = e.cur
		}
	case "jumpmark":
		e.jumpToMark(k, e.pending.awaitKey)
	case "record":
		if k.IsRune() {
			if e.recording && e.recordReg == k.R {
				e.stopRecording()
			} else {
				e.recordReg = k.R
				e.macros[k.R] = nil
				e.recording = true
			}
		}
	case "play":
		if k.IsRune() {
			e.playMacro(k.R)
		}
	case "bang":
		if k.IsRune() && k.R == '!' {
		}
	case "ctrlw":
		e.awaitCtrlW(k)
	}
}

func (e *Editor) awaitG(k keys.Key) {
	if k.IsCtrl() && k.R == 'G' {
		e.showWordCount()
		return
	}
	if !k.IsRune() {
		return
	}
	switch k.R {
	case 'g':
		e.doMotion("gg")
	case 'J':
		e.joinLines(e.count(), false)
	case 'u':
		e.pending.op = "gu"
	case 'U':
		e.pending.op = "gU"
	case '~':
		e.pending.op = "g~"
	case 'q':
		e.pending.op = "gq"
	case 'v':
		e.reselectVisual()
	case 'i':
		e.setCursor(e.lastInsertPos)
		e.cmdInsert('i')
	case '*':
		e.doMotion("*")
	case '#':
		e.doMotion("#")
	case ';':
		e.changeOlder()
	case ',':
		e.changeNewer()
	case 'e', 'E':
		e.doMotion("b")
	case 'f':
		e.gotoFileUnderCursor()
	}
}

func (e *Editor) awaitZ(k keys.Key) {
	if !k.IsRune() {
		return
	}
	switch k.R {
	case 'z':
		e.scrollToCursor(Center)
	case 't':
		e.scrollToCursor(Top)
	case 'b':
		e.scrollToCursor(Bottom)
	case '.':
		e.scrollToCursor(Center)
		e.pending.count = e.count()
		e.cursorLine()
	case '-':
		e.scrollToCursor(Bottom)
		e.cursorLine()
	case 'h':
		e.horizontalScroll(-e.count())
	case 'l':
		e.horizontalScroll(e.count())
	case 's', 'e':
		e.horizontalScrollCursor(0)
	case 'H':
		e.horizontalScrollCursor(-1 << 20)
	case 'L':
		e.horizontalScrollCursor(1 << 20)
	case 'w', 'W':
		e.horizontalScrollCursor(1 << 20)
	}
}

// doMotion moves the cursor with the named motion.
func (e *Editor) doMotion(name string) {
	if name == "f" && (!e.lastFind.set || e.lastFind.kind == 0) {
		e.lastFind.kind = 'f'
	}
	old := e.cur
	n, set := e.takeCount()
	res := e.motion(name, n, set)
	if !res.ok {
		return
	}
	if name == "f" || name == "t" || name == "F" || name == "T" {
		e.lastFind.kind = rune(name[0])
	}
	if name == "n" || name == "N" {
		e.pushJump(old)
	}
	switch name {
	case "gg", "G", "%", "{", "}", "(", ")", "*", "#", "n", "N":
		e.pushJump(old)
	}
	if name == "$" {
		e.desiredCol = 1 << 20
	} else {
		e.desiredCol = res.dest.Col
	}
	e.moveCursor(res.dest)
	switch {
	case res.r.kind == mLine && e.buf.LineLen(res.dest.Line) > 0:
		if name != "j" && name != "k" && name != "-" && name != "+" && name != "_" && name != "cr" && name != "$" {
			e.moveCursor(buffer.Pos{Line: res.dest.Line, Col: e.buf.FirstNonBlank(res.dest.Line).Col})
			e.desiredCol = e.cur.Col
		}
	}
	if name == "$" && e.buf.LineLen(e.cur.Line) > 0 {
		e.moveCursor(e.buf.LastCharPos(e.cur.Line))
	}
}

// operatorMotion handles keys while an operator is pending.
func (e *Editor) operatorMotion(k keys.Key) {
	p := &e.pending
	if k.IsRune() {
		r := k.R
		switch {
		case r >= '1' && r <= '9':
			p.opCount = p.opCount*10 + int(r-'0')
			return
		case r == '0' && p.opCount > 0:
			p.opCount *= 10
			return
		case r == 'i' || r == 'a':
			p.awaiting = "textobj"
			p.awaitKey = k
			return
		}
		if r == rune(p.op[0]) && len(p.op) == 1 {
			e.applyDoubleOperator()
			return
		}
		if strings.ContainsRune("ftFT", r) {
			p.awaiting = "find"
			p.awaitKey = k
			return
		}
	}
	name := e.keyToMotionName(k)
	if name == "" {
		if e.pending.awaiting != "" {
			return
		}
		e.abortPending()
		return
	}
	n, set := e.opMotionCount()
	res := e.motion(name, n, set)
	if !res.ok {
		e.abortPending()
		return
	}
	e.applyOperator(p.op, res.r, n)
}

func (e *Editor) opMotionCount() (int, bool) {
	n := 1
	set := false
	if e.pending.countSet && e.pending.count > 0 {
		n *= e.pending.count
		set = true
	}
	if e.pending.opCount > 0 {
		n *= e.pending.opCount
		set = true
	}
	return n, set
}

func (e *Editor) keyToMotionName(k keys.Key) string {
	if k.IsRune() {
		switch k.R {
		case 'h', 'j', 'k', 'l', 'w', 'W', 'b', 'B', 'e', 'E', '0', '^', '$',
			'{', '}', '(', ')', 'G', '|', '_', '+', '-', '%', ';', ',', ' ', 'n', 'N', '*', '#':
			return string(k.R)
		case 'f', 'F', 't', 'T':
			return string(k.R)
		case 'g':
			e.pending.awaiting = "gmotion"
			return ""
		}
		return ""
	}
	switch k.Name {
	case keys.Up:
		return "k"
	case keys.Down:
		return "j"
	case keys.Left:
		return "h"
	case keys.Right:
		return "l"
	case keys.Home:
		return "0"
	case keys.End:
		return "$"
	case keys.CR:
		return "cr"
	case keys.BS:
		return "backspace"
	}
	return ""
}

// applyDoubleOperator handles dd, yy, cc, >>, << and ==.
func (e *Editor) applyDoubleOperator() {
	n, _ := e.opMotionCount()
	if n < 1 {
		n = 1
	}
	last := e.cur.Line + n - 1
	if last > e.buf.LastLine() {
		last = e.buf.LastLine()
	}
	r := rangeInfo{
		from: buffer.Pos{Line: e.cur.Line},
		to:   buffer.Pos{Line: last + 1},
		kind: mLine,
	}
	e.applyOperator(e.pending.op, r, n)
}

// repeatChange runs the recorded last change, replacing any count.
func (e *Editor) repeatChange() {
	if !e.hasLastChange {
		e.errorf("E30: No previous change")
		return
	}
	ks := e.lastChange
	if e.pending.countSet {
		ks = replaceCount(ks, e.pending.count)
	}
	e.pending.reset()
	e.feedKeys(ks, false)
}

func replaceCount(ks []keys.Key, count int) []keys.Key {
	rest := ks
	for len(rest) > 0 && rest[0].IsRune() && rest[0].R >= '1' && rest[0].R <= '9' {
		rest = rest[1:]
		for len(rest) > 0 && rest[0].IsRune() && rest[0].R >= '0' && rest[0].R <= '9' {
			rest = rest[1:]
		}
		break
	}
	out := []keys.Key{}
	for _, c := range strconv.Itoa(count) {
		out = append(out, keys.Rune(c))
	}
	return append(out, rest...)
}

func (e *Editor) playMacro(reg rune) {
	if reg == '@' {
		reg = e.lastMacro
	}
	if reg == 0 {
		e.errorf("E748: No previously used register")
		return
	}
	ks, ok := e.macros[reg]
	if !ok || len(ks) == 0 {
		e.errorf("E748: No such register: %c", reg)
		return
	}
	e.lastMacro = reg
	count := e.count()
	for i := 0; i < count; i++ {
		e.feedKeys(ks, true)
	}
}

// jumpToMark implements the ' and ` motions. The special marks are:
//
//	'' ``   the position before the last jump
//	<  >    the last visual selection
//	.       the position of the last change
//	[  ]    (approximated with the last change)
//	"       the position when the buffer was last left
func (e *Editor) jumpToMark(k keys.Key, marker keys.Key) {
	name := k.R
	if name == 0 || !k.IsRune() {
		return
	}
	toLine := marker.R != '`'
	jump := func(pos buffer.Pos) {
		e.pushJump(e.cur)
		if toLine {
			pos = buffer.Pos{Line: pos.Line, Col: e.buf.FirstNonBlank(pos.Line).Col}
		}
		e.setCursor(pos)
	}
	switch name {
	case '\'', '`':
		if len(e.jumps) == 0 {
			e.errorf("E20: Mark not set")
			return
		}
		// The entry at the current index is the position we jumped away from,
		// which is what '' and `` go back to. Pushing the current position
		// first makes the command toggle between the two, like vim.
		jump(e.jumps[clamp(e.jumpIdx, 0, len(e.jumps)-1)])
		return
	case '.', '[', ']':
		if len(e.changeList) == 0 {
			e.errorf("E20: Mark not set")
			return
		}
		jump(e.changeList[clamp(e.changeIdx, 0, len(e.changeList)-1)])
		return
	case '"':
		pos, ok := e.cursors[e.buf]
		if !ok {
			e.errorf("E20: Mark not set")
			return
		}
		jump(pos)
		return
	case '<', '>':
		if !e.hasLastVisual {
			e.errorf("E20: Mark not set")
			return
		}
		if name == '<' {
			jump(e.lastVisual[0])
		} else {
			jump(e.lastVisual[1])
		}
		return
	}
	pos, ok := e.marks[name]
	if !ok {
		e.errorf("E20: Mark not set")
		return
	}
	jump(pos)
}

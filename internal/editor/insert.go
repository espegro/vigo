package editor

import (
	"strconv"
	"strings"
	"unicode"

	"vigo/internal/buffer"
	"vigo/internal/keys"
)

// cmdInsert enters insert or replace mode the way the given normal mode key
// would.
func (e *Editor) cmdInsert(r rune) {
	switch r {
	case 'i':
		e.enterInsert()
	case 'I':
		e.setCursor(e.buf.FirstNonBlank(e.cur.Line))
		e.enterInsert()
	case 'a':
		if n := e.buf.LineLen(e.cur.Line); n > 0 {
			e.cur = buffer.Pos{Line: e.cur.Line, Col: e.cur.Col + 1}
			e.desiredCol = e.cur.Col
		}
		e.enterInsert()
	case 'A':
		e.cur = e.buf.EndOfLine(e.cur.Line)
		e.desiredCol = e.cur.Col
		e.enterInsert()
	case 'o':
		e.openLine(true)
	case 'O':
		e.openLine(false)
	case 's':
		n := e.count()
		line := e.buf.LineLen(e.cur.Line)
		to := e.cur.Col + n
		if to > line {
			to = line
		}
		e.changeRange(rangeInfo{from: e.cur, to: buffer.Pos{Line: e.cur.Line, Col: to}, kind: mExclusive}, e.defaultRegister())
	case 'S':
		r := rangeInfo{from: buffer.Pos{Line: e.cur.Line}, to: buffer.Pos{Line: e.cur.Line + 1}, kind: mLine}
		e.changeRange(r, e.defaultRegister())
	case 'C':
		line := e.buf.LineLen(e.cur.Line)
		if e.cur.Col >= line {
			e.enterInsert()
			return
		}
		e.changeRange(rangeInfo{from: e.cur, to: buffer.Pos{Line: e.cur.Line, Col: line}, kind: mExclusive}, e.defaultRegister())
	case 'R':
		e.mode = ModeReplace
		e.replaceStack = nil
		e.inserted = e.inserted[:0]
		e.noteInsert()
	}
}

func (e *Editor) openLine(below bool) {
	indent := ""
	if e.opts.AutoIndent && !e.opts.Paste {
		indent = indentOf(e.buf.LineString(e.cur.Line))
	}
	line := e.cur.Line
	at := line + 1
	if !below {
		at = line
	}
	if below {
		if line < e.buf.LastLine() || e.buf.LineLen(line) > 0 {
			at = line + 1
		} else {
			at = line
		}
	}
	e.buf.Checkpoint(e.cur)
	e.buf.InsertLines(at, [][]rune{[]rune(indent)})
	at = clamp(at, 0, e.buf.LastLine())
	e.setCursor(buffer.Pos{Line: at, Col: len(indent)})
	e.enterInsert()
}

// insertKey handles keys while in insert or replace mode.
func (e *Editor) insertKey(k keys.Key) {
	if k.Paste {
		e.insertPaste(k)
		return
	}
	if k.IsSpecial(keys.Esc) || (k.IsCtrl() && (k.R == 'C' || k.R == '[')) {
		e.leaveInsert()
		return
	}
	if k.IsCtrl() {
		e.insertCtrl(k)
		return
	}
	switch k.Name {
	case keys.CR:
		e.insertNewline()
		return
	case keys.BS:
		e.insertBackspace()
		return
	case keys.Del:
		e.insertDelete()
		return
	case keys.Tab:
		e.insertTab()
		return
	case keys.Up:
		e.moveCursor(e.vertical(e.cur, -1))
		return
	case keys.Down:
		e.moveCursor(e.vertical(e.cur, 1))
		return
	case keys.Left:
		e.moveCursor(buffer.Pos{Line: e.cur.Line, Col: e.cur.Col - 1})
		return
	case keys.Right:
		e.moveCursor(buffer.Pos{Line: e.cur.Line, Col: e.cur.Col + 1})
		return
	case keys.Home:
		e.setCursor(buffer.Pos{Line: e.cur.Line, Col: 0})
		return
	case keys.End:
		e.setCursor(e.buf.EndOfLine(e.cur.Line))
		return
	}
	if !k.IsRune() {
		return
	}
	if k.R == '\t' {
		e.insertTab()
		return
	}
	if k.R < ' ' {
		return
	}
	e.insertRune(k.R)
}

func (e *Editor) insertCtrl(k keys.Key) {
	switch k.R {
	case 'T':
		e.shiftRange(currentLineRange(e.cur.Line), 1, true)
		e.setCursor(buffer.Pos{Line: e.cur.Line, Col: e.buf.FirstNonBlank(e.cur.Line).Col})
	case 'D':
		e.shiftRange(currentLineRange(e.cur.Line), 1, false)
	case 'W':
		e.deleteWordBefore()
	case 'U':
		e.deleteToLineStart()
	case 'R':
		e.insertRegisterPrompt()
	case 'A':
		reg := e.register('.')
		if !reg.Empty() {
			e.insertTextAt(e.cur, reg.Text())
			e.setCursor(buffer.Pos{Line: e.cur.Line, Col: e.cur.Col + len([]rune(reg.Text()))})
		}
	case 'N', 'P':
		e.completeWord(k.R == 'N')
	case 'O':
		e.insertOneNormal(k)
	case 'V':
		e.insertLiteral()
	case 'Y':
		line := e.buf.LineString(e.cur.Line)
		if e.cur.Col < len([]rune(line)) {
			e.insertRune([]rune(line)[e.cur.Col])
		}
	case 'E':
		if e.cur.Line < e.buf.LastLine() {
			l := e.buf.Line(e.cur.Line + 1)
			if e.cur.Col < len(l) {
				e.insertRune(l[e.cur.Col])
			}
		}
	case 'G':
		e.showPosition()
	}
}

func (e *Editor) insertOneNormal(k keys.Key) {
	saved := e.mode
	e.mode = ModeNormal
	e.noteKey(k)
	e.normalKey(k)
	e.afterKey()
	if e.mode == ModeNormal {
		e.mode = saved
	}
}

func (e *Editor) leaveInsert() {
	e.mode = ModeNormal
	e.replaceStack = nil
	e.flushInserted()
	if e.blockInsertActive {
		if e.cur.Col > 0 {
			e.moveCursor(buffer.Pos{Line: e.cur.Line, Col: e.cur.Col - 1})
		}
		e.applyBlockInsert()
		e.pending.clearOp()
		e.pending.awaiting = ""
		e.desiredCol = e.cur.Col
		return
	}
	if e.cur.Col > 0 {
		e.moveCursor(buffer.Pos{Line: e.cur.Line, Col: e.cur.Col - 1})
	}
	e.desiredCol = e.cur.Col
}

func (e *Editor) insertRune(r rune) {
	if r == 0 {
		return
	}
	e.completeActive = false
	e.buf.Checkpoint(e.cur)
	if e.mode == ModeReplace {
		line := e.buf.Line(e.cur.Line)
		if e.cur.Col < len(line) {
			l := append([]rune{}, line...)
			e.replaceStack = append(e.replaceStack, l[e.cur.Col])
			l[e.cur.Col] = r
			e.buf.SetLine(e.cur.Line, l)
			e.moveCursor(buffer.Pos{Line: e.cur.Line, Col: e.cur.Col + 1})
			e.rememberInserted(r)
			return
		}
	}
	pos := e.buf.Insert(e.cur, []rune{r})
	e.cur = pos
	e.desiredCol = pos.Col
	e.rememberInserted(r)
	if e.blockInsertActive {
		e.blockTextTyped = append(e.blockTextTyped, r)
	}
}

// rememberInserted appends to the text of the last insert session. The
// register itself is only built when somebody reads it, so inserting (or
// pasting) a lot of text stays linear.
func (e *Editor) rememberInserted(r rune) {
	e.inserted = append(e.inserted, r)
}

// flushInserted turns the accumulated insert text into register ".".
func (e *Editor) flushInserted() {
	if len(e.inserted) == 0 {
		return
	}
	e.regs['.'] = Register{Lines: []string{string(e.inserted)}}
}

func (e *Editor) insertNewline() {
	e.buf.Checkpoint(e.cur)
	indent := ""
	if e.opts.AutoIndent && !e.opts.Paste {
		indent = indentOf(e.buf.LineString(e.cur.Line))
	}
	e.buf.SplitLine(e.cur)
	e.cur = buffer.Pos{Line: e.cur.Line + 1, Col: 0}
	e.desiredCol = 0
	if indent != "" {
		e.buf.Insert(e.cur, []rune(indent))
		e.cur = buffer.Pos{Line: e.cur.Line, Col: len(indent)}
	}
	e.inserted = append(e.inserted, []rune("\n"+indent)...)
}

func (e *Editor) insertBackspace() {
	// In replace mode a backspace puts the overwritten character back.
	if e.mode == ModeReplace && len(e.replaceStack) > 0 && e.cur.Col > 0 {
		restored := e.replaceStack[len(e.replaceStack)-1]
		e.replaceStack = e.replaceStack[:len(e.replaceStack)-1]
		line := append([]rune{}, e.buf.Line(e.cur.Line)...)
		if e.cur.Col-1 < len(line) {
			line[e.cur.Col-1] = restored
			e.buf.SetLine(e.cur.Line, line)
			e.moveCursor(buffer.Pos{Line: e.cur.Line, Col: e.cur.Col - 1})
			return
		}
	}
	if e.cur.Col == 0 {
		if e.cur.Line == 0 {
			return
		}
		e.buf.Checkpoint(e.cur)
		prev := e.buf.LineLen(e.cur.Line - 1)
		e.buf.JoinLines(e.cur.Line-1, false)
		e.setCursor(buffer.Pos{Line: e.cur.Line - 1, Col: prev})
		return
	}
	e.buf.Checkpoint(e.cur)
	e.buf.DeleteRange(buffer.Pos{Line: e.cur.Line, Col: e.cur.Col - 1}, buffer.Pos{Line: e.cur.Line, Col: e.cur.Col})
	e.moveCursor(buffer.Pos{Line: e.cur.Line, Col: e.cur.Col - 1})
}

func (e *Editor) insertDelete() {
	if e.cur.Col >= e.buf.LineLen(e.cur.Line) {
		if e.cur.Line >= e.buf.LastLine() {
			return
		}
		e.buf.Checkpoint(e.cur)
		e.buf.JoinLines(e.cur.Line, false)
		return
	}
	e.buf.Checkpoint(e.cur)
	e.buf.DeleteRange(buffer.Pos{Line: e.cur.Line, Col: e.cur.Col}, buffer.Pos{Line: e.cur.Line, Col: e.cur.Col + 1})
}

func (e *Editor) insertTab() {
	e.buf.Checkpoint(e.cur)
	if e.opts.ExpandTab && !e.opts.Paste {
		ts := e.opts.TabStop
		if ts <= 0 {
			ts = 8
		}
		col := e.visualCol(e.cur)
		n := ts - col%ts
		e.buf.Insert(e.cur, []rune(strings.Repeat(" ", n)))
		e.cur = buffer.Pos{Line: e.cur.Line, Col: e.cur.Col + n}
		return
	}
	e.buf.Insert(e.cur, []rune{'\t'})
	e.cur = buffer.Pos{Line: e.cur.Line, Col: e.cur.Col + 1}
}

// visualCol is the screen column of a cursor position, expanding tabs.
func (e *Editor) visualCol(p buffer.Pos) int {
	ts := e.opts.TabStop
	if ts <= 0 {
		ts = 8
	}
	col := 0
	for i, r := range e.buf.Line(p.Line) {
		if i >= p.Col {
			break
		}
		if r == '\t' {
			col += ts - col%ts
		} else {
			col++
		}
	}
	return col
}

func (e *Editor) deleteWordBefore() {
	line := []rune(e.buf.LineString(e.cur.Line))
	col := e.cur.Col
	start := col
	for start > 0 && unicode.IsSpace(line[start-1]) {
		start--
	}
	cls := cWhite
	if start > 0 {
		cls = charClass(line[start-1], false)
	}
	for start > 0 && charClass(line[start-1], false) == cls {
		start--
	}
	if start == col {
		if col == 0 {
			e.insertBackspace()
			return
		}
		start = col - 1
	}
	e.buf.Checkpoint(e.cur)
	e.buf.DeleteRange(buffer.Pos{Line: e.cur.Line, Col: start}, buffer.Pos{Line: e.cur.Line, Col: col})
	e.setCursor(buffer.Pos{Line: e.cur.Line, Col: start})
}

func (e *Editor) deleteToLineStart() {
	if e.cur.Col == 0 {
		return
	}
	e.buf.Checkpoint(e.cur)
	e.buf.DeleteRange(buffer.Pos{Line: e.cur.Line, Col: 0}, buffer.Pos{Line: e.cur.Line, Col: e.cur.Col})
	e.setCursor(buffer.Pos{Line: e.cur.Line, Col: 0})
}

func (e *Editor) insertRegisterPrompt() {
	e.drawPrompt(`"`)
	k, err := e.nextKey()
	if err != nil {
		e.quit = true
		return
	}
	reg := e.defaultRegister()
	if k.IsRune() {
		reg = k.R
		if reg == '=' {
			text, ok := e.ask("=")
			if !ok {
				return
			}
			e.buf.Checkpoint(e.cur)
			e.insertTextAt(e.cur, text)
			return
		}
	}
	content := e.register(reg)
	if content.Empty() {
		return
	}
	e.buf.Checkpoint(e.cur)
	e.insertTextAt(e.cur, content.Text())
}

func (e *Editor) insertLiteral() {
	k, err := e.nextKey()
	if err != nil {
		e.quit = true
		return
	}
	if k.IsRune() {
		e.insertRune(k.R)
		return
	}
	switch k.Name {
	case keys.CR:
		e.insertRune('\n')
	case keys.Tab:
		e.insertRune('\t')
	}
}

// completeWord inserts the next buffer word that starts with the prefix before
// the cursor.
func (e *Editor) completeWord(next bool) {
	line := e.buf.Line(e.cur.Line)
	col := clamp(e.cur.Col, 0, len(line))
	start := col
	for start > 0 {
		r := line[start-1]
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' {
			start--
			continue
		}
		break
	}
	word := string(line[start:col])
	// The first ctrl-n starts a completion and remembers the prefix; further
	// presses cycle through the candidates of that same prefix, like vim.
	if !e.completeActive || e.completeBase != start {
		e.completeActive = true
		e.completeBase = start
		e.completeWordPrefix = word
		e.completeIdx = -1
	}
	candidates := e.completions(e.completeWordPrefix)
	if len(candidates) == 0 {
		e.completeActive = false
		e.messagef("Pattern not found")
		return
	}
	if next {
		e.completeIdx = (e.completeIdx + 1) % len(candidates)
	} else {
		e.completeIdx--
		if e.completeIdx < 0 {
			e.completeIdx = len(candidates) - 1
		}
	}
	chosen := candidates[e.completeIdx]
	e.buf.Checkpoint(e.cur)
	e.buf.DeleteRange(buffer.Pos{Line: e.cur.Line, Col: e.completeBase}, buffer.Pos{Line: e.cur.Line, Col: e.cur.Col})
	e.buf.Insert(buffer.Pos{Line: e.cur.Line, Col: e.completeBase}, []rune(chosen))
	e.cur = buffer.Pos{Line: e.cur.Line, Col: e.completeBase + len([]rune(chosen))}
	e.desiredCol = e.cur.Col
}

func (e *Editor) completions(prefix string) []string {
	seen := map[string]bool{}
	var out []string
	add := func(w string) {
		if w == "" || w == prefix || seen[w] {
			return
		}
		if !strings.HasPrefix(w, prefix) {
			return
		}
		seen[w] = true
		out = append(out, w)
	}
	for i := 0; i <= e.buf.LastLine(); i++ {
		for _, w := range splitWords(e.buf.LineString(i)) {
			add(w)
		}
	}
	return out
}

func splitWords(s string) []string {
	return strings.FieldsFunc(s, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_'
	})
}

// insertPaste inserts one non character key of a bracketed paste (tabs and
// newlines are handled by the batched path).
func (e *Editor) insertPaste(k keys.Key) {
	if k.IsSpecial(keys.CR) {
		e.buf.Checkpoint(e.cur)
		e.buf.SplitLine(e.cur)
		e.cur = buffer.Pos{Line: e.cur.Line + 1, Col: 0}
		e.inserted = append(e.inserted, '\n')
	}
}

// handlePasteKeys inserts a run of pasted keys. Consecutive characters are
// inserted in one go, which keeps a large paste linear instead of quadratic.
func (e *Editor) handlePasteKeys(ks []keys.Key) {
	var runes []rune
	flush := func() {
		if len(runes) == 0 {
			return
		}
		text := string(runes)
		runes = runes[:0]
		e.buf.Checkpoint(e.cur)
		end := e.buf.InsertText(e.cur, text)
		e.cur = end
		e.desiredCol = end.Col
		e.inserted = append(e.inserted, []rune(text)...)
	}
	for _, k := range ks {
		switch e.mode {
		case ModeCmdline, ModeSearchForward, ModeSearchBackward:
			if k.IsRune() {
				e.cmdInsertRune(k.R)
			}
		default:
			switch {
			case k.IsSpecial(keys.CR):
				runes = append(runes, '\n')
			case k.IsRune():
				runes = append(runes, k.R)
			default:
				flush()
				e.insertPaste(k)
			}
		}
	}
	flush()
}

func currentLineRange(line int) rangeInfo {
	return rangeInfo{from: buffer.Pos{Line: line}, to: buffer.Pos{Line: line + 1}, kind: mLine}
}

func itoa(n int) string { return strconv.Itoa(n) }

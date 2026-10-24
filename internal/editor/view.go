package editor

import (
	"fmt"
	"strings"

	"vigo/internal/buffer"
	"vigo/internal/screen"
)

// viewRow is one screen line in the text area.
type viewRow struct {
	line       int
	start, end int
	seg        int
	// disp is the screen column of start, so that drawing a row does not have
	// to walk the line from the beginning. Without it a single very long line
	// costs O(line length) per row and per frame.
	disp int
}

// seg is one displayed slice of a line.
type seg struct {
	start, end int
	disp       int
}

type scrollTarget int

const (
	Center scrollTarget = iota
	Top
	Bottom
)

// viewHeight is the number of text rows.
func (e *Editor) viewHeight() int {
	if e.win != nil && e.win.textH > 0 {
		return e.win.textH
	}
	_, h := e.scr.Size()
	if h-2 > 0 {
		return h - 2
	}
	return 1
}

// measureLayout computes the gutter and text widths.
func (e *Editor) measureLayout() {
	w := e.winWidth()
	e.gutterW = 0
	if e.opts.Number || e.opts.RelativeNumber {
		digits := len(itoa(e.buf.LastLine() + 1))
		if digits < e.opts.NumberWidth {
			digits = e.opts.NumberWidth
		}
		if digits < 1 {
			digits = 4
		}
		e.gutterW = digits + 1
	}
	e.textWidth = w - e.gutterW
	if e.textWidth < 1 {
		e.textWidth = 1
	}
}

func (e *Editor) winWidth() int {
	if e.win != nil && e.win.w > 0 {
		return e.win.w
	}
	w, _ := e.scr.Size()
	return w
}

// lineSegments returns the displayed rune ranges of a line, honouring wrap and
// the horizontal scroll offset.
func (e *Editor) lineSegments(line int) []seg {
	if e.segCacheValid() {
		if segs, ok := e.segCache[line]; ok {
			return segs
		}
	} else {
		e.segCache = map[int][]seg{}
		e.segCacheVersion = e.buf.Version()
		e.segCacheWidth = e.textWidth
		e.segCacheWrap = e.opts.Wrap
	}
	segs := e.computeSegments(line)
	e.segCache[line] = segs
	return segs
}

// segCacheValid reports whether the cached segments belong to the current
// buffer content and layout. Without this check a cache hit could hand back
// ranges that no longer fit the text, which is a crash.
func (e *Editor) segCacheValid() bool {
	return e.segCache != nil &&
		e.segCacheVersion == e.buf.Version() &&
		e.segCacheWidth == e.textWidth &&
		e.segCacheWrap == e.opts.Wrap
}

// computeSegments does the actual work for lineSegments. The result is cached
// per line because a single very long line would otherwise be scanned again for
// every screen row and for every cursor query in the same frame.
func (e *Editor) computeSegments(line int) []seg {
	n := e.buf.LineLen(line)
	if !e.opts.Wrap {
		start := clamp(e.leftcol, 0, n)
		end := start + e.textWidth
		if end > n {
			end = n
		}
		return []seg{{start: start, end: end, disp: e.displayColRange(line, 0, start)}}
	}
	if n == 0 {
		return []seg{{}}
	}
	runes := e.buf.Line(line)
	var out []seg
	start := 0
	used := 0
	displayStart := 0
	ts := e.tabStop()
	i := 0
	for i < n {
		w := screen.RuneWidth(runes[i])
		if runes[i] == '\t' {
			w = ts - (used % ts)
		}
		if used+w > e.textWidth {
			if used == 0 {
				i++
				used = 0
			}
			out = append(out, seg{start: start, end: i, disp: displayStart})
			displayStart += used
			start = i
			used = 0
			continue
		}
		used += w
		i++
	}
	out = append(out, seg{start: start, end: n, disp: displayStart})
	return out
}

func (e *Editor) tabStop() int {
	if e.opts.TabStop > 0 {
		return e.opts.TabStop
	}
	return 8
}

// segmentIndexFor returns the index of the segment holding a rune column.
func (e *Editor) segmentIndexFor(line, col int) int {
	segs := e.lineSegments(line)
	if col < 0 {
		col = 0
	}
	for i, s := range segs {
		if col < s.end {
			return i
		}
	}
	return len(segs) - 1
}

// computeRows fills the visible rows starting from the scroll position.
func (e *Editor) computeRows() {
	height := e.viewHeight()
	e.rows = e.rows[:0]
	line := clamp(e.topline, 0, e.buf.LastLine())
	seg := e.topSeg
	if seg < 0 {
		seg = 0
	}
	segs := e.lineSegments(line)
	if seg >= len(segs) {
		seg = 0
		line++
	}
	for len(e.rows) < height && line <= e.buf.LastLine() {
		segs = e.lineSegments(line)
		for seg < len(segs) && len(e.rows) < height {
			e.rows = append(e.rows, viewRow{line: line, start: segs[seg].start, end: segs[seg].end, seg: seg, disp: segs[seg].disp})
			seg++
		}
		line++
		seg = 0
	}
	for len(e.rows) < height {
		e.rows = append(e.rows, viewRow{line: -1})
	}
}

// prevSeg and nextSeg step the scroll position by one screen row.
func (e *Editor) prevSeg(line, seg int) (int, int) {
	if seg > 0 {
		return line, seg - 1
	}
	if line <= 0 {
		return -1, -1
	}
	prev := line - 1
	segs := e.lineSegments(prev)
	return prev, len(segs) - 1
}

func (e *Editor) nextSeg(line, seg int) (int, int) {
	segs := e.lineSegments(line)
	if seg+1 < len(segs) {
		return line, seg + 1
	}
	next := line + 1
	if next > e.buf.LastLine() {
		return -1, -1
	}
	return next, 0
}

// ensureCursorVisible scrolls so that the cursor is on screen.
func (e *Editor) ensureCursorVisible() {
	e.measureLayout()
	height := e.viewHeight()
	so := e.opts.ScrollOff
	if so > height/2 {
		so = height / 2
	}
	if e.topline > e.buf.LastLine() {
		e.topline = e.buf.LastLine()
		e.topSeg = 0
	}
	curSeg := e.segmentIndexFor(e.cur.Line, e.cur.Col)
	e.computeRows()
	idx := -1
	for i, r := range e.rows {
		if r.line == e.cur.Line && r.seg == curSeg {
			idx = i
			break
		}
	}
	switch {
	case idx < 0:
		if e.cur.Line < e.topline || (e.cur.Line == e.topline && curSeg < e.topSeg) {
			e.topline, e.topSeg = e.cur.Line, curSeg
			for i := 0; i < so; i++ {
				l, s := e.prevSeg(e.topline, e.topSeg)
				if l < 0 {
					break
				}
				e.topline, e.topSeg = l, s
			}
		} else {
			e.topline, e.topSeg = e.cur.Line, curSeg
			for i := 0; i < height-1-so; i++ {
				l, s := e.prevSeg(e.topline, e.topSeg)
				if l < 0 {
					break
				}
				e.topline, e.topSeg = l, s
			}
		}
	case idx < so:
		for i := idx; i < so; i++ {
			l, s := e.prevSeg(e.topline, e.topSeg)
			if l < 0 {
				break
			}
			e.topline, e.topSeg = l, s
		}
	case idx >= height-so:
		need := idx - (height - 1 - so)
		for i := 0; i < need; i++ {
			l, s := e.nextSeg(e.topline, e.topSeg)
			if l < 0 {
				break
			}
			e.topline, e.topSeg = l, s
		}
	}
	e.computeRows()
	e.ensureLeftCol()
}

func (e *Editor) ensureLeftCol() {
	if e.opts.Wrap {
		e.leftcol = 0
		return
	}
	so := e.opts.SideScrollOff
	if e.cur.Col < e.leftcol {
		e.leftcol = e.cur.Col
		if e.leftcol > so {
			e.leftcol -= so
		} else {
			e.leftcol = 0
		}
	}
	if e.cur.Col >= e.leftcol+e.textWidth {
		e.leftcol = e.cur.Col - e.textWidth + 1 + so
		if e.leftcol < 0 {
			e.leftcol = 0
		}
	}
}

// cursorRow returns the screen row holding the cursor.
func (e *Editor) cursorRow() int {
	curSeg := e.segmentIndexFor(e.cur.Line, e.cur.Col)
	for i, r := range e.rows {
		if r.line == e.cur.Line && r.seg == curSeg {
			return i
		}
	}
	return -1
}

// scrollToCursor implements zt, zz and zb.
func (e *Editor) scrollToCursor(target scrollTarget) {
	height := e.viewHeight()
	curSeg := e.segmentIndexFor(e.cur.Line, e.cur.Col)
	back := 0
	switch target {
	case Center:
		back = height / 2
	case Bottom:
		back = height - 1
	}
	e.topline, e.topSeg = e.cur.Line, curSeg
	for i := 0; i < back; i++ {
		l, s := e.prevSeg(e.topline, e.topSeg)
		if l < 0 {
			break
		}
		e.topline, e.topSeg = l, s
	}
	e.computeRows()
}

func (e *Editor) scrollLines(n int) {
	if n == 0 {
		return
	}
	for i := 0; i < abs(n); i++ {
		var l, s int
		if n > 0 {
			l, s = e.nextSeg(e.topline, e.topSeg)
		} else {
			l, s = e.prevSeg(e.topline, e.topSeg)
		}
		if l < 0 {
			break
		}
		e.topline, e.topSeg = l, s
	}
	e.computeRows()
}

func (e *Editor) scrollPages(n int) {
	lines := e.viewHeight() - 2
	if lines < 1 {
		lines = 1
	}
	e.scrollWithCursor(n * lines)
}

// scrollWithCursor scrolls the window and keeps the cursor on the same screen
// row, which is what vim's ctrl-d, ctrl-u, ctrl-f and ctrl-b do.
func (e *Editor) scrollWithCursor(n int) {
	if n == 0 {
		return
	}
	e.computeRows()
	row := e.cursorRow()
	e.scrollLines(n)
	if row >= 0 && row < len(e.rows) {
		if r := e.rows[row]; r.line >= 0 {
			e.moveCursor(buffer.Pos{Line: r.line, Col: e.cur.Col})
		}
	}
	e.ensureCursorVisible()
}

func (e *Editor) cursorLine() {
	if e.cur.Line > e.topline {
		e.setCursor(buffer.Pos{Line: e.topline, Col: 0})
		e.setCursor(e.buf.FirstNonBlank(e.cur.Line))
	}
}

func (e *Editor) horizontalScroll(n int) {
	e.leftcol += n
	if e.leftcol < 0 {
		e.leftcol = 0
	}
	e.ensureCursorVisible()
}

func (e *Editor) horizontalScrollCursor(hint int) {
	switch {
	case hint < 0:
		e.leftcol = 0
	case hint > 0:
		line := e.buf.LineLen(e.cur.Line)
		e.leftcol = line - e.textWidth + 1
		if e.leftcol < 0 {
			e.leftcol = 0
		}
	default:
		e.leftcol = e.cur.Col - e.textWidth/2
		if e.leftcol < 0 {
			e.leftcol = 0
		}
	}
	e.ensureCursorVisible()
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// displayColRange is the screen width of the runes between two columns of the
// same line.
func (e *Editor) displayColRange(line, from, to int) int {
	ts := e.tabStop()
	w := 0
	runes := e.buf.Line(line)
	if from < 0 {
		from = 0
	}
	for i := from; i < to && i < len(runes); i++ {
		if runes[i] == '\t' {
			w += ts - w%ts
			continue
		}
		w += screen.RuneWidth(runes[i])
	}
	return w
}

// displayCol is the screen column of a rune column, expanding tabs.
func (e *Editor) displayCol(line, col int) int {
	ts := e.tabStop()
	w := 0
	runes := e.buf.Line(line)
	for i := 0; i < col && i < len(runes); i++ {
		if runes[i] == '\t' {
			w += ts - w%ts
			continue
		}
		w += screen.RuneWidth(runes[i])
	}
	return w
}

// draw renders the whole screen.
func (e *Editor) draw() {
	e.updateSearchHighlight()
	e.saveLiveToWindow()
	e.layoutWindows()
	e.eachWindow(func(w *window) {
		for i, row := range e.rows {
			e.drawRow(w, i, row)
		}
		e.drawStatusline(w)
	})
	e.drawSeparators()
	e.measureLayout()
	e.updateCursorShape()
	s := e.scr
	if e.forceRedraw {
		e.forceRedraw = false
		w, h := s.Size()
		s.Resize(w, h)
	}
	e.drawCommandline()
	e.positionCursor()
	s.Flush(e.cursorX, e.cursorY, e.cursorVisible())
}

// eachWindow runs fn for every window with the window's state made current.
func (e *Editor) eachWindow(fn func(w *window)) {
	current := e.win
	s := e.scr
	w, h := s.Size()
	s.Clear(e.theme.normal)
	for _, win := range e.windows {
		e.loadWindowState(win)
		e.measureLayout()
		e.computeRows()
		e.ensureCursorVisible()
		e.cache()
		e.updateSearchHighlight()
		fn(win)
	}
	if current != nil {
		e.loadWindowState(current)
	}
	e.measureLayout()
	e.computeRows()
	e.ensureCursorVisible()
	_, _ = w, h
}

// drawSeparators draws the vertical dividers between side by side windows.
func (e *Editor) drawSeparators() {
	for _, s := range e.separators {
		for y := 0; y < s.h; y++ {
			e.scr.Set(s.x, s.y+y, '│', e.theme.statusLineNC)
		}
	}
}

// updateCursorShape uses a bar in insert mode and a block otherwise, like vim.
func (e *Editor) updateCursorShape() {
	if e.term == nil {
		return
	}
	shape := screen.CursorBlock
	switch e.mode {
	case ModeInsert, ModeReplace:
		shape = screen.CursorBar
	}
	if shape != e.cursorShape {
		e.cursorShape = shape
		e.term.SetCursorShape(shape)
	}
}

func (e *Editor) cursorVisible() bool {
	return true
}

func (e *Editor) drawRow(win *window, y int, row viewRow) {
	w := e.winWidth()
	rect := e.scr.Rect(win.x, win.y+y, w, 1)
	isCursorLine := row.line == e.cur.Line
	base := e.theme.normal
	if e.opts.CursorLine && isCursorLine {
		base = e.theme.cursorLine
	}
	rect.FillSpace(0, 0, w, base)
	if e.opts.CursorColumn && isCursorLine {
		e.highlightCursorColumn(rect, win)
	}
	if e.gutterW > 0 {
		e.drawGutter(rect, row, isCursorLine)
	}
	if row.line < 0 {
		rect.Text(e.gutterW, 0, 1, e.theme.nonText, "~")
		return
	}
	e.drawText(rect, row, base, isCursorLine)
}

func (e *Editor) drawGutter(rect screen.Rect, row viewRow, isCursorLine bool) {
	attr := e.theme.lineNr
	if isCursorLine {
		attr = e.theme.cursorLineNr
	}
	width := e.gutterW - 1
	if row.line < 0 {
		return
	}
	num := ""
	switch {
	case e.opts.Number && e.opts.RelativeNumber && !isCursorLine:
		num = itoa(abs(row.line - e.cur.Line))
	case e.opts.Number:
		num = itoa(row.line + 1)
	case e.opts.RelativeNumber:
		if isCursorLine {
			num = itoa(row.line + 1)
		} else {
			num = itoa(abs(row.line - e.cur.Line))
		}
	}
	if num == "" {
		return
	}
	if len(num) < width {
		num = strings.Repeat(" ", width-len(num)) + num
	}
	rect.Text(0, 0, width, attr, num)
}

// drawText draws the text of one row with highlighting.
func (e *Editor) drawText(rect screen.Rect, row viewRow, base screen.Attr, isCursorLine bool) {
	line := e.buf.Line(row.line)
	syn := e.syntaxSpans(row.line)
	searches := e.searchRangeIn(row.line)
	sel := e.selectionOnRow(row.line)
	ts := e.tabStop()
	x := e.gutterW
	col := row.start
	display := row.disp
	limit := rect.W
	for col < row.end && x < limit {
		if col >= len(line) {
			break
		}
		attr := base
		if g, ok := spanGroupAt(syn, col); ok {
			attr = e.syntaxAttr(g, base)
		}
		if m, ok := matchAt(searches, col); ok && e.opts.HlSearch {
			attr = e.theme.search
			_ = m
		}
		if sel != nil && col >= sel[0] && col < sel[1] {
			attr = e.theme.visual
		}
		if e.matchParenAt(row.line, col) {
			attr = e.theme.matchParen
		}
		r := line[col]
		switch {
		case r == '\t':
			width := ts - display%ts
			if e.opts.List && width >= 2 && x+1 < limit {
				rect.Set(x, 0, '^', attr)
				rect.Set(x+1, 0, 'I', attr)
				x += 2
				display += 2
				for i := 2; i < width && x < limit; i++ {
					rect.Set(x, 0, ' ', attr)
					x++
					display++
				}
				col++
				continue
			}
			for i := 0; i < width && x < limit; i++ {
				rect.Set(x, 0, ' ', attr)
				x++
			}
			display += width
		case r < ' ':
			rect.Set(x, 0, '^', attr)
			x++
			if x < limit {
				rect.Set(x, 0, r+0x40, attr)
				x++
			}
			display += 2
		default:
			width := screen.RuneWidth(r)
			if x+width > limit {
				if e.opts.Wrap {
					return
				}
				rect.Set(limit-1, 0, '@', attr)
				return
			}
			rect.Set(x, 0, r, attr)
			x += width
			display += width
		}
		col++
	}
	if e.opts.List && col >= row.end && x < limit {
		rect.Set(x, 0, '$', e.theme.nonText)
		x++
	}
	if !e.opts.Wrap && row.end < len(line) && limit >= 2 {
		rect.Set(limit-1, 0, '@', e.theme.nonText)
	}
	if row.start > 0 && !e.opts.Wrap && e.leftcol > 0 {
		rect.Set(e.gutterW, 0, '<', e.theme.nonText)
	}
}

func (e *Editor) syntaxAttr(g syntaxGroup, base screen.Attr) screen.Attr {
	a, ok := e.theme.syntax[g]
	if !ok {
		return base
	}
	if base != e.theme.normal {
		if a.BG.Set && e.theme.visual.BG.Set {
			a.BG = e.theme.visual.BG
		}
		if e.opts.CursorLine {
			a.BG = e.theme.cursorLine.BG
		}
	}
	return a
}

func (e *Editor) matchParenAt(line, col int) bool {
	if !e.opts.ShowMatch {
		return false
	}
	if e.mode != ModeNormal {
		return false
	}
	if line != e.cur.Line || col != e.cur.Col {
		return false
	}
	r := e.buf.RuneAt(buffer.Pos{Line: line, Col: col})
	_, ok := bracketPairs[r]
	if !ok {
		return false
	}
	if dest, ok := e.matchBracket(e.cur); ok {
		_ = dest
	}
	return true
}

// selectionOnRow returns the column range selected on a line.
func (e *Editor) selectionOnRow(line int) []int {
	if !e.mode.visual() {
		return nil
	}
	r := e.visualRange()
	nr := e.normalizeRange(r)
	if line < nr.from.Line || line > nr.to.Line {
		return nil
	}
	if r.kind == mLine || e.mode == ModeVisualLine {
		return []int{0, 1 << 20}
	}
	if r.block || e.mode == ModeVisualBlock {
		from, to, c1, c2 := e.blockBounds(r)
		if line < from || line > to {
			return nil
		}
		return []int{c1, c2}
	}
	start, end := 0, 1<<20
	if line == nr.from.Line {
		start = nr.from.Col
	}
	if line == nr.to.Line {
		end = nr.to.Col
	}
	if end < start {
		end = start
	}
	return []int{start, end}
}

func (e *Editor) drawStatusline(win *window) {
	if win.statusH == 0 {
		return
	}
	w := e.winWidth()
	y := win.y + win.h - 1
	rect := e.scr.Rect(win.x, y, w, 1)
	attr := e.theme.statusLine
	if win != e.win {
		attr = e.theme.statusLineNC
	}
	rect.FillSpace(0, 0, w, attr)
	name := e.buf.Path()
	if name == "" {
		name = "[No Name]"
	}
	var flags strings.Builder
	if e.buf.Modified() {
		flags.WriteString("[+]")
	}
	if e.buf.ReadOnly() {
		flags.WriteString("[RO]")
	}
	if e.buf.MissingEOL() {
		flags.WriteString("[noeol]")
	}
	if e.recording {
		fmt.Fprintf(&flags, "[recording @%c]", e.recordReg)
	}
	ft := ""
	if t := e.fileType(); t != "" {
		ft = fmt.Sprintf("[%s]", t)
	}
	left := fmt.Sprintf("%s %s %s", name, flags.String(), ft)
	pct := 100
	if e.buf.Len() > 1 {
		pct = (e.cur.Line + 1) * 100 / e.buf.Len()
	}
	right := fmt.Sprintf(" %d,%-3d %3d%% ", e.cur.Line+1, e.cur.Col+1, pct)
	if e.opts.Ruler {
		right = fmt.Sprintf(" %d,%-3d %3d%% ", e.cur.Line+1, e.cur.Col+1, pct)
	} else {
		right = " "
	}
	rect.Text(0, 0, w, attr, left)
	start := w - len(right)
	if start > len(left) {
		rect.Text(start, 0, len(right), attr, right)
	} else {
		rect.Text(len(left)+1, 0, w-len(left)-1, attr, right)
	}
}

func (e *Editor) drawCommandline() {
	w, h := e.scr.Size()
	rect := e.scr.Rect(0, h-1, w, 1)
	rect.FillSpace(0, 0, w, e.theme.cmdline)
	attr := e.theme.cmdline
	switch e.mode {
	case ModeCmdline, ModeSearchForward, ModeSearchBackward:
		text := e.cmdPrompt() + string(e.cmd.text)
		rect.Text(0, 0, w, attr, text)
		return
	}
	if e.msg != "" {
		if e.msgError {
			attr = e.theme.error
		}
		rect.Text(0, 0, w, attr, e.msg)
		return
	}
	mode := e.mode.String()
	if e.opts.ShowMode && mode != "" {
		rect.Text(0, 0, w, attr, mode)
	}
	if e.opts.ShowCmd {
		pending := e.pendingString()
		if pending != "" {
			x := w - len(pending)
			if x > 0 {
				rect.Text(x, 0, len(pending), attr, pending)
			}
		}
	}
}

func (e *Editor) pendingString() string {
	var sb strings.Builder
	if e.pending.countSet {
		sb.WriteString(itoa(e.pending.count))
	}
	if e.pending.regSet {
		sb.WriteString(`"`)
		sb.WriteRune(e.pending.reg)
	}
	if e.pending.op != "" {
		sb.WriteString(e.pending.op)
	}
	if e.pending.opCount > 0 {
		sb.WriteString(itoa(e.pending.opCount))
	}
	if sb.Len() == 0 {
		return ""
	}
	return sb.String()
}

// positionCursor works out where the block cursor goes.
func (e *Editor) positionCursor() {
	ox, oy := 0, 0
	if e.win != nil {
		ox, oy = e.win.x, e.win.y
	}
	if e.mode == ModeCmdline || e.mode == ModeSearchForward || e.mode == ModeSearchBackward {
		_, h := e.scr.Size()
		text := e.cmdPrompt() + string(e.cmd.text[:clamp(e.cmd.col, 0, len(e.cmd.text))])
		x := screen.StringWidth(text)
		w, _ := e.scr.Size()
		if x >= w {
			x = w - 1
		}
		e.cursorX, e.cursorY = x, h-1
		return
	}
	row := e.cursorRow()
	if row < 0 {
		e.cursorX, e.cursorY = 0, 0
		return
	}
	r := e.rows[row]
	// The cursor x is measured from the start of its row, while drawing a row
	// needs the absolute column (row.disp) for tab stops.
	display := e.displayColRange(e.cur.Line, r.start, e.cur.Col)
	x := ox + e.gutterW + display
	w, _ := e.scr.Size()
	if x >= w {
		x = w - 1
	}
	e.cursorX, e.cursorY = x, oy+row
}

func (e *Editor) showPosition() {
	pct := 100
	if e.buf.Len() > 1 {
		pct = (e.cur.Line + 1) * 100 / e.buf.Len()
	}
	e.messagef("line %d of %d --%d%%-- col %d", e.cur.Line+1, e.buf.Len(), pct, e.cur.Col+1)
}

// highlightCursorColumn paints the cursor column across the window, like
// 'cursorcolumn'.
func (e *Editor) highlightCursorColumn(rect screen.Rect, win *window) {
	x := e.gutterW + e.displayColRange(e.cur.Line, 0, e.cur.Col)
	if x < 0 || x >= rect.W {
		return
	}
	for y := 0; y < win.textH; y++ {
		row := e.rowsAt(y)
		if row == nil || row.line < 0 {
			continue
		}
		col := e.colAtDisplay(row, x-e.gutterW)
		if col < 0 {
			continue
		}
		cell := e.scr.CellAt(rect.X+x, rect.Y+y)
		attr := cell.A
		attr.BG = e.theme.cursorLine.BG
		if !attr.BG.Set {
			attr.Reverse = true
		}
		e.scr.Set(rect.X+x, rect.Y+y, cell.R, attr)
	}
}

func (e *Editor) rowsAt(y int) *viewRow {
	if y < 0 || y >= len(e.rows) {
		return nil
	}
	return &e.rows[y]
}

// colAtDisplay maps a display column back to a rune column in a row.
func (e *Editor) colAtDisplay(row *viewRow, display int) int {
	if display < 0 {
		return -1
	}
	col := row.start
	used := 0
	for col < row.end {
		w := screen.RuneWidth(e.buf.RuneAt(buffer.Pos{Line: row.line, Col: col}))
		if e.buf.RuneAt(buffer.Pos{Line: row.line, Col: col}) == '\t' {
			ts := e.tabStop()
			w = ts - used%ts
		}
		if used+w > display {
			return col
		}
		used += w
		col++
	}
	if col == row.end && used == display {
		return col
	}
	return -1
}

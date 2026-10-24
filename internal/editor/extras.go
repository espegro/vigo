package editor

import (
	"io"
	"os"
	"path/filepath"
	"strings"

	"vigo/internal/buffer"
)

func (e *Editor) historyAdd(text string) {
	if strings.TrimSpace(text) == "" {
		return
	}
	if len(e.cmdHistory) == 0 || e.cmdHistory[len(e.cmdHistory)-1] != text {
		e.cmdHistory = append(e.cmdHistory, text)
	}
	if len(e.cmdHistory) > 200 {
		e.cmdHistory = e.cmdHistory[1:]
	}
}

func readAll(r io.Reader) ([]byte, error) { return io.ReadAll(r) }

func splitLines(s string) [][]rune {
	b := buffer.FromBytes([]byte(s))
	out := make([][]rune, b.Len())
	for i := 0; i < b.Len(); i++ {
		out[i] = append([]rune{}, b.Line(i)...)
	}
	if len(out) > 0 && len(out[len(out)-1]) == 0 && strings.HasSuffix(s, "\n") {
		out = out[:len(out)-1]
	}
	if len(out) == 0 {
		out = [][]rune{{}}
	}
	return out
}

// writeCurrent writes the current buffer to its file (used by ZZ).
func (e *Editor) writeCurrent() {
	if e.buf.Path() == "" {
		e.errorf("E32: No file name")
		return
	}
	if err := e.writeBuffer(e.buf, e.buf.Path(), false); err != nil {
		e.errorf("%v", err)
	}
}

// stopRecording ends macro recording and drops the trailing q from the macro.
func (e *Editor) stopRecording() {
	if !e.recording {
		return
	}
	e.recording = false
	if m := e.macros[e.recordReg]; len(m) > 0 && m[len(m)-1].Is('q') {
		e.macros[e.recordReg] = m[:len(m)-1]
	}
}

// writeTestFile is a tiny helper for the window tests.
func writeTestFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0o644)
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// showWordCount implements g ctrl-g.
func (e *Editor) showWordCount() {
	totalWords, totalChars, totalBytes := 0, 0, 0
	cursorWords, cursorChars, cursorBytes := 0, 0, 0
	for i := 0; i <= e.buf.LastLine(); i++ {
		line := e.buf.LineString(i)
		runes := []rune(line)
		totalWords += len(strings.Fields(line))
		totalChars += len(runes) + 1
		totalBytes += len(line) + 1
		if i < e.cur.Line {
			cursorWords += len(strings.Fields(line))
			cursorChars += len(runes) + 1
			cursorBytes += len(line) + 1
		} else if i == e.cur.Line {
			col := clamp(e.cur.Col, 0, len(runes))
			cursorWords += len(strings.Fields(string(runes[:col])))
			cursorChars += col + 1
			cursorBytes += len(string(runes[:col])) + 1
		}
	}
	e.messagef("Col %d of %d; Line %d of %d; Word %d of %d; Byte %d of %d",
		e.cur.Col+1, e.buf.LineLen(e.cur.Line), e.cur.Line+1, e.buf.Len(),
		cursorWords+1, totalWords, cursorBytes, totalBytes)
	_ = totalChars
	_ = cursorChars
}

// changeOlder and changeNewer walk the change list (g; and g,).
func (e *Editor) changeOlder() {
	if len(e.changeList) == 0 || e.changeIdx <= 0 {
		e.errorf("E662: At start of changelist")
		return
	}
	e.changeIdx--
	e.setCursor(e.changeList[e.changeIdx])
	e.messagef("change %d of %d", e.changeIdx+1, len(e.changeList))
}

func (e *Editor) changeNewer() {
	if len(e.changeList) == 0 || e.changeIdx >= len(e.changeList)-1 {
		e.errorf("E663: At end of changelist")
		return
	}
	e.changeIdx++
	e.setCursor(e.changeList[e.changeIdx])
	e.messagef("change %d of %d", e.changeIdx+1, len(e.changeList))
}

// noteChange records a position in the change list.
func (e *Editor) noteChange(p buffer.Pos) {
	if n := len(e.changeList); n > 0 && e.changeList[n-1].Line == p.Line && e.changeList[n-1].Col == p.Col {
		return
	}
	if e.changeIdx < len(e.changeList)-1 {
		e.changeList = e.changeList[:e.changeIdx+1]
	}
	e.changeList = append(e.changeList, p)
	if len(e.changeList) > 100 {
		e.changeList = e.changeList[len(e.changeList)-100:]
	}
	e.changeIdx = len(e.changeList) - 1
}

// gotoFileUnderCursor implements gf.
func (e *Editor) gotoFileUnderCursor() {
	path := e.fileUnderCursor()
	if path == "" {
		e.errorf("E447: Can't find file in path")
		return
	}
	if e.buf.Modified() {
		e.errorf("E37: No write since last change (add ! to override)")
		return
	}
	if !fileExists(path) {
		e.errorf("E484: Can't open file %s", path)
		return
	}
	e.pushJump(e.cur)
	e.buf.ClearUndo()
	if err := e.openFileIn(e.buf, path, true); err != nil {
		e.errorf("%v", err)
		return
	}
	e.setFileType(guessFileType(path))
	e.topline, e.topSeg, e.leftcol = 0, 0, 0
	e.setCursor(buffer.Pos{})
	e.noteChange(e.cur)
	e.messagef("%q %dL, %dB", path, e.buf.Len(), len(e.buf.Bytes()))
}

// fileUnderCursor extracts the file name under the cursor.
func (e *Editor) fileUnderCursor() string {
	line := e.buf.Line(e.cur.Line)
	if len(line) == 0 {
		return ""
	}
	col := clamp(e.cur.Col, 0, len(line)-1)
	isPath := func(r rune) bool {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			return true
		}
		return strings.ContainsRune(`_-.~/+\:`, r)
	}
	start, end := col, col
	for start > 0 && isPath(line[start-1]) {
		start--
	}
	for end < len(line) && isPath(line[end]) {
		end++
	}
	path := string(line[start:end])
	path = strings.Trim(path, "\"'`()[]{}<>,;:")
	if path == "" {
		return ""
	}
	if !strings.HasPrefix(path, "/") && !strings.HasPrefix(path, "~") {
		dir := "."
		if e.buf.Path() != "" {
			dir = filepath.Dir(e.buf.Path())
		}
		path = filepath.Join(dir, path)
	}
	return expandPath(path)
}

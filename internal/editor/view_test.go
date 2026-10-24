package editor

import (
	"strings"
	"testing"

	"vigo/internal/keys"
)

func screenText(t *testing.T, ed *Editor) []string {
	t.Helper()
	ed.draw()
	return ed.scr.Dump()
}

func findRow(rows []string, needle string) int {
	for i, r := range rows {
		if strings.Contains(r, needle) {
			return i
		}
	}
	return -1
}

func TestDrawGutterAndStatusline(t *testing.T) {
	ed := newTestEditor(t, "package main\n\nfunc main() {}\n")
	ed.opts.Number = true
	ed.setFileType("go")
	rows := screenText(t, ed)
	if !strings.HasPrefix(rows[0], "   1 package main") {
		t.Fatalf("row 0 = %q", rows[0])
	}
	if !strings.HasPrefix(rows[1], "   2") {
		t.Fatalf("row 1 = %q", rows[1])
	}
	if strings.TrimSpace(rows[3]) != "~" {
		t.Fatalf("expected ~ filler, got %q", rows[3])
	}
	last := rows[len(rows)-2]
	if !strings.Contains(last, "[No Name]") || !strings.Contains(last, "[go]") {
		t.Fatalf("statusline = %q", last)
	}
	if !strings.Contains(last, "1,1") {
		t.Fatalf("statusline position = %q", last)
	}
}

func TestDrawCursorPositionAndInsert(t *testing.T) {
	ed := newTestEditor(t, "hello")

	ed.draw()
	if ed.cursorX != 0 || ed.cursorY != 0 {
		t.Fatalf("cursor at %d,%d", ed.cursorX, ed.cursorY)
	}
	ed.opts.Number = true
	ed.draw()
	if ed.cursorX != 5 || ed.cursorY != 0 {
		t.Fatalf("cursor with numbers at %d,%d", ed.cursorX, ed.cursorY)
	}
	ed.feedForTest(t, "$")
	ed.draw()
	if ed.cursorX != 9 {
		t.Fatalf("cursor after $ = %d", ed.cursorX)
	}
	ed.feedForTest(t, "a")
	ed.draw()
	if ed.cursorX != 10 {
		t.Fatalf("cursor in insert mode = %d", ed.cursorX)
	}
}

func TestDrawWrapping(t *testing.T) {
	long := strings.Repeat("abcdefghij", 9)
	ed := newTestEditor(t, long)
	ed.opts.Wrap = true
	rows := screenText(t, ed)
	if !strings.HasPrefix(rows[0], "abcdefghij") {
		t.Fatalf("row 0 = %q", rows[0])
	}
	// 80 column terminal: the line needs two rows.
	if !strings.HasPrefix(rows[1], "abcdefghij") {
		t.Fatalf("row 1 = %q", rows[1])
	}
	ed.feedForTest(t, "G$")
	ed.draw()
	if ed.cursorY != 1 {
		t.Fatalf("cursor row = %d (want 1)", ed.cursorY)
	}
	if ed.cursorX != 9 {
		t.Fatalf("cursor col = %d (want 9)", ed.cursorX)
	}
	ed.opts.Wrap = false
	rows = screenText(t, ed)
	if !strings.Contains(rows[1], "~") {
		t.Fatalf("nowrap should show a single row: %q", rows[1])
	}
	ed.draw()
	if ed.leftcol == 0 {
		t.Fatalf("leftcol should have scrolled, got %d", ed.leftcol)
	}
	if !strings.Contains(rows[0], "@") {
		t.Fatalf("expected @ marker for cut off text: %q", rows[0])
	}
}

func TestDrawScrolling(t *testing.T) {
	var sb strings.Builder
	for i := 1; i <= 100; i++ {
		sb.WriteString("line ")
		sb.WriteString(itoa(i))
		sb.WriteString("\n")
	}
	ed := newTestEditor(t, sb.String())
	rows := screenText(t, ed)
	if !strings.HasPrefix(rows[0], "line 1") {
		t.Fatalf("row 0 = %q", rows[0])
	}
	ed.feedForTest(t, "G")
	ed.draw()
	rows = ed.scr.Dump()
	if findRow(rows, "line 100") < 0 {
		t.Fatalf("line 100 not visible after G")
	}
	ed.feedForTest(t, "gg")
	ed.draw()
	rows = ed.scr.Dump()
	if !strings.HasPrefix(rows[0], "line 1") {
		t.Fatalf("after gg row 0 = %q", rows[0])
	}
	ed.feedForTest(t, "<C-d>")
	ed.draw()
	rows = ed.scr.Dump()
	if strings.HasPrefix(rows[0], "line 1") {
		t.Fatalf("ctrl-d did not scroll: %q", rows[0])
	}
	ed.feedForTest(t, "zz")
	ed.draw()
	rows = ed.scr.Dump()
	mid := len(rows)/2 - 1
	if rows[mid] == "" {
		t.Fatalf("empty middle row")
	}
}

func TestDrawVisualHighlight(t *testing.T) {
	ed := newTestEditor(t, "hello world")
	ed.feedForTest(t, "vl")
	ed.draw()
	a := ed.scr.AttrAt(0, 0)
	b := ed.scr.AttrAt(1, 0)
	if a != ed.theme.visual || b != ed.theme.visual {
		t.Fatalf("selection attrs: %#v %#v", a, b)
	}
	// The row of the statusline must not be selected.
	if ed.scr.AttrAt(0, 23) != ed.theme.cmdline {
		t.Fatalf("cmdline attrs: %#v", ed.scr.AttrAt(0, 23))
	}
}

func TestDrawSearchHighlight(t *testing.T) {
	ed := newTestEditor(t, "alpha beta\ngamma beta")
	ed.feedForTest(t, "/beta<CR>")
	ed.draw()
	sel := ed.theme.search
	found := false
	for x := 0; x < 10; x++ {
		if ed.scr.AttrAt(x, 0) == sel {
			found = true
		}
	}
	if !found {
		t.Fatalf("no search highlight on row 0: %q", ed.scr.Dump()[0])
	}
	ed.feedForTest(t, ":noh<CR>")
	ed.draw()
	for x := 0; x < 10; x++ {
		if ed.scr.AttrAt(x, 0) == sel {
			t.Fatalf("highlight should be cleared")
		}
	}
}

func TestDrawSyntaxHighlight(t *testing.T) {
	ed := newTestEditor(t, "func main() {\n\t// comment\n}\n")
	ed.setFileType("go")
	ed.opts.Syntax = true
	ed.draw()
	if ed.scr.AttrAt(0, 0) != ed.theme.syntax[groupKeyword] {
		t.Fatalf("func should be highlighted: %#v", ed.scr.AttrAt(0, 0))
	}
	if ed.scr.AttrAt(8, 1) != ed.theme.syntax[groupComment] {
		t.Fatalf("comment should be highlighted: %#v row %q", ed.scr.AttrAt(8, 1), ed.scr.Dump()[1])
	}
}

func TestDrawCursorLineAndNumbers(t *testing.T) {
	ed := newTestEditor(t, "one\ntwo\nthree")
	ed.opts.CursorLine = true
	ed.opts.RelativeNumber = true
	ed.opts.Number = true
	rows := screenText(t, ed)
	if !strings.HasPrefix(rows[0], "   1 one") {
		t.Fatalf("row 0 = %q", rows[0])
	}
	if !strings.HasPrefix(rows[1], "   1 two") {
		t.Fatalf("relative number row 1 = %q", rows[1])
	}
	ed.feedForTest(t, "j")
	ed.draw()
	rows = ed.scr.Dump()
	if !strings.HasPrefix(rows[1], "   2 two") || !strings.HasPrefix(rows[0], "   1 one") {
		t.Fatalf("after j: %q / %q", rows[0], rows[1])
	}
}

func TestDrawTabsAndList(t *testing.T) {
	ed := newTestEditor(t, "\tx")
	ed.opts.List = true
	rows := screenText(t, ed)
	if rows[0][0] != '^' || rows[0][1] != 'I' {
		t.Fatalf("list tab = %q", rows[0])
	}
	if !strings.Contains(rows[0], "$") {
		t.Fatalf("list eol = %q", rows[0])
	}
}

func TestDrawCommandline(t *testing.T) {
	ed := newTestEditor(t, "one")
	ed.feedForTest(t, ":set nu")
	rows := screenText(t, ed)
	if !strings.Contains(rows[len(rows)-1], ":set nu") {
		t.Fatalf("cmdline = %q", rows[len(rows)-1])
	}
	ed.draw()
	if ed.cursorY != 23 || ed.cursorX != 7 {
		t.Fatalf("cmdline cursor at %d,%d", ed.cursorX, ed.cursorY)
	}
	ed.feedForTest(t, "<Esc>")
	ed.feedForTest(t, "i")
	rows = screenText(t, ed)
	if !strings.Contains(rows[len(rows)-1], "-- INSERT --") {
		t.Fatalf("showmode = %q", rows[len(rows)-1])
	}
}

func TestScreenResizeKeepsWorking(t *testing.T) {
	ed := newTestEditor(t, "one\ntwo\nthree\nfour")
	ed.scr.Resize(30, 8)
	rows := screenText(t, ed)
	if len(rows) != 8 {
		t.Fatalf("rows = %d", len(rows))
	}
	if !strings.Contains(rows[6], "1,1") {
		t.Fatalf("statusline after resize = %q", rows[6])
	}
	if strings.TrimSpace(rows[5]) != "~" {
		t.Fatalf("row 5 = %q", rows[5])
	}
	ed.feedForTest(t, "<C-d>")
	ed.draw()
}

func TestKeyNotationRoundTrip(t *testing.T) {
	ks := keys.MustParse("ihello<Esc>dd:wq<CR>")
	want := []string{"i", "h", "e", "l", "l", "o", "<Esc>", "d", "d", ":", "w", "q", "<CR>"}
	if len(ks) != len(want) {
		t.Fatalf("got %d keys", len(ks))
	}
	for i, k := range ks {
		if k.String() != want[i] {
			t.Errorf("key %d = %q want %q", i, k.String(), want[i])
		}
	}
}

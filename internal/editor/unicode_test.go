package editor

import (
	"testing"

	"vigo/internal/buffer"
	"vigo/internal/keys"
	"vigo/internal/screen"
)

// Positions are rune based, never byte based.
func TestUnicodePositions(t *testing.T) {
	ed := newTestEditor(t, "héllo 中文字")
	ed.feedForTest(t, "x")
	if got := ed.text(); got != "éllo 中文字" {
		t.Fatalf("x removed the wrong rune: %q", got)
	}
	ed.feedForTest(t, "$")
	if ed.cur != (buffer.Pos{Line: 0, Col: 7}) {
		t.Fatalf("$ should be at rune index 7, got %v", ed.cur)
	}
	ed.feedForTest(t, "x")
	if got := ed.text(); got != "éllo 中文" {
		t.Fatalf("x on the last CJK rune: %q", got)
	}
}

// Wide characters occupy two terminal cells and never split across rows.
func TestUnicodeWideCells(t *testing.T) {
	ed := newTestEditor(t, "中x")
	ed.draw()
	if ed.scr.CellAt(0, 0).W != 2 {
		t.Fatalf("中 should be a wide cell: %#v", ed.scr.CellAt(0, 0))
	}
	if ed.scr.CellAt(1, 0).W != 0 {
		t.Fatalf("expected a continuation cell: %#v", ed.scr.CellAt(1, 0))
	}
	if r := ed.scr.CellAt(2, 0).R; r != 'x' {
		t.Fatalf("x should start at cell 2, got %q", r)
	}
	ed.feedForTest(t, "l")
	ed.draw()
	if ed.cursorX != 2 {
		t.Fatalf("cursor should be at cell 2, got %d", ed.cursorX)
	}
}

func TestUnicodeWidths(t *testing.T) {
	cases := []struct {
		r    rune
		want int
	}{
		{'a', 1}, {'é', 1}, {'中', 2}, {'あ', 2}, {'🎉', 2},
		{'\u0301', 0}, {'\t', 2}, {'\x00', 0},
	}
	for _, c := range cases {
		if got := screen.RuneWidth(c.r); got != c.want {
			t.Errorf("width(%q) = %d want %d", c.r, got, c.want)
		}
	}
}

// Wrapping must not cut a wide character in half.
func TestUnicodeWrap(t *testing.T) {
	ed := newTestEditor(t, "中中中")
	ed.scr.Resize(6, 6)
	ed.opts.Wrap = true
	ed.draw()
	if ed.scr.CellAt(4, 0).W != 2 {
		t.Fatalf("expected a full wide cell at 4: %#v", ed.scr.CellAt(4, 0))
	}
	rows := ed.scr.Dump()
	if got := []rune(rows[0]); len(got) > 5 && got[5] != ' ' && got[5] != '中' {
		t.Fatalf("row 0 = %q", rows[0])
	}
}

func TestUnicodeWordMotion(t *testing.T) {
	ed := newTestEditor(t, "naïve café 中文 foo")
	ed.feedForTest(t, "w")
	if ed.cur.Col != 6 {
		t.Fatalf("w = %v", ed.cur)
	}
	ed.feedForTest(t, "dw")
	if got := ed.text(); got != "naïve 中文 foo" {
		t.Fatalf("dw after unicode word: %q", got)
	}
	ed.feedForTest(t, "diw")
	if got := ed.text(); got != "naïve  foo" {
		t.Fatalf("diw on CJK word: %q", got)
	}
}

func TestUnicodeSearchAndSubstitute(t *testing.T) {
	ed := newTestEditor(t, "café naïveté")
	ed.feedForTest(t, "/café<CR>")
	if ed.cur != (buffer.Pos{Line: 0, Col: 0}) {
		t.Fatalf("search: %v", ed.cur)
	}
	ed.feedForTest(t, ":%s/é/E/g<CR>")
	if got := ed.text(); got != "cafE naïvetE" {
		t.Fatalf("substitute: %q", got)
	}
	ed = newTestEditor(t, "αβγ δεζ")
	ed.feedForTest(t, "/δ<CR>")
	if ed.cur != (buffer.Pos{Line: 0, Col: 4}) {
		t.Fatalf("greek search: %v", ed.cur)
	}
	ed.feedForTest(t, "dw")
	if got := ed.text(); got != "αβγ " {
		t.Fatalf("dw on greek: %q", got)
	}
}

func TestUnicodeCaseOperators(t *testing.T) {
	ed := newTestEditor(t, "æøå")
	ed.feedForTest(t, "gUiw")
	if got := ed.text(); got != "ÆØÅ" {
		t.Fatalf("gUiw: %q", got)
	}
	ed.feedForTest(t, "guiw")
	if got := ed.text(); got != "æøå" {
		t.Fatalf("guiw: %q", got)
	}
}

func TestUnicodeInsert(t *testing.T) {
	ed := newTestEditor(t, "")
	ed.feedForTest(t, "i日本語<Esc>")
	if got := ed.text(); got != "日本語" {
		t.Fatalf("insert: %q", got)
	}
	if ed.cur != (buffer.Pos{Line: 0, Col: 2}) {
		t.Fatalf("cursor after escape: %v", ed.cur)
	}
	ed.feedForTest(t, "aé<Esc>")
	if got := ed.text(); got != "日本語é" {
		t.Fatalf("append: %q", got)
	}
	if ed.cur.Col != 3 {
		t.Fatalf("cursor: %v", ed.cur)
	}
}

func TestUnicodeKeyNotation(t *testing.T) {
	ks := keys.MustParse("é中<Esc>")
	if len(ks) != 3 || ks[0].R != 'é' || ks[1].R != '中' {
		t.Fatalf("keys: %v", ks)
	}
	if ks[2].Name != keys.Esc {
		t.Fatalf("escape: %v", ks[2])
	}
}

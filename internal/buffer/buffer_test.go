package buffer

import "testing"

func TestFromStringKeepsMissingEOL(t *testing.T) {
	b := FromString("a\nb")
	if b.Len() != 2 || b.LineString(0) != "a" || b.LineString(1) != "b" {
		t.Fatalf("unexpected lines: %d %q", b.Len(), b.String())
	}
	if !b.MissingEOL() {
		t.Fatal("expected missing eol")
	}
	if got := b.String(); got != "a\nb" {
		t.Fatalf("round trip: %q", got)
	}

	c := FromString("a\nb\n")
	if c.MissingEOL() || c.Len() != 2 {
		t.Fatalf("trailing newline mishandled: %d %v", c.Len(), c.MissingEOL())
	}
	if got := c.String(); got != "a\nb\n" {
		t.Fatalf("round trip: %q", got)
	}
}

func TestFromBytesCRLF(t *testing.T) {
	b := FromBytes([]byte("a\r\nb\r\n"))
	if b.EOL() != "\r\n" {
		t.Fatalf("eol = %q", b.EOL())
	}
	if got := b.String(); got != "a\r\nb\r\n" {
		t.Fatalf("round trip: %q", got)
	}
}

func TestInsertDeleteRange(t *testing.T) {
	b := FromString("hello world")
	b.Insert(Pos{0, 5}, []rune(","))
	if got := b.LineString(0); got != "hello, world" {
		t.Fatalf("insert: %q", got)
	}
	removed := b.DeleteRange(Pos{0, 0}, Pos{0, 6})
	if string(removed[0]) != "hello," || b.LineString(0) != " world" {
		t.Fatalf("delete: %q %q", string(removed[0]), b.LineString(0))
	}
}

func TestDeleteRangeAcrossLines(t *testing.T) {
	b := FromString("one\ntwo\nthree")
	removed := b.DeleteRange(Pos{0, 1}, Pos{2, 2})
	if b.Len() != 1 || b.LineString(0) != "oree" {
		t.Fatalf("merged = %q (lines %d)", b.LineString(0), b.Len())
	}
	want := []string{"ne", "two", "th"}
	if len(removed) != 3 {
		t.Fatalf("removed = %d", len(removed))
	}
	for i, w := range want {
		if string(removed[i]) != w {
			t.Fatalf("removed[%d] = %q want %q", i, string(removed[i]), w)
		}
	}
}

func TestSplitAndJoin(t *testing.T) {
	b := FromString("hello world")
	p := b.SplitLine(Pos{0, 5})
	if p != (Pos{1, 0}) || b.LineString(0) != "hello" || b.LineString(1) != " world" {
		t.Fatalf("split: %q %q", b.LineString(0), b.LineString(1))
	}
	b.JoinLines(0, true)
	if b.LineString(0) != "hello world" || b.Len() != 1 {
		t.Fatalf("join: %q %d", b.LineString(0), b.Len())
	}
}

func TestUndoBlocks(t *testing.T) {
	b := FromString("abc")
	b.Checkpoint(Pos{0, 0})
	b.Insert(Pos{0, 3}, []rune("d"))
	b.Insert(Pos{0, 4}, []rune("e"))
	b.BreakBlock()
	if got := b.LineString(0); got != "abcde" {
		t.Fatalf("after insert: %q", got)
	}
	cur, ok := b.Undo(Pos{0, 5})
	if !ok || b.LineString(0) != "abc" || cur != (Pos{0, 0}) {
		t.Fatalf("undo: %q %v %v", b.LineString(0), cur, ok)
	}
	if b.Modified() {
		t.Fatal("undo back to saved state should clear modified")
	}
	cur, ok = b.Redo(cur)
	if !ok || b.LineString(0) != "abcde" || cur != (Pos{0, 5}) {
		t.Fatalf("redo: %q %v %v", b.LineString(0), cur, ok)
	}
}

func TestDeleteLinesKeepsOneLine(t *testing.T) {
	b := FromString("only")
	removed := b.DeleteLines(0, 0)
	if len(removed) != 1 || b.Len() != 1 || b.LineString(0) != "" {
		t.Fatalf("delete only line: %d %q", b.Len(), b.LineString(0))
	}
}

// The undo history must not grow without bound on a big buffer.
func TestUndoStaysWithinMemoryBudget(t *testing.T) {
	b := New()
	b.UndoLimit = 100000 // no count based limit
	line := make([]rune, 2000)
	for i := range line {
		line[i] = 'x'
	}
	b.lines = [][]rune{line}
	for i := 0; i < 400; i++ {
		b.Checkpoint(Pos{})
		b.BreakBlock()
		b.Insert(Pos{Line: 0, Col: 0}, []rune{'a'})
	}
	total := 0
	for _, s := range b.undo {
		total += s.bytes
	}
	if total > maxUndoBytes {
		t.Fatalf("undo history = %d bytes, budget is %d", total, maxUndoBytes)
	}
	if len(b.undo) == 0 {
		t.Fatal("undo history should not be empty")
	}
}

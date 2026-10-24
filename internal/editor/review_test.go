package editor

import (
	"os"
	"testing"
	"time"

	"vigo/internal/keys"
)

// expected values below were taken from vim 9.1 (`vim -es -u NONE`,
// `call setline(1, ["a","b","c","d"])` then the command and `%print`).
func TestExMoveAndCopyMatchVim(t *testing.T) {
	cases := []struct {
		cmd  string
		want string
	}{
		{":3m0", "c\na\nb\nd\n"},
		{":2m0", "b\na\nc\nd\n"},
		{":2m1", "a\nb\nc\nd\n"},
		{":1m$", "b\nc\nd\na\n"},
		{":4m2", "a\nb\nd\nc\n"},
		{":1,2m4", "c\nd\na\nb\n"},
		{":3,4m1", "a\nc\nd\nb\n"},
		{":1t0", "a\na\nb\nc\nd\n"},
		{":1t3", "a\nb\nc\na\nd\n"},
		{":2t2", "a\nb\nb\nc\nd\n"},
	}
	for _, c := range cases {
		ed := newTestEditor(t, "a\nb\nc\nd\n")
		ed.execEx(c.cmd[1:])
		if got := ed.text(); got != c.want {
			t.Errorf("%s -> %q want %q", c.cmd, got, c.want)
		}
	}
}

func TestCompletionCyclesThroughCandidates(t *testing.T) {
	ed := newTestEditor(t, "alpha\nalternate\n")
	ed.feedForTest(t, "oal")
	ed.feedForTest(t, "<C-n>")
	if got := ed.text(); got != "alpha\nalpha\nalternate\n" {
		t.Fatalf("first ctrl-n = %q", got)
	}
	ed.feedForTest(t, "<C-n>")
	if got := ed.text(); got != "alpha\nalternate\nalternate\n" {
		t.Fatalf("second ctrl-n = %q", got)
	}
	ed.feedForTest(t, "<C-n>")
	if got := ed.text(); got != "alpha\nalpha\nalternate\n" {
		t.Fatalf("third ctrl-n should wrap = %q", got)
	}
	ed.feedForTest(t, "<C-p>")
	if got := ed.text(); got != "alpha\nalternate\nalternate\n" {
		t.Fatalf("ctrl-p should go back = %q", got)
	}
}

func TestSubstituteConfirmFlag(t *testing.T) {
	// 'l' replaces this match and stops.
	ed := newTestEditor(t, "aa aa aa\n")
	ed.keyCh <- keys.Rune('y')
	ed.keyCh <- keys.Rune('l')
	ed.execEx("%s/aa/x/gc")
	if got := ed.text(); got != "x x aa\n" {
		t.Fatalf("y,l = %q", got)
	}
	// 'a' replaces all without asking again.
	ed = newTestEditor(t, "aa aa aa\n")
	ed.keyCh <- keys.Rune('a')
	ed.execEx("%s/aa/x/gc")
	if got := ed.text(); got != "x x x\n" {
		t.Fatalf("a = %q", got)
	}
	// 'q' stops right away.
	ed = newTestEditor(t, "aa aa aa\n")
	ed.keyCh <- keys.Rune('q')
	ed.execEx("%s/aa/x/gc")
	if got := ed.text(); got != "aa aa aa\n" {
		t.Fatalf("q = %q", got)
	}
}

// A confirmation prompt must never block when there is nobody to answer it,
// for instance when the substitution runs from a script.
func TestSubstituteConfirmDoesNotBlockInScripts(t *testing.T) {
	ed := newTestEditor(t, "aa aa\n")
	ed.scriptDepth = 1
	done := make(chan string, 1)
	go func() {
		ed.execEx("%s/aa/x/c")
		done <- ed.text()
	}()
	select {
	case got := <-done:
		if got != "x aa\n" {
			t.Fatalf("script confirm = %q", got)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("confirmation blocked in a script context")
	}
}

// The special marks ” , “ and '. were checked against vim: after G, gg the
// command ” returns to the line that was left, and it toggles when repeated.
func TestJumpBackMarks(t *testing.T) {
	ed := newTestEditor(t, "a\nb\nc\n")
	ed.feedForTest(t, "G")
	ed.feedForTest(t, "gg")
	ed.feedForTest(t, "''")
	if ed.cur.Line != 2 {
		t.Fatalf("'' after G,gg = line %d, want 3", ed.cur.Line+1)
	}
	ed.feedForTest(t, "''")
	if ed.cur.Line != 0 {
		t.Fatalf("'' again should toggle back to line 1, got %d", ed.cur.Line+1)
	}
	ed.feedForTest(t, "``")
	if ed.cur.Line != 2 {
		t.Fatalf("`` should toggle again, got line %d", ed.cur.Line+1)
	}
	// '. jumps to the position of the last change.
	ed = newTestEditor(t, "one\ntwo\nthree\n")
	ed.feedForTest(t, "jA!<Esc>")
	ed.feedForTest(t, "gg")
	ed.feedForTest(t, "`.")
	if ed.cur.Line != 1 {
		t.Fatalf("`. = line %d, want 2", ed.cur.Line+1)
	}
	if ed.cur.Col != 3 {
		t.Fatalf("`. = col %d, want 3", ed.cur.Col)
	}
}

// Replace mode remembers what it overwrote so backspace can restore it; vim
// gives "XZc" for "abc" with RXY<BS>Z.
func TestReplaceModeBackspaceRestores(t *testing.T) {
	ed := newTestEditor(t, "abc\n")
	ed.feedForTest(t, "RXY<BS>Z<Esc>")
	if got := ed.text(); got != "XZc\n" {
		t.Fatalf("RXY<BS>Z = %q want %q", got, "XZc\n")
	}
}

func TestReadonlyOptionBlocksWriting(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/ro.txt"
	if err := writeTestFile(path, "one\n"); err != nil {
		t.Fatal(err)
	}
	ed := newTestEditor(t, "")
	if err := ed.openFileIn(ed.buf, path, true); err != nil {
		t.Fatal(err)
	}
	ed.execEx("set readonly")
	if !ed.buf.ReadOnly() {
		t.Fatal(":set readonly did not mark the buffer")
	}
	ed.feedForTest(t, "A!<Esc>")
	if err := ed.writeBuffer(ed.buf, path, false); err == nil {
		t.Fatal("writing a readonly buffer should fail without !")
	}
	if err := ed.writeBuffer(ed.buf, path, true); err != nil {
		t.Fatalf("writing with ! should work: %v", err)
	}
	ed.execEx("set noreadonly")
	if ed.buf.ReadOnly() {
		t.Fatal(":set noreadonly did not clear the flag")
	}
	data, _ := os.ReadFile(path)
	if string(data) != "one!\n" {
		t.Fatalf("file = %q", data)
	}
}

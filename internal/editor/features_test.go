package editor

import (
	"runtime"
	"strings"
	"testing"

	"vigo/internal/buffer"
)

func TestIndentOperatorGo(t *testing.T) {
	src := "package main\nfunc f() {\nx := 1\nif x > 0 {\ny := 2\n}\n}\n"
	want := "package main\nfunc f() {\n\tx := 1\n\tif x > 0 {\n\t\ty := 2\n\t}\n}\n"
	ed := newTestEditor(t, src)
	ed.setFileType("go")
	ed.feedForTest(t, "gg=G")
	if got := ed.text(); got != want {
		t.Fatalf("indent go: got %q want %q", got, want)
	}
}

func TestIndentOperatorKeepsBracketsInStrings(t *testing.T) {
	src := "func f() {\ns := \"}{ \"\nif s == \"{\" {\nx := 1\n}\n}\n"
	want := "func f() {\n\ts := \"}{ \"\n\tif s == \"{\" {\n\t\tx := 1\n\t}\n}\n"
	ed := newTestEditor(t, src)
	ed.setFileType("go")
	ed.feedForTest(t, "gg=G")
	if got := ed.text(); got != want {
		t.Fatalf("indent strings: got %q want %q", got, want)
	}
}

func TestIndentOperatorPython(t *testing.T) {
	src := "def f():\nx = 1\nif x:\ny = 2\nreturn y\n"
	want := "def f():\n\tx = 1\n\tif x:\n\t\ty = 2\n\treturn y\n"
	ed := newTestEditor(t, src)
	ed.setFileType("python")
	ed.feedForTest(t, "gg=G")
	if got := ed.text(); got != want {
		t.Fatalf("indent python: got %q want %q", got, want)
	}
}

func TestIndentOperatorRange(t *testing.T) {
	src := "func f() {\nx := 1\ny := 2\n}\n"
	ed := newTestEditor(t, src)
	ed.setFileType("go")
	ed.feedForTest(t, "j==")
	if got := ed.text(); got != "func f() {\n\tx := 1\ny := 2\n}\n" {
		t.Fatalf("range indent: %q", got)
	}
}

func TestIndentUnknownFileTypeDoesNothing(t *testing.T) {
	src := "a\nb\n"
	ed := newTestEditor(t, src)
	ed.setFileType("")
	ed.feedForTest(t, "gg=G")
	if got := ed.text(); got != src {
		t.Fatalf("plain text changed: %q", got)
	}
}

func TestFilterRange(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("needs a unix shell")
	}
	ed := newTestEditor(t, "b\na\nc\n")
	ed.feedForTest(t, ":%!sort<CR>")
	if got := ed.text(); got != "a\nb\nc\n" {
		t.Fatalf("filter sort = %q", got)
	}
	ed.feedForTest(t, "u")
	if got := ed.text(); got != "b\na\nc\n" {
		t.Fatalf("undo filter = %q", got)
	}
}

func TestFilterWithSuffixCommand(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("needs a unix shell")
	}
	ed := newTestEditor(t, "one\ntwo\nthree\n")
	ed.feedForTest(t, ":2,3!tr a-z A-Z<CR>")
	if got := ed.text(); got != "one\nTWO\nTHREE\n" {
		t.Fatalf("filter tr = %q", got)
	}
}

func TestRetab(t *testing.T) {
	ed := newTestEditor(t, "\tx\n")
	ed.opts.ExpandTab = true
	ed.feedForTest(t, ":%retab<CR>")
	if got := ed.text(); got != "        x\n" {
		t.Fatalf("retab expandtab = %q", got)
	}
	ed = newTestEditor(t, "        x\n")
	ed.opts.ExpandTab = false
	ed.feedForTest(t, ":%retab<CR>")
	if got := ed.text(); got != "\tx\n" {
		t.Fatalf("retab noexpandtab = %q", got)
	}
}

func TestPasteOption(t *testing.T) {
	src := "    x\n"
	ed := newTestEditor(t, src)
	ed.opts.AutoIndent = true
	ed.feedForTest(t, "A<CR>y<Esc>")
	if got := ed.text(); got != "    x\n    y\n" {
		t.Fatalf("autoindent: %q", got)
	}
	ed = newTestEditor(t, src)
	ed.opts.AutoIndent = true
	ed.opts.Paste = true
	ed.feedForTest(t, "A<CR>y<Esc>")
	if got := ed.text(); got != "    x\ny\n" {
		t.Fatalf("paste: %q", got)
	}
	ed = newTestEditor(t, "x")
	ed.opts.ExpandTab = true
	ed.opts.Paste = true
	ed.feedForTest(t, "i\t<Esc>")
	if got := ed.text(); got != "\tx" {
		t.Fatalf("paste tab: %q", got)
	}
}

func TestBlockToEndOfLine(t *testing.T) {
	ed := newTestEditor(t, "ab\nabcd\nzz\n")
	ed.feedForTest(t, "<C-v>j$d")
	if got := ed.text(); got != "\n\nzz\n" {
		t.Fatalf("block $ delete: %q", got)
	}
	ed = newTestEditor(t, "ab\nabcd\n")
	ed.feedForTest(t, "<C-v>j$A!<Esc>")
	if got := ed.text(); got != "ab!\nabcd!\n" {
		t.Fatalf("block $ append: %q", got)
	}
}

func TestChangeList(t *testing.T) {
	ed := newTestEditor(t, "one\ntwo\nthree\n")
	ed.feedForTest(t, "jA!<Esc>")
	if ed.cur.Line != 1 {
		t.Fatalf("cursor = %v", ed.cur)
	}
	ed.feedForTest(t, "jA?<Esc>")
	ed.feedForTest(t, "g;")
	if ed.cur.Line != 1 {
		t.Fatalf("g; = %v", ed.cur)
	}
	ed.feedForTest(t, "g,")
	if ed.cur.Line != 2 {
		t.Fatalf("g, = %v", ed.cur)
	}
}

func TestGotoFileUnderCursor(t *testing.T) {
	dir := t.TempDir()
	if err := writeTestFile(dir+"/other.go", "package other\n"); err != nil {
		t.Fatal(err)
	}
	if err := writeTestFile(dir+"/main.go", "other.go\n"); err != nil {
		t.Fatal(err)
	}
	ed := newTestEditor(t, "other.go\n")
	ed.buf.SetPath(dir + "/main.go")
	ed.feedForTest(t, "gf")
	if !strings.HasSuffix(ed.buf.Path(), "other.go") {
		t.Fatalf("path = %q", ed.buf.Path())
	}
	if ed.fileType() != "go" {
		t.Fatalf("file type = %q", ed.fileType())
	}
	if ed.cur != (buffer.Pos{}) {
		t.Fatalf("cursor = %v", ed.cur)
	}
	ed = newTestEditor(t, "nosuchfile.go\n")
	ed.feedForTest(t, "gf")
	if !strings.Contains(ed.msg, "E484") {
		t.Fatalf("message = %q", ed.msg)
	}
}

func TestWordCount(t *testing.T) {
	ed := newTestEditor(t, "one two\nthree four five\n")
	ed.feedForTest(t, "g<C-g>")
	if !strings.Contains(ed.msg, "Word 1 of 5") {
		t.Fatalf("word count = %q", ed.msg)
	}
	ed.feedForTest(t, "jj")
	ed.feedForTest(t, "g<C-g>")
	if !strings.Contains(ed.msg, "Line 2 of 2") {
		t.Fatalf("line count = %q", ed.msg)
	}
}

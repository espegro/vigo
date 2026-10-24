package editor

import (
	"bytes"
	"strings"
	"testing"

	"vigo/internal/buffer"
	"vigo/internal/keys"
	"vigo/internal/screen"
)

func newTestEditor(t *testing.T, content string) *Editor {
	t.Helper()
	var out bytes.Buffer
	ed := &Editor{
		scr:     screen.New(80, 24, &out),
		buf:     buffer.FromString(content),
		regs:    map[rune]Register{},
		macros:  map[rune][]keys.Key{},
		marks:   map[rune]buffer.Pos{},
		opts:    DefaultOptions(),
		keyCh:   make(chan keys.Key, 256),
		theme:   makeTheme("dark"),
		cursors: map[*buffer.Buffer]buffer.Pos{},
	}
	ed.buffers = []*buffer.Buffer{ed.buf}
	ed.setFileType("")
	ed.initWindows(ed.buf)
	return ed
}

func (e *Editor) feedForTest(t *testing.T, s string) {
	t.Helper()
	ks, err := keys.Parse(s)
	if err != nil {
		t.Fatalf("parse %q: %v", s, err)
	}
	for _, k := range ks {
		e.handleKey(k)
	}
}

func (e *Editor) text() string { return e.buf.String() }

func TestInsertAndAppend(t *testing.T) {
	cases := []struct {
		name, start, keys, want string
		cursor                  buffer.Pos
	}{
		{"insert", "hello", "iX<Esc>", "Xhello", buffer.Pos{Line: 0, Col: 0}},
		{"append", "hello", "aX<Esc>", "hXello", buffer.Pos{Line: 0, Col: 1}},
		{"append end", "hello", "AX<Esc>", "helloX", buffer.Pos{Line: 0, Col: 5}},
		{"insert start", "hello", "IX<Esc>", "Xhello", buffer.Pos{Line: 0, Col: 0}},
		{"open below", "one\ntwo", "oX<Esc>", "one\nX\ntwo", buffer.Pos{Line: 1, Col: 0}},
		{"open above", "one\ntwo", "OX<Esc>", "X\none\ntwo", buffer.Pos{Line: 0, Col: 0}},
		{"substitute char", "hello", "sX<Esc>", "Xello", buffer.Pos{Line: 0, Col: 0}},
		{"change line", "hello world", "Snew<Esc>", "new", buffer.Pos{Line: 0, Col: 2}},
		{"change to eol", "hello world", "wCbye<Esc>", "hello bye", buffer.Pos{Line: 0, Col: 8}},
		{"insert middle", "hello world", "wiX<Esc>", "hello Xworld", buffer.Pos{Line: 0, Col: 6}},
	}
	for _, c := range cases {
		ed := newTestEditor(t, c.start)
		ed.feedForTest(t, c.keys)
		if got := ed.text(); got != c.want {
			t.Errorf("%s: text = %q want %q", c.name, got, c.want)
		}
		if ed.cur != c.cursor {
			t.Errorf("%s: cursor = %v want %v", c.name, ed.cur, c.cursor)
		}
	}
}

func TestMotions(t *testing.T) {
	src := "hello world\nfoo bar baz\nlast line here"
	cases := []struct {
		keys string
		want buffer.Pos
	}{
		{"w", buffer.Pos{Line: 0, Col: 6}},
		{"ww", buffer.Pos{Line: 1, Col: 0}},
		{"2w", buffer.Pos{Line: 1, Col: 0}},
		{"e", buffer.Pos{Line: 0, Col: 4}},
		{"b", buffer.Pos{Line: 0, Col: 0}},
		{"l", buffer.Pos{Line: 0, Col: 1}},
		{"$", buffer.Pos{Line: 0, Col: 10}},
		{"j", buffer.Pos{Line: 1, Col: 0}},
		{"jj$", buffer.Pos{Line: 2, Col: 13}},
		{"G", buffer.Pos{Line: 2, Col: 0}},
		{"3Gw", buffer.Pos{Line: 2, Col: 5}},
		{"gg", buffer.Pos{Line: 0, Col: 0}},
		{"2G$", buffer.Pos{Line: 1, Col: 10}},
		{"fo", buffer.Pos{Line: 0, Col: 4}},
		{"to", buffer.Pos{Line: 0, Col: 3}},
		{"2l", buffer.Pos{Line: 0, Col: 2}},
		{"j0l", buffer.Pos{Line: 1, Col: 1}},
	}
	for _, c := range cases {
		ed := newTestEditor(t, src)
		// Start from line 0, col 1 for a few cases that move left.
		ed.feedForTest(t, c.keys)
		if ed.cur != c.want {
			t.Errorf("%q: cursor = %v want %v", c.keys, ed.cur, c.want)
		}
	}
}

func TestOperators(t *testing.T) {
	cases := []struct {
		name, start, keys, want string
	}{
		{"dw", "hello world", "dw", "world"},
		{"dw last word", "hello world", "wdw", "hello "},
		{"3dw", "one two three four", "3dw", "four"},
		{"d$", "hello world", "lld$", "he"},
		{"dd", "one\ntwo\nthree", "dd", "two\nthree"},
		{"2dd", "one\ntwo\nthree", "2dd", "three"},
		{"dj", "one\ntwo\nthree", "dj", "three"},
		{"dk", "one\ntwo\nthree", "jdk", "three"},
		{"dG", "one\ntwo\nthree", "dG", ""},
		{"dgg", "one\ntwo\nthree", "Gdgg", ""},
		{"x", "hello", "x", "ello"},
		{"3x", "hello", "3x", "lo"},
		{"X", "hello", "lX", "ello"},
		{"D", "hello world", "wD", "hello "},
		{"diw", "hello world", "diw", " world"},
		{"daw", "hello world", "daw", "world"},
		{"ciw", "hello world", "ciwbye<Esc>", "bye world"},
		{"di\"", "say \"hello there\" ok", "f\"di\"", "say \"\" ok"},
		{"da\"", "say \"hello there\" ok", "f\"da\"", "say  ok"},
		{"di(", "f(a, b)", "f(di(", "f()"},
		{"da(", "f(a, b)", "f(da(", "f"},
		{"d2w", "one two three four", "wd2w", "one four"},
		{"dFo", "hello world", "$dFo", "hello wd"},
		{"dfw", "hello world", "dfw", "orld"},
		{"dtw", "hello world", "dtw", "world"},
		{"D on empty", "a\n\nb", "jD", "a\n\nb"},
		{"dip", "a\nb\nc\n\nd", "dip", "\nd"},
	}
	for _, c := range cases {
		ed := newTestEditor(t, c.start)
		ed.feedForTest(t, c.keys)
		if got := ed.text(); got != c.want {
			t.Errorf("%s: %q -> %q want %q", c.name, c.start, got, c.want)
		}
	}
}

func TestYankAndPut(t *testing.T) {
	ed := newTestEditor(t, "one\ntwo\nthree")
	ed.feedForTest(t, "yy")
	if got := ed.register('"'); !got.Linewise || got.Lines[0] != "one" {
		t.Fatalf("yank linewise = %#v", got)
	}
	ed.feedForTest(t, "p")
	if got := ed.text(); got != "one\none\ntwo\nthree" {
		t.Fatalf("after p: %q", got)
	}
	ed = newTestEditor(t, "hello world")
	ed.feedForTest(t, "yiw$p")
	if got := ed.text(); got != "hello worldhello" {
		t.Fatalf("yiw p: %q", got)
	}
	ed = newTestEditor(t, "hello\nworld")
	ed.feedForTest(t, `"ayyj"aP`)
	if got := ed.text(); got != "hello\nhello\nworld" {
		t.Fatalf("named register: %q", got)
	}
	if r := ed.register('a'); !r.Linewise || r.Lines[0] != "hello" {
		t.Fatalf("register a = %#v", r)
	}
}

func TestUndoRedo(t *testing.T) {
	ed := newTestEditor(t, "hello world")
	ed.feedForTest(t, "dw")
	if got := ed.text(); got != "world" {
		t.Fatalf("after dw: %q", got)
	}
	ed.feedForTest(t, "u")
	if got := ed.text(); got != "hello world" {
		t.Fatalf("after u: %q", got)
	}
	if ed.buf.Modified() {
		t.Fatal("buffer should not be modified after undo to saved state")
	}
	ed.feedForTest(t, "<C-r>")
	if got := ed.text(); got != "world" {
		t.Fatalf("after redo: %q", got)
	}
	ed.feedForTest(t, "u")
	if got := ed.text(); got != "hello world" {
		t.Fatalf("second undo: %q", got)
	}
	ed.feedForTest(t, "iX<Esc>")
	if got := ed.text(); got != "Xhello world" {
		t.Fatalf("insert after undo: %q", got)
	}
	ed.feedForTest(t, "u")
	if got := ed.text(); got != "hello world" {
		t.Fatalf("undo insert: %q", got)
	}
	ed.feedForTest(t, "<C-r><C-r>")
	if got := ed.text(); got != "Xhello world" {
		t.Fatalf("2x redo: %q", got)
	}
}

func TestVisualMode(t *testing.T) {
	cases := []struct {
		name, start, keys, want string
	}{
		{"vd", "hello world", "vwd", "orld"},
		{"v$d", "hello world", "v$d", ""},
		{"vjd", "one\ntwo\nthree", "Vjd", "three"},
		{"Vy", "one\ntwo", "Vy", "one\ntwo"},
		{"vp", "one\ntwo", "Vyp", "one\none\ntwo"},
		{"v~", "hello", "vll~", "HELlo"},
		{"vU", "hello", "vllU", "HELlo"},
		{"vu", "HELLO", "vllu", "helLO"},
		{"block d", "abcd\nefgh\nijkl", "<C-v>jljd", "cd\ngh\nkl"},
		{"block I", "ab\ncd\nef", "<C-v>jIx<Esc>", "xab\nxcd\nef"},
		{"block A", "ab\ncd\nef", "<C-v>jAx<Esc>", "abx\ncdx\nef"},
		{"gv reselect", "hello world", "vll~<Esc>gvU", "HELlo world"},
		{"visual r", "hello", "vllrx", "xxxlo"},
	}
	for _, c := range cases {
		ed := newTestEditor(t, c.start)
		ed.feedForTest(t, c.keys)
		if got := ed.text(); got != c.want {
			t.Errorf("%s: %q -> %q want %q", c.name, c.start, got, c.want)
		}
	}
}

func TestRepeatChange(t *testing.T) {
	ed := newTestEditor(t, "one\ntwo\nthree")
	ed.feedForTest(t, "x")
	if got := ed.text(); got != "ne\ntwo\nthree" {
		t.Fatalf("x: %q", got)
	}
	ed.feedForTest(t, "j.")
	if got := ed.text(); got != "ne\nwo\nthree" {
		t.Fatalf("after .: %q", got)
	}
	ed = newTestEditor(t, "a\nb\nc")
	ed.feedForTest(t, "iX<Esc>j.")
	if got := ed.text(); got != "Xa\nXb\nc" {
		t.Fatalf("insert repeat: %q", got)
	}
	ed = newTestEditor(t, "one two three four")
	ed.feedForTest(t, "dw")
	if got := ed.text(); got != "two three four" {
		t.Fatalf("dw: %q", got)
	}
	ed = newTestEditor(t, "one two three four")
	ed.feedForTest(t, "dw.")
	if got := ed.text(); got != "three four" {
		t.Fatalf("dw then .: %q", got)
	}
}

func TestMacros(t *testing.T) {
	ed := newTestEditor(t, "one\ntwo\nthree")
	ed.feedForTest(t, "qaiX<Esc>q")
	if got := ed.macros['a']; len(got) != 3 {
		t.Fatalf("macro content: %v", got)
	}
	ed.feedForTest(t, "j@a")
	if got := ed.text(); got != "Xone\nXtwo\nthree" {
		t.Fatalf("macro playback: %q", got)
	}
	ed.feedForTest(t, "j@@")
	if got := ed.text(); got != "Xone\nXtwo\nXthree" {
		t.Fatalf("macro repeat: %q", got)
	}
}

func TestSearch(t *testing.T) {
	ed := newTestEditor(t, "alpha beta\ngamma beta\ndelta")
	ed.feedForTest(t, "/beta<CR>")
	if ed.cur != (buffer.Pos{Line: 0, Col: 6}) {
		t.Fatalf("search: %v", ed.cur)
	}
	ed.feedForTest(t, "n")
	if ed.cur != (buffer.Pos{Line: 1, Col: 6}) {
		t.Fatalf("n: %v", ed.cur)
	}
	ed.feedForTest(t, "N")
	if ed.cur != (buffer.Pos{Line: 0, Col: 6}) {
		t.Fatalf("N: %v", ed.cur)
	}
	ed.feedForTest(t, "*")
	ed = newTestEditor(t, "alpha beta\ngamma beta\ndelta")
	ed.feedForTest(t, "w*")
	if ed.cur != (buffer.Pos{Line: 1, Col: 6}) {
		t.Fatalf("*: %v", ed.cur)
	}
	ed = newTestEditor(t, "aaa\nbbb")
	ed.feedForTest(t, "/zzz<CR>")
	if !strings.Contains(ed.msg, "E486") {
		t.Fatalf("expected pattern not found, got %q", ed.msg)
	}
}

func TestSubstitute(t *testing.T) {
	ed := newTestEditor(t, "one two one\none")
	ed.feedForTest(t, ":s/one/1/<CR>")
	if got := ed.text(); got != "1 two one\none" {
		t.Fatalf(":s: %q", got)
	}
	ed.feedForTest(t, ":%s/one/1/g<CR>")
	if got := ed.text(); got != "1 two 1\n1" {
		t.Fatalf(":%%s g: %q", got)
	}
	ed = newTestEditor(t, "a1 b2 c3")
	ed.feedForTest(t, ":s/\\(\\d\\)/x\\1/g<CR>")
	if got := ed.text(); got != "ax1 bx2 cx3" {
		t.Fatalf("groups: %q", got)
	}
	ed = newTestEditor(t, "hello world")
	ed.feedForTest(t, ":s/world/there/<CR>")
	if got := ed.text(); got != "hello there" {
		t.Fatalf("subst: %q", got)
	}
	ed.feedForTest(t, ":s/there/again/<CR>")
	if got := ed.text(); got != "hello again" {
		t.Fatalf("subst 2: %q", got)
	}
	ed.feedForTest(t, "u")
	if got := ed.text(); got != "hello there" {
		t.Fatalf("undo subst: %q", got)
	}
	ed = newTestEditor(t, "one\ntwo\nthree")
	ed.feedForTest(t, ":%s/^/> /<CR>")
	if got := ed.text(); got != "> one\n> two\n> three" {
		t.Fatalf("line anchors: %q", got)
	}
	ed = newTestEditor(t, "Hello hello")
	ed.feedForTest(t, ":%s/hello/x/gi<CR>")
	if got := ed.text(); got != "x x" {
		t.Fatalf("ignore case: %q", got)
	}
}

func TestExCommands(t *testing.T) {
	ed := newTestEditor(t, "one\ntwo\nthree\nfour")
	ed.feedForTest(t, ":2d<CR>")
	if got := ed.text(); got != "one\nthree\nfour" {
		t.Fatalf(":2d: %q", got)
	}
	ed.feedForTest(t, ":1,2m$<CR>")
	if got := ed.text(); got != "four\none\nthree" {
		t.Fatalf(":m: %q", got)
	}
	ed.feedForTest(t, ":1t0<CR>")
	if got := ed.text(); got != "four\nfour\none\nthree" {
		t.Fatalf(":t: %q", got)
	}
	ed = newTestEditor(t, "b\na\nc")
	ed.feedForTest(t, ":sort<CR>")
	if got := ed.text(); got != "a\nb\nc" {
		t.Fatalf(":sort: %q", got)
	}
	ed = newTestEditor(t, "one\ntwo\nthree")
	ed.feedForTest(t, ":1,2j<CR>")
	if got := ed.text(); got != "one two\nthree" {
		t.Fatalf(":j: %q", got)
	}
	ed = newTestEditor(t, "one\ntwo")
	ed.feedForTest(t, ":normal A!<CR>")
	if got := ed.text(); got != "one!\ntwo" {
		t.Fatalf(":normal: %q", got)
	}
	ed = newTestEditor(t, "one\ntwo\none")
	ed.feedForTest(t, ":g/one/d<CR>")
	if got := ed.text(); got != "two" {
		t.Fatalf(":g: %q", got)
	}
	ed = newTestEditor(t, "a\nb\nc")
	ed.feedForTest(t, ":2<CR>")
	if ed.cur.Line != 1 {
		t.Fatalf(":2: %v", ed.cur)
	}
	ed = newTestEditor(t, "a\nb\nc")
	ed.feedForTest(t, ":set number<CR>")
	if !ed.opts.Number {
		t.Fatal(":set number failed")
	}
	ed.feedForTest(t, ":set nonumber<CR>")
	if ed.opts.Number {
		t.Fatal(":set nonumber failed")
	}
	ed.feedForTest(t, ":set ts=4<CR>")
	if ed.opts.TabStop != 4 {
		t.Fatalf("ts = %d", ed.opts.TabStop)
	}
}

func TestMarksAndJumps(t *testing.T) {
	ed := newTestEditor(t, "one\ntwo\nthree\nfour")
	ed.feedForTest(t, "jmaG'a")
	if ed.cur.Line != 1 {
		t.Fatalf("mark jump: %v", ed.cur)
	}
	ed.feedForTest(t, "`a")
	if ed.cur.Line != 1 {
		t.Fatalf("backtick jump: %v", ed.cur)
	}
	ed = newTestEditor(t, "one\ntwo\nthree")
	ed.feedForTest(t, "Ggg<C-o>")
	if ed.cur.Line != 0 {
		t.Fatalf("ctrl-o: %v", ed.cur)
	}
	ed.feedForTest(t, "<C-i>")
	if ed.cur.Line != 2 {
		t.Fatalf("ctrl-i: %v", ed.cur)
	}
}

func TestNumbersAndJoin(t *testing.T) {
	ed := newTestEditor(t, "x = 41")
	ed.feedForTest(t, "$<C-a>")
	if got := ed.text(); got != "x = 42" {
		t.Fatalf("ctrl-a: %q", got)
	}
	ed.feedForTest(t, "<C-x>")
	if got := ed.text(); got != "x = 41" {
		t.Fatalf("ctrl-x: %q", got)
	}
	ed = newTestEditor(t, "one\ntwo")
	ed.feedForTest(t, "J")
	if got := ed.text(); got != "one two" {
		t.Fatalf("J: %q", got)
	}
	ed.feedForTest(t, "u")
	if got := ed.text(); got != "one\ntwo" {
		t.Fatalf("undo J: %q", got)
	}
}

func TestIndentAndCase(t *testing.T) {
	ed := newTestEditor(t, "one\ntwo")
	ed.feedForTest(t, ">>j>>")
	if got := ed.text(); got != "\tone\n\ttwo" {
		t.Fatalf(">>: %q", got)
	}
	ed.feedForTest(t, "gg<<j<<")
	if got := ed.text(); got != "one\ntwo" {
		t.Fatalf("<<: %q", got)
	}
	ed = newTestEditor(t, "one\ntwo\nthree")
	ed.feedForTest(t, "Vj>")
	if got := ed.text(); got != "\tone\n\ttwo\nthree" {
		t.Fatalf("Vj>: %q", got)
	}
	ed = newTestEditor(t, "hello")
	ed.feedForTest(t, "gUiw")
	if got := ed.text(); got != "HELLO" {
		t.Fatalf("gUiw: %q", got)
	}
	ed = newTestEditor(t, "HELLO")
	ed.feedForTest(t, "guiw")
	if got := ed.text(); got != "hello" {
		t.Fatalf("guiw: %q", got)
	}
}

func TestPercentAndParagraph(t *testing.T) {
	ed := newTestEditor(t, "f(a, [b, c])")
	ed.feedForTest(t, "%")
	if ed.cur != (buffer.Pos{Line: 0, Col: 11}) {
		t.Fatalf("%%: %v", ed.cur)
	}
	ed = newTestEditor(t, "a\nb\n\nc\nd")
	ed.feedForTest(t, "}")
	if ed.cur.Line != 2 {
		t.Fatalf("}: %v", ed.cur)
	}
	ed.feedForTest(t, "}")
	if ed.cur.Line != 4 {
		t.Fatalf("}}: %v", ed.cur)
	}
	ed.feedForTest(t, "{")
	if ed.cur.Line != 2 {
		t.Fatalf("{: %v", ed.cur)
	}
	ed = newTestEditor(t, "one\n\ntwo")
	ed.feedForTest(t, "jdd")
	if got := ed.text(); got != "one\ntwo" {
		t.Fatalf("dd on blank: %q", got)
	}
}

func TestDrawDoesNotPanic(t *testing.T) {
	ed := newTestEditor(t, "package main\n\nfunc main() {\n\tprintln(\"hi\")\n}\n")
	ed.opts.Number = true
	ed.opts.CursorLine = true
	ed.setFileType("go")
	ed.draw()
	ed.feedForTest(t, "jjviw")
	ed.draw()
	ed.opts.Wrap = false
	ed.draw()
	ed.scr.Resize(20, 5)
	ed.draw()
}

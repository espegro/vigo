package editor

import (
	"strings"
	"testing"

	"vigo/internal/buffer"
)

func windowCount(e *Editor) int { return len(e.windows) }

func TestSplitHorizontal(t *testing.T) {
	ed := newTestEditor(t, "one\ntwo\nthree")
	if windowCount(ed) != 1 {
		t.Fatalf("initial windows = %d", windowCount(ed))
	}
	ed.feedForTest(t, ":sp<CR>")
	if windowCount(ed) != 2 {
		t.Fatalf("after :sp = %d", windowCount(ed))
	}
	// The cursor stays in the window the user was in, like vim.
	if ed.win != ed.windows[1] {
		t.Fatalf("current window should be the lower one")
	}
	ed.draw()
	top, bottom := ed.windows[0], ed.windows[1]
	if top.y != 0 || top.h != 11 {
		t.Fatalf("top window rect = %+v", top)
	}
	if bottom.y != 11 || bottom.h != 12 {
		t.Fatalf("bottom window rect = %+v", bottom)
	}
	if top.buf != bottom.buf {
		t.Fatal(":sp should show the same buffer")
	}
	rows := ed.scr.Dump()
	if !strings.Contains(rows[0], "one") || !strings.Contains(rows[11], "one") {
		t.Fatalf("both windows should show the file: %q %q", rows[0], rows[11])
	}
	if !strings.Contains(rows[10], "1,1") || !strings.Contains(rows[22], "1,1") {
		t.Fatalf("both windows should have a statusline: %q %q", rows[10], rows[22])
	}
}

func TestSplitVerticalAndSeparator(t *testing.T) {
	ed := newTestEditor(t, "one\ntwo")
	ed.feedForTest(t, ":vs<CR>")
	if windowCount(ed) != 2 {
		t.Fatalf("after :vs = %d", windowCount(ed))
	}
	ed.draw()
	left, right := ed.windows[0], ed.windows[1]
	if left.x != 0 || left.w != 39 {
		t.Fatalf("left = %+v", left)
	}
	if right.x != 40 || right.w != 40 {
		t.Fatalf("right = %+v", right)
	}
	if len(ed.separators) != 1 || ed.separators[0].x != 39 {
		t.Fatalf("separators = %+v", ed.separators)
	}
	rows := ed.scr.Dump()
	if got := []rune(rows[0])[39]; got != '│' {
		t.Fatalf("separator = %q", got)
	}
	if !strings.Contains(rows[0], "one") || !strings.Contains(string([]rune(rows[0])[40:]), "one") {
		t.Fatalf("both columns should show the file: %q", rows[0])
	}
}

func TestWindowNavigation(t *testing.T) {
	ed := newTestEditor(t, "one\ntwo")
	ed.feedForTest(t, ":sp<CR>")
	lower := ed.win
	ed.feedForTest(t, "<C-w>k")
	if ed.win != ed.windows[0] {
		t.Fatal("ctrl-w k should move up")
	}
	ed.feedForTest(t, "<C-w>j")
	if ed.win != lower {
		t.Fatal("ctrl-w j should move down")
	}
	ed.feedForTest(t, "<C-w>w")
	if ed.win != ed.windows[0] {
		t.Fatal("ctrl-w w should cycle")
	}
	ed.feedForTest(t, "<C-w>p")
	if ed.win != lower {
		t.Fatal("ctrl-w p should go back")
	}
}

func TestWindowScrollIsIndependent(t *testing.T) {
	var sb strings.Builder
	for i := 1; i <= 100; i++ {
		sb.WriteString("line ")
		sb.WriteString(itoa(i))
		sb.WriteString("\n")
	}
	ed := newTestEditor(t, sb.String())
	ed.feedForTest(t, ":sp<CR>")
	ed.feedForTest(t, "<C-w>k")
	ed.feedForTest(t, "G")
	if ed.cur.Line != 99 {
		t.Fatalf("cursor = %v", ed.cur)
	}
	ed.feedForTest(t, "<C-w>j")
	if ed.cur.Line != 0 {
		t.Fatalf("the other window should keep its position, got %v", ed.cur)
	}
	ed.feedForTest(t, "<C-w>k")
	if ed.cur.Line != 99 {
		t.Fatalf("first window lost its position: %v", ed.cur)
	}
}

func TestWindowSharesBuffer(t *testing.T) {
	ed := newTestEditor(t, "one\ntwo")
	ed.feedForTest(t, ":sp<CR><C-w>k")
	ed.feedForTest(t, "A!<Esc>")
	ed.feedForTest(t, "<C-w>j")
	if got := ed.text(); got != "one!\ntwo" {
		t.Fatalf("text = %q", got)
	}
	ed.draw()
	rows := ed.scr.Dump()
	if !strings.Contains(rows[11], "one!") {
		t.Fatalf("other window shows %q", rows[11])
	}
}

func TestWindowCloseAndQuit(t *testing.T) {
	ed := newTestEditor(t, "one")
	ed.feedForTest(t, ":sp<CR>")
	if windowCount(ed) != 2 {
		t.Fatalf("windows = %d", windowCount(ed))
	}
	ed.feedForTest(t, "<C-w>c")
	if windowCount(ed) != 1 {
		t.Fatalf("after ctrl-w c = %d", windowCount(ed))
	}
	ed.feedForTest(t, ":q<CR>")
	if !ed.quit {
		t.Fatal(":q on the last window should quit")
	}
	if e := ed.msg; e != "" {
		t.Fatalf("unexpected message %q", e)
	}
	ed = newTestEditor(t, "one")
	ed.feedForTest(t, ":sp<CR>:q<CR>")
	if ed.quit {
		t.Fatal(":q should close the window, not quit")
	}
	if windowCount(ed) != 1 {
		t.Fatalf("windows = %d", windowCount(ed))
	}
	ed.feedForTest(t, ":sp<CR>:sp<CR>:only<CR>")
	if windowCount(ed) != 1 {
		t.Fatalf(":only left %d windows", windowCount(ed))
	}
	ed.feedForTest(t, "<C-w>c")
	if !strings.Contains(ed.msg, "E444") {
		t.Fatalf("expected E444, got %q", ed.msg)
	}
}

func TestWindowResizeAndEqualize(t *testing.T) {
	ed := newTestEditor(t, "one\ntwo")
	ed.feedForTest(t, ":sp<CR>")
	ed.draw()
	before := ed.windows[0].h
	ed.feedForTest(t, "<C-w>+")
	ed.draw()
	grown := ed.win.h
	if grown <= before-1 {
		t.Fatalf("ctrl-w + did not grow the current window: %d -> %d", before, grown)
	}
	before = ed.windows[0].h
	ed.feedForTest(t, "<C-w>=")
	ed.draw()
	if ed.windows[0].h == before && ed.windows[1].h != before+1 {
		t.Fatalf("ctrl-w = did not equalize: %d %d", ed.windows[0].h, ed.windows[1].h)
	}
	ed.feedForTest(t, "<C-w>x")
	ed.draw()
	if windowCount(ed) != 2 {
		t.Fatalf("ctrl-w x changed the window count: %d", windowCount(ed))
	}
}

func TestSplitWithNewFile(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/other.go"
	if err := writeTestFile(path, "package other\n"); err != nil {
		t.Fatal(err)
	}
	ed := newTestEditor(t, "one\ntwo")
	ed.feedForTest(t, ":vs "+path+"<CR>")
	if windowCount(ed) != 2 {
		t.Fatalf("windows = %d", windowCount(ed))
	}
	if len(ed.buffers) != 2 {
		t.Fatalf("buffers = %d", len(ed.buffers))
	}
	ed.draw()
	rows := ed.scr.Dump()
	if !strings.Contains(rows[0], "package other") {
		t.Fatalf("new window content: %q", rows[0])
	}
	// A different buffer needs its own file type.
	if ft := ed.fileTypeFor(ed.windows[0].buf); ft != "go" {
		t.Fatalf("file type of the new buffer = %q", ft)
	}
	if ft := ed.fileTypeFor(ed.windows[1].buf); ft != "" {
		t.Fatalf("file type of the old buffer = %q", ft)
	}
}

func TestWindowBufferDelete(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/two.txt"
	if err := writeTestFile(path, "second\n"); err != nil {
		t.Fatal(err)
	}
	ed := newTestEditor(t, "first\n")
	ed.feedForTest(t, ":vs "+path+"<CR>")
	if len(ed.buffers) != 2 {
		t.Fatalf("buffers = %d", len(ed.buffers))
	}
	ed.feedForTest(t, ":bd<CR>")
	if len(ed.buffers) != 1 {
		t.Fatalf("buffers after :bd = %d", len(ed.buffers))
	}
	if ed.buf != ed.buffers[0] {
		t.Fatal("deleted buffer still in use")
	}
	for _, w := range ed.windows {
		if w.buf != ed.buffers[0] {
			t.Fatal("window still points at the deleted buffer")
		}
	}
}

func TestWindowLayoutAfterResize(t *testing.T) {
	ed := newTestEditor(t, "one\ntwo")
	ed.feedForTest(t, ":sp<CR>")
	ed.scr.Resize(100, 30)
	ed.draw()
	if ed.windows[0].w != 100 || ed.windows[1].w != 100 {
		t.Fatalf("widths after resize: %d %d", ed.windows[0].w, ed.windows[1].w)
	}
	total := ed.windows[0].h + ed.windows[1].h
	if total != 29 {
		t.Fatalf("heights after resize = %d", total)
	}
}

func TestMultipleBuffersInWindows(t *testing.T) {
	ed := newTestEditor(t, "first\n")
	ed.feedForTest(t, ":sp<CR>:enew<CR>")
	if windowCount(ed) != 2 {
		t.Fatalf("windows = %d", windowCount(ed))
	}
	if ed.win.buf == ed.windows[0].buf {
		t.Fatal(":enew should load a different buffer in this window")
	}
	if ed.windows[0].buf == nil || ed.windows[1].buf == nil {
		t.Fatal("every window needs a buffer")
	}
	ed.draw()
	if got := ed.text(); got != "" {
		t.Fatalf("new buffer should be empty, got %q", got)
	}
	ed.feedForTest(t, "ihello<Esc>")
	if ed.windows[0].buf.LineString(0) != "first" {
		t.Fatalf("the other window changed: %q", ed.windows[0].buf.LineString(0))
	}
	ed.feedForTest(t, "<C-w>k")
	if ed.buf.LineString(0) != "first" {
		t.Fatalf("switching windows should switch buffer: %q", ed.buf.LineString(0))
	}
	_ = buffer.Pos{}
}

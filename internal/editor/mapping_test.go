package editor

import (
	"os"
	"strings"
	"testing"

	"vigo/internal/keys"
)

// feedMapped plays keys through the mapping machinery.
func feedMapped(t *testing.T, e *Editor, s string) {
	t.Helper()
	ks, err := keys.Parse(s)
	if err != nil {
		t.Fatalf("parse %q: %v", s, err)
	}
	for _, k := range ks {
		e.feedKey(k)
	}
	if e.mapPending {
		e.flushMappings()
	}
}

func TestNoremapNormal(t *testing.T) {
	ed := newTestEditor(t, "hello")
	ed.execEx("nnoremap Q x")
	feedMapped(t, ed, "Q")
	if got := ed.text(); got != "ello" {
		t.Fatalf("mapped key: %q", got)
	}
}

func TestMappingIsRecursiveOnlyWhenAsked(t *testing.T) {
	ed := newTestEditor(t, "")
	ed.execEx("nnoremap a ihello<Esc>")
	ed.execEx("nmap b a")
	ed.execEx("nnoremap c a")
	feedMapped(t, ed, "b")
	if got := ed.text(); got != "hello" {
		t.Fatalf("recursive map: %q", got)
	}
	ed = newTestEditor(t, "")
	ed.execEx("nnoremap a ihello<Esc>")
	ed.execEx("nnoremap c a")
	feedMapped(t, ed, "c")
	if got := ed.text(); got != "" {
		t.Fatalf("non recursive map should not expand: %q", got)
	}
	if ed.mode != ModeInsert {
		t.Fatalf("a should still enter insert mode, mode = %v", ed.mode)
	}
}

func TestInsertModeMapping(t *testing.T) {
	ed := newTestEditor(t, "")
	ed.execEx("inoremap jk <Esc>")
	feedMapped(t, ed, "iabcjk")
	if got := ed.text(); got != "abc" {
		t.Fatalf("insert mapping: %q", got)
	}
	if ed.mode != ModeNormal {
		t.Fatalf("mode = %v", ed.mode)
	}
}

func TestVisualModeMapping(t *testing.T) {
	ed := newTestEditor(t, "hello")
	ed.execEx("vnoremap Z gU")
	feedMapped(t, ed, "vllZ")
	if got := ed.text(); got != "HELlo" {
		t.Fatalf("visual mapping: %q", got)
	}
}

func TestLeaderMapping(t *testing.T) {
	ed := newTestEditor(t, "")
	ed.feedForTest(t, `:let mapleader = ","<CR>`)
	ed.execEx("nnoremap <leader>x iZ<Esc>")
	feedMapped(t, ed, ",x")
	if got := ed.text(); got != "Z" {
		t.Fatalf("leader mapping: %q", got)
	}
	// The default leader is a backslash.
	ed = newTestEditor(t, "")
	ed.execEx("nnoremap <leader>y iY<Esc>")
	feedMapped(t, ed, "\\y")
	if got := ed.text(); got != "Y" {
		t.Fatalf("default leader: %q", got)
	}
}

func TestMappingPrefixWaitsForMoreKeys(t *testing.T) {
	ed := newTestEditor(t, "one")
	ed.execEx("nnoremap zz iX<Esc>")
	ks := keys.MustParse("z")
	ed.feedKey(ks[0])
	if !ed.mapPending {
		t.Fatal("a mapping prefix should wait for more keys")
	}
	// A timeout flush lets the built in command run instead.
	ed.flushMappings()
	if ed.text() != "one" {
		t.Fatalf("flush should not run the mapping: %q", ed.text())
	}
	feedMapped(t, ed, "zz")
	if got := ed.text(); got != "Xone" {
		t.Fatalf("full mapping: %q", got)
	}
}

func TestMappingNopAndUnmap(t *testing.T) {
	ed := newTestEditor(t, "hello")
	ed.execEx("nnoremap Q <Nop>")
	feedMapped(t, ed, "Q")
	if got := ed.text(); got != "hello" {
		t.Fatalf("<Nop> mapping did nothing: %q", got)
	}
	ed.execEx("nunmap Q")
	feedMapped(t, ed, "Q")
	if ed.mode != ModeReplace {
		t.Fatalf("after unmap Q should be the built in command, mode = %v", ed.mode)
	}
}

func TestMapAppliesToSeveralModes(t *testing.T) {
	ed := newTestEditor(t, "hello")
	ed.execEx("map X x")
	feedMapped(t, ed, "X")
	if got := ed.text(); got != "ello" {
		t.Fatalf("normal: %q", got)
	}
	ed = newTestEditor(t, "hello")
	ed.execEx("map X ~")
	feedMapped(t, ed, "vllX")
	if got := ed.text(); got != "HELlo" {
		t.Fatalf("visual: %q", got)
	}
}

func TestMapListAndClear(t *testing.T) {
	ed := newTestEditor(t, "")
	ed.execEx("nnoremap Q x")
	ed.execEx("nmap")
	if !strings.Contains(ed.msg, "Q") || !strings.Contains(ed.msg, "x") {
		t.Fatalf("listing = %q", ed.msg)
	}
	ed.execEx("mapclear")
	if len(ed.maps['n']) != 0 {
		t.Fatalf("mapclear left %d mappings", len(ed.maps['n']))
	}
}

func TestNormalCommandRespectsMappings(t *testing.T) {
	ed := newTestEditor(t, "hello")
	ed.execEx("nnoremap Q x")
	ed.execEx("normal Q")
	if got := ed.text(); got != "ello" {
		t.Fatalf(":normal should remap: %q", got)
	}
	ed = newTestEditor(t, "hello")
	ed.execEx("nnoremap Q x")
	ed.execEx("normal! Q")
	if got := ed.text(); got != "hello" {
		t.Fatalf(":normal! should not remap: %q", got)
	}
	if ed.mode != ModeReplace {
		t.Fatalf("built in Q should enter replace mode, mode = %v", ed.mode)
	}
}

func TestCtrlKeyMapping(t *testing.T) {
	ed := newTestEditor(t, "hello")
	ed.execEx("nnoremap <C-s> :w! " + os.DevNull + "<CR>")
	feedMapped(t, ed, "<C-s>")
	if got := ed.text(); got != "hello" {
		t.Fatalf("text = %q", got)
	}
}

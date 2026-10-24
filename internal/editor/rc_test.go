package editor

import (
	"strings"
	"testing"
)

func writeRC(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := dir + "/vigorc"
	if err := writeTestFile(path, content); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestSourceRunsSettingsAndMappings(t *testing.T) {
	path := writeRC(t, `" a comment
set number
set shiftwidth=4
let mapleader = ","
nnoremap <leader>q xx
inoremap jk <Esc>
call NotAFunction()
if has('gui_running')
  set nonumber
endif
`)
	ed := newTestEditor(t, "hello")
	ed.execEx("source " + path)
	if !ed.opts.Number {
		t.Fatal(":set number from the rc was not applied")
	}
	if ed.opts.ShiftWidth != 4 {
		t.Fatalf("shiftwidth = %d", ed.opts.ShiftWidth)
	}
	if ed.leader != "," {
		t.Fatalf("leader = %q", ed.leader)
	}
	if !ed.opts.Number {
		t.Fatal("the if block should have been skipped")
	}
	if len(ed.maps['n']) != 1 || len(ed.maps['i']) != 1 {
		t.Fatalf("mappings: n=%d i=%d", len(ed.maps['n']), len(ed.maps['i']))
	}
	if len(ed.scriptErrors) == 0 && !strings.Contains(ed.msg, "not supported") {
		t.Fatal("the unsupported line should be reported")
	}
	feedMapped(t, ed, ",q")
	if got := ed.text(); got != "llo" {
		t.Fatalf("mapping from the rc: %q", got)
	}
}

func TestSourceRejectsMissingFile(t *testing.T) {
	ed := newTestEditor(t, "")
	ed.execEx("source /nonexistent/vigorc")
	if !strings.Contains(ed.msg, "E484") {
		t.Fatalf("message = %q", ed.msg)
	}
}

func TestArgumentsAndEnvRC(t *testing.T) {
	path := writeRC(t, "set number\nset expandtab\n")
	t.Setenv("VIGO_RC", path)
	dir := t.TempDir()
	file := dir + "/a.txt"
	if err := writeTestFile(file, "x\n"); err != nil {
		t.Fatal(err)
	}
	ed := newTestEditor(t, "")
	if err := ed.loadArgs([]string{file}); err != nil {
		t.Fatal(err)
	}
	if !ed.opts.Number || !ed.opts.ExpandTab {
		t.Fatalf("rc from $VIGO_RC not applied: %+v", ed.opts)
	}
	// -u NONE skips the startup file.
	ed = newTestEditor(t, "")
	if err := ed.loadArgs([]string{"-u", "NONE", file}); err != nil {
		t.Fatal(err)
	}
	if ed.opts.Number || ed.opts.ExpandTab {
		t.Fatal("-u NONE should skip the rc")
	}
	// -u FILE uses that file instead.
	ed = newTestEditor(t, "")
	if err := ed.loadArgs([]string{"-u", path, file}); err != nil {
		t.Fatal(err)
	}
	if !ed.opts.Number {
		t.Fatal("-u FILE was not sourced")
	}
}

func TestLetValues(t *testing.T) {
	ed := newTestEditor(t, "")
	ed.execEx(`let mapleader = ','`)
	if ed.leader != "," {
		t.Fatalf("single quoted leader = %q", ed.leader)
	}
	ed.execEx(`let g:mapleader = ";"`)
	if ed.leader != ";" {
		t.Fatalf("g:mapleader = %q", ed.leader)
	}
	ed.execEx(`let mapleader = "\\"`)
	if ed.leader != "\\" {
		t.Fatalf("escaped backslash = %q", ed.leader)
	}
}

func TestRCSkipsBlocksButKeepsNested(t *testing.T) {
	path := writeRC(t, `if 1
  if 0
    set nonumber
  endif
  set number
endif
set expandtab
`)
	ed := newTestEditor(t, "")
	ed.execEx("source " + path)
	if !ed.opts.ExpandTab {
		t.Fatal("settings after the block should run")
	}
	if ed.opts.Number {
		t.Fatal("the whole if block should be skipped")
	}
}

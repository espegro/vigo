package editor

import (
	"math/rand"
	"testing"
	"time"

	"vigo/internal/keys"
)

var fuzzAlphabet = []string{
	"h", "j", "k", "l", "w", "b", "e", "W", "B", "E", "0", "^", "$", "gg", "G",
	"d", "dd", "dw", "de", "d$", "dgg", "dG", "diw", "daw", "di\"", "da(", "dip",
	"y", "yy", "yw", "p", "P", "\"ap", "\"0p", "\"_dd", "\"Ayy",
	"c", "cw", "ciw", "cc", "C", "x", "X", "s", "S", "r", "R", "~", "J", "gJ",
	"i", "a", "o", "O", "I", "A", "<Esc>", "<CR>", "<BS>", "<Del>", "<Tab>",
	"<C-w>", "<C-u>", "<C-n>", "<C-t>", "<C-d>", "<C-a>", "<C-x>",
	"v", "V", "<C-v>", "vl", "Vj", "<C-v>jl", "vy", "vd", "v~", ">", "<", "=",
	"u", "<C-r>", ".", "q", "qa", "qq", "@a", "@@", "m", "ma", "'a", "`a",
	"/", "?", "n", "N", "*", "#", ":", "|", "%", ";", ",", "f", "t", "F", "T",
	"z", "zz", "zt", "zb", "<C-d>", "<C-f>", "<C-b>", "<C-e>", "<C-y>",
	"<C-o>", "<C-i>", ":w! /dev/null<CR>", ":sp<CR>", ":vs<CR>", "<C-w>w",
	"<C-w>q", "<C-w>o", "<C-w>h", "<C-w>j", "<C-w>=", "<C-w>x", ":only<CR>",
	":enew<CR>", ":e!<CR>", ":set nu<CR>", ":set nowrap<CR>", ":set wrap<CR>",
	":set paste<CR>", ":noh<CR>", ":s/a/b/<CR>", ":%s/x/y/g<CR>", ":g/a/d<CR>",
	":%!sort<CR>", ":d<CR>", ":m0<CR>", ":t1<CR>", ":j<CR>", ":retab<CR>",
	":normal! gg<CR>", ":g/zzz/d<CR>", ":1,2m0<CR>", ":5<CR>", ":0<CR>", ":$<CR>",
	"999j", "9999G", "0x", "0p", "100dd", "2i", "3w",
}

// TestFuzzNoPanic feeds pseudo random key sequences to the editor and checks
// that nothing panics, that buffers and windows stay bounded and that no key
// sequence blocks waiting for input it will never get.
func TestFuzzNoPanic(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	base := "package main\n\nfunc main() {\n\tx := 1\n\tif x > 0 {\n\t\ty := \"two\"\n\t}\n}\n// end\n\n\nsheet := `raw`\n"
	for round := 0; round < 30; round++ {
		ed := newTestEditor(t, base)
		ed.opts.Number = round%2 == 0
		ed.opts.CursorLine = round%3 == 0
		ed.opts.CursorColumn = round%5 == 0
		ed.opts.Wrap = round%4 != 0
		if round%7 == 0 {
			ed.setFileType("go")
		}
		for step := 0; step < 60; step++ {
			if len(ed.windows) > 4 {
				ed.execEx("only")
			}
			if ed.buf.Len() > 4000 {
				break
			}
			// Prompts (ctrl-r, the ! filter, :s///c) read from the key channel
			// rather than through handleKey, so leave an escape there: the
			// prompt is cancelled instead of the test hanging.
			select {
			case ed.keyCh <- keys.Special(keys.Esc):
			default:
			}
			str := fuzzAlphabet[rng.Intn(len(fuzzAlphabet))]
			if len(str) == 1 && (str == "r" || str == "f" || str == "m") {
				str += "x" // these keys wait for an argument
			}
			ks, err := keys.Parse(str)
			if err != nil {
				t.Fatalf("bad alphabet entry %q: %v", str, err)
			}
			done := make(chan struct{})
			go func() {
				defer close(done)
				defer func() {
					if r := recover(); r != nil {
						t.Errorf("panic after round %d step %d with %q: %v", round, step, str, r)
					}
				}()
				for _, k := range ks {
					ed.handleKey(k)
				}
				ed.draw()
			}()
			select {
			case <-done:
			case <-time.After(3 * time.Second):
				t.Fatalf("key %q blocked (round %d step %d), mode %v", str, round, step, ed.mode)
			}
			if step%40 == 0 {
				w, h := ed.scr.Size()
				ed.scr.Resize(1+step%50, 2+step%15)
				ed.draw()
				ed.scr.Resize(w, h)
				ed.draw()
			}
		}
	}
}

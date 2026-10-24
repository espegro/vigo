package editor

import "testing"

func TestProbe3Registers(t *testing.T) {
	ed := newTestEditor(t, "one two\nthree\nfour\nfive\n")
	ed.feedForTest(t, "yy")  // -> "0 og "
	ed.feedForTest(t, "jdd") // -> "1
	ed.feedForTest(t, `"0p`) // lim inn yanken
	t.Logf("etter yy, jdd, \"0p: %q", ed.text())
	ed = newTestEditor(t, "one\ntwo\n")
	ed.feedForTest(t, `"_dd`)
	t.Logf("black hole \"_dd -> %q (skal ikke lagre i noe register)", ed.text())
	ed.feedForTest(t, `"ap`)
	t.Logf("  \"ap etterpå: %q", ed.text())
	ed = newTestEditor(t, "one\ntwo\nthree\n")
	ed.feedForTest(t, `"ayy`)
	ed.feedForTest(t, `"Ayy`)
	t.Logf("registre: a=%v", ed.register('a').Lines)
}

func TestProbe3MarksAndJumps(t *testing.T) {
	ed := newTestEditor(t, "alpha\nbeta\ngamma\n")
	ed.feedForTest(t, "jmz")              // sett mark på linje 2
	ed.feedForTest(t, "ggOINSERTED<Esc>") // sett inn en linje øverst
	ed.feedForTest(t, "'z")
	t.Logf("etter innsetting over merket: 'z -> linje %d (vil vim gi 2)", ed.cur.Line+1)
	ed = newTestEditor(t, "a\nb\nc\n")
	ed.feedForTest(t, "G")
	ed.feedForTest(t, "gg``")
	t.Logf("gg`` -> %v (forventet linje 3)", ed.cur.Line+1)
}

func TestProbe3Misc(t *testing.T) {
	ed := newTestEditor(t, "a\nb\nc\nd\n")
	ed.feedForTest(t, ":.,$d<CR>")
	t.Logf(":.,$d -> %q", ed.text())
	ed = newTestEditor(t, "a1\nb2\nc3\n")
	ed.feedForTest(t, ":v/1/d<CR>")
	t.Logf(":v/1/d -> %q", ed.text())
	ed = newTestEditor(t, "f(a[1], b)\n")
	ed.feedForTest(t, "f(l%")
	t.Logf("f( l %% -> kol %d (forventet 2 = '[')", ed.cur.Col)
	ed = newTestEditor(t, "abc\n")
	ed.feedForTest(t, "RXY<Esc>")
	t.Logf("RXY -> %q", ed.text())
	ed = newTestEditor(t, "abc\n")
	ed.feedForTest(t, "RXY<BS>Z<Esc>")
	t.Logf("R XY<BS>Z -> %q (vim ville gitt 'aZC')", ed.text())
	ed = newTestEditor(t, "aaa\nbbb\n")
	ed.feedForTest(t, ">>j.")
	t.Logf(">> j . -> %q", ed.text())
	ed = newTestEditor(t, "hello\n")
	ed.feedForTest(t, "i<C-t>X<Esc>")
	t.Logf("insert <C-t> X -> %q", ed.text())
}

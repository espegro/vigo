package editor

import (
	"fmt"
	"runtime"
	"strings"
	"testing"
	"time"

	"vigo/internal/keys"
)

// bigText builds a file of n lines with a realistic mix of content.
func bigText(n int) string {
	var sb strings.Builder
	for i := 0; i < n; i++ {
		switch i % 5 {
		case 0:
			fmt.Fprintf(&sb, "func f%d() {\n", i)
		case 1:
			fmt.Fprintf(&sb, "\tx := %d // comment\n", i)
		case 2:
			fmt.Fprintf(&sb, "\tif x > 0 {\n")
		case 3:
			fmt.Fprintf(&sb, "\t\treturn x + %d\n", i)
		default:
			fmt.Fprintf(&sb, "\t}\n")
		}
	}
	return sb.String()
}

func heapDelta(before, after uint64) string {
	return fmt.Sprintf("%.1f MiB", float64(after-before)/(1<<20))
}

func TestLimitsLargeFile(t *testing.T) {
	const lines = 200000
	text := bigText(lines)
	t.Logf("fil: %d linjer, %.1f MiB tekst", lines, float64(len(text))/(1<<20))

	runtime.GC()
	var m0, m1 runtime.MemStats
	runtime.ReadMemStats(&m0)

	start := time.Now()
	ed := newTestEditor(t, text)
	open := time.Since(start)
	runtime.ReadMemStats(&m1)
	t.Logf("åpne+parse: %v, heap %s", open.Round(time.Millisecond), heapDelta(m0.HeapAlloc, m1.HeapAlloc))

	start = time.Now()
	ed.draw()
	t.Logf("første tegnediff (syntax av over 50k linjer): %v", time.Since(start).Round(time.Millisecond))

	ed.setFileType("go")
	start = time.Now()
	ed.draw()
	t.Logf("draw med syntax aktivert: %v", time.Since(start).Round(time.Millisecond))

	start = time.Now()
	ed.feedForTest(t, "G")
	ed.draw()
	t.Logf("G + draw: %v", time.Since(start).Round(time.Millisecond))

	start = time.Now()
	ed.feedForTest(t, "/return x + 199999<CR>")
	t.Logf("søk i hele fila: %v", time.Since(start).Round(time.Millisecond))

	start = time.Now()
	ed.draw()
	t.Logf("draw etter søk (highlight-kalkyle): %v", time.Since(start).Round(time.Millisecond))

	start = time.Now()
	for i := 0; i < 200; i++ {
		ed.feedForTest(t, "x")
		ed.feedForTest(t, "u")
	}
	t.Logf("200 x/undo-sykluser: %v", time.Since(start).Round(time.Millisecond))
}

func TestLimitsVeryLongLine(t *testing.T) {
	for _, n := range []int{100000, 1000000} {
		line := strings.Repeat("abcdefghij", n/10)
		ed := newTestEditor(t, line+"\nsecond\n")
		ed.opts.Wrap = true
		start := time.Now()
		ed.draw()
		first := time.Since(start)
		start = time.Now()
		ed.feedForTest(t, "G")
		ed.draw()
		g1 := time.Since(start).Round(time.Millisecond)
		start = time.Now()
		ed.draw()
		g2 := time.Since(start).Round(time.Millisecond)
		start = time.Now()
		ed.feedForTest(t, "gg")
		ed.draw()
		t.Logf("linje på %d tegn: første draw %v, G+draw %v, neste draw %v, gg+draw %v (wrap)",
			n, first.Round(time.Millisecond), g1, g2, time.Since(start).Round(time.Millisecond))

		ed.opts.Wrap = false
		start = time.Now()
		ed.draw()
		t.Logf("  uten wrap: %v", time.Since(start).Round(time.Millisecond))

		start = time.Now()
		ed.feedForTest(t, "$")
		ed.draw()
		t.Logf("  $ + draw (horisontal scroll): %v", time.Since(start).Round(time.Millisecond))
	}
}

func TestLimitsManyWindows(t *testing.T) {
	ed := newTestEditor(t, "one\ntwo\n")
	start := time.Now()
	for i := 0; i < 60; i++ {
		ed.execEx("sp")
	}
	ed.draw()
	t.Logf("60 splits: %d vinduer, %v", len(ed.windows), time.Since(start).Round(time.Millisecond))
	start = time.Now()
	for i := 0; i < 500; i++ {
		ed.draw()
	}
	t.Logf("500 draws med 61 vinduer: %v", time.Since(start).Round(time.Millisecond))
}

func TestLimitsUndoMemory(t *testing.T) {
	text := bigText(20000) // ~2 MiB
	ed := newTestEditor(t, text)
	ed.buf.UndoLimit = 100000
	runtime.GC()
	var m0, m1 runtime.MemStats
	runtime.ReadMemStats(&m0)
	for i := 0; i < 300; i++ {
		ed.feedForTest(t, "ggx")
		ed.feedForTest(t, "u")
	}
	runtime.ReadMemStats(&m1)
	t.Logf("%d endringer på en %d-linjers fil: heap %s (budsjett 128 MiB)", 300, ed.buf.Len(), heapDelta(m0.HeapAlloc, m1.HeapAlloc))
}

// pasteKeysFrom turns text into the keys a terminal would deliver.
func pasteKeysFrom(text string) []keys.Key {
	var out []keys.Key
	for _, r := range text {
		if r == '\n' {
			k := keys.Special(keys.CR)
			k.Paste = true
			out = append(out, k)
			continue
		}
		k := keys.Rune(r)
		k.Paste = true
		out = append(out, k)
	}
	return out
}

func TestLimitsPasteSize(t *testing.T) {
	ed := newTestEditor(t, "")
	blob := strings.Repeat("line of pasted text\n", 200000) // ~4 MiB
	start := time.Now()
	ed.execEx("set paste")
	ed.feedForTest(t, "i")
	ed.handlePasteKeys(pasteKeysFrom(blob))
	t.Logf("lim inn %.1f MiB: %d linjer, %v", float64(len(blob))/(1<<20), ed.buf.Len(), time.Since(start).Round(time.Millisecond))
}

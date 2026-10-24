package editor

import (
	"os"
	"runtime"
	"testing"
)

func TestPlatformDefaults(t *testing.T) {
	shell, args := shellCommandLine("echo hi")
	if runtime.GOOS == "windows" {
		if defaultEOL() != "\r\n" {
			t.Fatalf("windows default eol = %q", defaultEOL())
		}
		if !pollResize {
			t.Fatal("windows must poll the terminal size")
		}
		if shell != "cmd" || len(args) != 2 || args[0] != "/c" {
			t.Fatalf("windows shell = %q %q", shell, args)
		}
		return
	}
	if defaultEOL() != "\n" {
		t.Fatalf("unix default eol = %q", defaultEOL())
	}
	if pollResize {
		t.Fatal("unix has SIGWINCH and should not poll")
	}
	if shell != "sh" || len(args) != 2 || args[0] != "-c" {
		t.Fatalf("unix shell = %q %q", shell, args)
	}
}

func TestNewFileUsesPlatformEOL(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/new.txt"
	ed := newTestEditor(t, "")
	if err := ed.openFileIn(ed.buf, path, true); err != nil {
		t.Fatal(err)
	}
	ed.feedForTest(t, "ihello<CR>world<Esc>")
	if err := ed.writeBuffer(ed.buf, path, false); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want := "hello" + defaultEOL() + "world"
	if string(data) != want {
		t.Fatalf("new file = %q want %q", data, want)
	}
}

func TestCRLFFileKeepsItsLineEndings(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/dos.txt"
	if err := os.WriteFile(path, []byte("one\r\ntwo\r\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ed := newTestEditor(t, "")
	if err := ed.openFileIn(ed.buf, path, true); err != nil {
		t.Fatal(err)
	}
	if ed.buf.EOL() != "\r\n" {
		t.Fatalf("eol = %q", ed.buf.EOL())
	}
	ed.feedForTest(t, "A!<Esc>")
	if err := ed.writeBuffer(ed.buf, path, false); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	if string(data) != "one!\r\ntwo\r\n" {
		t.Fatalf("round trip = %q", data)
	}
	if ed.buf.Modified() {
		t.Fatal("buffer should be clean after writing")
	}
}

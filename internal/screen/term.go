package screen

import (
	"bufio"
	"encoding/base64"
	"errors"
	"fmt"
	"os"

	"golang.org/x/term"
)

// Terminal owns the tty: raw mode, alternate screen and terminal queries.
type Terminal struct {
	In        *os.File
	Out       *os.File
	raw       rawState
	out       *bufio.Writer
	altOn     bool
	setupWarn error
}

// SetupWarning reports a non fatal problem with the terminal setup, for
// instance a console that refused to enable virtual terminal processing.
func (t *Terminal) SetupWarning() error { return t.setupWarn }

// VTInput reports whether keys arrive as escape sequences. It is always true
// outside Windows, and can be false on a console that refused
// ENABLE_VIRTUAL_TERMINAL_INPUT.
func (t *Terminal) VTInput() bool { return t.raw.vtInput }

// OpenTerminal puts stdin into raw mode and prepares the alternate screen.
func OpenTerminal() (*Terminal, error) {
	in, out := os.Stdin, os.Stdout
	if !term.IsTerminal(int(in.Fd())) {
		return nil, errors.New("stdin is not a terminal")
	}
	raw, err := makeRawInput(in)
	if err != nil {
		return nil, fmt.Errorf("raw mode: %w", err)
	}
	t := &Terminal{In: in, Out: out, raw: raw, out: bufio.NewWriterSize(out, 1<<15)}
	if err := setupPlatform(t); err != nil {
		// Not fatal: the terminal may already handle escape sequences, or
		// refuse one of the flags we asked for.
		t.setupWarn = err
	}
	t.EnterAltScreen()
	t.EnableBracketedPaste()
	t.SetCursorShape(CursorBlock)
	t.Reset()
	return t, nil
}

// emit writes an escape sequence straight through.
func (t *Terminal) emit(s string) {
	t.out.WriteString(s)
	t.out.Flush()
}

// Size reports the terminal size in cells.
func (t *Terminal) Size() (int, int) {
	w, h, err := term.GetSize(int(t.Out.Fd()))
	if err != nil || w <= 0 || h <= 0 {
		return 80, 24
	}
	return w, h
}

// IsTerminal reports whether f refers to a terminal.
func IsTerminal(f *os.File) bool { return term.IsTerminal(int(f.Fd())) }

// Reset clears the screen and homes the cursor.
func (t *Terminal) Reset() { t.emit("\x1b[0m\x1b[H\x1b[2J") }

// EnterAltScreen switches to the alternate screen buffer.
func (t *Terminal) EnterAltScreen() {
	t.altOn = true
	t.emit("\x1b[?1049h\x1b[0m\x1b[H\x1b[2J")
}

// LeaveAltScreen switches back to the primary screen buffer.
func (t *Terminal) LeaveAltScreen() {
	if !t.altOn {
		return
	}
	t.altOn = false
	t.emit("\x1b[?1049l")
}

// EnableBracketedPaste turns on bracketed paste reporting.
func (t *Terminal) EnableBracketedPaste() { t.emit("\x1b[?2004h") }

// DisableBracketedPaste turns off bracketed paste reporting.
func (t *Terminal) DisableBracketedPaste() { t.emit("\x1b[?2004l") }

// CursorShape is a DECSCUSR cursor style.
type CursorShape int

// Cursor shapes.
const (
	CursorBlock CursorShape = 2
	CursorBar   CursorShape = 6
	CursorUnder CursorShape = 4
)

// SetCursorShape changes the cursor style.
func (t *Terminal) SetCursorShape(shape CursorShape) {
	t.emit(fmt.Sprintf("\x1b[%d q", int(shape)))
}

// SetTitle sets the window title.
func (t *Terminal) SetTitle(title string) {
	t.emit("\x1b]0;" + title + "\a")
}

// CopyToClipboard sends text to the terminal's clipboard using OSC 52.
func (t *Terminal) CopyToClipboard(text string) error {
	enc := base64.StdEncoding.EncodeToString([]byte(text))
	const limit = 100000
	if len(enc) > limit {
		return errors.New("clipboard data too large")
	}
	t.emit("\x1b]52;c;" + enc + "\a")
	return nil
}

// Restore leaves the alternate screen and restores the original terminal state.
func (t *Terminal) Restore() {
	if t.raw.restore == nil {
		return
	}
	t.SetCursorShape(CursorBlock)
	t.DisableBracketedPaste()
	t.LeaveAltScreen()
	t.raw.restore()
	t.raw.restore = nil
}

// Suspend gives the terminal back to the shell for a while, for instance while
// running a shell command from :!.
func (t *Terminal) Suspend() {
	t.SetCursorShape(CursorBlock)
	t.DisableBracketedPaste()
	t.LeaveAltScreen()
	if t.raw.restore != nil {
		t.raw.restore()
		t.raw.restore = nil
	}
}

// Resume takes the terminal back after Suspend.
func (t *Terminal) Resume() {
	if t.raw.restore == nil {
		if raw, err := makeRawInput(t.In); err == nil {
			t.raw = raw
		}
	}
	t.EnterAltScreen()
	t.EnableBracketedPaste()
	t.SetCursorShape(CursorBlock)
}

//go:build windows

package screen

import (
	"os"

	"golang.org/x/sys/windows"
)

// setupPlatform turns on virtual terminal handling so that the escape
// sequences this package writes are interpreted and the keys we read arrive as
// escape sequences. Windows Terminal and recent consoles enable this already;
// the legacy console has to be asked. The calls are best effort: a console that
// rejects one of the flags still works, and the error is reported at startup
// instead of breaking the build.
func setupPlatform(t *Terminal) error {
	var firstErr error
	note := func(err error) {
		if err != nil && firstErr == nil {
			firstErr = err
		}
	}
	if out := t.Out; out != nil {
		note(setMode(out, windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING))
	}
	if in := t.In; in != nil {
		note(setMode(in, windows.ENABLE_VIRTUAL_TERMINAL_INPUT))
	}
	return firstErr
}

// setMode turns on the given console mode flags.
func setMode(f *os.File, flags uint32) error {
	h := windows.Handle(f.Fd())
	var mode uint32
	if err := windows.GetConsoleMode(h, &mode); err != nil {
		return err
	}
	if mode&flags == flags {
		return nil
	}
	return windows.SetConsoleMode(h, mode|flags)
}

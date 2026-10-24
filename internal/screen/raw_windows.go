//go:build windows

package screen

import (
	"os"

	"golang.org/x/sys/windows"
)

// rawState is the console state needed to undo raw mode.
type rawState struct {
	restore func()
	// vtInput reports whether ENABLE_VIRTUAL_TERMINAL_INPUT could be turned on,
	// which is what makes arrow keys and friends arrive as escape sequences.
	vtInput bool
}

// makeRawInput switches the console to raw mode.
//
// ENABLE_VIRTUAL_TERMINAL_INPUT is requested because it makes key presses
// arrive as escape sequences, but some consoles (notably pseudo consoles used
// by ssh sessions) reject that flag with ERROR_INVALID_PARAMETER. In that case
// we still put the console into plain raw mode instead of failing.
func makeRawInput(f *os.File) (rawState, error) {
	h := windows.Handle(f.Fd())
	var mode uint32
	if err := windows.GetConsoleMode(h, &mode); err != nil {
		return rawState{}, err
	}
	raw := mode &^ (windows.ENABLE_ECHO_INPUT | windows.ENABLE_PROCESSED_INPUT | windows.ENABLE_LINE_INPUT)
	vt := false
	if err := windows.SetConsoleMode(h, raw|windows.ENABLE_VIRTUAL_TERMINAL_INPUT); err == nil {
		vt = true
	} else if err := windows.SetConsoleMode(h, raw); err != nil {
		return rawState{}, err
	}
	return rawState{
		restore: func() { windows.SetConsoleMode(h, mode) },
		vtInput: vt,
	}, nil
}

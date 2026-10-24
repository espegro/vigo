//go:build !windows

package screen

import (
	"os"

	"golang.org/x/term"
)

// rawState is the terminal state needed to undo raw mode.
type rawState struct {
	restore func()
	// vtInput reports whether keys arrive as escape sequences, which is always
	// the case on Unix.
	vtInput bool
}

// makeRawInput switches the terminal to raw mode.
func makeRawInput(f *os.File) (rawState, error) {
	state, err := term.MakeRaw(int(f.Fd()))
	if err != nil {
		return rawState{}, err
	}
	return rawState{
		restore: func() { term.Restore(int(f.Fd()), state) },
		vtInput: true,
	}, nil
}

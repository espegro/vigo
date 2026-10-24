//go:build windows

package editor

import (
	"os"
	"os/signal"
)

// watchSignals subscribes to the signals the editor reacts to. Windows has no
// window change signal, so the size is polled instead.
func watchSignals(ch chan os.Signal) {
	signal.Notify(ch, os.Interrupt)
}

// isResize always reports false on Windows.
func isResize(os.Signal) bool { return false }

// pollResize is true when the terminal size must be polled.
const pollResize = true

// defaultEOL is the line ending used for new files.
func defaultEOL() string { return "\r\n" }

// shellCommandLine builds a shell invocation for :! and the ! filter.
func shellCommandLine(command string) (string, []string) {
	return "cmd", []string{"/c", command}
}

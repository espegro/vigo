//go:build !windows

package editor

import (
	"os"
	"os/signal"
	"syscall"
)

// watchSignals subscribes to the signals the editor reacts to.
func watchSignals(ch chan os.Signal) {
	signal.Notify(ch, syscall.SIGWINCH, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGQUIT)
}

// isResize reports whether s means "the terminal changed size".
func isResize(s os.Signal) bool { return s == syscall.SIGWINCH }

// pollResize is true when the terminal size must be polled because the
// platform has no resize signal.
const pollResize = false

// defaultEOL is the line ending used for new files.
func defaultEOL() string { return "\n" }

// shellCommandLine builds a shell invocation for :! and the ! filter.
func shellCommandLine(command string) (string, []string) {
	return "sh", []string{"-c", command}
}

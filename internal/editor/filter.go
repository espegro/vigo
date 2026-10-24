package editor

import (
	"fmt"
	"os/exec"
	"strings"

	"vigo/internal/buffer"
)

// filterText pipes text through a shell command and returns its output.
func (e *Editor) filterText(text, command string) (string, error) {
	if e.term != nil {
		e.restoreTerminal()
		defer e.reenterTerminal()
	}
	shell, args := shellCommandLine(command)
	c := exec.Command(shell, args...)
	c.Stdin = strings.NewReader(text)
	out, err := c.Output()
	if err != nil {
		return string(out), err
	}
	return string(out), nil
}

// filterRange replaces the lines of a range with the output of a shell filter.
func (e *Editor) filterRange(r rangeInfo, command string) {
	nr := e.normalizeRange(r)
	from := clamp(nr.from.Line, 0, e.buf.LastLine())
	to := clamp(nr.to.Line-1, 0, e.buf.LastLine())
	if from > to {
		return
	}
	text := strings.Join(e.lineStrings(from, to), "\n") + "\n"
	out, err := e.filterText(text, command)
	if err != nil {
		e.errorf("shell returned %v", err)
		return
	}
	lines := splitLines(out)
	e.buf.Checkpoint(e.cur)
	e.buf.ReplaceLines(from, to, lines)
	line := clamp(from, 0, e.buf.LastLine())
	e.setCursor(buffer.Pos{Line: line, Col: e.buf.FirstNonBlank(line).Col})
	if len(out) > 0 {
		e.messagef("%d lines filtered", len(lines))
	}
}

// askFilter asks for a shell command and pipes the range through it, which is
// what "!" as an operator does.
func (e *Editor) askFilter(r rangeInfo) {
	nr := e.normalizeRange(r)
	from := clamp(nr.from.Line, 0, e.buf.LastLine())
	to := clamp(nr.to.Line-1, 0, e.buf.LastLine())
	if to < from {
		to = from
	}
	command, ok := e.ask(fmt.Sprintf(":%d,%d!", from+1, to+1))
	if !ok || strings.TrimSpace(command) == "" {
		return
	}
	e.filterRange(rangeInfo{
		from: buffer.Pos{Line: from},
		to:   buffer.Pos{Line: to + 1},
		kind: mLine,
	}, command)
}

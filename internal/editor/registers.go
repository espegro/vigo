package editor

import "strings"

// Register is the content of one vim register.
type Register struct {
	Lines     []string
	Linewise  bool
	Blockwise bool
	Width     int
}

// Text returns the register content as a plain string.
func (r Register) Text() string {
	if r.Linewise {
		return strings.Join(r.Lines, "\n") + "\n"
	}
	return strings.Join(r.Lines, "\n")
}

// Empty reports whether the register holds nothing.
func (r Register) Empty() bool {
	return len(r.Lines) == 0
}

// TextRegister builds a character wise register.
func TextRegister(text string, blockwise bool, width int) Register {
	return Register{Lines: []string{text}, Blockwise: blockwise, Width: width}
}

// LineRegister builds a line wise register.
func LineRegister(lines []string) Register {
	return Register{Lines: lines, Linewise: true}
}

func (e *Editor) register(name rune) Register {
	if name == '.' {
		e.flushInserted()
	}
	if r, ok := e.regs[name]; ok {
		return r
	}
	return Register{}
}

// storeRegister writes r to name, applying the vim side effects: the unnamed
// register always follows, deletes shift the numbered registers and yanks fill
// register 0.
func (e *Editor) storeRegister(name rune, r Register, isYank bool, isSmallDelete bool) {
	if name == '_' {
		return
	}
	if name == 0 {
		name = '"'
	}
	e.putRegister(name, r)
	switch {
	case name == '"':
		if isYank {
			e.putRegister('0', r)
		} else if isSmallDelete {
			e.putRegister('-', r)
		} else {
			e.shiftNumberedRegisters(r)
		}
	case name >= 'a' && name <= 'z', name >= 'A' && name <= 'Z':
		e.putRegister('"', r)
	case name >= '1' && name <= '9':
		e.putRegister('"', r)
	default:
		e.putRegister('"', r)
	}
}

func (e *Editor) putRegister(name rune, r Register) {
	if name >= 'A' && name <= 'Z' {
		lower := name - 'A' + 'a'
		prev := e.register(lower)
		if prev.Empty() {
			e.regs[lower] = r
		} else if prev.Linewise == r.Linewise && !prev.Blockwise && !r.Blockwise {
			e.regs[lower] = Register{
				Lines:    append(append([]string{}, prev.Lines...), r.Lines...),
				Linewise: prev.Linewise,
			}
		} else {
			e.regs[lower] = r
		}
		e.maybeClipboard(lower, e.regs[lower])
		return
	}
	e.regs[name] = r
	e.maybeClipboard(name, r)
}

func (e *Editor) maybeClipboard(name rune, r Register) {
	if name != '+' && name != '*' {
		return
	}
	if e.term == nil {
		return
	}
	if err := e.term.CopyToClipboard(r.Text()); err != nil {
		e.errorf("%v", err)
	}
}

func (e *Editor) shiftNumberedRegisters(r Register) {
	if r.Linewise && len(r.Lines) == 1 {
		e.putRegister('-', r)
	} else if !r.Linewise && !strings.Contains(r.Text(), "\n") {
		e.putRegister('-', r)
	}
	for i := 9; i > 1; i-- {
		if prev, ok := e.regs[rune('0'+i-1)]; ok {
			e.putRegister(rune('0'+i), prev)
		}
	}
	e.putRegister('1', r)
}

// defaultRegister returns the register used when the user did not name one.
func (e *Editor) defaultRegister() rune {
	if e.pending.regSet {
		return e.pending.reg
	}
	return '"'
}

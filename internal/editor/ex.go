package editor

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"vigo/internal/buffer"
	"vigo/internal/keys"
)

// exCommand is a parsed ex command line.
type exCommand struct {
	rangeSet bool
	from, to int
	cmd      string
	bang     bool
	count    int
	countSet bool
	args     string
}

var exCommandList = []string{
	"&", "!", "b", "bd", "bdelete", "bnext", "bp", "bprevious", "buffer",
	"buffers", "cd", "co", "copy", "d", "delete", "e", "edit", "enew",
	"close", "file", "files", "g", "global", "h", "help", "j", "join", "ls", "m", "marks",
	"move", "new", "only",
	"n", "next", "noh", "nohlsearch", "norm", "normal", "p", "prev",
	"previous", "print", "pu", "put", "pwd", "q", "qa", "qall", "quit",
	"read", "redo", "retab", "rew", "rewind", "s", "set", "sort", "substitute",
	"source", "u", "undo", "v", "version", "vglobal", "w", "wa", "wall", "write",
	"vnew", "wq", "x", "xit", "~", "sp", "split", "vs", "vsplit",
}

// mapCommands maps the :map family to a mode letter and whether the right hand
// side is remapped. Mode 0 means "normal, visual and operator-pending" and '!'
// means "insert and command line", like vim.
var mapCommands = map[string]struct {
	mode  byte
	remap bool
}{
	"map":      {0, true},
	"noremap":  {0, false},
	"nmap":     {'n', true},
	"nnoremap": {'n', false},
	"vmap":     {'v', true},
	"vnoremap": {'v', false},
	"xmap":     {'x', true},
	"xnoremap": {'x', false},
	"smap":     {'v', true},
	"snoremap": {'v', false},
	"omap":     {'o', true},
	"onoremap": {'o', false},
	"imap":     {'i', true},
	"inoremap": {'i', false},
	"lmap":     {'i', true},
	"lnoremap": {'i', false},
	"cmap":     {'c', true},
	"cnoremap": {'c', false},
}

var unmapCommands = map[string]byte{
	"unmap": 0, "nunmap": 'n', "vunmap": 'v', "xunmap": 'x', "sunmap": 'v',
	"ounmap": 'o', "iunmap": 'i', "lunmap": 'i', "cunmap": 'c',
}

var mapClearCommands = map[string]byte{
	"mapclear": 0, "nmapclear": 'n', "vmapclear": 'v', "xmapclear": 'x',
	"smapclear": 'v', "omapclear": 'o', "imapclear": 'i', "lmapclear": 'i',
	"cmapclear": 'c',
}

func exCommandNames() []string { return exCommandList }

// parseEx splits an ex command line into range, command, flags and arguments.
func (e *Editor) parseEx(line string) (exCommand, error) {
	var cmd exCommand
	rest := line
	trimmed := strings.TrimLeft(rest, " \t:")
	if trimmed == rest {
		// A line number on its own is a jump command.
	}
	from, to, rest, ok, err := e.parseRange(trimmed)
	if err != nil {
		return cmd, err
	}
	if ok {
		cmd.rangeSet = true
		cmd.from, cmd.to = from, to
	}
	rest = strings.TrimLeft(rest, " \t")
	if rest == "" {
		return cmd, nil
	}
	if rest[0] == '!' {
		cmd.cmd = "!"
		rest = rest[1:]
	} else if rest[0] == '&' || rest[0] == '~' {
		cmd.cmd = string(rest[0])
		rest = rest[1:]
	} else {
		i := 0
		for i < len(rest) && isCmdChar(rest[i]) {
			i++
		}
		cmd.cmd = rest[:i]
		rest = rest[i:]
	}
	// A bare number after the range is a line number for :m, :t and friends.
	if cmd.cmd == "" {
		rest = strings.TrimLeft(rest, " \t")
	}
	if strings.HasPrefix(rest, "!") {
		cmd.bang = true
		rest = rest[1:]
	}
	rest = strings.TrimLeft(rest, " \t")
	if cmd.cmd == "" {
		j := 0
		for j < len(rest) && rest[j] >= '0' && rest[j] <= '9' {
			j++
		}
		if j > 0 {
			n, err := strconv.Atoi(rest[:j])
			if err == nil {
				cmd.count, cmd.countSet = n, true
				rest = rest[j:]
			}
		}
	}
	cmd.args = strings.TrimLeft(rest, " \t")
	return cmd, nil
}

func isCmdChar(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
}

// parseRange parses an ex line range and returns 0-based line numbers.
func (e *Editor) parseRange(s string) (from, to int, rest string, ok bool, err error) {
	line, rest, found, err := e.parseAddress(s)
	if err != nil {
		return 0, 0, s, false, err
	}
	if !found {
		return 0, 0, s, false, nil
	}
	from, to = line, line
	if len(s) > 0 && s[0] == '%' {
		from, to = 0, e.buf.LastLine()
		if rest != "" && (rest[0] == ',' || rest[0] == ';') {
			rest = rest[1:]
			if rest != "" {
				l2, r2, found2, err2 := e.parseAddress(rest)
				if err2 == nil && found2 {
					to = l2
					rest = r2
				}
			}
		}
		return from, to, rest, true, nil
	}
	if rest != "" && (rest[0] == ',' || rest[0] == ';') {
		sep := rest[0]
		rest = rest[1:]
		if sep == ';' {
			from = line
		}
		if rest != "" && (rest[0] == '+' || rest[0] == '-' || rest[0] == '.' || rest[0] == '$' ||
			rest[0] == '\'' || rest[0] == '/' || rest[0] == '?' || (rest[0] >= '0' && rest[0] <= '9')) {
			l2, r2, found2, err := e.parseAddress(rest)
			if err != nil {
				return 0, 0, s, false, err
			}
			if found2 {
				to = l2
				rest = r2
			}
		} else {
			to = e.buf.LastLine()
		}
	}
	if to < from {
		from, to = to, from
	}
	return from, to, rest, true, nil
}

// parseAddress parses one address: . $ % N 'x /pat/ ?pat? +N -N
func (e *Editor) parseAddress(s string) (line int, rest string, ok bool, err error) {
	if s == "" {
		return 0, s, false, nil
	}
	switch s[0] {
	case '%':
		return e.buf.LastLine(), s[1:], true, nil
	case '.':
		line, s, ok = e.cur.Line, s[1:], true
	case '$':
		line, s, ok = e.buf.LastLine(), s[1:], true
	case '\'':
		if len(s) < 2 {
			return 0, s, false, fmt.Errorf("E14: Invalid address")
		}
		name := rune(s[1])
		s = s[2:]
		switch name {
		case '<':
			if e.hasLastVisual {
				line = e.lastVisual[0].Line
			}
		case '>':
			if e.hasLastVisual {
				line = e.lastVisual[1].Line
			}
		case '.', '\'':
			line = e.cur.Line
		case '"':
			line = e.cur.Line
		default:
			pos, ok2 := e.marks[name]
			if !ok2 {
				return 0, s, false, fmt.Errorf("E20: Mark not set")
			}
			line = pos.Line
		}
		ok = true
	case '/', '?':
		forward := s[0] == '/'
		delim := s[0]
		end := strings.IndexByte(s[1:], delim)
		if end < 0 {
			return 0, s, false, fmt.Errorf("E486: Pattern not found")
		}
		pat := s[1 : 1+end]
		s = s[2+end:]
		if pat == "" {
			pat = e.lastSearch
		}
		if pat == "" {
			return 0, s, false, nil
		}
		dest, found := e.doSearch(pat, forward, 1)
		if !found {
			return 0, s, false, fmt.Errorf("E486: Pattern not found: %s", pat)
		}
		line, ok = dest.Line, true
	default:
		if s[0] >= '0' && s[0] <= '9' {
			i := 0
			for i < len(s) && s[i] >= '0' && s[i] <= '9' {
				i++
			}
			n, err := strconv.Atoi(s[:i])
			if err != nil {
				return 0, s, false, err
			}
			line, s, ok = n-1, s[i:], true
		}
	}
	if !ok {
		return 0, s, false, nil
	}
	for s != "" {
		switch s[0] {
		case '+':
			s = s[1:]
			n, rest2, hasN := leadingNumber(s)
			if !hasN {
				n = 1
			}
			line += n
			s = rest2
		case '-':
			s = s[1:]
			n, rest2, hasN := leadingNumber(s)
			if !hasN {
				n = 1
			}
			line -= n
			s = rest2
		default:
			return clamp(line, 0, e.buf.LastLine()), s, true, nil
		}
	}
	return clamp(line, 0, e.buf.LastLine()), s, true, nil
}

func leadingNumber(s string) (int, string, bool) {
	i := 0
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	if i == 0 {
		return 0, s, false
	}
	n, err := strconv.Atoi(s[:i])
	if err != nil {
		return 0, s, false
	}
	return n, s[i:], true
}

// execEx runs a whole ex command line.
func (e *Editor) execEx(line string) {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return
	}
	cmd, err := e.parseEx(trimmed)
	if err != nil {
		e.errorf("%v", err)
		return
	}
	if cmd.cmd == "" {
		if cmd.rangeSet {
			e.gotoLine(cmd.from)
		}
		return
	}
	if spec, ok := mapCommands[cmd.cmd]; ok {
		mode := spec.mode
		if cmd.bang && mode == 0 {
			mode = '!'
		}
		e.exMapFamily(mode, spec.remap, cmd.args)
		return
	}
	if mode, ok := unmapCommands[cmd.cmd]; ok {
		if cmd.bang && mode == 0 {
			mode = '!'
		}
		e.exUnmapFamily(mode, cmd.args)
		return
	}
	if mode, ok := mapClearCommands[cmd.cmd]; ok {
		if cmd.bang && mode == 0 {
			mode = '!'
		}
		e.exMapClearFamily(mode)
		return
	}
	if cmd.rangeSet && cmd.to == e.buf.LastLine() && !hasExplicitRange(trimmed) {
	}
	switch cmd.cmd {
	case "w", "write":
		if e.exWrite(cmd) && cmd.cmd == "w" {
			return
		}
	case "wq":
		if e.exWrite(cmd) {
			e.quit = true
		}
	case "x", "xit":
		if !e.buf.Modified() || e.exWrite(cmd) {
			e.quit = true
		}
	case "q", "quit":
		e.exQuit(cmd)
	case "qa", "qall", "quita":
		e.exQuitAll(cmd)
	case "wa", "wall":
		e.exWriteAll(cmd)
	case "e", "edit":
		e.exEdit(cmd)
	case "enew":
		e.newBuffer("")
	case "r", "read":
		e.exRead(cmd)
	case "ls", "buffers", "files":
		e.exBuffers()
	case "b", "buffer":
		e.exSwitchBuffer(cmd)
	case "bn", "bnext":
		e.exCycleBuffer(1)
	case "bp", "bprevious":
		e.exCycleBuffer(-1)
	case "bd", "bdelete":
		e.exDeleteBuffer(cmd)
	case "n", "next":
		e.exCycleBuffer(1)
	case "prev", "previous":
		e.exCycleBuffer(-1)
	case "rew", "rewind":
		e.switchToBuffer(0)
	case "set", "se":
		msg, err := e.setOptions(cmd.args)
		if err != nil {
			e.errorf("%v", err)
			return
		}
		if msg != "" {
			e.messagef("%s", msg)
		}
	case "noh", "nohlsearch":
		c := e.cache()
		c.hlOff = true
		c.hlMatches = nil
	case "s", "substitute":
		e.exSubstitute(cmd)
	case "&":
		e.repeatSubstitute()
	case "~":
		e.repeatSubstituteWithLastReplacement()
	case "d", "delete":
		e.exDelete(cmd)
	case "m", "move":
		e.exMove(cmd, false)
	case "t", "co", "copy":
		e.exMove(cmd, true)
	case "pu", "put":
		e.exPut(cmd)
	case "j", "join":
		e.exJoin(cmd)
	case "norm", "normal":
		e.exNormal(cmd)
	case "g", "global":
		e.exGlobal(cmd, false)
	case "v", "vglobal":
		e.exGlobal(cmd, true)
	case "sort":
		e.exSort(cmd)
	case "retab":
		e.exRetab(cmd)
	case "u", "undo":
		n := 1
		if cmd.countSet {
			n = cmd.count
		}
		e.undo(n)
	case "redo":
		e.redo()
	case "pr", "print", "p":
		e.exPrint(cmd)
	case "!":
		e.exShell(cmd)
	case "version":
		e.messagef("vigo 0.1 (vi clone in Go)")
	case "help", "h":
		e.messagef("vigo has no help file yet, see README.md (try :version)")
	case "source", "so":
		e.exSource(cmd.args)
	case "let":
		e.exLet(cmd.args)
	case "marks":
		e.exMarks()
	case "reg", "registers":
		e.exRegisters()
	case "pwd":
		wd, _ := filepath.Abs(".")
		e.messagef("%s", wd)
	case "cd":
		dir := cmd.args
		if dir == "" {
			dir = "~"
		}
		if strings.HasPrefix(dir, "~") {
			home := userHome()
			dir = filepath.Join(home, strings.TrimPrefix(dir, "~"))
		}
		if err := chdir(dir); err != nil {
			e.errorf("E344: Can't find directory in %s", dir)
		}
	case "file":
		if cmd.args == "" {
			e.messagef("%q", e.buf.Path())
			return
		}
		e.buf.SetPath(cmd.args)
		e.setTitle()
	case "syntax":
		if cmd.args == "off" {
			e.opts.Syntax = false
		} else if cmd.args == "on" {
			e.opts.Syntax = true
		}
		e.invalidateCache(e.buf)
	case "sp", "split":
		e.newWindowBuffer(false, cmd.args)
	case "vs", "vsplit":
		e.newWindowBuffer(true, cmd.args)
	case "new":
		e.newWindowBuffer(false, cmd.args)
	case "vnew":
		e.newWindowBuffer(true, cmd.args)
	case "close", "cl":
		e.closeCurrentWindow(false)
	case "only", "on":
		e.onlyWindow(e.win)
	default:
		e.errorf("E492: Not an editor command: %s", cmd.cmd)
	}
}

func hasExplicitRange(line string) bool { return false }

func (e *Editor) gotoLine(line int) {
	e.pushJump(e.cur)
	line = clamp(line, 0, e.buf.LastLine())
	e.setCursor(buffer.Pos{Line: line, Col: e.buf.FirstNonBlank(line).Col})
}

func (e *Editor) exQuit(cmd exCommand) {
	if e.buf.Modified() && !cmd.bang {
		e.errorf("E37: No write since last change (add ! to override)")
		return
	}
	if len(e.windows) > 1 {
		e.closeWindow(e.win)
		e.layoutWindows()
		return
	}
	e.quit = true
}

func (e *Editor) exQuitAll(cmd exCommand) {
	for _, b := range e.buffers {
		if b.Modified() && !cmd.bang {
			e.errorf("E37: No write since last change for buffer %s (add ! to override)", b.Path())
			return
		}
	}
	e.quit = true
}

func (e *Editor) exWrite(cmd exCommand) bool {
	path := cmd.args
	if path == "" {
		path = e.buf.Path()
	}
	if path == "" {
		e.errorf("E32: No file name")
		return false
	}
	if strings.HasPrefix(path, ">>") {
		name := strings.TrimSpace(path[2:])
		if e.appendFile(name, cmd) {
			return true
		}
		return false
	}
	if cmd.rangeSet {
		lines := strings.Join(e.lineStrings(cmd.from, cmd.to), "\n") + "\n"
		if err := writeFileRange(path, lines, cmd.bang); err != nil {
			e.errorf("%v", err)
			return false
		}
		e.messagef("\"%s\" %d lines written", path, cmd.to-cmd.from+1)
		return true
	}
	if err := e.writeBuffer(e.buf, path, cmd.bang); err != nil {
		e.errorf("%v", err)
		return false
	}
	e.messagef("\"%s\" %dL, %dB written", path, e.buf.Len(), len(e.buf.Bytes()))
	return true
}

func (e *Editor) exWriteAll(cmd exCommand) {
	for _, b := range e.buffers {
		if !b.Modified() {
			continue
		}
		if b.Path() == "" {
			e.errorf("E32: No file name for buffer %d", e.bufferIndex(b)+1)
			return
		}
		if err := e.writeBuffer(b, b.Path(), cmd.bang); err != nil {
			e.errorf("%v", err)
			return
		}
	}
}

func (e *Editor) exEdit(cmd exCommand) {
	if cmd.args == "" {
		if e.buf.Modified() && !cmd.bang {
			e.errorf("E37: No write since last change (add ! to override)")
			return
		}
		if err := e.reloadBuffer(e.buf); err != nil {
			e.errorf("%v", err)
		}
		return
	}
	if e.buf.Modified() && !cmd.bang {
		e.errorf("E37: No write since last change (add ! to override)")
		return
	}
	name := expandPath(cmd.args)
	if err := e.openFileIn(e.buf, name, true); err != nil {
		e.errorf("E484: Can't open file %s", name)
		return
	}
	e.setFileType(guessFileType(name))
	e.setTitle()
	e.messagef("\"%s\" %dL, %dB", name, e.buf.Len(), len(e.buf.Bytes()))
}

func (e *Editor) exRead(cmd exCommand) {
	if cmd.args == "" {
		e.errorf("E32: No file name")
		return
	}
	name := expandPath(cmd.args)
	var (
		data []byte
		err  error
	)
	if strings.HasPrefix(cmd.args, "!") {
		data, err = e.captureShell(strings.TrimSpace(cmd.args[1:]))
		name = "!"
	} else {
		data, err = os.ReadFile(name)
	}
	if err != nil {
		e.errorf("E484: Can't open file %s", name)
		return
	}
	e.buf.Checkpoint(e.cur)
	lines := splitLines(string(data))
	at := e.cur.Line + 1
	e.buf.InsertLines(at, lines)
	e.setCursor(buffer.Pos{Line: clamp(at, 0, e.buf.LastLine()), Col: 0})
	e.messagef("\"%s\" %d lines read", name, len(lines))
}

func (e *Editor) exBuffers() {
	var sb strings.Builder
	for i, b := range e.buffers {
		marker := " "
		if i == e.bufIdx {
			marker = "%"
		} else if b.Modified() {
			marker = "+"
		}
		name := b.Path()
		if name == "" {
			name = "[No Name]"
		}
		fmt.Fprintf(&sb, "%s%3d  %-10s line %d\n", marker, i+1, name, e.cursorFor(b).Line+1)
	}
	e.messagef("%s", strings.TrimRight(sb.String(), "\n"))
}

func (e *Editor) exSwitchBuffer(cmd exCommand) {
	if cmd.args == "" {
		e.exBuffers()
		return
	}
	if n, err := strconv.Atoi(cmd.args); err == nil {
		e.switchToBuffer(n - 1)
		return
	}
	for i, b := range e.buffers {
		if filepath.Base(b.Path()) == cmd.args || b.Path() == cmd.args {
			e.switchToBuffer(i)
			return
		}
	}
	e.errorf("E93: No buffer %s", cmd.args)
}

func (e *Editor) exDeleteBuffer(cmd exCommand) {
	idx := e.bufIdx
	if cmd.args != "" {
		if n, err := strconv.Atoi(cmd.args); err == nil {
			idx = n - 1
		}
	}
	if idx < 0 || idx >= len(e.buffers) {
		e.errorf("E93: No such buffer")
		return
	}
	b := e.buffers[idx]
	if b.Modified() && !cmd.bang {
		e.errorf("E89: No write since last change (add ! to override)")
		return
	}
	if len(e.buffers) == 1 {
		e.buffers = []*buffer.Buffer{e.freshBuffer()}
		e.bufIdx = 0
		e.bindBuffer(e.buffers[0])
		return
	}
	e.buffers = append(e.buffers[:idx], e.buffers[idx+1:]...)
	if e.bufIdx >= len(e.buffers) {
		e.bufIdx = len(e.buffers) - 1
	}
	// Windows showing the deleted buffer move to another one.
	fallback := e.buffers[0]
	for _, w := range e.windows {
		if w.buf == b {
			w.buf = fallback
			w.topline, w.topSeg, w.leftcol = 0, 0, 0
			w.cur = buffer.Pos{}
		}
	}
	e.bindBuffer(e.buffers[e.bufIdx])
}

func (e *Editor) exDelete(cmd exCommand) {
	from, to := e.rangeOrCurrent(cmd)
	e.buf.Checkpoint(e.cur)
	lines := e.buf.DeleteLines(from, to)
	texts := make([]string, len(lines))
	for i, l := range lines {
		texts[i] = string(l)
	}
	e.storeRegister(e.defaultRegister(), LineRegister(texts), false, false)
	e.setCursor(buffer.Pos{Line: clamp(from, 0, e.buf.LastLine()), Col: e.buf.FirstNonBlank(clamp(from, 0, e.buf.LastLine())).Col})
}

func (e *Editor) rangeOrCurrent(cmd exCommand) (int, int) {
	if cmd.rangeSet {
		return clamp(cmd.from, 0, e.buf.LastLine()), clamp(cmd.to, 0, e.buf.LastLine())
	}
	switch cmd.cmd {
	case "sort", "g", "global", "v", "vglobal", "print", "p", "pr":
		return 0, e.buf.LastLine()
	}
	return e.cur.Line, e.cur.Line
}

// exMove implements :m and :t. The lines move to after the given address, and
// the address 0 means "before the first line", like in vim.
func (e *Editor) exMove(cmd exCommand, copyOnly bool) {
	args := strings.TrimSpace(cmd.args)
	if args == "" {
		e.errorf("E14: Invalid address")
		return
	}
	beforeFirst := strings.HasPrefix(args, "0")
	dest, _, ok, err := e.parseAddress(args)
	if err != nil || !ok {
		e.errorf("E14: Invalid address")
		return
	}
	from, to := e.rangeOrCurrent(cmd)
	count := to - from + 1
	e.buf.Checkpoint(e.cur)
	lines := make([][]rune, 0, count)
	for i := from; i <= to; i++ {
		lines = append(lines, append([]rune{}, e.buf.Line(i)...))
	}
	at := dest + 1
	switch {
	case beforeFirst:
		at = 0
	case copyOnly:
	default:
		if dest >= to {
			at = dest - count + 1
		}
	}
	if copyOnly {
		e.buf.InsertLines(clamp(at, 0, e.buf.Len()), lines)
		e.setCursor(buffer.Pos{Line: clamp(at, 0, e.buf.LastLine()), Col: 0})
		return
	}
	e.buf.DeleteLines(from, to)
	e.buf.InsertLines(clamp(at, 0, e.buf.Len()), lines)
	e.setCursor(buffer.Pos{Line: clamp(at, 0, e.buf.LastLine()), Col: 0})
}

func (e *Editor) exPut(cmd exCommand) {
	reg := e.defaultRegister()
	if cmd.args != "" {
		reg = rune(cmd.args[0])
	}
	e.setCursor(buffer.Pos{Line: e.rangeOrCurrentLine(cmd), Col: 0})
	saved := e.defaultRegister()
	_ = saved
	e.pending.regSet = true
	e.pending.reg = reg
	e.paste(1, false)
	e.pending.regSet = false
}

func (e *Editor) rangeOrCurrentLine(cmd exCommand) int {
	if cmd.rangeSet {
		return clamp(cmd.to, 0, e.buf.LastLine())
	}
	return e.cur.Line
}

func (e *Editor) exJoin(cmd exCommand) {
	from, to := e.rangeOrCurrent(cmd)
	e.buf.Checkpoint(e.cur)
	for i := from; i < to; i++ {
		e.buf.JoinLines(from, true)
	}
	e.setCursor(buffer.Pos{Line: from, Col: e.buf.FirstNonBlank(from).Col})
}

func (e *Editor) exNormal(cmd exCommand) {
	ks, err := keys.Parse(cmd.args)
	if err != nil {
		e.errorf("%v", err)
		return
	}
	if cmd.bang {
		e.feedKeys(ks, false)
		return
	}
	e.feedKeysMapped(ks)
}

func (e *Editor) exGlobal(cmd exCommand, invert bool) {
	args := cmd.args
	if args == "" {
		e.errorf("E35: No previous regular expression")
		return
	}
	delim := args[0]
	end := strings.IndexByte(args[1:], delim)
	pat := ""
	sub := ""
	if end < 0 {
		pat = args[1:]
	} else {
		pat = args[1 : 1+end]
		sub = strings.TrimLeft(args[2+end:], " \t")
	}
	if pat == "" {
		pat = e.lastSearch
	}
	if pat == "" {
		e.errorf("E35: No previous regular expression")
		return
	}
	re, err := e.compileSearch(pat)
	if err != nil {
		e.errorf("E383: Invalid pattern: %v", err)
		return
	}
	from, to := 0, e.buf.LastLine()
	if cmd.rangeSet {
		from, to = clamp(cmd.from, 0, e.buf.LastLine()), clamp(cmd.to, 0, e.buf.LastLine())
	}
	var targets []int
	for i := from; i <= to; i++ {
		m := re.MatchString(e.buf.LineString(i))
		if m != invert {
			targets = append(targets, i)
		}
	}
	if sub == "" {
		for _, t := range targets {
			e.messagef("%d:%s", t+1, e.buf.LineString(t))
		}
		return
	}
	if invert || substFromBottom(sub) {
		targets = reverseInts(targets)
	}
	for _, t := range targets {
		if t > e.buf.LastLine() {
			continue
		}
		e.execEx(fmt.Sprintf("%d%s", t+1, sub))
	}
}

func reverseInts(in []int) []int {
	out := make([]int, len(in))
	for i, v := range in {
		out[len(in)-1-i] = v
	}
	return out
}

// substFromBottom reports whether the :g sub command must run from the last
// matching line upwards, which is what :g/pat/d and :g/pat/m need.
func substFromBottom(sub string) bool {
	s := strings.TrimLeft(sub, " \t")
	if s == "" {
		return false
	}
	switch {
	case strings.HasPrefix(s, "d"), strings.HasPrefix(s, "m"), strings.HasPrefix(s, "t"):
		return true
	}
	return false
}

func (e *Editor) exSort(cmd exCommand) {
	from, to := e.rangeOrCurrent(cmd)
	lines := e.lineStrings(from, to)
	sort.Strings(lines)
	e.buf.Checkpoint(e.cur)
	for i, l := range lines {
		e.buf.SetLine(from+i, []rune(l))
	}
}

func (e *Editor) exPrint(cmd exCommand) {
	from, to := e.rangeOrCurrent(cmd)
	for i := from; i <= to && i <= e.buf.LastLine(); i++ {
		e.messagef("%s", e.buf.LineString(i))
	}
}

func (e *Editor) exShell(cmd exCommand) {
	command := cmd.args
	if command == "" {
		e.errorf("E34: No previous command")
		return
	}
	if cmd.rangeSet {
		e.filterRange(rangeInfo{
			from: buffer.Pos{Line: cmd.from},
			to:   buffer.Pos{Line: cmd.to + 1},
			kind: mLine,
		}, command)
		return
	}
	_, err := e.runShell(command)
	if err != nil {
		e.errorf("%v", err)
		return
	}
	e.messagef("Press ENTER or type command to continue")
}

// exRetab replaces tabs with spaces (or leading spaces with tabs) in a range.
func (e *Editor) exRetab(cmd exCommand) {
	from, to := e.rangeOrCurrent(cmd)
	ts := e.tabStop()
	e.buf.Checkpoint(e.cur)
	for i := from; i <= to && i <= e.buf.LastLine(); i++ {
		line := append([]rune{}, e.buf.Line(i)...)
		var out []rune
		if e.opts.ExpandTab {
			col := 0
			for _, r := range line {
				if r == '\t' {
					n := ts - col%ts
					out = append(out, []rune(strings.Repeat(" ", n))...)
					col += n
					continue
				}
				out = append(out, r)
				col++
			}
		} else {
			col := 0
			spaces := 0
			i2 := 0
			for i2 < len(line) && (line[i2] == ' ' || line[i2] == '\t') {
				if line[i2] == '\t' || col%ts == ts-1 {
					out = append(out, '\t')
					col += ts - col%ts
					spaces = 0
				} else {
					spaces++
					col++
				}
				i2++
			}
			out = append(out, []rune(strings.Repeat(" ", spaces))...)
			out = append(out, line[i2:]...)
		}
		e.buf.SetLine(i, out)
	}
}

func (e *Editor) exMarks() {
	var sb strings.Builder
	sb.WriteString("mark line  col file/text\n")
	names := make([]rune, 0, len(e.marks))
	for name := range e.marks {
		names = append(names, name)
	}
	sort.Slice(names, func(i, j int) bool { return names[i] < names[j] })
	for _, name := range names {
		p := e.marks[name]
		fmt.Fprintf(&sb, " %c   %5d %4d  %s\n", name, p.Line+1, p.Col, strings.TrimSpace(e.buf.LineString(p.Line)))
	}
	e.messagef("%s", strings.TrimRight(sb.String(), "\n"))
}

func (e *Editor) exRegisters() {
	var sb strings.Builder
	sb.WriteString("--- Registers ---\n")
	names := make([]rune, 0, len(e.regs))
	for name := range e.regs {
		names = append(names, name)
	}
	sort.Slice(names, func(i, j int) bool { return names[i] < names[j] })
	for _, name := range names {
		text := e.regs[name].Text()
		text = strings.ReplaceAll(text, "\n", "^J")
		fmt.Fprintf(&sb, "\"%c   %s\n", name, text)
	}
	e.messagef("%s", strings.TrimRight(sb.String(), "\n"))
}

func (e *Editor) runShell(command string) (string, error) {
	e.restoreTerminal()
	defer e.reenterTerminal()
	shell, shellArgs := shellCommandLine(command)
	c := exec.Command(shell, shellArgs...)
	c.Stdin = e.term.In
	c.Stdout = e.term.Out
	c.Stderr = e.term.Out
	return "", c.Run()
}

// captureShell runs a command and returns its output, used by :r !cmd.
func (e *Editor) captureShell(command string) ([]byte, error) {
	e.restoreTerminal()
	defer e.reenterTerminal()
	shell, shellArgs := shellCommandLine(command)
	c := exec.Command(shell, shellArgs...)
	c.Stdin = e.term.In
	out, err := c.Output()
	return out, err
}

func (e *Editor) restoreTerminal() {
	e.term.Suspend()
}

func (e *Editor) reenterTerminal() {
	e.term.Resume()
	e.scr.Resize(e.term.Size())
	e.forceRedraw = true
}

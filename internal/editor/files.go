package editor

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"vigo/internal/buffer"
)

// loadArgs implements the command line arguments: files plus +cmd and -c cmd.
func (e *Editor) loadArgs(args []string) error {
	var cmds []string
	var files []string
	rcPath := ""
	noRC := false
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--":
			files = append(files, args[i+1:]...)
			i = len(args)
		case arg == "-c":
			if i+1 < len(args) {
				i++
				cmds = append(cmds, args[i])
			}
		case strings.HasPrefix(arg, "+"):
			rest := arg[1:]
			if rest == "" {
				cmds = append(cmds, "$")
			} else {
				cmds = append(cmds, rest)
			}
		case arg == "-u":
			if i+1 < len(args) {
				i++
				if strings.EqualFold(args[i], "NONE") {
					noRC = true
				} else {
					rcPath = args[i]
				}
			}
		case strings.HasPrefix(arg, "-u") && len(arg) > 2:
			if strings.EqualFold(arg[2:], "NONE") {
				noRC = true
			} else {
				rcPath = arg[2:]
			}
		case arg == "--norc", arg == "-n":
			noRC = true
		case strings.HasPrefix(arg, "-") && len(arg) > 1:
			e.errorf("E-unknown option: %s", arg)
		default:
			files = append(files, arg)
		}
	}
	e.buffers = nil
	for _, f := range files {
		b := e.freshBuffer()
		name := expandPath(f)
		if err := e.openFileIn(b, name, true); err != nil {
			return err
		}
		e.buffers = append(e.buffers, b)
		if len(e.buffers) == 1 {
		}
	}
	if len(e.buffers) == 0 {
		e.buffers = []*buffer.Buffer{e.freshBuffer()}
	}
	if !noRC {
		e.loadRC(rcPath)
	}
	e.bufIdx = 0
	e.bindBuffer(e.buffers[0])
	if len(files) > 0 && e.buf.Path() != "" {
		e.messagef("\"%s\" %dL, %dB", e.buf.Path(), e.buf.Len(), len(e.buf.Bytes()))
	}
	if e.startupReport != "" {
		e.messagef("%s", e.startupReport)
		e.startupReport = ""
	}
	e.setFileType(guessFileType(e.buf.Path()))
	for _, c := range cmds {
		e.execEx(c)
	}
	return nil
}

// openFileIn loads a file into an existing buffer.
func (e *Editor) openFileIn(b *buffer.Buffer, path string, force bool) error {
	if path == "-" {
		data, err := readAll(os.Stdin)
		if err != nil {
			return err
		}
		b.SetContents(data)
		b.SetPath("")
		return nil
	}
	if path == "" {
		return nil
	}
	info, err := os.Stat(path)
	if err == nil && info.IsDir() {
		return fmt.Errorf("E502: %s is a directory", path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// A new file is fine; it just does not exist yet.
			b.SetContents(nil)
			b.SetPath(path)
			b.ClearUndo()
			b.SetEOL(defaultEOL())
			b.SetMissingEOL(true)
			return nil
		}
		return err
	}
	b.SetContents(data)
	b.SetPath(path)
	b.ClearUndo()
	e.rememberModTime(b, info)
	return nil
}

// newBuffer makes the given file the buffer in the only window.
func (e *Editor) newBuffer(path string) {
	b := e.freshBuffer()
	if path != "" {
		if err := e.openFileIn(b, expandPath(path), true); err != nil {
			e.errorf("%v", err)
			return
		}
	}
	e.buffers = append(e.buffers, b)
	e.bufIdx = len(e.buffers) - 1
	e.bindBuffer(b)
}

// newWindowBuffer opens path in a new split window.
func (e *Editor) newWindowBuffer(vertical bool, path string) {
	b := e.buf
	if path != "" {
		b = e.freshBuffer()
		if err := e.openFileIn(b, expandPath(path), true); err != nil {
			e.errorf("%v", err)
			return
		}
		e.buffers = append(e.buffers, b)
	}
	nw := e.splitWindow(vertical, b)
	if nw != nil {
		e.layoutWindows()
	}
}

// bindBuffer attaches a buffer to the current window.
func (e *Editor) bindBuffer(b *buffer.Buffer) {
	if e.buf != nil && e.buf != b {
		e.saveCursor(e.buf)
	}
	if e.win != nil {
		e.win.buf = b
	}
	e.buf = b
	e.cur = e.cursorFor(b)
	e.desiredCol = e.cur.Col
	e.topline = 0
	e.topSeg = 0
	e.leftcol = 0
	b.UndoLimit = e.opts.UndoLevels
	e.setTitle()
}

func (e *Editor) saveCursor(b *buffer.Buffer) {
	if e.cursors == nil {
		e.cursors = map[*buffer.Buffer]buffer.Pos{}
	}
	e.cursors[b] = e.cur
}

func (e *Editor) cursorFor(b *buffer.Buffer) buffer.Pos {
	if e.cursors == nil {
		return buffer.Pos{}
	}
	if p, ok := e.cursors[b]; ok {
		return b.ClampNormal(p)
	}
	return buffer.Pos{}
}

func (e *Editor) bufferIndex(b *buffer.Buffer) int {
	for i, x := range e.buffers {
		if x == b {
			return i
		}
	}
	return 0
}

func (e *Editor) switchToBuffer(i int) {
	if i < 0 || i >= len(e.buffers) {
		e.errorf("E93: No such buffer")
		return
	}
	e.saveCursor(e.buf)
	e.bufIdx = i
	e.bindBuffer(e.buffers[i])
	name := e.buf.Path()
	if name == "" {
		name = "[No Name]"
	}
	e.messagef("%d \"%s\"", i+1, name)
}

func (e *Editor) exCycleBuffer(dir int) {
	if len(e.buffers) <= 1 {
		e.errorf("E85: There is no listed buffer")
		return
	}
	i := e.bufIdx + dir
	if i < 0 {
		i = len(e.buffers) - 1
	}
	if i >= len(e.buffers) {
		i = 0
	}
	e.switchToBuffer(i)
}

func (e *Editor) rememberModTime(b *buffer.Buffer, info os.FileInfo) {
	if info == nil {
		return
	}
	if e.modTimes == nil {
		e.modTimes = map[*buffer.Buffer]time.Time{}
	}
	e.modTimes[b] = info.ModTime()
}

// writeBuffer writes a buffer to disk, refusing when the file changed under us.
func (e *Editor) writeBuffer(b *buffer.Buffer, path string, force bool) error {
	if b.ReadOnly() && !force {
		return fmt.Errorf("E45: 'readonly' option is set (add ! to override)")
	}
	if !force {
		if info, err := os.Stat(path); err == nil {
			if prev, ok := e.modTimes[b]; ok && !info.ModTime().Equal(prev) {
				return fmt.Errorf("E13: File exists (add ! to override)")
			}
		}
	}
	data := b.Bytes()
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("E212: Can't open file for writing: %v", err)
	}
	b.SetPath(path)
	b.MarkSaved()
	if info, err := os.Stat(path); err == nil {
		e.rememberModTime(b, info)
	}
	e.setTitle()
	return nil
}

func (e *Editor) appendFile(path string, cmd exCommand) bool {
	if path == "" {
		e.errorf("E32: No file name")
		return false
	}
	var content string
	if cmd.rangeSet {
		content = strings.Join(e.lineStrings(cmd.from, cmd.to), "\n") + "\n"
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		e.errorf("E212: Can't open file for writing")
		return false
	}
	defer f.Close()
	if _, err := f.WriteString(content); err != nil {
		e.errorf("E212: write failed")
		return false
	}
	e.messagef("appended")
	return true
}

func writeFileRange(path, content string, force bool) error {
	if !force {
		if _, err := os.Stat(path); err == nil {
			return fmt.Errorf("E13: File exists (add ! to override)")
		}
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("E212: Can't open file for writing: %v", err)
	}
	return nil
}

// reloadBuffer reads the buffer's file again from disk.
func (e *Editor) reloadBuffer(b *buffer.Buffer) error {
	if b.Path() == "" {
		return fmt.Errorf("E32: No file name")
	}
	data, err := os.ReadFile(b.Path())
	if err != nil {
		return fmt.Errorf("E484: Can't open file %s", b.Path())
	}
	b.SetContents(data)
	b.ClearUndo()
	e.invalidateCache(b)
	e.setCursor(buffer.Pos{})
	e.messagef("\"%s\" %dL, %dB", b.Path(), b.Len(), len(b.Bytes()))
	return nil
}

func expandPath(p string) string {
	if strings.HasPrefix(p, "~") {
		home := userHome()
		if p == "~" {
			return home
		}
		if strings.HasPrefix(p, "~/") {
			return filepath.Join(home, p[2:])
		}
	}
	return p
}

func userHome() string {
	if h, err := os.UserHomeDir(); err == nil {
		return h
	}
	return os.Getenv("HOME")
}

func chdir(dir string) error { return os.Chdir(dir) }

// freshBuffer creates an empty buffer using the platform's line ending.
func (e *Editor) freshBuffer() *buffer.Buffer {
	b := buffer.New()
	b.SetDefaultEOL(defaultEOL())
	return b
}

// Package editor implements the interactive editor: modes, motions, operators
// and the ex command line.
package editor

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"vigo/internal/buffer"
	"vigo/internal/keys"
	"vigo/internal/screen"
)

// Mode is the current editing mode.
type Mode int

// Editor modes.
const (
	ModeNormal Mode = iota
	ModeInsert
	ModeReplace
	ModeVisual
	ModeVisualLine
	ModeVisualBlock
	ModeCmdline
	ModeSearchForward
	ModeSearchBackward
)

// String names the mode as vim does in the showmode area.
func (m Mode) String() string {
	switch m {
	case ModeNormal:
		return ""
	case ModeInsert:
		return "-- INSERT --"
	case ModeReplace:
		return "-- REPLACE --"
	case ModeVisual:
		return "-- VISUAL --"
	case ModeVisualLine:
		return "-- VISUAL LINE --"
	case ModeVisualBlock:
		return "-- VISUAL BLOCK --"
	}
	return ""
}

func (m Mode) visual() bool {
	return m == ModeVisual || m == ModeVisualLine || m == ModeVisualBlock
}

// pendingState is the partially typed command.
type pendingState struct {
	count    int
	countSet bool
	op       string
	opCount  int
	reg      rune
	regSet   bool
	awaiting string
	awaitKey keys.Key
	keys     []keys.Key
	version  uint64
}

func (p *pendingState) busy() bool {
	return p.op != "" || p.awaiting != "" || p.countSet || p.regSet
}

func (p *pendingState) reset() { *p = pendingState{} }

// Editor is the whole interactive state.
type Editor struct {
	term    *screen.Terminal
	scr     *screen.Screen
	buf     *buffer.Buffer
	buffers []*buffer.Buffer
	bufIdx  int

	cur           buffer.Pos
	desiredCol    int
	lastInsertPos buffer.Pos
	mode          Mode

	visualStart buffer.Pos
	blockCol    int

	blockInsertActive bool
	blockInsertTop    buffer.Pos
	blockInsertBot    buffer.Pos
	blockTextTyped    []rune

	regs    map[rune]Register
	opts    Options
	pending pendingState

	msg      string
	msgError bool

	cmd cmdlineState

	topline int
	topSeg  int
	leftcol int

	jumps   []buffer.Pos
	jumpIdx int

	marks map[rune]buffer.Pos

	lastSearch       string
	lastSearchDir    int
	lastSubst        string
	lastSubstRep     string
	lastSubstFlags   string
	lastFind         findState
	lastChange       []keys.Key
	hasLastChange    bool
	lastChangeInsert bool
	lastVisual       [2]buffer.Pos
	hasLastVisual    bool

	recording bool
	recordReg rune
	lastMacro rune
	macros    map[rune][]keys.Key
	replaying bool

	completeActive     bool
	completeBase       int
	completeIdx        int
	completeWordPrefix string

	keyCh  chan keys.Key
	keyErr error

	caches    map[*buffer.Buffer]*bufCache
	fileTypes map[*buffer.Buffer]string

	root       *winNode
	win        *window
	windows    []*window
	separators []sepCol
	showStatus bool

	changeList []buffer.Pos
	changeIdx  int
	blockEOL   bool
	// inserted accumulates the text of the current insert session for register
	// "." without copying a string per keystroke.
	inserted []rune
	// replaceStack remembers the characters overwritten in replace mode so that
	// backspace can put them back, like vim does.
	replaceStack []rune

	maps          map[byte][]*mapping
	mapBuf        []keys.Key
	mapPending    bool
	mapDepth      int
	leader        string
	keyReader     *keys.Reader
	scriptDepth   int
	scriptErrors  []string
	startupReport string
	cmdHistory    []string
	searchHistory []string
	cursors       map[*buffer.Buffer]buffer.Pos
	modTimes      map[*buffer.Buffer]time.Time

	cursorX, cursorY int
	cursorShape      screen.CursorShape

	quit        bool
	exitCode    int
	showPending string

	rows        []viewRow
	textWidth   int
	gutterW     int
	forceRedraw bool
	theme       theme

	segCache        map[int][]seg
	segCacheVersion uint64
	segCacheWidth   int
	segCacheWrap    bool
}

// New creates an editor bound to a terminal.
func New(term *screen.Terminal) *Editor {
	w, h := term.Size()
	ed := &Editor{
		term:   term,
		scr:    screen.New(w, h, term.Out),
		buf:    newEmptyBuffer(),
		regs:   map[rune]Register{},
		macros: map[rune][]keys.Key{},
		marks:  map[rune]buffer.Pos{},
		opts:   DefaultOptions(),
	}
	ed.scr.SetTrueColor(trueColorSupported())
	ed.buf.UndoLimit = ed.opts.UndoLevels
	ed.buffers = []*buffer.Buffer{ed.buf}
	ed.setFileType("")
	ed.theme = makeTheme("dark")
	ed.initWindows(ed.buf)
	return ed
}

// newEmptyBuffer is used before the editor exists.
func newEmptyBuffer() *buffer.Buffer {
	b := buffer.New()
	b.SetDefaultEOL(defaultEOL())
	return b
}

// Run starts the interactive loop and returns the process exit code.
func Run(args []string) int {
	term, err := screen.OpenTerminal()
	if err != nil {
		fmt.Fprintf(os.Stderr, "vigo: %v\n", err)
		return 1
	}
	restored := false
	restore := func() {
		if !restored {
			restored = true
			term.Restore()
		}
	}
	defer restore()

	ed := New(term)
	if err := ed.loadArgs(args); err != nil {
		restore()
		fmt.Fprintf(os.Stderr, "vigo: %v\n", err)
		return 1
	}

	rd := keys.NewReader(term.In)
	ed.keyReader = rd
	ed.keyCh = make(chan keys.Key, 256)
	go func() {
		defer close(ed.keyCh)
		for {
			k, err := rd.ReadKey()
			if err != nil {
				ed.keyErr = err
				return
			}
			ed.keyCh <- k
		}
	}()

	sigs := make(chan os.Signal, 8)
	watchSignals(sigs)
	defer signal.Stop(sigs)

	var resizeTick <-chan time.Time
	if pollResize {
		ticker := time.NewTicker(300 * time.Millisecond)
		defer ticker.Stop()
		resizeTick = ticker.C
	}

	ed.setTitle()
	if err := term.SetupWarning(); err != nil && ed.startupReport == "" {
		ed.startupReport = fmt.Sprintf("terminal setup: %v", err)
	}
	if ed.startupReport == "" && !term.VTInput() {
		ed.startupReport = "console has no virtual terminal input; special keys may not work"
	}
	if ed.startupReport != "" {
		ed.messagef("%s", ed.startupReport)
		ed.startupReport = ""
	}
	for !ed.quit {
		ed.draw()
		var (
			timeout <-chan time.Time
			timer   *time.Timer
		)
		if ed.mapPending {
			timer = time.NewTimer(ed.mappingTimeout())
			timeout = timer.C
		}
		select {
		case k, ok := <-ed.keyCh:
			if timer != nil {
				timer.Stop()
			}
			if !ok {
				ed.quit = true
				break
			}
			ed.feedKey(k)
		case s := <-sigs:
			if timer != nil {
				timer.Stop()
			}
			if isResize(s) {
				ed.resize()
			} else {
				ed.quit = true
			}
		case <-resizeTick:
			if w, h := ed.term.Size(); w != canvasW(ed) || h != canvasH(ed) {
				ed.resize()
			}
		case <-timeout:
			ed.flushMappings()
		}
	}
	restore()
	return ed.exitCode
}

// mappingTimeout is how long to wait for the rest of a mapping prefix.
func (e *Editor) mappingTimeout() time.Duration {
	ms := e.opts.TimeoutLen
	if ms <= 0 {
		ms = 1000
	}
	return time.Duration(ms) * time.Millisecond
}

// feedKeysMapped plays keys through the mapping machinery, which is what
// :normal (without !) and the command line do.
func (e *Editor) feedKeysMapped(ks []keys.Key) {
	prev := e.replaying
	e.replaying = true
	for _, k := range ks {
		e.feedKey(k)
		if e.quit {
			break
		}
	}
	e.replaying = prev
	if e.mapPending {
		e.flushMappings()
	}
}

func (e *Editor) setTitle() {
	if !e.opts.Title || e.term == nil {
		return
	}
	name := e.buf.Path()
	if name == "" {
		name = "[No Name]"
	}
	e.term.SetTitle("vigo - " + filepath.Base(name))
}

// resize reacts to a terminal size change.
func (e *Editor) resize() {
	w, h := e.term.Size()
	e.scr.Resize(w, h)
	e.forceRedraw = true
	e.ensureCursorVisible()
}

// handleKey dispatches one key event.
func (e *Editor) handleKey(k keys.Key) {
	if k.Paste {
		e.handlePasteKeys([]keys.Key{k})
		return
	}
	if !e.replaying {
		e.noteKey(k)
	}
	e.clearMessage()
	switch e.mode {
	case ModeNormal:
		e.normalKey(k)
	case ModeInsert, ModeReplace:
		e.insertKey(k)
	case ModeVisual, ModeVisualLine, ModeVisualBlock:
		e.visualKey(k)
	case ModeCmdline, ModeSearchForward, ModeSearchBackward:
		e.cmdlineKey(k)
	}
	e.afterKey()
}

// noteKey records keys for macro recording and for the . command.
func (e *Editor) noteKey(k keys.Key) {
	if e.recording {
		e.macros[e.recordReg] = append(e.macros[e.recordReg], k)
	}
	if e.pending.keys == nil {
		e.pending.keys = []keys.Key{}
		e.pending.version = e.buf.Version()
	}
	e.pending.keys = append(e.pending.keys, k)
}

func (e *Editor) afterKey() {
	if e.mode != ModeNormal || e.pending.busy() {
		return
	}
	if e.pending.keys != nil && e.buf.Version() != e.pending.version {
		e.lastChange = append([]keys.Key{}, e.pending.keys...)
		e.hasLastChange = true
		e.noteChange(e.cur)
	}
	e.pending.reset()
	e.buf.BreakBlock()
}

// abortPending cancels a partially typed command.
func (e *Editor) abortPending() { e.pending.reset() }

// feedKeys replays keys. Recorded keys are normally not re-recorded, but macro
// playback passes record=true so that @@ keeps working.
func (e *Editor) feedKeys(ks []keys.Key, record bool) {
	prevReplay, prevRecord := e.replaying, e.recording
	e.replaying = true
	if !record {
		e.recording = false
	}
	for _, k := range ks {
		e.handleKey(k)
		if e.quit {
			break
		}
	}
	e.replaying = prevReplay
	e.recording = prevRecord
	if e.pending.awaiting != "" || e.pending.op != "" {
		e.pending.reset()
	}
}

// nextKey blocks for the next key event; prompts use it.
func (e *Editor) nextKey() (keys.Key, error) {
	if e.keyCh == nil {
		return keys.Key{}, fmt.Errorf("no input")
	}
	k, ok := <-e.keyCh
	if !ok {
		if e.keyErr != nil {
			return keys.Key{}, e.keyErr
		}
		return keys.Key{}, keys.ErrClosed
	}
	return k, nil
}

// ask shows a prompt on the command line and returns what the user typed.
func (e *Editor) ask(prompt string) (string, bool) {
	var text []rune
	for {
		e.drawPrompt(prompt + string(text))
		k, err := e.nextKey()
		if err != nil {
			e.quit = true
			return "", false
		}
		switch {
		case k.IsSpecial(keys.CR):
			return string(text), true
		case k.IsSpecial(keys.Esc) || (k.IsCtrl() && k.R == 'C'):
			return "", false
		case k.IsSpecial(keys.BS):
			if len(text) > 0 {
				text = text[:len(text)-1]
			}
		case k.IsRune():
			text = append(text, k.R)
		}
	}
}

// confirm shows a single key question, such as the substitute confirm prompt.
func (e *Editor) confirm(prompt string) rune {
	e.drawPrompt(prompt)
	for {
		k, err := e.nextKey()
		if err != nil {
			e.quit = true
			return 'q'
		}
		switch {
		case k.IsSpecial(keys.Esc), k.IsCtrl() && k.R == 'C':
			return 'q'
		case k.IsSpecial(keys.CR):
			return 'y'
		case k.IsRune():
			return unicode.ToLower(k.R)
		}
	}
}

func (e *Editor) drawPrompt(text string) {
	s := e.scr
	w, h := s.Size()
	rect := s.Rect(0, h-1, w, 1)
	rect.FillSpace(0, 0, w, e.theme.cmdline)
	rect.Text(0, 0, w, e.theme.cmdline, text)
	cx := screen.StringWidth(text)
	if cx >= w {
		cx = w - 1
	}
	s.Flush(cx, h-1, true)
}

// pager shows a multi line message and waits for a key press.
func (e *Editor) pager(lines []string) {
	if len(lines) == 0 {
		return
	}
	s := e.scr
	w, h := s.Size()
	shown := lines
	if len(shown) > h-1 {
		shown = shown[:h-1]
	}
	for i, line := range shown {
		rect := s.Rect(0, i, w, 1)
		rect.FillSpace(0, 0, w, e.theme.message)
		rect.Text(0, 0, w, e.theme.message, line)
	}
	rect := s.Rect(0, h-1, w, 1)
	rect.FillSpace(0, 0, w, e.theme.cmdline)
	rect.Text(0, 0, w, e.theme.cmdline, "Press ENTER or type command to continue")
	s.Flush(0, h-1, true)
	if _, err := e.nextKey(); err != nil {
		e.quit = true
		return
	}
	s.Resize(w, h)
	e.forceRedraw = true
}

// messagef shows a message on the last line.
func (e *Editor) messagef(format string, args ...any) {
	if e.scriptDepth > 0 {
		return
	}
	text := fmt.Sprintf(format, args...)
	if strings.Contains(text, "\n") {
		e.pager(strings.Split(strings.TrimRight(text, "\n"), "\n"))
		return
	}
	e.msg = text
	e.msgError = false
}

// errorf shows an error message on the last line.
func (e *Editor) errorf(format string, args ...any) {
	text := fmt.Sprintf(format, args...)
	if e.scriptDepth > 0 {
		if len(e.scriptErrors) < 20 {
			e.scriptErrors = append(e.scriptErrors, text)
		}
		return
	}
	e.msg = text
	e.msgError = true
}

func (e *Editor) clearMessage() {
	e.msg = ""
	e.msgError = false
}

func trueColorSupported() bool {
	ct := os.Getenv("COLORTERM")
	if strings.Contains(ct, "truecolor") || strings.Contains(ct, "24bit") {
		return true
	}
	return strings.Contains(os.Getenv("TERM"), "direct")
}

func (e *Editor) cursorClamp(p buffer.Pos) buffer.Pos {
	switch e.mode {
	case ModeInsert, ModeReplace:
		return e.buf.Clamp(p)
	default:
		return e.buf.ClampNormal(p)
	}
}

// setCursor moves the cursor, respecting the current mode's clamping rules.
func (e *Editor) setCursor(p buffer.Pos) {
	e.cur = e.cursorClamp(p)
	e.desiredCol = e.cur.Col
}

// moveCursor moves the cursor without resetting the desired column.
func (e *Editor) moveCursor(p buffer.Pos) { e.cur = e.cursorClamp(p) }

// setCursorDesired moves the cursor while keeping the desired column, which is
// what vertical motions need.
func (e *Editor) setCursorDesired(p buffer.Pos) {
	want := e.desiredCol
	e.cur = e.cursorClamp(p)
	e.desiredCol = want
}

// noteInsert records where insert mode last started.
func (e *Editor) noteInsert() { e.lastInsertPos = e.cur }

// enterInsert switches to insert mode.
func (e *Editor) enterInsert() {
	e.mode = ModeInsert
	e.pending.awaiting = ""
	e.noteInsert()
}

// enterVisual switches to a visual mode, remembering the anchor.
func (e *Editor) enterVisual(m Mode) {
	e.mode = m
	e.visualStart = e.cur
	e.blockCol = e.cur.Col
	e.blockEOL = false
}

// pushJump adds a position to the jump list.
func (e *Editor) pushJump(p buffer.Pos) {
	if len(e.jumps) == 0 {
		e.jumps = []buffer.Pos{p}
		e.jumpIdx = 0
		return
	}
	if e.jumps[e.jumpIdx] == p {
		return
	}
	if e.jumpIdx+1 < len(e.jumps) {
		e.jumps = e.jumps[:e.jumpIdx+1]
	}
	e.jumps = append(e.jumps, p)
	if len(e.jumps) > 100 {
		e.jumps = e.jumps[len(e.jumps)-100:]
	}
	e.jumpIdx = len(e.jumps) - 1
}

func (e *Editor) jumpOlder() {
	if e.jumpIdx <= 0 {
		e.errorf("E662: At start of jump list")
		return
	}
	e.jumpIdx--
	e.setCursor(e.jumps[e.jumpIdx])
}

func (e *Editor) jumpNewer() {
	if e.jumpIdx >= len(e.jumps)-1 {
		e.errorf("E663: At end of jump list")
		return
	}
	e.jumpIdx++
	e.setCursor(e.jumps[e.jumpIdx])
}

// takeCount returns the count typed before the command and clears it, so that a
// count cannot leak into the next command. The bool says whether the user typed
// one, which gg and G need to distinguish.
func (e *Editor) takeCount() (int, bool) {
	n := 1
	set := false
	if e.pending.countSet && e.pending.count > 0 {
		n = e.pending.count
		set = true
	}
	e.pending.count, e.pending.countSet = 0, false
	return n, set
}

// count returns the count from the pending state, defaulting to one.
func (e *Editor) count() int {
	n, _ := e.takeCount()
	return n
}

func canvasW(e *Editor) int { w, _ := e.scr.Size(); return w }
func canvasH(e *Editor) int { _, h := e.scr.Size(); return h }

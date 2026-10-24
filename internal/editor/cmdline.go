package editor

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	"vigo/internal/buffer"
	"vigo/internal/keys"
)

type cmdlineState struct {
	mode          Mode
	text          []rune
	col           int
	prefix        string
	histIdx       int
	histSaved     string
	searchOrigin  buffer.Pos
	completionIdx int
}

func (e *Editor) startCmdline(mode Mode, prefix string) {
	e.cmd = cmdlineState{
		mode:         mode,
		prefix:       prefix,
		text:         []rune(prefix),
		col:          len([]rune(prefix)),
		histIdx:      -1,
		searchOrigin: e.cur,
	}
	e.mode = mode
}

func (e *Editor) cmdLineText() string { return string(e.cmd.text) }

// cmdPrompt is the prompt character drawn before the command line text.
func (e *Editor) cmdPrompt() string {
	switch e.mode {
	case ModeCmdline:
		return ":"
	}
	return ""
}

// cmdlineKey edits and runs the command line.
func (e *Editor) cmdlineKey(k keys.Key) {
	if k.IsSpecial(keys.Esc) || (k.IsCtrl() && k.R == 'C') {
		e.cmdCancel()
		return
	}
	if k.IsSpecial(keys.CR) {
		e.cmdExecute()
		return
	}
	if k.IsSpecial(keys.BS) {
		if e.cmd.col <= len([]rune(e.cmd.prefix)) && len(e.cmd.text) <= len([]rune(e.cmd.prefix)) {
			e.cmdCancel()
			return
		}
		e.cmdBackspace()
		return
	}
	if k.IsCtrl() {
		switch k.R {
		case 'W':
			e.cmdDeleteWord()
		case 'U':
			e.cmdSetText(e.cmd.prefix)
		case 'P', 'N':
			if k.R == 'P' {
				e.cmdHistoryStep(-1)
			} else {
				e.cmdHistoryStep(1)
			}
		case 'A':
			e.cmd.col = len([]rune(e.cmd.prefix))
		case 'E':
			e.cmd.col = len(e.cmd.text)
		case 'B':
			e.cmdMove(-1)
		case 'F':
			e.cmdMove(1)
		case 'R':
			e.cmdInsertRegister()
		case 'V':
			e.cmdInsertLiteral()
		case 'D':
			e.cmdDeleteToEnd()
		}
		e.previewIfSearch()
		return
	}
	switch k.Name {
	case keys.Left:
		e.cmdMove(-1)
		return
	case keys.Right:
		e.cmdMove(1)
		return
	case keys.Home:
		e.cmd.col = len([]rune(e.cmd.prefix))
		return
	case keys.End:
		e.cmd.col = len(e.cmd.text)
		return
	case keys.Up:
		e.cmdHistoryStep(-1)
		e.previewIfSearch()
		return
	case keys.Down:
		e.cmdHistoryStep(1)
		e.previewIfSearch()
		return
	case keys.Del:
		e.cmdDeleteForward()
		return
	case keys.Tab:
		e.cmdComplete()
		return
	}
	if !k.IsRune() {
		return
	}
	e.cmdInsertRune(k.R)
	e.previewIfSearch()
}

func (e *Editor) cmdCancel() {
	origin := e.cmd.searchOrigin
	mode := e.cmd.mode
	e.cmd = cmdlineState{}
	e.mode = ModeNormal
	e.pending.reset()
	if mode == ModeSearchForward || mode == ModeSearchBackward {
		e.setCursor(origin)
	}
}

func (e *Editor) cmdExecute() {
	text := string(e.cmd.text)
	mode := e.cmd.mode
	prefix := e.cmd.prefix
	origin := e.cmd.searchOrigin
	e.cmd = cmdlineState{}
	e.mode = ModeNormal
	e.pending.reset()
	switch mode {
	case ModeCmdline:
		e.historyAdd(text)
		if strings.TrimSpace(text) == prefix && prefix != "" {
			return
		}
		e.execEx(text)
	case ModeSearchForward:
		e.setCursor(origin)
		pat := strings.TrimPrefix(text, prefix)
		e.execSearch(pat, true)
	case ModeSearchBackward:
		e.setCursor(origin)
		pat := strings.TrimPrefix(text, prefix)
		e.execSearch(pat, false)
	}
}

func (e *Editor) previewIfSearch() {
	if e.cmd.mode != ModeSearchForward && e.cmd.mode != ModeSearchBackward {
		return
	}
	pat := strings.TrimPrefix(string(e.cmd.text), e.cmd.prefix)
	if pat == "" {
		e.setCursor(e.cmd.searchOrigin)
		return
	}
	e.previewSearch(pat)
}

func (e *Editor) cmdInsertRune(r rune) {
	e.cmdSetText(insertRuneAt(string(e.cmd.text), e.cmd.col, r))
	e.cmd.col++
}

func insertRuneAt(s string, at int, r rune) string {
	runes := []rune(s)
	if at < 0 {
		at = 0
	}
	if at > len(runes) {
		at = len(runes)
	}
	out := append([]rune{}, runes[:at]...)
	out = append(out, r)
	out = append(out, runes[at:]...)
	return string(out)
}

func (e *Editor) cmdSetText(s string) {
	e.cmd.text = []rune(s)
	if e.cmd.col > len(e.cmd.text) {
		e.cmd.col = len(e.cmd.text)
	}
}

func (e *Editor) cmdBackspace() {
	if e.cmd.col <= len([]rune(e.cmd.prefix)) {
		return
	}
	runes := e.cmd.text
	at := e.cmd.col - 1
	out := append([]rune{}, runes[:at]...)
	out = append(out, runes[at+1:]...)
	e.cmd.text = out
	e.cmd.col = at
}

func (e *Editor) cmdDeleteForward() {
	if e.cmd.col >= len(e.cmd.text) {
		return
	}
	runes := e.cmd.text
	at := e.cmd.col
	out := append([]rune{}, runes[:at]...)
	out = append(out, runes[at+1:]...)
	e.cmd.text = out
}

func (e *Editor) cmdDeleteToEnd() {
	if e.cmd.col >= len(e.cmd.text) {
		return
	}
	e.cmd.text = append([]rune{}, e.cmd.text[:e.cmd.col]...)
}

func (e *Editor) cmdDeleteWord() {
	runes := e.cmd.text
	at := e.cmd.col
	start := at
	for start > 0 && unicode.IsSpace(runes[start-1]) {
		start--
	}
	for start > 0 && !unicode.IsSpace(runes[start-1]) {
		start--
	}
	if start < len([]rune(e.cmd.prefix)) {
		start = len([]rune(e.cmd.prefix))
	}
	out := append([]rune{}, runes[:start]...)
	out = append(out, runes[at:]...)
	e.cmd.text = out
	e.cmd.col = start
}

func (e *Editor) cmdMove(d int) {
	e.cmd.col += d
	low := len([]rune(e.cmd.prefix))
	if e.cmd.col < low {
		e.cmd.col = low
	}
	if e.cmd.col > len(e.cmd.text) {
		e.cmd.col = len(e.cmd.text)
	}
}

func (e *Editor) cmdHistoryStep(dir int) {
	hist := &e.cmdHistory
	if e.cmd.mode == ModeSearchForward || e.cmd.mode == ModeSearchBackward {
		hist = &e.searchHistory
	}
	h := *hist
	if len(h) == 0 {
		return
	}
	if dir < 0 {
		if e.cmd.histIdx < 0 {
			e.cmd.histSaved = string(e.cmd.text)
			e.cmd.histIdx = len(h) - 1
		} else if e.cmd.histIdx > 0 {
			e.cmd.histIdx--
		}
	} else {
		if e.cmd.histIdx < 0 {
			return
		}
		e.cmd.histIdx++
		if e.cmd.histIdx >= len(h) {
			e.cmd.histIdx = -1
			e.cmdSetText(e.cmd.histSaved)
			e.cmd.col = len(e.cmd.text)
			return
		}
	}
	entry := h[e.cmd.histIdx]
	if e.cmd.mode == ModeSearchForward {
		entry = e.cmd.prefix + entry
	} else if e.cmd.mode == ModeSearchBackward {
		entry = e.cmd.prefix + entry
	}
	e.cmdSetText(entry)
	e.cmd.col = len(e.cmd.text)
}

func (e *Editor) cmdInsertRegister() {
	e.drawPrompt(string(e.cmd.text))
	k, err := e.nextKey()
	if err != nil {
		e.quit = true
		return
	}
	if !k.IsRune() {
		return
	}
	reg := e.register(k.R)
	text := strings.ReplaceAll(reg.Text(), "\n", " ")
	e.cmdSetText(insertTextAtString(string(e.cmd.text), e.cmd.col, text))
	e.cmd.col += len([]rune(text))
}

func (e *Editor) cmdInsertLiteral() {
	k, err := e.nextKey()
	if err != nil {
		e.quit = true
		return
	}
	if k.IsRune() {
		e.cmdInsertRune(k.R)
	} else if k.IsSpecial(keys.CR) {
		e.cmdInsertRune('\n')
	}
}

func insertTextAtString(s string, at int, text string) string {
	runes := []rune(s)
	if at > len(runes) {
		at = len(runes)
	}
	out := append([]rune{}, runes[:at]...)
	out = append(out, []rune(text)...)
	out = append(out, runes[at:]...)
	return string(out)
}

// cmdComplete performs context sensitive completion of the command line.
func (e *Editor) cmdComplete() {
	text := string(e.cmd.text)
	fields := strings.Fields(text)
	cur := ""
	if len(text) > 0 && !unicode.IsSpace(rune(text[len(text)-1])) && len(fields) > 0 {
		cur = fields[len(fields)-1]
	}
	var candidates []string
	switch {
	case e.cmd.mode == ModeSearchForward || e.cmd.mode == ModeSearchBackward:
		return
	case strings.HasPrefix(text, "set ") || strings.HasPrefix(text, "se "):
		candidates = optionNames()
	case len(fields) > 0 && (fields[0] == "e" || fields[0] == "edit" || fields[0] == "w" ||
		fields[0] == "write" || fields[0] == "r" || fields[0] == "read" || fields[0] == "sp" || fields[0] == "vs"):
		candidates = fileCompletions(cur)
	default:
		candidates = exCommandNames()
	}
	if len(candidates) == 0 {
		return
	}
	if len(fields) > 1 && cur != "" {
		candidates = filterPrefix(candidates, cur)
	} else if len(fields) <= 1 {
		candidates = filterPrefix(candidates, text)
	}
	if len(candidates) == 0 {
		return
	}
	sort.Strings(candidates)
	chosen := candidates[0]
	if len(candidates) > 1 {
		chosen = commonPrefix(candidates)
		if chosen == "" {
			chosen = candidates[0]
		}
	}
	replaced := replaceLastWord(text, chosen)
	e.cmdSetText(replaced)
	e.cmd.col = len(e.cmd.text)
}

func filterPrefix(list []string, prefix string) []string {
	var out []string
	for _, s := range list {
		if strings.HasPrefix(s, prefix) {
			out = append(out, s)
		}
	}
	return out
}

func commonPrefix(list []string) string {
	if len(list) == 0 {
		return ""
	}
	p := list[0]
	for _, s := range list[1:] {
		for !strings.HasPrefix(s, p) {
			if len(p) == 0 {
				return ""
			}
			p = p[:len(p)-1]
		}
	}
	return p
}

func replaceLastWord(text, word string) string {
	i := len(text)
	for i > 0 && !unicode.IsSpace(rune(text[i-1])) {
		i--
	}
	return text[:i] + word
}

func fileCompletions(prefix string) []string {
	dir := "."
	base := prefix
	if prefix == "" {
		base = ""
	}
	if i := strings.LastIndexByte(prefix, '/'); i >= 0 {
		dir = prefix[:i]
		base = prefix[i+1:]
		if dir == "" {
			dir = "/"
		}
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []string
	for _, ent := range entries {
		name := ent.Name()
		if strings.HasPrefix(name, ".") && !strings.HasPrefix(base, ".") {
			continue
		}
		if !strings.HasPrefix(name, base) {
			continue
		}
		full := name
		if dir != "." || strings.Contains(prefix, "/") {
			full = filepath.Join(dir, name)
		}
		if ent.IsDir() {
			full += "/"
		}
		out = append(out, full)
	}
	return out
}

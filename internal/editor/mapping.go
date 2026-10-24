package editor

import (
	"fmt"
	"sort"
	"strings"

	"vigo/internal/keys"
)

// mapping is one key mapping.
type mapping struct {
	lhs    []keys.Key
	rhs    []keys.Key
	mode   byte
	remap  bool
	silent bool
	nowait bool
	buf    int
}

// mapMode returns the mapping mode letter for the current state.
func (e *Editor) mapMode() byte {
	switch e.mode {
	case ModeNormal:
		if e.pending.op != "" {
			return 'o'
		}
		return 'n'
	case ModeInsert, ModeReplace:
		return 'i'
	case ModeVisual, ModeVisualLine, ModeVisualBlock:
		return 'x'
	case ModeCmdline, ModeSearchForward, ModeSearchBackward:
		return 'c'
	}
	return 'n'
}

// mapsFor returns the mappings that apply to a mode, most specific first.
func (e *Editor) mapsFor(mode byte) []*mapping {
	if e.maps == nil {
		return nil
	}
	out := append([]*mapping{}, e.maps[mode]...)
	if mode == 'x' {
		out = append(out, e.maps['v']...)
	}
	return out
}

func keysPrefix(short, long []keys.Key) bool {
	if len(short) > len(long) {
		return false
	}
	for i := range short {
		if short[i] != long[i] {
			return false
		}
	}
	return true
}

// lookupMapping finds an exact match and the longest partial match.
func (e *Editor) lookupMapping(mode byte, buf []keys.Key) (exact, partial *mapping) {
	for _, m := range e.mapsFor(mode) {
		switch {
		case len(m.lhs) == len(buf) && keysPrefix(buf, m.lhs):
			if exact == nil || len(m.lhs) > len(exact.lhs) {
				exact = m
			}
		case len(m.lhs) > len(buf) && keysPrefix(buf, m.lhs):
			if partial == nil || len(m.lhs) < len(partial.lhs) {
				partial = m
			}
		}
	}
	return exact, partial
}

// feedKey is the mapping aware entry point for keys coming from the user.
func (e *Editor) feedKey(k keys.Key) {
	if len(e.maps) == 0 {
		e.handleKey(k)
		return
	}
	e.mapBuf = append(e.mapBuf, k)
	e.resolveMappings()
}

// flushMappings processes buffered keys when a mapping prefix times out.
func (e *Editor) flushMappings() {
	for len(e.mapBuf) > 0 {
		k := e.mapBuf[0]
		e.mapBuf = e.mapBuf[1:]
		e.handleKey(k)
	}
	e.mapPending = false
}

// resolveMappings decides what to do with the buffered keys.
func (e *Editor) resolveMappings() {
	for len(e.mapBuf) > 0 {
		if e.mapDepth > 200 {
			e.mapBuf = nil
			e.mapPending = false
			e.errorf("E223: recursive mapping")
			return
		}
		mode := e.mapMode()
		exact, partial := e.lookupMapping(mode, e.mapBuf)
		switch {
		case exact != nil:
			e.mapBuf = e.mapBuf[len(exact.lhs):]
			if mode == 'n' && e.pending.awaiting != "" {
				// A mapping takes over from a half typed built in command.
				e.pending.reset()
			}
			e.execMapping(exact)
		case partial != nil:
			e.mapPending = true
			return
		default:
			k := e.mapBuf[0]
			e.mapBuf = e.mapBuf[1:]
			e.handleKey(k)
		}
	}
	e.mapPending = false
}

// execMapping runs the right hand side of a mapping.
func (e *Editor) execMapping(m *mapping) {
	rhs := make([]keys.Key, 0, len(m.rhs))
	for _, k := range m.rhs {
		if k.IsSpecial("nop") {
			continue
		}
		rhs = append(rhs, k)
	}
	if m.remap {
		e.mapDepth++
		e.mapBuf = append(append([]keys.Key{}, rhs...), e.mapBuf...)
		e.resolveMappings()
		e.mapDepth--
		return
	}
	for _, k := range rhs {
		e.handleKey(k)
		if e.quit {
			return
		}
	}
}

func (e *Editor) addMapping(m *mapping) {
	if e.maps == nil {
		e.maps = map[byte][]*mapping{}
	}
	m.buf = e.bufIdx
	list := e.maps[m.mode]
	for i, old := range list {
		if old.buf == m.buf && len(old.lhs) == len(m.lhs) && keysPrefix(old.lhs, m.lhs) {
			list[i] = m
			return
		}
	}
	e.maps[m.mode] = append(list, m)
}

func (e *Editor) removeMapping(mode byte, lhs []keys.Key) bool {
	removed := false
	for _, name := range []byte{mode} {
		list := e.maps[name]
		out := list[:0]
		for _, m := range list {
			if m.buf == e.bufIdx && len(m.lhs) == len(lhs) && keysPrefix(lhs, m.lhs) {
				removed = true
				continue
			}
			out = append(out, m)
		}
		e.maps[name] = out
	}
	return removed
}

// expandLeader replaces <leader> and <localleader> in mapping text.
func (e *Editor) expandLeader(s string) string {
	leader := e.leader
	if leader == "" {
		leader = "\\"
	}
	s = replaceInsensitive(s, "<leader>", leader)
	s = replaceInsensitive(s, "<localleader>", leader)
	return s
}

func replaceInsensitive(s, old, new string) string {
	lower := strings.ToLower(s)
	idx := strings.Index(lower, old)
	for idx >= 0 {
		s = s[:idx] + new + s[idx+len(old):]
		lower = strings.ToLower(s)
		idx = strings.Index(lower, old)
	}
	return s
}

// parseMapArgs splits ":map" arguments into attributes, lhs and rhs.
func (e *Editor) parseMapArgs(args string) (lhs, rhs []keys.Key, attrs map[string]bool, err error) {
	attrs = map[string]bool{}
	rest := strings.TrimLeft(args, " \t")
	for strings.HasPrefix(rest, "<") {
		end := strings.IndexByte(rest, '>')
		if end < 0 {
			break
		}
		name := strings.ToLower(rest[1:end])
		switch name {
		case "silent", "buffer", "nowait", "special", "script":
			attrs[name] = true
			rest = strings.TrimLeft(rest[end+1:], " \t")
			continue
		case "expr":
			return nil, nil, nil, fmt.Errorf("E: <expr> mappings are not supported")
		}
		break
	}
	if rest == "" {
		return nil, nil, nil, fmt.Errorf("E471: Argument required")
	}
	sp := strings.IndexAny(rest, " \t")
	lhsText := rest
	rhsText := ""
	if sp >= 0 {
		lhsText = rest[:sp]
		rhsText = strings.TrimLeft(rest[sp+1:], " \t")
	}
	lhs, err = e.parseMapKeys(lhsText)
	if err != nil {
		return nil, nil, nil, err
	}
	if len(lhs) == 0 {
		return nil, nil, nil, fmt.Errorf("E474: Invalid argument")
	}
	if rhsText != "" {
		rhs, err = e.parseMapKeys(rhsText)
		if err != nil {
			return nil, nil, nil, err
		}
	}
	return lhs, rhs, attrs, nil
}

func (e *Editor) parseMapKeys(s string) ([]keys.Key, error) {
	s = e.expandLeader(s)
	ks, err := keys.Parse(s)
	if err != nil {
		return nil, err
	}
	return ks, nil
}

// exMap implements the :map family of commands.
func (e *Editor) exMap(mode byte, remap bool, args string) {
	args = strings.TrimSpace(args)
	if args == "" {
		e.listMappings(mode)
		return
	}
	lhs, rhs, attrs, err := e.parseMapArgs(args)
	if err != nil {
		e.errorf("%v", err)
		return
	}
	e.addMapping(&mapping{
		lhs:    lhs,
		rhs:    rhs,
		mode:   mode,
		remap:  remap,
		silent: attrs["silent"],
		nowait: attrs["nowait"],
		buf:    e.bufIdx,
	})
}

// removeMappingList removes a mapping from one mode.
// exMapClear removes mappings for a mode.
func (e *Editor) exMapClear(mode byte) {
	if mode == 0 {
		e.maps = map[byte][]*mapping{}
		return
	}
	e.maps[mode] = nil
}

// listMappings prints the mappings of a mode, like :map.
func (e *Editor) listMappings(mode byte) {
	var modes []byte
	if mode == 0 {
		modes = []byte{'n', 'x', 'v', 'i', 'c', 'o', '!'}
	} else {
		modes = []byte{mode}
	}
	var lines []string
	for _, m := range modes {
		list := append([]*mapping{}, e.maps[m]...)
		sort.Slice(list, func(i, j int) bool {
			return keysString(list[i].lhs) < keysString(list[j].lhs)
		})
		for _, mp := range list {
			name := mapModeName(m)
			lhs := keysString(mp.lhs)
			rhs := keysString(mp.rhs)
			if len(mp.rhs) == 0 {
				rhs = "<Nop>"
			}
			lines = append(lines, fmt.Sprintf("%-3s %-16s %s", name, lhs, rhs))
		}
	}
	if len(lines) == 0 {
		e.messagef("No mapping found")
		return
	}
	e.messagef("%s", strings.Join(lines, "\n"))
}

func mapModeName(m byte) string {
	switch m {
	case 'n':
		return ""
	case 'x', 'v':
		return "x"
	case 'i':
		return "!"
	case 'c':
		return "c"
	case 'o':
		return "o"
	}
	return string(m)
}

func keysString(ks []keys.Key) string {
	var sb strings.Builder
	for _, k := range ks {
		sb.WriteString(k.String())
	}
	return sb.String()
}

// expandModes turns the shortcut mode letters into the real ones.
func expandModes(mode byte) []byte {
	switch mode {
	case 0:
		return []byte{'n', 'x', 'o'}
	case '!':
		return []byte{'i', 'c'}
	}
	return []byte{mode}
}

func (e *Editor) exMapFamily(mode byte, remap bool, args string) {
	if strings.TrimSpace(args) == "" {
		e.listMappings(mode)
		return
	}
	for _, m := range expandModes(mode) {
		e.exMap(m, remap, args)
	}
}

func (e *Editor) exUnmapFamily(mode byte, args string) {
	if strings.TrimSpace(args) == "" {
		e.errorf("E474: Invalid argument")
		return
	}
	lhs, _, _, err := e.parseMapArgs(args)
	if err != nil {
		e.errorf("%v", err)
		return
	}
	removed := false
	for _, m := range expandModes(mode) {
		if e.removeMapping(m, lhs) {
			removed = true
		}
	}
	if !removed {
		e.errorf("E31: No such mapping")
	}
}

func (e *Editor) exMapClearFamily(mode byte) {
	if mode == 0 || mode == '!' {
		for _, m := range expandModes(mode) {
			e.exMapClear(m)
		}
		return
	}
	e.exMapClear(mode)
}

package editor

import (
	"strings"

	"vigo/internal/buffer"
)

// indentBlock re-indents the lines of a range the way the "=" operator does.
// Vim uses 'cindent', 'indentexpr' or 'equalprg' for this; vigo ships a small
// built in indenter for the C family and for Python, and leaves other file
// types alone just like vim without those options.
func (e *Editor) indentBlock(r rangeInfo) {
	nr := e.normalizeRange(r)
	from := clamp(nr.from.Line, 0, e.buf.LastLine())
	to := clamp(nr.to.Line-1, 0, e.buf.LastLine())
	if nr.kind != mLine && nr.to.Col == 0 && nr.to.Line > from {
		to = clamp(nr.to.Line-1, 0, e.buf.LastLine())
	}
	if from > to {
		return
	}
	unit := e.indentUnit(1)
	depth := 0
	switch e.fileType() {
	case "go", "c", "java", "javascript", "typescript", "rust", "css", "json", "protobuf":
		depth = e.depthBefore(from)
		e.buf.Checkpoint(e.cur)
		e.indentBraces(from, to, depth, unit)
	case "python":
		e.buf.Checkpoint(e.cur)
		e.indentPython(from, to, unit)
	default:
		return
	}
	line := from
	e.setCursor(buffer.Pos{Line: line, Col: e.buf.FirstNonBlank(line).Col})
}

// depthBefore counts the open brackets before a line.
func (e *Editor) depthBefore(line int) int {
	depth := 0
	inBlockComment := false
	for i := 0; i < line && i <= e.buf.LastLine(); i++ {
		depth += e.braceDelta(e.buf.LineString(i), &inBlockComment)
	}
	if depth < 0 {
		depth = 0
	}
	return depth
}

// braceDelta returns the net number of opening brackets on a line, ignoring
// brackets inside strings and comments.
func (e *Editor) braceDelta(s string, inBlockComment *bool) int {
	lang := e.lang()
	runes := []rune(s)
	net := 0
	i := 0
	for i < len(runes) {
		if *inBlockComment {
			if lang != nil && hasPrefixAt(runes, i, lang.blockClose) {
				*inBlockComment = false
				i += len([]rune(lang.blockClose))
				continue
			}
			i++
			continue
		}
		if lang != nil && lang.lineComment != "" && hasPrefixAt(runes, i, lang.lineComment) {
			break
		}
		if lang != nil && lang.blockOpen != "" && hasPrefixAt(runes, i, lang.blockOpen) {
			*inBlockComment = true
			i += len([]rune(lang.blockOpen))
			continue
		}
		switch runes[i] {
		case '"', '\'', '`':
			quote := runes[i]
			if lang != nil && !strings.ContainsRune(lang.quotes, quote) && quote != '`' {
				i++
				continue
			}
			i++
			for i < len(runes) {
				if runes[i] == '\\' {
					i += 2
					continue
				}
				if runes[i] == quote {
					i++
					break
				}
				i++
			}
			continue
		case '{', '(', '[':
			net++
		case '}', ')', ']':
			net--
		}
		i++
	}
	return net
}

func (e *Editor) indentBraces(from, to, depth int, unit string) {
	inBlockComment := false
	// Recompute the comment state at the start of the range.
	for i := 0; i < from && i <= e.buf.LastLine(); i++ {
		e.braceDelta(e.buf.LineString(i), &inBlockComment)
	}
	width := len([]rune(unit))
	if width == 0 {
		width = 1
	}
	for i := from; i <= to && i <= e.buf.LastLine(); i++ {
		text := e.buf.LineString(i)
		trimmed := strings.TrimSpace(text)
		delta := e.braceDelta(text, &inBlockComment)
		if trimmed == "" {
			e.buf.SetLine(i, nil)
			continue
		}
		level := depth
		if strings.HasPrefix(trimmed, "}") || strings.HasPrefix(trimmed, ")") || strings.HasPrefix(trimmed, "]") {
			level--
		}
		if strings.HasPrefix(trimmed, "case ") || strings.HasPrefix(trimmed, "default:") {
			level--
		}
		if level < 0 {
			level = 0
		}
		indent := strings.Repeat(unit, level)
		e.buf.SetLine(i, []rune(indent+trimmed))
		depth += delta
		if depth < 0 {
			depth = 0
		}
	}
	_ = width
}

func (e *Editor) indentPython(from, to int, unit string) {
	depth := 0
	for i := from; i <= to && i <= e.buf.LastLine(); i++ {
		text := e.buf.LineString(i)
		trimmed := strings.TrimSpace(text)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			e.buf.SetLine(i, []rune(trimmed))
			continue
		}
		if pythonDedent(trimmed) && depth > 0 {
			depth--
		}
		e.buf.SetLine(i, []rune(strings.Repeat(unit, depth)+trimmed))
		if strings.HasSuffix(trimmed, ":") {
			depth++
		}
	}
}

func pythonDedent(trimmed string) bool {
	word := trimmed
	if i := strings.IndexAny(word, " \t(:["); i > 0 {
		word = word[:i]
	}
	switch word {
	case "return", "break", "continue", "raise", "pass", "yield":
		return true
	case "else", "elif", "except", "finally":
		return true
	}
	return false
}

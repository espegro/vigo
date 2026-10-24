package editor

import (
	"fmt"
	"regexp"
	"strings"
)

type subFlags struct {
	global     bool
	ignoreCase bool
	caseForced bool
	sensitive  bool
	confirm    bool
	count      bool
	quiet      bool
	flags      string
}

func (e *Editor) exSubstitute(cmd exCommand) {
	args := cmd.args
	var (
		pat, rep, flagStr string
		repeat            bool
	)
	if args == "" {
		pat, rep, flagStr = e.lastSubst, e.lastSubstRep, e.lastSubstFlags
		repeat = true
	} else if isAlphaNum(args[0]) {
		pat, rep, flagStr = e.lastSubst, e.lastSubstRep, args
		repeat = true
	} else {
		p, r, f, err := splitSubstArgs(args)
		if err != nil {
			e.errorf("%v", err)
			return
		}
		pat, rep, flagStr = p, r, f
	}
	if pat == "" {
		pat = e.lastSearch
	}
	if pat == "" {
		e.errorf("E35: No previous substitute regular expression")
		return
	}
	if rep == "" && !repeat {
		rep = ""
	}
	if rep == "~" {
		rep = e.lastSubstRep
	}
	flags := parseSubFlags(flagStr)
	from, to := e.cur.Line, e.cur.Line
	if cmd.rangeSet {
		from, to = clamp(cmd.from, 0, e.buf.LastLine()), clamp(cmd.to, 0, e.buf.LastLine())
	}
	if !repeat {
		e.lastSubst = pat
		e.lastSubstRep = rep
	}
	e.lastSubstFlags = flagStr
	n, lines, lastLine := e.substituteLines(from, to, pat, rep, flags)
	if flags.count {
		e.messagef("%d matches", n)
		return
	}
	if n == 0 {
		if !flags.quiet {
			e.errorf("E486: Pattern not found: %s", pat)
		}
		return
	}
	if lastLine >= 0 {
		e.setCursor(e.buf.FirstNonBlank(clamp(lastLine, 0, e.buf.LastLine())))
	}
	if n > 1 || lines > 1 {
		e.messagef("%d substitutions on %d lines", n, lines)
	}
}

func (e *Editor) substituteLines(from, to int, pat, rep string, flags subFlags) (int, int, int) {
	ignoreCase := e.opts.IgnoreCase
	if e.opts.SmartCase && patternHasUpper(pat) {
		ignoreCase = false
	}
	if flags.ignoreCase {
		ignoreCase = true
	}
	if flags.sensitive {
		ignoreCase = false
	}
	expr, err := patternToRegexp(pat, ignoreCase)
	if err != nil {
		e.errorf("E383: Invalid pattern: %v", err)
		return 0, 0, -1
	}
	re, err := regexp.Compile(expr)
	if err != nil {
		e.errorf("E383: Invalid pattern: %v", err)
		return 0, 0, -1
	}
	quit := false
	total, changedLines, lastLine := 0, 0, -1
	e.buf.Checkpoint(e.cur)
	for line := to; line >= from && line <= e.buf.LastLine(); line-- {
		if quit {
			break
		}
		src := e.buf.LineString(line)
		out, n, quit2 := e.substituteOne(src, line, re, rep, pat, flags)
		if quit2 {
			quit = true
		}
		if n == 0 {
			continue
		}
		total += n
		changedLines++
		lastLine = line
		if out == src {
			continue
		}
		if strings.Contains(out, "\n") {
			parts := strings.Split(out, "\n")
			e.buf.SetLine(line, []rune(parts[0]))
			extra := make([][]rune, 0, len(parts)-1)
			for _, p := range parts[1:] {
				extra = append(extra, []rune(p))
			}
			e.buf.InsertLines(line+1, extra)
		} else {
			e.buf.SetLine(line, []rune(out))
		}
	}
	return total, changedLines, lastLine
}

// substituteOne rewrites one line; it returns the new text, how many matches
// were replaced and whether the user asked to stop.
func (e *Editor) substituteOne(src string, line int, re *regexp.Regexp, rep, pat string, flags subFlags) (string, int, bool) {
	locs := re.FindAllStringSubmatchIndex(src, -1)
	if len(locs) == 0 {
		return src, 0, false
	}
	if !flags.global {
		locs = locs[:1]
	}
	var out strings.Builder
	last := 0
	count := 0
	for _, loc := range locs {
		// Confirmation needs a key press, which a script cannot provide.
		if flags.confirm && e.scriptDepth == 0 {
			preview := src[:loc[0]] + e.previewReplacement(src, loc, rep) + src[loc[1]:]
			answer := e.confirm(fmt.Sprintf("replace with %s (y/n/a/q/l/^E/^Y)?", e.previewReplacement(src, loc, rep)))
			stop := false
			switch answer {
			case 'y':
			case 'n':
				continue
			case 'a':
				flags.confirm = false
			case 'l':
				flags.confirm = false
				stop = true
			case 'q', 0x1b:
				out.WriteString(src[last:])
				return out.String(), count, true
			default:
				continue
			}
			_ = preview
			if stop {
				replacement, err := expandReplacement(rep, loc, src)
				if err == nil {
					out.WriteString(src[last:loc[0]])
					out.WriteString(replacement)
					count++
				}
				out.WriteString(src[loc[1]:])
				return out.String(), count, false
			}
		}
		replacement, err := expandReplacement(rep, loc, src)
		if err != nil {
			continue
		}
		out.WriteString(src[last:loc[0]])
		out.WriteString(replacement)
		last = loc[1]
		count++
	}
	out.WriteString(src[last:])
	return out.String(), count, false
}

func (e *Editor) previewReplacement(src string, loc []int, rep string) string {
	text, err := expandReplacement(rep, loc, src)
	if err != nil {
		return ""
	}
	return text
}

func isAlphaNum(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9'
}

// splitSubstArgs splits /pat/rep/flags using any single character delimiter.
func splitSubstArgs(args string) (pat, rep, flags string, err error) {
	delim := args[0]
	if isAlphaNum(delim) || delim == '\\' || delim == '"' || delim == '|' {
		return "", "", "", fmt.Errorf("E146: Regular expressions can't be delimited by letters")
	}
	parts := []string{}
	cur := strings.Builder{}
	escaped := false
	for i := 1; i < len(args); i++ {
		c := args[i]
		if escaped {
			cur.WriteByte('\\')
			cur.WriteByte(c)
			escaped = false
			continue
		}
		if c == '\\' {
			escaped = true
			continue
		}
		if c == delim {
			parts = append(parts, cur.String())
			cur.Reset()
			if len(parts) == 2 {
				flags = args[i+1:]
				return parts[0], parts[1], strings.TrimSpace(flags), nil
			}
			continue
		}
		cur.WriteByte(c)
	}
	if escaped {
		cur.WriteByte('\\')
	}
	parts = append(parts, cur.String())
	switch len(parts) {
	case 1:
		return parts[0], "", "", nil
	default:
		return parts[0], parts[1], "", nil
	}
}

func parseSubFlags(s string) subFlags {
	var f subFlags
	f.flags = s
	for _, r := range s {
		switch r {
		case 'g':
			f.global = true
		case 'i':
			f.ignoreCase = true
		case 'I':
			f.sensitive = true
		case 'c':
			f.confirm = true
		case 'n':
			f.count = true
		case 'e':
			f.quiet = true
		case '&':
		}
	}
	return f
}

// repeatSubstitute implements & and :&.
func (e *Editor) repeatSubstitute() {
	if e.lastSubst == "" {
		e.errorf("E35: No previous substitute regular expression")
		return
	}
	flags := parseSubFlags(e.lastSubstFlags)
	n, lines, lastLine := e.substituteLines(e.cur.Line, e.cur.Line, e.lastSubst, e.lastSubstRep, flags)
	if n == 0 {
		e.errorf("E486: Pattern not found: %s", e.lastSubst)
		return
	}
	if lastLine >= 0 {
		e.setCursor(e.buf.FirstNonBlank(clamp(lastLine, 0, e.buf.LastLine())))
	}
	if n > 1 {
		e.messagef("%d substitutions on %d lines", n, lines)
	}
}

func (e *Editor) repeatSubstituteWithLastReplacement() {
	if e.lastSubst == "" {
		e.errorf("E35: No previous substitute regular expression")
		return
	}
	flags := parseSubFlags(e.lastSubstFlags)
	n, lines, lastLine := e.substituteLines(e.cur.Line, e.cur.Line, e.lastSubst, e.lastSubstRep, flags)
	if n == 0 {
		e.errorf("E486: Pattern not found: %s", e.lastSubst)
		return
	}
	if lastLine >= 0 {
		e.setCursor(e.buf.FirstNonBlank(clamp(lastLine, 0, e.buf.LastLine())))
	}
	if n > 1 {
		e.messagef("%d substitutions on %d lines", n, lines)
	}
}

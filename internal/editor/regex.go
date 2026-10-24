package editor

import (
	"fmt"
	"regexp"
	"strings"
)

type magicLevel int

const (
	veryMagic magicLevel = iota
	magic
	noMagic
	veryNoMagic
)

// patternToRegexp translates a vim regular expression into Go's RE2 syntax.
// The subset vim calls "magic" is the default, the \v \m \M \V switches change
// it and \c \C override the case sensitivity.
func patternToRegexp(pat string, ignoreCase bool) (string, error) {
	level := magic
	var out strings.Builder
	runes := []rune(pat)
	forceCase := 0
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		if r == '\\' && i+1 < len(runes) {
			next := runes[i+1]
			switch next {
			case 'v':
				level = veryMagic
				i++
				continue
			case 'm':
				level = magic
				i++
				continue
			case 'M':
				level = noMagic
				i++
				continue
			case 'V':
				level = veryNoMagic
				i++
				continue
			case 'c':
				forceCase = 1
				i++
				continue
			case 'C':
				forceCase = 2
				i++
				continue
			}
		}
		if r == '\\' {
			if i+1 >= len(runes) {
				return "", fmt.Errorf("E54: Unmatched \\")
			}
			next := runes[i+1]
			if i+2 < len(runes) {
				pair := string(runes[i+1 : i+3])
				if pair == "zs" || pair == "ze" {
					return "", fmt.Errorf(`E869: \zs and \ze are not supported`)
				}
			}
			i++
			lit, special, err := translateEscape(next, level)
			if err != nil {
				return "", err
			}
			if special {
				out.WriteString(lit)
			} else {
				out.WriteString(regexp.QuoteMeta(lit))
			}
			continue
		}
		if specialAtLevel(r, level) {
			if r == '(' || r == ')' || r == '|' {
				out.WriteRune(r)
				continue
			}
			out.WriteRune(r)
			continue
		}
		out.WriteString(regexp.QuoteMeta(string(r)))
	}
	expr := out.String()
	switch forceCase {
	case 1:
		ignoreCase = true
	case 2:
		ignoreCase = false
	}
	if ignoreCase {
		expr = "(?i)" + expr
	}
	return expr, nil
}

func specialAtLevel(r rune, level magicLevel) bool {
	switch level {
	case veryMagic:
		return strings.ContainsRune("()|+?{}.*[]^$", r)
	case magic:
		return strings.ContainsRune(".*[]^$", r)
	case noMagic:
		return strings.ContainsRune(".*[]^$", r)
	default:
		return false
	}
}

func vimEscapes(r rune) bool {
	return strings.ContainsRune("()|+?{}", r)
}

// translateEscape handles the character after a backslash. It returns the
// regexp text, whether the result is already a regexp fragment, and an error.
func translateEscape(r rune, level magicLevel) (string, bool, error) {
	switch r {
	case 'd', 'D', 'w', 'W', 's', 'S':
		return "\\" + string(r), true, nil
	case 'a':
		return "[A-Za-z]", true, nil
	case 'A':
		return "[^A-Za-z]", true, nil
	case 'l':
		return "[a-z]", true, nil
	case 'u':
		return "[A-Z]", true, nil
	case 'b':
		return "\\b", true, nil
	case 'B':
		return "\\B", true, nil
	case 'e':
		return "\\x1b", true, nil
	case 't':
		return "\\t", true, nil
	case 'n':
		return "\\n", true, nil
	case 'r':
		return "\\r", true, nil
	case '<':
		return "\\b", true, nil
	case '>':
		return "\\b", true, nil
	case '1', '2', '3', '4', '5', '6', '7', '8', '9':
		return "", false, fmt.Errorf("E65: backreferences are not supported")
	case '+', '?', '*':
		if level == magic || level == noMagic {
			return string(r), true, nil
		}
		return string(r), false, nil
	case '|':
		if level == magic || level == veryMagic {
			return "|", true, nil
		}
		return "|", false, nil
	case '(', ')':
		return string(r), true, nil
	case '{', '}':
		if level == magic || level == veryMagic {
			return string(r), true, nil
		}
		return string(r), false, nil
	case '.', '[', ']', '^', '$', '~', '"', '\\', '/':
		return string(r), true, nil
	}
	return string(r), false, nil
}

// expandReplacement substitutes & and \1..\9 in a vim replacement string.
func expandReplacement(rep string, match []int, src string) (string, error) {
	var out strings.Builder
	runes := []rune(rep)
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		if r == '&' {
			out.WriteString(src[match[0]:match[1]])
			continue
		}
		if r == '~' {
			out.WriteString(src[match[0]:match[1]])
			continue
		}
		if r != '\\' || i+1 >= len(runes) {
			out.WriteRune(r)
			continue
		}
		i++
		n := runes[i]
		switch {
		case n >= '1' && n <= '9':
			idx := int(n - '0')
			if idx*2+1 < len(match) && match[idx*2] >= 0 {
				out.WriteString(src[match[idx*2]:match[idx*2+1]])
			}
		case n == 'r', n == 'n':
			out.WriteByte('\n')
		case n == 't':
			out.WriteByte('\t')
		case n == 'e':
			out.WriteByte(0x1b)
		case n == '\\':
			out.WriteByte('\\')
		case n == '&':
			out.WriteByte('&')
		case n == 'u':
			if i+1 < len(runes) {
				i++
				out.WriteString(strings.ToUpper(string(runes[i])))
			}
		case n == 'l':
			if i+1 < len(runes) {
				i++
				out.WriteString(strings.ToLower(string(runes[i])))
			}
		case n == 'U':
			for i+1 < len(runes) && runes[i+1] != 'E' && runes[i+1] != 'e' {
				i++
				out.WriteString(strings.ToUpper(string(runes[i])))
			}
		case n == 'L':
			for i+1 < len(runes) && runes[i+1] != 'E' && runes[i+1] != 'e' {
				i++
				out.WriteString(strings.ToLower(string(runes[i])))
			}
		case n == 'E':
		default:
			out.WriteRune(n)
		}
	}
	return out.String(), nil
}

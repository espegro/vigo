// Package keys models keyboard input: the events the editor reacts to, a
// human readable notation for them, and the decoder that turns raw terminal
// bytes into events.
package keys

import (
	"fmt"
	"strings"
)

// Kind tells which of the Key fields carries the identity of the key.
type Kind uint8

const (
	// KindRune is a printable character, including space.
	KindRune Kind = iota
	// KindCtrl is a control character; R is the base character in upper case.
	KindCtrl
	// KindSpecial is a named non printing key such as esc or up.
	KindSpecial
)

// Names of the non printing keys.
const (
	Esc   = "esc"
	CR    = "cr"
	Tab   = "tab"
	BS    = "bs"
	Del   = "del"
	Ins   = "ins"
	Up    = "up"
	Down  = "down"
	Left  = "left"
	Right = "right"
	Home  = "home"
	End   = "end"
	PgUp  = "pgup"
	PgDn  = "pgdn"
	BTab  = "btab"
)

// Key is a single key event.
type Key struct {
	Kind Kind
	// R holds the character for KindRune and the base character for KindCtrl.
	R rune
	// Name holds the key name for KindSpecial.
	Name string
	// Alt is true when the key was pressed together with Meta/Alt.
	Alt bool
	// Ctrl is true for control modified special keys such as ctrl+arrow.
	Ctrl bool
	// Paste marks keys produced by a terminal bracketed paste.
	Paste bool
}

// Rune returns a printable key.
func Rune(r rune) Key { return Key{Kind: KindRune, R: r} }

// Ctrl returns a control key; r is the base character, normally a letter.
func Ctrl(r rune) Key { return Key{Kind: KindCtrl, R: upper(r)} }

// Special returns a named key such as keys.Up.
func Special(name string) Key { return Key{Kind: KindSpecial, Name: name} }

// IsRune reports whether k is a printable character.
func (k Key) IsRune() bool { return k.Kind == KindRune }

// IsCtrl reports whether k is a control character.
func (k Key) IsCtrl() bool { return k.Kind == KindCtrl }

// Is reports whether k is the control key c.
func (k Key) Is(r rune) bool {
	if k.Alt {
		return false
	}
	switch k.Kind {
	case KindRune:
		return k.R == r
	case KindCtrl:
		return upper(r) == k.R
	}
	return false
}

// IsSpecial reports whether k is the named special key.
func (k Key) IsSpecial(name string) bool {
	return k.Kind == KindSpecial && !k.Alt && k.Name == name
}

// IsAlt reports whether k is the alt modified character r.
func (k Key) IsAlt(r rune) bool {
	return k.Alt && k.Kind == KindRune && k.R == r
}

func upper(r rune) rune {
	if r >= 'a' && r <= 'z' {
		return r - 'a' + 'A'
	}
	return r
}

// String renders the key in the angle bracket notation used by vim, for
// example "a", "<C-r>" or "<Esc>".
func (k Key) String() string {
	var inner string
	switch k.Kind {
	case KindRune:
		switch k.R {
		case ' ':
			inner = "Space"
		case '<':
			inner = "lt"
		default:
			inner = string(k.R)
		}
	case KindCtrl:
		switch k.R {
		case '\\':
			inner = `C-\`
		default:
			inner = "C-" + strings.ToLower(string(k.R))
		}
	case KindSpecial:
		name := specialDisplay(k.Name)
		if k.Ctrl {
			name = "C-" + name
		}
		inner = name
	}
	if k.Alt {
		return "<M-" + inner + ">"
	}
	if k.Kind == KindRune && k.R != ' ' && k.R != '<' {
		return inner
	}
	return "<" + inner + ">"
}

func specialDisplay(name string) string {
	switch name {
	case Esc:
		return "Esc"
	case CR:
		return "CR"
	case Tab:
		return "Tab"
	case BS:
		return "BS"
	case Del:
		return "Del"
	case Ins:
		return "Ins"
	case Up:
		return "Up"
	case Down:
		return "Down"
	case Left:
		return "Left"
	case Right:
		return "Right"
	case Home:
		return "Home"
	case End:
		return "End"
	case PgUp:
		return "PageUp"
	case PgDn:
		return "PageDown"
	case BTab:
		return "S-Tab"
	}
	if IsFKey(name) {
		return strings.ToUpper(name)
	}
	return name
}

// IsFKey reports whether name names a function key.
func IsFKey(name string) bool {
	if len(name) < 2 || name[0] != 'f' {
		return false
	}
	n := 0
	for _, c := range name[1:] {
		if c < '0' || c > '9' {
			return false
		}
		n = n*10 + int(c-'0')
	}
	return n >= 1 && n <= 12
}

// FKey returns the function key names f1 through f12.
func FKey(n int) string { return fmt.Sprintf("f%d", n) }

// Parse turns vim style key notation into key events. Plain characters map to
// themselves, "<Esc>" and friends to special keys, "<C-x>" to control keys and
// "<M-x>" to alt modified keys.
func Parse(s string) ([]Key, error) {
	var out []Key
	for i := 0; i < len(s); {
		if s[i] != '<' {
			r, size := decodeRune(s[i:])
			out = append(out, Rune(r))
			i += size
			continue
		}
		end := strings.IndexByte(s[i:], '>')
		if end < 0 {
			out = append(out, Rune('<'))
			i++
			continue
		}
		body := s[i+1 : i+end]
		k, err := parseName(body)
		if err != nil {
			out = append(out, Rune('<'))
			i++
			continue
		}
		out = append(out, k)
		i += end + 1
	}
	return out, nil
}

// MustParse is Parse for static strings in tests.
func MustParse(s string) []Key {
	ks, err := Parse(s)
	if err != nil {
		panic(err)
	}
	return ks
}

func parseName(body string) (Key, error) {
	if body == "" {
		return Key{}, fmt.Errorf("empty key name")
	}
	rest := body
	alt := false
	ctrl := false
	for {
		switch {
		case len(rest) > 2 && (rest[0] == 'M' || rest[0] == 'A' || rest[0] == 'm' || rest[0] == 'a') && rest[1] == '-':
			alt = true
			rest = rest[2:]
			continue
		case len(rest) > 2 && (rest[0] == 'C' || rest[0] == 'c') && rest[1] == '-':
			ctrl = true
			rest = rest[2:]
			continue
		case len(rest) > 2 && (rest[0] == 'S' || rest[0] == 's') && rest[1] == '-' && !strings.EqualFold(rest, "space") && !strings.EqualFold(rest, "stab"):
			rest = rest[2:]
			continue
		}
		break
	}
	if rest == "" {
		return Key{}, fmt.Errorf("empty key name")
	}
	lower := strings.ToLower(rest)
	if len([]rune(rest)) == 1 {
		r := []rune(rest)[0]
		if ctrl {
			if r == '[' {
				return Key{Kind: KindSpecial, Name: Esc, Alt: alt}, nil
			}
			if !isAlphaNum(r) && !ctrlPunct(r) {
				return Key{}, fmt.Errorf("bad control key %q", body)
			}
			return Key{Kind: KindCtrl, R: upper(r), Alt: alt}, nil
		}
		return Key{Kind: KindRune, R: r, Alt: alt}, nil
	}
	name := ""
	switch lower {
	case "esc", "escape":
		name = Esc
	case "cr", "enter", "return":
		name = CR
	case "nl":
		name = CR
	case "tab", "c-i":
		name = Tab
	case "bs", "backspace":
		name = BS
	case "del", "delete":
		name = Del
	case "ins", "insert":
		name = Ins
	case "up":
		name = Up
	case "down":
		name = Down
	case "left":
		name = Left
	case "right":
		name = Right
	case "home":
		name = Home
	case "end":
		name = End
	case "pageup", "pgup", "page-up", "pagedown", "pgdn", "page-down":
		if strings.Contains(lower, "up") {
			name = PgUp
		} else {
			name = PgDn
		}
	case "space":
		if ctrl {
			return Key{Kind: KindCtrl, R: '@', Alt: alt}, nil
		}
		return Key{Kind: KindRune, R: ' ', Alt: alt}, nil
	case "lt":
		return Key{Kind: KindRune, R: '<', Alt: alt}, nil
	case "gt":
		return Key{Kind: KindRune, R: '>', Alt: alt}, nil
	case "bslash", "backslash":
		return Key{Kind: KindRune, R: '\\', Alt: alt}, nil
	case "bar":
		return Key{Kind: KindRune, R: '|', Alt: alt}, nil
	case "nop":
		return Key{Kind: KindSpecial, Name: "nop"}, nil
	case "nul":
		return Key{Kind: KindCtrl, R: '@', Alt: alt}, nil
	case "s-tab", "stab", "btab", "backtab":
		name = BTab
	default:
		if IsFKey(lower) {
			name = lower
		} else if ctrl {
			// ctrl+special, e.g. <C-Up>
			n, err := parseSpecial(rest)
			if err != nil {
				return Key{}, err
			}
			n.Ctrl = true
			n.Alt = alt
			return n, nil
		} else {
			return Key{}, fmt.Errorf("unknown key name %q", body)
		}
	}
	k := Key{Kind: KindSpecial, Name: name, Alt: alt}
	if ctrl && name != Tab {
		return Key{}, fmt.Errorf("bad control key %q", body)
	}
	if ctrl {
		k = Key{Kind: KindCtrl, R: 'I', Alt: alt}
	}
	return k, nil
}

func parseSpecial(s string) (Key, error) {
	lower := strings.ToLower(s)
	switch lower {
	case "up":
		return Special(Up), nil
	case "down":
		return Special(Down), nil
	case "left":
		return Special(Left), nil
	case "right":
		return Special(Right), nil
	case "home":
		return Special(Home), nil
	case "end":
		return Special(End), nil
	}
	if IsFKey(lower) {
		return Special(lower), nil
	}
	return Key{}, fmt.Errorf("unknown key name %q", s)
}

func isAlphaNum(r rune) bool {
	return r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9'
}

func ctrlPunct(r rune) bool {
	switch r {
	case '\\', ']', '^', '_', '@', '?', '-':
		return true
	}
	return false
}

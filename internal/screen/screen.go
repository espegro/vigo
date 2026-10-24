// Package screen renders a cell grid to a terminal, emitting escape sequences
// only for the cells that actually changed.
package screen

import (
	"bufio"
	"io"
	"strconv"
)

// Color is a terminal color; the zero value means "terminal default".
type Color struct {
	R, G, B uint8
	Set     bool
}

// RGB builds an explicit color.
func RGB(r, g, b uint8) Color { return Color{R: r, G: g, B: b, Set: true} }

// Attr describes how a cell is drawn.
type Attr struct {
	FG, BG    Color
	Bold      bool
	Dim       bool
	Italic    bool
	Underline bool
	Reverse   bool
	Blink     bool
}

// Cell is a single character cell.
type Cell struct {
	R rune
	// W is the cell width: 1 for normal, 2 for the first half of a wide
	// character and 0 for its trailing half.
	W uint8
	A Attr
}

var blank = Cell{R: ' ', W: 1}

// Screen is a double buffered terminal canvas.
type Screen struct {
	w, h      int
	cur, prev []Cell
	out       *bufio.Writer
	trueColor bool
	lastAttr  Attr
	attrSet   bool
}

// New creates a screen of w by h cells writing to out.
func New(w, h int, out io.Writer) *Screen {
	s := &Screen{out: bufio.NewWriterSize(out, 1<<16), trueColor: true}
	s.Resize(w, h)
	return s
}

// SetTrueColor selects between 24 bit and 256 color output.
func (s *Screen) SetTrueColor(on bool) { s.trueColor = on }

// Size returns the current dimensions.
func (s *Screen) Size() (int, int) { return s.w, s.h }

// Resize changes the canvas size and forces a full redraw.
func (s *Screen) Resize(w, h int) {
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	s.w, s.h = w, h
	cur := make([]Cell, w*h)
	prev := make([]Cell, w*h)
	for i := range cur {
		cur[i] = blank
		prev[i] = Cell{R: 0}
	}
	s.cur, s.prev = cur, prev
}

// Clear clears the canvas using attr as the default for empty cells.
func (s *Screen) Clear(attr Attr) {
	for i := range s.cur {
		c := blank
		c.A = attr
		s.cur[i] = c
	}
}

func (s *Screen) idx(x, y int) int { return y*s.w + x }

// Set places a rune at x,y, taking the rune width into account.
func (s *Screen) Set(x, y int, r rune, a Attr) {
	if y < 0 || y >= s.h || x < 0 || x >= s.w {
		return
	}
	w := RuneWidth(r)
	if w == 0 {
		return
	}
	if x+w > s.w {
		if w == 2 {
			s.cur[s.idx(x, y)] = Cell{R: ' ', W: 1, A: a}
		}
		return
	}
	s.cur[s.idx(x, y)] = Cell{R: r, W: uint8(w), A: a}
	if w == 2 && x+1 < s.w {
		s.cur[s.idx(x+1, y)] = Cell{R: 0, W: 0, A: a}
	}
}

// FillSpace fills a horizontal run of spaces with the given attributes.
func (s *Screen) FillSpace(x, y, w int, a Attr) {
	for i := 0; i < w; i++ {
		xx := x + i
		if xx < 0 || xx >= s.w || y < 0 || y >= s.h {
			continue
		}
		s.cur[s.idx(xx, y)] = Cell{R: ' ', W: 1, A: a}
	}
}

// Text draws str starting at x,y, clipping to max cells. It returns the number
// of cells drawn.
func (s *Screen) Text(x, y, max int, a Attr, str string) int {
	if y < 0 || y >= s.h || x >= s.w {
		return 0
	}
	used := 0
	for _, r := range str {
		w := RuneWidth(r)
		if used+w > max || x+used+w > s.w {
			break
		}
		s.Set(x+used, y, r, a)
		used += w
	}
	return used
}

// Flush writes the differences to the terminal and positions the cursor.
func (s *Screen) Flush(cursorX, cursorY int, showCursor bool) error {
	var buf []byte
	for y := 0; y < s.h; y++ {
		x := 0
		for x < s.w {
			cur := &s.cur[s.idx(x, y)]
			prev := &s.prev[s.idx(x, y)]
			if *cur == *prev {
				x++
				continue
			}
			if cur.W == 0 || cur.R == 0 {
				*prev = *cur
				x++
				continue
			}
			buf = appendCursor(buf, x, y)
			s.attrSet = false
			for x < s.w {
				cur = &s.cur[s.idx(x, y)]
				if cur.W == 0 || cur.R == 0 || *cur == s.prev[s.idx(x, y)] {
					break
				}
				buf = s.appendAttr(buf, cur.A)
				buf = appendRune(buf, cur.R)
				s.prev[s.idx(x, y)] = *cur
				if cur.W == 2 && x+1 < s.w {
					s.prev[s.idx(x+1, y)] = s.cur[s.idx(x+1, y)]
					x += 2
				} else {
					x++
				}
			}
		}
	}
	buf = s.appendAttr(buf, Attr{})
	if showCursor {
		buf = appendCursor(buf, cursorX, cursorY)
		buf = append(buf, "\x1b[?25h"...)
	} else {
		buf = append(buf, "\x1b[?25l"...)
	}
	if _, err := s.out.Write(buf); err != nil {
		return err
	}
	return s.out.Flush()
}

// WriteRaw writes bytes straight through, bypassing the diff.
func (s *Screen) WriteRaw(str string) error {
	if _, err := s.out.WriteString(str); err != nil {
		return err
	}
	return s.out.Flush()
}

func appendCursor(buf []byte, x, y int) []byte {
	buf = append(buf, "\x1b["...)
	buf = strconv.AppendInt(buf, int64(y+1), 10)
	buf = append(buf, ';')
	buf = strconv.AppendInt(buf, int64(x+1), 10)
	buf = append(buf, 'H')
	return buf
}

func appendRune(buf []byte, r rune) []byte {
	return append(buf, string(r)...)
}

func (s *Screen) appendAttr(buf []byte, a Attr) []byte {
	if s.attrSet && a == s.lastAttr {
		return buf
	}
	s.lastAttr = a
	s.attrSet = true
	buf = append(buf, "\x1b[0"...)
	if a.Bold {
		buf = append(buf, ";1"...)
	}
	if a.Dim {
		buf = append(buf, ";2"...)
	}
	if a.Italic {
		buf = append(buf, ";3"...)
	}
	if a.Underline {
		buf = append(buf, ";4"...)
	}
	if a.Blink {
		buf = append(buf, ";5"...)
	}
	if a.Reverse {
		buf = append(buf, ";7"...)
	}
	if a.FG.Set {
		buf = s.appendColor(buf, a.FG, true)
	}
	if a.BG.Set {
		buf = s.appendColor(buf, a.BG, false)
	}
	buf = append(buf, 'm')
	return buf
}

func (s *Screen) appendColor(buf []byte, c Color, fg bool) []byte {
	if s.trueColor {
		if fg {
			buf = append(buf, ";38;2;"...)
		} else {
			buf = append(buf, ";48;2;"...)
		}
		buf = strconv.AppendInt(buf, int64(c.R), 10)
		buf = append(buf, ';')
		buf = strconv.AppendInt(buf, int64(c.G), 10)
		buf = append(buf, ';')
		buf = strconv.AppendInt(buf, int64(c.B), 10)
		return buf
	}
	n := color256(c)
	if fg {
		buf = append(buf, ";38;5;"...)
	} else {
		buf = append(buf, ";48;5;"...)
	}
	buf = strconv.AppendInt(buf, int64(n), 10)
	return buf
}

func color256(c Color) int {
	if c.R == c.G && c.G == c.B {
		if c.R < 8 {
			return 16
		}
		if c.R > 248 {
			return 231
		}
		return 232 + (int(c.R)-8)*24/247
	}
	q := func(v uint8) int { return (int(v)*5 + 127) / 255 }
	return 16 + 36*q(c.R) + 6*q(c.G) + q(c.B)
}

// Rect returns a view onto part of the screen.
func (s *Screen) Rect(x, y, w, h int) Rect { return Rect{s: s, X: x, Y: y, W: w, H: h} }

// Rect is a view onto part of the screen.
type Rect struct {
	s    *Screen
	X, Y int
	W, H int
}

// Text draws inside the rectangle, clipping to its width.
func (r Rect) Text(x, y, max int, a Attr, str string) int {
	if max > r.W-x {
		max = r.W - x
	}
	return r.s.Text(r.X+x, r.Y+y, max, a, str)
}

// FillSpace fills inside the rectangle.
func (r Rect) FillSpace(x, y, w int, a Attr) {
	if x+w > r.W {
		w = r.W - x
	}
	r.s.FillSpace(r.X+x, r.Y+y, w, a)
}

// Set places a rune inside the rectangle.
func (r Rect) Set(x, y int, rn rune, a Attr) {
	if x < 0 || y < 0 || x >= r.W || y >= r.H {
		return
	}
	r.s.Set(r.X+x, r.Y+y, rn, a)
}

// Screen exposes the underlying screen.
func (r Rect) Screen() *Screen { return r.s }

// Dump renders the current canvas as plain text rows. It is meant for tests
// and debugging; continuation cells become spaces.
func (s *Screen) Dump() []string {
	out := make([]string, s.h)
	for y := 0; y < s.h; y++ {
		row := make([]rune, 0, s.w)
		for x := 0; x < s.w; x++ {
			c := s.cur[s.idx(x, y)]
			switch {
			case c.W == 0:
				row = append(row, ' ')
			case c.R == 0:
				row = append(row, ' ')
			default:
				row = append(row, c.R)
			}
		}
		out[y] = string(row)
	}
	return out
}

// CellAt returns the cell at x,y.
func (s *Screen) CellAt(x, y int) Cell {
	if x < 0 || y < 0 || x >= s.w || y >= s.h {
		return Cell{}
	}
	return s.cur[s.idx(x, y)]
}

// AttrAt returns the attributes of the cell at x,y.
func (s *Screen) AttrAt(x, y int) Attr { return s.CellAt(x, y).A }

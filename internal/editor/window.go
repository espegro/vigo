package editor

import (
	"vigo/internal/buffer"
	"vigo/internal/keys"
)

// window is one view of a buffer. The editor keeps the state of the current
// window in its own fields; switching windows copies between the two.
type window struct {
	buf        *buffer.Buffer
	cur        buffer.Pos
	desiredCol int
	topline    int
	topSeg     int
	leftcol    int

	// layout, filled in by layoutWindows
	x, y, w, h int
	textH      int
	statusH    int
}

func newWindow(b *buffer.Buffer) *window { return &window{buf: b} }

// winNode is a node in the window layout tree: either a leaf holding a window
// or a container splitting the area between its children.
type winNode struct {
	leaf bool
	win  *window
	vert bool
	kids []*winNode
	size float64
}

func leafNode(w *window) *winNode { return &winNode{leaf: true, win: w, size: 1} }

// count returns the number of windows in the subtree.
func (n *winNode) count() int {
	if n == nil {
		return 0
	}
	if n.leaf {
		return 1
	}
	total := 0
	for _, k := range n.kids {
		total += k.count()
	}
	return total
}

func (n *winNode) leaves(out []*window) []*window {
	if n == nil {
		return out
	}
	if n.leaf {
		return append(out, n.win)
	}
	for _, k := range n.kids {
		out = k.leaves(out)
	}
	return out
}

// initWindows sets up the first window for a buffer.
func (e *Editor) initWindows(b *buffer.Buffer) {
	w := newWindow(b)
	e.root = leafNode(w)
	e.win = w
	e.windows = []*window{w}
}

// findParent returns the container holding the node with this window.
func (e *Editor) findParent(n *winNode, w *window) (*winNode, int) {
	if n == nil || n.leaf {
		return nil, -1
	}
	for i, k := range n.kids {
		if k.leaf && k.win == w {
			return n, i
		}
		if p, idx := e.findParent(k, w); p != nil {
			return p, idx
		}
	}
	return nil, -1
}

func (e *Editor) findNode(n *winNode, w *window) *winNode {
	if n == nil {
		return nil
	}
	if n.leaf {
		if n.win == w {
			return n
		}
		return nil
	}
	for _, k := range n.kids {
		if found := e.findNode(k, w); found != nil {
			return found
		}
	}
	return nil
}

type sepCol struct{ x, y, h int }

// layoutWindows computes the rectangle of every window.
func (e *Editor) layoutWindows() {
	w, h := e.scr.Size()
	area := h - 1
	if area < 1 {
		area = 1
	}
	e.separators = e.separators[:0]
	e.showStatus = e.opts.LastStatus >= 2 || (e.opts.LastStatus >= 1 && e.root.count() > 1)
	e.layoutNode(e.root, 0, 0, w, area)
	e.windows = e.root.leaves(nil)
}

func (e *Editor) layoutNode(n *winNode, x, y, w, h int) {
	if n == nil {
		return
	}
	if n.leaf {
		status := 0
		if e.showStatus {
			status = 1
		}
		if h-status < 1 {
			status = 0
		}
		n.win.x, n.win.y, n.win.w, n.win.h = x, y, w, h
		n.win.statusH = status
		n.win.textH = h - status
		return
	}
	total := 0.0
	for _, k := range n.kids {
		total += k.size
	}
	if total <= 0 {
		total = float64(len(n.kids))
	}
	if n.vert {
		avail := w - (len(n.kids) - 1)
		if avail < len(n.kids) {
			avail = len(n.kids)
		}
		used := 0
		for i, k := range n.kids {
			kw := int(float64(avail) * k.size / total)
			if i == len(n.kids)-1 {
				kw = avail - used
			}
			if kw < 1 {
				kw = 1
			}
			off := used + i
			e.layoutNode(k, x+off, y, kw, h)
			used += kw
			if i < len(n.kids)-1 {
				e.separators = append(e.separators, sepCol{x: x + used + i, y: y, h: h})
			}
		}
		return
	}
	used := 0
	for i, k := range n.kids {
		kh := int(float64(h) * k.size / total)
		if i == len(n.kids)-1 {
			kh = h - used
		}
		if kh < 1 {
			kh = 1
		}
		e.layoutNode(k, x, y+used, w, kh)
		used += kh
	}
}

// saveLiveToWindow copies the editor state into the current window.
func (e *Editor) saveLiveToWindow() {
	if e.win == nil {
		return
	}
	e.win.buf = e.buf
	e.win.cur = e.cur
	e.win.desiredCol = e.desiredCol
	e.win.topline = e.topline
	e.win.topSeg = e.topSeg
	e.win.leftcol = e.leftcol
}

// useWindow makes w the current window.
func (e *Editor) useWindow(w *window) {
	if w == nil || w == e.win {
		return
	}
	e.saveLiveToWindow()
	e.loadWindowState(w)
}

func (e *Editor) loadWindowState(w *window) {
	e.win = w
	e.buf = w.buf
	e.cur = e.buf.Clamp(w.cur)
	e.desiredCol = w.desiredCol
	e.topline = w.topline
	e.topSeg = w.topSeg
	e.leftcol = w.leftcol
	e.setTitle()
}

// splitWindow splits the current window. The new window is placed above (for a
// horizontal split) or to the left (for a vertical split), like vim does.
func (e *Editor) splitWindow(vertical bool, b *buffer.Buffer) *window {
	if e.root == nil {
		e.initWindows(b)
		return e.win
	}
	old := e.win
	nw := newWindow(b)
	if b == old.buf {
		*nw = *old
		nw.buf = b
	}
	parent, idx := e.findParent(e.root, old)
	if parent != nil && parent.vert == vertical {
		parent.kids = append(parent.kids, nil)
		copy(parent.kids[idx+1:], parent.kids[idx:])
		parent.kids[idx] = leafNode(nw)
	} else {
		cont := &winNode{
			vert: vertical,
			kids: []*winNode{leafNode(nw), leafNode(old)},
			size: 1,
		}
		if parent == nil {
			e.root = cont
		} else {
			parent.kids[idx] = cont
		}
	}
	// Keep the cursor in the window the user was in, like vim does.
	e.win = old
	e.equalizeWindows()
	e.layoutWindows()
	return nw
}

// closeWindow removes a window. It returns true when that was the last one.
func (e *Editor) closeWindow(w *window) bool {
	if e.root == nil {
		return true
	}
	if e.root.leaf {
		e.root = nil
		e.win = nil
		e.windows = nil
		return true
	}
	parent, idx := e.findParent(e.root, w)
	if parent == nil {
		return false
	}
	parent.kids = append(parent.kids[:idx], parent.kids[idx+1:]...)
	if len(parent.kids) == 1 {
		e.collapse(parent)
	}
	saved := e.win
	if saved == w {
		e.saveLiveToWindow()
	}
	e.windows = e.root.leaves(nil)
	if e.win == w || e.win == nil {
		next := e.windows[0]
		if idx < len(e.windows) {
			next = e.windows[idx]
		}
		e.win = nil
		e.loadWindowState(next)
	} else {
		e.saveLiveToWindow()
	}
	return false
}

func (e *Editor) collapse(n *winNode) {
	if n == e.root {
		e.root = n.kids[0]
		return
	}
	parent, idx := e.findParentNode(e.root, n)
	if parent == nil {
		return
	}
	parent.kids[idx] = n.kids[0]
}

func (e *Editor) findParentNode(n, target *winNode) (*winNode, int) {
	if n == nil || n.leaf {
		return nil, -1
	}
	for i, k := range n.kids {
		if k == target {
			return n, i
		}
		if p, idx := e.findParentNode(k, target); p != nil {
			return p, idx
		}
	}
	return nil, -1
}

// equalizeWindows gives every window the same share, like 'equalalways'.
func (e *Editor) equalizeWindows() {
	e.root.equalize()
	e.layoutWindows()
}

func (n *winNode) equalize() {
	if n == nil || n.leaf {
		return
	}
	for _, k := range n.kids {
		k.size = 1
		k.equalize()
	}
}

// onlyWindow removes every window except w.
func (e *Editor) onlyWindow(w *window) {
	if w == nil {
		return
	}
	e.saveLiveToWindow()
	e.root = leafNode(w)
	e.win = w
	e.loadWindowState(w)
	e.equalizeWindows()
	e.layoutWindows()
}

// nextWindow moves to the next or previous window in layout order.
func (e *Editor) nextWindow(dir int) {
	if len(e.windows) < 2 {
		e.errorf("E85: There is only one window")
		return
	}
	idx := 0
	for i, w := range e.windows {
		if w == e.win {
			idx = i
		}
	}
	idx = (idx + dir + len(e.windows)) % len(e.windows)
	e.useWindow(e.windows[idx])
}

// moveWindowDir moves to the window in a direction, like ctrl-w h/j/k/l.
func (e *Editor) moveWindowDir(dir rune) {
	e.layoutWindows()
	cur := e.win
	var best *window
	bestScore := 1 << 30
	for _, w := range e.windows {
		if w == cur {
			continue
		}
		var ok bool
		switch dir {
		case 'h':
			ok = w.x+w.w <= cur.x
		case 'l':
			ok = w.x >= cur.x+cur.w
		case 'k':
			ok = w.y+w.h <= cur.y
		case 'j':
			ok = w.y >= cur.y+cur.h
		}
		if !ok {
			continue
		}
		score := 0
		switch dir {
		case 'h', 'l':
			score = abs(w.y-cur.y)*8 + abs(w.x-cur.x)
		case 'k', 'j':
			score = abs(w.x-cur.x)*8 + abs(w.y-cur.y)
		}
		if score < bestScore {
			bestScore, best = score, w
		}
	}
	if best == nil {
		e.errorf("E11: No window in that direction")
		return
	}
	e.useWindow(best)
}

// resizeWindow changes the size of the current window, like ctrl-w + and -.
func (e *Editor) resizeWindow(dir rune, n int) {
	node := e.findNode(e.root, e.win)
	parent, idx := e.findParent(e.root, e.win)
	if node == nil || parent == nil {
		e.errorf("E11: Cannot resize the last window")
		return
	}
	horizontal := dir == '+' || dir == '-'
	if parent.vert != horizontal {
		e.errorf("E11: No room to resize")
		return
	}
	delta := float64(n)
	if dir == '-' || dir == '<' {
		delta = -delta
	}
	if node.size+delta < 0.1 {
		delta = 0.1 - node.size
	}
	node.size += delta
	share := delta / float64(len(parent.kids)-1)
	for i, k := range parent.kids {
		if i == idx {
			continue
		}
		if k.size-share < 0.1 {
			share = k.size - 0.1
		}
	}
	for i, k := range parent.kids {
		if i == idx {
			continue
		}
		k.size -= share
	}
	e.layoutWindows()
}

// exchangeWindow swaps the current window with the next sibling (ctrl-w x).
func (e *Editor) exchangeWindow() {
	parent, idx := e.findParent(e.root, e.win)
	if parent == nil || len(parent.kids) < 2 {
		e.errorf("E11: No other window to exchange with")
		return
	}
	next := (idx + 1) % len(parent.kids)
	parent.kids[idx], parent.kids[next] = parent.kids[next], parent.kids[idx]
	e.layoutWindows()
}

// awaitCtrlW dispatches the key after ctrl-w.
func (e *Editor) awaitCtrlW(k keys.Key) {
	switch {
	case k.IsCtrl() && k.R == 'W':
		e.nextWindow(1)
		return
	case k.IsCtrl() && k.R == 'H':
		e.moveWindowDir('h')
		return
	case k.IsCtrl() && k.R == 'J':
		e.moveWindowDir('j')
		return
	case k.IsCtrl() && k.R == 'K':
		e.moveWindowDir('k')
		return
	case k.IsCtrl() && k.R == 'L':
		e.moveWindowDir('l')
		return
	case k.IsCtrl() && k.R == 'N':
		e.windowDirectionDown()
		return
	case k.IsCtrl() && (k.R == 'Q' || k.R == 'C'):
		e.closeCurrentWindow(false)
		return
	case k.IsCtrl() && k.R == 'O':
		e.onlyWindow(e.win)
		return
	case k.IsCtrl() && k.R == 'S':
		e.newWindowBuffer(false, "")
		return
	case k.IsCtrl() && k.R == 'V':
		e.newWindowBuffer(true, "")
		return
	case k.IsCtrl() && k.R == '=':
		e.equalizeWindows()
		return
	}
	if !k.IsRune() {
		return
	}
	switch k.R {
	case 'w':
		e.nextWindow(1)
	case 'W':
		e.nextWindow(1)
	case 'p':
		e.nextWindow(-1)
	case 'h':
		e.moveWindowDir('h')
	case 'j', 'J':
		e.moveWindowDir('j')
	case 'k', 'K':
		e.moveWindowDir('k')
	case 'l', 'L':
		e.moveWindowDir('l')
	case 't':
		if len(e.windows) > 0 {
			e.useWindow(e.windows[0])
		}
	case 'b':
		if len(e.windows) > 0 {
			e.useWindow(e.windows[len(e.windows)-1])
		}
	case 'q', 'c':
		e.closeCurrentWindow(false)
	case 'o':
		e.onlyWindow(e.win)
	case 's':
		e.newWindowBuffer(false, "")
	case 'v':
		e.newWindowBuffer(true, "")
	case 'n':
		e.newWindowBuffer(false, "")
	case '=':
		e.equalizeWindows()
	case '+':
		e.resizeWindow('+', 1)
	case '-':
		e.resizeWindow('-', 1)
	case '>':
		e.resizeWindow('>', 1)
	case '<':
		e.resizeWindow('<', 1)
	case 'x':
		e.exchangeWindow()
	case 'r':
		e.rotateWindows()
	case 'T':
		e.useWindow(e.windows[0])
	case 'B':
		e.useWindow(e.windows[len(e.windows)-1])
	}
}

func (e *Editor) windowDirectionDown() {
	if len(e.windows) > 0 {
		e.useWindow(e.windows[len(e.windows)-1])
	}
}

// closeCurrentWindow closes the current window, or quits when it is the last.
func (e *Editor) closeCurrentWindow(force bool) {
	if e.buf != nil && e.buf.Modified() && !force {
		e.errorf("E37: No write since last change (add ! to override)")
		return
	}
	if len(e.windows) <= 1 {
		if force {
			e.quit = true
		} else {
			e.errorf("E444: Cannot close last window")
		}
		return
	}
	e.closeWindow(e.win)
	e.layoutWindows()
}

// rotateWindows moves the current window to the end of the layout (ctrl-w r).
func (e *Editor) rotateWindows() {
	parent, idx := e.findParent(e.root, e.win)
	if parent == nil || len(parent.kids) < 2 {
		e.errorf("E11: No other window to rotate with")
		return
	}
	kids := append(parent.kids[:idx], parent.kids[idx+1:]...)
	parent.kids = append(kids, parent.kids[idx])
	e.layoutWindows()
}

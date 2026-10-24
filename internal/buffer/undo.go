package buffer

// maxUndoBytes caps the total size of the undo history. Every checkpoint copies
// the whole buffer, so a large file with many changes would otherwise grow
// without bound; the oldest entries are dropped first.
const maxUndoBytes = 128 << 20

// Checkpoint records the current state so the next change can be undone. The
// call is ignored while an undo block is open, which is what groups all the
// keystrokes of one insert session into a single undo step.
func (b *Buffer) Checkpoint(cur Pos) {
	if b.inBlock {
		return
	}
	b.undo = append(b.undo, b.snapshot(cur))
	b.trimUndo()
	b.redo = nil
	b.inBlock = true
}

// trimUndo enforces 'undolevels' and the memory budget.
func (b *Buffer) trimUndo() {
	if b.UndoLimit > 0 && len(b.undo) > b.UndoLimit {
		b.undo = b.undo[len(b.undo)-b.UndoLimit:]
	}
	total := 0
	for _, s := range b.undo {
		total += s.bytes
	}
	for total > maxUndoBytes && len(b.undo) > 1 {
		total -= b.undo[0].bytes
		b.undo = b.undo[1:]
	}
}

// BreakBlock ends the current undo block.
func (b *Buffer) BreakBlock() { b.inBlock = false }

// InBlock reports whether an undo block is open.
func (b *Buffer) InBlock() bool { return b.inBlock }

func (b *Buffer) snapshot(cur Pos) snapshot {
	lines := make([][]rune, len(b.lines))
	for i, l := range b.lines {
		lines[i] = append([]rune{}, l...)
	}
	size := 0
	for _, l := range lines {
		size += len(l)*4 + 24
	}
	return snapshot{lines: lines, cur: cur, missingEOL: b.missingEOL, seq: b.seq, bytes: size}
}

func (b *Buffer) restore(s snapshot) {
	b.lines = s.lines
	b.missingEOL = s.missingEOL
	b.version++
	b.seq = s.seq
}

// Undo reverts the last change and returns the cursor position to restore.
// cur is the cursor before the undo, kept for redo.
func (b *Buffer) Undo(cur Pos) (Pos, bool) {
	if len(b.undo) == 0 {
		return Pos{}, false
	}
	top := b.undo[len(b.undo)-1]
	b.undo = b.undo[:len(b.undo)-1]
	b.redo = append(b.redo, b.snapshot(cur))
	b.restore(top)
	b.inBlock = false
	return top.cur, true
}

// Redo reapplies the last undone change.
func (b *Buffer) Redo(cur Pos) (Pos, bool) {
	if len(b.redo) == 0 {
		return Pos{}, false
	}
	top := b.redo[len(b.redo)-1]
	b.redo = b.redo[:len(b.redo)-1]
	b.undo = append(b.undo, b.snapshot(cur))
	b.restore(top)
	b.inBlock = false
	return top.cur, true
}

// ClearUndo discards the whole undo history.
func (b *Buffer) ClearUndo() {
	b.undo = nil
	b.redo = nil
	b.inBlock = false
}

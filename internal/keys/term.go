package keys

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

const (
	// escTimeout is how long to wait for more bytes before treating a lone
	// escape byte as the escape key.
	escTimeout = 40 * time.Millisecond
	// seqTimeout bounds how long we wait for the rest of an escape sequence.
	seqTimeout = 200 * time.Millisecond
	// pasteTimeout bounds the gap between bytes of a bracketed paste.
	pasteTimeout = 500 * time.Millisecond
)

// ErrClosed is returned by ReadKey once the input stream is exhausted.
var ErrClosed = errors.New("input closed")

// Reader decodes raw terminal bytes into key events.
type Reader struct {
	ch         chan byte
	done       chan struct{}
	close      sync.Once
	mu         sync.Mutex
	err        error
	queue      []Key
	escTimeout time.Duration
}

// NewReader starts decoding from r. Reading happens in the background so that
// escape sequences can be told apart from a lone escape key.
func NewReader(r io.Reader) *Reader {
	rd := &Reader{ch: make(chan byte, 512), done: make(chan struct{}), escTimeout: escTimeout}
	go func() {
		defer close(rd.done)
		buf := make([]byte, 4096)
		for {
			n, err := r.Read(buf)
			for i := 0; i < n; i++ {
				rd.ch <- buf[i]
			}
			if err != nil {
				rd.mu.Lock()
				if err != io.EOF {
					rd.err = err
				}
				rd.mu.Unlock()
				return
			}
		}
	}()
	return rd
}

// Err reports a read error, if any.
func (r *Reader) Err() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.err
}

// ReadKey blocks until the next key event is available.
func (r *Reader) ReadKey() (Key, error) {
	for {
		if k, ok := r.dequeue(); ok {
			return k, nil
		}
		b, ok := r.get(0)
		if !ok {
			if err := r.Err(); err != nil {
				return Key{}, err
			}
			return Key{}, ErrClosed
		}
		var (
			k   *Key
			err error
		)
		switch {
		case b == 0x1b:
			k, err = r.parseEsc()
		case b == '\r' || b == '\n':
			k, err = ptr(Special(CR)), nil
		case b == '\t':
			k, err = ptr(Special(Tab)), nil
		case b == 0x7f || b == 0x08:
			k, err = ptr(Special(BS)), nil
		case b < 0x20:
			k, err = ptr(controlKey(b)), nil
		default:
			decoded, e := r.decodeChar(b)
			if e != nil {
				return Key{}, e
			}
			k = &decoded
		}
		if err != nil {
			return Key{}, err
		}
		if k == nil {
			continue
		}
		return *k, nil
	}
}

func ptr(k Key) *Key { return &k }

func (r *Reader) dequeue() (Key, bool) {
	if len(r.queue) == 0 {
		return Key{}, false
	}
	k := r.queue[0]
	r.queue = r.queue[1:]
	return k, true
}

// get returns the next byte, waiting up to d if d is positive.
func (r *Reader) get(d time.Duration) (byte, bool) {
	if d <= 0 {
		select {
		case b := <-r.ch:
			return b, true
		case <-r.done:
			select {
			case b := <-r.ch:
				return b, true
			default:
				return 0, false
			}
		}
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case b := <-r.ch:
		return b, true
	case <-timer.C:
		return 0, false
	case <-r.done:
		select {
		case b := <-r.ch:
			return b, true
		default:
			return 0, false
		}
	}
}

func controlKey(b byte) Key {
	switch b {
	case 0x00:
		return Ctrl('@')
	case 0x1c:
		return Ctrl('\\')
	case 0x1d:
		return Ctrl(']')
	case 0x1e:
		return Ctrl('^')
	case 0x1f:
		return Ctrl('_')
	}
	return Ctrl(rune('a' + b - 1))
}

func (r *Reader) decodeChar(b byte) (Key, error) {
	if b < utf8.RuneSelf {
		return Rune(rune(b)), nil
	}
	buf := []byte{b}
	for !utf8.FullRune(buf) && len(buf) < utf8.UTFMax {
		nb, ok := r.get(seqTimeout)
		if !ok {
			break
		}
		buf = append(buf, nb)
	}
	rn, _ := utf8.DecodeRune(buf)
	return Rune(rn), nil
}

func (r *Reader) parseEsc() (*Key, error) {
	r.mu.Lock()
	timeout := r.escTimeout
	r.mu.Unlock()
	b, ok := r.get(timeout)
	if !ok {
		return ptr(Special(Esc)), nil
	}
	switch {
	case b == '[':
		return r.parseCSI()
	case b == 'O':
		return r.parseSS3()
	case b == 0x1b:
		return ptr(Special(Esc)), nil
	case b == '\r' || b == '\n':
		k := Special(CR)
		k.Alt = true
		return &k, nil
	case b < 0x20:
		k := controlKey(b)
		k.Alt = true
		return &k, nil
	}
	k, err := r.decodeChar(b)
	if err != nil {
		return nil, err
	}
	k.Alt = true
	return &k, nil
}

func (r *Reader) parseSS3() (*Key, error) {
	b, ok := r.get(seqTimeout)
	if !ok {
		return nil, fmt.Errorf("truncated escape sequence")
	}
	switch b {
	case 'A':
		return ptr(Special(Up)), nil
	case 'B':
		return ptr(Special(Down)), nil
	case 'C':
		return ptr(Special(Right)), nil
	case 'D':
		return ptr(Special(Left)), nil
	case 'H':
		return ptr(Special(Home)), nil
	case 'F':
		return ptr(Special(End)), nil
	case 'P':
		return ptr(Special(FKey(1))), nil
	case 'Q':
		return ptr(Special(FKey(2))), nil
	case 'R':
		return ptr(Special(FKey(3))), nil
	case 'S':
		return ptr(Special(FKey(4))), nil
	}
	return nil, nil
}

func (r *Reader) parseCSI() (*Key, error) {
	params := make([]byte, 0, 16)
	for {
		b, ok := r.get(seqTimeout)
		if !ok {
			if len(params) == 0 {
				return ptr(Special(Esc)), nil
			}
			return nil, fmt.Errorf("truncated escape sequence")
		}
		if b >= 0x40 && b <= 0x7e {
			return r.dispatchCSI(string(params), b)
		}
		if b == 0x1b {
			return ptr(Special(Esc)), nil
		}
		params = append(params, b)
		if len(params) > 64 {
			return nil, fmt.Errorf("escape sequence too long")
		}
	}
}

func modsOf(param string) (alt, ctrl bool, shift bool) {
	parts := strings.Split(param, ";")
	if len(parts) < 2 {
		return false, false, false
	}
	n, err := strconv.Atoi(parts[1])
	if err != nil || n < 2 {
		return false, false, false
	}
	bits := n - 1
	return bits&2 != 0, bits&4 != 0, bits&1 != 0
}

func (r *Reader) dispatchCSI(params string, final byte) (*Key, error) {
	nums := strings.Split(params, ";")
	first := 1
	if params != "" {
		if n, err := strconv.Atoi(strings.TrimLeft(nums[0], "?<>")); err == nil {
			first = n
		}
	}
	alt, ctrl, _ := modsOf(params)
	switch final {
	case 'A', 'B', 'C', 'D', 'H', 'F':
		var name string
		switch final {
		case 'A':
			name = Up
		case 'B':
			name = Down
		case 'C':
			name = Right
		case 'D':
			name = Left
		case 'H':
			name = Home
		case 'F':
			name = End
		}
		k := Special(name)
		k.Alt, k.Ctrl = alt, ctrl
		return &k, nil
	case 'Z':
		k := Special(BTab)
		k.Alt, k.Ctrl = alt, ctrl
		return &k, nil
	case '~':
		switch first {
		case 200:
			return nil, r.readPaste()
		case 201:
			return nil, nil
		case 1, 7:
			k := Special(Home)
			k.Alt, k.Ctrl = alt, ctrl
			return &k, nil
		case 4, 8:
			k := Special(End)
			k.Alt, k.Ctrl = alt, ctrl
			return &k, nil
		case 2:
			k := Special(Ins)
			k.Alt, k.Ctrl = alt, ctrl
			return &k, nil
		case 3:
			k := Special(Del)
			k.Alt, k.Ctrl = alt, ctrl
			return &k, nil
		case 5:
			k := Special(PgUp)
			k.Alt, k.Ctrl = alt, ctrl
			return &k, nil
		case 6:
			k := Special(PgDn)
			k.Alt, k.Ctrl = alt, ctrl
			return &k, nil
		case 11, 12, 13, 14, 15:
			return ptr(Special(FKey(first - 10))), nil
		case 17, 18, 19, 20, 21:
			return ptr(Special(FKey(first - 11))), nil
		case 23:
			return ptr(Special(FKey(11))), nil
		case 24:
			return ptr(Special(FKey(12))), nil
		}
		return nil, nil
	case 'M', 'm':
		return nil, r.skipMouse(string(params), final)
	case 'u':
		rn := rune(first)
		k := Rune(rn)
		k.Alt, k.Ctrl = alt, ctrl
		if ctrl && rn >= 'a' && rn <= 'z' {
			k = Ctrl(rn)
			k.Alt = alt
		}
		return &k, nil
	}
	return nil, nil
}

func (r *Reader) skipMouse(params string, final byte) error {
	if final == 'm' {
		return nil
	}
	if strings.HasPrefix(params, "<") {
		return nil
	}
	for i := 0; i < 3; i++ {
		if _, ok := r.get(seqTimeout); !ok {
			return nil
		}
	}
	return nil
}

func (r *Reader) readPaste() error {
	const end = "\x1b[201~"
	var data []byte
	tail := make([]byte, 0, len(end))
	for {
		b, ok := r.get(pasteTimeout)
		if !ok {
			break
		}
		tail = append(tail, b)
		if len(tail) > len(end) {
			data = append(data, tail[0])
			tail = tail[1:]
		}
		if bytes.HasSuffix(tail, []byte(end)) {
			break
		}
	}
	if !bytes.HasSuffix(tail, []byte(end)) {
		data = append(data, tail...)
	}
	r.appendPaste(data)
	return nil
}

func (r *Reader) appendPaste(data []byte) {
	s := string(data)
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	for _, rn := range s {
		switch rn {
		case '\n':
			k := Special(CR)
			k.Paste = true
			r.queue = append(r.queue, k)
		case '\t':
			k := Rune('\t')
			k.Paste = true
			r.queue = append(r.queue, k)
		default:
			k := Rune(rn)
			k.Paste = true
			r.queue = append(r.queue, k)
		}
	}
}

func decodeRune(s string) (rune, int) { return utf8.DecodeRuneInString(s) }

// SetEscTimeout changes how long a lone escape byte is waited for.
func (r *Reader) SetEscTimeout(d time.Duration) {
	if d < 0 {
		d = 0
	}
	r.mu.Lock()
	r.escTimeout = d
	r.mu.Unlock()
}

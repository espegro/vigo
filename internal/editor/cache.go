package editor

import "vigo/internal/buffer"

// bufCache holds the derived data of one buffer: the syntax spans computed so
// far and the search highlight matches. The caches live per buffer because a
// split can show two different buffers at the same time.
type bufCache struct {
	synLines   []synLine
	synVersion uint64
	hlVersion  uint64
	hlMatches  []match
	hlOff      bool
}

func (e *Editor) cacheFor(b *buffer.Buffer) *bufCache {
	if e.caches == nil {
		e.caches = map[*buffer.Buffer]*bufCache{}
	}
	c, ok := e.caches[b]
	if !ok {
		c = &bufCache{}
		e.caches[b] = c
	}
	return c
}

// cache is the cache of the buffer in the current window.
func (e *Editor) cache() *bufCache { return e.cacheFor(e.buf) }

// invalidateCache drops the derived data of a buffer.
func (e *Editor) invalidateCache(b *buffer.Buffer) {
	c := e.cacheFor(b)
	c.synLines = nil
	c.synVersion = 0
	c.hlVersion = 0
	c.hlMatches = nil
}

// fileTypeFor reports the file type of a buffer.
func (e *Editor) fileTypeFor(b *buffer.Buffer) string {
	if e.fileTypes == nil {
		e.fileTypes = map[*buffer.Buffer]string{}
	}
	if ft, ok := e.fileTypes[b]; ok {
		return ft
	}
	ft := guessFileType(b.Path())
	e.fileTypes[b] = ft
	return ft
}

// fileType reports the file type of the buffer in the current window.
func (e *Editor) fileType() string { return e.fileTypeFor(e.buf) }

// setFileType sets the file type of the current buffer and throws away its
// syntax cache.
func (e *Editor) setFileType(ft string) {
	if e.fileTypes == nil {
		e.fileTypes = map[*buffer.Buffer]string{}
	}
	e.fileTypes[e.buf] = ft
	e.invalidateCache(e.buf)
}

package editor

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// rcPath returns the file with startup commands. An explicit path from -u wins,
// then $VIGO_RC, then ~/.vigorc.
func (e *Editor) rcPath(explicit string) string {
	if explicit != "" {
		return expandPath(explicit)
	}
	if env := os.Getenv("VIGO_RC"); env != "" {
		return expandPath(env)
	}
	home := userHome()
	if home == "" {
		return ""
	}
	return filepath.Join(home, ".vigorc")
}

// loadRC runs the startup file. Unknown commands are counted and reported as a
// single message, so a vimrc full of vim-only settings does not flood the
// screen.
func (e *Editor) loadRC(explicit string) {
	path := e.rcPath(explicit)
	if path == "" {
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if explicit != "" {
			e.errorf("E484: Can't open file %s", path)
		}
		return
	}
	e.scriptErrors = nil
	e.executeScript(string(data))
	if n := len(e.scriptErrors); n > 0 {
		shown := e.scriptErrors
		if len(shown) > 3 {
			shown = shown[:3]
		}
		e.startupReport = fmt.Sprintf("%s: %d line(s) not supported, for example: %s",
			filepath.Base(path), n, strings.Join(shown, "; "))
	}
	e.scriptErrors = nil
}

// executeScript runs ex commands from a file or string.
func (e *Editor) executeScript(text string) {
	e.scriptDepth++
	defer func() { e.scriptDepth-- }()
	lines := strings.Split(text, "\n")
	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "" || strings.HasPrefix(line, "\"") {
			continue
		}
		line = strings.TrimPrefix(line, ":")
		if skip, ok := blockStart(line); ok {
			i = skipBlock(lines, i, skip)
			continue
		}
		e.execEx(line)
	}
}

// blockStart reports whether a line opens a block that must be skipped
// entirely, and which keyword closes it.
func blockStart(line string) (string, bool) {
	word := line
	if i := strings.IndexAny(word, " \t("); i > 0 {
		word = word[:i]
	}
	switch word {
	case "if", "while", "for", "function", "func", "try":
		return word, true
	}
	return "", false
}

func skipBlock(lines []string, start int, kind string) int {
	closeWord := "end" + kind
	if kind == "func" {
		closeWord = "endfunction"
	}
	if kind == "try" {
		closeWord = "endtry"
	}
	depth := 0
	for i := start; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if k, ok := blockStart(line); ok && k == kind {
			depth++
			continue
		}
		word := line
		if j := strings.IndexAny(word, " \t("); j > 0 {
			word = word[:j]
		}
		if word == closeWord {
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return len(lines) - 1
}

// exSource implements :source.
func (e *Editor) exSource(args string) {
	path := strings.TrimSpace(args)
	if path == "" {
		path = e.rcPath("")
	}
	if path == "" {
		e.errorf("E484: Can't open file")
		return
	}
	expanded := expandPath(path)
	data, err := os.ReadFile(expanded)
	if err != nil {
		e.errorf("E484: Can't open file %s", path)
		return
	}
	e.scriptErrors = nil
	e.executeScript(string(data))
	if n := len(e.scriptErrors); n > 0 {
		shown := e.scriptErrors
		if len(shown) > 3 {
			shown = shown[:3]
		}
		e.messagef("%s: %d line(s) not supported, for example: %s",
			filepath.Base(expanded), n, strings.Join(shown, "; "))
	}
	e.scriptErrors = nil
}

// exLet implements the small part of :let that configuration files need.
func (e *Editor) exLet(args string) {
	name, value, ok := strings.Cut(args, "=")
	if !ok {
		return
	}
	name = strings.TrimSpace(name)
	value = strings.TrimSpace(value)
	value = unquote(value)
	switch name {
	case "mapleader", "g:mapleader":
		e.leader = value
	case "maplocalleader", "g:maplocalleader":
		e.leader = value
	}
}

func unquote(s string) string {
	if len(s) >= 2 {
		if s[0] == '\'' && s[len(s)-1] == '\'' {
			return s[1 : len(s)-1]
		}
		if s[0] == '"' && s[len(s)-1] == '"' {
			body := s[1 : len(s)-1]
			body = strings.ReplaceAll(body, "\\\\", "\\")
			body = strings.ReplaceAll(body, `\"`, `"`)
			return body
		}
	}
	return s
}

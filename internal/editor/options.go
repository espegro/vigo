package editor

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Options holds the editor settings that can be changed with :set.
type Options struct {
	Number         bool
	RelativeNumber bool
	CursorLine     bool
	CursorColumn   bool
	Wrap           bool
	List           bool
	ExpandTab      bool
	AutoIndent     bool
	Paste          bool
	TimeoutLen     int
	TTimeOutLen    int
	IgnoreCase     bool
	SmartCase      bool
	HlSearch       bool
	IncSearch      bool
	WrapScan       bool
	ShowCmd        bool
	Ruler          bool
	ShowMode       bool
	ShowMatch      bool
	Syntax         bool
	Title          bool
	TabStop        int
	ShiftWidth     int
	ScrollOff      int
	SideScrollOff  int
	LastStatus     int
	NumberWidth    int
	UndoLevels     int
	FileType       string
	ColorColumn    string
	Clipboard      string
	WhichWrap      string
	Background     string
}

// DefaultOptions returns the settings a fresh editor starts with.
func DefaultOptions() Options {
	return Options{
		Wrap:        true,
		ExpandTab:   false,
		HlSearch:    true,
		IncSearch:   true,
		WrapScan:    true,
		ShowCmd:     true,
		Ruler:       true,
		ShowMode:    true,
		ShowMatch:   true,
		Syntax:      true,
		Title:       true,
		TabStop:     8,
		ShiftWidth:  8,
		ScrollOff:   5,
		LastStatus:  2,
		NumberWidth: 4,
		UndoLevels:  1000,
		Background:  "dark",
		TimeoutLen:  1000,
		TTimeOutLen: 40,
	}
}

type optKind uint8

const (
	optBool optKind = iota
	optNumber
	optString
)

type optDef struct {
	name  string
	short string
	kind  optKind
	get   func(o *Options) string
	set   func(o *Options, s string) error
}

func boolOpt(name, short string, ptr func(o *Options) *bool) optDef {
	return optDef{name: name, short: short, kind: optBool,
		get: func(o *Options) string {
			if *ptr(o) {
				return "1"
			}
			return "0"
		},
		set: func(o *Options, s string) error {
			v, err := parseBool(s)
			if err != nil {
				return err
			}
			*ptr(o) = v
			return nil
		}}
}

func numOpt(name, short string, ptr func(o *Options) *int) optDef {
	return optDef{name: name, short: short, kind: optNumber,
		get: func(o *Options) string { return strconv.Itoa(*ptr(o)) },
		set: func(o *Options, s string) error {
			n, err := strconv.Atoi(strings.TrimSpace(s))
			if err != nil {
				return fmt.Errorf("E521: number required: %s", s)
			}
			*ptr(o) = n
			return nil
		}}
}

func strOpt(name, short string, ptr func(o *Options) *string) optDef {
	return optDef{name: name, short: short, kind: optString,
		get: func(o *Options) string { return *ptr(o) },
		set: func(o *Options, s string) error {
			*ptr(o) = s
			return nil
		}}
}

func parseBool(s string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "1", "true", "on", "yes":
		return true, nil
	case "0", "false", "off", "no":
		return false, nil
	}
	return false, fmt.Errorf("E474: invalid argument: %s", s)
}

var optDefs = []optDef{
	boolOpt("number", "nu", func(o *Options) *bool { return &o.Number }),
	boolOpt("relativenumber", "rnu", func(o *Options) *bool { return &o.RelativeNumber }),
	boolOpt("cursorline", "cul", func(o *Options) *bool { return &o.CursorLine }),
	boolOpt("cursorcolumn", "cuc", func(o *Options) *bool { return &o.CursorColumn }),
	boolOpt("wrap", "", func(o *Options) *bool { return &o.Wrap }),
	boolOpt("list", "", func(o *Options) *bool { return &o.List }),
	boolOpt("expandtab", "et", func(o *Options) *bool { return &o.ExpandTab }),
	boolOpt("autoindent", "ai", func(o *Options) *bool { return &o.AutoIndent }),
	boolOpt("paste", "", func(o *Options) *bool { return &o.Paste }),
	boolOpt("ignorecase", "ic", func(o *Options) *bool { return &o.IgnoreCase }),
	boolOpt("smartcase", "scs", func(o *Options) *bool { return &o.SmartCase }),
	boolOpt("hlsearch", "hls", func(o *Options) *bool { return &o.HlSearch }),
	boolOpt("incsearch", "is", func(o *Options) *bool { return &o.IncSearch }),
	boolOpt("wrapscan", "ws", func(o *Options) *bool { return &o.WrapScan }),
	boolOpt("showcmd", "sc", func(o *Options) *bool { return &o.ShowCmd }),
	boolOpt("ruler", "ru", func(o *Options) *bool { return &o.Ruler }),
	boolOpt("showmode", "smd", func(o *Options) *bool { return &o.ShowMode }),
	boolOpt("showmatch", "sm", func(o *Options) *bool { return &o.ShowMatch }),
	boolOpt("syntax", "syn", func(o *Options) *bool { return &o.Syntax }),
	boolOpt("title", "", func(o *Options) *bool { return &o.Title }),
	numOpt("tabstop", "ts", func(o *Options) *int { return &o.TabStop }),
	numOpt("shiftwidth", "sw", func(o *Options) *int { return &o.ShiftWidth }),
	numOpt("scrolloff", "so", func(o *Options) *int { return &o.ScrollOff }),
	numOpt("sidescrolloff", "siso", func(o *Options) *int { return &o.SideScrollOff }),
	numOpt("laststatus", "ls", func(o *Options) *int { return &o.LastStatus }),
	numOpt("numberwidth", "nuw", func(o *Options) *int { return &o.NumberWidth }),
	numOpt("undolevels", "ul", func(o *Options) *int { return &o.UndoLevels }),
	numOpt("timeoutlen", "tm", func(o *Options) *int { return &o.TimeoutLen }),
	numOpt("ttimeoutlen", "ttm", func(o *Options) *int { return &o.TTimeOutLen }),
	strOpt("filetype", "ft", func(o *Options) *string { return &o.FileType }),
	strOpt("colorcolumn", "cc", func(o *Options) *string { return &o.ColorColumn }),
	strOpt("clipboard", "cb", func(o *Options) *string { return &o.Clipboard }),
	strOpt("whichwrap", "ww", func(o *Options) *string { return &o.WhichWrap }),
	strOpt("background", "bg", func(o *Options) *string { return &o.Background }),
}

func findOpt(name string) *optDef {
	n := strings.ToLower(name)
	for i := range optDefs {
		if optDefs[i].name == n || (optDefs[i].short != "" && optDefs[i].short == n) {
			return &optDefs[i]
		}
	}
	return nil
}

// knownOptionName reports whether :set knows the name, including the options
// that are handled outside the generic table.
func knownOptionName(name string) bool {
	switch name {
	case "readonly", "ro", "filetype", "ft":
		return true
	}
	return findOpt(name) != nil
}

// setOptions implements the :set command. It returns a message to display, or
// an error describing what was wrong.
func (e *Editor) setOptions(args string) (string, error) {
	if args == "" {
		var sb strings.Builder
		for i := range optDefs {
			d := &optDefs[i]
			if d.kind == optBool && !toBool(d.get(&e.opts)) {
				continue
			}
			fmt.Fprintf(&sb, "  %s=%s\n", d.name, d.get(&e.opts))
		}
		return sb.String(), nil
	}
	if strings.TrimSpace(args) == "all" || strings.TrimSpace(args) == "all&" {
		var sb strings.Builder
		for i := range optDefs {
			d := &optDefs[i]
			fmt.Fprintf(&sb, "  %-16s %s\n", d.name, d.get(&e.opts))
		}
		return sb.String(), nil
	}
	var last string
	for _, field := range strings.Fields(args) {
		msg, err := e.setOneOption(field)
		if err != nil {
			return "", err
		}
		if msg != "" {
			last = msg
		}
	}
	return last, nil
}

func (e *Editor) setOneOption(field string) (string, error) {
	name, value := field, ""
	hasValue := false
	if i := strings.IndexByte(field, '='); i >= 0 {
		name, value, hasValue = field[:i], field[i+1:], true
	}
	query := strings.HasSuffix(name, "?")
	if query {
		name = strings.TrimSuffix(name, "?")
	}
	negated := false
	inverted := false
	switch {
	case strings.HasPrefix(name, "no") && knownOptionName(name[2:]):
		negated, name = true, name[2:]
	case strings.HasPrefix(name, "inv") && knownOptionName(name[3:]):
		inverted, name = true, name[3:]
	}
	def := findOpt(name)
	if name == "ft" || name == "filetype" {
		if query {
			return fmt.Sprintf("  filetype=%s", e.fileType()), nil
		}
		if hasValue {
			e.setFileType(value)
		}
		return "", nil
	}
	if name == "ro" || name == "readonly" {
		if query {
			return fmt.Sprintf("  readonly=%s", boolString(e.buf.ReadOnly())), nil
		}
		v := true
		switch {
		case negated:
			v = false
		case inverted:
			v = !e.buf.ReadOnly()
		case hasValue:
			parsed, err := parseBool(value)
			if err != nil {
				return "", err
			}
			v = parsed
		}
		e.buf.SetReadOnly(v)
		return "", nil
	}
	if def == nil {
		for _, extra := range extraOptionNames {
			if extra == name {
				return "", fmt.Errorf("E518: unknown option: %s (accepted but not implemented)", name)
			}
		}
		return "", fmt.Errorf("E518: unknown option: %s", name)
	}
	if query {
		return fmt.Sprintf("  %s=%s", def.name, def.get(&e.opts)), nil
	}
	switch def.kind {
	case optBool:
		var v bool
		switch {
		case negated:
			v = false
		case inverted:
			v = !toBool(def.get(&e.opts))
		case hasValue:
			parsed, err := parseBool(value)
			if err != nil {
				return "", err
			}
			v = parsed
		default:
			v = true
		}
		if err := def.set(&e.opts, boolString(v)); err != nil {
			return "", err
		}
		e.onOptionChanged(def.name)
	default:
		if !hasValue {
			return fmt.Sprintf("  %s=%s", def.name, def.get(&e.opts)), nil
		}
		if err := def.set(&e.opts, value); err != nil {
			return "", err
		}
		e.onOptionChanged(def.name)
	}
	return "", nil
}

func (e *Editor) onOptionChanged(name string) {
	switch name {
	case "undolevels":
		e.buf.UndoLimit = e.opts.UndoLevels
	case "wrap":
		if !e.opts.Wrap {
			e.topSeg = 0
			e.ensureCursorVisible()
		}
	case "syntax":
		e.invalidateCache(e.buf)
	case "ttimeoutlen":
		if e.keyReader != nil {
			e.keyReader.SetEscTimeout(time.Duration(e.opts.TTimeOutLen) * time.Millisecond)
		}
	}
}

func boolString(v bool) string {
	if v {
		return "1"
	}
	return "0"
}

func toBool(s string) bool {
	b, err := parseBool(s)
	return err == nil && b
}

// optionNames returns every option name, used for command line completion.
func optionNames() []string {
	out := make([]string, 0, len(optDefs)*2)
	out = append(out, "readonly", "ro")
	for i := range optDefs {
		out = append(out, optDefs[i].name)
		if optDefs[i].short != "" {
			out = append(out, optDefs[i].short)
		}
	}
	return out
}

// extraOptionNames are accepted by :set but have no effect yet.
var extraOptionNames = []string{
	"compatible", "nocompatible", "backup", "writebackup", "swapfile",
	"magic", "wrapscan", "wildmenu", "paste", "modifiable",
	"textwidth", "softtabstop", "smartindent", "cindent", "linebreak",
	"listchars", "matchtime", "report", "shortmess", "hidden", "autowrite",
	"shell", "shellcmdflag", "encoding", "fileencoding", "termguicolors",
	"signcolumn", "matchpairs", "selection", "virtualedit", "mouse",
}

// guessFileType maps a file name to a syntax name.
func guessFileType(path string) string {
	base := strings.ToLower(filepath.Base(path))
	switch base {
	case "makefile", "gnumakefile":
		return "make"
	case "go.mod", "go.sum":
		return "gomod"
	case "dockerfile":
		return "dockerfile"
	}
	switch strings.ToLower(filepath.Ext(path)) {
	case ".go":
		return "go"
	case ".c", ".h", ".cc", ".cpp", ".cxx", ".hpp", ".hh":
		return "c"
	case ".py", ".pyw":
		return "python"
	case ".sh", ".bash", ".zsh", ".ksh":
		return "sh"
	case ".js", ".mjs", ".cjs", ".jsx":
		return "javascript"
	case ".ts", ".tsx":
		return "typescript"
	case ".rs":
		return "rust"
	case ".json":
		return "json"
	case ".yaml", ".yml":
		return "yaml"
	case ".toml":
		return "toml"
	case ".html", ".htm":
		return "html"
	case ".css":
		return "css"
	case ".md", ".markdown":
		return "markdown"
	case ".java":
		return "java"
	case ".rb":
		return "ruby"
	case ".lua":
		return "lua"
	case ".sql":
		return "sql"
	case ".vim":
		return "vim"
	}
	return ""
}

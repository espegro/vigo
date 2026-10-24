package editor

import "strings"

type syntaxGroup uint8

// Syntax highlight groups.
const (
	groupNone syntaxGroup = iota
	groupComment
	groupString
	groupNumber
	groupKeyword
	groupType
	groupFunc
	groupPreproc
	groupConstant
	groupSpecial
)

type syntaxSpan struct {
	start, end int
	group      syntaxGroup
}

type lexState struct {
	blockComment bool
	rawString    bool
	rawQuote     rune
	rawTriple    bool
}

type langDef struct {
	name        string
	lineComment string
	blockOpen   string
	blockClose  string
	preproc     string
	keywords    map[string]bool
	types       map[string]bool
	constants   map[string]bool
	quotes      string
	tripleQuote bool
	rawQuote    rune
	identExtra  string
}

var langs = map[string]*langDef{}

func makeWords(s string) map[string]bool {
	m := map[string]bool{}
	for _, w := range strings.Fields(s) {
		m[w] = true
	}
	return m
}

func init() {
	addLang(&langDef{
		name:        "go",
		lineComment: "//",
		blockOpen:   "/*",
		blockClose:  "*/",
		quotes:      "\"'",
		rawQuote:    '`',
		keywords: makeWords(`break case chan const continue default defer else fallthrough for
			func go goto if import interface map package range return select struct switch type var`),
		types: makeWords(`bool byte complex64 complex128 error float32 float64 int int8 int16 int32 int64
			rune string uint uint8 uint16 uint32 uint64 uintptr any comparable`),
		constants: makeWords(`true false iota nil`),
	})
	addLang(&langDef{
		name:        "c",
		lineComment: "//",
		blockOpen:   "/*",
		blockClose:  "*/",
		preproc:     "#",
		quotes:      "\"'",
		keywords: makeWords(`alignas alignof auto break case const constexpr continue default do else enum
			extern for goto if inline register restrict return sizeof static struct switch typedef union
			volatile while class namespace template public private protected virtual override new delete
			try catch throw using operator this friend explicit mutable typename const_cast dynamic_cast
			static_cast reinterpret_cast public private protected throw noexcept decltype concurrency co_await
			co_return co_yield concept requires consteval constinit`),
		types: makeWords(`bool char char8_t char16_t char32_t double float int long short signed unsigned void
			wchar_t size_t ssize_t int8_t int16_t int32_t int64_t uint8_t uint16_t uint32_t uint64_t
			std string vector map set auto nullptr true false`),
		constants: makeWords(`NULL nullptr true false`),
	})
	addLang(&langDef{
		name:        "java",
		lineComment: "//",
		blockOpen:   "/*",
		blockClose:  "*/",
		quotes:      "\"'",
		keywords: makeWords(`abstract assert break case catch class const continue default do else enum extends
			final finally for goto if implements import instanceof interface native new package private
			protected public return static strictfp super switch synchronized this throw throws transient try
			volatile while var record sealed permits yield non-sealed`),
		types: makeWords(`boolean byte char double float int long short void String Integer Boolean Character
			Double Float Long Short Object List Map Set ArrayList HashMap`),
		constants: makeWords(`true false null`),
	})
	addLang(&langDef{
		name:        "javascript",
		lineComment: "//",
		blockOpen:   "/*",
		blockClose:  "*/",
		quotes:      "\"'",
		rawQuote:    '`',
		keywords: makeWords(`async await break case catch class const continue debugger default delete do else
			export extends finally for function if import in instanceof let new return static super switch
			this throw try typeof var void while with yield of type interface enum implements private public
			protected readonly declare namespace abstract as satisfies`),
		types: makeWords(`Array Boolean Date Error Function JSON Map Math Number Object Promise RegExp Set String
			Symbol WeakMap WeakSet any void never unknown string number boolean bigint symbol object`),
		constants: makeWords(`true false null undefined NaN Infinity this super`),
	})
	addLang(&langDef{
		name:        "rust",
		lineComment: "//",
		blockOpen:   "/*",
		blockClose:  "*/",
		quotes:      "\"'",
		keywords: makeWords(`as async await break const continue crate dyn else enum extern false fn for if impl
			in let loop match mod move mut pub ref return self Self static struct super trait true type unsafe
			use where while union`),
		types: makeWords(`bool char f32 f64 i8 i16 i32 i64 i128 isize str u8 u16 u32 u64 u128 usize String Vec
			Option Result Box Rc Arc HashMap HashSet`),
		constants: makeWords(`true false None Some Ok Err`),
	})
	addLang(&langDef{
		name:        "python",
		lineComment: "#",
		quotes:      "\"'",
		tripleQuote: true,
		preproc:     "@",
		keywords: makeWords(`and as assert async await break class continue def del elif else except finally for
			from global if import in is lambda nonlocal not or pass raise return try while with yield match case`),
		types:     makeWords(`bool bytes complex dict float frozenset int list object set str tuple type`),
		constants: makeWords(`True False None self cls NotImplemented Ellipsis`),
	})
	addLang(&langDef{
		name:        "ruby",
		lineComment: "#",
		quotes:      "\"'",
		keywords: makeWords(`alias and begin break case class def defined? do else elsif end ensure for if in module
			next not or redo rescue retry return self super then undef unless until when while yield require
			attr_accessor attr_reader attr_writer puts lambda proc`),
		types:     makeWords(`Array Hash String Integer Float Symbol Range Struct Proc IO File Dir`),
		constants: makeWords(`true false nil self`),
	})
	addLang(&langDef{
		name:        "lua",
		lineComment: "--",
		quotes:      "\"'",
		keywords: makeWords(`and break do else elseif end false for function goto if in local nil not or repeat
			return then true until while`),
		types:     makeWords(`string table math io os coroutine debug`),
		constants: makeWords(`true false nil`),
	})
	addLang(&langDef{
		name:        "sh",
		lineComment: "#",
		quotes:      "\"'",
		rawQuote:    '`',
		keywords: makeWords(`if then else elif fi case esac for while until do done in function select time coproc
			return break continue local export readonly declare typeset unset shift source eval exec trap set`),
		types:     makeWords(`alias bg bind builtin caller cd command compgen complete compopt dirs disown echo enable help history jobs kill let logout mapfile popd pushd pwd read readarray shopt suspend test times type ulimit umask unalias wait printf true false`),
		constants: makeWords(`$# $@ $* $? $$ $!`),
	})
	addLang(&langDef{
		name:        "sql",
		lineComment: "--",
		blockOpen:   "/*",
		blockClose:  "*/",
		quotes:      "'\"",
		keywords: makeWords(`select from where insert into values update set delete create table alter drop index
			view join inner left right full outer on group by order having limit offset union all distinct as
			and or not null is in between like exists primary key foreign references default cascade constraint
			begin commit rollback transaction with case when then else end`),
		types:     makeWords(`int integer bigint smallint serial text varchar char boolean date timestamp time numeric decimal real double uuid json jsonb bytea`),
		constants: makeWords(`true false null`),
	})
	addLang(&langDef{
		name:        "vim",
		lineComment: `"`,
		quotes:      "'",
		keywords: makeWords(`if else elseif endif for endfor while endwhile function endfunction return let unlet
			set setlocal setglobal call echo echom execute normal map noremap nmap nnoremap vmap inoremap
			augroup autocmd try catch finally throw silent verbose`),
		constants: makeWords(`true false v:true v:false v:null`),
	})
	addLang(&langDef{
		name:        "make",
		lineComment: "#",
		quotes:      "\"'",
		keywords:    makeWords(`ifeq ifneq ifdef ifndef else endif include export unexport define endef override .PHONY`),
	})
	addLang(&langDef{
		name:      "json",
		quotes:    "\"",
		keywords:  makeWords(``),
		constants: makeWords(`true false null`),
	})
	addLang(&langDef{
		name:        "yaml",
		lineComment: "#",
		quotes:      "\"'",
		constants:   makeWords(`true false null yes no on off ~`),
	})
	addLang(&langDef{
		name:        "toml",
		lineComment: "#",
		quotes:      "\"'",
		constants:   makeWords(`true false`),
	})
	addLang(&langDef{
		name:       "html",
		blockOpen:  "<!--",
		blockClose: "-->",
		quotes:     "\"'",
	})
	addLang(&langDef{
		name:        "css",
		blockOpen:   "/*",
		blockClose:  "*/",
		lineComment: "",
		quotes:      "\"'",
		keywords: makeWords(`important media supports keyframes font-face import charset page namespace
			px em rem vh vw pt pc cm mm in deg rad grad turn ms s hz khz dpi dpcm dppx`),
	})
	addLang(&langDef{
		name:        "markdown",
		quotes:      "\"'`",
		rawQuote:    '`',
		lineComment: "",
	})
	addLang(&langDef{
		name:        "gomod",
		lineComment: "//",
		quotes:      "\"`",
		keywords:    makeWords(`module go require replace exclude retract toolchain use`),
	})
	addLang(&langDef{
		name:        "dockerfile",
		lineComment: "#",
		quotes:      "\"'",
		keywords: makeWords(`FROM RUN CMD LABEL MAINTAINER EXPOSE ENV ADD COPY ENTRYPOINT VOLUME USER WORKDIR
			ARG ONBUILD STOPSIGNAL HEALTHCHECK SHELL AS`),
	})
}

func addLang(l *langDef) { langs[l.name] = l }

// syntaxSpans returns the highlight spans for a line, computing the lexer state
// forward from the top of the buffer when needed.
func (e *Editor) syntaxSpans(line int) []syntaxSpan {
	if !e.opts.Syntax || e.buf.Len() > 50000 {
		return nil
	}
	c := e.cache()
	if c.synVersion != e.buf.Version() {
		c.synLines = c.synLines[:0]
		c.synVersion = e.buf.Version()
	}
	for len(c.synLines) <= line && len(c.synLines) < e.buf.Len() {
		i := len(c.synLines)
		var state lexState
		if i > 0 {
			state = c.synLines[i-1].state
		}
		spans, next := e.highlightLine(i, state)
		c.synLines = append(c.synLines, synLine{spans: spans, state: next})
	}
	if line < len(c.synLines) {
		return c.synLines[line].spans
	}
	return nil
}

type synLine struct {
	spans []syntaxSpan
	state lexState
}

func (e *Editor) lang() *langDef {
	ft := e.fileType()
	if ft == "" {
		return nil
	}
	return langs[ft]
}

func (e *Editor) highlightLine(idx int, state lexState) ([]syntaxSpan, lexState) {
	lang := e.lang()
	if lang == nil {
		return nil, state
	}
	line := e.buf.Line(idx)
	var spans []syntaxSpan
	add := func(start, end int, g syntaxGroup) {
		if end > start {
			spans = append(spans, syntaxSpan{start: start, end: end, group: g})
		}
	}
	i := 0
	n := len(line)
	if state.blockComment {
		end := indexRunes(line, 0, []rune(lang.blockClose))
		if end < 0 {
			add(0, n, groupComment)
			return spans, state
		}
		add(0, end+len([]rune(lang.blockClose)), groupComment)
		i = end + len([]rune(lang.blockClose))
		state.blockComment = false
	}
	if state.rawString {
		seq := []rune{state.rawQuote}
		if state.rawTriple {
			seq = []rune{state.rawQuote, state.rawQuote, state.rawQuote}
		}
		end := indexRunes(line, i, seq)
		if end < 0 {
			add(i, n, groupString)
			return spans, state
		}
		add(i, end+len(seq), groupString)
		i = end + len(seq)
		state.rawString = false
		state.rawTriple = false
	}
	for i < n {
		r := line[i]
		if r == ' ' || r == '\t' {
			i++
			continue
		}
		if lang.lineComment != "" && hasPrefixAt(line, i, lang.lineComment) {
			add(i, n, groupComment)
			break
		}
		if lang.blockOpen != "" && hasPrefixAt(line, i, lang.blockOpen) {
			open := len([]rune(lang.blockOpen))
			end := indexRunes(line, i+open, []rune(lang.blockClose))
			if end < 0 {
				add(i, n, groupComment)
				state.blockComment = true
				break
			}
			add(i, end+len([]rune(lang.blockClose)), groupComment)
			i = end + len([]rune(lang.blockClose))
			continue
		}
		if lang.preproc != "" && i == firstNonSpace(line) && strings.ContainsRune(lang.preproc, r) {
			add(i, n, groupPreproc)
			break
		}
		if lang.name == "markdown" {
			if r == '#' {
				add(i, n, groupComment)
				break
			}
			if r == '>' && firstNonSpace(line) == i {
				add(i, n, groupString)
				break
			}
			i++
			continue
		}
		if strings.ContainsRune(lang.quotes, r) {
			if lang.tripleQuote && i+2 < n && line[i+1] == r && line[i+2] == r {
				seq := []rune{r, r, r}
				end := indexRunes(line, i+3, seq)
				if end < 0 {
					add(i, n, groupString)
					state.rawString = true
					state.rawQuote = r
					state.rawTriple = true
					break
				}
				add(i, end+3, groupString)
				i = end + 3
				continue
			}
			j := i + 1
			closed := false
			for j < n {
				if line[j] == '\\' {
					j += 2
					continue
				}
				if line[j] == r {
					closed = true
					j++
					break
				}
				j++
			}
			if !closed && lang.rawQuote == r {
				add(i, n, groupString)
				state.rawString = true
				state.rawQuote = r
				break
			}
			add(i, j, groupString)
			i = j
			continue
		}
		if lang.rawQuote != 0 && r == lang.rawQuote {
			seq := []rune{r}
			end := indexRunes(line, i+1, seq)
			if end < 0 {
				add(i, n, groupString)
				state.rawString = true
				state.rawQuote = r
				break
			}
			add(i, end+1, groupString)
			i = end + 1
			continue
		}
		if r >= '0' && r <= '9' {
			j := i
			for j < n && (line[j] >= '0' && line[j] <= '9' || line[j] >= 'a' && line[j] <= 'f' ||
				line[j] >= 'A' && line[j] <= 'F' || line[j] == 'x' || line[j] == 'X' || line[j] == '.' ||
				line[j] == '_' || line[j] == 'e' || line[j] == '+') {
				j++
			}
			add(i, j, groupNumber)
			i = j
			continue
		}
		if isIdentStart(r) {
			j := i
			for j < n && (isIdentRune(line[j]) || strings.ContainsRune(lang.identExtra, line[j])) {
				j++
			}
			word := string(line[i:j])
			k := j
			for k < n && (line[k] == ' ' || line[k] == '\t') {
				k++
			}
			group := syntaxGroup(groupNone)
			switch {
			case lang.keywords[word]:
				group = groupKeyword
			case lang.types[word]:
				group = groupType
			case lang.constants[word]:
				group = groupConstant
			case k < n && line[k] == '(':
				group = groupFunc
			}
			if group != groupNone {
				add(i, j, group)
			}
			i = j
			continue
		}
		i++
	}
	return spans, state
}

func firstNonSpace(line []rune) int {
	for i, r := range line {
		if r != ' ' && r != '\t' {
			return i
		}
	}
	return len(line)
}

func isIdentStart(r rune) bool {
	return r == '_' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r > 0x7f
}

func isIdentRune(r rune) bool {
	return isIdentStart(r) || r >= '0' && r <= '9'
}

func hasPrefixAt(line []rune, i int, prefix string) bool {
	p := []rune(prefix)
	if i+len(p) > len(line) {
		return false
	}
	for k, r := range p {
		if line[i+k] != r {
			return false
		}
	}
	return true
}

func indexRunes(line []rune, from int, needle []rune) int {
	if len(needle) == 0 || from >= len(line) {
		return -1
	}
	for i := from; i+len(needle) <= len(line); i++ {
		ok := true
		for k, r := range needle {
			if line[i+k] != r {
				ok = false
				break
			}
		}
		if ok {
			return i
		}
	}
	return -1
}

func spanGroupAt(spans []syntaxSpan, col int) (syntaxGroup, bool) {
	for _, s := range spans {
		if col >= s.start && col < s.end {
			return s.group, true
		}
		if s.start > col {
			break
		}
	}
	return groupNone, false
}

func matchAt(matches []match, col int) (match, bool) {
	for _, m := range matches {
		if col >= m.start && col < m.end {
			return m, true
		}
		if m.start > col {
			break
		}
	}
	return match{}, false
}

# vigo

A vi/vim clone written in Go. It aims to reproduce the *standard* vi/vim
behaviour closely: modes, motions, operators, text objects, registers, undo,
macros, marks, search, substitute and the ex command line.

```
go build -o vigo .
./vigo file.go
```

or

```
go run . file.go
```

Requires Go 1.25+ and `golang.org/x/term`. Any ANSI/UTF-8 terminal with 24-bit
colour or 256 colours works.

## Platforms

vigo builds for Linux, macOS and Windows, on amd64 and arm64:

```
GOOS=windows GOARCH=amd64 go build .
GOOS=darwin  GOARCH=arm64 go build .
```

The terminal layer only uses `golang.org/x/term`, so raw mode, the alternate
screen and the escape sequences work wherever the terminal understands them.
On Windows that means Windows Terminal or a Windows 10+ console: vigo asks for
`ENABLE_VIRTUAL_TERMINAL_PROCESSING` on startup when it is not already on.
Windows has no window change signal, so the size of the terminal is polled
four times a second there, while Unix gets `SIGWINCH`.

`:!`, the `!` filter and `:r !cmd` run `cmd /c` on Windows and `sh -c`
elsewhere, and new files use CRLF line endings on Windows, like vim's
`fileformat`. Files that already use CRLF keep them on any platform.

The Windows and macOS builds are cross-compiled and vetted, but vigo has only
been run and tested on Linux and Windows so far; the macOS build has not been
run yet.

What was verified on a real Windows 11 machine (Go 1.24.3, amd64):

* `go test ./...` passes, including the platform test that checks CRLF line
  endings, resize polling and the `cmd /c` shell
* a full editing session over an SSH pseudo console: the buffer is drawn, the
  statusline and line numbers work, `:set nu`, `:w` and `:q` do what they
  should
* a new file is written with CRLF and no trailing newline, like vim

Windows consoles that refuse `ENABLE_VIRTUAL_TERMINAL_INPUT` (pseudo consoles
used by ssh sessions do) still get plain raw mode, and vigo says so on the last
line at startup instead of failing to start.

Building from source needs Go 1.25 or newer, because `golang.org/x/term`
requires it. To build with an older toolchain, ask for older dependencies:

```
go get golang.org/x/term@v0.19.0 golang.org/x/sys@v0.19.0
```

That combination was used to run the test suite on Go 1.24.3.

## Command line

| argument | meaning |
| --- | --- |
| `file ...` | files to edit; each becomes a buffer |
| `+N` | start at line N |
| `+cmd` / `-c cmd` | run an ex command after loading |
| `-u file` | use `file` instead of `~/.vigorc` (`-u NONE` skips it) |
| `--norc` | do not read the startup file |
| `--help`, `--version` | the usual |

## Configuration

At startup vigo reads `$VIGO_RC`, or `~/.vigorc` when that is not set (unless
`-u NONE` or `--norc` is given). The file holds ex commands, one per line, with
`"` starting a comment:

```vim
" ~/.vigorc
set number
set shiftwidth=4
set expandtab
let mapleader = ","
nnoremap <leader>w :w<CR>
nnoremap <leader>q :q<CR>
inoremap jk <Esc>
```

`set`, `let` (for `mapleader`), the whole `:map` family and `:source` are
implemented. Lines that use features vigo does not have are skipped, and
`if`/`while`/`for`/`function`/`try` blocks are skipped as a whole so their
bodies never run; vigo reports how many lines it ignored.

### Key mappings

`map`, `nmap`, `vmap`, `xmap`, `smap`, `omap`, `imap`, `lmap`, `cmap` and every
`noremap` variant, plus `:unmap`, `:nunmap`, ... and `:mapclear`. The usual
attributes work: `<silent>`, `<buffer>`, `<nowait>` and `<script>`; `<expr>` is
rejected. `<leader>` and `<localleader>` are expanded when the mapping is
defined, `<Nop>` does nothing, and both sides accept the full key notation
(`<CR>`, `<Esc>`, `<C-x>`, `<Tab>`, `<Space>`, `<Bslash>`, `<Bar>`, ...).

Mappings without `nore` are expanded recursively, `noremap` variants are not,
and a prefix that could still grow into a mapping waits `timeoutlen`
milliseconds before the built in command runs. Unlike vim, when both a complete
mapping and a longer one match, the complete one runs immediately instead of
waiting for the longer one.

## What is implemented

### Modes

Normal, Insert, Replace (`R`), Visual (char/line/block), Command-line and
Search. The mode is shown on the last line, the cursor is a bar in insert mode
and a block otherwise.

### Motions

`h j k l`, arrows, `w W b B e E`, `0 ^ $`, `gg G`, `{ } ( )`, `f t F T ; ,`,
`%` (matching bracket), `| _ + - <CR> <BS>`, `H M L`, `n N * #`, marks
(`` `a `` and `'a`), `ctrl-o`/`ctrl-i` jump list, `ctrl-d`/`ctrl-u`/`ctrl-f`/
`ctrl-b`/`ctrl-e`/`ctrl-y` scrolling, `zz zt zb zh zl zs ze z.`.

Counts work everywhere: `3w`, `2d3w`, `4j`, `2G`.

### Operators

`d`, `y`, `c`, `>`, `<`, `=`, `g~`, `gu`, `gU`, `gq`, and the linewise
doubles `dd yy cc >> << ==`. Plus `x X s S C D Y r R ~ J gJ`, `p P` with
counts, and `.` which repeats the last change (a count given to `.` replaces
the count of the repeated command).

### Text objects

`iw aw iW aW`, `i" a" i' a' i` a`` `, `i( a( ib ab i[ a[ i{ a{ iB aB i< a<`,
`ip ap`, `is as`.

### Insert mode

`i I a A o O`, `ctrl-w`, `ctrl-u`, `ctrl-t`, `ctrl-d`, `ctrl-r{reg}`,
`ctrl-a`, `ctrl-v`, `ctrl-o`, `ctrl-n`/`ctrl-p` (buffer word completion),
`ctrl-e`/`ctrl-y`, `opt/autoindent`, `expandtab`/`tabstop`, bracketed paste.
`:set paste` disables autoindent and tab expansion while pasting.

### Visual mode

`v V ctrl-v` with `o`, `O`, `I`/`A` (block insert/append), `d x y c s p P`,
`~ U u`, `> < =`, `r`, `J`, `gJ`, `gu gU g~ gq`, `gv` to reselect, and `:`
which starts the command line with `'<,'>`.

In block mode `$` extends the selection to the end of every line, so the
classic `ctrl-v jj $ A` appends to a ragged column of lines.

### Windows and splits

`:sp [file]`, `:vs [file]`, `:new`, `:vnew`, `:close`, `:only`, and `:q` which
closes the current window and only quits once the last one is closed. The
`ctrl-w` prefix supports `w`/`ctrl-w` (next), `p` (previous), `h j k l` (move),
`t`/`b` (first/last), `s`/`v` (split), `n` (new), `q`/`c` (close), `o` (only),
`=` (equalize), `+`/`-`/`>`/`<` (resize), `x` (exchange) and `r` (rotate).

Every window keeps its own cursor, scroll position and view of the buffer,
shows its own statusline, and a `│` column divides side by side windows.
Buffers are shared, so an edit in one window shows up in the others, and a
split can show a different file with its own file type and highlighting.

### Re-indenting and filters

`=` re-indents the lines covered by a motion or a selection, so `gg=G` fixes the
indentation of a whole file. There is a built in indenter for the C family and
for Python (vim does this through 'cindent'/'indentexpr'/'equalprg'); other
file types are left alone, just like vim without those options.

`:{range}!{cmd}` and `!{motion}{cmd}` pipe lines through a shell command and
replace them with its output, so `:%!sort`, `:2,5!column -t` and `!}fmt` work.

### Registers

Unnamed `"`, numbered `0-9`, small delete `-`, named `a-z` (uppercase appends),
black hole `_`, last inserted text `.`, and `+`/`*` which are sent to the
terminal clipboard with OSC 52 when written.

### Undo

`u` and `ctrl-r` with counts. Every normal mode command is one undo block; a
whole insert session collapses into one block.

### Macros and marks

`q{reg}`/`q` to record, `@{reg}`/`@@` with counts to play back, `m{a-z}` to set
marks and `` ` ``/`'` to jump to them.

Other navigation helpers: `g;`/`g,` walk the change list, `ctrl-o`/`ctrl-i`
walk the jump list, `gf` opens the file under the cursor and `g ctrl-g` shows
the word, character and byte counts.

### Search

`/`, `?`, `n`, `N`, `*`, `#`, `g*`, `g#`, `incsearch` preview while typing,
`hlsearch` highlighting, `:noh` to switch it off, `wrapscan`, `ignorecase` and
`smartcase`. Patterns use vim syntax (magic, `\v \m \M \V`, `\c \C`, `\< \>`,
`\d \w \s \a \l \u`, `[[:alpha:]]`, ...).

### Substitute

`:s/pat/rep/flags` with ranges (`:5,10s`, `:%s`, `:'<,'>s`), flags
`g i I c n e`, `&` and `\1`-`\9` in the replacement, the case switches
`\U \L \u \l \E`, `\r` to split lines, `:&` and `:~` to repeat, and an
interactive confirm prompt for the `c` flag.

### ex commands

`:w :wq :x :q :q! :qa :wa :e :r :ls :b :bn :bp :bd :n :prev :rew :set :noh
:s :& :~ :d :m :t :j :p :pu :normal :g :v :sort :u :redo :marks :registers
:pwd :cd :file :syntax :! :sp :vs :new :vnew :close :only :retab :version
:help` and `:N` to jump to a line.

`:set` supports (with the usual abbreviations):

```
number relativenumber cursorline wrap list expandtab autoindent ignorecase
smartcase hlsearch incsearch showcmd ruler showmode showmatch syntax title
wrapscan tabstop shiftwidth scrolloff sidescrolloff laststatus numberwidth
undolevels filetype colorcolumn clipboard whichwrap background cursorcolumn
paste
```

Several more names are accepted but do nothing yet.

### Rendering

A cell based double buffered renderer: only changed cells are written to the
terminal, wide (CJK/emoji) characters take two cells, tabs are expanded,
`set list` shows `^I` and `$`, lines wrap or scroll horizontally with `@` and
`<` markers, syntax highlighting is lexer based (Go, C/C++, Java, JavaScript,
TypeScript, Rust, Python, Ruby, Lua, shell, SQL, vim, make, JSON, YAML, TOML,
HTML, CSS, Markdown, go.mod, Dockerfile) and the theme adapts to `background`.

### Unicode

The buffer stores runes, not bytes: positions, columns, syntax spans and search
results are all rune indexed, and only the last layer (the terminal) thinks in
cell columns. `screen.RuneWidth` reports two cells for the East Asian wide and
fullwidth blocks plus the emoji ranges, one for everything else and zero for
combining marks. Input is decoded as UTF-8 and Go's regexp works on bytes, so
the search and substitute code converts between byte and rune offsets.
`internal/editor/unicode_test.go` covers this.

Known gaps:

* Combining marks are measured as zero width but not rendered (a cell holds one
  rune), so `e` + U+0301 displays as `e`.
* Deletion and motions work on runes, not grapheme clusters: `x` after `e` +
  U+0301 removes the accent, and ZWJ emoji sequences can be split.
* East Asian *ambiguous* width characters are always treated as one cell (no
  `ambiwidth` option) and right-to-left scripts are drawn in logical order.
* Everything above U+007F counts as a word character (there is no `iskeyword`
  option yet), so `w` treats NBSP and `«` as word characters.
* Files are assumed to be UTF-8; there is no `fileencoding` support.

## Testing

```
go test ./...
```

`STATUS.md` has the current state of the project: what is verified, what was
fixed in the last review pass, deliberate differences from vim and the open
limitations.

`internal/buffer`, `internal/keys` and `internal/editor` have unit tests. The
editor tests drive the real key handling (`feedKeys`) and assert on buffer
contents, cursor positions and the rendered screen, which is how vim's own test
suite works.

## Architecture

```
main.go                 command line entry point
internal/keys           key events, key notation, terminal byte decoder
internal/screen         cell buffer, diff renderer, raw mode, escape helpers
internal/buffer         lines, positions, undo history, search helpers
internal/editor         modes, motions, operators, ex commands, view
internal/editor/syntax  lexer based syntax highlighting
```

## Not implemented (yet)

* `<expr>` mappings, `:autocmd`, `:highlight`/`:colorscheme` and `:function`
  definitions; a vimrc that relies on them works only partially
* Undo tree (`:earlier`, `:later`, persistent undo, `g-`)
* Tags, `ctrl-]`, completion plugins, language server integration
* Folds, spell checking, abbreviations, digraphs
* `cindent`, `cinoptions`, `indentexpr` and `equalprg`: the built in indenter
  that `=` uses is a heuristic, and those option names are accepted but ignored
* `\zs`, `\ze`, back references and the more exotic vim regexp atoms
* Reading from the system clipboard (writing with OSC 52 works)
* `:global` with multiple commands, `:argdo`/`:bufdo`/`:windo`
* Mouse support, `virtualedit`, `clipboard=unnamed`, session files
* Windows specifics that vim has and vigo does not: `fileformat` beyond CRLF
  detection, `shellslash`, console `conpty` edge cases and testing on a real
  Windows terminal

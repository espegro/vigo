# vigo status

A vi/vim clone written in Go. This document is the result of a full code review
pass plus everything that has been verified so far: what works, what was broken
and is now fixed, what is deliberately different from vim, and what is still
missing.

## Build and run

```
go build -o vigo .
./vigo file.go
go test ./...
```

Requires Go 1.25+ (because of `golang.org/x/term`); see *Platforms* in
`README.md` for building with an older toolchain. Builds for Linux, macOS and
Windows on amd64/arm64.

## Architecture

| Package | Lines | Purpose |
| --- | --- | --- |
| `main.go` | 51 | argument handling, usage |
| `internal/keys` | 837 | key events, key notation, terminal byte decoder |
| `internal/screen` | 724 | cell buffer, diff renderer, raw mode, escape helpers |
| `internal/buffer` | 553 | lines as runes, positions, undo history |
| `internal/editor` | 10305 | modes, motions, operators, ex commands, view |

Roughly 12 500 lines of code and 2 300 lines of tests (102 test functions).

The editor keeps a single event loop. Keys arrive from a reader goroutine
through a channel, are resolved against the key mappings, and are then handed to
the mode specific handlers. Everything the editor knows about the terminal is
behind `internal/screen`.

## Feature status

Implemented and exercised by tests or terminal runs:

* modes: normal, insert, replace, visual (char/line/block), command line, search
* motions with counts, operators (`d y c > < = g~ gu gU gq` and the linewise
  doubles), text objects, registers, undo/redo blocks, macros, marks, jump list,
  change list, `.` repeat, `gf`, `g ctrl-g`, `%`, `g;`/`g,`
* search and substitute with vim style patterns, `incsearch`, `hlsearch`,
  `:noh`, smartcase, `\v \m \M \V`, `\1`-`\9`, `&`, case switches, `c` flag
* ex commands: `w wq x q q! qa wa e enew r ls b bn bp bd n prev rew set noh s &
  ~ d m t j p pu normal g v sort u redo marks registers pwd cd file syntax
  source let ! sp vs new vnew close only retab version help` and `:N`
* windows and splits with `ctrl-w` navigation, resizing, rotation, exchange
* key mappings (`map`, `nmap`, ..., `unmap`, `mapclear`, `<leader>`, `<Nop>`,
  attributes) and a `~/.vigorc` style startup file
* syntax highlighting for 20+ file types, true colour with 256 colour fallback,
  wide characters, wrapping, `list`, line numbers, cursor line/column
* platform support: Unix and Windows raw mode, `sh -c` vs `cmd /c`, CRLF
  detection and platform default line endings, resize signal or polling

Partial or intentionally simple:

* `=` re-indents with a built-in heuristic (no `cindent`/`indentexpr`/`equalprg`)
* `gq` wraps by counting runes, not display columns
* `:sort` ignores flags, `:g` handles one command per match
* startup files skip `if`/`function`/etc. blocks instead of executing them
* syntax highlighting is disabled above 50 000 lines

Missing (see also *Not implemented* in `README.md`):

* `<expr>` mappings, `:autocmd`, `:function`, `:highlight`/`:colorscheme`
* undo tree, persistent undo, `:earlier`/`:later`
* tags/`ctrl-]`, folds, spell checking, abbreviations, digraphs, mouse
* reading the system clipboard (writing works through OSC 52)
* `iskeyword`, `virtualedit`, sessions, `:argdo`/`:bufdo`/`:windo`
* `it`/`at` tag objects, `\zs`/`\ze` and back references in patterns

## Verification

| What | How | Result |
| --- | --- | --- |
| Unit tests | `go test ./...` | 102 tests pass |
| Random keys | `TestFuzzNoPanic` (30 rounds x 60 keys, resizes, splits) | no panics, no hangs, bounded growth |
| Real terminal (Linux) | scripted sessions in a pty: insert, `gg=G`, `:%s`, `yyp`, `:w`, `:q`, splits, `:!` | files written as expected |
| Real terminal (Windows 11) | `go test ./...` with Go 1.24.3 on the machine | all packages pass, re-run after the review fixes (exit 0) |
| Real terminal (Windows 11) | scripted TUI session over an SSH pseudo console | buffer, statusline, line numbers, `:set nu`, `:w`, `:q` work; written file has CRLF |
| vim comparison | `vim -es -u NONE` for the same commands | `:`m`/`:t` ranges, `''`, replace mode backspace and bracketed paste match vim |
| Cross compilation | `GOOS`/`GOARCH` builds | windows/amd64, windows/arm64, darwin/amd64, darwin/arm64, linux/arm64, linux/386 |
| Static checks | `go vet ./...`, `gofmt -l .` | clean |

macOS has not been run, only cross compiled.

## Bugs found in this review and fixed

| # | Symptom | Cause | Fix |
| --- | --- | --- | --- |
| 1 | `:3m0` moved the line to the wrong place, `:2m0` did nothing | the address `0` (before the first line) was clamped to line 1 | `exMove` checks for a leading `0` and inserts at the top |
| 2 | first `ctrl-n` completed to the *second* candidate and cycling restarted after the first hit | off-by-one index plus recomputing the prefix after each completion | the completion remembers its prefix and starts at candidate 0 |
| 3 | `:%s/x/y/gc` kept replacing after answering `l` (last) | the loop only disabled the `g` flag | `l` replaces the current match and returns |
| 4 | a confirmation prompt could block forever | `:s///c` asked for a key even when nothing could answer | scripts (`scriptDepth > 0`) substitute without asking; the prompt path stays for interactive use |
| 5 | Windows: vigo refused to start with "The parameter is incorrect." | `ENABLE_VIRTUAL_TERMINAL_PROCESSING` was set on the *input* handle and the failure was fatal | correct flag per handle, best effort, plus an own raw mode that falls back if `ENABLE_VIRTUAL_TERMINAL_INPUT` is rejected |
| 6 | `''` and `` `` `` jumped to the last visual selection | the special marks were wired to the wrong state | `''`/`` `` `` use the jump list (and toggle), `'.`/`` `. ``/`` `[ ``/`` `] `` use the change list, `` `" `` the last cursor of the buffer |
| 7 | `R` followed by backspace deleted instead of restoring | replace mode did not remember overwritten characters | a replace stack restores them, matching vim (`abc` + `RXY<BS>Z` gives `XZc`) |
| 8 | undo history could grow to hundreds of megabytes | every checkpoint copied the whole buffer, limited only by `undolevels` | a 128 MB budget drops the oldest snapshots |
| 9 | `:set readonly` was accepted but had no effect | no wiring to the buffer flag | `:set readonly`/`ro`/`noreadonly` set and clear it, `:w!` still overrides |
| 10 | `noreadonly` was not recognised | the `no` prefix was only stripped for options in the generic table | option name lookup knows the special options too |
| 11 | pasting a few megabytes never finished (the test timed out after three minutes) | register `.` was rebuilt with `reg.Lines[0] += string(r)` for every inserted character, which is quadratic | the insert text is accumulated in a `[]rune` and the register is built when it is read; a run of pasted characters is inserted in one call (3.8 MiB paste: 945 ms) |
| 12 | one very long line made every frame cost ~300 ms | drawing each wrapped screen row called `displayCol` from column 0, so a 1M character line was rescanned ~20 times per frame | the display column of every wrapped segment is cached with the segment list; the same frame is ~0 ms |
| 13 | crash: `index out of range` while drawing after an edit | the segment cache that fix 12 introduced returned stale ranges because it only validated itself on a cache miss | the cache is validated on every lookup (buffer version, width, wrap) and the renderer stops at the end of the line defensively; found by `TestFuzzNoPanic` |

## Deliberate differences from vim (verified, not bugs)

* Pasting text in normal mode inserts it literally. vim 9.1 does the same
  (checked); pasting a macro still needs insert mode or `:set paste`.
* `:s///c` in a script substitutes without asking, because there is nobody to
  answer; vim does not prompt there either.
* A mapping that is a complete match runs immediately even if a longer mapping
  could still match; vim waits for `timeoutlen` in that case.
* `.` and `@` replay the recorded keys literally instead of re-mapping them,
  because the recording already contains the expanded right hand sides.
* Prompts (`:s///c`, the `!` filter, `ctrl-r{reg}`) read from the terminal
  channel, so embedding the editor without a key source will block there.
* `''`-style marks, the jump list and the change list are per buffer; vim keeps
  a global jump list that also carries the file name.

## Size and performance limits

Measured on this machine with `go test ./internal/editor -run TestLimits -v`.
The numbers are from the current revision.

| Case | Result |
| --- | --- |
| 200 000 lines / 2.8 MiB file: open and parse | 35 ms, heap grows by 18.6 MiB (about 6.6x the text, because lines are stored as `[]rune`) |
| the same file: draw, `G`, scrolling | under 1 ms per frame (syntax highlighting is off above 50 000 lines) |
| the same file: search for a pattern in every line | 130 ms |
| the same file: recompute the search highlight | 35 ms |
| the same file: 200 edit/undo cycles | 5.4 s, so **27 ms per edit** |
| 1 000 000 characters on one line, with wrapping | first frame 26 ms, later frames under 1 ms |
| horizontal scroll (`$`) on that line | 24 ms |
| 61 split windows: creation and 500 frames | instant, 45 ms for all 500 frames |
| 300 changes on a 20 000 line file: undo memory | 6.3 MiB |
| pasting 3.8 MiB | 945 ms, 200 001 lines |

Explicit limits in the code:

| Limit | Value | Where |
| --- | --- | --- |
| syntax highlighting is disabled above | 50 000 lines | `internal/editor/syntax.go` |
| search highlight matches kept | 2 000 | `internal/editor/search.go` |
| undo levels | `undolevels`, default 1 000 | `internal/editor/options.go` |
| undo history memory | 128 MiB, oldest dropped first | `internal/buffer/undo.go` |
| command line history | 200 entries | `internal/editor/extras.go` |
| search history | 100 entries | `internal/editor/search.go` |
| change list and jump list | 100 entries each | `internal/editor/extras.go` |
| mapping recursion depth | 200 | `internal/editor/mapping.go` |
| escape sequence length | 64 bytes | `internal/keys/term.go` |

There is no hard limit on file size, number of lines, line length, number of
registers, register size or number of windows; the practical limits are memory
and the numbers above. The one that matters most is that **the cost of a change
grows with the size of the file**, because a checkpoint copies the whole buffer:
at 2.8 MiB that is 27 ms per edit, so a 100 MiB file would be unusable. Moving
to block based undo is the first item in the next steps below.

## Cleanup done in this pass

Removed code that nothing called: `screen.FillRect`, `Rect.Sub`, `Rect.String`,
`Screen.TextTrunc`/`Rect.TextTrunc`, `Terminal.Writer`, `screen.SizeOf`,
`editor.shellCommand`, `buffer.AppendToLine`, `buffer.InsertString`,
`buffer.FindForward`, `buffer.FindBackward`, `buffer.CanUndo`, `buffer.CanRedo`,
`editor.optionsList`, the unused `viewRows` field, and the buffer test that only
covered the removed search helpers.

Two slow test suites stay on purpose: `TestLimits*` in
`internal/editor/limits_bench_test.go` guards the performance work above
(they take about 8 seconds and are worth running before a release).

## Known limitations (open)

Ordered by how likely a user is to notice:

1. Marks do not follow text edits. Inserting a line above a mark leaves the mark
   behind, where vim adjusts it. Marks are absolute positions.
2. Combining marks are measured as zero width but not rendered (a cell holds one
   rune), and editing works on runes rather than grapheme clusters.
3. Everything above U+007F counts as a word character (no `iskeyword`), so `w`
   treats NBSP and `«` as word characters.
4. Syntax highlighting recomputes the lexer state forward from the top of the
   buffer whenever the buffer version changes, which costs one pass over the
   file per edit; it is switched off above 50 000 lines for that reason.
5. No `:autocmd`/`:function`/`<expr>` mappings, so a full vimrc only works
   partially; unsupported lines are reported at startup.
6. No tag support, folds, spell checking, mouse, `virtualedit`, sessions,
   `:argdo`/`:bufdo`/`:windo`, or undo tree.
7. `gq` and `textwidth` count runes instead of display columns, so wrapped text
   with tabs is slightly off.
8. `:sort` without flags only; `:g` runs a single command per matching line.
9. Windows and macOS are cross compiled and vetted; only Linux and Windows have
   been run, and only Linux has an automated terminal smoke test.
10. The statusline is always shown (`laststatus=2`) and there is no
    `:set statusline=...`, `%{}` expansion or `tabline`.

## Suggested next steps

1. Make marks follow edits (adjust stored positions on insert/delete), which also
   fixes `` `[ ``/`` `] `` and `'<`/`'>`.
2. Add `:autocmd` and file type based indenting, so a vimrc that only uses
   `filetype plugin indent on` behaves.
3. Unicode: render combining marks and treat grapheme clusters as one unit for
   `x`, backspace and `h`/`l`.
4. Efficient undo: block based storage instead of whole buffer snapshots, which
   is what makes editing a large file slow today (27 ms per change on 2.8 MiB).
5. An automated terminal test harness (pty driven) that runs the TUI scripted,
   so rendering regressions are caught without a human.
6. Re-run the test suite on Windows and macOS for every change; today that
   requires reaching the Windows machine by hand.

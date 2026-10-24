// Command vigo is a vi/vim clone written in Go.
package main

import (
	"fmt"
	"os"

	"vigo/internal/editor"
)

// version is overridden at build time for releases:
// go build -ldflags "-X main.version=1.2.3"
var version = "0.1"

func main() {
	args := os.Args[1:]
	if len(args) > 0 {
		switch args[0] {
		case "-h", "--help":
			usage()
			return
		case "-v", "--version":
			fmt.Printf("vigo %s\n", version)
			return
		}
	}
	os.Exit(editor.Run(args))
}

func usage() {
	fmt.Print(`vigo ` + version + ` - a vi/vim clone written in Go

usage: vigo [options] [file ...]

options:
  +N            start at line N
  +command      run an ex command after loading the file
  -c command    the same as +command
  -u file       use file instead of ~/.vigorc (-u NONE skips it)
  --norc        do not read ~/.vigorc
  -h, --help    show this help
  -v, --version show the version

vigo implements most of vi/vim: modes, motions, operators, text objects,
registers, undo, macros, marks, search, :substitute, windows, key mappings and
a good part of the ex command line. Type :q to quit, :q! to quit without saving.

Settings and mappings are read from ~/.vigorc (or $VIGO_RC):
  set number
  let mapleader = ","
  nnoremap <leader>w :w<CR>
`)
}

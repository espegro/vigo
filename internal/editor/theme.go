package editor

import "vigo/internal/screen"

// theme holds the attributes used when drawing.
type theme struct {
	normal       screen.Attr
	lineNr       screen.Attr
	cursorLineNr screen.Attr
	cursorLine   screen.Attr
	visual       screen.Attr
	search       screen.Attr
	incSearch    screen.Attr
	matchParen   screen.Attr
	statusLine   screen.Attr
	statusLineNC screen.Attr
	cmdline      screen.Attr
	message      screen.Attr
	error        screen.Attr
	nonText      screen.Attr
	syntax       map[syntaxGroup]screen.Attr
}

func rgb(r, g, b uint8) screen.Color { return screen.RGB(r, g, b) }

func makeTheme(background string) theme {
	dark := background != "light"
	var (
		text     screen.Color
		comment  screen.Color
		str      screen.Color
		number   screen.Color
		keyword  screen.Color
		typeName screen.Color
		funcName screen.Color
		preproc  screen.Color
		special  screen.Color
	)
	if dark {
		text = rgb(0xd4, 0xd4, 0xd4)
		comment = rgb(0x5c, 0x63, 0x70)
		str = rgb(0x98, 0xc3, 0x79)
		number = rgb(0xd1, 0x9a, 0x66)
		keyword = rgb(0xc6, 0x78, 0xdd)
		typeName = rgb(0xe5, 0xc0, 0x7b)
		funcName = rgb(0x61, 0xaf, 0xef)
		preproc = rgb(0x56, 0xb6, 0xc2)
		special = rgb(0x56, 0xb6, 0xc2)
	} else {
		text = rgb(0x24, 0x29, 0x2e)
		comment = rgb(0x6a, 0x73, 0x7d)
		str = rgb(0x22, 0x86, 0x3a)
		number = rgb(0xb0, 0x88, 0x00)
		keyword = rgb(0xd7, 0x3a, 0x49)
		typeName = rgb(0x6f, 0x42, 0xc1)
		funcName = rgb(0x00, 0x5c, 0xc5)
		preproc = rgb(0x03, 0x2f, 0x62)
		special = rgb(0x03, 0x2f, 0x62)
	}
	normal := screen.Attr{FG: text}
	th := theme{
		normal:     normal,
		nonText:    screen.Attr{FG: rgb(0x56, 0x5c, 0x56)},
		lineNr:     screen.Attr{FG: rgb(0x80, 0x80, 0x80)},
		matchParen: screen.Attr{FG: rgb(0x00, 0xff, 0xff), Bold: true},
		syntax:     map[syntaxGroup]screen.Attr{},
	}
	if dark {
		th.cursorLineNr = screen.Attr{FG: rgb(0xff, 0xff, 0x00), Bold: true}
		th.cursorLine = screen.Attr{FG: text, BG: rgb(0x26, 0x26, 0x26)}
		th.visual = screen.Attr{FG: rgb(0xff, 0xff, 0xff), BG: rgb(0x44, 0x4c, 0x54)}
		th.search = screen.Attr{FG: rgb(0x00, 0x00, 0x00), BG: rgb(0xff, 0xff, 0x00)}
		th.incSearch = screen.Attr{FG: rgb(0xff, 0xff, 0xff), BG: rgb(0xaf, 0x5f, 0x00)}
		th.statusLine = screen.Attr{FG: rgb(0xe4, 0xe4, 0xe4), BG: rgb(0x30, 0x30, 0x30), Bold: true}
		th.statusLineNC = screen.Attr{FG: rgb(0xa0, 0xa0, 0xa0), BG: rgb(0x20, 0x20, 0x20)}
		th.cmdline = screen.Attr{FG: text}
		th.message = screen.Attr{FG: text}
		th.error = screen.Attr{FG: rgb(0xff, 0xff, 0xff), BG: rgb(0xa0, 0x20, 0x20), Bold: true}
	} else {
		th.cursorLineNr = screen.Attr{FG: rgb(0x00, 0x00, 0x00), Bold: true}
		th.cursorLine = screen.Attr{FG: text, BG: rgb(0xef, 0xef, 0xef)}
		th.visual = screen.Attr{FG: rgb(0x00, 0x00, 0x00), BG: rgb(0xb4, 0xd5, 0xfe)}
		th.search = screen.Attr{FG: rgb(0x00, 0x00, 0x00), BG: rgb(0xff, 0xd7, 0x00)}
		th.incSearch = screen.Attr{FG: rgb(0xff, 0xff, 0xff), BG: rgb(0xff, 0x8c, 0x00)}
		th.statusLine = screen.Attr{FG: rgb(0x00, 0x00, 0x00), BG: rgb(0xd0, 0xd0, 0xd0), Bold: true}
		th.statusLineNC = screen.Attr{FG: rgb(0x40, 0x40, 0x40), BG: rgb(0xe0, 0xe0, 0xe0)}
		th.cmdline = screen.Attr{FG: text}
		th.message = screen.Attr{FG: text}
		th.error = screen.Attr{FG: rgb(0xff, 0xff, 0xff), BG: rgb(0xc0, 0x20, 0x20), Bold: true}
	}
	full := func(c screen.Color) screen.Attr { return screen.Attr{FG: c} }
	th.syntax[groupComment] = full(comment)
	th.syntax[groupString] = full(str)
	th.syntax[groupNumber] = full(number)
	th.syntax[groupKeyword] = screen.Attr{FG: keyword, Bold: true}
	th.syntax[groupType] = full(typeName)
	th.syntax[groupFunc] = full(funcName)
	th.syntax[groupPreproc] = full(preproc)
	th.syntax[groupConstant] = full(number)
	th.syntax[groupSpecial] = full(special)
	return th
}

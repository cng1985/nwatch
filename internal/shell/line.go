package shell

import (
	"bytes"
	"unicode/utf8"
)

const (
	modeNormal = iota
	modeESC
	modeCSI
	modeSS3
)

var pasteEnd = []byte("\x1b[201~")

// submission 是用户按下回车后提交的一行。
// fromScreen 为真时，这一行来自方向键历史或 Tab 补全，调用方应改用屏幕上的文本。
type submission struct {
	text       string
	fromScreen bool
}

// lineEdit 把终端按键还原成命令行，处理退格、方向键和粘贴。
type lineEdit struct {
	buf        []byte
	runes      []rune
	col        int
	mode       int
	seq        []byte
	paste      bool
	skipLF     bool
	unreliable bool
}

func (e *lineEdit) Feed(p []byte) []submission {
	if len(p) == 0 {
		return nil
	}
	e.buf = append(e.buf, p...)
	var out []submission
	for {
		if len(e.buf) == 0 {
			return out
		}
		if e.skipLF {
			e.skipLF = false
			if e.buf[0] == '\n' {
				e.buf = e.buf[1:]
				continue
			}
		}
		if e.paste {
			sub, ok := e.feedPaste()
			if !ok {
				return out
			}
			if sub != nil {
				out = append(out, *sub)
			}
			continue
		}
		if e.mode != modeNormal {
			if !e.feedEsc() {
				return out
			}
			continue
		}
		b := e.buf[0]
		if b == 0x1b {
			e.mode = modeESC
			e.seq = []byte{0x1b}
			e.buf = e.buf[1:]
			continue
		}
		if b < 0x20 || b == 0x7f {
			if sub := e.feedControl(); sub != nil {
				out = append(out, *sub)
			}
			continue
		}
		r, size := utf8.DecodeRune(e.buf)
		if r == utf8.RuneError && !utf8.FullRune(e.buf) {
			return out
		}
		if r != utf8.RuneError {
			e.insert(r)
		}
		if size < 1 {
			size = 1
		}
		e.buf = e.buf[size:]
	}
}

func (e *lineEdit) feedPaste() (*submission, bool) {
	if len(e.buf) == 0 {
		return nil, false
	}
	if e.buf[0] == 0x1b {
		if bytes.HasPrefix(pasteEnd, e.buf) && len(e.buf) < len(pasteEnd) {
			return nil, false
		}
		if bytes.HasPrefix(e.buf, pasteEnd) {
			e.buf = e.buf[len(pasteEnd):]
			e.paste = false
			return nil, true
		}
		e.buf = e.buf[1:]
		return nil, true
	}
	if e.buf[0] == '\r' {
		e.buf = e.buf[1:]
		e.skipLF = true
		sub := e.submit()
		return &sub, true
	}
	if e.buf[0] == '\n' {
		e.buf = e.buf[1:]
		sub := e.submit()
		return &sub, true
	}
	r, size := utf8.DecodeRune(e.buf)
	if r == utf8.RuneError && !utf8.FullRune(e.buf) {
		return nil, false
	}
	if r != utf8.RuneError {
		e.insert(r)
	}
	if size < 1 {
		size = 1
	}
	e.buf = e.buf[size:]
	return nil, true
}

func (e *lineEdit) feedEsc() bool {
	if len(e.buf) == 0 {
		return false
	}
	b := e.buf[0]
	e.buf = e.buf[1:]
	switch e.mode {
	case modeESC:
		switch b {
		case '[':
			e.mode = modeCSI
			e.seq = append(e.seq, b)
		case 'O':
			e.mode = modeSS3
			e.seq = append(e.seq, b)
		default:
			e.resetEsc()
		}
	case modeSS3:
		e.applySS3(b)
		e.resetEsc()
	case modeCSI:
		e.seq = append(e.seq, b)
		if b >= 0x40 && b <= 0x7E {
			e.applyCSI(e.seq)
			e.resetEsc()
		} else if len(e.seq) > 32 {
			e.resetEsc()
		}
	default:
		e.resetEsc()
	}
	return true
}

func (e *lineEdit) resetEsc() {
	e.mode = modeNormal
	e.seq = nil
}

func (e *lineEdit) feedControl() *submission {
	b := e.buf[0]
	e.buf = e.buf[1:]
	switch b {
	case '\r':
		e.skipLF = true
		sub := e.submit()
		return &sub
	case '\n':
		sub := e.submit()
		return &sub
	case 0x7f, '\b':
		e.backspace()
	case 0x03:
		e.runes = nil
		e.col = 0
		e.unreliable = false
	case 0x15:
		e.runes = nil
		e.col = 0
	case 0x17:
		e.backWord()
	case 0x0b:
		if e.col < len(e.runes) {
			e.runes = e.runes[:e.col]
		}
	case 0x01:
		e.col = 0
	case 0x05:
		e.col = len(e.runes)
	case 0x04:
		e.deleteForward()
	case '\t':
		e.unreliable = true
	}
	return nil
}

func (e *lineEdit) applySS3(b byte) {
	switch b {
	case 'A', 'B':
		e.markUnknown()
	case 'C':
		e.move(1)
	case 'D':
		e.move(-1)
	case 'H':
		e.col = 0
	case 'F':
		e.col = len(e.runes)
	}
}

func (e *lineEdit) applyCSI(seq []byte) {
	n, final := csiArg(seq)
	switch final {
	case 'A', 'B':
		e.markUnknown()
	case 'C':
		e.move(n)
	case 'D':
		e.move(-n)
	case 'H':
		e.col = 0
	case 'F':
		e.col = len(e.runes)
	case '~':
		switch n {
		case 1, 7:
			e.col = 0
		case 3:
			e.deleteForward()
		case 4, 8:
			e.col = len(e.runes)
		case 200:
			e.paste = true
		case 201:
			e.paste = false
		}
	}
}

func csiArg(seq []byte) (int, byte) {
	if len(seq) == 0 {
		return 1, 0
	}
	final := seq[len(seq)-1]
	n := 0
	seen := false
	for _, c := range seq {
		if c >= '0' && c <= '9' {
			seen = true
			n = n*10 + int(c-'0')
			continue
		}
		if seen {
			break
		}
	}
	if !seen {
		n = 1
	}
	return n, final
}

func (e *lineEdit) markUnknown() {
	e.unreliable = true
	e.runes = nil
	e.col = 0
}

func (e *lineEdit) submit() submission {
	sub := submission{text: string(e.runes), fromScreen: e.unreliable}
	e.runes = nil
	e.col = 0
	e.unreliable = false
	return sub
}

func (e *lineEdit) insert(r rune) {
	if r < 0x20 || r == 0x7f {
		return
	}
	if e.col > len(e.runes) {
		e.col = len(e.runes)
	}
	e.runes = append(e.runes, 0)
	copy(e.runes[e.col+1:], e.runes[e.col:])
	e.runes[e.col] = r
	e.col++
	if len(e.runes) > 4000 {
		e.runes = e.runes[:4000]
		if e.col > len(e.runes) {
			e.col = len(e.runes)
		}
	}
}

func (e *lineEdit) backspace() {
	if e.col == 0 || e.col > len(e.runes) {
		return
	}
	e.runes = append(e.runes[:e.col-1], e.runes[e.col:]...)
	e.col--
}

func (e *lineEdit) deleteForward() {
	if e.col >= len(e.runes) {
		return
	}
	e.runes = append(e.runes[:e.col], e.runes[e.col+1:]...)
}

func (e *lineEdit) backWord() {
	if e.col == 0 {
		return
	}
	i := e.col
	for i > 0 && e.runes[i-1] == ' ' {
		i--
	}
	for i > 0 && e.runes[i-1] != ' ' {
		i--
	}
	e.runes = append(e.runes[:i], e.runes[e.col:]...)
	e.col = i
}

func (e *lineEdit) move(delta int) {
	e.col += delta
	if e.col < 0 {
		e.col = 0
	}
	if e.col > len(e.runes) {
		e.col = len(e.runes)
	}
}

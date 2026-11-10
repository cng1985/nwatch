package shell

import (
	"strings"
	"sync"
	"unicode/utf8"
)

// screenLine 跟踪终端当前行的可见文本，用来还原方向键历史和 Tab 补全后的命令。
type screenLine struct {
	mu      sync.Mutex
	line    []rune
	col     int
	esc     []byte
	partial []byte
}

func (s *screenLine) Write(p []byte) {
	if len(p) == 0 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	data := p
	if len(s.partial) > 0 {
		data = append(append([]byte{}, s.partial...), p...)
		s.partial = nil
	}
	for len(data) > 0 {
		if s.esc != nil || data[0] == 0x1b || data[0] < 0x20 || data[0] == 0x7f {
			s.byte(data[0])
			data = data[1:]
			continue
		}
		r, size := utf8.DecodeRune(data)
		if r == utf8.RuneError && !utf8.FullRune(data) {
			s.partial = append([]byte{}, data...)
			return
		}
		if r != utf8.RuneError {
			s.put(r)
		}
		if size < 1 {
			size = 1
		}
		data = data[size:]
	}
}

func (s *screenLine) Text() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return string(s.line)
}

func (s *screenLine) byte(b byte) {
	if s.esc != nil {
		s.esc = append(s.esc, b)
		if s.finishEsc() || len(s.esc) > 64 {
			s.esc = nil
		}
		return
	}
	switch b {
	case 0x1b:
		s.esc = []byte{0x1b}
	case '\r':
		s.col = 0
	case '\n':
		s.line = nil
		s.col = 0
	case '\b':
		if s.col > 0 {
			s.col--
		}
	default:
		if b < 0x20 || b == 0x7f {
			return
		}
		s.put(rune(b))
	}
}

func (s *screenLine) finishEsc() bool {
	if len(s.esc) < 2 {
		return false
	}
	switch s.esc[1] {
	case '[':
		if len(s.esc) < 3 {
			return false
		}
		last := s.esc[len(s.esc)-1]
		if last < 0x40 || last > 0x7E {
			return false
		}
		s.applyCSI(s.esc[2:len(s.esc)-1], last)
		return true
	case ']':
		last := s.esc[len(s.esc)-1]
		if last == '\a' {
			return true
		}
		return last == '\\' && len(s.esc) >= 3 && s.esc[len(s.esc)-2] == 0x1b
	case 'O':
		if len(s.esc) < 3 {
			return false
		}
		return true
	default:
		return true
	}
}

func (s *screenLine) applyCSI(params []byte, final byte) {
	nums := csiNums(params)
	n := func(i, def int) int {
		if i < len(nums) && nums[i] > 0 {
			return nums[i]
		}
		return def
	}
	switch final {
	case 'C':
		s.col += n(0, 1)
	case 'D':
		s.col -= n(0, 1)
		if s.col < 0 {
			s.col = 0
		}
	case 'G':
		s.col = n(0, 1) - 1
		if s.col < 0 {
			s.col = 0
		}
	case 'K':
		mode := 0
		if len(nums) > 0 {
			mode = nums[0]
		}
		switch mode {
		case 1:
			for i := 0; i < s.col && i < len(s.line); i++ {
				s.line[i] = ' '
			}
		case 2:
			s.line = nil
			s.col = 0
		default:
			if s.col < len(s.line) {
				s.line = s.line[:s.col]
			}
		}
	case 'J':
		s.line = nil
		s.col = 0
	case 'H', 'f':
		col := 1
		if len(nums) >= 2 && nums[1] > 0 {
			col = nums[1]
		}
		s.col = col - 1
	case 'P':
		count := n(0, 1)
		if s.col < len(s.line) {
			end := s.col + count
			if end > len(s.line) {
				end = len(s.line)
			}
			s.line = append(s.line[:s.col], s.line[end:]...)
		}
	}
}

func csiNums(params []byte) []int {
	if len(params) == 0 {
		return nil
	}
	var nums []int
	n := 0
	seen := false
	for _, c := range params {
		if c >= '0' && c <= '9' {
			seen = true
			n = n*10 + int(c-'0')
			continue
		}
		if c == ';' {
			nums = append(nums, n)
			n = 0
			seen = false
		}
	}
	if seen || len(nums) > 0 {
		nums = append(nums, n)
	}
	return nums
}

func (s *screenLine) put(r rune) {
	if s.col < 0 {
		s.col = 0
	}
	for len(s.line) < s.col {
		s.line = append(s.line, ' ')
	}
	if s.col == len(s.line) {
		s.line = append(s.line, r)
	} else {
		s.line[s.col] = r
	}
	s.col++
	if len(s.line) > 2000 {
		extra := len(s.line) - 2000
		s.line = s.line[extra:]
		s.col -= extra
		if s.col < 0 {
			s.col = 0
		}
	}
}

// commandFromScreen 从当前屏幕行里去掉提示符，留下用户提交的命令。
func commandFromScreen(line string) string {
	line = strings.TrimRight(line, " \t")
	if line == "" {
		return ""
	}
	for i, r := range line {
		if r != '$' && r != '#' && r != '%' && r != '>' {
			continue
		}
		rest := line[i+len(string(r)):]
		if rest == "" {
			return ""
		}
		if rest[0] == ' ' || (r == '>' && winPrompt(line[:i+len(string(r))])) || promptHost(line[:i]) {
			return strings.TrimSpace(rest)
		}
	}
	return ""
}

func winPrompt(prefix string) bool {
	prefix = strings.TrimSpace(prefix)
	if strings.HasPrefix(prefix, "PS ") && strings.HasSuffix(prefix, ">") {
		return true
	}
	return len(prefix) >= 3 && ((prefix[0] >= 'A' && prefix[0] <= 'Z') || (prefix[0] >= 'a' && prefix[0] <= 'z')) && prefix[1] == ':' && strings.HasSuffix(prefix, ">")
}

func promptHost(prefix string) bool {
	return strings.Contains(prefix, "@") && !strings.Contains(prefix, " ")
}

package shell

import (
	"strings"
	"unicode/utf8"
)

// suppressForPassword 在当前屏幕行是密码提示、且还没有一条正常命令时跳过记录。
// 交互式 shell 会自己关闭终端回显，不能靠回显标志区分密码。
func suppressForPassword(visible string) bool {
	if commandFromScreen(visible) != "" {
		return false
	}
	line := strings.ToLower(visible)
	return strings.Contains(line, "password") || strings.Contains(line, "passphrase") || strings.Contains(visible, "密码") || strings.Contains(visible, "口令")
}

func sanitizeCommand(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if r == '\t' {
			b.WriteByte(' ')
			continue
		}
		if r < 0x20 || r == 0x7f {
			continue
		}
		b.WriteRune(r)
	}
	return clipBytes(strings.TrimSpace(b.String()), 2000)
}

func clipBytes(s string, max int) string {
	if len(s) <= max {
		return s
	}
	runes := []rune(s)
	for len(runes) > 0 && len(string(runes)) > max {
		runes = runes[:len(runes)-1]
	}
	out := string(runes)
	if !utf8.ValidString(out) {
		return ""
	}
	return out
}

package cronjob

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

var aliases = map[string]string{
	"@yearly":   "0 0 1 1 *",
	"@annually": "0 0 1 1 *",
	"@monthly":  "0 0 1 * *",
	"@weekly":   "0 0 * * 0",
	"@daily":    "0 0 * * *",
	"@midnight": "0 0 * * *",
	"@hourly":   "0 * * * *",
}

var monthNames = map[string]int{
	"jan": 1, "feb": 2, "mar": 3, "apr": 4, "may": 5, "jun": 6,
	"jul": 7, "aug": 8, "sep": 9, "oct": 10, "nov": 11, "dec": 12,
}

var dowNames = map[string]int{
	"sun": 0, "mon": 1, "tue": 2, "wed": 3, "thu": 4, "fri": 5, "sat": 6,
}

// Expr 是标准 5 段 cron：分 时 日 月 周。日和周都写了具体值时，满足任意一个即匹配。
type Expr struct {
	raw    string
	minute [60]bool
	hour   [24]bool
	dom    [32]bool
	month  [13]bool
	dow    [7]bool
	domAny bool
	dowAny bool
}

func Parse(input string) (*Expr, error) {
	text := strings.TrimSpace(input)
	if text == "" {
		return nil, errors.New("cron 表达式不能为空")
	}
	if alias, ok := aliases[strings.ToLower(text)]; ok {
		text = alias
	}
	fields := strings.Fields(strings.ToLower(text))
	if len(fields) != 5 {
		return nil, errors.New("cron 需要 5 段：分 时 日 月 周")
	}
	e := &Expr{raw: strings.Join(fields, " "), domAny: fields[2] == "*", dowAny: fields[4] == "*"}
	if err := fill(fields[0], 0, 59, e.minute[:], nil); err != nil {
		return nil, fmt.Errorf("分钟%w", err)
	}
	if err := fill(fields[1], 0, 23, e.hour[:], nil); err != nil {
		return nil, fmt.Errorf("小时%w", err)
	}
	if err := fill(fields[2], 1, 31, e.dom[:], nil); err != nil {
		return nil, fmt.Errorf("日期%w", err)
	}
	if err := fill(fields[3], 1, 12, e.month[:], monthNames); err != nil {
		return nil, fmt.Errorf("月份%w", err)
	}
	var dow [8]bool
	if err := fill(fields[4], 0, 7, dow[:], dowNames); err != nil {
		return nil, fmt.Errorf("星期%w", err)
	}
	if dow[7] {
		dow[0] = true
	}
	copy(e.dow[:], dow[:7])
	return e, nil
}

func (e *Expr) String() string {
	if e == nil {
		return ""
	}
	return e.raw
}

// Next 返回 from 之后的下一次匹配，秒和纳秒为 0，时区与 from 相同。
func (e *Expr) Next(from time.Time) (time.Time, error) {
	if e == nil {
		return time.Time{}, errors.New("cron 表达式不能为空")
	}
	loc := from.Location()
	t := time.Date(from.Year(), from.Month(), from.Day(), from.Hour(), from.Minute(), 0, 0, loc).Add(time.Minute)
	limit := t.AddDate(4, 0, 0)
	for t.Before(limit) {
		if !e.month[int(t.Month())] {
			t = time.Date(t.Year(), t.Month()+1, 1, 0, 0, 0, 0, loc)
			continue
		}
		if !e.dayMatches(t) {
			t = time.Date(t.Year(), t.Month(), t.Day()+1, 0, 0, 0, 0, loc)
			continue
		}
		if !e.hour[t.Hour()] {
			t = time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), 0, 0, 0, loc).Add(time.Hour)
			continue
		}
		if !e.minute[t.Minute()] {
			t = t.Add(time.Minute)
			continue
		}
		return t, nil
	}
	return time.Time{}, errors.New("未来 4 年内没有匹配的执行时间")
}

func (e *Expr) NextN(from time.Time, n int) ([]time.Time, error) {
	if n <= 0 {
		n = 5
	}
	if n > 8 {
		n = 8
	}
	out := make([]time.Time, 0, n)
	cursor := from
	for len(out) < n {
		next, err := e.Next(cursor)
		if err != nil {
			return nil, err
		}
		out = append(out, next)
		cursor = next
	}
	return out, nil
}

func (e *Expr) dayMatches(t time.Time) bool {
	dom := e.dom[t.Day()]
	dow := e.dow[int(t.Weekday())]
	switch {
	case e.domAny && e.dowAny:
		return true
	case e.domAny:
		return dow
	case e.dowAny:
		return dom
	default:
		return dom || dow
	}
}

func Next(raw string, from time.Time) (time.Time, error) {
	e, err := Parse(raw)
	if err != nil {
		return time.Time{}, err
	}
	return e.Next(from)
}

func NormalizeExpr(raw string) (string, error) {
	e, err := Parse(raw)
	if err != nil {
		return "", err
	}
	return e.raw, nil
}

func Describe(raw string) string {
	e, err := Parse(raw)
	if err != nil {
		text := strings.TrimSpace(raw)
		if text == "" {
			return "未填写"
		}
		return text
	}
	parts := strings.Fields(e.raw)
	minute, hour, dom, month, dow := parts[0], parts[1], parts[2], parts[3], parts[4]
	if month != "*" {
		return e.raw
	}
	if minute == "*" && hour == "*" && dom == "*" && dow == "*" {
		return "每分钟"
	}
	if strings.HasPrefix(minute, "*/") && hour == "*" && dom == "*" && dow == "*" && number(strings.TrimPrefix(minute, "*/")) {
		return "每 " + strings.TrimPrefix(minute, "*/") + " 分钟"
	}
	if hour == "*" && dom == "*" && dow == "*" && number(minute) {
		if minute == "0" {
			return "每小时整点"
		}
		return "每小时第 " + minute + " 分钟"
	}
	if dom == "*" && dow == "*" && number(minute) && number(hour) {
		return "每天 " + hhmm(hour, minute)
	}
	if dom == "*" && month == "*" && number(minute) && number(hour) {
		if label, ok := canonicalDow(dow); ok {
			return "每" + label + " " + hhmm(hour, minute)
		}
	}
	if dow == "*" && number(minute) && number(hour) && number(dom) {
		return "每月 " + dom + " 日 " + hhmm(hour, minute)
	}
	return e.raw
}

func fill(field string, min, max int, bits []bool, names map[string]int) error {
	field = strings.TrimSpace(field)
	if field == "" {
		return errors.New("不能为空")
	}
	for _, part := range strings.Split(field, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			return errors.New("格式不正确")
		}
		step := 1
		if i := strings.IndexByte(part, '/'); i >= 0 {
			n, err := strconv.Atoi(part[i+1:])
			if err != nil || n <= 0 {
				return errors.New("步长不正确")
			}
			step = n
			part = part[:i]
		}
		start, end := min, max
		switch {
		case part == "*":
		case strings.Contains(part, "-"):
			bounds := strings.Split(part, "-")
			if len(bounds) != 2 {
				return errors.New("范围不正确")
			}
			var err error
			start, err = parseValue(bounds[0], min, max, names)
			if err != nil {
				return err
			}
			end, err = parseValue(bounds[1], min, max, names)
			if err != nil {
				return err
			}
			if start > end {
				return errors.New("范围起止颠倒")
			}
		default:
			n, err := parseValue(part, min, max, names)
			if err != nil {
				return err
			}
			start = n
			if step == 1 {
				end = n
			}
		}
		for v := start; v <= end; v += step {
			bits[v] = true
		}
	}
	return nil
}

func parseValue(s string, min, max int, names map[string]int) (int, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	if names != nil {
		if v, ok := names[s]; ok {
			return v, nil
		}
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < min || n > max {
		return 0, errors.New("取值不正确")
	}
	return n, nil
}

func number(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func canonicalDow(s string) (string, bool) {
	names := []string{"周日", "周一", "周二", "周三", "周四", "周五", "周六"}
	if v, ok := dowNames[s]; ok {
		return names[v], true
	}
	if !number(s) {
		return "", false
	}
	n, _ := strconv.Atoi(s)
	if n == 7 {
		n = 0
	}
	if n < 0 || n >= len(names) {
		return "", false
	}
	return names[n], true
}

func hhmm(hour, minute string) string {
	h, _ := strconv.Atoi(hour)
	m, _ := strconv.Atoi(minute)
	return fmt.Sprintf("%02d:%02d", h, m)
}

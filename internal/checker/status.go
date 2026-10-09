package checker

import (
	"strconv"
	"strings"
)

// MatchStatus supports exact codes ("200"), lists ("200,201") and ranges ("200-299").
func MatchStatus(code int, expr string) bool {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		expr = "200"
	}
	for _, part := range strings.Split(expr, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if strings.Contains(part, "-") {
			bounds := strings.SplitN(part, "-", 2)
			lo, err1 := strconv.Atoi(strings.TrimSpace(bounds[0]))
			hi, err2 := strconv.Atoi(strings.TrimSpace(bounds[1]))
			if err1 != nil || err2 != nil {
				continue
			}
			if code >= lo && code <= hi {
				return true
			}
			continue
		}
		n, err := strconv.Atoi(part)
		if err == nil && code == n {
			return true
		}
	}
	return false
}

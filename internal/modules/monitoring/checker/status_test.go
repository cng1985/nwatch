package checker

import "testing"

func TestMatchStatus(t *testing.T) {
	cases := []struct {
		code int
		expr string
		want bool
	}{
		{200, "", true},
		{201, "", false},
		{201, "200,201", true},
		{204, "200-299", true},
		{404, "200-299", false},
		{301, "200-299,301", true},
	}
	for _, tc := range cases {
		if got := MatchStatus(tc.code, tc.expr); got != tc.want {
			t.Fatalf("MatchStatus(%d, %q)=%v", tc.code, tc.expr, got)
		}
	}
}

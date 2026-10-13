package cronjob

import (
	"testing"
	"time"
)

func TestParseAndNext(t *testing.T) {
	loc := time.FixedZone("CST", 8*3600)
	at := func(y int, m time.Month, d, hh, mm int) time.Time {
		return time.Date(y, m, d, hh, mm, 0, 0, loc)
	}
	cases := []struct {
		expr string
		from time.Time
		want time.Time
		text string
	}{
		{"* * * * *", at(2026, 1, 1, 0, 0).Add(30 * time.Second), at(2026, 1, 1, 0, 1), "每分钟"},
		{"*/15 * * * *", at(2026, 1, 1, 10, 7), at(2026, 1, 1, 10, 15), "每 15 分钟"},
		{"0 * * * *", at(2026, 1, 1, 0, 0), at(2026, 1, 1, 1, 0), "每小时整点"},
		{"@hourly", at(2026, 1, 1, 0, 0), at(2026, 1, 1, 1, 0), "每小时整点"},
		{"30 4 * * *", at(2026, 1, 1, 5, 0), at(2026, 1, 2, 4, 30), "每天 04:30"},
		{"@daily", at(2026, 1, 1, 5, 0), at(2026, 1, 2, 0, 0), "每天 00:00"},
		{"0 9 * * 1", at(2026, 1, 1, 0, 0), at(2026, 1, 5, 9, 0), "每周一 09:00"},
		{"0 9 * * MON", at(2026, 1, 1, 0, 0), at(2026, 1, 5, 9, 0), "每周一 09:00"},
		{"0 0 1 JAN *", at(2026, 6, 1, 0, 0), at(2027, 1, 1, 0, 0), "0 0 1 jan *"},
		{"0 0 29 2 *", at(2026, 3, 1, 0, 0), at(2028, 2, 29, 0, 0), "0 0 29 2 *"},
		{"0 0 1 * 1", at(2026, 1, 1, 0, 0), at(2026, 1, 5, 0, 0), "0 0 1 * 1"},
		{"0 8 1 * *", at(2026, 1, 2, 0, 0), at(2026, 2, 1, 8, 0), "每月 1 日 08:00"},
	}
	for _, tc := range cases {
		e, err := Parse(tc.expr)
		if err != nil {
			t.Fatalf("%s: %v", tc.expr, err)
		}
		got, err := e.Next(tc.from)
		if err != nil {
			t.Fatalf("%s next: %v", tc.expr, err)
		}
		if !got.Equal(tc.want) {
			t.Fatalf("%s next = %s, want %s", tc.expr, got, tc.want)
		}
		if text := Describe(tc.expr); text != tc.text {
			t.Fatalf("%s text = %q, want %q", tc.expr, text, tc.text)
		}
	}
}

func TestParseRejectsInvalid(t *testing.T) {
	for _, expr := range []string{"", "* * *", "60 * * * *", "a * * * *", "0 0 * * 8"} {
		if _, err := Parse(expr); err == nil {
			t.Fatalf("expected error for %q", expr)
		}
	}
	if _, err := Parse("0 0 31 2 *"); err != nil {
		t.Fatal(err)
	}
	if _, err := Next("0 0 31 2 *", time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)); err == nil {
		t.Fatal("feb 31 should have no next time")
	}
}

func TestNextStaysInSourceZone(t *testing.T) {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatal(err)
	}
	next, err := Next("0 2 * * *", time.Date(2026, 3, 1, 3, 0, 0, 0, loc))
	if err != nil {
		t.Fatal(err)
	}
	if next.Location().String() != "Asia/Shanghai" || next.Hour() != 2 || next.Day() != 2 {
		t.Fatalf("got %s", next)
	}
}

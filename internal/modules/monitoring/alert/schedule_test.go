package alert

import (
	"testing"
	"time"
)

func TestDueOffset(t *testing.T) {
	cases := []struct {
		stage int
		want  time.Duration
	}{
		{0, 15 * time.Second},
		{-1, 15 * time.Second},
		{1, 45 * time.Second},
		{2, 45*time.Second + time.Minute},
		{3, 45*time.Second + 2*time.Minute},
	}
	for _, tc := range cases {
		if got := DueOffset(tc.stage); got != tc.want {
			t.Fatalf("stage %d got %s want %s", tc.stage, got, tc.want)
		}
	}
}

func TestPlan(t *testing.T) {
	start := time.Date(2026, 10, 9, 8, 0, 0, 0, time.UTC)

	send, sent, nextStage, next := Plan(start, 0, start.Add(14*time.Second))
	if send || nextStage != 0 || !next.Equal(start.Add(15*time.Second)) {
		t.Fatalf("before 15s: send=%v stage=%d next=%s", send, nextStage, next)
	}

	send, sent, nextStage, next = Plan(start, 0, start.Add(15*time.Second))
	if !send || sent != 0 || nextStage != 1 || !next.Equal(start.Add(45*time.Second)) {
		t.Fatalf("at 15s: send=%v sent=%d nextStage=%d next=%s", send, sent, nextStage, next)
	}

	send, sent, nextStage, next = Plan(start, 0, start.Add(44*time.Second))
	if !send || sent != 0 || nextStage != 1 || !next.Equal(start.Add(45*time.Second)) {
		t.Fatalf("late 15s slot: send=%v sent=%d nextStage=%d next=%s", send, sent, nextStage, next)
	}

	send, sent, nextStage, next = Plan(start, 1, start.Add(45*time.Second))
	if !send || sent != 1 || nextStage != 2 || !next.Equal(start.Add(105*time.Second)) {
		t.Fatalf("at 45s: send=%v sent=%d nextStage=%d next=%s", send, sent, nextStage, next)
	}

	send, sent, nextStage, next = Plan(start, 2, start.Add(104*time.Second))
	if send || nextStage != 2 || !next.Equal(start.Add(105*time.Second)) {
		t.Fatalf("before 105s: send=%v stage=%d next=%s", send, nextStage, next)
	}

	send, sent, nextStage, next = Plan(start, 2, start.Add(105*time.Second))
	if !send || sent != 2 || nextStage != 3 || !next.Equal(start.Add(165*time.Second)) {
		t.Fatalf("at 105s: send=%v sent=%d nextStage=%d next=%s", send, sent, nextStage, next)
	}

	// 15 秒和 45 秒都过了，只发一次，落到 45 秒那一档。
	send, sent, nextStage, next = Plan(start, 0, start.Add(45*time.Second))
	if !send || sent != 1 || nextStage != 2 || !next.Equal(start.Add(105*time.Second)) {
		t.Fatalf("catch up 45s: send=%v sent=%d nextStage=%d next=%s", send, sent, nextStage, next)
	}

	// 200 秒时最新到期档是 165 秒，下一次是 225 秒。
	send, sent, nextStage, next = Plan(start, 0, start.Add(200*time.Second))
	if !send || sent != 3 || nextStage != 4 || !next.Equal(start.Add(225*time.Second)) {
		t.Fatalf("catch up 200s: send=%v sent=%d nextStage=%d next=%s", send, sent, nextStage, next)
	}
}

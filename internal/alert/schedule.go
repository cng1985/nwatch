package alert

import "time"

// DueOffset 是故障开始后第 stage 次通知的时间。
// 0 → 15 秒，1 → 45 秒，之后每分钟一档（105 秒、165 秒……）。
func DueOffset(stage int) time.Duration {
	switch {
	case stage <= 0:
		return 15 * time.Second
	case stage == 1:
		return 45 * time.Second
	default:
		return 45*time.Second + time.Duration(stage-1)*time.Minute
	}
}

// Plan 决定现在要不要发通知。
// 还没到点就返回下一次的到期时间。多个时间点一起到期时只发一次，并跳到最新的那一档。
func Plan(start time.Time, stage int, now time.Time) (send bool, sentStage, newStage int, next time.Time) {
	if stage < 0 {
		stage = 0
	}
	due := start.Add(DueOffset(stage))
	if now.Before(due) {
		return false, stage, stage, due
	}
	const maxCatchUp = 2_000_000
	for stage < maxCatchUp {
		following := start.Add(DueOffset(stage + 1))
		if now.Before(following) {
			break
		}
		stage++
	}
	return true, stage, stage + 1, start.Add(DueOffset(stage + 1))
}

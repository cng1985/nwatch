package cronjob

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/cng1985/nwatch/internal/checker"
	"github.com/cng1985/nwatch/internal/httpreq"
	"github.com/cng1985/nwatch/internal/model"
)

const maxHTTPBody = 16 * 1024

type outcome struct {
	ExitCode int
	Success  bool
	Stdout   string
	Stderr   string
	Message  string
	Duration time.Duration
	TimedOut bool
}

func runJob(ctx context.Context, job *model.CronJob, timeout time.Duration) outcome {
	if job.IsHTTP() {
		return executeHTTP(ctx, job, timeout)
	}
	out := checker.Execute(ctx, job.Command, job.WorkDir, timeout)
	ok, judged := out.Judge("", "")
	message := ""
	if !ok {
		message = judged
	}
	return outcome{
		ExitCode: out.ExitCode,
		Success:  ok,
		Stdout:   out.Stdout,
		Stderr:   out.Stderr,
		Message:  message,
		Duration: out.Duration,
		TimedOut: out.TimedOut,
	}
}

func executeHTTP(ctx context.Context, job *model.CronJob, timeout time.Duration) outcome {
	start := time.Now()
	fail := func(message string, timedOut bool) outcome {
		return outcome{Message: message, TimedOut: timedOut, Duration: time.Since(start), ExitCode: 0}
	}
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	method := http.MethodGet
	bodyType, bodyText := "", ""
	if strings.EqualFold(job.Method, http.MethodPost) {
		method = http.MethodPost
		bodyType = job.BodyType
		bodyText = job.Body
	}
	payload, err := httpreq.Encode(bodyType, bodyText)
	if err != nil {
		return fail("请求体不正确", false)
	}
	reqCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, method, job.URL, payload.Body)
	if err != nil {
		return fail("地址不正确", false)
	}
	for key, value := range job.Headers {
		req.Header.Set(key, value)
	}
	if payload.ContentType != "" {
		req.Header.Set("Content-Type", payload.ContentType)
	}
	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", "NMonitor/1.0")
	}
	client := &http.Client{Timeout: timeout}
	if method == http.MethodPost {
		client.CheckRedirect = func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		}
	} else {
		client.CheckRedirect = func(_ *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return errors.New("重定向次数过多")
			}
			return nil
		}
	}
	resp, err := client.Do(req)
	if err != nil {
		if errors.Is(reqCtx.Err(), context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return fail("请求超时", true)
		}
		return fail("请求失败", false)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, maxHTTPBody))
	text := cleanBody(string(raw))
	message := "HTTP " + itoa(resp.StatusCode)
	success := resp.StatusCode >= 200 && resp.StatusCode < 300
	if !success {
		if line := firstLine(text); line != "" {
			message = message + " " + line
		}
	}
	return outcome{
		ExitCode: resp.StatusCode,
		Success:  success,
		Stdout:   text,
		Message:  message,
		Duration: time.Since(start),
	}
}

func cleanBody(s string) string {
	s = strings.ToValidUTF8(s, "\uFFFD")
	s = strings.Map(func(r rune) rune {
		if r == 0 {
			return -1
		}
		return r
	}, s)
	if len(s) <= maxHTTPBody && utf8.ValidString(s) {
		return s
	}
	return clip(s, maxHTTPBody)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [16]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

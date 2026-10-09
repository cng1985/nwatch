package notifier

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"net/url"
	"strconv"
	"testing"
	"time"
)

func TestSignDingTalk(t *testing.T) {
	now := time.UnixMilli(1700000000000)
	signed, err := SignDingTalk("https://oapi.dingtalk.com/robot/send?access_token=abc", "SEC123", now)
	if err != nil {
		t.Fatal(err)
	}
	u, err := url.Parse(signed)
	if err != nil {
		t.Fatal(err)
	}
	ts := strconv.FormatInt(now.UnixMilli(), 10)
	mac := hmac.New(sha256.New, []byte("SEC123"))
	_, _ = mac.Write([]byte(ts + "\n" + "SEC123"))
	want := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	if u.Query().Get("timestamp") != ts || u.Query().Get("sign") != want {
		t.Fatalf("sign mismatch: %s", signed)
	}
	plain, err := SignDingTalk("https://example.com/hook", "", now)
	if err != nil || plain != "https://example.com/hook" {
		t.Fatalf("plain %s %v", plain, err)
	}
}

func TestMaskSecret(t *testing.T) {
	if MaskSecret("https://example.com/hook/abcd") != "****abcd" {
		t.Fatal(MaskSecret("https://example.com/hook/abcd"))
	}
	if MaskSecret("") != "" || MaskSecret("ab") != "****" {
		t.Fatal("short secret")
	}
}

func TestFormatDuration(t *testing.T) {
	if FormatDuration(205) != "3分25秒" {
		t.Fatal(FormatDuration(205))
	}
}

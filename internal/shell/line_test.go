package shell

import "testing"

func feedText(t *testing.T, in string) []submission {
	t.Helper()
	var edit lineEdit
	got := edit.Feed([]byte(in))
	if len(edit.buf) != 0 && !edit.paste {
		t.Fatalf("pending %q mode %d", edit.buf, edit.mode)
	}
	return got
}

func TestLineEditEditing(t *testing.T) {
	got := feedText(t, "echx\x7fo hello\r")
	if len(got) != 1 || got[0].text != "echo hello" || got[0].fromScreen {
		t.Fatalf("%+v", got)
	}
	got = feedText(t, "foo\x15bar\n")
	if len(got) != 1 || got[0].text != "bar" {
		t.Fatalf("%+v", got)
	}
	got = feedText(t, "echo a\necho b\r\n")
	if len(got) != 2 || got[0].text != "echo a" || got[1].text != "echo b" {
		t.Fatalf("%+v", got)
	}
	got = feedText(t, "keep\x03next\r")
	if len(got) != 1 || got[0].text != "next" {
		t.Fatalf("%+v", got)
	}
	got = feedText(t, "ls\x1b[Dx\r")
	if len(got) != 1 || got[0].text != "lxs" {
		t.Fatalf("%+v", got)
	}
	got = feedText(t, "rm old\x17new\r")
	if len(got) != 1 || got[0].text != "rm new" {
		t.Fatalf("%+v", got)
	}
}

func TestLineEditHistoryAndPaste(t *testing.T) {
	got := feedText(t, "foo\x1b[A\r")
	if len(got) != 1 || !got[0].fromScreen || got[0].text != "" {
		t.Fatalf("%+v", got)
	}
	got = feedText(t, "echo\t")
	if len(got) != 0 {
		t.Fatalf("%+v", got)
	}
	var edit lineEdit
	got = edit.Feed([]byte("echo\t"))
	got = append(got, edit.Feed([]byte("\r"))...)
	if len(got) != 1 || !got[0].fromScreen || got[0].text != "echo" {
		t.Fatalf("%+v", got)
	}
	got = feedText(t, "\x1b[200~echo a\necho b\n\x1b[201~")
	if len(got) != 2 || got[0].text != "echo a" || got[1].text != "echo b" || got[0].fromScreen || got[1].fromScreen {
		t.Fatalf("%+v", got)
	}
	got = feedText(t, "中文命令\r")
	if len(got) != 1 || got[0].text != "中文命令" {
		t.Fatalf("%+v", got)
	}
}

func TestCommandFromScreen(t *testing.T) {
	cases := []struct {
		line string
		want string
	}{
		{"user@host:~$ ls -l", "ls -l"},
		{"root@localhost:~# apt update", "apt update"},
		{"user@host:~$ echo $HOME", "echo $HOME"},
		{"PS C:\\Users\\admin> Get-Process", "Get-Process"},
		{"C:\\>dir", "dir"},
		{"user@host:~$", ""},
		{"", ""},
	}
	for _, item := range cases {
		if got := commandFromScreen(item.line); got != item.want {
			t.Fatalf("%q => %q want %q", item.line, got, item.want)
		}
	}
}

func TestScreenRedraw(t *testing.T) {
	var screen screenLine
	screen.Write([]byte("\x1b[01;32muser@host\x1b[00m:~$ ls"))
	screen.Write([]byte("\ruser@host:~$ uptime\x1b[K"))
	if got := commandFromScreen(screen.Text()); got != "uptime" {
		t.Fatalf("%q", screen.Text())
	}
}

func TestSanitizeCommand(t *testing.T) {
	if got := sanitizeCommand("  echo\tok\x00 \n"); got != "echo ok" {
		t.Fatalf("%q", got)
	}
	long := stringsRepeat("a", 3000)
	if got := sanitizeCommand(long); len(got) != 2000 {
		t.Fatalf("len %d", len(got))
	}
}

func stringsRepeat(s string, n int) string {
	out := make([]byte, 0, len(s)*n)
	for i := 0; i < n; i++ {
		out = append(out, s...)
	}
	return string(out)
}

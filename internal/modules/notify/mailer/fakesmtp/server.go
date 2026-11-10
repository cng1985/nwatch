package fakesmtp

import (
	"bufio"
	"io"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

type Server struct {
	Addr     string
	failLeft atomic.Int32
	mu       sync.Mutex
	messages []string
	ln       net.Listener
}

func Start(t testing.TB) *Server {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	s := &Server{Addr: ln.Addr().String(), ln: ln}
	go s.serve()
	t.Cleanup(func() { _ = ln.Close() })
	return s
}

func (s *Server) FailNext(n int) {
	s.failLeft.Store(int32(n))
}

func (s *Server) Messages() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]string, len(s.messages))
	copy(out, s.messages)
	return out
}

func (s *Server) serve() {
	for {
		conn, err := s.ln.Accept()
		if err != nil {
			return
		}
		go s.handle(conn)
	}
}

func (s *Server) handle(conn net.Conn) {
	defer conn.Close()
	reader := bufio.NewReader(conn)
	_, _ = io.WriteString(conn, "220 localhost ESMTP\r\n")
	inData := false
	var data strings.Builder
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return
		}
		if inData {
			if strings.TrimRight(line, "\r\n") == "." {
				inData = false
				if s.reject() {
					_, _ = io.WriteString(conn, "450 暂时失败，请稍后重试\r\n")
					data.Reset()
					continue
				}
				s.mu.Lock()
				s.messages = append(s.messages, data.String())
				s.mu.Unlock()
				data.Reset()
				_, _ = io.WriteString(conn, "250 OK\r\n")
				continue
			}
			data.WriteString(line)
			continue
		}
		cmd := strings.ToUpper(strings.TrimSpace(line))
		switch {
		case strings.HasPrefix(cmd, "EHLO"), strings.HasPrefix(cmd, "HELO"):
			_, _ = io.WriteString(conn, "250-localhost\r\n250-AUTH PLAIN LOGIN\r\n250 OK\r\n")
		case strings.HasPrefix(cmd, "AUTH"):
			_, _ = io.WriteString(conn, "235 OK\r\n")
		case strings.HasPrefix(cmd, "DATA"):
			_, _ = io.WriteString(conn, "354 Go ahead\r\n")
			inData = true
		case strings.HasPrefix(cmd, "QUIT"):
			_, _ = io.WriteString(conn, "221 Bye\r\n")
			return
		default:
			_, _ = io.WriteString(conn, "250 OK\r\n")
		}
	}
}

func (s *Server) reject() bool {
	for {
		cur := s.failLeft.Load()
		if cur <= 0 {
			return false
		}
		if s.failLeft.CompareAndSwap(cur, cur-1) {
			return true
		}
	}
}

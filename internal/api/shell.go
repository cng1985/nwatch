package api

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/cng1985/nwatch/internal/model"
	"github.com/cng1985/nwatch/internal/rbac"
	"github.com/cng1985/nwatch/internal/shell"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var shellUpgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin:     sameOrigin,
}

func (s *Server) shellInfo(c *gin.Context) {
	ok(c, s.shellHub.Info())
}

func (s *Server) recordShellCommand(username, remote, sessionID, command string) {
	row := model.ShellCommand{
		Username:  username,
		Remote:    remote,
		SessionID: sessionID,
		Command:   command,
		CreatedAt: time.Now(),
	}
	if err := s.db.Create(&row).Error; err != nil {
		slog.Error("记录终端命令失败", "err", err.Error(), "user", username, "session", sessionID)
		return
	}
	slog.Info("在线终端执行命令: "+command, "user", username, "remote", remote, "session", sessionID)
}

func (s *Server) shellCommands(c *gin.Context) {
	principal := principalOf(c)
	if principal == nil || (!principal.Can(rbac.PermShellUse) && !principal.Can(rbac.PermLogRead)) {
		fail(c, http.StatusForbidden, "没有权限")
		return
	}
	page, size := pageParams(c)
	q := s.db.Model(&model.ShellCommand{})
	if userName := strings.TrimSpace(c.Query("username")); userName != "" {
		q = q.Where("username = ?", userName)
	}
	if kw := strings.TrimSpace(c.Query("keyword")); kw != "" {
		q = q.Where("command LIKE ?", "%"+kw+"%")
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		fail(c, http.StatusInternalServerError, "查询失败")
		return
	}
	var items []model.ShellCommand
	if err := q.Order("created_at desc, id desc").Offset((page - 1) * size).Limit(size).Find(&items).Error; err != nil {
		fail(c, http.StatusInternalServerError, "查询失败")
		return
	}
	ok(c, gin.H{"items": items, "total": total})
}

func (s *Server) shellTicket(c *gin.Context) {
	token, err := s.shellHub.Issue(c.GetString("username"))
	if err != nil {
		fail(c, shellStatus(err), err.Error())
		return
	}
	ok(c, gin.H{"ticket": token, "expiresIn": 30})
}

func (s *Server) shellConnect(c *gin.Context) {
	if !sameOrigin(c.Request) {
		fail(c, http.StatusForbidden, "不允许跨站连接终端")
		return
	}
	if !s.shellHub.Enabled() {
		fail(c, http.StatusForbidden, shell.ErrDisabled.Error())
		return
	}
	username, err := s.shellHub.Redeem(strings.TrimSpace(c.Query("ticket")))
	if err != nil {
		fail(c, shellStatus(err), err.Error())
		return
	}
	cols, rows := shell.NormalizeSize(atoi(c.Query("cols"), 0), atoi(c.Query("rows"), 0))
	conn, err := shellUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		slog.Warn("在线终端升级失败", "err", err.Error(), "user", username)
		return
	}
	sess, err := s.shellHub.Open(username, c.ClientIP(), cols, rows)
	if err != nil {
		payload, _ := json.Marshal(map[string]any{"type": "error", "message": err.Error()})
		_ = conn.WriteMessage(websocket.TextMessage, payload)
		_ = conn.Close()
		return
	}
	serveShell(conn, sess, cols, rows)
}

func serveShell(conn *websocket.Conn, sess *shell.Session, cols, rows int) {
	var writeMu sync.Mutex
	write := func(kind int, data []byte) error {
		writeMu.Lock()
		defer writeMu.Unlock()
		_ = conn.SetWriteDeadline(time.Now().Add(15 * time.Second))
		return conn.WriteMessage(kind, data)
	}

	outputDone := make(chan struct{})
	go func() {
		defer close(outputDone)
		buf := make([]byte, 32*1024)
		for {
			n, err := sess.Read(buf)
			if n > 0 {
				chunk := append([]byte(nil), buf[:n]...)
				if werr := write(websocket.BinaryMessage, chunk); werr != nil {
					return
				}
			}
			if err != nil {
				payload, _ := json.Marshal(map[string]any{"type": "exit", "reason": sess.Reason()})
				_ = write(websocket.TextMessage, payload)
				_ = conn.Close()
				return
			}
		}
	}()

	pingDone := make(chan struct{})
	go func() {
		defer close(pingDone)
		ticker := time.NewTicker(20 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-sess.Done():
				return
			case <-outputDone:
				return
			case <-ticker.C:
				writeMu.Lock()
				err := conn.WriteControl(websocket.PingMessage, []byte("p"), time.Now().Add(5*time.Second))
				writeMu.Unlock()
				if err != nil {
					return
				}
			}
		}
	}()

	ready, _ := json.Marshal(map[string]any{
		"type":      "ready",
		"sessionId": sess.ID(),
		"shell":     sess.Shell(),
		"cols":      cols,
		"rows":      rows,
	})
	_ = write(websocket.TextMessage, ready)

	conn.SetReadLimit(256 * 1024)
	_ = conn.SetReadDeadline(time.Now().Add(70 * time.Second))
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(70 * time.Second))
	})
	for {
		kind, data, err := conn.ReadMessage()
		if err != nil {
			break
		}
		sess.Touch()
		switch kind {
		case websocket.BinaryMessage:
			if _, err := sess.Write(data); err != nil {
				sess.CloseWith("exit")
				<-outputDone
				<-pingDone
				_ = conn.Close()
				return
			}
		case websocket.TextMessage:
			var msg struct {
				Type string `json:"type"`
				Cols int    `json:"cols"`
				Rows int    `json:"rows"`
			}
			if json.Unmarshal(data, &msg) == nil && msg.Type == "resize" {
				_ = sess.Resize(msg.Cols, msg.Rows)
			}
		}
	}
	sess.CloseWith("client")
	<-outputDone
	<-pingDone
	_ = conn.Close()
}

func shellStatus(err error) int {
	switch {
	case errors.Is(err, shell.ErrDisabled):
		return http.StatusForbidden
	case errors.Is(err, shell.ErrNoShell), errors.Is(err, shell.ErrUnsupported):
		return http.StatusServiceUnavailable
	case errors.Is(err, shell.ErrBusy), errors.Is(err, shell.ErrTooManyTickets):
		return http.StatusTooManyRequests
	case errors.Is(err, shell.ErrTicket):
		return http.StatusUnauthorized
	default:
		return http.StatusBadRequest
	}
}

func sameOrigin(r *http.Request) bool {
	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if origin == "" {
		return true
	}
	parsed, err := url.Parse(origin)
	if err != nil || parsed.Host == "" {
		return false
	}
	return strings.EqualFold(parsed.Host, r.Host)
}

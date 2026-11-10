package shell

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cng1985/nwatch/internal/platform/errors"
	"github.com/cng1985/nwatch/internal/platform/middleware"
	"github.com/cng1985/nwatch/internal/platform/response"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// Controller 负责终端路由注册、票据签发与 WebSocket 适配。
type Controller struct {
	resource *Resource
	hub      *Hub
}

func NewController(resource *Resource, hub *Hub) *Controller {
	hub.SetRecorder(resource.repo.RecordCommand)
	return &Controller{resource: resource, hub: hub}
}

var shellUpgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin:     sameOrigin,
}

// RegisterRoutes 注册终端路由。WebSocket 走票据认证，注册在公开路由组。
func (ctl *Controller) RegisterRoutes(public *gin.RouterGroup, authed *gin.RouterGroup) {
	authed.GET("/shell", middleware.Permit("shell.use"), ctl.info)
	authed.GET("/shell/commands", ctl.commands)
	authed.POST("/shell/ticket", middleware.Permit("shell.use"), ctl.ticket)
	public.GET("/shell/ws", ctl.connect)
}

func (ctl *Controller) info(c *gin.Context) {
	result, err := ctl.resource.Info(c.Request.Context())
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.SuccessResponse(c, result)
}

func (ctl *Controller) commands(c *gin.Context) {
	// 命令审计允许终端使用者和日志查看者读取。
	principal := middleware.PrincipalOf(c)
	if principal == nil || (!principal.Can("shell.use") && !principal.Can("log.read")) {
		response.HandleError(c, errors.Forbidden("没有权限"))
		return
	}
	var search ShellCommandSearch
	if err := c.ShouldBindQuery(&search); err != nil {
		response.HandleError(c, errors.BadRequest("请求格式不正确"))
		return
	}
	page, err := ctl.resource.SearchCommands(c.Request.Context(), &search)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.SuccessPage(c, page)
}

func (ctl *Controller) ticket(c *gin.Context) {
	principal := middleware.PrincipalOf(c)
	username := ""
	if principal != nil {
		username = principal.Username
	}
	result, err := ctl.resource.IssueTicket(c.Request.Context(), username)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.SuccessResponse(c, result)
}

func (ctl *Controller) connect(c *gin.Context) {
	if !sameOrigin(c.Request) {
		response.HandleError(c, errors.Forbidden("不允许跨站连接终端"))
		return
	}
	if !ctl.hub.Enabled() {
		response.HandleError(c, errors.Forbidden(ErrDisabled.Error()))
		return
	}
	username, err := ctl.hub.Redeem(strings.TrimSpace(c.Query("ticket")))
	if err != nil {
		response.HandleError(c, shellError(err))
		return
	}
	cols, rows := NormalizeSize(atoiDefault(c.Query("cols"), 0), atoiDefault(c.Query("rows"), 0))
	conn, err := shellUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		slog.Warn("在线终端升级失败", "err", err.Error(), "user", username)
		return
	}
	sess, err := ctl.hub.Open(username, c.ClientIP(), cols, rows)
	if err != nil {
		payload, _ := json.Marshal(map[string]any{"type": "error", "message": err.Error()})
		_ = conn.WriteMessage(websocket.TextMessage, payload)
		_ = conn.Close()
		return
	}
	serveShell(conn, sess, cols, rows)
}

func atoiDefault(s string, def int) int {
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return n
}

func serveShell(conn *websocket.Conn, sess *Session, cols, rows int) {
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

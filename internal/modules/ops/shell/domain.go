package shell

// ShellCommandSearch 是终端命令审计分页查询参数。
type ShellCommandSearch struct {
	Username string `form:"username"`
	Keyword  string `form:"keyword"`
	Page     int    `form:"page"`
	PageSize int    `form:"pageSize"`
}

// ShellCommandSimple 是命令审计列表行。
type ShellCommandSimple = ShellCommand

// ShellInfoResponse 是终端能力信息。
type ShellInfoResponse = Info

// ShellTicketResponse 是终端连接票据。
type ShellTicketResponse struct {
	Ticket    string `json:"ticket"`
	ExpiresIn int    `json:"expiresIn"`
}

package transfer

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/cng1985/nwatch/internal/model"
	"github.com/xuri/excelize/v2"
)

const (
	monitorSheet = "监控"
	helpSheet    = "说明"
	maxImport    = 2000
)

type column struct {
	key      string
	header   string
	width    float64
	readOnly bool
	wrap     bool
	drop     []string
}

var monitorColumns = []column{
	{key: "id", header: "编号", width: 10},
	{key: "name", header: "名称", width: 24},
	{key: "group", header: "分组", width: 16},
	{key: "type", header: "类型", width: 14, drop: []string{"http", "tls", "tcp", "cpu", "memory", "disk", "script", "process"}},
	{key: "url", header: "地址", width: 36},
	{key: "host", header: "主机", width: 22},
	{key: "port", header: "端口", width: 10},
	{key: "method", header: "请求方法", width: 12, drop: []string{"GET", "POST", "HEAD"}},
	{key: "headers", header: "请求头", width: 36, wrap: true},
	{key: "bodyType", header: "请求体类型", width: 14, drop: []string{"json", "form", "formdata", "xml", "text"}},
	{key: "body", header: "请求体", width: 36, wrap: true},
	{key: "threshold", header: "阈值", width: 10},
	{key: "memThreshold", header: "内存阈值", width: 12},
	{key: "match", header: "匹配方式", width: 14, drop: []string{"name", "command"}},
	{key: "command", header: "命令", width: 36, wrap: true},
	{key: "workDir", header: "工作目录", width: 24},
	{key: "interval", header: "检测间隔", width: 12},
	{key: "timeout", header: "超时", width: 10},
	{key: "enabled", header: "启用", width: 10, drop: []string{"是", "否"}},
	{key: "failureThreshold", header: "连续失败次数", width: 14},
	{key: "recoveryThreshold", header: "连续恢复次数", width: 14},
	{key: "expectedStatusCodes", header: "期望状态码", width: 14},
	{key: "bodyContains", header: "响应包含", width: 18},
	{key: "bodyNotContains", header: "响应不含", width: 18},
	{key: "followRedirects", header: "跟随重定向", width: 12, drop: []string{"是", "否"}},
	{key: "tlsWarningDays", header: "证书预警天数", width: 14},
	{key: "tlsCriticalDays", header: "证书严重天数", width: 14},
	{key: "tlsNotifyDays", header: "证书通知天数", width: 20},
	{key: "notifiers", header: "通知渠道", width: 24},
	{key: "status", header: "状态", width: 12, readOnly: true},
	{key: "lastResult", header: "最近结果", width: 28, readOnly: true, wrap: true},
	{key: "availability", header: "24h可用率", width: 12, readOnly: true},
	{key: "lastCheck", header: "最近检测", width: 20, readOnly: true},
}

var headerAlias = map[string]string{
	"编号": "id", "id": "id",
	"名称": "name", "name": "name", "监控名称": "name",
	"分组": "group", "group": "group",
	"类型": "type", "type": "type",
	"地址": "url", "url": "url",
	"主机": "host", "host": "host",
	"端口": "port", "port": "port",
	"请求方法": "method", "方法": "method", "method": "method",
	"请求头": "headers", "headers": "headers",
	"请求体类型": "bodyType", "bodytype": "bodyType",
	"请求体": "body", "body": "body",
	"阈值": "threshold", "threshold": "threshold",
	"内存阈值": "memThreshold", "内存阈值mb": "memThreshold", "memthreshold": "memThreshold",
	"匹配方式": "match", "match": "match",
	"命令": "command", "command": "command",
	"工作目录": "workDir", "workdir": "workDir",
	"检测间隔": "interval", "检测间隔秒": "interval", "interval": "interval",
	"超时": "timeout", "超时秒": "timeout", "timeout": "timeout",
	"启用": "enabled", "enabled": "enabled",
	"连续失败次数": "failureThreshold", "failurethreshold": "failureThreshold",
	"连续恢复次数": "recoveryThreshold", "recoverythreshold": "recoveryThreshold",
	"期望状态码": "expectedStatusCodes", "expectedstatuscodes": "expectedStatusCodes",
	"响应包含": "bodyContains", "bodycontains": "bodyContains",
	"响应不含": "bodyNotContains", "bodynotcontains": "bodyNotContains",
	"跟随重定向": "followRedirects", "followredirects": "followRedirects",
	"证书预警天数": "tlsWarningDays", "tlswarningdays": "tlsWarningDays",
	"证书严重天数": "tlsCriticalDays", "tlscriticaldays": "tlsCriticalDays",
	"证书通知天数": "tlsNotifyDays", "tlsnotifydays": "tlsNotifyDays",
	"通知渠道": "notifiers", "notifiers": "notifiers",
	"状态": "status", "status": "status",
	"最近结果": "lastResult", "lastresult": "lastResult",
	"24h可用率": "availability", "可用率": "availability", "availability": "availability",
	"最近检测": "lastCheck", "lastcheck": "lastCheck",
}

var readOnlyField = map[string]bool{
	"status": true, "lastResult": true, "availability": true, "lastCheck": true,
}

var helpLines = []string{
	"导入时只读取「监控」表，这一页不用改。",
	"编号：本机导出的表格里会带上。编号还在，就按编号更新，名称也可以改。编号留空时按名称查找，只有一条同名监控就更新，没有就新建。同名监控有多条时必须填写编号。",
	"从别的环境导入时，先清空编号列，避免改到本机同编号的监控。",
	"导入只会新建和更新，不会删除表格里没有的监控。",
	"分组：填写分组名称。没有这个分组时会自动创建。留空表示不分组。",
	"类型：http、tls、tcp、cpu、memory、disk、script、process。也可以写证书、内存、磁盘、脚本、进程、TCP端口。https 地址的类型仍填 http。",
	"地址：HTTP 填完整 URL，证书可以填域名。只写了域名时，类型为 http 会自动补上 http://。",
	"主机：TCP 填主机，磁盘填绝对路径，进程填进程名。",
	"端口：TCP 必填。证书不是 443 时填写。",
	"请求方法：GET、POST、HEAD。请求头可以写 JSON，例如 {\"Authorization\":\"Bearer token\"}，也可以每行一个「名称: 值」。",
	"请求体类型：json、form、formdata、xml、text。请求体是 POST 的内容。",
	"阈值：CPU、内存、磁盘填写使用率百分比。进程填写 CPU 百分比，0 表示不按 CPU 告警。",
	"内存阈值：进程内存，单位 MB，0 表示不按内存告警。",
	"匹配方式：进程填 name 或 command。name 只按进程名，command 还要求命令行包含「命令」列。也可以写进程名、命令行。",
	"命令：进程要匹配的命令行，或脚本内容。工作目录是脚本的绝对路径，可以留空。",
	"检测间隔和超时的单位是秒。间隔在 5 秒到 7 天之间，超时在 1 到 120 秒之间，并且必须小于检测间隔。",
	"启用、跟随重定向：填写是或否。留空时，新建视为启用、HTTP 跟随重定向；更新则保持原来的值。",
	"连续失败次数和连续恢复次数：1 到 100。留空时失败 3 次、恢复 1 次。",
	"期望状态码留给 HTTP，多个用逗号，留空为 200。响应包含、响应不含用于 HTTP 响应或脚本输出。",
	"证书预警天数、证书严重天数、证书通知天数留给证书监控。通知天数例如 30,14,7,3,1,0。",
	"通知渠道：填写已经存在的渠道名称，多个用逗号或顿号分隔。留空表示不绑定。不会新建渠道。",
	"状态、最近结果、24h可用率、最近检测只供查看，导入时忽略。",
}

// SheetMonitor 是表格里的一条监控配置。状态类列不在这里。
type SheetMonitor struct {
	Line                int
	ID                  uint
	Name                string
	Group               string
	Type                string
	URL                 string
	Host                string
	Port                int
	Method              string
	Headers             map[string]string
	Body                string
	BodyType            string
	Threshold           float64
	MemThreshold        float64
	Match               string
	Command             string
	WorkDir             string
	Interval            int
	Timeout             int
	Enabled             *bool
	FailureThreshold    int
	RecoveryThreshold   int
	ExpectedStatusCodes string
	BodyContains        string
	BodyNotContains     string
	FollowRedirects     *bool
	TLSWarningDays      int
	TLSCriticalDays     int
	TLSNotifyDays       string
	Notifiers           []string
}

// SheetError 是表格本身的问题，例如缺表头或文件打不开。
type SheetError struct{ msg string }

func (e *SheetError) Error() string {
	if e == nil {
		return "表格内容不正确"
	}
	return e.msg
}

func sheetErr(msg string) error { return &SheetError{msg: msg} }

// RowErrors 汇总每一行的校验错误。
type RowErrors struct{ Messages []string }

func (e *RowErrors) Error() string {
	if e == nil || len(e.Messages) == 0 {
		return "导入内容有误"
	}
	msgs := e.Messages
	if len(msgs) > 20 {
		extra := len(msgs) - 20
		msgs = append(append([]string{}, msgs[:20]...), fmt.Sprintf("另有 %d 条错误", extra))
	}
	return strings.Join(msgs, "；")
}

// MonitorsToXLSX 把监控写成 xlsx。availability 的键是监控编号，值如 99.93%。
func MonitorsToXLSX(monitors []model.Monitor, availability map[uint]string, loc *time.Location) ([]byte, error) {
	if loc == nil {
		loc = time.Local
	}
	f := excelize.NewFile()
	defer f.Close()
	if err := f.SetSheetName("Sheet1", monitorSheet); err != nil {
		return nil, err
	}
	headerStyle, err := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Color: "FFFFFF"},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"0F9F6E"}, Pattern: 1},
		Alignment: &excelize.Alignment{Vertical: "center", Horizontal: "center"},
	})
	if err != nil {
		return nil, err
	}
	readStyle, err := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Color: "FFFFFF"},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"60707B"}, Pattern: 1},
		Alignment: &excelize.Alignment{Vertical: "center", Horizontal: "center"},
	})
	if err != nil {
		return nil, err
	}
	wrapStyle, err := f.NewStyle(&excelize.Style{
		Alignment: &excelize.Alignment{WrapText: true, Vertical: "top"},
	})
	if err != nil {
		return nil, err
	}
	for i, col := range monitorColumns {
		cell, err := excelize.CoordinatesToCellName(i+1, 1)
		if err != nil {
			return nil, err
		}
		if err := f.SetCellValue(monitorSheet, cell, col.header); err != nil {
			return nil, err
		}
		style := headerStyle
		if col.readOnly {
			style = readStyle
		}
		if err := f.SetCellStyle(monitorSheet, cell, cell, style); err != nil {
			return nil, err
		}
		name, err := excelize.ColumnNumberToName(i + 1)
		if err != nil {
			return nil, err
		}
		if col.wrap {
			if err := f.SetColStyle(monitorSheet, name, wrapStyle); err != nil {
				return nil, err
			}
		}
		if err := f.SetColWidth(monitorSheet, name, name, col.width); err != nil {
			return nil, err
		}
	}
	if err := f.SetRowHeight(monitorSheet, 1, 22); err != nil {
		return nil, err
	}
	for r, m := range monitors {
		for c, col := range monitorColumns {
			cell, err := excelize.CoordinatesToCellName(c+1, r+2)
			if err != nil {
				return nil, err
			}
			value := cellValue(col.key, m, availability, loc)
			if value == nil {
				continue
			}
			if err := f.SetCellValue(monitorSheet, cell, value); err != nil {
				return nil, err
			}
		}
	}
	last, err := excelize.CoordinatesToCellName(len(monitorColumns), 1)
	if err != nil {
		return nil, err
	}
	if err := f.AutoFilter(monitorSheet, "A1:"+last, nil); err != nil {
		return nil, err
	}
	end := len(monitors) + 20
	if end < 200 {
		end = 200
	}
	for i, col := range monitorColumns {
		if len(col.drop) == 0 {
			continue
		}
		start, err := excelize.CoordinatesToCellName(i+1, 2)
		if err != nil {
			return nil, err
		}
		stop, err := excelize.CoordinatesToCellName(i+1, end)
		if err != nil {
			return nil, err
		}
		dv := excelize.NewDataValidation(true)
		dv.Sqref = start + ":" + stop
		if err := dv.SetDropList(col.drop); err != nil {
			return nil, err
		}
		if err := f.AddDataValidation(monitorSheet, dv); err != nil {
			return nil, err
		}
	}
	if err := f.SetPanes(monitorSheet, &excelize.Panes{
		Freeze: true, YSplit: 1, TopLeftCell: "A2", ActivePane: "bottomLeft",
	}); err != nil {
		return nil, err
	}
	if _, err := f.NewSheet(helpSheet); err != nil {
		return nil, err
	}
	title, err := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true}})
	if err != nil {
		return nil, err
	}
	wrap, err := f.NewStyle(&excelize.Style{Alignment: &excelize.Alignment{WrapText: true, Vertical: "center"}})
	if err != nil {
		return nil, err
	}
	for i, line := range helpLines {
		cell := "A" + strconv.Itoa(i+1)
		if err := f.SetCellValue(helpSheet, cell, line); err != nil {
			return nil, err
		}
		style := wrap
		if i == 0 {
			style = title
		}
		if err := f.SetCellStyle(helpSheet, cell, cell, style); err != nil {
			return nil, err
		}
		if err := f.SetRowHeight(helpSheet, i+1, 32); err != nil {
			return nil, err
		}
	}
	if err := f.SetColWidth(helpSheet, "A", "A", 120); err != nil {
		return nil, err
	}
	idx, err := f.GetSheetIndex(monitorSheet)
	if err != nil {
		return nil, err
	}
	f.SetActiveSheet(idx)
	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func cellValue(key string, m model.Monitor, availability map[uint]string, loc *time.Location) any {
	switch key {
	case "id":
		if m.ID == 0 {
			return nil
		}
		return m.ID
	case "name":
		return m.Name
	case "group":
		if m.Group == nil {
			return nil
		}
		return m.Group.Name
	case "type":
		return m.Type
	case "url":
		return blankNil(m.URL)
	case "host":
		return blankNil(m.Host)
	case "port":
		return m.Port
	case "method":
		return blankNil(m.Method)
	case "headers":
		return blankNil(formatHeaders(m.Headers))
	case "bodyType":
		return blankNil(m.BodyType)
	case "body":
		return blankNil(m.Body)
	case "threshold":
		return m.Threshold
	case "memThreshold":
		return m.MemThreshold
	case "match":
		return blankNil(m.Match)
	case "command":
		return blankNil(m.Command)
	case "workDir":
		return blankNil(m.WorkDir)
	case "interval":
		return m.Interval
	case "timeout":
		return m.Timeout
	case "enabled":
		return yesNo(m.Enabled)
	case "failureThreshold":
		return m.FailureThreshold
	case "recoveryThreshold":
		return m.RecoveryThreshold
	case "expectedStatusCodes":
		return blankNil(m.ExpectedStatusCodes)
	case "bodyContains":
		return blankNil(m.BodyContains)
	case "bodyNotContains":
		return blankNil(m.BodyNotContains)
	case "followRedirects":
		return yesNo(m.FollowRedirects)
	case "tlsWarningDays":
		return m.TLSWarningDays
	case "tlsCriticalDays":
		return m.TLSCriticalDays
	case "tlsNotifyDays":
		return blankNil(m.TLSNotifyDays)
	case "notifiers":
		return blankNil(joinNotifiers(m.Notifiers))
	case "status":
		return statusText(m.Status)
	case "lastResult":
		return blankNil(lastResult(m))
	case "availability":
		return blankNil(availability[m.ID])
	case "lastCheck":
		if m.LastCheckAt == nil {
			return nil
		}
		return m.LastCheckAt.In(loc).Format("2006-01-02 15:04:05")
	default:
		return nil
	}
}

func blankNil(s string) any {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return s
}

func yesNo(v bool) string {
	if v {
		return "是"
	}
	return "否"
}

func statusText(status string) string {
	switch status {
	case model.StatusUp:
		return "正常运行"
	case model.StatusDown:
		return "服务异常"
	case model.StatusPaused:
		return "已暂停"
	default:
		return "等待检测"
	}
}

func lastResult(m model.Monitor) string {
	switch m.Type {
	case model.TypeCPU, model.TypeMemory, model.TypeDisk, model.TypeScript, model.TypeProcess:
		return m.LastMessage
	}
	if m.LastResponseTime > 0 {
		return strconv.Itoa(m.LastResponseTime) + " ms"
	}
	return m.LastMessage
}

func formatHeaders(headers map[string]string) string {
	if len(headers) == 0 {
		return ""
	}
	raw, err := json.Marshal(headers)
	if err != nil {
		return ""
	}
	return string(raw)
}

func joinNotifiers(items []model.Notifier) string {
	names := make([]string, 0, len(items))
	for _, item := range items {
		if strings.TrimSpace(item.Name) != "" {
			names = append(names, item.Name)
		}
	}
	sort.Strings(names)
	return strings.Join(names, ",")
}

// ParseMonitorXLSX 读取监控表。没有数据行时返回空切片。
func ParseMonitorXLSX(r io.Reader) ([]SheetMonitor, error) {
	f, err := excelize.OpenReader(r)
	if err != nil {
		return nil, sheetErr("请上传 xlsx 格式的 Excel 文件")
	}
	defer f.Close()
	sheet := pickSheet(f)
	if sheet == "" {
		return nil, sheetErr("表格里没有工作表")
	}
	rows, err := f.GetRows(sheet)
	if err != nil {
		return nil, sheetErr("无法读取表格")
	}
	if len(rows) == 0 {
		return nil, sheetErr("表格是空的")
	}
	keys, err := headerKeys(rows[0])
	if err != nil {
		return nil, err
	}
	var out []SheetMonitor
	var rowErrs []string
	for i := 1; i < len(rows); i++ {
		row, keep, err := parseRow(i+1, keys, rows[i])
		if err != nil {
			rowErrs = append(rowErrs, fmt.Sprintf("第%d行：%s", i+1, err.Error()))
			continue
		}
		if !keep {
			continue
		}
		out = append(out, row)
		if len(out) > maxImport {
			return nil, sheetErr(fmt.Sprintf("一次最多导入 %d 条监控", maxImport))
		}
	}
	if len(rowErrs) > 0 {
		return nil, &RowErrors{Messages: rowErrs}
	}
	return out, nil
}

func pickSheet(f *excelize.File) string {
	sheets := f.GetSheetList()
	for _, name := range sheets {
		if name == monitorSheet || strings.EqualFold(name, "monitors") {
			return name
		}
	}
	for _, name := range sheets {
		if name != helpSheet {
			return name
		}
	}
	if len(sheets) > 0 {
		return sheets[0]
	}
	return ""
}

func headerKeys(cells []string) ([]string, error) {
	if len(cells) > 80 {
		cells = cells[:80]
	}
	keys := make([]string, len(cells))
	seen := map[string]struct{}{}
	hasName := false
	for i, cell := range cells {
		key, ok := headerAlias[canonHeader(cell)]
		if !ok {
			continue
		}
		if _, dup := seen[key]; dup {
			return nil, sheetErr("表头「" + strings.TrimSpace(cell) + "」重复")
		}
		seen[key] = struct{}{}
		keys[i] = key
		if key == "name" {
			hasName = true
		}
	}
	if !hasName {
		return nil, sheetErr("表格需要包含「名称」列")
	}
	return keys, nil
}

func parseRow(line int, keys, cells []string) (SheetMonitor, bool, error) {
	values := map[string]string{}
	blank := true
	for i, key := range keys {
		if key == "" || readOnlyField[key] {
			continue
		}
		val := ""
		if i < len(cells) {
			val = strings.TrimSpace(cells[i])
		}
		if val != "" {
			blank = false
		}
		values[key] = val
	}
	if blank {
		return SheetMonitor{}, false, nil
	}
	row := SheetMonitor{Line: line, Name: values["name"]}
	if row.Name == "" {
		return SheetMonitor{}, false, errors.New("名称不能为空")
	}
	id, err := parseUintCell(values["id"], "编号")
	if err != nil {
		return SheetMonitor{}, false, err
	}
	row.ID = uint(id)
	originalType := strings.ToLower(strings.TrimSpace(values["type"]))
	row.Type = canonType(values["type"])
	row.Group = values["group"]
	row.URL = values["url"]
	if row.URL != "" && !strings.Contains(row.URL, "://") && row.Type == model.TypeHTTP {
		if strings.ReplaceAll(originalType, " ", "") == "https" {
			row.URL = "https://" + row.URL
		} else {
			row.URL = "http://" + row.URL
		}
	}
	row.Host = values["host"]
	row.Port, err = parseIntCell(values["port"], "端口")
	if err != nil {
		return SheetMonitor{}, false, err
	}
	row.Method = values["method"]
	row.Headers, err = parseHeaders(values["headers"])
	if err != nil {
		return SheetMonitor{}, false, err
	}
	row.BodyType = canonBodyType(values["bodyType"])
	row.Body = values["body"]
	row.Threshold, err = parseFloatCell(values["threshold"], "阈值")
	if err != nil {
		return SheetMonitor{}, false, err
	}
	row.MemThreshold, err = parseFloatCell(values["memThreshold"], "内存阈值")
	if err != nil {
		return SheetMonitor{}, false, err
	}
	row.Match = canonMatch(values["match"])
	row.Command = values["command"]
	row.WorkDir = values["workDir"]
	row.Interval, err = parseIntCell(values["interval"], "检测间隔")
	if err != nil {
		return SheetMonitor{}, false, err
	}
	row.Timeout, err = parseIntCell(values["timeout"], "超时")
	if err != nil {
		return SheetMonitor{}, false, err
	}
	enabled, ok, err := parseBoolCell(values["enabled"], "启用")
	if err != nil {
		return SheetMonitor{}, false, err
	}
	if ok {
		row.Enabled = &enabled
	}
	row.FailureThreshold, err = parseIntCell(values["failureThreshold"], "连续失败次数")
	if err != nil {
		return SheetMonitor{}, false, err
	}
	row.RecoveryThreshold, err = parseIntCell(values["recoveryThreshold"], "连续恢复次数")
	if err != nil {
		return SheetMonitor{}, false, err
	}
	row.ExpectedStatusCodes = values["expectedStatusCodes"]
	row.BodyContains = values["bodyContains"]
	row.BodyNotContains = values["bodyNotContains"]
	follow, ok, err := parseBoolCell(values["followRedirects"], "跟随重定向")
	if err != nil {
		return SheetMonitor{}, false, err
	}
	if ok {
		row.FollowRedirects = &follow
	}
	row.TLSWarningDays, err = parseIntCell(values["tlsWarningDays"], "证书预警天数")
	if err != nil {
		return SheetMonitor{}, false, err
	}
	row.TLSCriticalDays, err = parseIntCell(values["tlsCriticalDays"], "证书严重天数")
	if err != nil {
		return SheetMonitor{}, false, err
	}
	row.TLSNotifyDays = values["tlsNotifyDays"]
	row.Notifiers = splitList(values["notifiers"])
	return row, true, nil
}

func canonHeader(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "\uFEFF")
	s = strings.ReplaceAll(s, " ", "")
	s = strings.ReplaceAll(s, "\n", "")
	s = strings.NewReplacer("（", "", "）", "", "(", "", ")", "", "：", "", ":", "", "*", "").Replace(s)
	return strings.ToLower(s)
}

func canonType(raw string) string {
	s := strings.ToLower(strings.TrimSpace(raw))
	compact := strings.ReplaceAll(s, " ", "")
	switch compact {
	case "http", "https":
		return model.TypeHTTP
	case "tls", "证书", "https证书":
		return model.TypeTLS
	case "tcp", "tcp端口":
		return model.TypeTCP
	case "cpu":
		return model.TypeCPU
	case "memory", "内存":
		return model.TypeMemory
	case "disk", "磁盘":
		return model.TypeDisk
	case "script", "脚本":
		return model.TypeScript
	case "process", "进程":
		return model.TypeProcess
	default:
		return s
	}
}

func canonBodyType(raw string) string {
	s := strings.ToLower(strings.TrimSpace(raw))
	compact := strings.NewReplacer("-", "", "_", "", " ", "").Replace(s)
	switch compact {
	case "formdata", "multipart":
		return "formdata"
	case "", "json", "form", "xml", "text":
		return compact
	default:
		return s
	}
}

func canonMatch(raw string) string {
	s := strings.ToLower(strings.TrimSpace(raw))
	s = strings.ReplaceAll(s, " ", "")
	switch s {
	case "":
		return ""
	case "name", "进程名", "只按进程名":
		return model.MatchName
	case "command", "命令行", "进程名和命令行":
		return model.MatchCommand
	default:
		return strings.TrimSpace(raw)
	}
}

func parseHeaders(raw string) (map[string]string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	if strings.HasPrefix(raw, "{") {
		var typed map[string]string
		if err := json.Unmarshal([]byte(raw), &typed); err == nil {
			return typed, nil
		}
		var generic map[string]any
		if err := json.Unmarshal([]byte(raw), &generic); err != nil {
			return nil, errors.New("请求头需要是 JSON 对象，或每行一个「名称: 值」")
		}
		out := make(map[string]string, len(generic))
		for k, v := range generic {
			if v == nil {
				out[k] = ""
				continue
			}
			if s, ok := v.(string); ok {
				out[k] = s
				continue
			}
			out[k] = strings.TrimSpace(fmt.Sprint(v))
		}
		return out, nil
	}
	raw = strings.ReplaceAll(raw, "\r\n", "\n")
	raw = strings.ReplaceAll(raw, "\r", "\n")
	out := map[string]string{}
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		key, val, ok := strings.Cut(line, ":")
		if !ok {
			key, val, ok = strings.Cut(line, "=")
		}
		if !ok || strings.TrimSpace(key) == "" {
			return nil, errors.New("请求头需要是 JSON 对象，或每行一个「名称: 值」")
		}
		out[strings.TrimSpace(key)] = strings.TrimSpace(val)
	}
	return out, nil
}

func splitList(raw string) []string {
	raw = strings.NewReplacer("\r\n", ",", "\n", ",", "，", ",", "、", ",", ";", ",", "；", ",").Replace(raw)
	var out []string
	seen := map[string]struct{}{}
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if _, ok := seen[part]; ok {
			continue
		}
		seen[part] = struct{}{}
		out = append(out, part)
	}
	return out
}

func parseBoolCell(raw, label string) (bool, bool, error) {
	s := strings.ToLower(strings.TrimSpace(raw))
	switch s {
	case "":
		return false, false, nil
	case "1", "true", "yes", "y", "是", "启用", "开", "开启":
		return true, true, nil
	case "0", "false", "no", "n", "否", "停用", "关", "关闭":
		return false, true, nil
	default:
		return false, false, fmt.Errorf("%s请填写是或否", label)
	}
}

func parseFloatCell(raw, label string) (float64, error) {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimSuffix(raw, "%")
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, nil
	}
	n, err := strconv.ParseFloat(raw, 64)
	if err != nil || math.IsNaN(n) || math.IsInf(n, 0) {
		return 0, fmt.Errorf("%s需要是数字", label)
	}
	return n, nil
}

func parseIntCell(raw, label string) (int, error) {
	n, err := parseFloatCell(raw, label)
	if err != nil {
		return 0, err
	}
	if math.Abs(n-math.Round(n)) > 1e-6 {
		return 0, fmt.Errorf("%s需要是整数", label)
	}
	rounded := math.Round(n)
	if rounded > math.MaxInt32 || rounded < 0 {
		return 0, fmt.Errorf("%s不合法", label)
	}
	return int(rounded), nil
}

func parseUintCell(raw, label string) (uint64, error) {
	n, err := parseFloatCell(raw, label)
	if err != nil {
		return 0, err
	}
	if math.Abs(n-math.Round(n)) > 1e-6 || n < 0 || n > float64(math.MaxUint32) {
		return 0, fmt.Errorf("%s不合法", label)
	}
	return uint64(math.Round(n)), nil
}

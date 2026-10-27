package transfer

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/cng1985/nwatch/internal/model"
	"github.com/cng1985/nwatch/internal/monitor"
	"github.com/xuri/excelize/v2"
	"gorm.io/gorm"
)

const requestSheet = "网络请求"

var requestColumns = []column{
	{key: "name", header: "名称", width: 24},
	{key: "group", header: "分组", width: 16},
	{key: "type", header: "类型", width: 12, drop: []string{"http", "https"}},
	{key: "url", header: "地址", width: 42},
}

var requestHelp = []string{
	"导入时只读取「网络请求」表，这一页不用改。",
	"表格只有四列：名称、分组、类型、地址。",
	"按名称更新已有的网络请求。没有同名监控就新建。不会删除表格里没有的行。",
	"改了名称会当成一条新的监控，原来的仍会保留。同名监控有多条时，这一行会失败。",
	"已有同名监控如果不是网络请求，这一行也会失败，整次导入不会写入。",
	"分组：填写分组名称。没有这个分组时会自动创建。留空表示不分组。",
	"类型：填 http 或 https。留空时按地址里的协议判断。",
	"地址：可以写完整 URL。没写 http:// 或 https:// 时，类型为 https 会补 https://，否则补 http://。",
	"类型和地址的协议不一致时，以类型为准。",
	"新建的监控使用系统默认检测间隔和超时，请求方法是 GET。",
	"更新时只改名称、分组、类型和地址，检测间隔、请求方法、请求头、请求体和通知渠道保持不变。",
}

// SheetRequest 是网络请求表格里的一行，只含名称、分组、类型和地址。
type SheetRequest struct {
	Line     int
	Name     string
	Group    string
	HasGroup bool
	URL      string
}

// RequestsToXLSX 把 HTTP/HTTPS 监控写成只含四列的表格。
func RequestsToXLSX(monitors []model.Monitor) ([]byte, error) {
	f := excelize.NewFile()
	defer f.Close()
	if err := f.SetSheetName("Sheet1", requestSheet); err != nil {
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
	for i, col := range requestColumns {
		cell, err := excelize.CoordinatesToCellName(i+1, 1)
		if err != nil {
			return nil, err
		}
		if err := f.SetCellValue(requestSheet, cell, col.header); err != nil {
			return nil, err
		}
		if err := f.SetCellStyle(requestSheet, cell, cell, headerStyle); err != nil {
			return nil, err
		}
		name, err := excelize.ColumnNumberToName(i + 1)
		if err != nil {
			return nil, err
		}
		if err := f.SetColWidth(requestSheet, name, name, col.width); err != nil {
			return nil, err
		}
	}
	if err := f.SetRowHeight(requestSheet, 1, 22); err != nil {
		return nil, err
	}
	row := 2
	for _, m := range monitors {
		if m.Type != model.TypeHTTP {
			continue
		}
		values := []any{m.Name, requestGroup(m), requestKind(m), m.URL}
		for c, value := range values {
			if value == "" {
				continue
			}
			cell, err := excelize.CoordinatesToCellName(c+1, row)
			if err != nil {
				return nil, err
			}
			if err := f.SetCellValue(requestSheet, cell, value); err != nil {
				return nil, err
			}
		}
		row++
	}
	last, err := excelize.CoordinatesToCellName(len(requestColumns), 1)
	if err != nil {
		return nil, err
	}
	if err := f.AutoFilter(requestSheet, "A1:"+last, nil); err != nil {
		return nil, err
	}
	end := row + 20
	if end < 200 {
		end = 200
	}
	for i, col := range requestColumns {
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
		if err := f.AddDataValidation(requestSheet, dv); err != nil {
			return nil, err
		}
	}
	if err := f.SetPanes(requestSheet, &excelize.Panes{
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
	for i, line := range requestHelp {
		cell := "A" + fmt.Sprintf("%d", i+1)
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
	idx, err := f.GetSheetIndex(requestSheet)
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

func requestGroup(m model.Monitor) string {
	if m.Group == nil {
		return ""
	}
	return m.Group.Name
}

func requestKind(m model.Monitor) string {
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(m.URL)), "https://") {
		return "https"
	}
	return "http"
}

// ParseRequestXLSX 读取网络请求表。没有数据行时返回空切片。
func ParseRequestXLSX(r io.Reader) ([]SheetRequest, error) {
	f, err := excelize.OpenReader(r)
	if err != nil {
		return nil, sheetErr("请上传 xlsx 格式的 Excel 文件")
	}
	defer f.Close()
	sheet, err := pickRequestSheet(f)
	if err != nil {
		return nil, err
	}
	rows, err := f.GetRows(sheet)
	if err != nil {
		return nil, sheetErr("无法读取表格")
	}
	if len(rows) == 0 {
		return nil, sheetErr("表格是空的")
	}
	keys, err := requestHeaderKeys(rows[0])
	if err != nil {
		return nil, err
	}
	var out []SheetRequest
	var rowErrs []string
	for i := 1; i < len(rows); i++ {
		row, keep, err := parseRequestRow(i+1, keys, rows[i])
		if err != nil {
			rowErrs = append(rowErrs, fmt.Sprintf("第%d行：%s", i+1, err.Error()))
			continue
		}
		if !keep {
			continue
		}
		out = append(out, row)
		if len(out) > maxImport {
			return nil, sheetErr(fmt.Sprintf("一次最多导入 %d 条网络请求", maxImport))
		}
	}
	if len(rowErrs) > 0 {
		return nil, &RowErrors{Messages: rowErrs}
	}
	return out, nil
}

func pickRequestSheet(f *excelize.File) (string, error) {
	sheets := f.GetSheetList()
	for _, name := range sheets {
		if name == requestSheet {
			return name, nil
		}
	}
	for _, name := range sheets {
		if name == monitorSheet {
			return "", sheetErr("这是完整监控表。网络请求导入只需要名称、分组、类型、地址")
		}
	}
	for _, name := range sheets {
		if name != helpSheet {
			return name, nil
		}
	}
	if len(sheets) > 0 {
		return "", sheetErr("表格里没有网络请求表")
	}
	return "", sheetErr("表格里没有工作表")
}

func requestHeaderKeys(cells []string) ([]string, error) {
	keys, err := headerKeys(cells)
	if err != nil {
		return nil, err
	}
	hasURL := false
	for _, key := range keys {
		switch key {
		case "url":
			hasURL = true
		case "id", "host", "port", "method", "headers", "bodyType", "body", "threshold", "memThreshold", "match", "command", "workDir", "interval", "timeout", "notifiers":
			return nil, sheetErr("网络请求表格只需要名称、分组、类型、地址")
		}
	}
	if !hasURL {
		return nil, sheetErr("表格需要包含「地址」列")
	}
	return keys, nil
}

func parseRequestRow(line int, keys, cells []string) (SheetRequest, bool, error) {
	values := map[string]string{}
	present := map[string]bool{}
	blank := true
	for i, key := range keys {
		if key != "name" && key != "group" && key != "type" && key != "url" {
			continue
		}
		present[key] = true
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
		return SheetRequest{}, false, nil
	}
	if values["name"] == "" {
		return SheetRequest{}, false, errors.New("名称不能为空")
	}
	kind, err := canonRequestKind(values["type"])
	if err != nil {
		return SheetRequest{}, false, err
	}
	addr, err := applyRequestURL(values["url"], kind)
	if err != nil {
		return SheetRequest{}, false, err
	}
	return SheetRequest{
		Line:     line,
		Name:     values["name"],
		Group:    values["group"],
		HasGroup: present["group"],
		URL:      addr,
	}, true, nil
}

func canonRequestKind(raw string) (string, error) {
	s := strings.ToLower(strings.TrimSpace(raw))
	s = strings.ReplaceAll(s, " ", "")
	switch s {
	case "", "http", "https":
		return s, nil
	default:
		return "", errors.New("类型仅支持 http 或 https")
	}
}

func applyRequestURL(raw, kind string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", errors.New("地址不能为空")
	}
	scheme, rest, ok := splitHTTPScheme(raw)
	if !ok {
		if strings.Contains(raw, "://") {
			return "", errors.New("地址需要以 http:// 或 https:// 开头")
		}
		scheme = "http"
		if kind == "https" {
			scheme = "https"
		}
		rest = raw
	} else if kind == "http" || kind == "https" {
		scheme = kind
	}
	out := scheme + "://" + rest
	if len(out) > 2048 {
		return "", errors.New("地址过长")
	}
	return out, nil
}

func splitHTTPScheme(raw string) (scheme, rest string, ok bool) {
	lower := strings.ToLower(raw)
	for _, item := range []string{"https", "http"} {
		prefix := item + "://"
		if strings.HasPrefix(lower, prefix) {
			return item, raw[len(prefix):], true
		}
	}
	return "", raw, false
}

// ImportRequestsXLSX 读取网络请求表并写入 HTTP 监控。校验失败时不会改动已有数据。
func ImportRequestsXLSX(db *gorm.DB, r io.Reader, defaultInterval, defaultTimeout int) (ImportStats, error) {
	rows, err := ParseRequestXLSX(r)
	if err != nil {
		return ImportStats{}, err
	}
	return ApplyRequestSheet(db, rows, defaultInterval, defaultTimeout)
}

// ApplyRequestSheet 按名称新建或更新网络请求，只改名称、分组和地址。
func ApplyRequestSheet(db *gorm.DB, rows []SheetRequest, defaultInterval, defaultTimeout int) (ImportStats, error) {
	if len(rows) > maxImport {
		return ImportStats{}, sheetErr(fmt.Sprintf("一次最多导入 %d 条网络请求", maxImport))
	}
	var stats ImportStats
	err := db.Transaction(func(tx *gorm.DB) error {
		plans := make([]plan, 0, len(rows))
		var rowErrs []string
		for _, row := range rows {
			item, err := prepareRequest(tx, row, defaultInterval, defaultTimeout)
			if err != nil {
				rowErrs = append(rowErrs, fmt.Sprintf("第%d行：%s", row.Line, err.Error()))
				continue
			}
			plans = append(plans, item)
		}
		if len(rowErrs) > 0 {
			return &RowErrors{Messages: rowErrs}
		}
		if err := rejectConflicts(plans); err != nil {
			return err
		}
		created, updated := 0, 0
		for _, item := range plans {
			if err := writeMonitor(tx, item); err != nil {
				return err
			}
			if item.existing == nil {
				created++
			} else {
				updated++
			}
		}
		stats = ImportStats{Created: created, Updated: updated, Total: created + updated}
		return nil
	})
	if err != nil {
		return ImportStats{}, err
	}
	return stats, nil
}

func prepareRequest(tx *gorm.DB, row SheetRequest, defaultInterval, defaultTimeout int) (plan, error) {
	existing, err := findRequestTarget(tx, row.Name)
	if err != nil {
		return plan{}, err
	}
	var groupID *uint
	if row.HasGroup {
		groupID, err = ensureGroup(tx, row.Group)
		if err != nil {
			return plan{}, err
		}
	}
	var req monitor.Request
	var links []model.Notifier
	if existing == nil {
		req = monitor.Request{Name: row.Name, Type: model.TypeHTTP, URL: row.URL, Method: "GET", GroupID: groupID}
		links = []model.Notifier{}
	} else {
		req = requestFromMonitor(existing)
		req.Name = row.Name
		req.Type = model.TypeHTTP
		req.URL = row.URL
		if row.HasGroup {
			req.GroupID = groupID
		}
		links = existing.Notifiers
		if links == nil {
			links = []model.Notifier{}
		}
	}
	if err := req.Normalize(defaultInterval, defaultTimeout); err != nil {
		return plan{}, err
	}
	return plan{row: SheetMonitor{Line: row.Line, Name: row.Name}, req: req, existing: existing, notifiers: links}, nil
}

func findRequestTarget(tx *gorm.DB, name string) (*model.Monitor, error) {
	var found []model.Monitor
	if err := tx.Preload("Notifiers").Where("name = ?", name).Find(&found).Error; err != nil {
		return nil, err
	}
	if len(found) > 1 {
		return nil, errors.New("存在多条同名监控，无法按名称更新")
	}
	if len(found) == 0 {
		return nil, nil
	}
	if found[0].Type != model.TypeHTTP {
		return nil, errors.New("已有同名监控不是网络请求")
	}
	copied := found[0]
	return &copied, nil
}

func requestFromMonitor(m *model.Monitor) monitor.Request {
	enabled := m.Enabled
	follow := m.FollowRedirects
	var groupID *uint
	if m.GroupID != nil {
		id := *m.GroupID
		groupID = &id
	}
	var headers map[string]string
	if len(m.Headers) > 0 {
		headers = make(map[string]string, len(m.Headers))
		for key, value := range m.Headers {
			headers[key] = value
		}
	}
	ids := make([]uint, 0, len(m.Notifiers))
	for _, item := range m.Notifiers {
		ids = append(ids, item.ID)
	}
	return monitor.Request{
		Name:                m.Name,
		GroupID:             groupID,
		Type:                m.Type,
		URL:                 m.URL,
		Host:                m.Host,
		Port:                m.Port,
		Method:              m.Method,
		Headers:             headers,
		Body:                m.Body,
		BodyType:            m.BodyType,
		Threshold:           m.Threshold,
		MemThreshold:        m.MemThreshold,
		Match:               m.Match,
		Command:             m.Command,
		WorkDir:             m.WorkDir,
		Interval:            m.Interval,
		Timeout:             m.Timeout,
		FailureThreshold:    m.FailureThreshold,
		RecoveryThreshold:   m.RecoveryThreshold,
		ExpectedStatusCodes: m.ExpectedStatusCodes,
		BodyContains:        m.BodyContains,
		BodyNotContains:     m.BodyNotContains,
		FollowRedirects:     &follow,
		TLSWarningDays:      m.TLSWarningDays,
		TLSCriticalDays:     m.TLSCriticalDays,
		TLSNotifyDays:       m.TLSNotifyDays,
		NotifierIDs:         ids,
		Enabled:             &enabled,
	}
}

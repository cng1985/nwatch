package transfer

import (
	"bytes"
	"strings"
	"testing"

	"github.com/cng1985/nwatch/internal/model"
	"github.com/xuri/excelize/v2"
)

func TestRequestSheetRoundTripKeepsDetails(t *testing.T) {
	for _, col := range requestColumns {
		if headerAlias[canonHeader(col.header)] != col.key {
			t.Fatalf("表头 %s 没有对应字段", col.header)
		}
	}
	db := openExcelDB(t)
	group := model.Group{Name: "生产"}
	if err := db.Create(&group).Error; err != nil {
		t.Fatal(err)
	}
	hook := model.Notifier{Name: "运维钉钉", Type: model.NotifierDingTalk, Enabled: true, WebhookURL: "https://example.com/hook"}
	if err := db.Create(&hook).Error; err != nil {
		t.Fatal(err)
	}
	gid := group.ID
	current := model.Monitor{
		Name: "官网", GroupID: &gid, Type: model.TypeHTTP, URL: "https://www.example.com/health",
		Method: "POST", Headers: map[string]string{"Authorization": "Bearer token"},
		Body: `{"ok":true}`, BodyType: "json", Interval: 30, Timeout: 5, Enabled: true,
		FailureThreshold: 2, RecoveryThreshold: 1, ExpectedStatusCodes: "200,201",
		FollowRedirects: false, Status: model.StatusUp,
	}
	current.SyncHeaders()
	if err := db.Create(&current).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&current).Association("Notifiers").Replace(&hook); err != nil {
		t.Fatal(err)
	}
	other := model.Monitor{
		Name: "磁盘", Type: model.TypeDisk, Host: "/", Threshold: 90, Interval: 60, Timeout: 5,
		Enabled: true, FailureThreshold: 3, RecoveryThreshold: 1, Status: model.StatusUnknown,
	}
	if err := db.Create(&other).Error; err != nil {
		t.Fatal(err)
	}
	var stored []model.Monitor
	if err := db.Preload("Group").Preload("Notifiers").Order("id").Find(&stored).Error; err != nil {
		t.Fatal(err)
	}
	raw, err := RequestsToXLSX(stored)
	if err != nil {
		t.Fatal(err)
	}
	book, err := excelize.OpenReader(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	defer book.Close()
	rows, err := book.GetRows(requestSheet)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || rows[0][0] != "名称" || rows[0][1] != "分组" || rows[0][2] != "类型" || rows[0][3] != "地址" {
		t.Fatalf("表头或行数不对 %#v", rows)
	}
	if rows[1][0] != "官网" || rows[1][1] != "生产" || rows[1][2] != "https" || rows[1][3] != "https://www.example.com/health" {
		t.Fatalf("导出内容不对 %#v", rows[1])
	}
	if help, err := book.GetRows(helpSheet); err != nil || len(help) == 0 {
		t.Fatalf("说明页缺失 %v", err)
	}
	parsed, err := ParseRequestXLSX(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	if len(parsed) != 1 || parsed[0].Name != "官网" || parsed[0].URL != "https://www.example.com/health" {
		t.Fatalf("解析结果 %#v", parsed)
	}
	parsed[0].Group = "业务"
	parsed[0].URL = "http://www.example.com/health"
	stats, err := ApplyRequestSheet(db, append(parsed, SheetRequest{
		Line: 3, Name: "支付", Group: "业务", HasGroup: true, URL: "https://pay.example.com/health",
	}), 60, 5)
	if err != nil {
		t.Fatal(err)
	}
	if stats.Created != 1 || stats.Updated != 1 {
		t.Fatalf("stats %+v", stats)
	}
	var got model.Monitor
	if err := db.Preload("Group").Preload("Notifiers").Where("name = ?", "官网").First(&got).Error; err != nil {
		t.Fatal(err)
	}
	if got.URL != "http://www.example.com/health" || got.Group == nil || got.Group.Name != "业务" {
		t.Fatalf("地址或分组没更新 %+v", got)
	}
	if got.Method != "POST" || got.BodyType != "json" || got.Body != `{"ok":true}` || got.Headers["Authorization"] != "Bearer token" {
		t.Fatalf("请求内容被改掉 %+v headers=%v", got, got.Headers)
	}
	if got.Interval != 30 || got.Timeout != 5 || got.FailureThreshold != 2 || got.FollowRedirects {
		t.Fatalf("检测配置被改掉 %+v", got)
	}
	if len(got.Notifiers) != 1 || got.Notifiers[0].Name != "运维钉钉" {
		t.Fatalf("通知渠道被改掉 %+v", got.Notifiers)
	}
	var created model.Monitor
	if err := db.Preload("Group").Where("name = ?", "支付").First(&created).Error; err != nil {
		t.Fatal(err)
	}
	if created.Type != model.TypeHTTP || created.URL != "https://pay.example.com/health" || created.Method != "GET" || created.Interval != 60 {
		t.Fatalf("新建不对 %+v", created)
	}
	if created.Group == nil || created.Group.Name != "业务" {
		t.Fatalf("新建分组不对 %+v", created.Group)
	}
	var disk model.Monitor
	if err := db.Where("name = ?", "磁盘").First(&disk).Error; err != nil || disk.Type != model.TypeDisk {
		t.Fatalf("无关监控被改动 %+v %v", disk, err)
	}
}

func TestRequestSheetValidation(t *testing.T) {
	db := openExcelDB(t)
	proc := model.Monitor{
		Name: "nginx", Type: model.TypeProcess, Host: "nginx", Match: model.MatchName,
		Interval: 30, Timeout: 5, Enabled: true, FailureThreshold: 1, RecoveryThreshold: 1,
		Status: model.StatusUnknown,
	}
	if err := db.Create(&proc).Error; err != nil {
		t.Fatal(err)
	}
	full := model.Monitor{
		Name: "官网", Type: model.TypeHTTP, URL: "https://www.example.com", Method: "GET",
		Interval: 60, Timeout: 5, Enabled: true, FailureThreshold: 3, RecoveryThreshold: 1,
		Status: model.StatusUnknown,
	}
	raw, err := MonitorsToXLSX([]model.Monitor{full}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ParseRequestXLSX(bytes.NewReader(raw)); err == nil || !strings.Contains(err.Error(), "只需要名称、分组、类型、地址") {
		t.Fatalf("完整监控表应被拒绝：%v", err)
	}
	if _, err := ParseRequestXLSX(bytes.NewReader(sheetBytes(t, []string{"名称", "分组"}, []string{"官网", "生产"}))); err == nil {
		t.Fatal("缺少地址列应该失败")
	}
	bad := sheetBytes(t, []string{"名称", "类型", "地址"}, []string{"新接口", "tcp", "10.0.0.8"})
	if _, err := ImportRequestsXLSX(db, bytes.NewReader(bad), 60, 5); err == nil || !strings.Contains(err.Error(), "类型仅支持") {
		t.Fatalf("非法类型应被拒绝：%v", err)
	}
	var count int64
	if err := db.Model(&model.Monitor{}).Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("失败导入仍写入了数据 %d %v", count, err)
	}
	clash := sheetBytes(t, []string{"名称", "类型", "地址"}, []string{"nginx", "http", "http://127.0.0.1"})
	if _, err := ImportRequestsXLSX(db, bytes.NewReader(clash), 60, 5); err == nil || !strings.Contains(err.Error(), "不是网络请求") {
		t.Fatalf("同名进程应被拒绝：%v", err)
	}
	dup := sheetBytes(t,
		[]string{"名称", "类型", "地址"},
		[]string{"甲", "https", "a.example.com"},
		[]string{"甲", "http", "b.example.com"},
	)
	if _, err := ImportRequestsXLSX(db, bytes.NewReader(dup), 60, 5); err == nil || !strings.Contains(err.Error(), "名称相同") {
		t.Fatalf("重名应被拒绝：%v", err)
	}
	plain := sheetBytes(t, []string{"名称", "类型", "地址"}, []string{"内网", "http", "10.0.0.8/health"})
	stats, err := ImportRequestsXLSX(db, bytes.NewReader(plain), 60, 5)
	if err != nil || stats.Created != 1 {
		t.Fatalf("导入 %+v %v", stats, err)
	}
	var got model.Monitor
	if err := db.Where("name = ?", "内网").First(&got).Error; err != nil {
		t.Fatal(err)
	}
	if got.URL != "http://10.0.0.8/health" || got.Method != "GET" || got.Interval != 60 {
		t.Fatalf("新建不对 %+v", got)
	}
	flip := sheetBytes(t, []string{"名称", "类型", "地址"}, []string{"内网", "https", "http://10.0.0.8/health"})
	stats, err = ImportRequestsXLSX(db, bytes.NewReader(flip), 60, 5)
	if err != nil || stats.Updated != 1 || stats.Created != 0 {
		t.Fatalf("改协议 %+v %v", stats, err)
	}
	if err := db.Where("name = ?", "内网").First(&got).Error; err != nil {
		t.Fatal(err)
	}
	if got.URL != "https://10.0.0.8/health" || got.Interval != 60 {
		t.Fatalf("类型没有覆盖协议 %+v", got)
	}
	if err := db.Model(&model.Monitor{}).Count(&count).Error; err != nil || count != 2 {
		t.Fatalf("拒绝的行不应该写入 %d %v", count, err)
	}
}

func sheetBytes(t *testing.T, header []string, rows ...[]string) []byte {
	t.Helper()
	f := excelize.NewFile()
	defer f.Close()
	all := append([][]string{header}, rows...)
	for r, row := range all {
		for c, value := range row {
			cell, err := excelize.CoordinatesToCellName(c+1, r+1)
			if err != nil {
				t.Fatal(err)
			}
			if err := f.SetCellValue("Sheet1", cell, value); err != nil {
				t.Fatal(err)
			}
		}
	}
	buf, err := f.WriteToBuffer()
	if err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

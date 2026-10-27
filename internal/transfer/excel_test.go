package transfer

import (
	"bytes"
	"path/filepath"
	"testing"
	"time"

	"github.com/cng1985/nwatch/internal/database"
	"github.com/cng1985/nwatch/internal/model"
	"github.com/xuri/excelize/v2"
	"gorm.io/gorm"
)

func TestMonitorSheetRoundTrip(t *testing.T) {
	for _, col := range monitorColumns {
		if headerAlias[canonHeader(col.header)] != col.key {
			t.Fatalf("表头 %s 没有对应字段", col.header)
		}
	}
	group := model.Group{Name: "生产"}
	hook := model.Notifier{Name: "运维钉钉", Type: model.NotifierDingTalk, Enabled: true}
	db := openExcelDB(t)
	if err := db.Create(&group).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&hook).Error; err != nil {
		t.Fatal(err)
	}
	gid := group.ID
	follow := false
	enabled := true
	httpMonitor := model.Monitor{
		Name: "ERP API", GroupID: &gid, Type: model.TypeHTTP, URL: "https://erp.example.com/health",
		Method: "POST", Headers: map[string]string{"Authorization": "Bearer token", "X-Trace": "1"},
		Body: `{"ok":true}`, BodyType: "json", Interval: 60, Timeout: 5, Enabled: enabled,
		FailureThreshold: 2, RecoveryThreshold: 1, ExpectedStatusCodes: "200,201",
		BodyContains: "ok", FollowRedirects: follow, Status: model.StatusUp, LastResponseTime: 42,
	}
	httpMonitor.SyncHeaders()
	checked := time.Date(2026, 10, 27, 8, 30, 0, 0, time.UTC)
	httpMonitor.LastCheckAt = &checked
	if err := db.Create(&httpMonitor).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&httpMonitor).Association("Notifiers").Replace(&hook); err != nil {
		t.Fatal(err)
	}
	proc := model.Monitor{
		Name: "nginx", Type: model.TypeProcess, Host: "nginx", Match: model.MatchCommand,
		Command: "nginx: master", Threshold: 0, MemThreshold: 256, Interval: 30, Timeout: 5,
		Enabled: true, FailureThreshold: 1, RecoveryThreshold: 1, Status: model.StatusUnknown,
	}
	if err := db.Create(&proc).Error; err != nil {
		t.Fatal(err)
	}
	var stored []model.Monitor
	if err := db.Preload("Group").Preload("Notifiers").Order("id").Find(&stored).Error; err != nil {
		t.Fatal(err)
	}
	raw, err := MonitorsToXLSX(stored, map[uint]string{httpMonitor.ID: "99.50%"}, time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	book, err := excelize.OpenReader(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	defer book.Close()
	if rows, err := book.GetRows(helpSheet); err != nil || len(rows) == 0 {
		t.Fatalf("说明页缺失 %v", err)
	}
	parsed, err := ParseMonitorXLSX(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	if len(parsed) != 2 {
		t.Fatalf("解析出 %d 行", len(parsed))
	}
	if parsed[0].Name != "ERP API" || parsed[0].Group != "生产" || parsed[0].Headers["Authorization"] != "Bearer token" {
		t.Fatalf("http 行 %+v", parsed[0])
	}
	if parsed[0].Notifiers[0] != "运维钉钉" || parsed[0].Enabled == nil || !*parsed[0].Enabled || parsed[0].FollowRedirects == nil || *parsed[0].FollowRedirects {
		t.Fatalf("开关或渠道不对 %+v", parsed[0])
	}
	if parsed[1].Threshold != 0 || parsed[1].MemThreshold != 256 || parsed[1].Match != model.MatchCommand {
		t.Fatalf("进程行 %+v", parsed[1])
	}

	parsed[0].Name = "ERP 接口"
	parsed[0].Group = ""
	parsed[0].Notifiers = nil
	parsed[0].Interval = 120
	off := false
	parsed[0].Enabled = &off
	parsed = append(parsed, SheetMonitor{
		Line: 4, Name: "证书", Type: "证书", URL: "api.example.com", Interval: 3600, Timeout: 10,
		TLSWarningDays: 14, TLSCriticalDays: 7,
	})
	stats, err := ApplyMonitorSheet(db, parsed, 60, 5)
	if err != nil {
		t.Fatal(err)
	}
	if stats.Created != 1 || stats.Updated != 2 || stats.Total != 3 {
		t.Fatalf("stats %+v", stats)
	}
	var got model.Monitor
	if err := db.Preload("Notifiers").First(&got, httpMonitor.ID).Error; err != nil {
		t.Fatal(err)
	}
	if got.Name != "ERP 接口" || got.GroupID != nil || got.Enabled || got.Status != model.StatusPaused || got.Interval != 120 {
		t.Fatalf("更新结果 %+v", got)
	}
	if len(got.Notifiers) != 0 {
		t.Fatalf("通知渠道未清空 %+v", got.Notifiers)
	}
	var cert model.Monitor
	if err := db.Where("name = ?", "证书").First(&cert).Error; err != nil {
		t.Fatal(err)
	}
	if cert.Type != model.TypeTLS || cert.URL != "api.example.com" || cert.Status != model.StatusUnknown {
		t.Fatalf("新建证书 %+v", cert)
	}
	var nginx model.Monitor
	if err := db.Where("name = ?", "nginx").First(&nginx).Error; err != nil {
		t.Fatal(err)
	}
	if nginx.Threshold != 0 || nginx.MemThreshold != 256 || nginx.Match != model.MatchCommand {
		t.Fatalf("进程被改坏 %+v", nginx)
	}
}

func TestMonitorSheetRejectsBadRows(t *testing.T) {
	db := openExcelDB(t)
	keep := model.Monitor{
		Name: "保留", Type: model.TypeHTTP, URL: "http://127.0.0.1/health", Method: "GET",
		Interval: 60, Timeout: 5, Enabled: true, FailureThreshold: 3, RecoveryThreshold: 1,
		ExpectedStatusCodes: "200", Status: model.StatusUnknown,
	}
	if err := db.Create(&keep).Error; err != nil {
		t.Fatal(err)
	}
	rows := []SheetMonitor{
		{Line: 2, ID: keep.ID, Name: "保留", Type: "http", URL: "http://127.0.0.1/health", Interval: 30, Timeout: 5},
		{Line: 3, Name: "坏数据", Type: "nope", URL: "http://127.0.0.1", Interval: 30, Timeout: 5},
	}
	if _, err := ApplyMonitorSheet(db, rows, 60, 5); err == nil {
		t.Fatal("应该拒绝非法类型")
	}
	var got model.Monitor
	if err := db.First(&got, keep.ID).Error; err != nil {
		t.Fatal(err)
	}
	if got.Interval != 60 {
		t.Fatalf("失败的导入改动了原监控: %+v", got)
	}
	var count int64
	if err := db.Model(&model.Monitor{}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("条数 %d", count)
	}

	dup := []SheetMonitor{
		{Line: 2, Name: "保留", Type: "http", URL: "http://127.0.0.1/health", Interval: 60, Timeout: 5},
		{Line: 3, Name: "保留", Type: "http", URL: "http://127.0.0.1/health", Interval: 60, Timeout: 5},
	}
	if _, err := ApplyMonitorSheet(db, dup, 60, 5); err == nil {
		t.Fatal("同名行应该冲突")
	}

	missing := []SheetMonitor{
		{Line: 2, Name: "新监控", Type: "tcp", Host: "10.0.0.8", Port: 3306, Interval: 30, Timeout: 3, Notifiers: []string{"不存在"}},
	}
	if _, err := ApplyMonitorSheet(db, missing, 60, 5); err == nil {
		t.Fatal("缺少渠道应该失败")
	}
}

func TestParseChineseSheet(t *testing.T) {
	f := excelize.NewFile()
	defer f.Close()
	if err := f.SetSheetName("Sheet1", "监控"); err != nil {
		t.Fatal(err)
	}
	header := []any{"名称", "类型", "地址", "请求头", "启用", "分组"}
	if err := f.SetSheetRow("监控", "A1", &header); err != nil {
		t.Fatal(err)
	}
	row := []any{"官网", "https", "www.example.com", "Accept: text/plain", "否", "边缘"}
	if err := f.SetSheetRow("监控", "A2", &row); err != nil {
		t.Fatal(err)
	}
	buf, err := f.WriteToBuffer()
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := ParseMonitorXLSX(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if len(parsed) != 1 {
		t.Fatalf("%d 行", len(parsed))
	}
	got := parsed[0]
	if got.Type != model.TypeHTTP || got.URL != "https://www.example.com" || got.Headers["Accept"] != "text/plain" {
		t.Fatalf("%+v", got)
	}
	if got.Enabled == nil || *got.Enabled || got.Group != "边缘" {
		t.Fatalf("%+v", got)
	}
	db := openExcelDB(t)
	stats, err := ApplyMonitorSheet(db, parsed, 60, 5)
	if err != nil {
		t.Fatal(err)
	}
	if stats.Created != 1 {
		t.Fatalf("%+v", stats)
	}
	var group model.Group
	if err := db.Where("name = ?", "边缘").First(&group).Error; err != nil {
		t.Fatal(err)
	}
	var monitor model.Monitor
	if err := db.Where("name = ?", "官网").First(&monitor).Error; err != nil {
		t.Fatal(err)
	}
	if monitor.GroupID == nil || *monitor.GroupID != group.ID || monitor.Enabled || monitor.Status != model.StatusPaused {
		t.Fatalf("%+v", monitor)
	}
}

func TestParseRejectsMissingName(t *testing.T) {
	f := excelize.NewFile()
	defer f.Close()
	header := []any{"类型", "地址"}
	if err := f.SetSheetRow("Sheet1", "A1", &header); err != nil {
		t.Fatal(err)
	}
	buf, err := f.WriteToBuffer()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ParseMonitorXLSX(bytes.NewReader(buf.Bytes())); err == nil {
		t.Fatal("缺少名称列应该失败")
	}
}

func openExcelDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := database.Open(filepath.Join(t.TempDir(), "excel.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close(db) })
	if err := database.Migrate(db); err != nil {
		t.Fatal(err)
	}
	return db
}

package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/cng1985/nwatch/internal/model"
	"github.com/xuri/excelize/v2"
)

func TestRequestExcelImportExport(t *testing.T) {
	s := newTestServer(t)
	token := login(t, s)
	rec := do(t, s, http.MethodPost, "/api/groups", token, map[string]any{"name": "生产", "sort": 1})
	if rec.Code != 200 {
		t.Fatalf("group %d %s", rec.Code, rec.Body.String())
	}
	var created struct {
		Data model.Group `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil || created.Data.ID == 0 {
		t.Fatalf("group body %s", rec.Body.String())
	}
	rec = do(t, s, http.MethodPost, "/api/monitors", token, map[string]any{
		"name": "官网", "groupId": created.Data.ID, "type": "http", "url": "https://www.example.com",
		"method": "POST", "bodyType": "json", "body": `{"ok":true}`, "headers": map[string]string{"X-Trace": "1"},
		"interval": 30, "timeout": 5, "failureThreshold": 2, "recoveryThreshold": 1,
	})
	if rec.Code != 200 {
		t.Fatalf("create %d %s", rec.Code, rec.Body.String())
	}
	rec = do(t, s, http.MethodPost, "/api/monitors", token, map[string]any{
		"name": "忽略", "type": "tcp", "host": "127.0.0.1", "port": 9, "interval": 30, "timeout": 3,
	})
	if rec.Code != 200 {
		t.Fatalf("tcp %d %s", rec.Code, rec.Body.String())
	}

	rec = do(t, s, http.MethodGet, "/api/monitors/requests/export?keyword=官网", token, nil)
	if rec.Code != 200 {
		t.Fatalf("export %d %s", rec.Code, rec.Body.String())
	}
	book, err := excelize.OpenReader(bytes.NewReader(rec.Body.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	defer book.Close()
	rows, err := book.GetRows("网络请求")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || len(rows[0]) != 4 || rows[1][0] != "官网" || rows[1][2] != "https" {
		t.Fatalf("导出不对 %#v", rows)
	}
	if err := book.SetCellValue("网络请求", "B2", "业务"); err != nil {
		t.Fatal(err)
	}
	if err := book.SetCellValue("网络请求", "C2", "http"); err != nil {
		t.Fatal(err)
	}
	buf, err := book.WriteToBuffer()
	if err != nil {
		t.Fatal(err)
	}
	rec = postExcel(t, s, "/api/monitors/requests/import", token, buf.Bytes())
	if rec.Code != 200 {
		t.Fatalf("import %d %s", rec.Code, rec.Body.String())
	}
	rec = do(t, s, http.MethodGet, "/api/monitors?keyword=官网", token, nil)
	if rec.Code != 200 {
		t.Fatal(rec.Body.String())
	}
	var listed struct {
		Data struct {
			Items []model.Monitor `json:"items"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &listed); err != nil {
		t.Fatal(err)
	}
	if len(listed.Data.Items) != 1 {
		t.Fatalf("数量 %+v", listed.Data.Items)
	}
	got := listed.Data.Items[0]
	if got.Group == nil || got.Group.Name != "业务" || got.URL != "http://www.example.com" {
		t.Fatalf("没按表格更新 %+v", got)
	}
	if got.Method != "POST" || got.BodyType != "json" || got.Interval != 30 || got.Headers["X-Trace"] != "1" {
		t.Fatalf("其他字段被改掉 %+v headers=%v", got, got.Headers)
	}

	rec = do(t, s, http.MethodGet, "/api/monitors/requests/export?template=1", token, nil)
	if rec.Code != 200 {
		t.Fatalf("template %d %s", rec.Code, rec.Body.String())
	}
	template, err := excelize.OpenReader(bytes.NewReader(rec.Body.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	defer template.Close()
	header, err := template.GetRows("网络请求")
	if err != nil || len(header) != 1 || strings.Join(header[0], ",") != "名称,分组,类型,地址" {
		t.Fatalf("模板 %#v %v", header, err)
	}
	viewer := viewerToken(t, s, token)
	rec = do(t, s, http.MethodGet, "/api/monitors/requests/export?template=1", viewer, nil)
	if rec.Code != 200 {
		t.Fatalf("viewer export %d", rec.Code)
	}
	rec = postExcel(t, s, "/api/monitors/requests/import", viewer, buf.Bytes())
	if rec.Code != http.StatusForbidden {
		t.Fatalf("viewer import %d %s", rec.Code, rec.Body.String())
	}
}

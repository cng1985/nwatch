package api

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cng1985/nwatch/internal/model"
	"github.com/cng1985/nwatch/internal/rbac"
	"github.com/xuri/excelize/v2"
)

func TestMonitorExcelImportExport(t *testing.T) {
	s := newTestServer(t)
	token := login(t, s)
	rec := do(t, s, http.MethodPost, "/api/groups", token, map[string]any{"name": "生产", "sort": 1})
	if rec.Code != 200 {
		t.Fatalf("group %d %s", rec.Code, rec.Body.String())
	}
	rec = do(t, s, http.MethodPost, "/api/monitors", token, map[string]any{
		"name": "官网", "type": "http", "url": "https://www.example.com", "method": "GET",
		"interval": 60, "timeout": 5, "failureThreshold": 2, "recoveryThreshold": 1,
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

	rec = do(t, s, http.MethodGet, "/api/monitors/export?keyword=官网", token, nil)
	if rec.Code != 200 {
		t.Fatalf("export %d %s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet" {
		t.Fatalf("content-type %s", ct)
	}
	book, err := excelize.OpenReader(bytes.NewReader(rec.Body.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	defer book.Close()
	rows, err := book.GetRows("监控")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || rows[1][1] != "官网" {
		t.Fatalf("筛选导出不对 %#v", rows)
	}
	nameCol := columnIndex(t, rows[0], "名称")
	groupCol := columnIndex(t, rows[0], "分组")
	rows[1][groupCol] = "生产"
	cell, err := excelize.CoordinatesToCellName(groupCol+1, 2)
	if err != nil {
		t.Fatal(err)
	}
	if err := book.SetCellValue("监控", cell, "生产"); err != nil {
		t.Fatal(err)
	}
	_ = nameCol
	buf, err := book.WriteToBuffer()
	if err != nil {
		t.Fatal(err)
	}
	rec = postExcel(t, s, "/api/monitors/import", token, buf.Bytes())
	if rec.Code != 200 {
		t.Fatalf("import %d %s", rec.Code, rec.Body.String())
	}
	var imported struct {
		Data struct {
			Created int `json:"created"`
			Updated int `json:"updated"`
			Total   int `json:"total"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &imported); err != nil {
		t.Fatal(err)
	}
	if imported.Data.Updated != 1 || imported.Data.Created != 0 || imported.Data.Total != 1 {
		t.Fatalf("stats %+v", imported.Data)
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
	if len(listed.Data.Items) != 1 || listed.Data.Items[0].Group == nil || listed.Data.Items[0].Group.Name != "生产" {
		t.Fatalf("分组没写上 %+v", listed.Data.Items)
	}

	rec = do(t, s, http.MethodGet, "/api/monitors/export?template=1", token, nil)
	if rec.Code != 200 {
		t.Fatalf("template %d %s", rec.Code, rec.Body.String())
	}
	template, err := excelize.OpenReader(bytes.NewReader(rec.Body.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	defer template.Close()
	header, err := template.GetRows("监控")
	if err != nil || len(header) != 1 || header[0][1] != "名称" {
		t.Fatalf("模板 %#v %v", header, err)
	}

	viewer := viewerToken(t, s, token)
	rec = do(t, s, http.MethodGet, "/api/monitors/export?template=1", viewer, nil)
	if rec.Code != 200 {
		t.Fatalf("viewer export %d %s", rec.Code, rec.Body.String())
	}
	rec = postExcel(t, s, "/api/monitors/import", viewer, buf.Bytes())
	if rec.Code != http.StatusForbidden {
		t.Fatalf("viewer import %d %s", rec.Code, rec.Body.String())
	}
}

func columnIndex(t *testing.T, header []string, name string) int {
	t.Helper()
	for i, cell := range header {
		if cell == name {
			return i
		}
	}
	t.Fatalf("缺少列 %s", name)
	return -1
}

func postExcel(t *testing.T, s *Server, path, token string, content []byte) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "monitors.xlsx")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, path, &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	return rec
}

func viewerToken(t *testing.T, s *Server, adminToken string) string {
	t.Helper()
	rec := do(t, s, http.MethodGet, "/api/access/roles", adminToken, nil)
	if rec.Code != 200 {
		t.Fatalf("roles %s", rec.Body.String())
	}
	var roles struct {
		Data []rbac.RoleView `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &roles); err != nil {
		t.Fatal(err)
	}
	var viewerID uint
	for _, role := range roles.Data {
		if role.Key == rbac.RoleViewer {
			viewerID = role.ID
		}
	}
	if viewerID == 0 {
		t.Fatal("缺少只读角色")
	}
	rec = do(t, s, http.MethodPost, "/api/access/users", adminToken, map[string]any{
		"username": "excel-viewer", "displayName": "只读", "password": "viewer-pass", "roleIds": []uint{viewerID},
	})
	if rec.Code != 200 {
		t.Fatalf("user %d %s", rec.Code, rec.Body.String())
	}
	return loginAs(t, s, "excel-viewer", "viewer-pass")
}

func TestMonitorExcelBadFile(t *testing.T) {
	s := newTestServer(t)
	token := login(t, s)
	rec := postExcel(t, s, "/api/monitors/import", token, []byte("not-an-xlsx"))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("bad file %d %s", rec.Code, rec.Body.String())
	}
	unauth := do(t, s, http.MethodGet, "/api/monitors/export", "", nil)
	if unauth.Code != http.StatusUnauthorized {
		t.Fatalf("auth %d", unauth.Code)
	}
}

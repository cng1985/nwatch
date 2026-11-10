package tests

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/cng1985/nwatch/internal/platform/auth"
)

func TestRolePermissions(t *testing.T) {
	s := newTestServer(t)
	adminToken := login(t, s)

	rec := do(t, s, http.MethodGet, "/api/access/roles", adminToken, nil)
	if rec.Code != 200 {
		t.Fatalf("roles %d %s", rec.Code, rec.Body.String())
	}
	var roles struct {
		Data struct {
			Items []auth.RoleView `json:"items"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &roles); err != nil {
		t.Fatal(err)
	}
	var viewerID uint
	for _, role := range roles.Data.Items {
		if role.Key == auth.RoleViewer {
			viewerID = role.ID
		}
	}
	if viewerID == 0 {
		t.Fatal("missing viewer role")
	}
	rec = do(t, s, http.MethodPost, "/api/access/users", adminToken, map[string]any{
		"username": "audit", "displayName": "审计", "password": "audit-pass", "roleIds": []uint{viewerID},
	})
	if rec.Code != 200 {
		t.Fatalf("create user %d %s", rec.Code, rec.Body.String())
	}
	var created struct {
		Data auth.UserView `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	viewerToken := loginAs(t, s, "audit", "audit-pass")
	rec = do(t, s, http.MethodGet, "/api/monitors", viewerToken, nil)
	if rec.Code != 200 {
		t.Fatalf("viewer list %d %s", rec.Code, rec.Body.String())
	}
	rec = do(t, s, http.MethodPost, "/api/monitors", viewerToken, map[string]any{
		"name": "blocked", "type": "http", "url": "http://127.0.0.1", "interval": 30, "timeout": 3,
	})
	assertCode(t, rec, 403)
	rec = do(t, s, http.MethodGet, "/api/settings", viewerToken, nil)
	assertCode(t, rec, 403)
	rec = do(t, s, http.MethodPost, "/api/shell/ticket", viewerToken, nil)
	assertCode(t, rec, 403)
	rec = do(t, s, http.MethodGet, "/api/access/users", viewerToken, nil)
	assertCode(t, rec, 403)

	rec = do(t, s, http.MethodPut, "/api/account/password", adminToken, map[string]string{
		"oldPassword": "test-pass", "newPassword": "next-pass",
	})
	if rec.Code != 200 {
		t.Fatalf("password %d %s", rec.Code, rec.Body.String())
	}
	loginAs(t, s, "admin", "next-pass")
	rec = do(t, s, http.MethodPost, "/api/auth/login", "", map[string]string{"username": "admin", "password": "test-pass"})
	assertCode(t, rec, 401)

	rec = do(t, s, http.MethodGet, "/api/auth/me", viewerToken, nil)
	if rec.Code != 200 || !jsonContains(rec.Body.String(), "audit") {
		t.Fatalf("me %d %s", rec.Code, rec.Body.String())
	}
	var usersList struct {
		Data struct {
			Items []auth.UserView `json:"items"`
		} `json:"data"`
	}
	rec = do(t, s, http.MethodGet, "/api/access/users", adminToken, nil)
	if err := json.Unmarshal(rec.Body.Bytes(), &usersList); err != nil {
		t.Fatal(err)
	}
	var auditID uint
	for _, u := range usersList.Data.Items {
		if u.Username == "audit" {
			auditID = u.ID
		}
	}
	if auditID == 0 {
		t.Fatal("missing audit user")
	}
	rec = do(t, s, http.MethodDelete, "/api/access/users/"+strconv.FormatUint(uint64(auditID), 10), viewerToken, nil)
	assertCode(t, rec, 403)
	rec = do(t, s, http.MethodDelete, "/api/access/users/"+strconv.FormatUint(uint64(auditID), 10), adminToken, nil)
	if rec.Code != 200 {
		t.Fatalf("delete user %d %s", rec.Code, rec.Body.String())
	}
}

func TestGroupAPI(t *testing.T) {
	s := newTestServer(t)
	token := login(t, s)
	rec := do(t, s, http.MethodPost, "/api/groups", token, map[string]any{"name": "生产", "description": "线上", "sort": 1})
	if rec.Code != 200 {
		t.Fatalf("create %d %s", rec.Code, rec.Body.String())
	}
	var created struct {
		Data struct {
			ID   uint   `json:"id"`
			Name string `json:"name"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	rec = do(t, s, http.MethodPost, "/api/groups", token, map[string]any{"name": "生产"})
	assertCode(t, rec, 400)
	rec = do(t, s, http.MethodGet, "/api/groups", token, nil)
	if rec.Code != 200 || !jsonContains(rec.Body.String(), `"monitorCount":0`) {
		t.Fatalf("list %d %s", rec.Code, rec.Body.String())
	}
	rec = do(t, s, http.MethodPut, "/api/groups/"+strconv.FormatUint(uint64(created.Data.ID), 10), token, map[string]any{"name": "生产二区", "sort": 2})
	if rec.Code != 200 {
		t.Fatalf("update %d %s", rec.Code, rec.Body.String())
	}
	rec = do(t, s, http.MethodDelete, "/api/groups/"+strconv.FormatUint(uint64(created.Data.ID), 10), token, nil)
	if rec.Code != 200 {
		t.Fatalf("delete %d %s", rec.Code, rec.Body.String())
	}
}

func jsonContains(body, want string) bool {
	return strings.Contains(body, want)
}

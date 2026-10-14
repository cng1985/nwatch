package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"testing"

	"github.com/cng1985/nwatch/internal/rbac"
)

func TestRolePermissions(t *testing.T) {
	s := newTestServer(t)
	adminToken := login(t, s)

	rec := do(t, s, http.MethodGet, "/api/access/roles", adminToken, nil)
	if rec.Code != 200 {
		t.Fatalf("roles %d %s", rec.Code, rec.Body.String())
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
		t.Fatal("missing viewer role")
	}
	rec = do(t, s, http.MethodPost, "/api/access/users", adminToken, map[string]any{
		"username": "audit", "displayName": "审计", "password": "audit-pass", "roleIds": []uint{viewerID},
	})
	if rec.Code != 200 {
		t.Fatalf("create user %d %s", rec.Code, rec.Body.String())
	}
	var created struct {
		Data rbac.UserView `json:"data"`
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
	if rec.Code != http.StatusForbidden {
		t.Fatalf("viewer write %d %s", rec.Code, rec.Body.String())
	}
	rec = do(t, s, http.MethodGet, "/api/settings", viewerToken, nil)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("viewer settings %d %s", rec.Code, rec.Body.String())
	}
	rec = do(t, s, http.MethodPost, "/api/shell/ticket", viewerToken, nil)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("viewer shell %d %s", rec.Code, rec.Body.String())
	}
	rec = do(t, s, http.MethodGet, "/api/access/users", viewerToken, nil)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("viewer users %d %s", rec.Code, rec.Body.String())
	}

	rec = do(t, s, http.MethodPut, "/api/account/password", adminToken, map[string]string{
		"oldPassword": "test-pass", "newPassword": "next-pass",
	})
	if rec.Code != 200 {
		t.Fatalf("password %d %s", rec.Code, rec.Body.String())
	}
	loginAs(t, s, "admin", "next-pass")
	rec = do(t, s, http.MethodPost, "/api/auth/login", "", map[string]string{"username": "admin", "password": "test-pass"})
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("old password %d", rec.Code)
	}

	rec = do(t, s, http.MethodDelete, "/api/access/users/"+strconv.FormatUint(uint64(created.Data.ID), 10), adminToken, nil)
	if rec.Code != 200 {
		t.Fatalf("delete user %d %s", rec.Code, rec.Body.String())
	}
}

func loginAs(t *testing.T, s *Server, username, password string) string {
	t.Helper()
	rec := do(t, s, http.MethodPost, "/api/auth/login", "", map[string]string{"username": username, "password": password})
	if rec.Code != 200 {
		t.Fatalf("login %s %d %s", username, rec.Code, rec.Body.String())
	}
	var body struct {
		Data struct {
			Token       string   `json:"token"`
			Permissions []string `json:"permissions"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Data.Token == "" || len(body.Data.Permissions) == 0 {
		t.Fatalf("session %+v", body.Data)
	}
	return body.Data.Token
}

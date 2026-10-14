package rbac

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/cng1985/nwatch/internal/config"
	"github.com/cng1985/nwatch/internal/database"
	"github.com/cng1985/nwatch/internal/model"
	"github.com/cng1985/nwatch/internal/settings"
	"golang.org/x/crypto/bcrypt"
)

func testStore(t *testing.T, password string) (*Store, *config.Config, *settings.Store) {
	t.Helper()
	dir := t.TempDir()
	cfg := config.Default()
	cfg.Database.Path = filepath.Join(dir, "nmonitor.db")
	cfg.Security.Username = "admin"
	cfg.Security.Password = password
	cfg.Security.JWTSecret = "test-secret"
	db, err := database.Open(cfg.Database.Path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close(db) })
	if err := database.Migrate(db); err != nil {
		t.Fatal(err)
	}
	prefs, err := settings.New(db, cfg)
	if err != nil {
		t.Fatal(err)
	}
	store := New(db)
	if err := store.Ensure(cfg, prefs); err != nil {
		t.Fatal(err)
	}
	return store, cfg, prefs
}

func TestEnsureKeepsExistingPasswordAndRoles(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Default()
	cfg.Database.Path = filepath.Join(dir, "nmonitor.db")
	cfg.Security.Username = "root"
	cfg.Security.Password = "from-config"
	db, err := database.Open(cfg.Database.Path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close(db) })
	if err := database.Migrate(db); err != nil {
		t.Fatal(err)
	}
	prefs, err := settings.New(db, cfg)
	if err != nil {
		t.Fatal(err)
	}
	hash, err := bcrypt.GenerateFromPassword([]byte("stored-secret"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	if err := prefs.Update(map[string]string{settings.KeyPasswordHash: string(hash)}); err != nil {
		t.Fatal(err)
	}
	store := New(db)
	if err := store.Ensure(cfg, prefs); err != nil {
		t.Fatal(err)
	}
	if err := store.Ensure(cfg, prefs); err != nil {
		t.Fatal(err)
	}
	principal, err := store.Authenticate("root", "stored-secret")
	if err != nil || !principal.Can(PermAccessWrite) || !principal.Can(PermMonitorRead) {
		t.Fatalf("admin login %+v %v", principal, err)
	}
	if _, err := store.Authenticate("root", "from-config"); err != ErrBadCredentials {
		t.Fatalf("config password should not replace the saved hash: %v", err)
	}
	users, err := store.ListUsers()
	if err != nil || len(users) != 1 || users[0].Username != "root" {
		t.Fatalf("users %+v %v", users, err)
	}

	roles, err := store.ListRoles()
	if err != nil || len(roles) != 3 || roles[0].Key != RoleAdmin || roles[0].UserCount != 1 {
		t.Fatalf("roles %+v %v", roles, err)
	}
	operator := roles[1]
	if _, err := store.UpdateRole(operator.ID, RoleInput{Name: operator.Name, Description: "只保留日志", Permissions: []string{PermLogRead}}); err != nil {
		t.Fatal(err)
	}
	if err := store.Ensure(cfg, prefs); err != nil {
		t.Fatal(err)
	}
	roles, _ = store.ListRoles()
	if len(roles[1].Permissions) != 1 || roles[1].Permissions[0] != PermLogRead {
		t.Fatalf("operator perms were reset: %+v", roles[1].Permissions)
	}
	admin, err := store.UpdateRole(roles[0].ID, RoleInput{Name: roles[0].Name, Permissions: []string{PermLogRead}})
	if err != nil || !containsPerm(admin.Permissions, PermAccessWrite) || !containsPerm(admin.Permissions, PermShellUse) {
		t.Fatalf("admin role must keep every permission: %+v %v", admin.Permissions, err)
	}
}

func TestEnsureUpgradesLegacyBuiltinPerms(t *testing.T) {
	store, cfg, prefs := testStore(t, "test-pass")
	roles, err := store.ListRoles()
	if err != nil {
		t.Fatal(err)
	}
	var operatorID, viewerID uint
	for _, role := range roles {
		if role.Key == RoleOperator {
			operatorID = role.ID
			if !containsPerm(role.Permissions, PermDockerWrite) || !containsPerm(role.Permissions, PermKubeWrite) {
				t.Fatalf("fresh operator %+v", role.Permissions)
			}
		}
		if role.Key == RoleViewer {
			viewerID = role.ID
			if !containsPerm(role.Permissions, PermDockerRead) || containsPerm(role.Permissions, PermDockerWrite) {
				t.Fatalf("fresh viewer %+v", role.Permissions)
			}
		}
	}
	raw, err := json.Marshal(legacyOperatorPerms())
	if err != nil {
		t.Fatal(err)
	}
	if err := store.db.Model(&model.Role{}).Where("key = ?", RoleOperator).Update("permissions", string(raw)).Error; err != nil {
		t.Fatal(err)
	}
	viewRaw, err := json.Marshal(legacyViewerPerms())
	if err != nil {
		t.Fatal(err)
	}
	if err := store.db.Model(&model.Role{}).Where("key = ?", RoleViewer).Update("permissions", string(viewRaw)).Error; err != nil {
		t.Fatal(err)
	}
	if err := store.Ensure(cfg, prefs); err != nil {
		t.Fatal(err)
	}
	roles, err = store.ListRoles()
	if err != nil {
		t.Fatal(err)
	}
	for _, role := range roles {
		if role.Key == RoleOperator && !containsPerm(role.Permissions, PermDockerWrite) {
			t.Fatalf("operator not upgraded %+v", role.Permissions)
		}
		if role.Key == RoleViewer && !containsPerm(role.Permissions, PermKubeRead) {
			t.Fatalf("viewer not upgraded %+v", role.Permissions)
		}
	}
	if _, err := store.UpdateRole(operatorID, RoleInput{Name: "运维", Permissions: []string{PermLogRead}}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.UpdateRole(viewerID, RoleInput{Name: "只读", Permissions: []string{PermCertRead}}); err != nil {
		t.Fatal(err)
	}
	if err := store.Ensure(cfg, prefs); err != nil {
		t.Fatal(err)
	}
	roles, _ = store.ListRoles()
	for _, role := range roles {
		if role.Key == RoleOperator && (len(role.Permissions) != 1 || role.Permissions[0] != PermLogRead) {
			t.Fatalf("custom operator reset %+v", role.Permissions)
		}
		if role.Key == RoleViewer && (len(role.Permissions) != 1 || role.Permissions[0] != PermCertRead) {
			t.Fatalf("custom viewer reset %+v", role.Permissions)
		}
	}
}

func TestUserAndRoleGuards(t *testing.T) {
	store, _, _ := testStore(t, "test-pass")
	roles, err := store.ListRoles()
	if err != nil {
		t.Fatal(err)
	}
	var adminID, viewerID uint
	for _, role := range roles {
		if role.Key == RoleAdmin {
			adminID = role.ID
		}
		if role.Key == RoleViewer {
			viewerID = role.ID
		}
	}
	admin, err := store.Authenticate("admin", "test-pass")
	if err != nil {
		t.Fatal(err)
	}
	on := true
	off := false
	viewer, err := store.CreateUser(UserInput{Username: "audit", DisplayName: "审计", Password: "audit-pass", Enabled: &on, RoleIDs: []uint{viewerID}})
	if err != nil || viewer.DisplayName != "审计" {
		t.Fatalf("create %+v %v", viewer, err)
	}
	if _, err := store.CreateUser(UserInput{Username: "audit", Password: "audit-pass", RoleIDs: []uint{viewerID}}); err != ErrUsernameTaken {
		t.Fatalf("duplicate %v", err)
	}
	if _, err := store.CreateUser(UserInput{Username: "1bad", Password: "audit-pass", RoleIDs: []uint{viewerID}}); err != ErrBadUsername {
		t.Fatalf("username %v", err)
	}
	principal, err := store.Authenticate("audit", "audit-pass")
	if err != nil || principal.Can(PermMonitorWrite) || !principal.Can(PermMonitorRead) || principal.Can(PermShellUse) {
		t.Fatalf("viewer %+v %v", principal.PermissionList(), err)
	}
	if _, err := store.UpdateUser(admin.ID, admin.ID, UserInput{Enabled: &off, RoleIDs: []uint{adminID}}); err != ErrDisableSelf {
		t.Fatalf("disable self %v", err)
	}
	if err := store.DeleteUser(admin.ID, admin.ID); err != ErrDeleteSelf {
		t.Fatalf("delete self %v", err)
	}
	if _, err := store.UpdateUser(viewer.ID, admin.ID, UserInput{Enabled: &on, RoleIDs: []uint{viewerID}}); err != ErrLastAdmin {
		t.Fatalf("last admin %v", err)
	}
	if err := store.DeleteUser(admin.ID, viewer.ID); err != nil {
		t.Fatal(err)
	}
	managed, err := store.CreateRole(RoleInput{Name: "监控管理", Permissions: []string{PermMonitorWrite}})
	if err != nil || !containsPerm(managed.Permissions, PermMonitorRead) {
		t.Fatalf("write implies read %+v %v", managed.Permissions, err)
	}
	if err := store.DeleteRole(managed.ID); err != nil {
		t.Fatal(err)
	}
	custom, err := store.CreateRole(RoleInput{Name: "终端", Permissions: []string{PermShellUse}})
	if err != nil || !containsPerm(custom.Permissions, PermShellUse) {
		t.Fatalf("role %+v %v", custom, err)
	}
	if _, err := store.CreateRole(RoleInput{Name: "终端", Permissions: []string{PermLogRead}}); err != ErrRoleNameTaken {
		t.Fatalf("role name %v", err)
	}
	shellUser, err := store.CreateUser(UserInput{Username: "ops", Password: "ops-pass", Enabled: &on, RoleIDs: []uint{custom.ID, viewerID}})
	if err != nil {
		t.Fatal(err)
	}
	principal, err = store.Authenticate("ops", "ops-pass")
	if err != nil || !principal.Can(PermShellUse) || !principal.Can(PermMonitorRead) {
		t.Fatalf("union %+v %v", principal.PermissionList(), err)
	}
	if _, err := store.PrincipalByID(shellUser.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.UpdateUser(admin.ID, shellUser.ID, UserInput{Enabled: &off, RoleIDs: []uint{custom.ID}}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.PrincipalByID(shellUser.ID); err != ErrDisabled {
		t.Fatalf("disabled %v", err)
	}
	if err := store.DeleteRole(custom.ID); err != ErrRoleInUse {
		t.Fatalf("in use %v", err)
	}
	if err := store.DeleteRole(viewerID); err != ErrBuiltinDelete {
		t.Fatalf("builtin %v", err)
	}
	if _, err := store.UpdateRole(viewerID, RoleInput{Name: "访客", Permissions: viewerPerms()}); err != ErrBuiltinRename {
		t.Fatalf("rename %v", err)
	}
	if err := store.DeleteUser(admin.ID, shellUser.ID); err != nil {
		t.Fatal(err)
	}
	if err := store.DeleteRole(custom.ID); err != nil {
		t.Fatal(err)
	}
	if !store.CheckPassword(admin.ID, "test-pass") {
		t.Fatal("password")
	}
	if err := store.SetPassword(admin.ID, "next-pass"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Authenticate("admin", "next-pass"); err != nil {
		t.Fatal(err)
	}
}

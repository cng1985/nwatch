package rbac

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/cng1985/nwatch/internal/config"
	"github.com/cng1985/nwatch/internal/model"
	"github.com/cng1985/nwatch/internal/settings"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

var usernameRE = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9._-]{1,31}$`)

// RoleBrief 是登录信息和用户列表里的角色摘要。
type RoleBrief struct {
	ID   uint   `json:"id"`
	Key  string `json:"key"`
	Name string `json:"name"`
}

// Principal 是一次请求对应的登录用户。
type Principal struct {
	ID          uint
	Username    string
	DisplayName string
	Enabled     bool
	Roles       []RoleBrief
	perms       map[string]struct{}
}

// Can 判断是否拥有权限。管理权限同时包含对应的查看权限。
func (p *Principal) Can(perm string) bool {
	if p == nil {
		return false
	}
	if _, ok := p.perms[perm]; ok {
		return true
	}
	if strings.HasSuffix(perm, ".read") {
		_, ok := p.perms[strings.TrimSuffix(perm, ".read")+".write"]
		return ok
	}
	return false
}

// PermissionList 按目录顺序返回已授予的权限。
func (p *Principal) PermissionList() []string {
	if p == nil {
		return []string{}
	}
	out := make([]string, 0, len(p.perms))
	for _, key := range All() {
		if _, ok := p.perms[key]; ok {
			out = append(out, key)
		}
	}
	return out
}

// UserView 是用户管理接口返回的账号。
type UserView struct {
	ID          uint        `json:"id"`
	Username    string      `json:"username"`
	DisplayName string      `json:"displayName"`
	Enabled     bool        `json:"enabled"`
	Roles       []RoleBrief `json:"roles"`
	CreatedAt   time.Time   `json:"createdAt"`
}

// RoleView 是角色管理接口返回的角色。
type RoleView struct {
	ID          uint      `json:"id"`
	Key         string    `json:"key"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Builtin     bool      `json:"builtin"`
	Permissions []string  `json:"permissions"`
	UserCount   int       `json:"userCount"`
	CreatedAt   time.Time `json:"createdAt"`
}

// UserInput 是创建或更新用户的输入。密码为空表示更新时不修改密码。
// Enabled 为空时，创建默认启用，更新则保持原状态。
type UserInput struct {
	Username    string
	DisplayName string
	Password    string
	Enabled     *bool
	RoleIDs     []uint
}

// RoleInput 是创建或更新角色的输入。
type RoleInput struct {
	Name        string
	Description string
	Permissions []string
}

// Store 保存用户和角色。
type Store struct {
	db *gorm.DB
}

func New(db *gorm.DB) *Store {
	return &Store{db: db}
}

// ConfigUsername 是首次启动时创建的管理员用户名。
func ConfigUsername(cfg *config.Config) string {
	name := strings.TrimSpace(cfg.Security.Username)
	if name == "" {
		return "admin"
	}
	return name
}

// Ensure 创建内置角色，并在还没有用户时用现有管理员密码创建第一个账号。
func (s *Store) Ensure(cfg *config.Config, prefs *settings.Store) error {
	if err := s.ensureRole(RoleAdmin, "管理员", "管理系统配置、用户、角色，以及全部监控和主机操作。", All(), true); err != nil {
		return err
	}
	if err := s.ensureRole(RoleOperator, "运维", "管理监控、通知、脚本、定时任务和系统服务，不能修改系统配置、使用在线终端或管理账号。", operatorPerms(), false); err != nil {
		return err
	}
	if err := s.ensureRole(RoleViewer, "只读", "查看监控、主机、告警和运行日志。", viewerPerms(), false); err != nil {
		return err
	}
	var count int64
	if err := s.db.Model(&model.User{}).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	var admin model.Role
	if err := s.db.Where("key = ?", RoleAdmin).First(&admin).Error; err != nil {
		return err
	}
	hash := ""
	if prefs != nil {
		hash = prefs.Snapshot().PasswordHash
	}
	if hash == "" {
		plain := ""
		if cfg != nil {
			plain = cfg.Security.Password
		}
		if plain == "" {
			plain = "admin"
		}
		encoded, err := hashPassword(plain)
		if err != nil {
			return err
		}
		hash = encoded
	}
	name := "admin"
	if cfg != nil {
		name = ConfigUsername(cfg)
	}
	display := name
	if name == "admin" {
		display = "管理员"
	}
	user := model.User{Username: name, DisplayName: display, PasswordHash: hash, Enabled: true}
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&user).Error; err != nil {
			return err
		}
		return tx.Model(&user).Association("Roles").Replace([]model.Role{admin})
	})
}

func (s *Store) ensureRole(key, name, description string, perms []string, syncPerms bool) error {
	raw, err := json.Marshal(perms)
	if err != nil {
		return err
	}
	var role model.Role
	err = s.db.Where("key = ?", key).First(&role).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		role = model.Role{Key: key, Name: name, Description: description, Builtin: true, Permissions: string(raw)}
		return s.db.Create(&role).Error
	}
	if err != nil {
		return err
	}
	updates := map[string]any{}
	if !role.Builtin {
		updates["builtin"] = true
	}
	if syncPerms && role.Permissions != string(raw) {
		updates["permissions"] = string(raw)
	}
	if len(updates) == 0 {
		return nil
	}
	return s.db.Model(&role).Updates(updates).Error
}

// Authenticate 校验用户名和密码。
func (s *Store) Authenticate(username, password string) (*Principal, error) {
	user, err := s.userByUsername(strings.TrimSpace(username))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, ErrBadCredentials
		}
		return nil, err
	}
	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)) != nil {
		return nil, ErrBadCredentials
	}
	if !user.Enabled {
		return nil, ErrDisabled
	}
	return principalOf(user), nil
}

// PrincipalByID 读取仍可登录的用户。停用或不存在时令牌失效。
func (s *Store) PrincipalByID(id uint) (*Principal, error) {
	user, err := s.userByID(id)
	if err != nil {
		return nil, err
	}
	if !user.Enabled {
		return nil, ErrDisabled
	}
	return principalOf(user), nil
}

// PrincipalByUsername 按用户名读取仍可登录的用户。
func (s *Store) PrincipalByUsername(username string) (*Principal, error) {
	user, err := s.userByUsername(strings.TrimSpace(username))
	if err != nil {
		return nil, err
	}
	if !user.Enabled {
		return nil, ErrDisabled
	}
	return principalOf(user), nil
}

// CheckPassword 校验指定用户的当前密码。
func (s *Store) CheckPassword(id uint, plain string) bool {
	var user model.User
	if err := s.db.Select("password_hash").First(&user, id).Error; err != nil {
		return false
	}
	return bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(plain)) == nil
}

// SetPassword 修改指定用户的密码。
func (s *Store) SetPassword(id uint, plain string) error {
	if err := validatePassword(plain); err != nil {
		return err
	}
	hash, err := hashPassword(plain)
	if err != nil {
		return err
	}
	res := s.db.Model(&model.User{}).Where("id = ?", id).Update("password_hash", hash)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) ListUsers() ([]UserView, error) {
	var users []model.User
	if err := s.db.Preload("Roles").Order("id").Find(&users).Error; err != nil {
		return nil, err
	}
	out := make([]UserView, 0, len(users))
	for _, user := range users {
		out = append(out, userView(user))
	}
	return out, nil
}

func (s *Store) CreateUser(in UserInput) (UserView, error) {
	username := strings.TrimSpace(in.Username)
	if !usernameRE.MatchString(username) {
		return UserView{}, ErrBadUsername
	}
	if err := validatePassword(in.Password); err != nil {
		return UserView{}, err
	}
	roles, err := s.rolesByIDs(s.db, in.RoleIDs)
	if err != nil {
		return UserView{}, err
	}
	var taken int64
	if err := s.db.Model(&model.User{}).Where("username = ?", username).Count(&taken).Error; err != nil {
		return UserView{}, err
	}
	if taken > 0 {
		return UserView{}, ErrUsernameTaken
	}
	hash, err := hashPassword(in.Password)
	if err != nil {
		return UserView{}, err
	}
	user := model.User{
		Username:     username,
		DisplayName:  trimRunes(in.DisplayName, 32),
		PasswordHash: hash,
		Enabled:      enabledValue(in.Enabled, true),
	}
	err = s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&user).Error; err != nil {
			return err
		}
		return tx.Model(&user).Association("Roles").Replace(roles)
	})
	if err != nil {
		return UserView{}, err
	}
	user.Roles = roles
	return userView(user), nil
}

func (s *Store) UpdateUser(actorID, id uint, in UserInput) (UserView, error) {
	if in.Password != "" {
		if err := validatePassword(in.Password); err != nil {
			return UserView{}, err
		}
	}
	roles, err := s.rolesByIDs(s.db, in.RoleIDs)
	if err != nil {
		return UserView{}, err
	}
	var view UserView
	err = s.db.Transaction(func(tx *gorm.DB) error {
		user, err := userByIDTx(tx, id)
		if err != nil {
			return err
		}
		enabled := enabledValue(in.Enabled, user.Enabled)
		if actorID != 0 && actorID == id && !enabled {
			return ErrDisableSelf
		}
		if err := ensureWriters(tx, func(users []model.User, perms map[uint][]string) ([]model.User, map[uint][]string) {
			for i := range users {
				if users[i].ID == id {
					users[i].Enabled = enabled
					users[i].Roles = roles
				}
			}
			return users, perms
		}); err != nil {
			return err
		}
		user.DisplayName = trimRunes(in.DisplayName, 32)
		user.Enabled = enabled
		if in.Password != "" {
			hash, err := hashPassword(in.Password)
			if err != nil {
				return err
			}
			user.PasswordHash = hash
		}
		if err := tx.Save(user).Error; err != nil {
			return err
		}
		if err := tx.Model(user).Association("Roles").Replace(roles); err != nil {
			return err
		}
		user.Roles = roles
		view = userView(*user)
		return nil
	})
	return view, err
}

func (s *Store) DeleteUser(actorID, id uint) error {
	if actorID != 0 && actorID == id {
		return ErrDeleteSelf
	}
	return s.db.Transaction(func(tx *gorm.DB) error {
		user, err := userByIDTx(tx, id)
		if err != nil {
			return err
		}
		if err := ensureWriters(tx, func(users []model.User, perms map[uint][]string) ([]model.User, map[uint][]string) {
			kept := users[:0]
			for _, item := range users {
				if item.ID != id {
					kept = append(kept, item)
				}
			}
			return kept, perms
		}); err != nil {
			return err
		}
		if err := tx.Model(user).Association("Roles").Clear(); err != nil {
			return err
		}
		return tx.Delete(&model.User{}, id).Error
	})
}

func (s *Store) ListRoles() ([]RoleView, error) {
	var roles []model.Role
	if err := s.db.Find(&roles).Error; err != nil {
		return nil, err
	}
	counts, err := s.roleUserCounts(s.db)
	if err != nil {
		return nil, err
	}
	sortRoles(roles)
	out := make([]RoleView, 0, len(roles))
	for _, role := range roles {
		out = append(out, roleView(role, counts[role.ID]))
	}
	return out, nil
}

func (s *Store) CreateRole(in RoleInput) (RoleView, error) {
	name := strings.TrimSpace(in.Name)
	if !validRoleName(name) {
		return RoleView{}, ErrBadName
	}
	perms, err := Normalize(in.Permissions)
	if err != nil {
		return RoleView{}, err
	}
	var taken int64
	if err := s.db.Model(&model.Role{}).Where("name = ?", name).Count(&taken).Error; err != nil {
		return RoleView{}, err
	}
	if taken > 0 {
		return RoleView{}, ErrRoleNameTaken
	}
	key, err := newRoleKey()
	if err != nil {
		return RoleView{}, err
	}
	raw, _ := json.Marshal(perms)
	role := model.Role{Key: key, Name: name, Description: trimRunes(in.Description, 200), Permissions: string(raw)}
	if err := s.db.Create(&role).Error; err != nil {
		return RoleView{}, err
	}
	return roleView(role, 0), nil
}

func (s *Store) UpdateRole(id uint, in RoleInput) (RoleView, error) {
	name := strings.TrimSpace(in.Name)
	if !validRoleName(name) {
		return RoleView{}, ErrBadName
	}
	var view RoleView
	err := s.db.Transaction(func(tx *gorm.DB) error {
		var role model.Role
		if err := tx.First(&role, id).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrRoleMissing
			}
			return err
		}
		if role.Builtin && name != role.Name {
			return ErrBuiltinRename
		}
		perms := All()
		if role.Key != RoleAdmin {
			var err error
			perms, err = Normalize(in.Permissions)
			if err != nil {
				return err
			}
		}
		var taken int64
		if err := tx.Model(&model.Role{}).Where("name = ? AND id <> ?", name, id).Count(&taken).Error; err != nil {
			return err
		}
		if taken > 0 {
			return ErrRoleNameTaken
		}
		if err := ensureWriters(tx, func(users []model.User, table map[uint][]string) ([]model.User, map[uint][]string) {
			table[role.ID] = perms
			return users, table
		}); err != nil {
			return err
		}
		raw, _ := json.Marshal(perms)
		role.Name = name
		role.Description = trimRunes(in.Description, 200)
		role.Permissions = string(raw)
		if err := tx.Save(&role).Error; err != nil {
			return err
		}
		counts, err := s.roleUserCounts(tx)
		if err != nil {
			return err
		}
		view = roleView(role, counts[role.ID])
		return nil
	})
	return view, err
}

func (s *Store) DeleteRole(id uint) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		var role model.Role
		if err := tx.First(&role, id).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrRoleMissing
			}
			return err
		}
		if role.Builtin {
			return ErrBuiltinDelete
		}
		var used int64
		if err := tx.Table("user_roles").Where("role_id = ?", id).Count(&used).Error; err != nil {
			return err
		}
		if used > 0 {
			return ErrRoleInUse
		}
		return tx.Delete(&model.Role{}, id).Error
	})
}

func (s *Store) userByID(id uint) (*model.User, error) {
	return userByIDTx(s.db, id)
}

func userByIDTx(tx *gorm.DB, id uint) (*model.User, error) {
	var user model.User
	err := tx.Preload("Roles").First(&user, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (s *Store) userByUsername(username string) (*model.User, error) {
	var user model.User
	err := s.db.Preload("Roles").Where("username = ?", username).First(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (s *Store) rolesByIDs(tx *gorm.DB, ids []uint) ([]model.Role, error) {
	ids = uniqueUints(ids)
	if len(ids) == 0 {
		return nil, ErrNoRole
	}
	var roles []model.Role
	if err := tx.Where("id IN ?", ids).Find(&roles).Error; err != nil {
		return nil, err
	}
	if len(roles) != len(ids) {
		return nil, ErrRoleMissing
	}
	return roles, nil
}

func (s *Store) roleUserCounts(tx *gorm.DB) (map[uint]int, error) {
	var rows []struct {
		RoleID uint
		N      int
	}
	err := tx.Table("user_roles").Select("role_id, count(*) as n").Group("role_id").Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := map[uint]int{}
	for _, row := range rows {
		out[row.RoleID] = row.N
	}
	return out, nil
}

func ensureWriters(tx *gorm.DB, mutate func([]model.User, map[uint][]string) ([]model.User, map[uint][]string)) error {
	var roles []model.Role
	if err := tx.Find(&roles).Error; err != nil {
		return err
	}
	perms := map[uint][]string{}
	for _, role := range roles {
		perms[role.ID] = decodePerms(role.Permissions)
	}
	var users []model.User
	if err := tx.Preload("Roles").Find(&users).Error; err != nil {
		return err
	}
	users, perms = mutate(users, perms)
	if countAccessWriters(users, perms) == 0 {
		return ErrLastAdmin
	}
	return nil
}

func countAccessWriters(users []model.User, perms map[uint][]string) int {
	n := 0
	for _, user := range users {
		if !user.Enabled {
			continue
		}
		for _, role := range user.Roles {
			if containsPerm(perms[role.ID], PermAccessWrite) {
				n++
				break
			}
		}
	}
	return n
}

func principalOf(user *model.User) *Principal {
	perms := map[string]struct{}{}
	roles := make([]RoleBrief, 0, len(user.Roles))
	for _, role := range user.Roles {
		roles = append(roles, RoleBrief{ID: role.ID, Key: role.Key, Name: role.Name})
		for _, key := range decodePerms(role.Permissions) {
			if knownPerm(key) {
				perms[key] = struct{}{}
			}
		}
	}
	display := strings.TrimSpace(user.DisplayName)
	if display == "" {
		display = user.Username
	}
	return &Principal{
		ID: user.ID, Username: user.Username, DisplayName: display,
		Enabled: user.Enabled, Roles: roles, perms: perms,
	}
}

func userView(user model.User) UserView {
	roles := make([]RoleBrief, 0, len(user.Roles))
	for _, role := range user.Roles {
		roles = append(roles, RoleBrief{ID: role.ID, Key: role.Key, Name: role.Name})
	}
	sortBriefs(roles)
	return UserView{
		ID: user.ID, Username: user.Username, DisplayName: user.DisplayName,
		Enabled: user.Enabled, Roles: roles, CreatedAt: user.CreatedAt,
	}
}

func roleView(role model.Role, users int) RoleView {
	return RoleView{
		ID: role.ID, Key: role.Key, Name: role.Name, Description: role.Description,
		Builtin: role.Builtin, Permissions: decodePerms(role.Permissions),
		UserCount: users, CreatedAt: role.CreatedAt,
	}
}

func decodePerms(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return []string{}
	}
	var list []string
	if err := json.Unmarshal([]byte(raw), &list); err != nil || list == nil {
		return []string{}
	}
	return list
}

func hashPassword(plain string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

func validatePassword(plain string) error {
	if len(plain) < 6 || len(plain) > 72 {
		return ErrBadPassword
	}
	return nil
}

func validRoleName(name string) bool {
	n := len([]rune(name))
	return n > 0 && n <= 32
}

func trimRunes(value string, max int) string {
	value = strings.TrimSpace(value)
	runes := []rune(value)
	if len(runes) > max {
		return string(runes[:max])
	}
	return value
}

func enabledValue(value *bool, fallback bool) bool {
	if value == nil {
		return fallback
	}
	return *value
}

func uniqueUints(ids []uint) []uint {
	seen := map[uint]struct{}{}
	out := make([]uint, 0, len(ids))
	for _, id := range ids {
		if id == 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

func containsPerm(list []string, key string) bool {
	for _, item := range list {
		if item == key {
			return true
		}
	}
	return false
}

func newRoleKey() (string, error) {
	buf := make([]byte, 6)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return "r" + hex.EncodeToString(buf), nil
}

func sortRoles(roles []model.Role) {
	order := map[string]int{RoleAdmin: 0, RoleOperator: 1, RoleViewer: 2}
	for i := 1; i < len(roles); i++ {
		item := roles[i]
		j := i
		for j > 0 && roleLess(item, roles[j-1], order) {
			roles[j] = roles[j-1]
			j--
		}
		roles[j] = item
	}
}

func roleLess(a, b model.Role, order map[string]int) bool {
	if a.Builtin != b.Builtin {
		return a.Builtin
	}
	if a.Builtin && b.Builtin && order[a.Key] != order[b.Key] {
		return order[a.Key] < order[b.Key]
	}
	return a.ID < b.ID
}

func sortBriefs(roles []RoleBrief) {
	order := map[string]int{RoleAdmin: 0, RoleOperator: 1, RoleViewer: 2}
	for i := 1; i < len(roles); i++ {
		item := roles[i]
		j := i
		for j > 0 && briefLess(item, roles[j-1], order) {
			roles[j] = roles[j-1]
			j--
		}
		roles[j] = item
	}
}

func briefLess(a, b RoleBrief, order map[string]int) bool {
	ao, aok := order[a.Key]
	bo, bok := order[b.Key]
	if aok != bok {
		return aok
	}
	if aok && bok && ao != bo {
		return ao < bo
	}
	return a.ID < b.ID
}

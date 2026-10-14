package auth

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/cng1985/nwatch/internal/config"
	"github.com/cng1985/nwatch/internal/rbac"
	"github.com/cng1985/nwatch/internal/settings"
	"github.com/golang-jwt/jwt/v5"
	"gorm.io/gorm"
)

type Service struct {
	secret []byte
	ttl    time.Duration
	users  *rbac.Store

	mu       sync.Mutex
	attempts map[string][]time.Time
}

type Claims struct {
	Username string `json:"username"`
	UserID   uint   `json:"uid"`
	jwt.RegisteredClaims
}

func New(cfg *config.Config, db *gorm.DB, store *settings.Store) (*Service, error) {
	secret, err := loadSecret(cfg)
	if err != nil {
		return nil, err
	}
	users := rbac.New(db)
	if err := users.Ensure(cfg, store); err != nil {
		return nil, err
	}
	ttl := cfg.Security.JWTTTL
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	return &Service{
		secret:   secret,
		ttl:      ttl,
		users:    users,
		attempts: map[string][]time.Time{},
	}, nil
}

// Users 返回用户和角色存储。
func (s *Service) Users() *rbac.Store { return s.users }

func loadSecret(cfg *config.Config) ([]byte, error) {
	if cfg.Security.JWTSecret != "" {
		return []byte(cfg.Security.JWTSecret), nil
	}
	path := filepath.Join(filepath.Dir(cfg.Database.Path), "jwt.secret")
	if b, err := os.ReadFile(path); err == nil && len(b) >= 16 {
		return b, nil
	}
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return nil, err
	}
	encoded := []byte(hex.EncodeToString(buf))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	if err := os.WriteFile(path, encoded, 0o600); err != nil {
		return nil, err
	}
	return encoded, nil
}

var ErrRateLimited = errors.New("尝试过于频繁，请稍后再试")

func (s *Service) Login(ip, username, password string) (string, *rbac.Principal, error) {
	if !s.allow(ip) {
		return "", nil, ErrRateLimited
	}
	principal, err := s.users.Authenticate(username, password)
	if err != nil {
		return "", nil, err
	}
	token, err := s.sign(principal)
	if err != nil {
		return "", nil, err
	}
	return token, principal, nil
}

// Principal 根据令牌加载当前用户。用户被停用或删除后，原来的令牌不能继续使用。
func (s *Service) Principal(claims *Claims) (*rbac.Principal, error) {
	if claims == nil {
		return nil, errors.New("无效令牌")
	}
	if claims.UserID > 0 {
		return s.users.PrincipalByID(claims.UserID)
	}
	return s.users.PrincipalByUsername(claims.Username)
}

func (s *Service) sign(principal *rbac.Principal) (string, error) {
	now := time.Now()
	claims := Claims{
		Username: principal.Username,
		UserID:   principal.ID,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   principal.Username,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(s.ttl)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.secret)
}

func (s *Service) Parse(token string) (*Claims, error) {
	parsed, err := jwt.ParseWithClaims(token, &Claims{}, func(t *jwt.Token) (any, error) {
		if t.Method != jwt.SigningMethodHS256 {
			return nil, errors.New("签名算法不正确")
		}
		return s.secret, nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := parsed.Claims.(*Claims)
	if !ok || !parsed.Valid {
		return nil, errors.New("无效令牌")
	}
	return claims, nil
}

func (s *Service) allow(ip string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	cutoff := now.Add(-time.Minute)
	prev := s.attempts[ip]
	kept := prev[:0]
	for _, ts := range prev {
		if ts.After(cutoff) {
			kept = append(kept, ts)
		}
	}
	if len(kept) >= 8 {
		s.attempts[ip] = kept
		return false
	}
	s.attempts[ip] = append(kept, now)
	return true
}

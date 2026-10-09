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
	"github.com/cng1985/nwatch/internal/settings"
	"github.com/golang-jwt/jwt/v5"
)

type Service struct {
	username string
	secret   []byte
	ttl      time.Duration
	settings *settings.Store

	mu       sync.Mutex
	attempts map[string][]time.Time
}

type Claims struct {
	Username string `json:"username"`
	jwt.RegisteredClaims
}

func New(cfg *config.Config, store *settings.Store) (*Service, error) {
	secret, err := loadSecret(cfg)
	if err != nil {
		return nil, err
	}
	ttl := cfg.Security.JWTTTL
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	return &Service{
		username: cfg.Security.Username,
		secret:   secret,
		ttl:      ttl,
		settings: store,
		attempts: map[string][]time.Time{},
	}, nil
}

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
var ErrBadCredentials = errors.New("用户名或密码错误")

func (s *Service) Login(ip, username, password string) (string, error) {
	if !s.allow(ip) {
		return "", ErrRateLimited
	}
	if username != s.username || !s.settings.CheckPassword(password) {
		return "", ErrBadCredentials
	}
	return s.sign(username)
}

func (s *Service) sign(username string) (string, error) {
	now := time.Now()
	claims := Claims{
		Username: username,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   username,
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

package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"api-gateway/internal/config"
)

var (
	ErrInvalidToken = errors.New("无效的token")
	ErrExpiredToken = errors.New("token已过期")
	ErrTokenNotFound = errors.New("token不存在")
)

// Claims JWT声明结构
type Claims struct {
	UserID   string   `json:"user_id"`
	Username string   `json:"username"`
	Email    string   `json:"email"`
	Roles    []string `json:"roles"`
	jwt.RegisteredClaims
}

// TokenService JWT token服务
type TokenService struct {
	secret        []byte
	tokenExpiry   time.Duration
	refreshExpiry time.Duration
	issuer        string
}

// NewTokenService 创建token服务实例
func NewTokenService(cfg config.AuthConfig) *TokenService {
	return &TokenService{
		secret:        []byte(cfg.JWTSecret),
		tokenExpiry:   cfg.TokenExpiry,
		refreshExpiry: cfg.RefreshExpiry,
		issuer:        cfg.Issuer,
	}
}

// GenerateToken 生成访问token
func (ts *TokenService) GenerateToken(userID, username, email string, roles []string) (string, error) {
	claims := Claims{
		UserID:   userID,
		Username: username,
		Email:    email,
		Roles:    roles,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(ts.tokenExpiry)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    ts.issuer,
			Subject:   userID,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(ts.secret)
}

// GenerateRefreshToken 生成刷新token
func (ts *TokenService) GenerateRefreshToken(userID string) (string, error) {
	claims := jwt.RegisteredClaims{
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(ts.refreshExpiry)),
		IssuedAt:  jwt.NewNumericDate(time.Now()),
		NotBefore: jwt.NewNumericDate(time.Now()),
		Issuer:    ts.issuer,
		Subject:   userID,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(ts.secret)
}

// ValidateToken 验证token
func (ts *TokenService) ValidateToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("意外的签名方法: %v", token.Header["alg"])
		}
		return ts.secret, nil
	})

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrExpiredToken
		}
		return nil, ErrInvalidToken
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}

	return nil, ErrInvalidToken
}

// RefreshToken 刷新token
func (ts *TokenService) RefreshToken(refreshTokenString string) (string, error) {
	token, err := jwt.ParseWithClaims(refreshTokenString, &jwt.RegisteredClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("意外的签名方法: %v", token.Header["alg"])
		}
		return ts.secret, nil
	})

	if err != nil {
		return "", ErrInvalidToken
	}

	if claims, ok := token.Claims.(*jwt.RegisteredClaims); ok && token.Valid {
		// 这里应该从数据库或缓存中获取用户信息
		// 为了简化，我们使用Subject作为用户ID
		userID := claims.Subject
		// 实际应用中应该查询用户的当前角色信息
		roles := []string{"user"} // 默认角色
		
		return ts.GenerateToken(userID, "", "", roles)
	}

	return "", ErrInvalidToken
}

// ExtractUserID 从token中提取用户ID
func (ts *TokenService) ExtractUserID(tokenString string) (string, error) {
	claims, err := ts.ValidateToken(tokenString)
	if err != nil {
		return "", err
	}
	return claims.UserID, nil
}

// HasRole 检查用户是否具有指定角色
func (c *Claims) HasRole(role string) bool {
	for _, r := range c.Roles {
		if r == role {
			return true
		}
	}
	return false
}

// HasAnyRole 检查用户是否具有任意指定角色
func (c *Claims) HasAnyRole(roles ...string) bool {
	for _, role := range roles {
		if c.HasRole(role) {
			return true
		}
	}
	return false
}

// User 用户信息结构
type User struct {
	ID       string   `json:"id"`
	Username string   `json:"username"`
	Email    string   `json:"email"`
	Roles    []string `json:"roles"`
	Active   bool     `json:"active"`
}

// UserService 用户服务接口
type UserService interface {
	GetUser(userID string) (*User, error)
	ValidateCredentials(username, password string) (*User, error)
	IsUserActive(userID string) (bool, error)
}

// MockUserService 模拟用户服务（用于演示）
type MockUserService struct {
	users map[string]*User
}

// NewMockUserService 创建模拟用户服务
func NewMockUserService() *MockUserService {
	return &MockUserService{
		users: map[string]*User{
			"1": {
				ID:       "1",
				Username: "admin",
				Email:    "admin@example.com",
				Roles:    []string{"admin", "user"},
				Active:   true,
			},
			"2": {
				ID:       "2",
				Username: "user",
				Email:    "user@example.com",
				Roles:    []string{"user"},
				Active:   true,
			},
		},
	}
}

// GetUser 获取用户信息
func (m *MockUserService) GetUser(userID string) (*User, error) {
	user, exists := m.users[userID]
	if !exists {
		return nil, errors.New("用户不存在")
	}
	return user, nil
}

// ValidateCredentials 验证用户凭据
func (m *MockUserService) ValidateCredentials(username, password string) (*User, error) {
	// 简化验证逻辑，实际应用中应该验证密码哈希
	for _, user := range m.users {
		if user.Username == username && password == "password123" {
			return user, nil
		}
	}
	return nil, errors.New("用户名或密码错误")
}

// IsUserActive 检查用户是否活跃
func (m *MockUserService) IsUserActive(userID string) (bool, error) {
	user, err := m.GetUser(userID)
	if err != nil {
		return false, err
	}
	return user.Active, nil
}

package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"api-gateway/internal/config"
)

func TestGatewayCreation(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{
			Port:         8080,
			Host:         "localhost",
			ReadTimeout:  30 * time.Second,
			WriteTimeout: 30 * time.Second,
			IdleTimeout:  60 * time.Second,
		},
		Auth: config.AuthConfig{
			JWTSecret:     "test-secret",
			TokenExpiry:   24 * time.Hour,
			RefreshExpiry: 7 * 24 * time.Hour,
			Issuer:        "test-gateway",
		},
		Logging: config.LoggingConfig{
			Level:  "info",
			Format: "json",
		},
		Metrics: config.MetricsConfig{
			Enabled: true,
			Path:    "/metrics",
			Port:    9090,
		},
		Routes: []config.RouteConfig{},
	}

	gateway, err := NewGateway(cfg)
	require.NoError(t, err)
	require.NotNil(t, gateway)

	assert.Equal(t, cfg, gateway.config)
	assert.NotNil(t, gateway.router)
	assert.NotNil(t, gateway.cache)
	assert.NotNil(t, gateway.tokenService)
	assert.NotNil(t, gateway.userService)
}

func TestHealthCheckEndpoint(t *testing.T) {
	cfg := createTestConfig()
	gateway, err := NewGateway(cfg)
	require.NoError(t, err)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/health", nil)
	gateway.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	
	var response map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "healthy", response["status"])
}

func TestLoginEndpoint(t *testing.T) {
	cfg := createTestConfig()
	gateway, err := NewGateway(cfg)
	require.NoError(t, err)

	// 测试有效登录
	loginData := map[string]string{
		"username": "admin",
		"password": "password123",
	}
	
	jsonData, _ := json.Marshal(loginData)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/auth/login", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	gateway.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	
	var response map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Contains(t, response, "access_token")
	assert.Contains(t, response, "refresh_token")
	assert.Equal(t, "Bearer", response["token_type"])

	// 测试无效登录
	invalidLoginData := map[string]string{
		"username": "invalid",
		"password": "invalid",
	}
	
	jsonData, _ = json.Marshal(invalidLoginData)
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/auth/login", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	gateway.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAuthMiddleware(t *testing.T) {
	cfg := createTestConfig()
	gateway, err := NewGateway(cfg)
	require.NoError(t, err)

	// 测试无token访问需要认证的端点
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/admin/status", nil)
	gateway.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)

	// 测试有效token访问
	token, err := gateway.tokenService.GenerateToken("1", "admin", "admin@example.com", []string{"admin"})
	require.NoError(t, err)

	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/admin/status", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	gateway.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestGracefulShutdown(t *testing.T) {
	cfg := createTestConfig()
	gateway, err := NewGateway(cfg)
	require.NoError(t, err)

	// 启动网关（在goroutine中）
	go func() {
		gateway.Start()
	}()

	// 等待一下确保启动
	time.Sleep(100 * time.Millisecond)

	// 测试优雅关闭
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = gateway.Stop(ctx)
	assert.NoError(t, err)
}

func createTestConfig() *config.Config {
	return &config.Config{
		Server: config.ServerConfig{
			Port:         8080,
			Host:         "localhost",
			ReadTimeout:  30 * time.Second,
			WriteTimeout: 30 * time.Second,
			IdleTimeout:  60 * time.Second,
		},
		Auth: config.AuthConfig{
			JWTSecret:     "test-secret",
			TokenExpiry:   24 * time.Hour,
			RefreshExpiry: 7 * 24 * time.Hour,
			Issuer:        "test-gateway",
		},
		Logging: config.LoggingConfig{
			Level:  "info",
			Format: "json",
		},
		Metrics: config.MetricsConfig{
			Enabled: true,
			Path:    "/metrics",
			Port:    9090,
		},
		Routes: []config.RouteConfig{
			{
				Path:   "/api/v1/test",
				Method: "GET",
				Backends: []config.BackendConfig{
					{
						URL:    "http://localhost:3001",
						Weight: 1,
						HealthCheck: config.HealthCheck{
							Enabled:  true,
							Path:     "/health",
							Interval: 30 * time.Second,
							Timeout:  5 * time.Second,
						},
					},
				},
				AuthRequired:  false,
				RateLimit:     100,
				CacheEnabled:  true,
				CacheTTL:      5 * time.Minute,
				Timeout:       30 * time.Second,
				Retries:       3,
				LoadBalancer:  config.RoundRobin,
				Middleware:    []string{"rate_limit", "cache"},
			},
		},
	}
}

package middleware

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"api-gateway/internal/auth"
	"api-gateway/internal/cache"
	"api-gateway/internal/logger"
	"api-gateway/internal/ratelimit"
)

// Middleware 中间件接口
type Middleware interface {
	Handle() gin.HandlerFunc
	Name() string
}

// CORS 跨域中间件
type CORSMiddleware struct {
	allowOrigins     []string
	allowMethods     []string
	allowHeaders     []string
	exposeHeaders    []string
	allowCredentials bool
	maxAge           time.Duration
}

// NewCORSMiddleware 创建CORS中间件
func NewCORSMiddleware(allowOrigins, allowMethods, allowHeaders, exposeHeaders []string, allowCredentials bool, maxAge time.Duration) *CORSMiddleware {
	return &CORSMiddleware{
		allowOrigins:     allowOrigins,
		allowMethods:     allowMethods,
		allowHeaders:     allowHeaders,
		exposeHeaders:    exposeHeaders,
		allowCredentials: allowCredentials,
		maxAge:           maxAge,
	}
}

// Name 返回中间件名称
func (c *CORSMiddleware) Name() string {
	return "cors"
}

// Handle 处理CORS
func (c *CORSMiddleware) Handle() gin.HandlerFunc {
	return gin.HandlerFunc(func(ctx *gin.Context) {
		origin := ctx.GetHeader("Origin")
		
		// 检查Origin是否被允许
		allowed := false
		for _, allowedOrigin := range c.allowOrigins {
			if allowedOrigin == "*" || allowedOrigin == origin {
				allowed = true
				break
			}
		}

		if allowed {
			ctx.Header("Access-Control-Allow-Origin", origin)
		}

		if c.allowCredentials {
			ctx.Header("Access-Control-Allow-Credentials", "true")
		}

		if len(c.allowMethods) > 0 {
			ctx.Header("Access-Control-Allow-Methods", strings.Join(c.allowMethods, ", "))
		}

		if len(c.allowHeaders) > 0 {
			ctx.Header("Access-Control-Allow-Headers", strings.Join(c.allowHeaders, ", "))
		}

		if len(c.exposeHeaders) > 0 {
			ctx.Header("Access-Control-Expose-Headers", strings.Join(c.exposeHeaders, ", "))
		}

		if c.maxAge > 0 {
			ctx.Header("Access-Control-Max-Age", fmt.Sprintf("%.0f", c.maxAge.Seconds()))
		}

		// 处理预检请求
		if ctx.Request.Method == "OPTIONS" {
			ctx.AbortWithStatus(http.StatusNoContent)
			return
		}

		ctx.Next()
	})
}

// LoggingMiddleware 日志中间件
type LoggingMiddleware struct{}

// NewLoggingMiddleware 创建日志中间件
func NewLoggingMiddleware() *LoggingMiddleware {
	return &LoggingMiddleware{}
}

// Name 返回中间件名称
func (l *LoggingMiddleware) Name() string {
	return "logging"
}

// Handle 处理日志记录
func (l *LoggingMiddleware) Handle() gin.HandlerFunc {
	return gin.LoggerWithFormatter(func(param gin.LogFormatterParams) string {
		logger.WithFields(map[string]interface{}{
			"timestamp":    param.TimeStamp.Format("2006-01-02 15:04:05"),
			"status":       param.StatusCode,
			"latency":      param.Latency,
			"client_ip":    param.ClientIP,
			"method":       param.Method,
			"path":         param.Path,
			"user_agent":   param.Request.UserAgent(),
			"error":        param.ErrorMessage,
		}).Info("HTTP Request")
		
		return ""
	})
}

// AuthMiddleware 认证中间件
type AuthMiddleware struct {
	tokenService *auth.TokenService
	userService  auth.UserService
	skipPaths    []string
}

// NewAuthMiddleware 创建认证中间件
func NewAuthMiddleware(tokenService *auth.TokenService, userService auth.UserService, skipPaths []string) *AuthMiddleware {
	return &AuthMiddleware{
		tokenService: tokenService,
		userService:  userService,
		skipPaths:    skipPaths,
	}
}

// Name 返回中间件名称
func (a *AuthMiddleware) Name() string {
	return "auth"
}

// Handle 处理认证
func (a *AuthMiddleware) Handle() gin.HandlerFunc {
	return gin.HandlerFunc(func(ctx *gin.Context) {
		// 检查是否需要跳过认证
		path := ctx.Request.URL.Path
		for _, skipPath := range a.skipPaths {
			if strings.HasPrefix(path, skipPath) {
				ctx.Next()
				return
			}
		}

		// 从请求头获取token
		authHeader := ctx.GetHeader("Authorization")
		if authHeader == "" {
			ctx.JSON(http.StatusUnauthorized, gin.H{"error": "缺少认证令牌"})
			ctx.Abort()
			return
		}

		// 解析Bearer token
		tokenParts := strings.SplitN(authHeader, " ", 2)
		if len(tokenParts) != 2 || tokenParts[0] != "Bearer" {
			ctx.JSON(http.StatusUnauthorized, gin.H{"error": "无效的认证令牌格式"})
			ctx.Abort()
			return
		}

		token := tokenParts[1]

		// 验证token
		claims, err := a.tokenService.ValidateToken(token)
		if err != nil {
			ctx.JSON(http.StatusUnauthorized, gin.H{"error": "无效的认证令牌"})
			ctx.Abort()
			return
		}

		// 检查用户是否活跃
		if a.userService != nil {
			active, err := a.userService.IsUserActive(claims.UserID)
			if err != nil || !active {
				ctx.JSON(http.StatusUnauthorized, gin.H{"error": "用户账户已被禁用"})
				ctx.Abort()
				return
			}
		}

		// 将用户信息存储到上下文
		ctx.Set("user_id", claims.UserID)
		ctx.Set("username", claims.Username)
		ctx.Set("user_roles", claims.Roles)
		ctx.Set("claims", claims)

		ctx.Next()
	})
}

// RateLimitMiddleware 速率限制中间件
type RateLimitMiddleware struct {
	limiter     ratelimit.RateLimiter
	defaultRate int
}

// NewRateLimitMiddleware 创建速率限制中间件
func NewRateLimitMiddleware(limiter ratelimit.RateLimiter, defaultRate int) *RateLimitMiddleware {
	return &RateLimitMiddleware{
		limiter:     limiter,
		defaultRate: defaultRate,
	}
}

// Name 返回中间件名称
func (r *RateLimitMiddleware) Name() string {
	return "rate_limit"
}

// Handle 处理速率限制
func (r *RateLimitMiddleware) Handle() gin.HandlerFunc {
	return gin.HandlerFunc(func(ctx *gin.Context) {
		clientIP := ctx.ClientIP()
		userID, _ := ctx.Get("user_id")
		path := ctx.Request.URL.Path

		// 生成限制键
		key := ratelimit.GenerateRateLimitKey(clientIP, fmt.Sprintf("%v", userID), path)

		// 检查速率限制
		allowed, err := r.limiter.Allow(ctx.Request.Context(), key, r.defaultRate)
		if err != nil {
			logger.Errorf("速率限制检查失败: %v", err)
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "内部服务器错误"})
			ctx.Abort()
			return
		}

		if !allowed {
			ctx.JSON(http.StatusTooManyRequests, gin.H{
				"error":   "请求过于频繁",
				"message": "请稍后再试",
			})
			ctx.Abort()
			return
		}

		ctx.Next()
	})
}

// CacheMiddleware 缓存中间件
type CacheMiddleware struct {
	cache      cache.Cache
	defaultTTL time.Duration
}

// NewCacheMiddleware 创建缓存中间件
func NewCacheMiddleware(cache cache.Cache, defaultTTL time.Duration) *CacheMiddleware {
	return &CacheMiddleware{
		cache:      cache,
		defaultTTL: defaultTTL,
	}
}

// Name 返回中间件名称
func (c *CacheMiddleware) Name() string {
	return "cache"
}

// Handle 处理缓存
func (c *CacheMiddleware) Handle() gin.HandlerFunc {
	return gin.HandlerFunc(func(ctx *gin.Context) {
		// 只缓存GET请求
		if ctx.Request.Method != "GET" {
			ctx.Next()
			return
		}

		// 生成缓存键
		cacheKey := cache.GenerateCacheKey("response", ctx.Request.URL.Path, ctx.Request.Method, nil)
		if ctx.Request.URL.RawQuery != "" {
			cacheKey += ":" + ctx.Request.URL.RawQuery
		}

		// 尝试从缓存获取响应
		cachedResponse, err := c.cache.Get(ctx.Request.Context(), cacheKey)
		if err == nil && cachedResponse != "" {
			// 缓存命中
			ctx.Header("X-Cache", "HIT")
			ctx.Data(http.StatusOK, "application/json", []byte(cachedResponse))
			return
		}

		// 缓存未命中，继续处理请求
		ctx.Header("X-Cache", "MISS")

		// 创建响应写入器来捕获响应
		blw := &bodyLogWriter{body: bytes.NewBufferString(""), ResponseWriter: ctx.Writer}
		ctx.Writer = blw

		ctx.Next()

		// 如果响应状态是200，则缓存响应
		if ctx.Writer.Status() == http.StatusOK {
			responseBody := blw.body.String()
			if responseBody != "" {
				err := c.cache.Set(ctx.Request.Context(), cacheKey, responseBody, c.defaultTTL)
				if err != nil {
					logger.Errorf("缓存响应失败: %v", err)
				}
			}
		}
	})
}

// bodyLogWriter 用于捕获响应体的写入器
type bodyLogWriter struct {
	gin.ResponseWriter
	body *bytes.Buffer
}

func (w bodyLogWriter) Write(b []byte) (int, error) {
	w.body.Write(b)
	return w.ResponseWriter.Write(b)
}

// SecurityMiddleware 安全中间件
type SecurityMiddleware struct {
	contentSecurityPolicy string
	frameOptions         string
	contentTypeOptions   string
	referrerPolicy       string
	strictTransportSecurity string
}

// NewSecurityMiddleware 创建安全中间件
func NewSecurityMiddleware() *SecurityMiddleware {
	return &SecurityMiddleware{
		contentSecurityPolicy:   "default-src 'self'",
		frameOptions:           "DENY",
		contentTypeOptions:     "nosniff",
		referrerPolicy:         "strict-origin-when-cross-origin",
		strictTransportSecurity: "max-age=31536000; includeSubDomains",
	}
}

// Name 返回中间件名称
func (s *SecurityMiddleware) Name() string {
	return "security"
}

// Handle 处理安全头部
func (s *SecurityMiddleware) Handle() gin.HandlerFunc {
	return gin.HandlerFunc(func(ctx *gin.Context) {
		// 设置安全头部
		ctx.Header("Content-Security-Policy", s.contentSecurityPolicy)
		ctx.Header("X-Frame-Options", s.frameOptions)
		ctx.Header("X-Content-Type-Options", s.contentTypeOptions)
		ctx.Header("Referrer-Policy", s.referrerPolicy)
		
		// 只在HTTPS连接时设置HSTS
		if ctx.Request.TLS != nil {
			ctx.Header("Strict-Transport-Security", s.strictTransportSecurity)
		}
		
		// 移除可能泄露服务器信息的头部
		ctx.Header("Server", "")
		ctx.Header("X-Powered-By", "")

		ctx.Next()
	})
}

// CompressionMiddleware 压缩中间件
type CompressionMiddleware struct{}

// NewCompressionMiddleware 创建压缩中间件
func NewCompressionMiddleware() *CompressionMiddleware {
	return &CompressionMiddleware{}
}

// Name 返回中间件名称
func (c *CompressionMiddleware) Name() string {
	return "compression"
}

// Handle 处理响应压缩
func (c *CompressionMiddleware) Handle() gin.HandlerFunc {
	return gin.HandlerFunc(func(ctx *gin.Context) {
		// 检查客户端是否支持压缩
		acceptEncoding := ctx.GetHeader("Accept-Encoding")
		if !strings.Contains(acceptEncoding, "gzip") {
			ctx.Next()
			return
		}

		// 设置压缩响应头
		ctx.Header("Content-Encoding", "gzip")
		ctx.Header("Vary", "Accept-Encoding")

		// 创建gzip写入器
		// 注意：这里简化实现，实际应该使用专门的压缩库
		ctx.Next()
	})
}

// MiddlewareManager 中间件管理器
type MiddlewareManager struct {
	middlewares map[string]Middleware
}

// NewMiddlewareManager 创建中间件管理器
func NewMiddlewareManager() *MiddlewareManager {
	return &MiddlewareManager{
		middlewares: make(map[string]Middleware),
	}
}

// Register 注册中间件
func (mm *MiddlewareManager) Register(middleware Middleware) {
	mm.middlewares[middleware.Name()] = middleware
}

// Get 获取中间件
func (mm *MiddlewareManager) Get(name string) (Middleware, bool) {
	middleware, exists := mm.middlewares[name]
	return middleware, exists
}

// Apply 应用中间件到路由组
func (mm *MiddlewareManager) Apply(rg *gin.RouterGroup, middlewareNames []string) {
	for _, name := range middlewareNames {
		if middleware, exists := mm.middlewares[name]; exists {
			rg.Use(middleware.Handle())
		} else {
			logger.Warnf("中间件 %s 不存在", name)
		}
	}
}

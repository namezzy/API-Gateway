package gateway

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"api-gateway/internal/auth"
	"api-gateway/internal/cache"
	"api-gateway/internal/config"
	"api-gateway/internal/healthcheck"
	"api-gateway/internal/loadbalancer"
	"api-gateway/internal/logger"
	"api-gateway/internal/metrics"
	"api-gateway/internal/middleware"
	"api-gateway/internal/ratelimit"
)

// Gateway API网关核心结构
type Gateway struct {
	config            *config.Config
	router            *gin.Engine
	middlewareManager *middleware.MiddlewareManager
	loadBalancers     map[string]loadbalancer.LoadBalancer
	cache             cache.Cache
	tokenService      *auth.TokenService
	userService       auth.UserService
	rateLimiter       ratelimit.RateLimiter
	healthChecker     *healthcheck.BackendHealthChecker
	systemChecker     *healthcheck.SystemHealthChecker
	metricsCollector  *metrics.MetricsCollector
	httpClient        *http.Client
	server            *http.Server
}

// NewGateway 创建新的网关实例
func NewGateway(cfg *config.Config) (*Gateway, error) {
	// 设置Gin模式
	if cfg.Logging.Level == "debug" {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	// 创建缓存
	var cacheInstance cache.Cache
	var err error
	if cfg.Redis.Addr != "" {
		cacheInstance, err = cache.NewRedisCache(cfg.Redis)
		if err != nil {
			logger.Warnf("Redis连接失败，使用内存缓存: %v", err)
			cacheInstance = cache.NewMemoryCache()
		}
	} else {
		cacheInstance = cache.NewMemoryCache()
	}

	// 创建认证服务
	tokenService := auth.NewTokenService(cfg.Auth)
	userService := auth.NewMockUserService()

	// 创建速率限制器
	rateLimiter := ratelimit.NewTokenBucketLimiter(cacheInstance)

	// 创建健康检查器
	healthChecker := healthcheck.NewBackendHealthChecker()
	systemChecker := healthcheck.NewSystemHealthChecker()

	// 创建指标收集器
	metricsCollector := metrics.NewMetricsCollector()

	// 创建HTTP客户端
	httpClient := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 20,
			IdleConnTimeout:     90 * time.Second,
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: false,
			},
		},
	}

	gateway := &Gateway{
		config:            cfg,
		middlewareManager: middleware.NewMiddlewareManager(),
		loadBalancers:     make(map[string]loadbalancer.LoadBalancer),
		cache:             cacheInstance,
		tokenService:      tokenService,
		userService:       userService,
		rateLimiter:       rateLimiter,
		healthChecker:     healthChecker,
		systemChecker:     systemChecker,
		metricsCollector:  metricsCollector,
		httpClient:        httpClient,
	}

	// 初始化中间件
	gateway.initializeMiddlewares()

	// 初始化路由
	gateway.initializeRoutes()

	// 初始化负载均衡器
	gateway.initializeLoadBalancers()

	// 添加系统依赖检查
	gateway.addSystemDependencies()

	return gateway, nil
}

// initializeMiddlewares 初始化中间件
func (g *Gateway) initializeMiddlewares() {
	// 注册基础中间件
	g.middlewareManager.Register(middleware.NewLoggingMiddleware())
	g.middlewareManager.Register(middleware.NewSecurityMiddleware())
	g.middlewareManager.Register(middleware.NewCORSMiddleware(
		[]string{"*"}, // 允许的源
		[]string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		[]string{"Origin", "Content-Type", "Accept", "Authorization"},
		[]string{},
		true,
		24*time.Hour,
	))
	g.middlewareManager.Register(middleware.NewCompressionMiddleware())
	g.middlewareManager.Register(middleware.NewAuthMiddleware(
		g.tokenService,
		g.userService,
		[]string{"/health", "/metrics", "/auth"},
	))
	g.middlewareManager.Register(middleware.NewRateLimitMiddleware(g.rateLimiter, 100))
	g.middlewareManager.Register(middleware.NewCacheMiddleware(g.cache, 5*time.Minute))
}

// initializeRoutes 初始化路由
func (g *Gateway) initializeRoutes() {
	g.router = gin.New()

	// 基础中间件
	g.router.Use(g.metricsMiddleware())
	g.router.Use(g.middlewareManager.Get("logging").Handle())
	g.router.Use(g.middlewareManager.Get("security").Handle())
	g.router.Use(g.middlewareManager.Get("cors").Handle())
	g.router.Use(gin.Recovery())

	// 健康检查端点
	g.router.GET("/health", g.healthCheckHandler)
	g.router.GET("/health/detailed", g.detailedHealthCheckHandler)

	// 认证端点
	authGroup := g.router.Group("/auth")
	{
		authGroup.POST("/login", g.loginHandler)
		authGroup.POST("/refresh", g.refreshTokenHandler)
		authGroup.POST("/logout", g.logoutHandler)
	}

	// 管理端点
	adminGroup := g.router.Group("/admin")
	adminGroup.Use(g.middlewareManager.Get("auth").Handle())
	{
		adminGroup.GET("/status", g.statusHandler)
		adminGroup.GET("/backends", g.backendsHandler)
		adminGroup.POST("/backends/health", g.updateBackendHealthHandler)
	}

	// 代理路由
	g.setupProxyRoutes()
}

// setupProxyRoutes 设置代理路由
func (g *Gateway) setupProxyRoutes() {
	for _, route := range g.config.Routes {
		routeGroup := g.router.Group(route.Path)

		// 应用路由特定的中间件
		if route.AuthRequired {
			routeGroup.Use(g.middlewareManager.Get("auth").Handle())
		}

		if route.RateLimit > 0 {
			routeGroup.Use(g.routeRateLimitMiddleware(route.RateLimit))
		}

		if route.CacheEnabled {
			routeGroup.Use(g.middlewareManager.Get("cache").Handle())
		}

		// 应用自定义中间件
		g.middlewareManager.Apply(routeGroup, route.Middleware)

		// 注册路由处理器
		routeGroup.Any("/*path", g.proxyHandler(route))
	}
}

// initializeLoadBalancers 初始化负载均衡器
func (g *Gateway) initializeLoadBalancers() {
	for _, route := range g.config.Routes {
		lb := loadbalancer.CreateLoadBalancer(route.LoadBalancer)

		for _, backendCfg := range route.Backends {
			backend, err := loadbalancer.NewBackend(backendCfg)
			if err != nil {
				logger.Errorf("创建后端服务失败 %s: %v", backendCfg.URL, err)
				continue
			}

			lb.AddBackend(backend)
			
			// 添加到健康检查器
			if backendCfg.HealthCheck.Enabled {
				g.healthChecker.AddBackend(route.Path, backend, lb)
			}

			logger.Infof("添加后端服务: %s -> %s", route.Path, backendCfg.URL)
		}

		g.loadBalancers[route.Path] = lb
	}
}

// addSystemDependencies 添加系统依赖检查
func (g *Gateway) addSystemDependencies() {
	// 添加Redis检查
	if g.config.Redis.Addr != "" {
		g.systemChecker.AddDependency(healthcheck.NewRedisChecker("redis"))
	}

	// 可以添加更多依赖检查，如数据库等
}

// proxyHandler 代理处理器
func (g *Gateway) proxyHandler(route config.RouteConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		// 获取负载均衡器
		lb, exists := g.loadBalancers[route.Path]
		if !exists {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "负载均衡器未找到"})
			return
		}

		// 选择后端服务
		backend, err := lb.NextBackend(c.ClientIP())
		if err != nil {
			g.metricsCollector.GetMetrics().RecordBackendRequest(
				"unavailable", c.Request.Method, http.StatusServiceUnavailable, time.Since(start))
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "后端服务不可用"})
			return
		}

		// 增加连接计数
		backend.AddConnection()
		defer backend.RemoveConnection()

		// 创建反向代理
		proxy := g.createReverseProxy(backend, route)
		
		// 记录请求大小
		var requestSize int64
		if c.Request.Body != nil {
			if body, err := io.ReadAll(c.Request.Body); err == nil {
				requestSize = int64(len(body))
				c.Request.Body = io.NopCloser(bytes.NewReader(body))
			}
		}

		// 代理请求
		proxy.ServeHTTP(c.Writer, c.Request)

		// 记录指标
		duration := time.Since(start)
		g.metricsCollector.GetMetrics().RecordBackendRequest(
			backend.URL.String(), c.Request.Method, c.Writer.Status(), duration)
		
		// 记录完整的请求指标
		g.metricsCollector.Record(metrics.RequestMetrics{
			Method:       c.Request.Method,
			Path:         c.Request.URL.Path,
			StatusCode:   c.Writer.Status(),
			Duration:     duration,
			RequestSize:  requestSize,
			ResponseSize: int64(c.Writer.Size()),
			Backend:      backend.URL.String(),
		})
	}
}

// createReverseProxy 创建反向代理
func (g *Gateway) createReverseProxy(backend *loadbalancer.Backend, route config.RouteConfig) *httputil.ReverseProxy {
	proxy := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL.Scheme = backend.URL.Scheme
			req.URL.Host = backend.URL.Host
			req.Host = backend.URL.Host

			// 移除网关路径前缀
			if strings.HasPrefix(req.URL.Path, route.Path) {
				req.URL.Path = req.URL.Path[len(route.Path):]
				if req.URL.Path == "" {
					req.URL.Path = "/"
				}
			}

			// 添加追踪头
			req.Header.Set("X-Forwarded-For", req.RemoteAddr)
			req.Header.Set("X-Forwarded-Proto", req.URL.Scheme)
			req.Header.Set("X-Gateway-Request-ID", generateRequestID())
		},
		
		Transport: &http.Transport{
			MaxIdleConns:       100,
			IdleConnTimeout:    90 * time.Second,
			DisableCompression: false,
		},

		ModifyResponse: func(resp *http.Response) error {
			// 添加响应头
			resp.Header.Set("X-Gateway", "api-gateway")
			resp.Header.Set("X-Backend", backend.URL.String())
			return nil
		},

		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			logger.Errorf("代理请求失败: %v", err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadGateway)
			w.Write([]byte(`{"error": "后端服务错误"}`))
		},
	}

	return proxy
}

// metricsMiddleware 指标中间件
func (g *Gateway) metricsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		
		// 增加活跃连接数
		g.metricsCollector.GetMetrics().IncrementActiveConnections()
		defer g.metricsCollector.GetMetrics().DecrementActiveConnections()

		c.Next()

		// 记录HTTP指标
		duration := time.Since(start)
		g.metricsCollector.GetMetrics().RecordHTTPRequest(
			c.Request.Method,
			c.Request.URL.Path,
			c.Writer.Status(),
			duration,
			c.Request.ContentLength,
			int64(c.Writer.Size()),
		)
	}
}

// routeRateLimitMiddleware 路由级别的速率限制中间件
func (g *Gateway) routeRateLimitMiddleware(limit int) gin.HandlerFunc {
	return func(c *gin.Context) {
		clientIP := c.ClientIP()
		userID, _ := c.Get("user_id")
		path := c.Request.URL.Path

		key := ratelimit.GenerateRateLimitKey(clientIP, fmt.Sprintf("%v", userID), path)

		allowed, err := g.rateLimiter.Allow(c.Request.Context(), key, limit)
		if err != nil {
			logger.Errorf("速率限制检查失败: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "内部服务器错误"})
			c.Abort()
			return
		}

		g.metricsCollector.GetMetrics().RecordRateLimit(allowed)

		if !allowed {
			c.Header("X-RateLimit-Limit", strconv.Itoa(limit))
			c.Header("X-RateLimit-Remaining", "0")
			c.Header("Retry-After", "60")
			
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error":   "请求过于频繁",
				"message": "请稍后再试",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// healthCheckHandler 健康检查处理器
func (g *Gateway) healthCheckHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":    "healthy",
		"timestamp": time.Now().UTC(),
		"version":   "1.0.0",
	})
}

// detailedHealthCheckHandler 详细健康检查处理器
func (g *Gateway) detailedHealthCheckHandler(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	healthStatus := g.systemChecker.CheckHealth(ctx)
	backendStatus := g.healthChecker.GetAllStatus()

	result := map[string]interface{}{
		"system":   healthStatus,
		"backends": backendStatus,
	}

	statusCode := http.StatusOK
	if healthStatus["status"] == "unhealthy" {
		statusCode = http.StatusServiceUnavailable
	}

	c.JSON(statusCode, result)
}

// loginHandler 登录处理器
func (g *Gateway) loginHandler(c *gin.Context) {
	var req struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求参数"})
		return
	}

	// 验证用户凭据
	user, err := g.userService.ValidateCredentials(req.Username, req.Password)
	if err != nil {
		g.metricsCollector.GetMetrics().RecordAuth(false)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "用户名或密码错误"})
		return
	}

	// 生成访问令牌
	accessToken, err := g.tokenService.GenerateToken(user.ID, user.Username, user.Email, user.Roles)
	if err != nil {
		logger.Errorf("生成访问令牌失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "内部服务器错误"})
		return
	}

	// 生成刷新令牌
	refreshToken, err := g.tokenService.GenerateRefreshToken(user.ID)
	if err != nil {
		logger.Errorf("生成刷新令牌失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "内部服务器错误"})
		return
	}

	g.metricsCollector.GetMetrics().RecordAuth(true)

	c.JSON(http.StatusOK, gin.H{
		"access_token":  accessToken,
		"refresh_token": refreshToken,
		"token_type":    "Bearer",
		"expires_in":    int(g.config.Auth.TokenExpiry.Seconds()),
		"user": gin.H{
			"id":       user.ID,
			"username": user.Username,
			"email":    user.Email,
			"roles":    user.Roles,
		},
	})
}

// refreshTokenHandler 刷新令牌处理器
func (g *Gateway) refreshTokenHandler(c *gin.Context) {
	var req struct {
		RefreshToken string `json:"refresh_token" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求参数"})
		return
	}

	// 刷新令牌
	newAccessToken, err := g.tokenService.RefreshToken(req.RefreshToken)
	if err != nil {
		g.metricsCollector.GetMetrics().RecordTokenValidation("invalid")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "无效的刷新令牌"})
		return
	}

	g.metricsCollector.GetMetrics().RecordTokenValidation("valid")

	c.JSON(http.StatusOK, gin.H{
		"access_token": newAccessToken,
		"token_type":   "Bearer",
		"expires_in":   int(g.config.Auth.TokenExpiry.Seconds()),
	})
}

// logoutHandler 登出处理器
func (g *Gateway) logoutHandler(c *gin.Context) {
	// 在实际实现中，这里应该将令牌加入黑名单
	c.JSON(http.StatusOK, gin.H{"message": "登出成功"})
}

// statusHandler 状态处理器
func (g *Gateway) statusHandler(c *gin.Context) {
	status := map[string]interface{}{
		"uptime":        time.Since(time.Now()).String(),
		"load_balancers": make(map[string]interface{}),
		"cache_stats":   "enabled",
	}

	for path, lb := range g.loadBalancers {
		backends := lb.GetBackends()
		backendInfo := make([]map[string]interface{}, len(backends))
		
		for i, backend := range backends {
			backendInfo[i] = map[string]interface{}{
				"url":         backend.URL.String(),
				"healthy":     backend.IsHealthy(),
				"connections": backend.GetCurrentConnections(),
				"weight":      backend.Weight,
			}
		}
		
		status["load_balancers"].(map[string]interface{})[path] = backendInfo
	}

	c.JSON(http.StatusOK, status)
}

// backendsHandler 后端服务处理器
func (g *Gateway) backendsHandler(c *gin.Context) {
	backends := make(map[string][]map[string]interface{})

	for path, lb := range g.loadBalancers {
		backendList := lb.GetBackends()
		backends[path] = make([]map[string]interface{}, len(backendList))
		
		for i, backend := range backendList {
			backends[path][i] = map[string]interface{}{
				"url":         backend.URL.String(),
				"healthy":     backend.IsHealthy(),
				"connections": backend.GetCurrentConnections(),
				"weight":      backend.Weight,
				"last_check":  backend.LastCheck,
			}
		}
	}

	c.JSON(http.StatusOK, backends)
}

// updateBackendHealthHandler 更新后端健康状态处理器
func (g *Gateway) updateBackendHealthHandler(c *gin.Context) {
	var req struct {
		Backend string `json:"backend" binding:"required"`
		Healthy bool   `json:"healthy"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求参数"})
		return
	}

	// 更新所有负载均衡器中的后端状态
	updated := false
	for _, lb := range g.loadBalancers {
		lb.UpdateBackendHealth(req.Backend, req.Healthy)
		updated = true
	}

	if !updated {
		c.JSON(http.StatusNotFound, gin.H{"error": "后端服务未找到"})
		return
	}

	// 更新指标
	g.metricsCollector.GetMetrics().UpdateBackendHealth(req.Backend, req.Healthy)

	c.JSON(http.StatusOK, gin.H{"message": "后端状态更新成功"})
}

// Start 启动网关
func (g *Gateway) Start() error {
	// 启动健康检查器
	go g.healthChecker.Start(context.Background())

	// 定期更新系统指标
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		
		for range ticker.C {
			g.metricsCollector.UpdateSystemMetrics()
		}
	}()

	// 创建HTTP服务器
	g.server = &http.Server{
		Addr:           fmt.Sprintf("%s:%d", g.config.Server.Host, g.config.Server.Port),
		Handler:        g.router,
		ReadTimeout:    g.config.Server.ReadTimeout,
		WriteTimeout:   g.config.Server.WriteTimeout,
		IdleTimeout:    g.config.Server.IdleTimeout,
		MaxHeaderBytes: 1 << 20, // 1MB
	}

	logger.Infof("API网关启动在 %s", g.server.Addr)

	// 启动HTTPS或HTTP服务器
	if g.config.Server.TLS.Enabled {
		return g.server.ListenAndServeTLS(g.config.Server.TLS.CertFile, g.config.Server.TLS.KeyFile)
	}
	
	return g.server.ListenAndServe()
}

// Stop 停止网关
func (g *Gateway) Stop(ctx context.Context) error {
	logger.Info("正在停止API网关...")

	// 停止健康检查器
	g.healthChecker.Stop()

	// 关闭缓存连接
	if err := g.cache.Close(); err != nil {
		logger.Errorf("关闭缓存连接失败: %v", err)
	}

	// 优雅关闭HTTP服务器
	if g.server != nil {
		return g.server.Shutdown(ctx)
	}

	return nil
}

// generateRequestID 生成请求ID
func generateRequestID() string {
	return fmt.Sprintf("req_%d", time.Now().UnixNano())
}

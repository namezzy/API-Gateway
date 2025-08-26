package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

var (
	configFile = flag.String("config", "configs/config.yaml", "配置文件路径")
	port       = flag.Int("port", 8080, "服务端口")
	version    = flag.Bool("version", false, "显示版本信息")
)

const (
	appVersion = "1.0.0"
	appName    = "API Gateway (Simplified)"
)

// Backend 后端服务
type Backend struct {
	URL         *url.URL
	Weight      int
	Healthy     bool
	Connections int64
	mutex       sync.RWMutex
}

// LoadBalancer 负载均衡器
type LoadBalancer struct {
	backends []*Backend
	current  uint64
	mutex    sync.RWMutex
}

// Gateway 网关结构
type Gateway struct {
	loadBalancer *LoadBalancer
	rateLimiter  *RateLimiter
	server       *http.Server
	metrics      *Metrics
}

// RateLimiter 简单的速率限制器
type RateLimiter struct {
	requests map[string][]time.Time
	limit    int
	window   time.Duration
	mutex    sync.RWMutex
}

// Metrics 简单的指标收集器
type Metrics struct {
	totalRequests  int64
	totalErrors    int64
	responseTime   int64
	activeRequests int64
	mutex          sync.RWMutex
}

// NewBackend 创建后端服务
func NewBackend(urlStr string, weight int) (*Backend, error) {
	u, err := url.Parse(urlStr)
	if err != nil {
		return nil, err
	}
	return &Backend{
		URL:     u,
		Weight:  weight,
		Healthy: true,
	}, nil
}

// IsHealthy 检查后端是否健康
func (b *Backend) IsHealthy() bool {
	b.mutex.RLock()
	defer b.mutex.RUnlock()
	return b.Healthy
}

// SetHealthy 设置后端健康状态
func (b *Backend) SetHealthy(healthy bool) {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	b.Healthy = healthy
}

// AddConnection 增加连接计数
func (b *Backend) AddConnection() {
	atomic.AddInt64(&b.Connections, 1)
}

// RemoveConnection 减少连接计数
func (b *Backend) RemoveConnection() {
	atomic.AddInt64(&b.Connections, -1)
}

// NewLoadBalancer 创建负载均衡器
func NewLoadBalancer() *LoadBalancer {
	return &LoadBalancer{
		backends: make([]*Backend, 0),
	}
}

// AddBackend 添加后端服务
func (lb *LoadBalancer) AddBackend(backend *Backend) {
	lb.mutex.Lock()
	defer lb.mutex.Unlock()
	lb.backends = append(lb.backends, backend)
}

// NextBackend 获取下一个后端服务（轮询）
func (lb *LoadBalancer) NextBackend() (*Backend, error) {
	lb.mutex.RLock()
	defer lb.mutex.RUnlock()

	if len(lb.backends) == 0 {
		return nil, fmt.Errorf("没有可用的后端服务")
	}

	// 过滤健康的后端
	healthyBackends := make([]*Backend, 0)
	for _, backend := range lb.backends {
		if backend.IsHealthy() {
			healthyBackends = append(healthyBackends, backend)
		}
	}

	if len(healthyBackends) == 0 {
		return nil, fmt.Errorf("没有健康的后端服务")
	}

	// 轮询选择
	next := atomic.AddUint64(&lb.current, 1)
	return healthyBackends[(next-1)%uint64(len(healthyBackends))], nil
}

// GetBackends 获取所有后端服务
func (lb *LoadBalancer) GetBackends() []*Backend {
	lb.mutex.RLock()
	defer lb.mutex.RUnlock()
	result := make([]*Backend, len(lb.backends))
	copy(result, lb.backends)
	return result
}

// NewRateLimiter 创建速率限制器
func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	return &RateLimiter{
		requests: make(map[string][]time.Time),
		limit:    limit,
		window:   window,
	}
}

// Allow 检查是否允许请求
func (rl *RateLimiter) Allow(clientIP string) bool {
	rl.mutex.Lock()
	defer rl.mutex.Unlock()

	now := time.Now()
	windowStart := now.Add(-rl.window)

	// 清理过期的请求记录
	if requests, exists := rl.requests[clientIP]; exists {
		validRequests := make([]time.Time, 0)
		for _, reqTime := range requests {
			if reqTime.After(windowStart) {
				validRequests = append(validRequests, reqTime)
			}
		}
		rl.requests[clientIP] = validRequests
	}

	// 检查是否超出限制
	if len(rl.requests[clientIP]) >= rl.limit {
		return false
	}

	// 记录当前请求
	rl.requests[clientIP] = append(rl.requests[clientIP], now)
	return true
}

// NewMetrics 创建指标收集器
func NewMetrics() *Metrics {
	return &Metrics{}
}

// RecordRequest 记录请求
func (m *Metrics) RecordRequest(duration time.Duration, isError bool) {
	atomic.AddInt64(&m.totalRequests, 1)
	if isError {
		atomic.AddInt64(&m.totalErrors, 1)
	}
	atomic.StoreInt64(&m.responseTime, duration.Milliseconds())
}

// IncrementActive 增加活跃请求
func (m *Metrics) IncrementActive() {
	atomic.AddInt64(&m.activeRequests, 1)
}

// DecrementActive 减少活跃请求
func (m *Metrics) DecrementActive() {
	atomic.AddInt64(&m.activeRequests, -1)
}

// GetStats 获取统计信息
func (m *Metrics) GetStats() map[string]interface{} {
	return map[string]interface{}{
		"total_requests":   atomic.LoadInt64(&m.totalRequests),
		"total_errors":     atomic.LoadInt64(&m.totalErrors),
		"active_requests":  atomic.LoadInt64(&m.activeRequests),
		"avg_response_ms":  atomic.LoadInt64(&m.responseTime),
	}
}

// NewGateway 创建网关实例
func NewGateway() *Gateway {
	lb := NewLoadBalancer()
	rateLimiter := NewRateLimiter(100, time.Minute) // 每分钟100次请求
	metrics := NewMetrics()

	// 添加一些示例后端服务
	backends := []string{
		"http://httpbin.org",
		"http://jsonplaceholder.typicode.com",
	}

	for _, backendURL := range backends {
		if backend, err := NewBackend(backendURL, 1); err == nil {
			lb.AddBackend(backend)
			log.Printf("添加后端服务: %s", backendURL)
		}
	}

	return &Gateway{
		loadBalancer: lb,
		rateLimiter:  rateLimiter,
		metrics:      metrics,
	}
}

// createProxy 创建反向代理
func (g *Gateway) createProxy(backend *Backend) *httputil.ReverseProxy {
	return &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL.Scheme = backend.URL.Scheme
			req.URL.Host = backend.URL.Host
			req.Host = backend.URL.Host
			req.Header.Set("X-Forwarded-For", req.RemoteAddr)
			req.Header.Set("X-Gateway", "simple-api-gateway")
		},
		ModifyResponse: func(resp *http.Response) error {
			resp.Header.Set("X-Gateway", "simple-api-gateway")
			resp.Header.Set("X-Backend", backend.URL.String())
			return nil
		},
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			log.Printf("代理错误: %v", err)
			backend.SetHealthy(false)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadGateway)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "后端服务不可用",
			})
		},
	}
}

// proxyHandler 代理处理器
func (g *Gateway) proxyHandler(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	g.metrics.IncrementActive()
	defer g.metrics.DecrementActive()

	// 速率限制检查
	clientIP := strings.Split(r.RemoteAddr, ":")[0]
	if !g.rateLimiter.Allow(clientIP) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusTooManyRequests)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "请求过于频繁，请稍后再试",
		})
		g.metrics.RecordRequest(time.Since(start), true)
		return
	}

	// 选择后端服务
	backend, err := g.loadBalancer.NextBackend()
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{
			"error": err.Error(),
		})
		g.metrics.RecordRequest(time.Since(start), true)
		return
	}

	// 增加连接计数
	backend.AddConnection()
	defer backend.RemoveConnection()

	// 创建代理并处理请求
	proxy := g.createProxy(backend)
	proxy.ServeHTTP(w, r)

	// 记录指标
	duration := time.Since(start)
	g.metrics.RecordRequest(duration, false)

	log.Printf("请求 %s %s -> %s (耗时: %v)", r.Method, r.URL.Path, backend.URL.String(), duration)
}

// healthHandler 健康检查处理器
func (g *Gateway) healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	
	health := map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now().UTC(),
		"version":   appVersion,
		"backends":  make([]map[string]interface{}, 0),
	}

	backends := g.loadBalancer.GetBackends()
	for _, backend := range backends {
		backendInfo := map[string]interface{}{
			"url":         backend.URL.String(),
			"healthy":     backend.IsHealthy(),
			"connections": atomic.LoadInt64(&backend.Connections),
		}
		health["backends"] = append(health["backends"].([]map[string]interface{}), backendInfo)
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(health)
}

// metricsHandler 指标处理器
func (g *Gateway) metricsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(g.metrics.GetStats())
}

// statusHandler 状态处理器
func (g *Gateway) statusHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	
	status := map[string]interface{}{
		"name":       appName,
		"version":    appVersion,
		"uptime":     time.Since(time.Now()).String(),
		"metrics":    g.metrics.GetStats(),
		"backends":   make([]map[string]interface{}, 0),
	}

	backends := g.loadBalancer.GetBackends()
	for _, backend := range backends {
		backendInfo := map[string]interface{}{
			"url":         backend.URL.String(),
			"healthy":     backend.IsHealthy(),
			"connections": atomic.LoadInt64(&backend.Connections),
			"weight":      backend.Weight,
		}
		status["backends"] = append(status["backends"].([]map[string]interface{}), backendInfo)
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(status)
}

// setupRoutes 设置路由
func (g *Gateway) setupRoutes() *http.ServeMux {
	mux := http.NewServeMux()

	// 系统端点
	mux.HandleFunc("/health", g.healthHandler)
	mux.HandleFunc("/metrics", g.metricsHandler)
	mux.HandleFunc("/status", g.statusHandler)

	// API代理端点
	mux.HandleFunc("/api/", g.proxyHandler)
	mux.HandleFunc("/", g.proxyHandler)

	return mux
}

// Start 启动网关
func (g *Gateway) Start(addr string) error {
	mux := g.setupRoutes()
	
	g.server = &http.Server{
		Addr:           addr,
		Handler:        mux,
		ReadTimeout:    30 * time.Second,
		WriteTimeout:   30 * time.Second,
		IdleTimeout:    60 * time.Second,
		MaxHeaderBytes: 1 << 20, // 1MB
	}

	log.Printf("API网关启动在 %s", addr)
	log.Printf("健康检查: http://%s/health", strings.TrimPrefix(addr, ":"))
	log.Printf("指标端点: http://%s/metrics", strings.TrimPrefix(addr, ":"))
	log.Printf("状态端点: http://%s/status", strings.TrimPrefix(addr, ":"))

	return g.server.ListenAndServe()
}

// Stop 停止网关
func (g *Gateway) Stop(ctx context.Context) error {
	if g.server != nil {
		return g.server.Shutdown(ctx)
	}
	return nil
}

// healthCheck 简单的后端健康检查
func (g *Gateway) healthCheck() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		backends := g.loadBalancer.GetBackends()
		for _, backend := range backends {
			go func(b *Backend) {
				client := &http.Client{Timeout: 5 * time.Second}
				resp, err := client.Get(b.URL.String() + "/health")
				if err != nil || resp.StatusCode != http.StatusOK {
					b.SetHealthy(false)
					log.Printf("后端服务不健康: %s", b.URL.String())
				} else {
					b.SetHealthy(true)
				}
				if resp != nil {
					resp.Body.Close()
				}
			}(backend)
		}
	}
}

func main() {
	flag.Parse()

	if *version {
		fmt.Printf("%s v%s\n", appName, appVersion)
		os.Exit(0)
	}

	// 创建网关实例
	gateway := NewGateway()

	// 启动健康检查
	go gateway.healthCheck()

	// 启动网关
	addr := ":" + strconv.Itoa(*port)
	
	// 在goroutine中启动服务器
	serverErr := make(chan error, 1)
	go func() {
		if err := gateway.Start(addr); err != nil && err != http.ErrServerClosed {
			serverErr <- fmt.Errorf("网关启动失败: %w", err)
		}
	}()

	// 等待中断信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// 等待退出信号或启动错误
	select {
	case err := <-serverErr:
		log.Printf("服务器错误: %v", err)
	case sig := <-sigChan:
		log.Printf("接收到信号: %s，开始优雅关闭", sig)
	}

	// 优雅关闭
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	log.Println("正在关闭服务...")
	if err := gateway.Stop(shutdownCtx); err != nil {
		log.Printf("关闭网关失败: %v", err)
	}

	log.Println("服务已停止")
}

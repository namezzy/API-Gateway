package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
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

// 配置结构
type Config struct {
	Server struct {
		Port int    `json:"port"`
		Host string `json:"host"`
	} `json:"server"`
	Routes []Route `json:"routes"`
}

type Route struct {
	Path     string    `json:"path"`
	Method   string    `json:"method"`
	Backends []Backend `json:"backends"`
}

type Backend struct {
	URL    string `json:"url"`
	Weight int    `json:"weight"`
}

// 负载均衡器
type LoadBalancer struct {
	backends []*BackendServer
	current  uint64
	mutex    sync.RWMutex
}

type BackendServer struct {
	URL     *url.URL
	Weight  int
	Healthy bool
	mutex   sync.RWMutex
}

func (lb *LoadBalancer) AddBackend(backend *BackendServer) {
	lb.mutex.Lock()
	defer lb.mutex.Unlock()
	lb.backends = append(lb.backends, backend)
}

func (lb *LoadBalancer) NextBackend() (*BackendServer, error) {
	lb.mutex.RLock()
	defer lb.mutex.RUnlock()

	if len(lb.backends) == 0 {
		return nil, fmt.Errorf("没有可用的后端服务")
	}

	// 过滤健康的后端
	var healthyBackends []*BackendServer
	for _, backend := range lb.backends {
		if backend.IsHealthy() {
			healthyBackends = append(healthyBackends, backend)
		}
	}

	if len(healthyBackends) == 0 {
		return nil, fmt.Errorf("没有健康的后端服务")
	}

	// 轮询算法
	next := atomic.AddUint64(&lb.current, 1)
	return healthyBackends[(next-1)%uint64(len(healthyBackends))], nil
}

func (bs *BackendServer) IsHealthy() bool {
	bs.mutex.RLock()
	defer bs.mutex.RUnlock()
	return bs.Healthy
}

func (bs *BackendServer) SetHealthy(healthy bool) {
	bs.mutex.Lock()
	defer bs.mutex.Unlock()
	bs.Healthy = healthy
}

// API网关
type Gateway struct {
	config        *Config
	loadBalancers map[string]*LoadBalancer
	server        *http.Server
	metrics       *Metrics
}

type Metrics struct {
	requestCount   int64
	errorCount     int64
	totalLatency   int64
	requestLatency time.Duration
	mutex          sync.RWMutex
}

func (m *Metrics) IncrementRequests() {
	atomic.AddInt64(&m.requestCount, 1)
}

func (m *Metrics) IncrementErrors() {
	atomic.AddInt64(&m.errorCount, 1)
}

func (m *Metrics) RecordLatency(latency time.Duration) {
	atomic.AddInt64(&m.totalLatency, int64(latency))
}

func (m *Metrics) GetStats() map[string]interface{} {
	return map[string]interface{}{
		"request_count": atomic.LoadInt64(&m.requestCount),
		"error_count":   atomic.LoadInt64(&m.errorCount),
		"avg_latency":   time.Duration(atomic.LoadInt64(&m.totalLatency) / max(atomic.LoadInt64(&m.requestCount), 1)),
	}
}

func NewGateway(config *Config) *Gateway {
	gw := &Gateway{
		config:        config,
		loadBalancers: make(map[string]*LoadBalancer),
		metrics:       &Metrics{},
	}

	// 初始化负载均衡器
	for _, route := range config.Routes {
		lb := &LoadBalancer{}
		for _, backendCfg := range route.Backends {
			u, err := url.Parse(backendCfg.URL)
			if err != nil {
				log.Printf("解析后端URL失败 %s: %v", backendCfg.URL, err)
				continue
			}
			backend := &BackendServer{
				URL:     u,
				Weight:  backendCfg.Weight,
				Healthy: true,
			}
			lb.AddBackend(backend)
		}
		gw.loadBalancers[route.Path] = lb
	}

	return gw
}

func (gw *Gateway) setupRoutes() {
	mux := http.NewServeMux()

	// 健康检查
	mux.HandleFunc("/health", gw.healthHandler)
	mux.HandleFunc("/metrics", gw.metricsHandler)
	mux.HandleFunc("/status", gw.statusHandler)

	// 代理路由
	for _, route := range gw.config.Routes {
		path := route.Path
		if !strings.HasSuffix(path, "/") {
			path += "/"
		}
		mux.HandleFunc(path, gw.proxyHandler(route))
	}

	// 添加中间件
	handler := gw.loggingMiddleware(gw.corsMiddleware(mux))

	gw.server = &http.Server{
		Addr:         fmt.Sprintf("%s:%d", gw.config.Server.Host, gw.config.Server.Port),
		Handler:      handler,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
}

func (gw *Gateway) proxyHandler(route Route) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		gw.metrics.IncrementRequests()

		// 获取负载均衡器
		lb, exists := gw.loadBalancers[route.Path]
		if !exists {
			gw.metrics.IncrementErrors()
			http.Error(w, "负载均衡器未找到", http.StatusInternalServerError)
			return
		}

		// 选择后端服务
		backend, err := lb.NextBackend()
		if err != nil {
			gw.metrics.IncrementErrors()
			http.Error(w, "后端服务不可用: "+err.Error(), http.StatusServiceUnavailable)
			return
		}

		// 创建反向代理
		proxy := &httputil.ReverseProxy{
			Director: func(req *http.Request) {
				req.URL.Scheme = backend.URL.Scheme
				req.URL.Host = backend.URL.Host
				
				// 移除路由前缀
				if strings.HasPrefix(req.URL.Path, route.Path) {
					req.URL.Path = strings.TrimPrefix(req.URL.Path, route.Path)
					if req.URL.Path == "" {
						req.URL.Path = "/"
					}
				}

				req.Header.Set("X-Forwarded-For", req.RemoteAddr)
				req.Header.Set("X-Gateway", "api-gateway")
			},
			ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
				log.Printf("代理错误: %v", err)
				gw.metrics.IncrementErrors()
				backend.SetHealthy(false)
				http.Error(w, "后端服务错误", http.StatusBadGateway)
			},
		}

		// 代理请求
		proxy.ServeHTTP(w, r)

		// 记录延迟
		latency := time.Since(start)
		gw.metrics.RecordLatency(latency)
	}
}

func (gw *Gateway) healthHandler(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now().UTC(),
		"version":   "1.0.0",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (gw *Gateway) metricsHandler(w http.ResponseWriter, r *http.Request) {
	stats := gw.metrics.GetStats()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

func (gw *Gateway) statusHandler(w http.ResponseWriter, r *http.Request) {
	status := map[string]interface{}{
		"load_balancers": make(map[string]interface{}),
		"metrics":        gw.metrics.GetStats(),
	}

	for path, lb := range gw.loadBalancers {
		lb.mutex.RLock()
		backends := make([]map[string]interface{}, len(lb.backends))
		for i, backend := range lb.backends {
			backends[i] = map[string]interface{}{
				"url":     backend.URL.String(),
				"healthy": backend.IsHealthy(),
				"weight":  backend.Weight,
			}
		}
		lb.mutex.RUnlock()
		status["load_balancers"].(map[string]interface{})[path] = backends
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

// 中间件
func (gw *Gateway) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		
		// 包装ResponseWriter来获取状态码
		wrapper := &responseWrapper{ResponseWriter: w, statusCode: 200}
		
		next.ServeHTTP(wrapper, r)
		
		log.Printf("%s %s %d %v %s",
			r.Method,
			r.URL.Path,
			wrapper.statusCode,
			time.Since(start),
			r.RemoteAddr,
		)
	})
}

func (gw *Gateway) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Origin, Content-Type, Accept, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

type responseWrapper struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWrapper) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// 健康检查器
func (gw *Gateway) startHealthChecker() {
	ticker := time.NewTicker(30 * time.Second)
	go func() {
		defer ticker.Stop()
		for range ticker.C {
			gw.performHealthChecks()
		}
	}()
}

func (gw *Gateway) performHealthChecks() {
	for _, lb := range gw.loadBalancers {
		lb.mutex.RLock()
		backends := make([]*BackendServer, len(lb.backends))
		copy(backends, lb.backends)
		lb.mutex.RUnlock()

		for _, backend := range backends {
			go gw.checkBackendHealth(backend)
		}
	}
}

func (gw *Gateway) checkBackendHealth(backend *BackendServer) {
	healthURL := backend.URL.String() + "/health"
	
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", healthURL, nil)
	if err != nil {
		backend.SetHealthy(false)
		return
	}

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		backend.SetHealthy(false)
		return
	}
	defer resp.Body.Close()

	healthy := resp.StatusCode >= 200 && resp.StatusCode < 300
	backend.SetHealthy(healthy)
	
	if !healthy {
		log.Printf("后端服务不健康: %s (状态码: %d)", backend.URL.String(), resp.StatusCode)
	}
}

func (gw *Gateway) Start() error {
	gw.setupRoutes()
	gw.startHealthChecker()

	log.Printf("API网关启动在 %s", gw.server.Addr)
	return gw.server.ListenAndServe()
}

func (gw *Gateway) Stop(ctx context.Context) error {
	log.Println("正在停止API网关...")
	if gw.server != nil {
		return gw.server.Shutdown(ctx)
	}
	return nil
}

func loadConfig(configFile string) (*Config, error) {
	// 默认配置
	defaultConfig := &Config{
		Server: struct {
			Port int    `json:"port"`
			Host string `json:"host"`
		}{
			Port: 8080,
			Host: "0.0.0.0",
		},
		Routes: []Route{
			{
				Path:   "/api/v1/users",
				Method: "GET",
				Backends: []Backend{
					{URL: "http://localhost:3001", Weight: 1},
					{URL: "http://localhost:3002", Weight: 1},
				},
			},
			{
				Path:   "/api/v1/orders",
				Method: "GET",
				Backends: []Backend{
					{URL: "http://localhost:3003", Weight: 1},
				},
			},
		},
	}

	if configFile == "" {
		return defaultConfig, nil
	}

	file, err := os.Open(configFile)
	if err != nil {
		log.Printf("无法打开配置文件 %s，使用默认配置: %v", configFile, err)
		return defaultConfig, nil
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %w", err)
	}

	return &config, nil
}

func max(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

func main() {
	var (
		configFile = flag.String("config", "", "配置文件路径")
		port       = flag.Int("port", 8080, "服务端口")
		version    = flag.Bool("version", false, "显示版本信息")
	)
	flag.Parse()

	if *version {
		fmt.Println("API Gateway v1.0.0")
		os.Exit(0)
	}

	// 加载配置
	config, err := loadConfig(*configFile)
	if err != nil {
		log.Fatalf("配置加载失败: %v", err)
	}

	// 覆盖端口配置
	if *port != 8080 {
		config.Server.Port = *port
	}

	// 创建并启动网关
	gateway := NewGateway(config)

	// 优雅关闭
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 启动网关
	serverErr := make(chan error, 1)
	go func() {
		if err := gateway.Start(); err != nil && err != http.ErrServerClosed {
			serverErr <- fmt.Errorf("网关启动失败: %w", err)
		}
	}()

	// 等待中断信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-serverErr:
		log.Printf("服务器错误: %v", err)
	case sig := <-sigChan:
		log.Printf("接收到信号: %s，开始优雅关闭", sig)
	}

	// 优雅关闭
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := gateway.Stop(shutdownCtx); err != nil {
		log.Printf("关闭网关失败: %v", err)
	}

	log.Println("网关已停止")
}

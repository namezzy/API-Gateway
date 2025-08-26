package healthcheck

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"api-gateway/internal/config"
	"api-gateway/internal/loadbalancer"
	"api-gateway/internal/logger"
)

// HealthChecker 健康检查器接口
type HealthChecker interface {
	Start(ctx context.Context)
	Stop()
	CheckBackend(backend *loadbalancer.Backend) bool
	GetStatus(backendURL string) HealthStatus
}

// HealthStatus 健康状态
type HealthStatus struct {
	Healthy      bool      `json:"healthy"`
	LastCheck    time.Time `json:"last_check"`
	ResponseTime int64     `json:"response_time_ms"`
	ErrorMessage string    `json:"error_message,omitempty"`
}

// BackendHealthChecker 后端健康检查器
type BackendHealthChecker struct {
	backends      map[string]*loadbalancer.Backend
	loadBalancers map[string]loadbalancer.LoadBalancer
	status        map[string]HealthStatus
	client        *http.Client
	stopChan      chan struct{}
	mutex         sync.RWMutex
	running       bool
}

// NewBackendHealthChecker 创建后端健康检查器
func NewBackendHealthChecker() *BackendHealthChecker {
	return &BackendHealthChecker{
		backends:      make(map[string]*loadbalancer.Backend),
		loadBalancers: make(map[string]loadbalancer.LoadBalancer),
		status:        make(map[string]HealthStatus),
		client: &http.Client{
			Timeout: 10 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 10,
				IdleConnTimeout:     30 * time.Second,
			},
		},
		stopChan: make(chan struct{}),
	}
}

// AddBackend 添加后端服务
func (hc *BackendHealthChecker) AddBackend(routeName string, backend *loadbalancer.Backend, lb loadbalancer.LoadBalancer) {
	hc.mutex.Lock()
	defer hc.mutex.Unlock()
	
	key := fmt.Sprintf("%s:%s", routeName, backend.URL.String())
	hc.backends[key] = backend
	hc.loadBalancers[key] = lb
	hc.status[key] = HealthStatus{
		Healthy:   true,
		LastCheck: time.Now(),
	}
}

// RemoveBackend 移除后端服务
func (hc *BackendHealthChecker) RemoveBackend(routeName string, backendURL string) {
	hc.mutex.Lock()
	defer hc.mutex.Unlock()
	
	key := fmt.Sprintf("%s:%s", routeName, backendURL)
	delete(hc.backends, key)
	delete(hc.loadBalancers, key)
	delete(hc.status, key)
}

// Start 开始健康检查
func (hc *BackendHealthChecker) Start(ctx context.Context) {
	hc.mutex.Lock()
	if hc.running {
		hc.mutex.Unlock()
		return
	}
	hc.running = true
	hc.mutex.Unlock()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	logger.Info("健康检查器启动")

	for {
		select {
		case <-ctx.Done():
			logger.Info("健康检查器停止：上下文取消")
			return
		case <-hc.stopChan:
			logger.Info("健康检查器停止：接收到停止信号")
			return
		case <-ticker.C:
			hc.performHealthChecks()
		}
	}
}

// Stop 停止健康检查
func (hc *BackendHealthChecker) Stop() {
	hc.mutex.Lock()
	defer hc.mutex.Unlock()
	
	if !hc.running {
		return
	}
	
	hc.running = false
	close(hc.stopChan)
}

// performHealthChecks 执行健康检查
func (hc *BackendHealthChecker) performHealthChecks() {
	hc.mutex.RLock()
	backends := make(map[string]*loadbalancer.Backend)
	loadBalancers := make(map[string]loadbalancer.LoadBalancer)
	
	for key, backend := range hc.backends {
		backends[key] = backend
		loadBalancers[key] = hc.loadBalancers[key]
	}
	hc.mutex.RUnlock()

	var wg sync.WaitGroup
	for key, backend := range backends {
		wg.Add(1)
		go func(k string, b *loadbalancer.Backend, lb loadbalancer.LoadBalancer) {
			defer wg.Done()
			hc.checkSingleBackend(k, b, lb)
		}(key, backend, loadBalancers[key])
	}
	
	wg.Wait()
}

// checkSingleBackend 检查单个后端服务
func (hc *BackendHealthChecker) checkSingleBackend(key string, backend *loadbalancer.Backend, lb loadbalancer.LoadBalancer) {
	start := time.Now()
	healthy := hc.CheckBackend(backend)
	responseTime := time.Since(start).Milliseconds()

	status := HealthStatus{
		Healthy:      healthy,
		LastCheck:    time.Now(),
		ResponseTime: responseTime,
	}

	if !healthy {
		status.ErrorMessage = "健康检查失败"
	}

	hc.mutex.Lock()
	hc.status[key] = status
	hc.mutex.Unlock()

	// 更新负载均衡器中的后端状态
	if backend.IsHealthy() != healthy {
		backend.SetHealthy(healthy)
		lb.UpdateBackendHealth(backend.URL.String(), healthy)
		
		if healthy {
			logger.Infof("后端服务恢复健康: %s", backend.URL.String())
		} else {
			logger.Warnf("后端服务不健康: %s", backend.URL.String())
		}
	}
}

// CheckBackend 检查后端服务健康状态
func (hc *BackendHealthChecker) CheckBackend(backend *loadbalancer.Backend) bool {
	healthCheckURL := backend.URL.String() + "/health"
	
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", healthCheckURL, nil)
	if err != nil {
		logger.Errorf("创建健康检查请求失败 %s: %v", backend.URL.String(), err)
		return false
	}

	resp, err := hc.client.Do(req)
	if err != nil {
		logger.Debugf("健康检查请求失败 %s: %v", backend.URL.String(), err)
		return false
	}
	defer resp.Body.Close()

	// 认为状态码在200-299范围内的响应为健康
	healthy := resp.StatusCode >= 200 && resp.StatusCode < 300
	
	if !healthy {
		logger.Debugf("后端服务返回不健康状态码 %s: %d", backend.URL.String(), resp.StatusCode)
	}

	return healthy
}

// GetStatus 获取后端服务状态
func (hc *BackendHealthChecker) GetStatus(backendURL string) HealthStatus {
	hc.mutex.RLock()
	defer hc.mutex.RUnlock()
	
	for key, status := range hc.status {
		if key == backendURL || (len(key) > len(backendURL) && key[len(key)-len(backendURL):] == backendURL) {
			return status
		}
	}
	
	return HealthStatus{
		Healthy:      false,
		LastCheck:    time.Now(),
		ErrorMessage: "后端服务未找到",
	}
}

// GetAllStatus 获取所有后端服务状态
func (hc *BackendHealthChecker) GetAllStatus() map[string]HealthStatus {
	hc.mutex.RLock()
	defer hc.mutex.RUnlock()
	
	result := make(map[string]HealthStatus)
	for key, status := range hc.status {
		result[key] = status
	}
	
	return result
}

// SystemHealthChecker 系统健康检查器
type SystemHealthChecker struct {
	dependencies map[string]DependencyChecker
	mutex        sync.RWMutex
}

// DependencyChecker 依赖检查器接口
type DependencyChecker interface {
	Check(ctx context.Context) error
	Name() string
}

// NewSystemHealthChecker 创建系统健康检查器
func NewSystemHealthChecker() *SystemHealthChecker {
	return &SystemHealthChecker{
		dependencies: make(map[string]DependencyChecker),
	}
}

// AddDependency 添加依赖检查器
func (shc *SystemHealthChecker) AddDependency(checker DependencyChecker) {
	shc.mutex.Lock()
	defer shc.mutex.Unlock()
	shc.dependencies[checker.Name()] = checker
}

// CheckHealth 检查系统健康状态
func (shc *SystemHealthChecker) CheckHealth(ctx context.Context) map[string]interface{} {
	shc.mutex.RLock()
	dependencies := make(map[string]DependencyChecker)
	for name, checker := range shc.dependencies {
		dependencies[name] = checker
	}
	shc.mutex.RUnlock()

	result := map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now().UTC(),
		"checks":    make(map[string]interface{}),
	}

	allHealthy := true
	checks := result["checks"].(map[string]interface{})

	for name, checker := range dependencies {
		start := time.Now()
		err := checker.Check(ctx)
		duration := time.Since(start)

		check := map[string]interface{}{
			"status":   "healthy",
			"duration": duration.String(),
		}

		if err != nil {
			check["status"] = "unhealthy"
			check["error"] = err.Error()
			allHealthy = false
		}

		checks[name] = check
	}

	if !allHealthy {
		result["status"] = "unhealthy"
	}

	return result
}

// DatabaseChecker 数据库检查器
type DatabaseChecker struct {
	name string
	// 这里应该包含数据库连接
}

// NewDatabaseChecker 创建数据库检查器
func NewDatabaseChecker(name string) *DatabaseChecker {
	return &DatabaseChecker{name: name}
}

// Name 返回检查器名称
func (dc *DatabaseChecker) Name() string {
	return dc.name
}

// Check 检查数据库连接
func (dc *DatabaseChecker) Check(ctx context.Context) error {
	// 简化实现，实际应该执行数据库ping操作
	// 模拟检查延迟
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(10 * time.Millisecond):
		return nil
	}
}

// RedisChecker Redis检查器
type RedisChecker struct {
	name string
	// 这里应该包含Redis连接
}

// NewRedisChecker 创建Redis检查器
func NewRedisChecker(name string) *RedisChecker {
	return &RedisChecker{name: name}
}

// Name 返回检查器名称
func (rc *RedisChecker) Name() string {
	return rc.name
}

// Check 检查Redis连接
func (rc *RedisChecker) Check(ctx context.Context) error {
	// 简化实现，实际应该执行Redis ping操作
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(5 * time.Millisecond):
		return nil
	}
}

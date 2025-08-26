package metrics

import (
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Metrics 指标收集器
type Metrics struct {
	// HTTP请求指标
	RequestsTotal        *prometheus.CounterVec
	RequestDuration      *prometheus.HistogramVec
	RequestSize          *prometheus.HistogramVec
	ResponseSize         *prometheus.HistogramVec
	
	// 后端服务指标
	BackendRequestsTotal    *prometheus.CounterVec
	BackendRequestDuration  *prometheus.HistogramVec
	BackendHealthStatus     *prometheus.GaugeVec
	
	// 速率限制指标
	RateLimitRequestsTotal *prometheus.CounterVec
	
	// 缓存指标
	CacheRequestsTotal *prometheus.CounterVec
	CacheHitRatio      *prometheus.GaugeVec
	
	// 系统指标
	ActiveConnections    prometheus.Gauge
	TotalConnections     prometheus.Counter
	SystemUptime         prometheus.Gauge
	
	// 认证指标
	AuthRequestsTotal    *prometheus.CounterVec
	TokenValidationTotal *prometheus.CounterVec
}

// NewMetrics 创建指标收集器
func NewMetrics() *Metrics {
	return &Metrics{
		// HTTP请求指标
		RequestsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "http_requests_total",
				Help: "HTTP请求总数",
			},
			[]string{"method", "path", "status_code"},
		),
		
		RequestDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "http_request_duration_seconds",
				Help:    "HTTP请求持续时间",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"method", "path"},
		),
		
		RequestSize: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "http_request_size_bytes",
				Help:    "HTTP请求大小",
				Buckets: prometheus.ExponentialBuckets(1024, 2, 10),
			},
			[]string{"method", "path"},
		),
		
		ResponseSize: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "http_response_size_bytes",
				Help:    "HTTP响应大小",
				Buckets: prometheus.ExponentialBuckets(1024, 2, 10),
			},
			[]string{"method", "path"},
		),
		
		// 后端服务指标
		BackendRequestsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "backend_requests_total",
				Help: "后端服务请求总数",
			},
			[]string{"backend", "method", "status_code"},
		),
		
		BackendRequestDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "backend_request_duration_seconds",
				Help:    "后端服务请求持续时间",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"backend", "method"},
		),
		
		BackendHealthStatus: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "backend_health_status",
				Help: "后端服务健康状态 (1=健康, 0=不健康)",
			},
			[]string{"backend"},
		),
		
		// 速率限制指标
		RateLimitRequestsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "rate_limit_requests_total",
				Help: "速率限制请求总数",
			},
			[]string{"result"}, // allowed, denied
		),
		
		// 缓存指标
		CacheRequestsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "cache_requests_total",
				Help: "缓存请求总数",
			},
			[]string{"result"}, // hit, miss
		),
		
		CacheHitRatio: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "cache_hit_ratio",
				Help: "缓存命中率",
			},
			[]string{"cache_type"},
		),
		
		// 系统指标
		ActiveConnections: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "active_connections",
			Help: "当前活跃连接数",
		}),
		
		TotalConnections: promauto.NewCounter(prometheus.CounterOpts{
			Name: "total_connections",
			Help: "总连接数",
		}),
		
		SystemUptime: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "system_uptime_seconds",
			Help: "系统运行时间（秒）",
		}),
		
		// 认证指标
		AuthRequestsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "auth_requests_total",
				Help: "认证请求总数",
			},
			[]string{"result"}, // success, failure
		),
		
		TokenValidationTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "token_validation_total",
				Help: "令牌验证总数",
			},
			[]string{"result"}, // valid, invalid, expired
		),
	}
}

// RecordHTTPRequest 记录HTTP请求指标
func (m *Metrics) RecordHTTPRequest(method, path string, statusCode int, duration time.Duration, requestSize, responseSize int64) {
	statusStr := strconv.Itoa(statusCode)
	
	m.RequestsTotal.WithLabelValues(method, path, statusStr).Inc()
	m.RequestDuration.WithLabelValues(method, path).Observe(duration.Seconds())
	
	if requestSize > 0 {
		m.RequestSize.WithLabelValues(method, path).Observe(float64(requestSize))
	}
	
	if responseSize > 0 {
		m.ResponseSize.WithLabelValues(method, path).Observe(float64(responseSize))
	}
}

// RecordBackendRequest 记录后端请求指标
func (m *Metrics) RecordBackendRequest(backend, method string, statusCode int, duration time.Duration) {
	statusStr := strconv.Itoa(statusCode)
	
	m.BackendRequestsTotal.WithLabelValues(backend, method, statusStr).Inc()
	m.BackendRequestDuration.WithLabelValues(backend, method).Observe(duration.Seconds())
}

// UpdateBackendHealth 更新后端健康状态
func (m *Metrics) UpdateBackendHealth(backend string, healthy bool) {
	var value float64
	if healthy {
		value = 1
	}
	m.BackendHealthStatus.WithLabelValues(backend).Set(value)
}

// RecordRateLimit 记录速率限制指标
func (m *Metrics) RecordRateLimit(allowed bool) {
	var result string
	if allowed {
		result = "allowed"
	} else {
		result = "denied"
	}
	m.RateLimitRequestsTotal.WithLabelValues(result).Inc()
}

// RecordCacheRequest 记录缓存请求指标
func (m *Metrics) RecordCacheRequest(hit bool) {
	var result string
	if hit {
		result = "hit"
	} else {
		result = "miss"
	}
	m.CacheRequestsTotal.WithLabelValues(result).Inc()
}

// UpdateCacheHitRatio 更新缓存命中率
func (m *Metrics) UpdateCacheHitRatio(cacheType string, ratio float64) {
	m.CacheHitRatio.WithLabelValues(cacheType).Set(ratio)
}

// RecordAuth 记录认证指标
func (m *Metrics) RecordAuth(success bool) {
	var result string
	if success {
		result = "success"
	} else {
		result = "failure"
	}
	m.AuthRequestsTotal.WithLabelValues(result).Inc()
}

// RecordTokenValidation 记录令牌验证指标
func (m *Metrics) RecordTokenValidation(result string) {
	m.TokenValidationTotal.WithLabelValues(result).Inc()
}

// IncrementActiveConnections 增加活跃连接数
func (m *Metrics) IncrementActiveConnections() {
	m.ActiveConnections.Inc()
	m.TotalConnections.Inc()
}

// DecrementActiveConnections 减少活跃连接数
func (m *Metrics) DecrementActiveConnections() {
	m.ActiveConnections.Dec()
}

// UpdateSystemUptime 更新系统运行时间
func (m *Metrics) UpdateSystemUptime(uptime time.Duration) {
	m.SystemUptime.Set(uptime.Seconds())
}

// MetricsCollector 指标收集器
type MetricsCollector struct {
	metrics   *Metrics
	startTime time.Time
}

// NewMetricsCollector 创建指标收集器
func NewMetricsCollector() *MetricsCollector {
	return &MetricsCollector{
		metrics:   NewMetrics(),
		startTime: time.Now(),
	}
}

// GetMetrics 获取指标实例
func (mc *MetricsCollector) GetMetrics() *Metrics {
	return mc.metrics
}

// UpdateSystemMetrics 更新系统指标
func (mc *MetricsCollector) UpdateSystemMetrics() {
	uptime := time.Since(mc.startTime)
	mc.metrics.UpdateSystemUptime(uptime)
}

// RequestMetrics 请求指标结构
type RequestMetrics struct {
	Method       string
	Path         string
	StatusCode   int
	Duration     time.Duration
	RequestSize  int64
	ResponseSize int64
	Backend      string
	CacheHit     bool
	RateLimited  bool
	Authenticated bool
}

// Record 记录请求指标
func (mc *MetricsCollector) Record(rm RequestMetrics) {
	// 记录HTTP请求指标
	mc.metrics.RecordHTTPRequest(rm.Method, rm.Path, rm.StatusCode, rm.Duration, rm.RequestSize, rm.ResponseSize)
	
	// 记录后端请求指标
	if rm.Backend != "" {
		mc.metrics.RecordBackendRequest(rm.Backend, rm.Method, rm.StatusCode, rm.Duration)
	}
	
	// 记录缓存指标
	mc.metrics.RecordCacheRequest(rm.CacheHit)
	
	// 记录速率限制指标
	mc.metrics.RecordRateLimit(!rm.RateLimited)
	
	// 记录认证指标
	mc.metrics.RecordAuth(rm.Authenticated)
}

// CustomMetrics 自定义指标
type CustomMetrics struct {
	gauges     map[string]prometheus.Gauge
	counters   map[string]prometheus.Counter
	histograms map[string]prometheus.Histogram
}

// NewCustomMetrics 创建自定义指标
func NewCustomMetrics() *CustomMetrics {
	return &CustomMetrics{
		gauges:     make(map[string]prometheus.Gauge),
		counters:   make(map[string]prometheus.Counter),
		histograms: make(map[string]prometheus.Histogram),
	}
}

// RegisterGauge 注册仪表盘指标
func (cm *CustomMetrics) RegisterGauge(name, help string) prometheus.Gauge {
	gauge := promauto.NewGauge(prometheus.GaugeOpts{
		Name: name,
		Help: help,
	})
	cm.gauges[name] = gauge
	return gauge
}

// RegisterCounter 注册计数器指标
func (cm *CustomMetrics) RegisterCounter(name, help string) prometheus.Counter {
	counter := promauto.NewCounter(prometheus.CounterOpts{
		Name: name,
		Help: help,
	})
	cm.counters[name] = counter
	return counter
}

// RegisterHistogram 注册直方图指标
func (cm *CustomMetrics) RegisterHistogram(name, help string, buckets []float64) prometheus.Histogram {
	histogram := promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    name,
		Help:    help,
		Buckets: buckets,
	})
	cm.histograms[name] = histogram
	return histogram
}

// GetGauge 获取仪表盘指标
func (cm *CustomMetrics) GetGauge(name string) (prometheus.Gauge, bool) {
	gauge, exists := cm.gauges[name]
	return gauge, exists
}

// GetCounter 获取计数器指标
func (cm *CustomMetrics) GetCounter(name string) (prometheus.Counter, bool) {
	counter, exists := cm.counters[name]
	return counter, exists
}

// GetHistogram 获取直方图指标
func (cm *CustomMetrics) GetHistogram(name string) (prometheus.Histogram, bool) {
	histogram, exists := cm.histograms[name]
	return histogram, exists
}

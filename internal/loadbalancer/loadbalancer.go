package loadbalancer

import (
	"errors"
	"hash/fnv"
	"math/rand"
	"net/url"
	"sync"
	"sync/atomic"
	"time"

	"api-gateway/internal/config"
	"api-gateway/internal/logger"
)

var (
	ErrNoBackendsAvailable = errors.New("没有可用的后端服务")
)

// Backend 后端服务
type Backend struct {
	URL            *url.URL
	Weight         int
	MaxConnections int
	CurrentConns   int64
	Healthy        bool
	LastCheck      time.Time
	mutex          sync.RWMutex
}

// NewBackend 创建后端服务实例
func NewBackend(cfg config.BackendConfig) (*Backend, error) {
	u, err := url.Parse(cfg.URL)
	if err != nil {
		return nil, err
	}

	return &Backend{
		URL:            u,
		Weight:         cfg.Weight,
		MaxConnections: cfg.MaxConnections,
		Healthy:        true,
		LastCheck:      time.Now(),
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
	b.LastCheck = time.Now()
}

// CanAcceptConnection 检查是否可以接受新连接
func (b *Backend) CanAcceptConnection() bool {
	currentConns := atomic.LoadInt64(&b.CurrentConns)
	return b.IsHealthy() && (b.MaxConnections == 0 || currentConns < int64(b.MaxConnections))
}

// AddConnection 增加连接计数
func (b *Backend) AddConnection() {
	atomic.AddInt64(&b.CurrentConns, 1)
}

// RemoveConnection 减少连接计数
func (b *Backend) RemoveConnection() {
	atomic.AddInt64(&b.CurrentConns, -1)
}

// GetCurrentConnections 获取当前连接数
func (b *Backend) GetCurrentConnections() int64 {
	return atomic.LoadInt64(&b.CurrentConns)
}

// LoadBalancer 负载均衡器接口
type LoadBalancer interface {
	NextBackend(clientIP string) (*Backend, error)
	AddBackend(backend *Backend)
	RemoveBackend(backendURL string)
	GetBackends() []*Backend
	UpdateBackendHealth(backendURL string, healthy bool)
}

// RoundRobinBalancer 轮询负载均衡器
type RoundRobinBalancer struct {
	backends []*Backend
	current  uint64
	mutex    sync.RWMutex
}

// NewRoundRobinBalancer 创建轮询负载均衡器
func NewRoundRobinBalancer() *RoundRobinBalancer {
	return &RoundRobinBalancer{
		backends: make([]*Backend, 0),
	}
}

// NextBackend 获取下一个后端服务
func (rr *RoundRobinBalancer) NextBackend(clientIP string) (*Backend, error) {
	rr.mutex.RLock()
	defer rr.mutex.RUnlock()

	if len(rr.backends) == 0 {
		return nil, ErrNoBackendsAvailable
	}

	// 过滤健康的后端
	healthyBackends := make([]*Backend, 0)
	for _, backend := range rr.backends {
		if backend.CanAcceptConnection() {
			healthyBackends = append(healthyBackends, backend)
		}
	}

	if len(healthyBackends) == 0 {
		return nil, ErrNoBackendsAvailable
	}

	// 轮询选择
	next := atomic.AddUint64(&rr.current, 1)
	return healthyBackends[(next-1)%uint64(len(healthyBackends))], nil
}

// AddBackend 添加后端服务
func (rr *RoundRobinBalancer) AddBackend(backend *Backend) {
	rr.mutex.Lock()
	defer rr.mutex.Unlock()
	rr.backends = append(rr.backends, backend)
}

// RemoveBackend 移除后端服务
func (rr *RoundRobinBalancer) RemoveBackend(backendURL string) {
	rr.mutex.Lock()
	defer rr.mutex.Unlock()

	for i, backend := range rr.backends {
		if backend.URL.String() == backendURL {
			rr.backends = append(rr.backends[:i], rr.backends[i+1:]...)
			break
		}
	}
}

// GetBackends 获取所有后端服务
func (rr *RoundRobinBalancer) GetBackends() []*Backend {
	rr.mutex.RLock()
	defer rr.mutex.RUnlock()
	return append([]*Backend{}, rr.backends...)
}

// UpdateBackendHealth 更新后端健康状态
func (rr *RoundRobinBalancer) UpdateBackendHealth(backendURL string, healthy bool) {
	rr.mutex.RLock()
	defer rr.mutex.RUnlock()

	for _, backend := range rr.backends {
		if backend.URL.String() == backendURL {
			backend.SetHealthy(healthy)
			break
		}
	}
}

// WeightedRoundRobinBalancer 加权轮询负载均衡器
type WeightedRoundRobinBalancer struct {
	backends []*Backend
	weights  []int
	current  []int
	mutex    sync.RWMutex
}

// NewWeightedRoundRobinBalancer 创建加权轮询负载均衡器
func NewWeightedRoundRobinBalancer() *WeightedRoundRobinBalancer {
	return &WeightedRoundRobinBalancer{
		backends: make([]*Backend, 0),
		weights:  make([]int, 0),
		current:  make([]int, 0),
	}
}

// NextBackend 获取下一个后端服务
func (wrr *WeightedRoundRobinBalancer) NextBackend(clientIP string) (*Backend, error) {
	wrr.mutex.Lock()
	defer wrr.mutex.Unlock()

	if len(wrr.backends) == 0 {
		return nil, ErrNoBackendsAvailable
	}

	// 过滤健康的后端
	healthyIndices := make([]int, 0)
	for i, backend := range wrr.backends {
		if backend.CanAcceptConnection() {
			healthyIndices = append(healthyIndices, i)
		}
	}

	if len(healthyIndices) == 0 {
		return nil, ErrNoBackendsAvailable
	}

	// 加权轮询算法
	totalWeight := 0
	maxCurrentWeight := -1
	selectedIndex := -1

	for _, i := range healthyIndices {
		wrr.current[i] += wrr.weights[i]
		totalWeight += wrr.weights[i]

		if wrr.current[i] > maxCurrentWeight {
			maxCurrentWeight = wrr.current[i]
			selectedIndex = i
		}
	}

	if selectedIndex >= 0 {
		wrr.current[selectedIndex] -= totalWeight
		return wrr.backends[selectedIndex], nil
	}

	return nil, ErrNoBackendsAvailable
}

// AddBackend 添加后端服务
func (wrr *WeightedRoundRobinBalancer) AddBackend(backend *Backend) {
	wrr.mutex.Lock()
	defer wrr.mutex.Unlock()
	wrr.backends = append(wrr.backends, backend)
	wrr.weights = append(wrr.weights, backend.Weight)
	wrr.current = append(wrr.current, 0)
}

// RemoveBackend 移除后端服务
func (wrr *WeightedRoundRobinBalancer) RemoveBackend(backendURL string) {
	wrr.mutex.Lock()
	defer wrr.mutex.Unlock()

	for i, backend := range wrr.backends {
		if backend.URL.String() == backendURL {
			wrr.backends = append(wrr.backends[:i], wrr.backends[i+1:]...)
			wrr.weights = append(wrr.weights[:i], wrr.weights[i+1:]...)
			wrr.current = append(wrr.current[:i], wrr.current[i+1:]...)
			break
		}
	}
}

// GetBackends 获取所有后端服务
func (wrr *WeightedRoundRobinBalancer) GetBackends() []*Backend {
	wrr.mutex.RLock()
	defer wrr.mutex.RUnlock()
	return append([]*Backend{}, wrr.backends...)
}

// UpdateBackendHealth 更新后端健康状态
func (wrr *WeightedRoundRobinBalancer) UpdateBackendHealth(backendURL string, healthy bool) {
	wrr.mutex.RLock()
	defer wrr.mutex.RUnlock()

	for _, backend := range wrr.backends {
		if backend.URL.String() == backendURL {
			backend.SetHealthy(healthy)
			break
		}
	}
}

// LeastConnectionsBalancer 最少连接数负载均衡器
type LeastConnectionsBalancer struct {
	backends []*Backend
	mutex    sync.RWMutex
}

// NewLeastConnectionsBalancer 创建最少连接数负载均衡器
func NewLeastConnectionsBalancer() *LeastConnectionsBalancer {
	return &LeastConnectionsBalancer{
		backends: make([]*Backend, 0),
	}
}

// NextBackend 获取下一个后端服务
func (lc *LeastConnectionsBalancer) NextBackend(clientIP string) (*Backend, error) {
	lc.mutex.RLock()
	defer lc.mutex.RUnlock()

	if len(lc.backends) == 0 {
		return nil, ErrNoBackendsAvailable
	}

	var selectedBackend *Backend
	minConnections := int64(-1)

	for _, backend := range lc.backends {
		if !backend.CanAcceptConnection() {
			continue
		}

		connections := backend.GetCurrentConnections()
		if minConnections == -1 || connections < minConnections {
			minConnections = connections
			selectedBackend = backend
		}
	}

	if selectedBackend == nil {
		return nil, ErrNoBackendsAvailable
	}

	return selectedBackend, nil
}

// AddBackend 添加后端服务
func (lc *LeastConnectionsBalancer) AddBackend(backend *Backend) {
	lc.mutex.Lock()
	defer lc.mutex.Unlock()
	lc.backends = append(lc.backends, backend)
}

// RemoveBackend 移除后端服务
func (lc *LeastConnectionsBalancer) RemoveBackend(backendURL string) {
	lc.mutex.Lock()
	defer lc.mutex.Unlock()

	for i, backend := range lc.backends {
		if backend.URL.String() == backendURL {
			lc.backends = append(lc.backends[:i], lc.backends[i+1:]...)
			break
		}
	}
}

// GetBackends 获取所有后端服务
func (lc *LeastConnectionsBalancer) GetBackends() []*Backend {
	lc.mutex.RLock()
	defer lc.mutex.RUnlock()
	return append([]*Backend{}, lc.backends...)
}

// UpdateBackendHealth 更新后端健康状态
func (lc *LeastConnectionsBalancer) UpdateBackendHealth(backendURL string, healthy bool) {
	lc.mutex.RLock()
	defer lc.mutex.RUnlock()

	for _, backend := range lc.backends {
		if backend.URL.String() == backendURL {
			backend.SetHealthy(healthy)
			break
		}
	}
}

// IPHashBalancer IP哈希负载均衡器
type IPHashBalancer struct {
	backends []*Backend
	mutex    sync.RWMutex
}

// NewIPHashBalancer 创建IP哈希负载均衡器
func NewIPHashBalancer() *IPHashBalancer {
	return &IPHashBalancer{
		backends: make([]*Backend, 0),
	}
}

// NextBackend 获取下一个后端服务
func (ih *IPHashBalancer) NextBackend(clientIP string) (*Backend, error) {
	ih.mutex.RLock()
	defer ih.mutex.RUnlock()

	if len(ih.backends) == 0 {
		return nil, ErrNoBackendsAvailable
	}

	// 过滤健康的后端
	healthyBackends := make([]*Backend, 0)
	for _, backend := range ih.backends {
		if backend.CanAcceptConnection() {
			healthyBackends = append(healthyBackends, backend)
		}
	}

	if len(healthyBackends) == 0 {
		return nil, ErrNoBackendsAvailable
	}

	// 使用IP哈希选择后端
	hash := fnv.New32a()
	hash.Write([]byte(clientIP))
	index := int(hash.Sum32()) % len(healthyBackends)

	return healthyBackends[index], nil
}

// AddBackend 添加后端服务
func (ih *IPHashBalancer) AddBackend(backend *Backend) {
	ih.mutex.Lock()
	defer ih.mutex.Unlock()
	ih.backends = append(ih.backends, backend)
}

// RemoveBackend 移除后端服务
func (ih *IPHashBalancer) RemoveBackend(backendURL string) {
	ih.mutex.Lock()
	defer ih.mutex.Unlock()

	for i, backend := range ih.backends {
		if backend.URL.String() == backendURL {
			ih.backends = append(ih.backends[:i], ih.backends[i+1:]...)
			break
		}
	}
}

// GetBackends 获取所有后端服务
func (ih *IPHashBalancer) GetBackends() []*Backend {
	ih.mutex.RLock()
	defer ih.mutex.RUnlock()
	return append([]*Backend{}, ih.backends...)
}

// UpdateBackendHealth 更新后端健康状态
func (ih *IPHashBalancer) UpdateBackendHealth(backendURL string, healthy bool) {
	ih.mutex.RLock()
	defer ih.mutex.RUnlock()

	for _, backend := range ih.backends {
		if backend.URL.String() == backendURL {
			backend.SetHealthy(healthy)
			break
		}
	}
}

// CreateLoadBalancer 创建负载均衡器
func CreateLoadBalancer(lbType config.LoadBalancerType) LoadBalancer {
	switch lbType {
	case config.LeastConn:
		return NewLeastConnectionsBalancer()
	case config.WeightedRound:
		return NewWeightedRoundRobinBalancer()
	case config.IPHash:
		return NewIPHashBalancer()
	default:
		return NewRoundRobinBalancer()
	}
}

// RandomBalancer 随机负载均衡器（额外实现）
type RandomBalancer struct {
	backends []*Backend
	mutex    sync.RWMutex
	rand     *rand.Rand
}

// NewRandomBalancer 创建随机负载均衡器
func NewRandomBalancer() *RandomBalancer {
	return &RandomBalancer{
		backends: make([]*Backend, 0),
		rand:     rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// NextBackend 获取下一个后端服务
func (r *RandomBalancer) NextBackend(clientIP string) (*Backend, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	if len(r.backends) == 0 {
		return nil, ErrNoBackendsAvailable
	}

	// 过滤健康的后端
	healthyBackends := make([]*Backend, 0)
	for _, backend := range r.backends {
		if backend.CanAcceptConnection() {
			healthyBackends = append(healthyBackends, backend)
		}
	}

	if len(healthyBackends) == 0 {
		return nil, ErrNoBackendsAvailable
	}

	// 随机选择
	index := r.rand.Intn(len(healthyBackends))
	return healthyBackends[index], nil
}

// AddBackend 添加后端服务
func (r *RandomBalancer) AddBackend(backend *Backend) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	r.backends = append(r.backends, backend)
}

// RemoveBackend 移除后端服务
func (r *RandomBalancer) RemoveBackend(backendURL string) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	for i, backend := range r.backends {
		if backend.URL.String() == backendURL {
			r.backends = append(r.backends[:i], r.backends[i+1:]...)
			break
		}
	}
}

// GetBackends 获取所有后端服务
func (r *RandomBalancer) GetBackends() []*Backend {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	return append([]*Backend{}, r.backends...)
}

// UpdateBackendHealth 更新后端健康状态
func (r *RandomBalancer) UpdateBackendHealth(backendURL string, healthy bool) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	for _, backend := range r.backends {
		if backend.URL.String() == backendURL {
			backend.SetHealthy(healthy)
			logger.Infof("Updated backend %s health status to %v", backendURL, healthy)
			break
		}
	}
}

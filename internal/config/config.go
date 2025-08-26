package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v2"
)

// Config 应用配置结构
type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Redis    RedisConfig    `yaml:"redis"`
	Routes   []RouteConfig  `yaml:"routes"`
	Auth     AuthConfig     `yaml:"auth"`
	Logging  LoggingConfig  `yaml:"logging"`
	Metrics  MetricsConfig  `yaml:"metrics"`
}

// ServerConfig 服务器配置
type ServerConfig struct {
	Port         int           `yaml:"port"`
	Host         string        `yaml:"host"`
	ReadTimeout  time.Duration `yaml:"read_timeout"`
	WriteTimeout time.Duration `yaml:"write_timeout"`
	IdleTimeout  time.Duration `yaml:"idle_timeout"`
	TLS          TLSConfig     `yaml:"tls"`
}

// TLSConfig TLS配置
type TLSConfig struct {
	Enabled  bool   `yaml:"enabled"`
	CertFile string `yaml:"cert_file"`
	KeyFile  string `yaml:"key_file"`
}

// RedisConfig Redis配置
type RedisConfig struct {
	Addr         string `yaml:"addr"`
	Password     string `yaml:"password"`
	DB           int    `yaml:"db"`
	PoolSize     int    `yaml:"pool_size"`
	MinIdleConns int    `yaml:"min_idle_conns"`
}

// RouteConfig 路由配置
type RouteConfig struct {
	Path         string           `yaml:"path"`
	Method       string           `yaml:"method"`
	Backends     []BackendConfig  `yaml:"backends"`
	AuthRequired bool             `yaml:"auth_required"`
	RateLimit    int              `yaml:"rate_limit"`
	CacheEnabled bool             `yaml:"cache_enabled"`
	CacheTTL     time.Duration    `yaml:"cache_ttl"`
	Timeout      time.Duration    `yaml:"timeout"`
	Retries      int              `yaml:"retries"`
	LoadBalancer LoadBalancerType `yaml:"load_balancer"`
	Middleware   []string         `yaml:"middleware"`
}

// BackendConfig 后端服务配置
type BackendConfig struct {
	URL            string        `yaml:"url"`
	Weight         int           `yaml:"weight"`
	MaxConnections int           `yaml:"max_connections"`
	HealthCheck    HealthCheck   `yaml:"health_check"`
	Timeout        time.Duration `yaml:"timeout"`
}

// HealthCheck 健康检查配置
type HealthCheck struct {
	Enabled  bool          `yaml:"enabled"`
	Path     string        `yaml:"path"`
	Interval time.Duration `yaml:"interval"`
	Timeout  time.Duration `yaml:"timeout"`
}

// AuthConfig 认证配置
type AuthConfig struct {
	JWTSecret     string        `yaml:"jwt_secret"`
	TokenExpiry   time.Duration `yaml:"token_expiry"`
	RefreshExpiry time.Duration `yaml:"refresh_expiry"`
	Issuer        string        `yaml:"issuer"`
}

// LoggingConfig 日志配置
type LoggingConfig struct {
	Level      string `yaml:"level"`
	Format     string `yaml:"format"`
	Output     string `yaml:"output"`
	MaxSize    int    `yaml:"max_size"`
	MaxBackups int    `yaml:"max_backups"`
	MaxAge     int    `yaml:"max_age"`
}

// MetricsConfig 指标配置
type MetricsConfig struct {
	Enabled bool   `yaml:"enabled"`
	Path    string `yaml:"path"`
	Port    int    `yaml:"port"`
}

// LoadBalancerType 负载均衡类型
type LoadBalancerType string

const (
	RoundRobin    LoadBalancerType = "round_robin"
	LeastConn     LoadBalancerType = "least_conn"
	WeightedRound LoadBalancerType = "weighted_round"
	IPHash        LoadBalancerType = "ip_hash"
)

// Load 从文件加载配置
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %w", err)
	}

	// 设置默认值
	setDefaults(&config)

	// 验证配置
	if err := validate(&config); err != nil {
		return nil, fmt.Errorf("配置验证失败: %w", err)
	}

	return &config, nil
}

// setDefaults 设置默认配置值
func setDefaults(config *Config) {
	if config.Server.Port == 0 {
		config.Server.Port = 8080
	}
	if config.Server.Host == "" {
		config.Server.Host = "0.0.0.0"
	}
	if config.Server.ReadTimeout == 0 {
		config.Server.ReadTimeout = 30 * time.Second
	}
	if config.Server.WriteTimeout == 0 {
		config.Server.WriteTimeout = 30 * time.Second
	}
	if config.Server.IdleTimeout == 0 {
		config.Server.IdleTimeout = 60 * time.Second
	}

	if config.Redis.Addr == "" {
		config.Redis.Addr = "localhost:6379"
	}
	if config.Redis.PoolSize == 0 {
		config.Redis.PoolSize = 10
	}
	if config.Redis.MinIdleConns == 0 {
		config.Redis.MinIdleConns = 2
	}

	if config.Auth.TokenExpiry == 0 {
		config.Auth.TokenExpiry = 24 * time.Hour
	}
	if config.Auth.RefreshExpiry == 0 {
		config.Auth.RefreshExpiry = 7 * 24 * time.Hour
	}

	if config.Logging.Level == "" {
		config.Logging.Level = "info"
	}
	if config.Logging.Format == "" {
		config.Logging.Format = "json"
	}

	if config.Metrics.Path == "" {
		config.Metrics.Path = "/metrics"
	}
	if config.Metrics.Port == 0 {
		config.Metrics.Port = 9090
	}

	// 设置路由默认值
	for i := range config.Routes {
		route := &config.Routes[i]
		if route.LoadBalancer == "" {
			route.LoadBalancer = RoundRobin
		}
		if route.Timeout == 0 {
			route.Timeout = 30 * time.Second
		}
		if route.Retries == 0 {
			route.Retries = 3
		}
		if route.CacheTTL == 0 {
			route.CacheTTL = 5 * time.Minute
		}

		// 设置后端服务默认值
		for j := range route.Backends {
			backend := &route.Backends[j]
			if backend.Weight == 0 {
				backend.Weight = 1
			}
			if backend.MaxConnections == 0 {
				backend.MaxConnections = 100
			}
			if backend.Timeout == 0 {
				backend.Timeout = 30 * time.Second
			}
			if backend.HealthCheck.Interval == 0 {
				backend.HealthCheck.Interval = 30 * time.Second
			}
			if backend.HealthCheck.Timeout == 0 {
				backend.HealthCheck.Timeout = 5 * time.Second
			}
			if backend.HealthCheck.Path == "" {
				backend.HealthCheck.Path = "/health"
			}
		}
	}
}

// validate 验证配置
func validate(config *Config) error {
	if config.Server.Port < 1 || config.Server.Port > 65535 {
		return fmt.Errorf("无效的服务器端口: %d", config.Server.Port)
	}

	if config.Auth.JWTSecret == "" {
		return fmt.Errorf("JWT密钥不能为空")
	}

	for i, route := range config.Routes {
		if route.Path == "" {
			return fmt.Errorf("路由 %d 的路径不能为空", i)
		}
		if route.Method == "" {
			return fmt.Errorf("路由 %d 的方法不能为空", i)
		}
		if len(route.Backends) == 0 {
			return fmt.Errorf("路由 %d 必须至少有一个后端服务", i)
		}

		for j, backend := range route.Backends {
			if backend.URL == "" {
				return fmt.Errorf("路由 %d 的后端服务 %d URL不能为空", i, j)
			}
		}
	}

	return nil
}

package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"api-gateway/internal/config"
	"api-gateway/internal/logger"
)

// Cache 缓存接口
type Cache interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error
	Del(ctx context.Context, keys ...string) error
	Exists(ctx context.Context, keys ...string) (int64, error)
	Incr(ctx context.Context, key string) (int64, error)
	Expire(ctx context.Context, key string, expiration time.Duration) error
	Close() error
}

// RedisCache Redis缓存实现
type RedisCache struct {
	client *redis.Client
}

// NewRedisCache 创建Redis缓存实例
func NewRedisCache(cfg config.RedisConfig) (Cache, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:         cfg.Addr,
		Password:     cfg.Password,
		DB:           cfg.DB,
		PoolSize:     cfg.PoolSize,
		MinIdleConns: cfg.MinIdleConns,
	})

	// 测试连接
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("连接Redis失败: %w", err)
	}

	logger.Info("Redis连接成功")
	return &RedisCache{client: rdb}, nil
}

// Get 获取缓存值
func (r *RedisCache) Get(ctx context.Context, key string) (string, error) {
	val, err := r.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return "", nil
	}
	return val, err
}

// Set 设置缓存值
func (r *RedisCache) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	var data string
	switch v := value.(type) {
	case string:
		data = v
	default:
		bytes, err := json.Marshal(value)
		if err != nil {
			return fmt.Errorf("序列化缓存值失败: %w", err)
		}
		data = string(bytes)
	}

	return r.client.Set(ctx, key, data, expiration).Err()
}

// Del 删除缓存键
func (r *RedisCache) Del(ctx context.Context, keys ...string) error {
	return r.client.Del(ctx, keys...).Err()
}

// Exists 检查键是否存在
func (r *RedisCache) Exists(ctx context.Context, keys ...string) (int64, error) {
	return r.client.Exists(ctx, keys...).Result()
}

// Incr 增加计数器
func (r *RedisCache) Incr(ctx context.Context, key string) (int64, error) {
	return r.client.Incr(ctx, key).Result()
}

// Expire 设置键过期时间
func (r *RedisCache) Expire(ctx context.Context, key string, expiration time.Duration) error {
	return r.client.Expire(ctx, key, expiration).Err()
}

// Close 关闭连接
func (r *RedisCache) Close() error {
	return r.client.Close()
}

// MemoryCache 内存缓存实现（用于开发和测试）
type MemoryCache struct {
	data map[string]cacheItem
}

type cacheItem struct {
	value      string
	expiration time.Time
}

// NewMemoryCache 创建内存缓存实例
func NewMemoryCache() Cache {
	return &MemoryCache{
		data: make(map[string]cacheItem),
	}
}

// Get 获取缓存值
func (m *MemoryCache) Get(ctx context.Context, key string) (string, error) {
	item, exists := m.data[key]
	if !exists {
		return "", nil
	}

	if !item.expiration.IsZero() && time.Now().After(item.expiration) {
		delete(m.data, key)
		return "", nil
	}

	return item.value, nil
}

// Set 设置缓存值
func (m *MemoryCache) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	var data string
	switch v := value.(type) {
	case string:
		data = v
	default:
		bytes, err := json.Marshal(value)
		if err != nil {
			return fmt.Errorf("序列化缓存值失败: %w", err)
		}
		data = string(bytes)
	}

	item := cacheItem{value: data}
	if expiration > 0 {
		item.expiration = time.Now().Add(expiration)
	}

	m.data[key] = item
	return nil
}

// Del 删除缓存键
func (m *MemoryCache) Del(ctx context.Context, keys ...string) error {
	for _, key := range keys {
		delete(m.data, key)
	}
	return nil
}

// Exists 检查键是否存在
func (m *MemoryCache) Exists(ctx context.Context, keys ...string) (int64, error) {
	count := int64(0)
	for _, key := range keys {
		if _, exists := m.data[key]; exists {
			count++
		}
	}
	return count, nil
}

// Incr 增加计数器
func (m *MemoryCache) Incr(ctx context.Context, key string) (int64, error) {
	item, exists := m.data[key]
	if !exists {
		m.data[key] = cacheItem{value: "1"}
		return 1, nil
	}

	// 简单实现，实际应该解析数字
	val := len(item.value) + 1
	m.data[key] = cacheItem{value: fmt.Sprintf("%d", val)}
	return int64(val), nil
}

// Expire 设置键过期时间
func (m *MemoryCache) Expire(ctx context.Context, key string, expiration time.Duration) error {
	if item, exists := m.data[key]; exists {
		item.expiration = time.Now().Add(expiration)
		m.data[key] = item
	}
	return nil
}

// Close 关闭连接
func (m *MemoryCache) Close() error {
	m.data = make(map[string]cacheItem)
	return nil
}

// GenerateCacheKey 生成缓存键
func GenerateCacheKey(prefix, path, method string, params map[string]string) string {
	key := fmt.Sprintf("%s:%s:%s", prefix, method, path)
	if len(params) > 0 {
		for k, v := range params {
			key += fmt.Sprintf(":%s=%s", k, v)
		}
	}
	return key
}

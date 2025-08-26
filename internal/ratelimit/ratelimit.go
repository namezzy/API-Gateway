package ratelimit

import (
	"context"
	"fmt"
	"time"

	"golang.org/x/time/rate"
	"api-gateway/internal/cache"
	"api-gateway/internal/logger"
)

// RateLimiter 速率限制器接口
type RateLimiter interface {
	Allow(ctx context.Context, key string, limit int) (bool, error)
	Reset(ctx context.Context, key string) error
}

// TokenBucketLimiter 令牌桶速率限制器
type TokenBucketLimiter struct {
	cache   cache.Cache
	buckets map[string]*rate.Limiter
}

// NewTokenBucketLimiter 创建令牌桶限制器
func NewTokenBucketLimiter(cache cache.Cache) *TokenBucketLimiter {
	return &TokenBucketLimiter{
		cache:   cache,
		buckets: make(map[string]*rate.Limiter),
	}
}

// Allow 检查是否允许请求
func (tbl *TokenBucketLimiter) Allow(ctx context.Context, key string, limit int) (bool, error) {
	limiter, exists := tbl.buckets[key]
	if !exists {
		// 创建新的限制器，每秒最多limit个请求，突发容量为limit
		limiter = rate.NewLimiter(rate.Limit(limit), limit)
		tbl.buckets[key] = limiter
	}

	return limiter.Allow(), nil
}

// Reset 重置限制器
func (tbl *TokenBucketLimiter) Reset(ctx context.Context, key string) error {
	delete(tbl.buckets, key)
	return nil
}

// SlidingWindowLimiter 滑动窗口速率限制器
type SlidingWindowLimiter struct {
	cache  cache.Cache
	window time.Duration
}

// NewSlidingWindowLimiter 创建滑动窗口限制器
func NewSlidingWindowLimiter(cache cache.Cache, window time.Duration) *SlidingWindowLimiter {
	return &SlidingWindowLimiter{
		cache:  cache,
		window: window,
	}
}

// Allow 检查是否允许请求
func (swl *SlidingWindowLimiter) Allow(ctx context.Context, key string, limit int) (bool, error) {
	now := time.Now()
	windowStart := now.Add(-swl.window)
	
	// 使用Redis的ZREMRANGEBYSCORE清理过期记录
	cacheKey := fmt.Sprintf("rate_limit:%s", key)
	
	// 清理过期记录（这里简化实现，实际应使用Redis的有序集合）
	currentCount, err := swl.getCurrentCount(ctx, cacheKey, windowStart)
	if err != nil {
		logger.Errorf("获取当前计数失败: %v", err)
		return false, err
	}

	if currentCount >= int64(limit) {
		return false, nil
	}

	// 增加计数
	if err := swl.incrementCount(ctx, cacheKey, now); err != nil {
		logger.Errorf("增加计数失败: %v", err)
		return false, err
	}

	return true, nil
}

// getCurrentCount 获取当前窗口内的请求计数
func (swl *SlidingWindowLimiter) getCurrentCount(ctx context.Context, key string, windowStart time.Time) (int64, error) {
	// 简化实现：使用计数器而不是有序集合
	// 实际生产环境应该使用Redis的ZCOUNT命令
	val, err := swl.cache.Get(ctx, key)
	if err != nil || val == "" {
		return 0, nil
	}
	
	// 这里应该解析时间戳列表并计算在窗口内的数量
	// 简化为直接返回计数
	count, _ := swl.cache.Incr(ctx, key+"_count")
	return count, nil
}

// incrementCount 增加计数
func (swl *SlidingWindowLimiter) incrementCount(ctx context.Context, key string, timestamp time.Time) error {
	// 简化实现：直接增加计数器
	_, err := swl.cache.Incr(ctx, key+"_count")
	if err != nil {
		return err
	}
	
	// 设置过期时间
	return swl.cache.Expire(ctx, key+"_count", swl.window)
}

// Reset 重置限制器
func (swl *SlidingWindowLimiter) Reset(ctx context.Context, key string) error {
	cacheKey := fmt.Sprintf("rate_limit:%s", key)
	return swl.cache.Del(ctx, cacheKey, cacheKey+"_count")
}

// FixedWindowLimiter 固定窗口速率限制器
type FixedWindowLimiter struct {
	cache  cache.Cache
	window time.Duration
}

// NewFixedWindowLimiter 创建固定窗口限制器
func NewFixedWindowLimiter(cache cache.Cache, window time.Duration) *FixedWindowLimiter {
	return &FixedWindowLimiter{
		cache:  cache,
		window: window,
	}
}

// Allow 检查是否允许请求
func (fwl *FixedWindowLimiter) Allow(ctx context.Context, key string, limit int) (bool, error) {
	now := time.Now()
	windowKey := fmt.Sprintf("rate_limit:%s:%d", key, now.Unix()/int64(fwl.window.Seconds()))
	
	// 获取当前窗口的计数
	count, err := fwl.cache.Incr(ctx, windowKey)
	if err != nil {
		logger.Errorf("增加计数失败: %v", err)
		return false, err
	}
	
	// 设置窗口过期时间
	if count == 1 {
		if err := fwl.cache.Expire(ctx, windowKey, fwl.window); err != nil {
			logger.Errorf("设置过期时间失败: %v", err)
		}
	}
	
	return count <= int64(limit), nil
}

// Reset 重置限制器
func (fwl *FixedWindowLimiter) Reset(ctx context.Context, key string) error {
	// 删除所有相关的窗口键（简化实现）
	pattern := fmt.Sprintf("rate_limit:%s:*", key)
	// 注意：实际实现中应该使用Redis的SCAN命令来查找并删除匹配的键
	return fwl.cache.Del(ctx, pattern)
}

// LimiterConfig 限制器配置
type LimiterConfig struct {
	Type     string        `yaml:"type"`     // token_bucket, sliding_window, fixed_window
	Window   time.Duration `yaml:"window"`   // 窗口大小
	BurstSize int          `yaml:"burst_size"` // 突发容量
}

// LimiterManager 限制器管理器
type LimiterManager struct {
	limiters map[string]RateLimiter
	cache    cache.Cache
}

// NewLimiterManager 创建限制器管理器
func NewLimiterManager(cache cache.Cache) *LimiterManager {
	return &LimiterManager{
		limiters: make(map[string]RateLimiter),
		cache:    cache,
	}
}

// GetLimiter 获取限制器
func (lm *LimiterManager) GetLimiter(name string, config LimiterConfig) RateLimiter {
	limiter, exists := lm.limiters[name]
	if exists {
		return limiter
	}

	switch config.Type {
	case "sliding_window":
		limiter = NewSlidingWindowLimiter(lm.cache, config.Window)
	case "fixed_window":
		limiter = NewFixedWindowLimiter(lm.cache, config.Window)
	default:
		limiter = NewTokenBucketLimiter(lm.cache)
	}

	lm.limiters[name] = limiter
	return limiter
}

// GenerateRateLimitKey 生成速率限制键
func GenerateRateLimitKey(clientIP, userID, path string) string {
	if userID != "" {
		return fmt.Sprintf("user:%s:%s", userID, path)
	}
	return fmt.Sprintf("ip:%s:%s", clientIP, path)
}

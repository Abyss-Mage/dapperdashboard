// Package cache is the optional Redis cache for the public website's data (owner 2026-11-13). Without DAPPER_REDIS_URL, or while Redis
// is unreachable, every call is a miss and the app reads PostgreSQL as before: Redis can never break the site.
package cache

import (
	"context"
	"log"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/redis/go-redis/v9"
)

const genKey = "dapper:public:gen"

type Cache struct {
	rdb      *redis.Client
	lastWarn atomic.Int64
	ttl      time.Duration
}

// New connects to redisURL (redis://:password@127.0.0.1:6379/0 or rediss://…). An empty URL gives nil, which is a valid, empty cache.
func New(redisURL string) (*Cache, error) {
	if redisURL == "" {
		return nil, nil
	}
	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, err
	}
	// Short limits: a slow or stopped Redis must cost a page view milliseconds, not seconds.
	opt.DialTimeout, opt.ReadTimeout, opt.WriteTimeout, opt.MaxRetries = time.Second, 300*time.Millisecond, 300*time.Millisecond, 0
	c := &Cache{rdb: redis.NewClient(opt), ttl: time.Minute}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := c.rdb.Ping(ctx).Err(); err != nil {
		log.Printf("Redis cache is not reachable yet (the site reads the database meanwhile): %v", err)
	}
	return c, nil
}

func (c *Cache) warn(err error) {
	if now := time.Now().Unix(); now-c.lastWarn.Load() > 60 {
		c.lastWarn.Store(now)
		log.Printf("Redis cache unavailable, reading the database instead: %v", err)
	}
}

// key puts the current generation in front, so Invalidate drops every earlier entry at once.
func (c *Cache) key(ctx context.Context, name string) (string, bool) {
	gen, err := c.rdb.Get(ctx, genKey).Int64()
	if err != nil && err != redis.Nil {
		c.warn(err)
		return "", false
	}
	return "dapper:public:" + strconv.FormatInt(gen, 10) + ":" + name, true
}

// Get returns a cached value, or false on a miss, an error or no cache.
func (c *Cache) Get(ctx context.Context, name string) ([]byte, bool) {
	if c == nil {
		return nil, false
	}
	k, ok := c.key(ctx, name)
	if !ok {
		return nil, false
	}
	b, err := c.rdb.Get(ctx, k).Bytes()
	if err != nil {
		if err != redis.Nil {
			c.warn(err)
		}
		return nil, false
	}
	return b, true
}

// Set stores a value for the cache's lifetime (one minute, a backstop to Invalidate).
func (c *Cache) Set(ctx context.Context, name string, value []byte) {
	if c == nil {
		return
	}
	if k, ok := c.key(ctx, name); ok {
		if err := c.rdb.Set(ctx, k, value, c.ttl).Err(); err != nil {
			c.warn(err)
		}
	}
}

// Invalidate drops everything cached, after any change made in the dashboard.
func (c *Cache) Invalidate(ctx context.Context) {
	if c == nil {
		return
	}
	if err := c.rdb.Incr(ctx, genKey).Err(); err != nil {
		c.warn(err)
	}
}

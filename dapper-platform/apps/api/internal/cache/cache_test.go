package cache

import (
	"context"
	"os"
	"testing"
	"time"
)

// No Redis, or Redis down: every call is a quick miss and nothing fails.
func TestCacheFallsBackQuietly(t *testing.T) {
	ctx := context.Background()
	none, err := New("")
	if err != nil || none != nil {
		t.Fatal("empty address must mean no cache")
	}
	none.Set(ctx, "k", []byte("v"))
	none.Invalidate(ctx)
	if _, ok := none.Get(ctx, "k"); ok {
		t.Fatal("nil cache returned a value")
	}
	if _, err = New("http://not-redis"); err == nil {
		t.Fatal("bad address accepted")
	}
	down, err := New("redis://127.0.0.1:1/0")
	if err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	down.Set(ctx, "k", []byte("v"))
	if _, ok := down.Get(ctx, "k"); ok || time.Since(start) > 5*time.Second {
		t.Fatal("unreachable Redis must miss quickly")
	}
}

// With a real Redis (DAPPER_TEST_REDIS_URL): values come back, and Invalidate drops them.
func TestCacheRoundTrip(t *testing.T) {
	url := os.Getenv("DAPPER_TEST_REDIS_URL")
	if url == "" {
		t.Skip("real Redis required")
	}
	ctx := context.Background()
	c, err := New(url)
	if err != nil {
		t.Fatal(err)
	}
	c.Set(ctx, "test", []byte("synthetic"))
	if b, ok := c.Get(ctx, "test"); !ok || string(b) != "synthetic" {
		t.Fatal("value not returned")
	}
	c.Invalidate(ctx)
	if _, ok := c.Get(ctx, "test"); ok {
		t.Fatal("value survived Invalidate")
	}
}

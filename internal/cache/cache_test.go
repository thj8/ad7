package cache

import (
	"sync"
	"testing"
	"time"
)

func TestGetSet(t *testing.T) {
	c := New[string](Options{})

	// Set value
	c.Set("key1", "value1")

	// Get existing value
	if v, ok := c.Get("key1"); !ok || v != "value1" {
		t.Errorf("Get(\"key1\") = %q, %v; want %q, %v", v, ok, "value1", true)
	}

	// Get non-existent value
	if v, ok := c.Get("key2"); ok || v != "" {
		t.Errorf("Get(\"key2\") = %q, %v; want %q, %v", v, ok, "", false)
	}
}

func TestTTL(t *testing.T) {
	c := New[string](Options{DefaultTTL: 50 * time.Millisecond})

	c.Set("key1", "value1")

	// Immediately get - should work
	if v, ok := c.Get("key1"); !ok || v != "value1" {
		t.Errorf("Get(\"key1\") = %q, %v; want %q, %v", v, ok, "value1", true)
	}

	// Wait for TTL to expire
	time.Sleep(60 * time.Millisecond)

	// Should have expired
	if v, ok := c.Get("key1"); ok {
		t.Errorf("Get(\"key1\") = %q, %v; want %q, %v (should have expired)", v, ok, "", false)
	}
}

func TestDelete(t *testing.T) {
	c := New[string](Options{})

	c.Set("key1", "value1")
	c.Delete("key1")

	if v, ok := c.Get("key1"); ok {
		t.Errorf("Get(\"key1\") after Delete = %q, %v; want %q, %v", v, ok, "", false)
	}
}

func TestLen(t *testing.T) {
	c := New[string](Options{})

	if c.Len() != 0 {
		t.Errorf("Len() = %d; want %d", c.Len(), 0)
	}

	c.Set("key1", "value1")
	if c.Len() != 1 {
		t.Errorf("Len() = %d; want %d", c.Len(), 1)
	}

	c.Set("key2", "value2")
	if c.Len() != 2 {
		t.Errorf("Len() = %d; want %d", c.Len(), 2)
	}
}

func TestConcurrentAccess(t *testing.T) {
	c := New[int](Options{})
	var wg sync.WaitGroup

	// Concurrent Set
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			c.Set(string(rune('a'+i)), i)
		}(i)
	}
	wg.Wait()

	// Concurrent Get
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if v, ok := c.Get(string(rune('a' + i))); !ok || v != i {
				t.Errorf("Get(\"%c\") = %d, %v; want %d, %v", 'a'+i, v, ok, i, true)
			}
		}(i)
	}
	wg.Wait()
}

func TestBackgroundCleanup(t *testing.T) {
	c := New[string](Options{
		DefaultTTL:      20 * time.Millisecond,
		CleanupInterval: 30 * time.Millisecond,
	})
	defer c.Stop()

	c.Set("key1", "value1")
	if c.Len() != 1 {
		t.Errorf("Len() = %d; want %d", c.Len(), 1)
	}

	// Wait for cleanup to run
	time.Sleep(50 * time.Millisecond)

	if c.Len() != 0 {
		t.Errorf("Len() after cleanup = %d; want %d", c.Len(), 0)
	}
}

func TestZeroTTL(t *testing.T) {
	// Zero TTL means never expire
	c := New[string](Options{})

	c.Set("key1", "value1")

	// Wait a bit - should still be there
	time.Sleep(10 * time.Millisecond)

	if v, ok := c.Get("key1"); !ok || v != "value1" {
		t.Errorf("Get(\"key1\") = %q, %v; want %q, %v", v, ok, "value1", true)
	}
}

func TestSetWithTTL(t *testing.T) {
	c := New[string](Options{DefaultTTL: time.Hour}) // 默认很长

	c.SetWithTTL("key1", "value1", 50*time.Millisecond) // 单条目短 TTL

	if v, ok := c.Get("key1"); !ok || v != "value1" {
		t.Errorf("Get(\"key1\") = %q, %v; want %q, %v", v, ok, "value1", true)
	}

	time.Sleep(60 * time.Millisecond)

	if v, ok := c.Get("key1"); ok {
		t.Errorf("Get(\"key1\") = %q, %v; want %q, %v (should have expired)", v, ok, "", false)
	}
}

func TestGetOrSet(t *testing.T) {
	c := New[string](Options{})

	called := 0
	fn := func() (string, error) {
		called++
		return "computed", nil
	}

	// 第一次调用：应该执行 fn
	v, err := c.GetOrSet("key1", fn)
	if err != nil {t.Errorf("GetOrSet() error = %v", err)
	}
	if v != "computed" {
		t.Errorf("GetOrSet() = %q, want %q", v, "computed")
	}
	if called != 1 {
		t.Errorf("called = %d, want 1", called)
	}

	// 第二次调用：应该从缓存获取，不执行 fn
	v2, err := c.GetOrSet("key1", fn)
	if err != nil {
		t.Errorf("GetOrSet() error = %v", err)
	}
	if v2 != "computed" {
		t.Errorf("GetOrSet() = %q, want %q", v2, "computed")
	}
	if called != 1 {
		t.Errorf("called = %d, want 1 (should not call fn again)", called)
	}
}

func TestDeleteByPrefix(t *testing.T) {
	c := New[string](Options{})

	c.Set("prefix:key1", "value1")
	c.Set("prefix:key2", "value2")
	c.Set("other:key3", "value3")

	if c.Len() != 3 {
		t.Errorf("Len() = %d, want 3", c.Len())
	}

	c.DeleteByPrefix("prefix:")

	if c.Len() != 1 {
		t.Errorf("Len() after DeleteByPrefix = %d, want 1", c.Len())
	}

	if _, ok := c.Get("prefix:key1"); ok {
		t.Error("prefix:key1 should be deleted")
	}
	if _, ok := c.Get("prefix:key2"); ok {
		t.Error("prefix:key2 should be deleted")
	}
	if v, ok := c.Get("other:key3"); !ok || v != "value3" {
		t.Error("other:key3 should still exist")
	}
}

func TestDeleteByPrefixEmpty(t *testing.T) {
	c := New[string](Options{})

	// 删除不存在的前缀应该不会报错
	c.DeleteByPrefix("nonexistent:")

	if c.Len() != 0 {
		t.Errorf("Len() = %d, want 0", c.Len())
	}
}

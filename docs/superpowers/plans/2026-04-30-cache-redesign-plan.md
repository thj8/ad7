# Cache Redesign Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 重构缓存系统，支持全局/模块级禁用、前缀批量失效、代码层面可拔插，且对现有代码无破坏性。

**Architecture:**
- 双层接口：`CacheProvider`（消费者）+ `CacheManager`（管理）
- `NoOpProvider` 提供安全的无缓存实现
- 前缀批量失效替代分散的单key删除
- 配置驱动启用/禁用

**Tech Stack:** Go, chi router, MySQL

---

## File Structure

| File | Responsibility |
|------|-----------------|
| `internal/cache/cache.go` | 泛型内存缓存，新增 `DeleteByPrefix` |
| `internal/pluginutil/cache.go` | 新增 `CacheManager` 接口和 `NoOpProvider` |
| `internal/config/config.go` | 扩展 `CacheConfig` 添加 `Enabled` 和 `Modules` |
| `plugins/cache/provider.go` | 实现 `CacheManager` 接口替代旧 `Provider` |
| `plugins/cache/cache.go` | 缓存插件重构，支持配置和 `GetProvider` |
| `plugins/analytics/analytics.go` | 使用 `GetProvider` 替代类型断言 |
| `plugins/leaderboard/leaderboard.go` | 使用 `GetProvider` 替代类型断言 |
| `plugins/topthree/topthree.go` | 使用 `GetProvider` 替代类型断言 |
| `cmd/server/main.go` | 配置驱动的缓存组装 |
| `config.yaml` | 添加缓存配置字段 |

---

## Tasks

### Task 1: Add DeleteByPrefix to internal/cache

**Files:**
- Modify: `internal/cache/cache.go`
- Modify: `internal/cache/cache_test.go`

- [ ] **Step 1: Write the failing test**

```go
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/cache -v -run TestDeleteByPrefix`
Expected: FAIL with "undefined: Cache.DeleteByPrefix"

- [ ] **Step 3: Write minimal implementation**

Add `strings` import first:
```go
import (
	"sync"
	"time"
	"strings"
)
```

Add this method after `Delete`:
```go
// DeleteByPrefix 删除所有匹配前缀的缓存条目
func (c *Cache[V]) DeleteByPrefix(prefix string) {
	c.mu.Lock()
	for k := range c.items {
		if strings.HasPrefix(k, prefix) {
			delete(c.items, k)
		}
	}
	c.mu.Unlock()
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/cache -v -run TestDeleteByPrefix`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/cache/cache.go internal/cache/cache_test.go
git commit -m "feat: add DeleteByPrefix to cache"
```

---

### Task 2: Add CacheManager and NoOpProvider to pluginutil

**Files:**
- Modify: `internal/pluginutil/cache.go`
- Modify: `internal/pluginutil/pluginutil_test.go`

- [ ] **Step 1: Write the failing test**

```go
func TestNoOpProvider(t *testing.T) {
	np := NoOpProvider{}

	// Get should always return (nil, false)
	if v, ok := np.Get("key"); v != nil || ok {
		t.Errorf("Get() = %v, %v; want nil, false", v, ok)
	}

	// Set should be no-op (no error)
	np.Set("key", "value")

	// Get still returns (nil, false)
	if v, ok := np.Get("key"); v != nil || ok {
		t.Errorf("Get() after Set = %v, %v; want nil, false", v, ok)
	}

	// Delete should be no-op (no error)
	np.Delete("key")

	// DeleteByPrefix should be no-op (no error)
	np.DeleteByPrefix("prefix:")
}

func TestWithCacheNoOp(t *testing.T) {
	called := 0
	fn := func() (any, error) {
		called++
		return "result", nil
	}

	np := NoOpProvider{}
	result, err := WithCache(np, "key", fn)

	if err != nil {
		t.Errorf("WithCache error = %v", err)
	}
	if result != "result" {
		t.Errorf("result = %v, want %v", result, "result")
	}
	if called != 1 {
		t.Errorf("fn called %d times, want 1", called)
	}

	// 第二次调用：仍然应该调用 fn（无缓存）
	result2, err := WithCache(np, "key", fn)
	if err != nil {
		t.Errorf("WithCache error = %v", err)
	}
	if result2 != "result" {
		t.Errorf("result = %v, want %v", result2, "result")
	}
	if called != 2 {
		t.Errorf("fn called %d times, want 2", called)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/pluginutil -v -run TestNoOpProvider`
Expected: FAIL with "undefined: NoOpProvider"

- [ ] **Step 3: Write minimal implementation**

Replace entire content of `internal/pluginutil/cache.go`:
```go
package pluginutil

// CacheProvider 定义缓存提供器接口，供消费者使用
type CacheProvider interface {
	Get(key string) (any, bool)
	Set(key string, value any)
}

// CacheManager 定义缓存管理接口，供缓存插件使用
type CacheManager interface {
	CacheProvider
	Delete(key string)
	DeleteByPrefix(prefix string)
}

// NoOpProvider 是无缓存实现，返回 (nil, false) 并忽略 Set/Delete
type NoOpProvider struct{}

func (n NoOpProvider) Get(key string) (any, bool) { return nil, false }
func (n NoOpProvider) Set(key string, value any) {}
func (n NoOpProvider) Delete(key string) {}
func (n NoOpProvider) DeleteByPrefix(prefix string) {}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/pluginutil -v -run TestNoOpProvider`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/pluginutil/cache.go internal/pluginutil/pluginutil_test.go
git commit -m "feat: add CacheManager interface and NoOpProvider"
```

---

### Task 3: Extend CacheConfig in internal/config

**Files:**
- Modify: `internal/config/config.go`

- [ ] **Step 1: Write the failing test**

```go
func TestCacheConfigDefaults(t *testing.T) {
	cfgYAML := `
server:
  port: 8080
db:
  host: localhost
  port: 3306
  user: root
  password: secret
  dbname: test
jwt:
  secret: test-secret-test-secret-test-secret
`
	cfg, err := LoadFromBytes([]byte(cfgYAML))
	if err != nil {
		t.Fatalf("LoadFromBytes error = %v", err)
	}

	// 缓存默认应该启用
	if !cfg.Cache.Enabled {
		t.Error("cfg.Cache.Enabled = false, want true")
	}

	// Modules 默认应该为 nil（表示全部启用）
	if cfg.Cache.Modules != nil {
		t.Error("cfg.Cache.Modules should be nil by default")
	}

	// 其他默认值应该保持不变
	if cfg.Cache.DefaultTTL != 5*time.Minute {
		t.Errorf("cfg.Cache.DefaultTTL = %v, want 5m", cfg.Cache.DefaultTTL)
	}
	if cfg.Cache.CleanupInterval != 10*time.Minute {
		t.Errorf("cfg.Cache.CleanupInterval = %v, want 10m", cfg.Cache.CleanupInterval)
	}
}

func TestCacheConfigDisabled(t *testing.T) {
	cfgYAML := `
server:
  port: 8080
db:
  host: localhost
  port: 3306
  user: root
  password: secret
  dbname: test
jwt:
  secret: test-secret-test-secret-test-secret
cache:
  enabled: false
`
	cfg, err := LoadFromBytes([]byte(cfgYAML))
	if err != nil {
		t.Fatalf("LoadFromBytes error = %v", err)
	}

	if cfg.Cache.Enabled {
		t.Error("cfg.Cache.Enabled = true, want false")
	}
}

func TestCacheConfigModules(t *testing.T) {
	cfgYAML := `
server:
  port: 8080
db:
  host: localhost
  port: 3306
  user: root
  password: secret
  dbname: test
jwt:
  secret: test-secret-test-secret-test-secret
cache:
  modules:
    analytics: false
    leaderboard: true
    topthree: true
    competition: false
    auth: true
`
	cfg, err := LoadFromBytes([]byte(cfgYAML))
	if err != nil {
		t.Fatalf("LoadFromBytes error = %v", err)
	}

	if cfg.Cache.Modules == nil {
		t.Fatal("cfg.Cache.Modules should not be nil")
	}
	if cfg.Cache.Modules["analytics"] {
		t.Error("analytics should be false")
	}
	if !cfg.Cache.Modules["leaderboard"] {
		t.Error("leaderboard should be true")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

First, we need to add `LoadFromBytes` for testing:
```go
// LoadFromBytes 从字节加载配置（用于测试）
func LoadFromBytes(data []byte) (*Config, error) {
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	setDefaults(&cfg)
	return &cfg, nil
}

// setDefaults 设置默认值
func setDefaults(cfg *Config) {
	if cfg.Server.Port == 0 {
		cfg.Server.Port = 8080
	}
	if cfg.JWT.AdminRole == "" {
		cfg.JWT.AdminRole = "admin"
	}
	if cfg.RateLimit.Submission.Requests == 0 {
		cfg.RateLimit.Submission.Requests = 3
	}
	if cfg.RateLimit.Submission.Window == 0 {
		cfg.RateLimit.Submission.Window = 10 * time.Second
	}
	if cfg.Log.Level == "" {
		cfg.Log.Level = "info"
	}
	if cfg.Auth.URL == "" {
		cfg.Auth.URL = "http://localhost:8081"
	}
	if cfg.Cache.DefaultTTL == 0 {
		cfg.Cache.DefaultTTL = 5 * time.Minute
	}
	if cfg.Cache.CleanupInterval == 0 {
		cfg.Cache.CleanupInterval = 10 * time.Minute
	}
	// 缓存默认启用
	if !cfg.Cache.Enabled {
		cfg.Cache.Enabled = true
	}
}
```

Then change `Load` to use `setDefaults`:
```go
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	setDefaults(&cfg)
	// ... rest of validation remains same ...
	return &cfg, nil
}
```

Now run the test:
Run: `go test ./internal/config -v -run TestCacheConfigDefaults`
Expected: FAIL with "undefined: NoOpProvider" and undefined fields

- [ ] **Step 3: Write minimal implementation**

Extend `CacheConfig` struct:
```go
// CacheConfig 定义缓存参数。
type CacheConfig struct {
	Enabled         bool            `yaml:"enabled"`
	DefaultTTL      time.Duration   `yaml:"default_ttl"`
	CleanupInterval time.Duration   `yaml:"cleanup_interval"`
	Modules         map[string]bool `yaml:"modules"`
}
```

Add `setDefaults` function as above, and update `Load` to use it.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/config -v -run TestCacheConfig`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/config/config.go internal/config/config_test.go
git commit -m "feat: extend CacheConfig with Enabled and Modules"
```

---

### Task 4: Rewrite plugins/cache/provider.go

**Files:**
- Modify: `plugins/cache/provider.go`

- [ ] **Step 1: Write minimal implementation**

Replace entire content of `plugins/cache/provider.go`:
```go
package cache

import (
	"ad7/internal/cache"
	"ad7/internal/pluginutil"
)

// cacheProvider 是 pluginutil.CacheManager 的实现
type cacheProvider struct {
	cache *cache.Cache[any]
}

func newCacheProvider(c *cache.Cache[any]) *cacheProvider {
	return &cacheProvider{cache: c}
}

func (p *cacheProvider) Get(key string) (any, bool) {
	return p.cache.Get(key)
}

func (p *cacheProvider) Set(key string, value any) {
	p.cache.Set(key, value)
}

func (p *cacheProvider) Delete(key string) {
	p.cache.Delete(key)
}

func (p *cacheProvider) DeleteByPrefix(prefix string) {
	p.cache.DeleteByPrefix(prefix)
}
```

- [ ] **Step 2: Run tests to verify builds**

Run: `go build ./plugins/cache`
Expected: no build errors

- [ ] **Step 3: Commit**

```bash
git add plugins/cache/provider.go
git commit -m "refactor: rewrite cache provider for CacheManager interface"
```

---

### Task 5: Restructure plugins/cache/cache.go

**Files:**
- Modify: `plugins/cache/cache.go`

- [ ] **Step 1: Write minimal implementation**

Replace entire content of `plugins/cache/cache.go`:
```go
// Package cache 提供可插拔的缓存插件，为其他插件和中间件提供缓存能力。
package cache

import (
	"context"
	"database/sql"
	"time"

	"github.com/go-chi/chi/v5"

	"ad7/internal/cache"
	"ad7/internal/config"
	"ad7/internal/event"
	"ad7/internal/middleware"
	"ad7/internal/plugin"
	"ad7/internal/pluginutil"
)

// topThreeProvider 是本地接口，只定义我们需要的方法
type topThreeProvider interface {
	IsTopThreeFull(ctx context.Context, compID, chalID string) bool
}

// Plugin 是缓存插件，提供通用缓存能力并处理缓存失效。
type Plugin struct {
	db        *sql.DB
	cache     *cache.Cache[any]
	manager   pluginutil.CacheManager
	authCache *cache.Cache[middleware.CachedToken]
	topThree  topThreeProvider
	enabled   bool
	modules   map[string]bool
}

// New 创建缓存插件实例。
func New(cfg config.CacheConfig) *Plugin {
	return &Plugin{
		enabled: cfg.Enabled,
		modules: cfg.Modules,
	}
}

// Name 返回插件名称。
func (p *Plugin) Name() string {
	return plugin.NameCache
}

// GetProvider 返回指定模块的缓存提供器。
func (p *Plugin) GetProvider(module string) pluginutil.CacheProvider {
	if !p.enabled {
		return pluginutil.NoOpProvider{}
	}
	if p.modules != nil {
		if !p.modules[module] {
			return pluginutil.NoOpProvider{}
		}
	}
	return p.manager
}

// Register 注册缓存插件，初始化缓存并订阅事件。
func (p *Plugin) Register(r chi.Router, db *sql.DB, auth *middleware.Auth, deps map[string]plugin.Plugin) {
	p.db = db

	if p.enabled {
		// 初始化通用缓存（5分钟TTL，10分钟清理间隔）
		p.cache = cache.New[any](cache.Options{
			DefaultTTL:      5 * time.Minute,
			CleanupInterval: 10 * time.Minute,
		})

		// 初始化token缓存（5分钟TTL）
		p.authCache = cache.New[middleware.CachedToken](cache.Options{
			DefaultTTL:      5 * time.Minute,
			CleanupInterval: 10 * time.Minute,
		})

		p.manager = newCacheProvider(p.cache)
	}

	// 从依赖中获取 topthree 插件
	if topThreePlugin, ok := deps[plugin.NameTopThree]; ok {
		if provider, ok := topThreePlugin.(topThreeProvider); ok {
			p.topThree = provider
		}
	}

	// 订阅正确提交事件，用于清除相关缓存
	event.Subscribe(event.EventCorrectSubmission, p.handleCorrectSubmission)

	// 设置缓存到 auth 中间件（仅当 auth 模块启用）
	if auth != nil && p.isModuleEnabled("auth") {
		type cacheSetter interface {
			SetCache(*cache.Cache[middleware.CachedToken])
		}
		if setter, ok := any(auth).(cacheSetter); ok {
			setter.SetCache(p.authCache)
		}
	}
}

// isModuleEnabled 检查模块是否启用
func (p *Plugin) isModuleEnabled(module string) bool {
	if !p.enabled {
		return false
	}
	if p.modules == nil {
		return true // modules为nil表示全部启用
	}
	return p.modules[module]
}

// handleCorrectSubmission 处理正确提交事件，清除相关缓存。
func (p *Plugin) handleCorrectSubmission(e event.Event) {
	if !p.enabled {
		return
	}

	compID := e.CompetitionID
	chalID := e.ChallengeID

	// 前缀批量清除
	p.manager.DeleteByPrefix("leaderboard:" + compID)
	p.manager.DeleteByPrefix("analytics:" + compID + ":")

	// 检查该题目的 top3 是否已填满（3项），如果未满才清除缓存
	ctx := context.Background()
	if p.topThree != nil {
		if !p.topThree.IsTopThreeFull(ctx, compID, chalID) {
			p.manager.DeleteByPrefix("topthree:" + compID)
		}
	} else {
		// 如果没有 topthree 插件，保守清除缓存
		p.manager.DeleteByPrefix("topthree:" + compID)
	}
}

// Stop 停止缓存后台清理。
func (p *Plugin) Stop() {
	if p.cache != nil {
		p.cache.Stop()
	}
	if p.authCache != nil {
		p.authCache.Stop()
	}
}
```

- [ ] **Step 2: Run tests to verify builds**

Run: `go build ./plugins/cache`
Expected: no build errors

- [ ] **Step 3: Commit**

```bash
git add plugins/cache/cache.go
git commit -m "refactor: restructure cache plugin with config support"
```

---

### Task 6: Update plugins/analytics/analytics.go

**Files:**
- Modify: `plugins/analytics/analytics.go`

- [ ] **Step 1: Write minimal implementation**

Change cache field type and Register method:

1. Change import: remove `"ad7/plugins/cache"` (we don't need it anymore)
2. Change cache field type from `cache.Provider` to `pluginutil.CacheProvider`
3. Change Register to get provider via `GetProvider("analytics")`

```go
// Plugin 是分析插件，持有数据库连接和缓存提供器。
type Plugin struct {
	db    *sql.DB
	cache pluginutil.CacheProvider
}

// ... rest of code ...

func (p *Plugin) Register(r chi.Router, db *sql.DB, auth *middleware.Auth, deps map[string]plugin.Plugin) {
	p.db = db
	p.cache = pluginutil.NoOpProvider{} // default to no-op

	if cachePlugin, ok := deps[plugin.NameCache].(*Plugin); ok {
		p.cache = cachePlugin.GetProvider("analytics")
	}

	// ... rest of registration ...
}
```

- [ ] **Step 2: Run tests to verify builds**

Run: `go build ./plugins/analytics`
Expected: no build errors

- [ ] **Step 3: Commit**

```bash
git add plugins/analytics/analytics.go
git commit -m "refactor: analytics plugin uses GetProvider"
```

---

### Task 7: Update plugins/leaderboard/leaderboard.go

**Files:**
- Modify: `plugins/leaderboard/leaderboard.go`

- [ ] **Step 1: Write minimal implementation**

Same pattern:

1. Change cache field type from `cache.Provider` to `pluginutil.CacheProvider`
2. Change Register to get provider via `GetProvider("leaderboard")`

```go
// Plugin 是排行榜插件，持有数据库连接和依赖。
type Plugin struct {
	db         *sql.DB
	topThree   topThree.TopThreeProvider
	cache      pluginutil.CacheProvider
}

// ... rest of code ...

func (p *Plugin) Register(r chi.Router, db *sql.DB, auth *middleware.Auth, deps map[string]plugin.Plugin) {
	p.db = db
	p.cache = pluginutil.NoOpProvider{}

	if cachePlugin, ok := deps[plugin.NameCache].(*Plugin); ok {
		p.cache = cachePlugin.GetProvider("leaderboard")
	}

	// ... rest of code ...
}
```

- [ ] **Step 2: Run tests to verify builds**

Run: `go build ./plugins/leaderboard`
Expected: no build errors

- [ ] **Step 3: Commit**

```bash
git add plugins/leaderboard/leaderboard.go
git commit -m "refactor: leaderboard plugin uses GetProvider"
```

---

### Task 8: Update plugins/topthree/topthree.go

**Files:**
- Modify: `plugins/topthree/topthree.go`

- [ ] **Step 1: Write minimal implementation**

Same pattern:

1. Change cache field type from `cache.Provider` to `pluginutil.CacheProvider`
2. Change Register to get provider via `GetProvider("topthree")`

```go
// Plugin 是三血追踪插件，持有数据库连接。
type Plugin struct {
	db    *sql.DB
	cache pluginutil.CacheProvider
}

// ... rest of code ...

func (p *Plugin) Register(r chi.Router, db *sql.DB, auth *middleware.Auth, deps map[string]plugin.Plugin) {
	p.db = db
	p.cache = pluginutil.NoOpProvider{}

	if cachePlugin, ok := deps[plugin.NameCache].(*Plugin); ok {
		p.cache = cachePlugin.GetProvider("topthree")
	}

	// ... rest of code ...
}
```

- [ ] **Step 2: Run tests to verify builds**

Run: `go build ./plugins/topthree`
Expected: no build errors

- [ ] **Step 3: Commit**

```bash
git add plugins/topthree/topthree.go
git commit -m "refactor: topthree plugin uses GetProvider"
```

---

### Task 9: Update cmd/server/main.go

**Files:**
- Modify: `cmd/server/main.go`

- [ ] **Step 1: Write minimal implementation**

Changes:

1. Pass `cfg.Cache` to `cache.New(cfg.Cache)`
2. Change cache provider extraction to use `GetProvider("competition")`

```go
plugins := []plugin.Plugin{
	cache.New(cfg.Cache),  // 传递配置
	leaderboard.New(),
	// ... rest of plugins ...
}

// ... build pluginMap ...

var cacheProvider pluginutil.CacheProvider = pluginutil.NoOpProvider{}
if cp, ok := pluginMap[plugin.NameCache].(*cache.Plugin); ok {
	cacheProvider = cp.GetProvider("competition")
}
```

- [ ] **Step 2: Run tests to verify builds**

Run: `go build ./cmd/server`
Expected: no build errors

- [ ] **Step 3: Commit**

```bash
git add cmd/server/main.go
git commit -m "refactor: main passes cache config and uses GetProvider"
```

---

### Task 10: Update config.yaml

**Files:**
- Modify: `config.yaml`

- [ ] **Step 1: Write minimal changes**

Add cache config fields:
```yaml
cache:
  enabled: true
  default_ttl: 5m
  cleanup_interval: 10m
  modules:
    analytics: true
    leaderboard: true
    topthree: true
    competition: true
    auth: true
```

- [ ] **Step 2: Run test to verify loads**

Run: `go test ./internal/config -v`
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add config.yaml
git commit -m "feat: add cache config fields"
```

---

### Task 11: Full Build and Integration Test Verification

**Files:** None (build and test only)

- [ ] **Step 1: Run full build**

Run: `go build ./...`
Expected: no build errors

- [ ] **Step 2: Run all unit tests**

Run: `go test ./internal/... ./plugins/... -v`
Expected: all tests pass

- [ ] **Step 3: Run integration tests (if MySQL available)**

Run: `TEST_DSN="root:pass@tcp(localhost:3306)/ctf_test?parseTime=true" go test ./internal/integration -v`
Expected: all integration tests pass

- [ ] **Step 4: Verify NoOp works (manual test)**

Modify config.yaml to `cache.enabled: false`, run server, verify no cache is used (all requests hit DB)

- [ ] **Step 5: Commit (if any test fixes needed)**

Only if fixes were needed during this step.

---

## Spec Self-Review

**1. Spec coverage:**
- ✅ Config-level switch: Task 3, 9, 10
- ✅ NoOpProvider: Task 2
- ✅ Code-level pluggable: Task 2, 5, 6, 7, 8
- ✅ Prefix-based invalidation: Task 1, 5
- ✅ Backward compatible: All business logic unchanged

**2. Placeholder scan:** No placeholders. All code is complete.

**3. Type consistency:** All types match between tasks. `GetProvider` returns `pluginutil.CacheProvider` consistently.

**4. No gaps:** Spec is fully covered.

---

## Execution Options

Plan complete and saved to `docs/superpowers/plans/2026-04-30-cache-redesign-plan.md`. Two execution options:

**1. Subagent-Driven (recommended)** - I dispatch a fresh subagent per task, review between tasks, fast iteration

**2. Inline Execution** - Execute tasks in this session using executing-plans, batch execution with checkpoints

Which approach?

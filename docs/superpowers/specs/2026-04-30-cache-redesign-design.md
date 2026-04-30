# Cache Redesign Design

## Problem

Current caching has three issues:

1. **Three separate interfaces**: `pluginutil.CacheProvider` (Get/Set), `cache.Provider` (richer), `*cache.Cache[V]` (direct). No unified nil-safe mechanism.
2. **Cannot easily disable**: No config switch to turn off cache globally or per-module. No no-op implementation.
3. **Scattered key management**: Cache keys hardcoded across plugins and cache plugin. No prefix-based batch invalidation.

## Goals

- Config-level switch to disable cache globally and per-module
- Code-level plugability: pass no-op to skip cache without changing business logic
- Prefix-based batch invalidation to replace scattered key management
- Zero changes to business plugins (analytics, leaderboard, topthree, handler)

## Design

### 1. Two-Layer Interface (`internal/pluginutil/cache.go`)

```go
// CacheProvider — read/write for consumers (plugins, handlers)
type CacheProvider interface {
    Get(key string) (any, bool)
    Set(key string, value any)
}

// CacheManager — management for cache plugin (adds deletion)
type CacheManager interface {
    CacheProvider
    Delete(key string)
    DeleteByPrefix(prefix string)
}

// NoOpProvider — safe empty implementation, usable as both CacheProvider and CacheManager
type NoOpProvider struct{}

func (n NoOpProvider) Get(string) (any, bool) { return nil, false }
func (n NoOpProvider) Set(string, any)        {}
func (n NoOpProvider) Delete(string)          {}
func (n NoOpProvider) DeleteByPrefix(string)  {}
```

`NoOpProvider` satisfies both `CacheProvider` and `CacheManager`. When used:
- `Get` returns `(nil, false)` — `WithCache` skips cache, calls fn directly
- `Set` is no-op — nothing stored
- `Delete`/`DeleteByPrefix` are no-op — nothing to delete

### 2. Add `DeleteByPrefix` to `*cache.Cache[V]` (`internal/cache/cache.go`)

```go
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

### 3. Update `cacheProvider` (`plugins/cache/provider.go`)

Implement `CacheManager` instead of `Provider`:

```go
type cacheProvider struct {
    cache *cache.Cache[any]
}

func (p *cacheProvider) Get(key string) (any, bool)        { return p.cache.Get(key) }
func (p *cacheProvider) Set(key string, value any)         { p.cache.Set(key, value) }
func (p *cacheProvider) Delete(key string)                 { p.cache.Delete(key) }
func (p *cacheProvider) DeleteByPrefix(prefix string)      { p.cache.DeleteByPrefix(prefix) }
func (p *cacheProvider) InvalidateByCompetition(string)    {} // deprecated, kept for compat
func (p *cacheProvider) InvalidateByChallenge(string, string) {} // deprecated, kept for compat
```

### 4. Configuration (`internal/config/config.go`)

```yaml
cache:
  enabled: true
  modules:
    analytics: true
    leaderboard: true
    topthree: true
    competition: true
    auth: true
```

```go
type CacheConfig struct {
    Enabled bool            `yaml:"enabled"`
    Modules map[string]bool `yaml:"modules"`
}
```

### 5. Cache Plugin Changes (`plugins/cache/cache.go`)

Event handler switches from per-key deletion to prefix-based:

```go
func (p *Plugin) handleCorrectSubmission(e event.Event) {
    compID := e.CompetitionID

    // Prefix-based batch invalidation
    p.manager.DeleteByPrefix("leaderboard:" + compID)
    p.manager.DeleteByPrefix("analytics:" + compID + ":")

    // topthree: check if full before clearing
    if p.topThree == nil || !p.topThree.IsTopThreeFull(ctx, compID, chalID) {
        p.manager.DeleteByPrefix("topthree:" + compID)
    }
}
```

Plugin stores `CacheManager` internally (instead of `Provider`), and exposes per-module `CacheProvider` to consumers:

```go
func (p *Plugin) GetProvider(module string) pluginutil.CacheProvider {
    if !p.enabled || !p.modules[module] {
        return pluginutil.NoOpProvider{}
    }
    return p.manager
}
```

### 6. Main Assembly (`cmd/server/main.go`)

```go
// Before:
var cacheProvider pluginutil.CacheProvider
if cp, ok := pluginMap[plugin.NameCache].(cache.Provider); ok {
    cacheProvider = cp
}
compH := handler.NewCompetitionHandler(compSvc, teamResolver, cacheProvider)

// After:
var cachePlugin *cache.Plugin // already type-asserted for Stop()
compH := handler.NewCompetitionHandler(compSvc, teamResolver, cachePlugin.GetProvider("competition"))
```

Other plugins get their provider during Register:
```go
func (p *LeaderboardPlugin) Register(..., deps ...) {
    if cp, ok := deps[plugin.NameCache].(*cache.Plugin); ok {
        p.cache = cp.GetProvider("leaderboard")
    } else {
        p.cache = pluginutil.NoOpProvider{}
    }
}
```

### 7. Auth Token Cache

Auth middleware uses `*cache.Cache[CachedToken]` directly — this stays unchanged. Token cache has its own independent lifecycle and doesn't participate in the event-driven invalidation. If `cache.modules.auth` is false, simply don't call `SetCache` on the middleware.

## Files Changed

| File | Change |
|------|--------|
| `internal/cache/cache.go` | Add `DeleteByPrefix` method |
| `internal/pluginutil/cache.go` | Add `CacheManager` interface, `NoOpProvider` type |
| `plugins/cache/provider.go` | Implement `CacheManager`, add `DeleteByPrefix` |
| `plugins/cache/cache.go` | Store `CacheManager`, add `GetProvider(module)`, update event handler to use prefix |
| `internal/config/config.go` | Add `CacheConfig` with `Enabled` and `Modules` |
| `config.yaml` | Add `cache:` section |
| `cmd/server/main.go` | Read config, use `GetProvider(module)` per consumer |
| `plugins/analytics/analytics.go` | Get provider via `GetProvider("analytics")` instead of type assertion |
| `plugins/leaderboard/leaderboard.go` | Same pattern |
| `plugins/topthree/topthree.go` | Same pattern |

## Files NOT Changed

- `internal/handler/competition.go` — still takes `CacheProvider`, no signature change
- `internal/middleware/auth.go` — still uses `*cache.Cache[CachedToken]` directly
- `internal/service/*` — no cache awareness
- `internal/store/*` — no cache awareness

## How to Disable Cache

**Global off:**
```yaml
cache:
  enabled: false
```
All `GetProvider()` calls return `NoOpProvider`. Zero cache reads, zero cache writes.

**Per-module off:**
```yaml
cache:
  enabled: true
  modules:
    analytics: false  # analytics hits DB every time
```

**Code-level (no config needed):**
```go
handler.NewCompetitionHandler(compSvc, teamResolver, pluginutil.NoOpProvider{})
```

## Backward Compatibility

- `pluginutil.WithCache()` works unchanged — NoOp's Get returns false, falls through to fn
- `CacheProvider` interface unchanged (Get + Set)
- Existing `Provider` interface removed from `plugins/cache/provider.go`, replaced by `CacheManager`
- Plugins that previously did type assertion to `cache.Provider` switch to `GetProvider(module)` — cleaner and no breakage

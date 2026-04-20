# 插件数据库访问限制设计文档

**日期**: 2026-04-20
**状态**: 待审核
**分支**: `feature/plugin-db-access-restriction`

---

## 一、概述

### 目标

1. **插件不能直接查询主表** - 插件只能查询自己创建的表，主表查询通过 `pluginutil` 进行
2. **插件之间通过接口互相调用** - 如 leaderboard 通过 topthree 暴露的接口获取三血数据，而不是直接查询 `topthree_records` 表
3. **使用依赖注入管理插件依赖关系** - 插件声明依赖，主程序按依赖顺序初始化

### 背景

根据 `CLAUDE.md` 新增约束：
> **查询数据库**: 插件不允许直接查询数据库，仅可以查自己插件的数据库

---

## 二、插件系统改造

### 2.1 Plugin 接口修改

**文件**: `internal/plugin/plugin.go`

**修改前**:
```go
type Plugin interface {
    Register(r chi.Router, db *sql.DB, auth *middleware.Auth)
}
```

**修改后**:
```go
type Plugin interface {
    // Name 返回插件的唯一名称，用于依赖管理
    Name() string

    // Register 方法在服务启动时被调用
    // 参数：
    //   - r: chi 路由器，用于注册路由
    //   - db: 数据库连接，供插件查询自己的表
    //   - auth: 认证中间件，用于保护插件路由
    //   - deps: 已初始化的依赖插件，key 是插件名称
    Register(r chi.Router, db *sql.DB, auth *middleware.Auth, deps map[string]Plugin)
}
```

### 2.2 插件名称常量

**文件**: `internal/plugin/names.go`（新建）

```go
package plugin

const (
    NameLeaderboard  = "leaderboard"
    NameNotification = "notification"
    NameHints        = "hints"
    NameAnalytics    = "analytics"
    NameTopThree     = "topthree"
)
```

### 2.3 TopThreeProvider 接口

**文件**: `plugins/topthree/provider.go`（新建）

```go
package topthree

import "context"

// TopThreeProvider 定义 topthree 插件暴露给其他插件的接口
type TopThreeProvider interface {
    // GetBloodRank 获取用户在某道题目的三血排名
    // 返回值: 1=一血, 2=二血, 3=三血, 0=未入榜
    GetBloodRank(ctx context.Context, compID, chalID, userID string) (int, error)

    // GetCompTopThree 获取比赛每道题目的三血信息
    // 返回值: map[challengeID]BloodRankEntry
    GetCompTopThree(ctx context.Context, compID string) (map[string]BloodRankEntry, error)
}

// BloodRankEntry 表示单道题目的三血排名信息
type BloodRankEntry struct {
    ChallengeID string
    FirstBlood  string // 用户ID
    SecondBlood string // 用户ID
    ThirdBlood  string // 用户ID
}
```

topthree 插件同时实现 `Plugin` 和 `TopThreeProvider` 接口。

---

## 三、pluginutil 扩展

**文件**: `internal/pluginutil/queries.go`

为 analytics 插件添加所需的聚合查询函数：

| 函数名 | 说明 |
|--------|------|
| `GetCategoryStats` | 获取按分类统计数据 |
| `GetUserStats` | 获取用户统计数据 |
| `GetChallengeStats` | 获取单题目统计数据 |
| `GetAverageSolveTime` | 获取平均解题时间 |

### 3.1 GetCategoryStats

```go
// CategoryStats 表示单个分类的统计数据
type CategoryStats struct {
    Category          string
    TotalChallenges   int
    TotalSolves       int
    UniqueUsersSolved int
    TotalAttempts     int
}

// GetCategoryStats 获取按分类统计的数据
func GetCategoryStats(ctx context.Context, db DBTX, compID string) ([]CategoryStats, error)
```

### 3.2 GetUserStats

```go
// UserStats 表示单个用户的统计数据
type UserStats struct {
    UserID         string
    TotalSolves    int
    TotalScore     int
    TotalAttempts  int
    SuccessRate    float64
    FirstSolveTime time.Time
    LastSolveTime  time.Time
}

// GetUserStats 获取用户统计数据
func GetUserStats(ctx context.Context, db DBTX, compID string) ([]UserStats, error)
```

### 3.3 GetChallengeStats

```go
// ChallengeStats 表示单个题目的统计数据
type ChallengeStats struct {
    ChallengeID       string
    Title             string
    Category          string
    Score             int
    TotalSolves       int
    TotalAttempts     int
    SuccessRate       float64
    UniqueUsersSolved int
    FirstSolveTime    time.Time
    AverageSolveTime  float64 // 秒
}

// GetChallengeStats 获取单题目统计数据
func GetChallengeStats(ctx context.Context, db DBTX, compID string) ([]ChallengeStats, error)
```

### 3.4 GetAverageSolveTime

```go
// GetAverageSolveTime 获取比赛平均解题时间（秒）
func GetAverageSolveTime(ctx context.Context, db DBTX, compID string) (float64, error)
```

---

## 四、插件初始化流程修改

**文件**: `cmd/server/main.go`

### 4.1 初始化流程

```
1. 创建所有插件实例
   ↓
2. 按依赖关系排序（topthree 先于 leaderboard）
   ↓
3. 按顺序调用 Register，传入依赖插件
```

### 4.2 拓扑排序

实现简单的拓扑排序，确保依赖的插件先初始化：

- leaderboard 依赖 topthree
- 其他插件无依赖

---

## 五、各插件改造详情

### 5.1 topthree 插件

**文件**: `plugins/topthree/topthree.go`

**改动**:
1. 实现 `Name() string` 方法，返回 `plugin.NameTopThree`
2. 实现 `TopThreeProvider` 接口的方法
3. 修改 `Register` 方法签名，接受 `deps map[string]Plugin` 参数

### 5.2 leaderboard 插件

**文件**: `plugins/leaderboard/leaderboard.go`

**改动**:
1. 实现 `Name() string` 方法，返回 `plugin.NameLeaderboard`
2. 修改 `Register` 方法签名，接受 `deps map[string]Plugin` 参数
3. 从 `deps` 中获取 `topthree` 插件，并断言为 `TopThreeProvider` 接口
4. 移除直接查询 `topthree_records` 表的 SQL
5. 通过 `TopThreeProvider.GetCompTopThree()` 获取三血数据

### 5.3 analytics 插件

**文件**: `plugins/analytics/analytics.go`

**改动**:
1. 实现 `Name() string` 方法，返回 `plugin.NameAnalytics`
2. 修改 `Register` 方法签名，接受 `deps map[string]Plugin` 参数
3. 移除所有直接查询主表的 SQL
4. 改用 `pluginutil` 中的函数：
   - `GetCategoryStats`
   - `GetUserStats`
   - `GetChallengeStats`
   - `GetAverageSolveTime`

### 5.4 notification 插件

**文件**: `plugins/notification/notification.go`

**改动**:
1. 实现 `Name() string` 方法，返回 `plugin.NameNotification`
2. 修改 `Register` 方法签名，接受 `deps map[string]Plugin` 参数
3. 其他逻辑不变（只查询自己的 `notifications` 表）

### 5.5 hints 插件

**文件**: `plugins/hints/hints.go`

**改动**:
1. 实现 `Name() string` 方法，返回 `plugin.NameHints`
2. 修改 `Register` 方法签名，接受 `deps map[string]Plugin` 参数
3. 其他逻辑不变（只查询自己的 `hints` 表）

---

## 六、数据流程图

### 6.1 leaderboard 获取三血数据

**修改前**:
```
leaderboard → SELECT FROM topthree_records
```

**修改后**:
```
leaderboard → deps["topthree"].(TopThreeProvider).GetCompTopThree()
    ↓
topthree → SELECT FROM topthree_records (自己的表)
    ↓
返回 BloodRankEntry 给 leaderboard
```

### 6.2 analytics 获取统计数据

**修改前**:
```
analytics → SELECT FROM competition_challenges + challenges + submissions
```

**修改后**:
```
analytics → pluginutil.GetCategoryStats()
    ↓
pluginutil → SELECT FROM competition_challenges + challenges + submissions
    ↓
返回 CategoryStats 给 analytics
```

---

## 七、向后兼容性

- 插件 `Register` 方法签名改变，所有插件都需要更新
- 旧的插件调用方式不再支持
- 建议在 feature 分支开发，测试通过后再合并

---

## 八、测试计划

1. 单元测试
   - 测试 pluginutil 新增函数
   - 测试 topthree 的 TopThreeProvider 实现

2. 集成测试
   - 测试 leaderboard 通过接口获取三血数据
   - 测试 analytics 通过 pluginutil 获取统计数据
   - 测试插件初始化和依赖注入

---

## 九、风险和注意事项

1. **性能影响**: analytics 插件的查询逻辑不变，只是从插件移到 pluginutil，性能影响很小

2. **依赖顺序**: 必须确保插件按正确顺序初始化，否则会导致运行时错误

3. **接口设计**: TopThreeProvider 接口设计要稳定，避免频繁变更影响依赖插件

---

## 十、验收标准

- [ ] 所有插件都实现 `Name()` 方法
- [ ] leaderboard 通过 TopThreeProvider 接口获取三血数据，不直接查询 topthree_records 表
- [ ] analytics 通过 pluginutil 函数获取统计数据，不直接查询主表
- [ ] notification 和 hints 只查询自己的表
- [ ] 插件按依赖顺序正确初始化
- [ ] 所有现有功能正常工作
- [ ] 所有测试通过

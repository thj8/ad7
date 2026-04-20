# 测试分布设计

日期: 2026-04-20

## 目标

将集成测试从 `internal/integration/integration_test.go` 单体文件拆分到各插件目录，补全缺失测试用例，使 `go test ./plugins/xxx/...` 可独立运行。

## 方案

**共享 testutil + 各目录独立测试**（方案 B）。

创建 `internal/testutil/` 包提供共享 setup 和 helpers。每个插件目录有自己的 `_test.go` 和独立 `TestMain`。

## 新增文件

```
internal/testutil/
  testutil.go            # TestEnv 结构体、NewTestEnv、Cleanup、helpers

plugins/leaderboard/
  leaderboard_test.go    # TestCompetitionLeaderboard

plugins/notification/
  notification_test.go   # TestCompetitionNotifications + Update/Delete

plugins/analytics/
  analytics_test.go      # TestAnalyticsOverview, Categories, Users, Challenges

plugins/hints/
  hints_test.go          # TestHints（CRUD + 可见性切换 + 404）

plugins/topthree/
  topthree_test.go       # TestTopThree + BaseModel 软删除
```

## internal/testutil/testutil.go

### TestEnv 结构体

```go
type TestEnv struct {
    Server *httptest.Server
    DB     *sql.DB
}
```

### NewTestEnv(t *testing.M) *TestEnv

- 从环境变量读取 `TEST_DSN`（回退到默认 DSN）
- 创建 store，组装 services/handlers/plugins
- 构建完整 chi 路由
- 返回包含 httptest.Server 的 TestEnv

### Cleanup(t *testing.T, db *sql.DB)

按依赖顺序清空表：topthree_records, hints, competition_challenges, notifications, submissions, competitions, challenges。

### Helpers

- `MakeToken(userID, role string) string` — 签发 JWT
- `MakeExpiredToken(userID, role string) string` — 签发过期 JWT
- `DoRequest(t, serverURL, method, path, body, token string) *http.Response`
- `DecodeJSON(t *testing.T, r *http.Response) map[string]any`
- `GetID(t *testing.T, m map[string]any) string`
- `AssertStatus(t *testing.T, resp *http.Response, want int)`

## 修改文件

### internal/integration/integration_test.go

精简为仅核心测试：

| 测试 | 覆盖 |
|------|------|
| TestListChallenges | GET /challenges，flag 不泄露 |
| TestGetChallenge | GET /challenges/{id}，404，401 |
| TestAdminCreateChallenge | POST，403，401 |
| TestAdminUpdateChallenge | PUT，404，403，401 |
| TestAdminDeleteChallenge | DELETE，软删除验证，403，401 |
| TestCompetitions | 完整 CRUD + 403，400 |
| TestCompetitionChallenges | 添加/移除题目，flag 不泄露 |
| TestSubmitInCompetition | 正确/错误/已解，401 |
| TestAdminListSubmissions | 列表 + 按 user_id/challenge_id 过滤 |
| TestSubmitFlagRateLimit | 3 次后 429，不同用户不受影响 |
| TestCompetitionStartEnd | 开始/结束/重复 409/404/403 |
| TestCompetitionAutoStatus | 自动激活、自动结束、ListActive 过滤 |
| TestInputValidation（新增） | 超长 title/flag/description，非法 JSON，400 |
| TestJWTExpiry（新增） | 过期 token → 401 |
| TestAdminListAllCompetitions（新增） | GET /admin/competitions 返回全部（含未激活） |

### 各插件 TestMain

每个插件的 `_test.go` 调用 `testutil.NewTestEnv` 并 `defer env.Close()`。插件测试包之间无共享状态。

## 新增测试用例

| 位置 | 测试 | 验证内容 |
|------|------|---------|
| plugins/analytics/ | TestAnalyticsUsers | /competitions/{id}/analytics/users 返回用户维度统计 |
| plugins/analytics/ | TestAnalyticsChallenges | /competitions/{id}/analytics/challenges 返回题目维度统计 |
| plugins/notification/ | TestNotificationUpdate | PUT 更新通知内容 |
| plugins/notification/ | TestNotificationDelete | DELETE 删除通知，验证已移除 |
| internal/integration/ | TestInputValidation | 超长字段返回 400 |
| internal/integration/ | TestJWTExpiry | 过期 token → 401 |
| internal/integration/ | TestAdminListAllCompetitions | 管理员可查看全部比赛 |

## Notification Update/Delete 功能

当前 notification 插件只有 create 和 list。需要新增：

### 新增路由

```
PUT  /api/v1/admin/notifications/{id}   — 管理员更新通知
DELETE /api/v1/admin/notifications/{id}  — 管理员删除通知
```

### 实现要点

- Update：解析 JSON body 更新 title 和/或 message，返回 204
- Delete：软删除（`is_deleted = 1`），返回 204
- 两个端点都需要 admin 权限
- 不存在的 notification ID 返回 404

## 约束

- 每个测试包可独立运行：`go test ./plugins/leaderboard/...`
- 全部测试通过：`go test ./...`
- 集成测试需要 MySQL（与之前一致）
- 通过 `TEST_DSN` 环境变量配置数据库连接

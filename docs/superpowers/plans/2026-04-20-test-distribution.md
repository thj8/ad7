# 测试分布实现计划

> **给 agentic worker：** 必须使用 superpowers:subagent-driven-development（推荐）或 superpowers:executing-plans 按任务执行。步骤使用 checkbox（`- [ ]`）语法跟踪进度。

**目标：** 将集成测试拆分到各插件目录，补全缺失测试，给 notification 插件增加 update/delete 接口，添加 pre-push git hook。

**架构：** 创建 `internal/testutil/` 共享包，每个插件目录有独立 `_test.go` 和 `TestMain`。先实现 notification update/delete，再迁移测试。

**技术栈：** Go testing, chi/httptest, MySQL, JWT, git hooks

---

## 任务 1: 创建 `internal/testutil/testutil.go`

**文件：**
- 创建: `internal/testutil/testutil.go`

- [ ] **步骤 1: 创建 testutil 包**

提供以下导出函数和类型：
- `TestEnv` 结构体（含 `Server *httptest.Server`, `DB *sql.DB`）
- `DSN()` — 读取 TEST_DSN 环境变量
- `NewTestEnv(m *testing.M) *TestEnv` — 连接 DB、组装完整路由、创建 httptest.Server
- `(e *TestEnv) Close()` — 关闭 server 和 DB
- `Cleanup(t *testing.T, db *sql.DB)` — 按依赖顺序清空所有表
- `MakeToken(userID, role string) string` — 签发 1 小时有效 JWT
- `MakeExpiredToken(userID, role string) string` — 签发已过期 JWT
- `DoRequest(t, serverURL, method, path, body, token string) *http.Response`
- `DecodeJSON(t, response) map[string]any`
- `GetID(t, m) string`
- `AssertStatus(t, response, want int)`

路由注册与当前 `internal/integration/integration_test.go` 的 TestMain 完全一致（含 rate limit 配置 3 requests/10s）。

常量：`TestSecret = "test-secret"`, `AdminRole = "admin"`

- [ ] **步骤 2: 验证编译**

运行: `go build ./internal/testutil/`
预期: 成功

- [ ] **步骤 3: 提交**

```
git add internal/testutil/testutil.go
git commit -m "feat: 添加 testutil 共享集成测试基础设施包"
```

---

## 任务 2: Notification 插件增加 Update/Delete 接口

**文件：**
- 修改: `plugins/notification/notification.go`
- 参考: `plugins/hints/hints.go`（已有 update/delete 模式）

- [ ] **步骤 1: 实现 update 和 delete handler**

在 `notification.go` 中新增：

1. `Register` 方法中添加两条路由：
   ```go
   r.With(auth.Authenticate, auth.RequireAdmin).Put("/api/v1/admin/notifications/{id}", p.update)
   r.With(auth.Authenticate, auth.RequireAdmin).Delete("/api/v1/admin/notifications/{id}", p.delete)
   ```

2. `update` handler：
   - 解析 URL 参数 `id`，调用 `pluginutil.ParseID` 验证
   - 解析 JSON body：`{"title": "...", "message": "..."}`
   - 仅更新非空字段（与 hints 插件模式一致）
   - 使用 `pluginutil.DBTX` 接口支持事务
   - `result.RowsAffected() == 0` 时返回 404
   - 成功返回 204
   - 记录操作日志

3. `delete` handler：
   - 解析 URL 参数 `id`，调用 `pluginutil.ParseID` 验证
   - 软删除：`UPDATE notifications SET is_deleted = 1, updated_at = NOW() WHERE res_id = ? AND is_deleted = 0`
   - `result.RowsAffected() == 0` 时返回 404
   - 成功返回 204
   - 记录操作日志

- [ ] **步骤 2: 验证编译**

运行: `go build ./...`
预期: 成功

- [ ] **步骤 3: 启动服务器验证路由不冲突**

运行: `go run ./cmd/server -config config.yaml`（3 秒后 Ctrl+C）
预期: 无 panic，正常启动

- [ ] **步骤 4: 提交**

```
git add plugins/notification/notification.go
git commit -m "feat: notification 插件增加 update/delete 接口"
```

---

## 任务 3: 更新 testutil 注册 notification 新路由

**文件：**
- 修改: `internal/testutil/testutil.go`

- [ ] **步骤 1: 确认 testutil 注册了完整路由**

testutil.go 中 `NewTestEnv` 通过 `p.Register(r, st.DB(), auth)` 注册所有插件路由。因为 notification 插件的 `Register` 方法内部自行注册路由（包括新增的 update/delete），testutil 无需修改。

验证：确认 `plugins` 列表包含 `notification.New()` 即可。无需改动。

- [ ] **步骤 2: 跳过此任务（无需修改）**

---

## 任务 4: 重写 `internal/integration/integration_test.go` 使用 testutil

**文件：**
- 修改: `internal/integration/integration_test.go`

- [ ] **步骤 1: 重写测试文件**

新文件使用 `testutil` 包。移除所有本地 helpers（`makeToken`、`doRequest`、`cleanup`、`decodeJSON`、`getID`、`assertStatus`）和 TestMain 中的 setup 代码。

新的 TestMain：
```go
var env *testutil.TestEnv

func TestMain(m *testing.M) {
    env = testutil.NewTestEnv(m)
    defer env.Close()
    os.Exit(m.Run())
}
```

所有 helper 调用替换为 testutil 版本（注意 `DoRequest` 多了 `env.Server.URL` 参数）。

保留的核心测试（12 个）：
- TestListChallenges, TestGetChallenge, TestAdminCreateChallenge, TestAdminUpdateChallenge, TestAdminDeleteChallenge
- TestCompetitions, TestCompetitionChallenges
- TestSubmitInCompetition, TestAdminListSubmissions
- TestSubmitFlagRateLimit, TestCompetitionStartEnd, TestCompetitionAutoStatus

移除的插件测试（后续任务迁移到插件目录）：
- TestCompetitionLeaderboard, TestCompetitionNotifications, TestAnalyticsOverview, TestAnalyticsCategories
- TestHints, TestTopThree, TestTopThreeBaseModelSoftDelete

- [ ] **步骤 2: 验证测试通过**

运行: `go test ./internal/integration/... -v -count=1`
预期: 12 个测试通过

- [ ] **步骤 3: 提交**

```
git add internal/integration/integration_test.go
git commit -m "refactor: 集成测试使用 testutil，移除插件测试（迁移到插件目录）"
```

---

## 任务 5: 创建 `plugins/leaderboard/leaderboard_test.go`

**文件：**
- 创建: `plugins/leaderboard/leaderboard_test.go`

- [ ] **步骤 1: 编写排行榜测试**

从旧 `integration_test.go` 的 `TestCompetitionLeaderboard` 迁移，改用 testutil helpers。

TestMain 调用 `testutil.NewTestEnv(m)`，测试函数使用 `testutil.Cleanup(t, env.DB)`。

测试内容不变：创建比赛 + 2 道题，user1 解两题，user2 解一题，验证排行榜排序和逐题详情。

- [ ] **步骤 2: 独立验证**

运行: `go test ./plugins/leaderboard/... -v -count=1`
预期: 通过（1 个测试）

- [ ] **步骤 3: 提交**

```
git add plugins/leaderboard/leaderboard_test.go
git commit -m "feat: 添加排行榜插件集成测试"
```

---

## 任务 6: 创建 `plugins/notification/notification_test.go`

**文件：**
- 创建: `plugins/notification/notification_test.go`

- [ ] **步骤 1: 编写通知测试（含 update/delete）**

从旧 `integration_test.go` 的 `TestCompetitionNotifications` 迁移，加上新增的 update/delete 测试。

测试内容：
1. 创建通知、查看通知列表（原有）
2. 非管理员不能创建（403）
3. 缺少字段返回 400
4. **新增**: Update 通知内容，验证列表中内容已更新
5. **新增**: Delete 通知，验证列表中已移除
6. **新增**: Update/Delete 不存在的 ID 返回 404

- [ ] **步骤 2: 独立验证**

运行: `go test ./plugins/notification/... -v -count=1`
预期: 通过（1 个测试，含 update/delete 子场景）

- [ ] **步骤 3: 提交**

```
git add plugins/notification/notification_test.go
git commit -m "feat: 添加通知插件集成测试（含 update/delete）"
```

---

## 任务 7: 创建 `plugins/analytics/analytics_test.go`

**文件：**
- 创建: `plugins/analytics/analytics_test.go`

- [ ] **步骤 1: 编写分析测试（含新增 users/challenges）**

4 个测试函数：
- `TestAnalyticsOverview` — 从旧文件迁移
- `TestAnalyticsCategories` — 从旧文件迁移
- `TestAnalyticsUsers`（新增）— 创建比赛 + 2 道题 + 2 用户提交，验证 `/analytics/users` 返回 `users` 数组含 `user_id`、`total_solves`、`total_score`、`total_attempts`、`success_rate`
- `TestAnalyticsChallenges`（新增）— 同上数据，验证 `/analytics/challenges` 返回 `challenges` 数组含 `challenge_id`、`title`、`total_solves`、`unique_users_solved`

- [ ] **步骤 2: 独立验证**

运行: `go test ./plugins/analytics/... -v -count=1`
预期: 通过（4 个测试）

- [ ] **步骤 3: 提交**

```
git add plugins/analytics/analytics_test.go
git commit -m "feat: 添加分析插件集成测试（含 users/challenges 端点）"
```

---

## 任务 8: 创建 `plugins/hints/hints_test.go`

**文件：**
- 创建: `plugins/hints/hints_test.go`

- [ ] **步骤 1: 编写提示测试**

从旧 `integration_test.go` 的 `TestHints` 迁移，改用 testutil helpers。

测试内容不变：创建 2 个 hint，验证列表，更新可见性，验证列表，删除，验证列表，测试 404。

- [ ] **步骤 2: 独立验证**

运行: `go test ./plugins/hints/... -v -count=1`
预期: 通过（1 个测试）

- [ ] **步骤 3: 提交**

```
git add plugins/hints/hints_test.go
git commit -m "feat: 添加提示插件集成测试"
```

---

## 任务 9: 创建 `plugins/topthree/topthree_test.go`

**文件：**
- 创建: `plugins/topthree/topthree_test.go`

- [ ] **步骤 1: 编写一二三血测试**

从旧 `integration_test.go` 的 `TestTopThree` 和 `TestTopThreeBaseModelSoftDelete` 迁移，改用 testutil helpers。

两个测试函数：
- `TestTopThree` — 4 用户提交，验证一血追踪
- `TestTopThreeBaseModelSoftDelete` — 验证 BaseModel 字段 + 软删除逻辑

- [ ] **步骤 2: 独立验证**

运行: `go test ./plugins/topthree/... -v -count=1`
预期: 通过（2 个测试）

- [ ] **步骤 3: 提交**

```
git add plugins/topthree/topthree_test.go
git commit -m "feat: 添加一二三血插件集成测试"
```

---

## 任务 10: 添加缺失测试到 `internal/integration/integration_test.go`

**文件：**
- 修改: `internal/integration/integration_test.go`

在现有测试后添加 3 个新测试函数。

- [ ] **步骤 1: 添加 TestInputValidation**

测试内容：
- title 超过 255 字符 → 400
- flag 超过 255 字符 → 400
- description 超过 4096 字符 → 400
- 非法 JSON body → 400
- 比赛 title 超长 → 400

- [ ] **步骤 2: 添加 TestJWTExpiry**

使用 `testutil.MakeExpiredToken` 创建过期 token，请求 `/api/v1/challenges` 期望 401。

- [ ] **步骤 3: 添加 TestAdminListAllCompetitions**

创建一个 active 比赛和一个已结束比赛（past end_time 触发 auto-end）。验证：
- `GET /api/v1/admin/competitions` 返回两个比赛
- 非 admin 访问返回 403

- [ ] **步骤 4: 验证全部集成测试通过**

运行: `go test ./internal/integration/... -v -count=1`
预期: 15 个测试通过（12 原有 + 3 新增）

- [ ] **步骤 5: 提交**

```
git add internal/integration/integration_test.go
git commit -m "feat: 添加输入验证、JWT 过期、管理员全量列表测试"
```

---

## 任务 11: 全量测试验证

**文件：** 无（仅验证）

- [ ] **步骤 1: 运行全部测试**

运行: `go test ./... -count=1`
预期: 全部通过

- [ ] **步骤 2: 逐个验证插件独立运行**

```bash
go test ./plugins/leaderboard/... -v -count=1
go test ./plugins/notification/... -v -count=1
go test ./plugins/analytics/... -v -count=1
go test ./plugins/hints/... -v -count=1
go test ./plugins/topthree/... -v -count=1
go test ./internal/integration/... -v -count=1
```

预期: 每个包独立运行通过。

---

## 任务 12: 添加 pre-push git hook

**文件：**
- 创建: `.git/hooks/pre-push`

- [ ] **步骤 1: 创建 hook 脚本**

```bash
#!/bin/sh
echo "推送前运行全部测试..."
go test ./...
if [ $? -ne 0 ]; then
  echo "错误: 测试失败，推送已中止。"
  exit 1
fi
echo "全部测试通过，继续推送。"
exit 0
```

- [ ] **步骤 2: 设置可执行权限**

运行: `chmod +x .git/hooks/pre-push`

- [ ] **步骤 3: 验证 hook**

运行: `go test ./...`
预期: 全部通过（模拟 hook 行为）

注：`.git/hooks/` 不被 git 跟踪，无需提交。

---

## Spec 覆盖检查

| Spec 需求 | 任务 |
|---|---|
| 创建 `internal/testutil/testutil.go` | 任务 1 |
| Notification 增加 update/delete | 任务 2 |
| 迁移排行榜测试 | 任务 5 |
| 迁移通知测试（含 update/delete） | 任务 6 |
| 迁移分析测试 + 新增 users/challenges | 任务 7 |
| 迁移提示测试 | 任务 8 |
| 迁移一二三血测试 | 任务 9 |
| 精简 integration_test.go | 任务 4 |
| 新增 TestInputValidation | 任务 10 |
| 新增 TestJWTExpiry | 任务 10 |
| 新增 TestAdminListAllCompetitions | 任务 10 |
| Pre-push git hook | 任务 12 |
| 独立 `go test ./plugins/xxx/...` | 任务 11 验证 |

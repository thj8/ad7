# Superpowers 中 Git Worktrees 的使用详解

## 1. 什么是 Git Worktree？

Git Worktree 允许你在同一个仓库中同时在多个分支上工作，无需来回切换。每个 worktree 是一个独立的目录，指向同一个 Git 仓库的不同分支。

## 2. 工作流程（实际例子）

### 阶段一：选择目录位置
```
1. 检查是否已有 .worktrees/ 或 worktrees/ 目录
   → 没有，继续

2. 检查 CLAUDE.md 是否有指定
   → 没有，询问用户

3. 用户选择 .worktrees/（项目本地隐藏目录）
```

### 阶段二：安全验证（关键！）
```
1. 检查 .worktrees/ 是否在 .gitignore 中
   → 发现不在！

2. 立即修复：添加 .worktrees/ 到 .gitignore 并提交
   → 防止 worktree 内容被意外提交到仓库
```

### 阶段三：创建 Worktree
```bash
git worktree add .worktrees/router-refactor -b feature/router-refactor
```
- 创建目录：`.worktrees/router-refactor/`
- 创建新分支：`feature/router-refactor`
- 自动切换到该 worktree

### 阶段四：项目设置（可选）
- 检测到是 Go 项目（有 go.mod）
- 运行 `go mod download`（依赖已存在时可跳过）

### 阶段五：验证基线
- 运行测试确保基础状态正常
- 注意区分：新引入的问题 vs 已存在的问题

---

## 3. 目录选择优先级

```
1. 优先使用已存在的 .worktrees/（隐藏目录，更干净）
2. 其次使用已存在的 worktrees/
3. 如果都没有，检查 CLAUDE.md 配置
4. 最后询问用户
```

## 4. 为什么需要 .gitignore 验证？

**安全原因：** Worktree 目录虽然是独立的，但如果不小心 `git add` 了里面的文件，会污染主仓库。所以必须确保：
- `.worktrees/` 在 `.gitignore` 中
- 如果不在，立即添加并提交

---

## 5. 与其他技能的配合

```
brainstorming（设计完成后）
    ↓
using-git-worktrees（创建隔离工作区）
    ↓
subagent-driven-development / executing-plans（执行计划）
    ↓
finishing-a-development-branch（完成，可选清理 worktree）
```

---

## 6. 常用 Git Worktree 命令

```bash
# 列出所有 worktree
git worktree list

# 删除 worktree（只删除目录，不删除分支）
git worktree remove .worktrees/router-refactor

# 删除 worktree 同时删除分支
git worktree remove .worktrees/router-refactor --force

# 手动添加现有分支的 worktree
git worktree add .worktrees/my-branch my-branch
```

---

## 7. 实际例子

```
主仓库：      /Users/sugar/src/project/ad7/ (分支: main)
Worktree:     /Users/sugar/src/project/ad7/.worktrees/router-refactor/ (分支: feature/router-refactor)
```

两个目录共享同一个 `.git` 仓库，但可以独立工作。

---

## 核心原则

**安全验证 → 创建隔离环境 → 工作 → 完成（可选清理）**

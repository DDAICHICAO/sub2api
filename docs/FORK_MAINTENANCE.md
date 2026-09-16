# 二次开发与上游更新

本仓库在保留作者完整 Git 历史的基础上进行二次开发，可持续合并作者更新。

## 仓库与分支

| 名称 | 用途 |
| --- | --- |
| `origin` | 自有仓库：`https://github.com/DDAICHICAO/sub2api.git` |
| `upstream` | 作者仓库：`https://github.com/Wei-Shaw/sub2api.git`，只拉取 |
| `main` | 上游基线，只做 fast-forward 更新，不提交二开修改 |
| `codex/my-feature` | 二开集成分支，开发、构建和部署使用此分支 |

初始化基线为上游 `881f3202694c6bc932446931a30c27d9675178b9`（版本 `0.2.5`）。
保留原有 LICENSE 和作者声明。GitHub 仓库不必具有 Fork 标识，共同提交历史即可支持正常 merge。

## 新电脑配置

```powershell
git clone --branch codex/my-feature https://github.com/DDAICHICAO/sub2api.git
cd sub2api
git remote add upstream https://github.com/Wei-Shaw/sub2api.git
git remote set-url --push upstream DISABLED
git config remote.pushDefault origin
git config push.default simple
git config push.followTags false
git fetch upstream
```

远程和 Git 配置仅保存在本机，重新克隆时需要重新配置。`DISABLED` 防止误推作者仓库。

## 合并作者更新

先提交或妥善保存当前修改，确认 `git status --short` 为空。以下命令逐条执行，任一步失败立即停止。

```powershell
git fetch upstream --prune
git fetch origin --prune
git switch main
git merge --ff-only origin/main
git merge --ff-only upstream/main
git push origin main
git switch codex/my-feature
git merge --ff-only origin/codex/my-feature
git switch -c codex/sync-upstream-YYYYMMDD
git merge --no-ff upstream/main
```

把 `YYYYMMDD` 替换为实际日期；分支已存在时使用新的唯一名称。新电脑首次 `git switch main` 会跟踪 `origin/main`。
冲突时逐项保留二开意图并兼容上游变化，解决后 `git add <文件>`、`git commit`；放弃此次合并可执行 `git merge --abort`。
不要对共享二开历史执行 reset、强推或 rebase。

按更新范围检查数据库迁移、配置兼容性和依赖变化。在具备项目工具链的环境中，安装前端依赖并运行相关检查：

```powershell
pnpm --dir frontend install --frozen-lockfile
pnpm --dir frontend run lint:check
pnpm --dir frontend run typecheck
pnpm --dir frontend run test:run
pnpm --dir frontend run build
Push-Location backend
go test ./...
Pop-Location
```

以当前仓库 CI、`DEV_GUIDE.md` 和各目录文档为准补充检查。测试通过后：

```powershell
git switch codex/my-feature
git merge --ff-only codex/sync-upstream-YYYYMMDD
git push origin codex/my-feature
```

协作时也可将同步分支推到 `origin`，创建目标为 `codex/my-feature` 的 PR，并选择保留提交历史的 merge，避免 squash 导致下次同步重复出现上游改动。
只需要某个独立特性时可以评估 `git cherry-pick <commit>`，但必须检查依赖提交；通常优先完整合并上游。

## 发布边界

- 不自动合并、自动部署；更新经过审查和测试后再进入二开分支。
- 不批量推送上游 tags：现有 Release workflow 会被 `v*` 标签触发。
- 自有版本发布前单独配置镜像仓库、发布权限和 secrets；上游镜像不会包含二开代码。
- 初始同步仅验证 Git 历史、远程和文档，不代表已完成应用测试或生产部署。

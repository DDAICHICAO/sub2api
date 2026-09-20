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

## 自有发行版（从 0.2.6 开始）

- 更新检查、回滚版本列表、安装脚本和页面上的版本链接使用 `DDAICHICAO/sub2api`。镜像使用 `ghcr.io/ddaichicao/sub2api`，生产应固定版本，不使用作者镜像。
- `main` 仍是作者基线；构建和手动发布必须选择 `codex/my-feature` 对应的已验证提交/tag。Release workflow 的前后端 checkout 都绑定发布 tag，避免混入其他分支。
- 正式发布使用完整 Release（`simple_release=false`），提供平台归档和 `checksums.txt`，才能使用页面内的二进制更新。现有二进制/归档名称保留 `sub2api` 以兼容更新器，不属于外发请求标识。
- 发往上游的自动生成提示词标签、保留 Python 工具别名、模型描述、Vertex 批任务默认名、Grok/Ollama 辅助请求 UA 已改成中性值。Grok OAuth 不再附加可选的项目 referrer。
- HTTP 上游传输会去掉 `X-Sub2API-*` 内部请求头；鉴权、请求 ID、用户正文、工具结果和上游协议要求的身份字段保留。
- 自有 Compose 默认 `UPSTREAM_BILLING_PROBE_DISABLED=true`，同时阻止定时和手动 `/v1/sub2api/billing` 探测。关闭该功能不删除已有余额数据；需要该专用协议时必须明确接受它暴露软件特征后再关闭此开关。
- 更新 Redis 缓存使用自有命名空间，并校验缓存里的 repository；不会从作者遗留缓存回退安装作者版本。
- 保留 LICENSE、作者声明、Go module 路径、数据库表名、内部稳定身份哈希种子。删除名称不能保证上游无法通过行为识别实现，历史用户内容也不会被批量替换。

### 保留数据的容器升级

先保存旧镜像、Compose/.env、应用数据、Redis 快照，并使用数据库容器的 `pg_dump -Fc` 备份、`pg_restore --list` 校验。确认新旧版本的迁移差异以及真实 PostgreSQL 数据目录；不能仅根据宿主机目录名称判断。

只修改现有 Compose 中应用服务的镜像引用，保留服务名、项目名、网络、所有挂载和环境密钥。使用现有完整 Compose 文件组合执行：

```bash
docker compose -f docker-compose.yml -f compose.proxy.yml up -d --no-deps --pull never sub2api
```

不要执行 `down -v`、清空持久化目录、重建数据库或重置初始化配置。切换前后比较受保护表的记录数/哈希、挂载和数据库/Redis/Caddy 容器 ID。回滚只需恢复备份的 Compose/.env 并用旧镜像重建应用；有迁移时须另行评估数据库恢复，不能盲目覆盖运行中的数据。

## Docker 更新与版本核验

Docker 容器必须通过固定版本镜像重建，禁止在线替换容器内二进制（更新和回退均适用）。后台返回 `deployment_mode=docker` 时只提供镜像操作说明，直接调用更新或回退接口返回 `DOCKER_IMAGE_UPDATE_REQUIRED`。

1. 在部署目录备份 Compose 文件。将应用 `image` 改为目标版本，例如 `ghcr.io/ddaichicao/sub2api:0.2.10`。
2. 检查 `.env` 的 `COMPOSE_FILE`、命令行 `-f` 及覆盖文件；覆盖文件中的旧镜像会覆盖主文件。运行 `docker compose config --images`，确认最终应用镜像符合目标才继续。
3. `docker compose pull sub2api` 成功后运行 `docker compose up -d --no-deps sub2api`，仅重建应用。
4. 核对 `docker compose ps sub2api`、`docker compose exec -T sub2api /app/sub2api --version` 和后台实际服务版本一致，并检查公网健康与既有认证请求路径。

裸二进制部署保留原地更新能力。网页更新成功不等于 Docker 镜像已更新；容器重建会丢弃容器可写层中的二进制替换，因此不能仅根据 `docker ps` 或历史使用记录推断运行版本及模型路由。

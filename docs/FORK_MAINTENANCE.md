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

自有 0.2.18 起可安装 [宿主机更新服务](../deploy/docker-updater/README.md)，恢复后台一键更新，同时通过 Compose 重建保证镜像和程序版本一致。未安装时仍显示手动更新说明。首次启用需一次镜像升级，新组件不会修改旧容器的页面资源。

Docker 容器必须通过固定版本镜像重建，禁止在线替换容器内二进制（更新和回退均适用）。后台返回 `deployment_mode=docker` 时，已安装宿主机服务的部署通过独立 Docker 更新接口显示在线升级及进度；未安装时提供手动说明。旧二进制更新或回退接口仍返回 `DOCKER_IMAGE_UPDATE_REQUIRED`。

### 每次发布必须保留后台点击升级（用户要求，2026-10-14）

无论合并作者分支还是发布自研功能，后台在线升级都是必须保留的发行能力。不能为了版本一致性再次移除按钮，也不能把手动 SSH/Compose 命令作为已启用在线升级部署的正常更新方式。

- 发布前检查版本面板、admin Docker 更新接口、宿主机 socket 协议和更新任务状态恢复的兼容性，运行对应回归测试。宿主助手不随镜像升级，候选版本必须兼容已安装助手；若助手需要变更，先提供并验证迁移路径，不得发布一个旧助手无法安装的新版本却宣称可在线更新。
- 正式发布后核对自有发行源能发现新版本，Release 归档和 checksums 完整，固定版本镜像可拉取，版本与提交标签匹配；不能只以 CI 成功或创建 tag 视为升级已可用。
- 在有更新的场景验证管理员可点击升级、看到进度、跨应用重启恢复状态和显示结果。不能仅用宿主 socket 测试代替浏览器按钮验收；未完成的验证须明确记录。
- 升级后核对镜像标签、二进制和后台版本一致，保留数据库备份、挂载、环境变量及账号配置，并验证公网真实请求。数据库迁移不兼容时不得盲目自动恢复数据库。
- 默认由用户在后台点击更新。2026-10-14 授权的首次 0.2.18 bootstrap 已完成，不是以后自动切换生产镜像的长期授权。当前已是最新版时没有升级按钮属于正常状态；检测到下一版时必须可用。

以下为未安装助手或故障恢复时的手动步骤：

1. 在部署目录备份 Compose 文件。将应用 `image` 改为目标版本，例如 `ghcr.io/ddaichicao/sub2api:0.2.10`。
2. 检查 `.env` 的 `COMPOSE_FILE`、命令行 `-f` 及覆盖文件；覆盖文件中的旧镜像会覆盖主文件。运行 `docker compose config --images`，确认最终应用镜像符合目标才继续。
3. `docker compose pull sub2api` 成功后运行 `docker compose up -d --no-deps sub2api`，仅重建应用。
4. 核对 `docker compose ps sub2api`、`docker compose exec -T sub2api /app/sub2api --version` 和后台实际服务版本一致，并检查公网健康与既有认证请求路径。

裸二进制部署保留原地更新能力。网页更新成功不等于 Docker 镜像已更新；容器重建会丢弃容器可写层中的二进制替换，因此不能仅根据 `docker ps` 或历史使用记录推断运行版本及模型路由。

## 0.2.15 同步（2026-10-08）

- 合入上游 `3f1a2ea0a`（0.2.14），包括 GPT-6.1 Sol、GPT-6 Sol/Luna、远程模型目录发现及安全修复。保留自有发行源、Docker 更新保护、外发请求处理和容量错误跨账号重试。
- 发布流程采用上游矩阵构建，前后端及镜像绑定同一解析后的提交；自有版本使用 0.2.15，避免与上游基线混淆。
- 新模型需要同时核对账号映射、分组白名单和 `/v1/models`。账号映射存在不表示分组允许访问；上游目录可见也不代替完成态真实请求验证。保留已有白名单开关和原有模型，仅追加确认的模型。
- 此次迁移新增审计字段、计费倍率字段、提现幂等索引、充值赠送字段及平台约束。部署前备份数据库及完整 Compose 组合，升级后检查原有账号、密钥、余额和持久化挂载。
- 本地前端构建和相关页面/模型配置测试、后端模型/更新/重试测试、迁移测试、嵌入资源构建通过；发布工具的 10 项测试在隔离 Linux 临时目录通过。Windows 的发布工具测试受默认编码和 Bash 环境限制，不作为 Linux 发布失败的依据。
- 修正上游客户端配置测试遗漏的 `api_key_model_discovery` 预期，并检查自有网络边界测试关闭响应体时的错误。完整 CI 和生产验收结果需以实际执行为准。

## 0.2.16：搜索历史与上下文压缩兼容修复（2026-10-14）

- 移植上游 PR #7939 的两个提交 `dab3b87ea`、`64caa9af8`，修复 OAuth 请求回放 `web_search_call` 历史却未声明搜索工具时，上游报 `response protection is unavailable` 的问题。
- 标准 Responses 补齐缓存搜索工具声明；Responses Lite 使用 `additional_tools`，保留尾部 `compaction_trigger` 的位置。原请求无工具时设置 `tool_choice=none`，保留禁止调用工具的语义；保留历史内容，不关闭上游保护。
- 覆盖转换、HTTP 透传、WebSocket 路径，保持独立 `/responses/compact` 和 API Key 账号路径原有行为。此次没有数据库迁移、前端改动、代理或账号调度配置修改。
- 发布前同一生产账号及公网入口的合成搜索历史对照：缺少声明时 0.54 秒返回同名失败；补齐声明后 5.54 秒完成。短 ping 测试不能作为多轮或压缩请求正常的证据。
- 迁移后的主机可能以独立 Compose 配置启动。部署前读取运行容器的 `com.docker.compose.project.config_files`，备份并使用实际配置，仅变更应用镜像；同时同步常规 Compose 文件中的版本，防止下次从旧文件启动造成回退。

## 0.2.17：完整同步作者 0.2.16（2026-10-14）

- 完整合并作者 `9c2cca9e627291bc719bada3d57de23b625a4180`，保留共同历史；自有版本升至 0.2.17，不能用双方相同的版本号推断内容一致。
- 包含 GPT-6.1 Sol 500k 客户端配置、订阅长上下文计费调整、加密推理签名恢复、流式心跳与工具调用兼容、模型映射及模型级调度规则，以及 Claude Haiku 5.5、Gemini 显式缓存、Vertex API Key、Cline 和 Command Code 支持。
- Go 及构建镜像统一升级至 1.27.2；合入上游依赖安全修复及对应 CI 工具升级。保留自有发行源、镜像归属、Docker 原地更新保护、外发标识处理、账单探测关闭开关与容量错误跨账号重试。
- 上次单独移植的搜索历史修复继续保留；冲突仅涉及同等 lint 修正及客户端配置测试，配置测试同时校验上游特性与模型发现开关。
- 新增五个 SQL 迁移：两个 242 文件分别取消平台枚举数据库约束、增加分组多语言字段；243–245 新增 Gemini 缓存及计费/变更租约字段。迁移按完整文件名及校验和登记，两个 242 文件不会互相覆盖。更新容器前仍需备份数据库，不能把切回旧镜像视为数据库回滚。
- 此次请求范围为合并和发布；Release、归档、校验文件与 GHCR 镜像完成后单独报告，生产容器升级与公网验收另行执行。

- 发布页安装命令与文档固定到当前发布标签，避免从仅保存作者基线的 `main` 下载原版安装器；用量页面测试使用受控计时器并在退出时清理，避免动画回调污染完整测试结果。

## 0.2.19：同步作者主分支（2026-10-20）

- 合入作者 `8f2cbb53d`，包含正式版 0.2.17 及之后的 Gemini 计费、断流和异步生图下载修复；自有版本继续独立递增。
- 纳入 OAuth 混合分组歧义路由保护、Codex workspace 导入隔离、Responses created_at 补齐、API Key 搜索模拟、GPT-5.5 退役处理，以及 Gemini 4 Argon、思考摘要和支付退款修复。混合路由保护不能保证修复上游实际返回不同模型的问题。
- 保留自有发行源、镜像身份核验、搜索历史/压缩兼容、自研重试逻辑和后台 Docker 在线升级。此次不修改宿主助手或 socket 协议，兼容 0.2.18 已安装助手；上游差异不包含新增数据库迁移。
- 发布后核验正式 Release、五个平台归档校验和、两种 Linux 架构镜像，以及线上旧版本的更新发现和助手可用性。由用户在后台点击升级，发布操作不自动切换生产容器。

## 0.2.20：同步作者主分支（2026-10-25）

- 合入作者 `3f1c8f36c`，含作者 0.2.18 及受限管理员 API 密钥。自有版本独立递增至 0.2.20。
- 合并 overload 身份配对与空流安全重试、queued/metadata 事件后的重试保护；冲突保留自有 codex.rate_limits/codex.response.metadata/keepalive 判定和跨账号容量错误处理。不能仅凭 overloaded 文案断定是物理容量不足，也不能保证所有上游过载都会消失。
- 纳入账号/调度快照性能优化、Cloudflare 1010 不惩罚账号、Vertex 配额、兼容流结束与计费、工具参数和生图修复；Go 与构建/CI 对齐 1.27.3，DOMPurify 升级 3.4.17。
- 新迁移 246 将已有 gemini/antigravity 分组 allow_image_generation 置 true（composite 不变），247 新增受限管理员密钥表；升级前由助手备份数据库，不把镜像回退等同于数据库恢复。
- 保留后台 Docker 在线升级和既有助手协议、自有发行源、搜索历史/压缩兼容。发布后核验产物和旧版后台更新发现，线上更新仍由用户点击。

## 0.2.21：同步作者主分支（2026-10-29）

- 合入作者 a3298b46e，包含作者 0.2.19 和 OAuth 非流式返回补充修复；自有版本独立递增。
- 新增 GPT-6.1 Terra 支持；非流式 Responses 保留客户端原始 stream 标志，处理缺失 Content-Type 的 SSE 及 incomplete/cancelled JSON 返回。保留自有搜索历史、身份处理和容量错误跨账号重试。
- 合入 Grok 模型目录同步及响应模型诊断、Gemini 同账号重试配置与模型列表修复、国产 Coding Plan 频率型 429 秒级冷却，以及图片计费、DeepSeek 缓存计费、工具及角色兼容修复。
- 迁移 248 新增 groups.image_count_billing，默认 false，不自动开启按张计费。宿主助手升级前备份数据库；镜像回退不等于数据库回滚。
- 后台 Docker 在线升级、宿主助手协议和自有发行源保持不变。发布后核验归档校验和、两种 Linux 镜像身份、实机候选镜像与后台可点击更新；生产切换由用户点击。
- 发布前安全扫描发现 GHSA-83rm-6c8r-g728；将 vue-i18n 和测试用 @intlify/message-compiler 从 9.14.5 同步升级到 11.4.16，并更新锁文件，不增加漏洞豁免。现有 Composition API、运行时 JIT 与中英文语言加载方式保持不变；重跑前端全量测试、构建和依赖审计。

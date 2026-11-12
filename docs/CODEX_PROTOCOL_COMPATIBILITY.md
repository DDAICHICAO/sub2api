# Codex 客户端协议兼容性

本页是客户端身份和会话转发的维护入口，记录 0.2.25 的行为及验证边界，供后续合并作者分支时回归检查。

## 参考基线

本次对照 OpenAI Codex `rust-v0.167.2`，提交 `ce21072c90d32c40af7a55b60b2fb35c84a43a6a`：

- `codex-rs/login/src/auth/default_client.rs`：客户端名称、构建版本、系统及终端组成 User-Agent。
- `codex-rs/codex-api/src/endpoint/responses.rs`：HTTP 的 `x-client-request-id` 使用线程 ID，并发送 session/thread headers。
- `codex-rs/core/src/client.rs`：WebSocket 保持上述关联；`build_responses_headers` 仅在已配置且非空时附加 beta features。

参考源：<https://github.com/openai/codex/tree/ce21072c90d32c40af7a55b60b2fb35c84a43a6a>。测试同时覆盖本机 CLI 使用的 0.156.1；不把自动同步版本当成所有客户端的实际版本。

## 0.2.25 行为变化

| 范围 | 之前 | 现在 |
| --- | --- | --- |
| 默认 OAuth 身份处理 | 真实 CLI/TUI/VS Code/Desktop 也被替换为统一 CLI 模板 | 保留受支持客户端的 UA、实际版本、系统与终端，按 UA 配对 originator/version |
| HTTP 会话头 | 白名单遗漏 `session-id`、`thread-id`、`x-client-request-id` | 普通转发和透传均接收这些头，并继续执行账号/用户隔离 |
| 请求关联 | 原本相等的 thread ID 和 request ID 被不同命名空间拆开 | 原始值相等时使用同一线程命名空间；原本不同的值仍独立处理 |
| 能力声明 | 客户端未声明 beta features 时也补入 remote compaction v2 | 受支持客户端未声明时保持缺省；显式声明保持原值，原生 compaction_trigger 仍执行必要协商 |

受支持身份通过既有客户端名称、版本格式及最低版本检查；这只是格式判断，不证明请求来自官方软件。最低版本沿用既有兼容规则。未知/非法/过旧身份的兜底、管理员账号 UA 覆写、`gateway.force_codex_cli` 的优先级，以及关闭身份强制配置后的历史行为保持兼容。

## 明确保留的网关行为

- 以选中的上游账号替换鉴权，保留跨用户、跨账号会话隔离和换号保护。
- 原有模型映射、正文协议转换、工具 ID 规范化、可选指纹收敛和传输选择仍由原配置决定。这些不是完全透明的字节转发。
- 自动同步 Codex 版本仅用于兼容兜底和既有管理探测等路径，不再把受支持客户端重新标成同步版本。
- 重试和模型冷却继续遵循 0.2.24 已对齐的作者策略；不因本次发布修改账号、代理或 MCP 配置。
- 本次未模拟 TLS、HTTP/2 指纹或生成官方客户端认证证明；协议修正不能保证消除上游过载、账号限流或模型路由差异。

## 回归验证

在 `backend` 下运行：

```sh
go test -tags=unit ./internal/service -run 'Codex|OpenAI.*(Headers|Identity|Passthrough|Compact)|AccountIdentity|BetaFeatures' -count=1
go test -tags=unit ./...
go build ./cmd/server
```

`openai_codex_protocol_conformance_test.go` 覆盖四类客户端 × HTTP/透传/compact/WS，实际 Forward 调用中的正文、推理参数与工具数据保留，以及版本同步不覆盖客户端实际构建。既有测试继续检查显式覆写、隔离命名空间、压缩协商及兼容桥接。

CI 还运行集成测试、前端类型/关键测试、lint、安全扫描和宿主升级助手测试。发布需验证平台归档校验和、双架构 OCI 版本/源码提交、宿主助手候选镜像和真实后台更新按钮。发布与生产升级分开：由用户在后台点击更新后，再验收真实账号的连续对话和错误率。

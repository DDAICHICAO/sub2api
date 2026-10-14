# Docker 后台一键更新

自有发行版 0.2.18 起，管理员版本面板可以提交镜像更新任务。需要在宿主机安装一次独立更新服务；旧容器不能仅靠安装宿主机服务获得新界面，首次还需按固定镜像升级到 0.2.18。

## 安装

在 Docker 宿主机以 root 运行本目录脚本，默认应用容器 `sub2api`、数据库容器 `sub2api-postgres`，本地应用端口 8080：

```bash
python3 deploy/docker-updater/install.py
systemctl status sub2api-updater
```

安装器读取运行容器的 Compose 标签和真实 `/app/data` 挂载。已有单个、root 所有的 JSON Compose 配置会继续原地维护；多文件/YAML 部署会生成 `/etc/sub2api-updater/compose.json` 有效配置快照。后一种部署从第一次在线更新起，必须继续使用这个受管配置，不可再用旧文件重建应用。安装本身不重启应用。配置写入 `/etc/sub2api-updater/config.json`，权限 0600；非默认端口使用 `--health-url` 和 `--settings-url`。

应用通过 `/app/data/.updater/control.sock` 请求宿主机服务，socket 仅允许 root 和应用组访问。没有公开 TCP 端口，也不向应用挂载 Docker socket。管理员接口只选择服务端查询到的最新版本，客户端不能指定镜像、宿主路径或命令。宿主服务再次校验固定仓库的正式 release、tag commit、OCI 标签和镜像二进制版本。

## 更新流程

1. 同一时间只运行一个任务；重复提交同一版本返回原任务。
2. 拉取并核对 `ghcr.io/ddaichicao/sub2api:<version>`。
3. 保存受管 Compose，执行 `pg_dump -Fc` 并用 `pg_restore --list` 校验。备份失败即停止。
4. 只修改 `services.sub2api.image`，使用原项目名、工作目录及配置执行 `up -d --no-deps --pull never sub2api`。
5. 核对运行镜像、本地健康接口、后台版本，以及环境变量、挂载、端口和其他容器 ID。
6. 失败时恢复旧 Compose/镜像并再次验证。**不会自动覆盖数据库**：新版本可能已执行迁移，镜像回退不等于数据库回退。

页面显示拉取、备份、切换、校验、恢复阶段。浏览器重连或页面刷新后从宿主服务读取持久任务状态。应用短暂重启不会终止宿主任务。更新服务本身重启时，未完成任务标记为 interrupted，保留任务 ID、配置及备份，需检查宿主状态；不会自行重复执行切换。

## 排错和恢复

```bash
journalctl -u sub2api-updater -n 100 --no-pager
cat /var/lib/sub2api-updater/status.json
```

备份保存在 `/var/lib/sub2api-updater/backups/<job-id>/`，含 `compose.json` 和 `database.dump`，权限仅 root 可读。保留与清理由宿主管理员管理。回退校验仍失败时，先检查容器日志和数据库迁移兼容性，再决定是否恢复数据库；不要直接覆盖运行中的数据库。

更新组件本身不随应用镜像更新。修改组件时保留配置和任务状态，替换 `/etc/sub2api-updater/updater.py` 后在没有运行任务时重启 service。

## 验证

```bash
python3 -m unittest discover -s deploy/docker-updater -p 'test_*.py'
cd backend && go test -tags=unit ./internal/service -run TestDockerUpdater
```

新宿主启用后还应验证真实管理员按钮、重连后的结果以及数据/挂载保持一致，不能把模拟测试视为生产更新已验收。

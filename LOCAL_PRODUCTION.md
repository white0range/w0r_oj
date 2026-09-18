# 生产 Compose 与 4GB 配置验证

这套文件既可用于本机模拟，也可用于小流量的 Linux 单机部署：Vue 会被构建成静态文件，Compose 网关是唯一绑定宿主机端口的容器，Go、Agent、MySQL、Redis、Elasticsearch 和 Qdrant 只在 Docker 私有网络中通信。Docker 镜像使用仓库内不含密钥的 `config/config.production.example.yaml`，不需要也不应复制真实的 `config.production.yaml` 到服务器。

## 第一次运行

1. 复制环境变量模板：

   ```powershell
   Copy-Item .env.local-production.example .env.local-production
   ```

2. 编辑 `.env.local-production`，把三项 Secret 和两个 MySQL 密码替换为本地随机值。三项 Secret 都必须至少 32 个字符；`DEEPSEEK_API_KEY` 可先留空，但 Chat 功能不可用。

3. 确认 Docker Desktop 正在运行并处于 Linux containers 模式。先填写
   `BOOTSTRAP_ADMIN_USERNAME` 和 `BOOTSTRAP_ADMIN_PASSWORD`，再运行一次管理员初始化：

   ```powershell
   docker compose --env-file .env.local-production -f docker-compose.prod.yml --profile tools run --rm bootstrap-admin
   ```

   成功后从 `.env.local-production` 清空 `BOOTSTRAP_ADMIN_PASSWORD`，然后启动全部服务：

   ```powershell
   docker compose --env-file .env.local-production -f docker-compose.prod.yml up -d --build
   ```

4. 等待 MySQL 和 Elasticsearch 变为健康状态，检查服务：

   ```powershell
   docker compose --env-file .env.local-production -f docker-compose.prod.yml ps
   Invoke-WebRequest http://localhost:8088/ping
   ```

5. 浏览器访问 `http://localhost:8088`。

## 常用命令

```powershell
# 查看 Go 后端日志
docker compose --env-file .env.local-production -f docker-compose.prod.yml logs -f api

# 只停止容器，保留数据库、Redis、ES、Qdrant 数据卷
docker compose --env-file .env.local-production -f docker-compose.prod.yml down

# 首次创建演示题目；该命令可重复执行
docker compose --env-file .env.local-production -f docker-compose.prod.yml --profile tools run --rm seed-problems
```

不要把 `down -v` 当作普通停机命令；它会删除本地数据库等 Docker Volume。

## 4GB 服务器配置

`docker-compose.4g.yml` 是叠加在生产 Compose 上的低内存配置，不可单独启动。它把 Judge 和 Chat 并发都限制为 1，收紧数据库连接池与各服务内存上限，同时保留 Elasticsearch、Qdrant、Agent 和完整判题功能。

```bash
docker compose --env-file .env.local-production \
  -f docker-compose.prod.yml \
  -f docker-compose.4g.yml \
  up -d --build
```

验证最终配置和运行状态：

```bash
docker compose --env-file .env.local-production \
  -f docker-compose.prod.yml \
  -f docker-compose.4g.yml \
  config --quiet

docker compose --env-file .env.local-production \
  -f docker-compose.prod.yml \
  -f docker-compose.4g.yml \
  ps

docker stats --no-stream
```

4GB 主机还应在宿主机上配置 2GB Swap，并把 `vm.swappiness` 设置为较低值（例如 10）。Swap 是瞬时峰值的保险，不是容器内存配额；创建前先用 `swapon --show` 检查服务器是否已经存在 Swap，避免重复配置。

## Judge 的 Docker Desktop 说明

当前 Judge 会让 Docker 守护进程把临时判题目录挂载到编译和运行容器。Linux 服务器可以稳定使用默认的 `/tmp/gojo-judge`。在 Windows 的 Docker Desktop 上，如果提交时报“bind source path does not exist”，请先不要绕过隔离或关闭判题限制；这是 Docker Desktop 的宿主机路径转换限制。

此时可先验证登录、题目、搜索和 Chat；完整 Judge 验证应在 Linux/WSL2 Docker 环境完成，或下一步将 Judge 改为 Docker Volume 传递源码与编译产物，使其完全不依赖宿主机路径。

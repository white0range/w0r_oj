# 本机生产模式验证

这套文件模拟服务器上的生产运行方式：Vue 会被构建成静态文件，Nginx 是唯一对本机开放的入口，Go、Agent、MySQL、Redis、Elasticsearch 和 Qdrant 只在 Docker 私有网络中通信。Docker 镜像使用仓库内不含密钥的 `config/config.production.example.yaml`，不需要也不应复制真实的 `config.production.yaml` 到服务器。

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

## Judge 的 Docker Desktop 说明

当前 Judge 会让 Docker 守护进程把临时判题目录挂载到编译和运行容器。Linux 服务器可以稳定使用默认的 `/tmp/gojo-judge`。在 Windows 的 Docker Desktop 上，如果提交时报“bind source path does not exist”，请先不要绕过隔离或关闭判题限制；这是 Docker Desktop 的宿主机路径转换限制。

此时可先验证登录、题目、搜索和 Chat；完整 Judge 验证应在 Linux/WSL2 Docker 环境完成，或下一步将 Judge 改为 Docker Volume 传递源码与编译产物，使其完全不依赖宿主机路径。

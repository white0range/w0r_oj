# 生产部署与运维

本指南适用于在单台 Linux 主机上使用 Docker Compose v2 部署 w0r_oj。React 静态文件由 Nginx 网关提供，Go API、Judge Worker、Python Agent、MySQL、Redis、Elasticsearch 和 Qdrant 运行在私有 Compose 网络中。

## Compose 文件

| 文件 | 用途 |
| --- | --- |
| [docker-compose.yml](../docker-compose.yml) | 本地开发。启动 Vite 前端开发服务和基础依赖，应用进程在宿主机运行 |
| [docker-compose.prod.yml](../docker-compose.prod.yml) | 完整的单机生产服务定义，可独立使用 |
| [docker-compose.4g.yml](../docker-compose.4g.yml) | 按服务名覆盖生产资源配置，适用于 4 GiB 主机 |

下面的命令以 4 GiB 主机为例。该配置将 Chat Worker 与正式判题各设为一个并发槽，并限制应用和中间件容器的内存。试跑槽在生产配置下默认开启。编译和运行沙箱由宿主机 Docker 单独创建，容量核算需计入沙箱在并行执行正式判题和试跑时的占用。

## 准备配置

在仓库根目录创建私有环境文件。

```bash
test -f .env.local-production || cp .env.local-production.example .env.local-production
stat -c '%g' /var/run/docker.sock
```

在 `.env.local-production` 填写以下配置。生产镜像内置 [Go 生产模板](../config/config.production.example.yaml)，Compose 将密钥和连接参数注入容器。

| 配置 | 要求 |
| --- | --- |
| `MYSQL_DATABASE`、`MYSQL_USER`、`MYSQL_PASSWORD`、`MYSQL_ROOT_PASSWORD` | 生产数据库名和账号密码 |
| `GOJO_SQL_DSN` | 与上述业务账号、数据库及密码一致，主机使用 `mysql:3306` |
| `GOJO_JWT_SECRET`、`REDIS_PASSWORD`、`AGENT_SERVICE_TOKEN` | 分别生成长度至少 32 字符的随机密钥，例如每项单独执行一次 `openssl rand -hex 32` |
| `DEEPSEEK_API_KEY`、`DASHSCOPE_API_KEY` | 对话模型和 Embedding API 的密钥 |
| `DOCKER_GID` | 宿主机 Docker Socket 的组 ID，即上面 `stat` 返回的值 |
| `JUDGE_WORK_DIR` | Worker 可写的宿主机绝对路径，默认为 `/tmp/gojo-judge` |
| `GOJO_JUDGE_TRIAL_ENABLED` | 试跑默认开启。设置为 `false` 可释放专用试跑槽 |

`AGENT_SERVICE_TOKEN` 由 Compose 注入 Go API 和 Python Agent。私有环境文件和密钥只保存在部署端。Go API 和 Agent 通过内网服务名通信，`GO_BACKEND_BASE_URL` 使用 `http://api:8080`。

## 首次启动

在同一个 Shell 中定义下面的命令。使用标准生产配置时，从函数中去掉 `-f docker-compose.4g.yml`。

```bash
gojo_compose() {
  docker compose --env-file .env.local-production \
    -f docker-compose.prod.yml -f docker-compose.4g.yml "$@"
}

gojo_compose config --quiet
docker pull golang:alpine
docker build -f Dockerfile.judge-compiler -t gojo-judge-compiler:go1.26-v1 .
gojo_compose build
```

Judge Worker 以镜像内的非 root 用户运行。下例为默认路径配置写权限，使用自定义 `JUDGE_WORK_DIR` 时替换为环境文件中的绝对路径。

```bash
judge_uid="$(gojo_compose run --rm --no-deps -T --entrypoint id judge-worker -u)"
sudo install -d -m 0750 -o "$judge_uid" /tmp/gojo-judge
gojo_compose up -d
gojo_compose ps
curl -fsS http://127.0.0.1:8088/ping
```

`/ping` 用于确认网关和 Go API 可达。`LOCAL_HTTP_PORT` 如有修改，检查时使用对应端口。首次初始化时 Go API 会创建业务表。

按需在环境文件中设置 `BOOTSTRAP_ADMIN_USERNAME` 和 `BOOTSTRAP_ADMIN_PASSWORD`，然后执行初始化。管理员用户名为 3–32 个字符，密码为 12–72 字节。初始化后清除文件里的初始密码。

```bash
gojo_compose --profile tools run --rm bootstrap-admin
```

测试部署若需要示例题，可运行 `gojo_compose --profile tools run --rm seed-problems`。

## 公网入口与 HTTPS

Compose 网关默认绑定 `127.0.0.1:8088`。在宿主机用 Nginx 或 Caddy 为站点域名配置 HTTPS，反向代理到该地址。生产登录会签发带 `Secure` 属性的 Refresh Cookie，浏览器通过 HTTPS 使用该 Cookie。代理应保留站点 Host 和请求来源信息，并为 `/api/` 下的 SSE 请求关闭响应缓冲。生产防火墙仅开放站点与运维所需的入口，数据库、队列、Agent API 和 Docker Socket 通过内部边界访问。

Go 的 `server.trusted_proxy_cidrs` 默认是空列表。处于反向代理后时，IP 限流可能按网关来源合计。需要按访客区分时，依据实际网络设置可信代理范围，并核对宿主机代理、Compose 网关和 Go 三段的转发头。

## 更新

在更新后的仓库版本上检查 Compose 配置，再构建和替换服务。

```bash
gojo_compose config --quiet
docker image inspect gojo-judge-compiler:go1.26-v1 >/dev/null 2>&1 || \
  docker build -f Dockerfile.judge-compiler -t gojo-judge-compiler:go1.26-v1 .
gojo_compose up -d --build
gojo_compose ps
```

Go API、Judge Worker 和 Python Agent 的配合以同一仓库版本为准。更新前备份业务数据库。MySQL、Redis、Elasticsearch 和 Qdrant 使用命名数据卷。站点路由由网关镜像中的 [Nginx 配置](../nginx/default.conf)提供，配置更新时需重建 `gateway`。

### GitHub Actions 自动部署

[发布工作流](../.github/workflows/deploy.yml)在 `main` 更新时运行 Go、Python 测试和 React 前端构建。检查通过且仓库变量 `GOJO_DEPLOY_ENABLED` 为 `true` 时，才以 `gojo-deploy` 用户连接腾讯云并发送本次测试通过的提交号。也可以在 GitHub Actions 手动运行同一工作流，部署仍限定在 `main`。

[部署脚本](../ops/gojo-deploy.sh)针对现有腾讯云部署，使用 `/home/ubuntu/w0r_oj`、`gojo-local-production` 和现有生产环境文件。它用锁串行部署，确认目标仍为远端 `main` 最新提交，逐个构建镜像，更新服务后通过网关检查 `/ping`。检查成功才更新 `/home/ubuntu/.local/state/gojo-deploy/deployed-sha`。

#### 服务器端一次性设置

由服务器管理员在审阅脚本后执行。为 Actions 创建 `gojo-deploy` 登录用户，交由现有的 `ubuntu` 用户执行固定脚本。安装目录及脚本由 `root` 管理。安装前确认生产检出目录位于干净的 `main` 分支，并确认 `ubuntu` 可以拉取 `origin` 和更新现有 Compose 项目。

```bash
sudo adduser --disabled-password --gecos '' gojo-deploy
sudo install -o root -g root -m 0755 ops/gojo-deploy.sh /usr/local/sbin/gojo-deploy
sudo install -d -o gojo-deploy -g gojo-deploy -m 0700 /home/gojo-deploy/.ssh
sudo visudo -f /etc/sudoers.d/gojo-deploy
```

在 `visudo` 中加入这一行。末尾的 `""` 将匹配范围限定为无命令行参数的调用。

```sudoers
gojo-deploy ALL=(ubuntu) NOPASSWD: /usr/local/sbin/gojo-deploy ""
```

已有自动部署的服务器可在审阅新版脚本后重新执行上面的 `sudo install` 命令。新版脚本会复用本机已有的判题编译镜像，缺少时在更新服务前构建。调整编译镜像的 Dockerfile 时，更新 Worker 与部署命令中的镜像版本标签。宿主机上暂时缺少预热镜像时，Judge Worker 使用 `golang:alpine` 冷编译。

在自己的电脑上生成 Actions 专用密钥。工作流使用此密钥进行非交互登录。

```bash
ssh-keygen -t ed25519 -f ~/.ssh/gojo_actions_deploy -C gojo-actions-deploy -N ''
```

用 `sudoedit /home/gojo-deploy/.ssh/authorized_keys` 写入下面这一行，把 `ssh-ed25519 ...` 换成 `.pub` 文件的完整一行。将文件权限设为 `0600`，所有者设为 `gojo-deploy`。给这个用户保留这一条带 `restrict` 和强制命令的公钥，原有 `ubuntu` 登录继续由管理员维护。

```text
restrict,command="/usr/local/sbin/gojo-deploy --ssh" ssh-ed25519 ...
```

这条 SSH 密钥只能调用固定部署命令，`gojo-deploy` 无需加入 Docker 组。服务器管理员仍需信任能进入 `main` 的部署代码。w0r_oj 的 Judge Worker 使用宿主机 Docker Socket，同机项目之间的运行权限由服务器管理员负责隔离。

#### GitHub 端配置

在仓库的 **Settings → Environments** 中创建 `production` 环境，将可部署分支限制在 `main`，并添加以下环境 Secrets。

| Secret | 内容 |
| --- | --- |
| `DEPLOY_HOST` | 服务器公网地址，使用 SSH 的 TCP 22 端口 |
| `DEPLOY_SSH_KEY` | 专用私钥 `gojo_actions_deploy` 的完整内容 |
| `DEPLOY_KNOWN_HOSTS` | 从服务器控制台核验的主机公钥记录。格式为 `服务器地址 ssh-ed25519 公钥内容`，地址与 `DEPLOY_HOST` 一致 |

主机公钥可通过已登录的腾讯云控制台读取，例如 `cat /etc/ssh/ssh_host_ed25519_key.pub`。生产环境文件继续保存在部署目录。配置完成后，在 **Settings → Secrets and variables → Actions → Variables** 添加仓库变量 `GOJO_DEPLOY_ENABLED=true`，在 **Actions** 手动运行一次工作流验证。随后合并或推送到 `main` 会自动部署。

部署失败时，Actions 会保留失败步骤日志，服务器上的 `deployed-sha` 表示最近一次探活通过的版本。探活失败时容器可能已经替换，检查生产日志后，可通过在 `main` 上回退问题提交来重新部署。

## 日志与排障

Go 和 Python 日志输出到标准输出，由 Docker 收集。4 GiB 配置使用 `json-file`，每个文件最大 10 MB，每个容器保留最多 3 个日志文件。Agent 事件使用 `turn_id` 关联，模型回调可记录耗时、错误类型和模型返回的 Token 用量。回合重试可通过日志中的 `attempt_id` 区分。

```bash
gojo_compose ps
docker stats --no-stream
gojo_compose logs --tail 100 api judge-worker agent
gojo_compose logs --no-color api agent judge-worker 2>&1 | rg 'turn_id=87([^0-9]|$)'
```

| 现象 | 检查项 |
| --- | --- |
| `/ping` 无法访问 | 查看 `gateway` 和 `api` 状态，检查端口和依赖服务是否已就绪 |
| 判题积压，或出现 `SE` | 查看 `judge-worker` 日志，检查 Docker Socket 组权限、`golang:alpine` 镜像和工作目录写权限 |
| 判题或试跑编译慢 | 用 `docker image inspect gojo-judge-compiler:go1.26-v1` 检查预热镜像是否已构建 |
| Agent 请求返回 401 | 核对 Go 与 Python 的服务密钥，确认 Python 用内网 API 地址访问工具 |
| 对话停在等待或处理中 | 检查 API、Agent 日志及 `chat_turn_queue`、`chat_turn_processing` 队列，按 `turn_id` 核对执行状态 |
| 搜索或向量索引更新缓慢 | 查看 Elasticsearch、Agent 和 Qdrant 日志，检查 `sync:pending`、`sync:processing`、`sync:retry_at` 和 `sync:dead_letter` 的积压 |

处理同步死信时先修复下游故障，再针对相应任务重放。记忆同步任务还需保留其回合分发标记，避免绕过重试去重。

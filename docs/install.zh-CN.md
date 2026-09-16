# 安装 ContextGate

[English](install.md) · [发行版](https://github.com/SamuelSupe/contextGate/releases/tag/v0.8.0)

**ContextGate 0.8.0** 使用 `contextgate` 主命令，并保留 `mcpdbhub` 别名。

## 升级到 0.8.0

从 **0.6.x 或 0.7.x** 升级时，备份 PostgreSQL 元数据库和匹配主密钥，停止服务后更换程序，保留原库、密钥和部署配置。本版本不新增元数据迁移，也不强制轮换 Token。详见 [0.8.0 升级说明](releases/0.8.0.zh-CN.md#兼容与升级)。

从 0.4.x/0.5.x PostgreSQL 部署升级时，先备份元数据库及匹配主密钥，更换程序后继续使用原库和密钥。以 `admin` 和原密码重新登录；所有旧配置 MCP Token 失效，每名管理员需重新签发个人 Token。数据源、语义、本体及查询 Agent/OAuth 授权保留。具体步骤见[管理员升级清单](administrators.zh-CN.md)。

## PostgreSQL 元数据

0.8.0 必须配置 `MCPDBHUB_DATABASE_URL` 并预先创建 PostgreSQL 数据库；账号需要建表及读写权限。Compose 可自动创建专用数据库。`--data-dir` 仅保存独立加密主密钥，元数据由 PostgreSQL 保存。

**从 0.3.0 或更早版本升级：** 不提供 SQLite 元数据导入或兼容后端，需要重新初始化管理员、配置数据源和授权。保留旧数据库及主密钥备份，使用全新的 PG 数据库。SQLite 查询数据源仍支持。详见[升级说明](releases/0.4.0.md#upgrading-from-03x-or-earlier)。

## Linux 发行包

支持 Linux arm64 / amd64，要求 glibc 2.36 或更新版本，例如 Debian 12、Ubuntu 24.04。发行包自带所需 C++ 运行库和内嵌管理 UI，无需 Go、Node.js 或数据库客户端。Alpine/musl、macOS、Windows 不能直接运行这些 Linux 包；macOS 可通过 Docker Desktop 或 OrbStack 使用容器。

根据 `uname -m` 选择：`x86_64` 下载 `linux-amd64`，`aarch64` / `arm64` 下载 `linux-arm64`。

```sh
# 示例：Linux arm64。amd64 用户替换文件名中的 arm64。
curl -fLO https://github.com/SamuelSupe/contextGate/releases/download/v0.8.0/contextgate-0.8.0-linux-arm64.tar.gz
curl -fLO https://github.com/SamuelSupe/contextGate/releases/download/v0.8.0/SHA256SUMS
sha256sum --ignore-missing -c SHA256SUMS
tar -xzf contextgate-0.8.0-linux-arm64.tar.gz
cd contextgate-0.8.0-linux-arm64
./contextgate version
mkdir -p data databases
# Use a pre-created PostgreSQL database and its dedicated owner role.
export MCPDBHUB_DATABASE_URL='postgres://contextgate@127.0.0.1:5432/contextgate?sslmode=disable'
# Read the database password without echoing it (Bash or Zsh).
read -r -s PGPASSWORD
export PGPASSWORD
./contextgate serve --data-dir ./data --database-dir ./databases
```

保留解压后的完整目录：根目录 `contextgate` 启动器设置私有运行库路径，再运行 `libexec/contextgate`。不要只复制其中一个文件。启动器保留当前工作目录、参数和信号传递。

打开 `http://127.0.0.1:8080`，使用服务日志中的一次性设置码创建首个超级管理员的用户名和密码。按以下流程配置：

1. **Data sources → Add data source**：选择产品、填写连接与数据库读取账号，配置 TLS 和执行上限。
2. 保存并检查连接与只读保护证据。`Not verified` 不等同于已验证账号权限；InfluxDB 3 Core 明确采用查询 API 隔离。
3. **Agents → Create Agent**：选择可访问的数据源和到期时间。保存唯一一次显示的 Token。
4. **Connect** 中复制 HTTP 或 stdio 配置。包内 `examples/` 提供占位示例；先调用 `list_data_sources`，再执行相应查询工具。
5. 在 **Audit log** 查看调用结果，用请求 ID 关联脱敏错误。撤销 Token 后更新客户端，不会恢复旧凭证。

## 从源码运行 Docker

```sh
git clone https://github.com/SamuelSupe/contextGate.git
cd contextGate
mkdir -p databases
umask 077
if [ ! -e .env ]; then
  printf 'MCPDBHUB_POSTGRES_PASSWORD=%s\n' "$(openssl rand -hex 24)" > .env
fi
docker compose up --build -d
docker compose logs hub
```

默认只监听宿主机回环端口；数据库文件需允许容器 UID 10001 读取，数据库目录只读挂载。PostgreSQL 元数据保存在 `hub-postgres`，独立密钥保存在 `hub-data`。只在首次安装生成 `.env`，重启保留已有密码。不要执行 `docker compose down -v` 来升级。

## 远程部署、持久化与升级

远程部署应在 HTTPS 反向代理后运行，设置 `MCPDBHUB_PUBLIC_URL=https://db.example.com`，并保留原 Host 与 Authorization 请求头；OAuth 的资源地址为 `https://db.example.com/mcp`。二进制服务可通过 `--listen 0.0.0.0:8080` 监听容器或内网接口。完整配置变量见 [README](../README.zh-CN.md)。

当前 PG 存储请停止 ContextGate 后用 `pg_dump` / `pg_restore` 备份和恢复元数据库。将匹配的 `master.key` 放在程序目录外，单独备份并保护；如使用 `MCPDBHUB_MASTER_KEY`，另行备份该外部密钥。恢复时密钥必须与配置匹配。

升级前保留旧程序和停止状态下的配置备份，新版本使用同一 `MCPDBHUB_DATABASE_URL` 和密钥目录启动。只有从 0.3.x 或更早版本升级才需要新建 PostgreSQL 存储并重新配置；现有 PostgreSQL 部署保留元数据。不要删除主密钥、配置库或数据卷。数据库迁移后如需回退，应恢复与旧程序配套的配置备份。

忘记管理员密码时，保留同一 `MCPDBHUB_DATABASE_URL` 和主密钥，停止服务，通过 stdin 执行 `./contextgate reset-password --data-dir /原配置路径 --username admin --password-stdin`；具体无回显命令见 [恢复说明](../README.zh-CN.md#管理员忘记密码后的恢复)。原数据源与查询 Agent Token 保留，指定管理员的会话及配置 Token 失效。

## 运行故障

| 现象 | 排查 |
|---|---|
| `Exec format error` | 选择与主机 CPU/系统对应的 Linux 发行包 |
| glibc 版本错误 | 使用 glibc ≥ 2.36 的系统或从源码构建 Docker 镜像 |
| 运行库缺失 | 保留完整目录，使用根目录 `./contextgate` 启动器 |
| 数据源连接失败 | 检查网络、TLS、数据库读取账号和 UI 中的脱敏诊断 |
| Agent 无法发现数据源 | 检查授权、启停、到期时间、撤销状态及数据源启用状态 |
| `query_denied` | 查询超出只读子集；参阅对应产品的[限制](support-matrix.zh-CN.md) |
| 游标无效 | 使用相同身份、参数和上限；过期、升级或授权改变后重新开始查询 |

版本、二进制摘要与源码提交记录在包内 `BUILD.json`。已执行的产品/平台验收与剩余边界见 [验收说明](validation.zh-CN.md)。

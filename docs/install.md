# 安装 v0.1.0

[English](install.en.md) · [发行版](https://github.com/SamuelSupe/mcpdbhub/releases/tag/v0.1.0)

## Linux 发行包

支持 Linux arm64 / amd64，要求 glibc 2.36 或更新版本，例如 Debian 12、Ubuntu 24.04。发行包自带所需 C++ 运行库和内嵌管理 UI，无需 Go、Node.js 或数据库客户端。Alpine/musl、macOS、Windows 不能直接运行这些 Linux 包；macOS 可通过 Docker Desktop 或 OrbStack 使用容器。

根据 `uname -m` 选择：`x86_64` 下载 `linux-amd64`，`aarch64` / `arm64` 下载 `linux-arm64`。

```sh
# 示例：Linux arm64。amd64 用户替换文件名中的 arm64。
curl -fLO https://github.com/SamuelSupe/mcpdbhub/releases/download/v0.1.0/mcpdbhub-0.1.0-linux-arm64.tar.gz
curl -fLO https://github.com/SamuelSupe/mcpdbhub/releases/download/v0.1.0/SHA256SUMS
sha256sum --ignore-missing -c SHA256SUMS
tar -xzf mcpdbhub-0.1.0-linux-arm64.tar.gz
cd mcpdbhub-0.1.0-linux-arm64
./mcpdbhub version
mkdir -p data databases
./mcpdbhub serve --data-dir ./data --database-dir ./databases
```

保留解压后的完整目录：根目录 `mcpdbhub` 启动器设置私有运行库路径，再运行 `libexec/mcpdbhub`。不要只复制其中一个文件。启动器保留当前工作目录、参数和信号传递。

打开 `http://127.0.0.1:8080`，使用服务日志中的一次性设置码创建管理员密码。按以下流程配置：

1. **Data sources → Add data source**：选择产品、填写连接与数据库读取账号，配置 TLS 和执行上限。
2. 保存并检查连接与只读保护证据。`Not verified` 不等同于已验证账号权限；InfluxDB 3 Core 明确采用查询 API 隔离。
3. **Agents → Create Agent**：选择可访问的数据源和到期时间。保存唯一一次显示的 Token。
4. **Connect** 中复制 HTTP 或 stdio 配置。包内 `examples/` 提供占位示例；先调用 `list_data_sources`，再执行相应查询工具。
5. 在 **Audit log** 查看调用结果，用请求 ID 关联脱敏错误。撤销 Token 后更新客户端，不会恢复旧凭证。

## 从源码运行 Docker

```sh
git clone --branch v0.1.0 https://github.com/SamuelSupe/mcpdbhub.git
cd mcpdbhub
mkdir -p databases
docker compose up --build -d
docker compose logs hub
```

默认只监听宿主机回环端口；数据库文件需允许容器 UID 10001 读取，数据库目录只读挂载。配置保存在 `hub-data` 卷中。不要执行 `docker compose down -v` 来升级。

## 远程部署、持久化与升级

远程部署应在 HTTPS 反向代理后运行，设置 `MCPDBHUB_PUBLIC_URL=https://db.example.com`，并保留原 Host 与 Authorization 请求头；OAuth 的资源地址为 `https://db.example.com/mcp`。二进制服务可通过 `--listen 0.0.0.0:8080` 监听容器或内网接口。完整配置变量见 [README](../README.md)。

将配置目录放在发行包目录外，以便替换程序时保留配置。备份前停止服务，复制配置 SQLite 与匹配的 `master.key`，单独保护密钥；如使用 `MCPDBHUB_MASTER_KEY`，另行备份该外部密钥。恢复时密钥必须与配置匹配。

升级前保留旧程序和停止状态下的配置备份，新版本使用原配置路径启动。不要删除主密钥、配置库或数据卷。数据库迁移后如需回退，应恢复与旧程序配套的配置备份。

忘记管理员密码时，停止服务，通过 stdin 执行 `./mcpdbhub reset-password --data-dir /原配置路径 --password-stdin`；具体无回显命令见 [恢复说明](../README.md#管理员忘记密码后的恢复)。原数据源与 Agent Token 保留，管理员会话失效。

## 运行故障

| 现象 | 排查 |
|---|---|
| `Exec format error` | 选择与主机 CPU/系统对应的 Linux 发行包 |
| glibc 版本错误 | 使用 glibc ≥ 2.36 的系统或从源码构建 Docker 镜像 |
| 运行库缺失 | 保留完整目录，使用根目录 `./mcpdbhub` 启动器 |
| 数据源连接失败 | 检查网络、TLS、数据库读取账号和 UI 中的脱敏诊断 |
| Agent 无法发现数据源 | 检查授权、启停、到期时间、撤销状态及数据源启用状态 |
| `query_denied` | 查询超出只读子集；参阅对应产品的[限制](support-matrix.md) |
| 游标无效 | 使用相同身份、参数和上限；过期、升级或授权改变后重新开始查询 |

版本、二进制摘要与源码提交记录在包内 `BUILD.json`。已执行的产品/平台验收与剩余边界见 [验收说明](validation.md)。

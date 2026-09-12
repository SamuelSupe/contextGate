# Install v0.3.0

[简体中文](install.zh-CN.md) · [Release](https://github.com/SamuelSupe/mcpdbhub/releases/tag/v0.3.0)

## Linux distributions

Packages target Linux arm64 and amd64 with glibc 2.36 or newer, such as Debian 12 or Ubuntu 24.04. They include private C++ runtime libraries and the embedded UI. Go, Node.js and database client programs are unnecessary at runtime. These Linux packages do not run directly on Alpine/musl, macOS or Windows. On macOS, use Docker Desktop or OrbStack.

Check `uname -m`: choose `linux-amd64` for `x86_64`, or `linux-arm64` for `aarch64` / `arm64`.

```sh
# Linux arm64 example; replace arm64 with amd64 for x86_64.
curl -fLO https://github.com/SamuelSupe/mcpdbhub/releases/download/v0.3.0/mcpdbhub-0.3.0-linux-arm64.tar.gz
curl -fLO https://github.com/SamuelSupe/mcpdbhub/releases/download/v0.3.0/SHA256SUMS
sha256sum --ignore-missing -c SHA256SUMS
tar -xzf mcpdbhub-0.3.0-linux-arm64.tar.gz
cd mcpdbhub-0.3.0-linux-arm64
./mcpdbhub version
mkdir -p data databases
./mcpdbhub serve --data-dir ./data --database-dir ./databases
```

Keep the entire extracted directory together. The root `mcpdbhub` launcher sets the private library path and executes `libexec/mcpdbhub`; copying either file alone is insufficient. It preserves the working directory, arguments and process signals.

Open `http://127.0.0.1:8080` and enter the one-time setup code from the server log to create the administrator password.

1. In **Data sources → Add data source**, choose a product, enter its connection and database reader credentials, then configure TLS and query limits.
2. Save and inspect connectivity and read-only evidence. `Not verified` is not proof of account privileges. InfluxDB 3 Core explicitly uses query API isolation.
3. In **Agents → Create Agent**, select permitted data sources and an expiration. Save the token displayed once.
4. Open **Connect** for HTTP or stdio configuration. The package includes placeholder files in `examples/`. Start with `list_data_sources`, then use the matching native query tool.
5. Inspect calls in **Audit log** and correlate redacted failures by request ID. Revoked credentials cannot be restored; update the client when issuing a replacement.

## Docker from source

```sh
git clone --branch v0.3.0 https://github.com/SamuelSupe/mcpdbhub.git
cd mcpdbhub
mkdir -p databases
docker compose up --build -d
docker compose logs hub
```

The host port binds to loopback by default. Database files must be readable by container UID 10001; their directory is mounted read-only. Configuration persists in the `hub-data` volume. Do not use `docker compose down -v` when upgrading.

## Remote access, backups and upgrades

For remote deployment, place the service behind HTTPS and set `MCPDBHUB_PUBLIC_URL=https://db.example.com`. Preserve the public Host and Authorization headers. The OAuth resource is `https://db.example.com/mcp`. Use `--listen 0.0.0.0:8080` when the binary must listen on a container or private network interface. See the [README](../README.md) for all configuration variables.

Keep configuration outside the extracted program directory. Stop the service before backing up its SQLite configuration and matching `master.key`; protect the key separately. If using `MCPDBHUB_MASTER_KEY`, back up that external key too. Restoring configuration requires the matching key.

Before upgrading, preserve the old program and a stopped-service configuration backup. Start the new program with the existing configuration path. Never delete the key, database or volume to upgrade. If a migration has run, rollback requires restoring the configuration backup matching the old program.

For password recovery, stop the service and supply a new password through stdin to `./mcpdbhub reset-password --data-dir /existing/config --password-stdin`. The [recovery instructions](../README.md#recover-a-forgotten-administrator-password) show a prompt that does not echo input. Recovery preserves data sources and Agent credentials while invalidating administrator sessions.

## Troubleshooting

| Symptom | Check |
|---|---|
| `Exec format error` | Select the package matching the operating system and CPU |
| glibc version error | Use glibc ≥ 2.36 or build the Docker image from source |
| Missing runtime library | Retain the complete package and use the root `./mcpdbhub` launcher |
| Connection failure | Check network, TLS, reader credentials and the redacted UI diagnostic |
| No visible sources | Check grants, expiration, revocation, paused Agents and disabled sources |
| `query_denied` | The query exceeds the supported read-only subset; see [limits](support-matrix.md) |
| Invalid cursor | Keep identity, parameters and limits identical; restart after expiration, upgrade or access changes |

`BUILD.json` identifies the version, binary digest and source commit. See [validation](validation.md) for tested products/platforms and remaining limitations.

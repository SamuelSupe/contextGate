# Install ContextGate

[简体中文](install.zh-CN.md) · [Release](https://github.com/SamuelSupe/contextGate/releases/tag/v0.6.0)

**ContextGate 0.6.0** uses the `contextgate` executable and retains `mcpdbhub` as an alias.

## Upgrade to 0.6.0

For an existing 0.4.x/0.5.x PostgreSQL installation, back up the metadata database and matching master key, then retain both when replacing the application. Sign in again as `admin` with the existing password. All legacy Configuration MCP tokens are revoked; each administrator must issue a new personal token. Sources, semantics, ontologies and query Agent/OAuth grants remain intact. Follow the [administrator upgrade checklist](administrators.md#upgrade-from-050-or-an-earlier-postgresql-release).

## PostgreSQL metadata

0.6.0 requires `MCPDBHUB_DATABASE_URL` pointing to a pre-created PostgreSQL database. Its dedicated owner role needs schema/table creation and read/write privileges. The Docker Compose installation below provisions this database for you. `--data-dir` stores the independent encryption key; PostgreSQL stores configuration and audit records.

**Upgrading from 0.3.0 or earlier:** there is no SQLite metadata import or fallback. Initialize a fresh PostgreSQL store, administrator, data sources and grants. Preserve the old database and key backup; do not point the new service at old metadata. SQLite remains a read-only query data source. See the [upgrade checklist](releases/0.4.0.md#upgrading-from-03x-or-earlier).

## Linux distributions

Packages target Linux arm64 and amd64 with glibc 2.36 or newer, such as Debian 12 or Ubuntu 24.04. They include private C++ runtime libraries and the embedded UI. Go, Node.js and database client programs are unnecessary at runtime. These Linux packages do not run directly on Alpine/musl, macOS or Windows. On macOS, use Docker Desktop or OrbStack.

Check `uname -m`: choose `linux-amd64` for `x86_64`, or `linux-arm64` for `aarch64` / `arm64`.

```sh
# Linux arm64 example; replace arm64 with amd64 for x86_64.
curl -fLO https://github.com/SamuelSupe/contextGate/releases/download/v0.6.0/contextgate-0.6.0-linux-arm64.tar.gz
curl -fLO https://github.com/SamuelSupe/contextGate/releases/download/v0.6.0/SHA256SUMS
sha256sum --ignore-missing -c SHA256SUMS
tar -xzf contextgate-0.6.0-linux-arm64.tar.gz
cd contextgate-0.6.0-linux-arm64
./contextgate version
mkdir -p data databases
# Use a pre-created PostgreSQL database and its dedicated owner role.
export MCPDBHUB_DATABASE_URL='postgres://contextgate@127.0.0.1:5432/contextgate?sslmode=disable'
# Read the database password without echoing it (Bash or Zsh).
read -r -s PGPASSWORD
export PGPASSWORD
./contextgate serve --data-dir ./data --database-dir ./databases
```

Keep the entire extracted directory together. The root `contextgate` launcher sets the private library path and executes `libexec/contextgate`; copying either file alone is insufficient. It preserves the working directory, arguments and process signals.

Open `http://127.0.0.1:8080` and enter the one-time setup code from the server log to create the first super administrator username and password.

1. In **Data sources → Add data source**, choose a product, enter its connection and database reader credentials, then configure TLS and query limits.
2. Save and inspect connectivity and read-only evidence. `Not verified` is not proof of account privileges. InfluxDB 3 Core explicitly uses query API isolation.
3. In **Agents → Create Agent**, select permitted data sources and an expiration. Save the token displayed once.
4. Open **Connect** for HTTP or stdio configuration. The package includes placeholder files in `examples/`. Start with `list_data_sources`, then use the matching native query tool.
5. Inspect calls in **Audit log** and correlate redacted failures by request ID. Revoked credentials cannot be restored; update the client when issuing a replacement.

## Docker from source

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

The host port binds to loopback by default. Database files must be readable by container UID 10001; their directory is mounted read-only. PostgreSQL metadata persists in `hub-postgres`; the independent key is in `hub-data`. Generate `.env` only for a new installation; retain the existing password on restart. Do not use `docker compose down -v` when upgrading.

## Remote access, backups and upgrades

For remote deployment, place the service behind HTTPS and set `MCPDBHUB_PUBLIC_URL=https://db.example.com`. Preserve the public Host and Authorization headers. The OAuth resource is `https://db.example.com/mcp`. Use `--listen 0.0.0.0:8080` when the binary must listen on a container or private network interface. See the [README](../README.md) for all configuration variables.

For current PostgreSQL storage, stop ContextGate and use `pg_dump` / `pg_restore` for the metadata database. Keep its matching `master.key` outside the program directory and protect it separately. If using `MCPDBHUB_MASTER_KEY`, back up that external key too. Restoring configuration requires the matching key.

Before upgrading, preserve the old program and a stopped-service configuration backup. Start the new program with the same `MCPDBHUB_DATABASE_URL` and key directory. Only upgrades from 0.3.x or earlier require a fresh PostgreSQL store and reconfiguration; current PostgreSQL installations retain their metadata. Never delete the key, database or volume to upgrade. If a migration has run, rollback requires restoring the configuration backup matching the old program.

For password recovery, keep the same `MCPDBHUB_DATABASE_URL` and master key, stop the service and supply a new password through stdin to `./contextgate reset-password --data-dir /existing/config --username admin --password-stdin`. The [recovery instructions](../README.md#recover-a-forgotten-administrator-password) show a prompt that does not echo input. Recovery preserves data sources and query Agent credentials while invalidating only the selected administrator’s sessions and Configuration MCP token.

## Troubleshooting

| Symptom | Check |
|---|---|
| `Exec format error` | Select the package matching the operating system and CPU |
| glibc version error | Use glibc ≥ 2.36 or build the Docker image from source |
| Missing runtime library | Retain the complete package and use the root `./contextgate` launcher |
| Connection failure | Check network, TLS, reader credentials and the redacted UI diagnostic |
| No visible sources | Check grants, expiration, revocation, paused Agents and disabled sources |
| `query_denied` | The query exceeds the supported read-only subset; see [limits](support-matrix.md) |
| Invalid cursor | Keep identity, parameters and limits identical; restart after expiration, upgrade or access changes |

`BUILD.json` identifies the version, binary digest and source commit. See [validation](validation.md) for tested products/platforms and remaining limitations.

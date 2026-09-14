#!/usr/bin/env bash
set -euo pipefail
umask 077
if [[ $# != 1 || -e "$1" ]]; then
  echo 'Usage: bash scripts/backup-metadata.sh NEW_BACKUP_DIRECTORY' >&2
  echo 'The destination must not already exist. Run from the Compose project directory.' >&2
  exit 1
fi
mkdir -m 700 -p "$1"
backup_dir=$(cd "$1" && pwd)
echo 'Backing up PostgreSQL metadata and its matching encryption key...'
docker compose exec -T postgres sh -c 'exec pg_dump -U "$POSTGRES_USER" -d "$POSTGRES_DB" --format=custom --no-owner --no-acl' > "$backup_dir/metadata.dump"
docker compose exec -T hub sh -c 'if [ -n "${MCPDBHUB_MASTER_KEY:-}" ]; then printf %s "$MCPDBHUB_MASTER_KEY" | base64 -d; else cat "${MCPDBHUB_DATA_DIR:-/data}/master.key"; fi' > "$backup_dir/master.key"
if (( $(wc -c < "$backup_dir/master.key") != 32 )); then
  echo 'Backup failed: expected a 32-byte encryption key. Do not use this incomplete backup.' >&2
  exit 1
fi
docker compose exec -T hub contextgate version > "$backup_dir/build.txt"
cd "$backup_dir"
if command -v sha256sum >/dev/null; then
  sha256sum metadata.dump master.key build.txt > SHA256SUMS
else
  shasum -a 256 metadata.dump master.key build.txt > SHA256SUMS
fi
echo "Backup created at $backup_dir. Verify it before relying on it for recovery."

#!/usr/bin/env bash
set -euo pipefail
umask 077
if [[ $# != 1 || ! -f "$1/SHA256SUMS" ]]; then
  echo 'Usage: bash scripts/verify-backup.sh BACKUP_DIRECTORY' >&2
  exit 1
fi
backup_dir=$(cd "$1" && pwd)
# Check only the three expected files, not paths supplied by an edited manifest.
expected=$(mktemp)
trap 'rm -f "$expected"' EXIT
(
  cd "$backup_dir"
  if command -v sha256sum >/dev/null; then sha256sum metadata.dump master.key build.txt; else shasum -a 256 metadata.dump master.key build.txt; fi
) > "$expected"
cmp "$expected" "$backup_dir/SHA256SUMS" || { echo 'Backup checksum verification failed.' >&2; exit 1; }
name="mcpdbhub-restore-check-$$-$RANDOM"
pg_image=${MCPDBHUB_PG_IMAGE:-postgres:17.11}
hub_image=${MCPDBHUB_VERIFY_IMAGE:-contextgate:local}
cleanup() {
  docker rm -f -v "$name-pg" >/dev/null 2>&1 || true
  docker network rm "$name" >/dev/null 2>&1 || true
  rm -f "$expected"
}
trap cleanup EXIT
docker network create --internal "$name" >/dev/null
docker run -d --name "$name-pg" --network "$name" -e POSTGRES_USER=hub -e POSTGRES_DB=hub -e POSTGRES_HOST_AUTH_METHOD=trust "$pg_image" >/dev/null
ready=false
for _ in {1..30}; do
  if docker exec "$name-pg" pg_isready -h 127.0.0.1 -U hub -d hub >/dev/null 2>&1; then ready=true; break; fi
  sleep 1
done
[[ $ready == true ]] || { echo 'Temporary PostgreSQL did not become ready.' >&2; exit 1; }
docker exec -i "$name-pg" pg_restore -U hub -d hub --exit-on-error --no-owner --no-acl < "$backup_dir/metadata.dump"
# The verification command only reads metadata. No server or background workers
# are started, and the private network has no path to any configured source.
docker run --rm --network "$name" --user 0:0 \
  --mount "type=bind,src=$backup_dir,dst=/backup,readonly" \
  -e "MCPDBHUB_DATABASE_URL=postgres://hub@$name-pg:5432/hub?sslmode=disable" \
  --entrypoint contextgate "$hub_image" verify-metadata --data-dir /backup

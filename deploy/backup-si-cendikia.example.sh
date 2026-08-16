#!/usr/bin/env bash
set -Eeuo pipefail
umask 077

# Configure these through the environment or an untracked wrapper.
: "${DATABASE_URL:?DATABASE_URL wajib diisi}"
BACKUP_ROOT="${BACKUP_ROOT:-/var/backups/si-cendikia}"
FILE_ROOT="${FILE_ROOT:-/var/lib/si-cendikia/files}"
RETENTION_DAYS="${RETENTION_DAYS:-14}"
STAMP="$(date -u +%Y%m%dT%H%M%SZ)"
DEST="${BACKUP_ROOT}/${STAMP}"

mkdir -p "${DEST}/files"
pg_dump --format=custom --file="${DEST}/database.dump" "${DATABASE_URL}"
rsync -a --delete "${FILE_ROOT}/" "${DEST}/files/"
sha256sum "${DEST}/database.dump" > "${DEST}/SHA256SUMS"
find "${BACKUP_ROOT}" -mindepth 1 -maxdepth 1 -type d -mtime "+${RETENTION_DAYS}" -exec rm -rf -- {} +

printf 'backup selesai: %s\n' "${DEST}"

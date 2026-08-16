#!/usr/bin/env bash
set -Eeuo pipefail
umask 077

# DRILL ONLY: restore into a dedicated database and file root. Never point this
# at production without an explicit maintenance plan.
: "${DATABASE_URL:?DATABASE_URL wajib diisi}"
: "${BACKUP_DIR:?BACKUP_DIR wajib diisi}"
RESTORE_DATABASE_URL="${RESTORE_DATABASE_URL:-${DATABASE_URL}}"
RESTORE_FILE_ROOT="${RESTORE_FILE_ROOT:-/tmp/si-cendikia-restore-files}"

sha256sum --check "${BACKUP_DIR}/SHA256SUMS"
mkdir -p "${RESTORE_FILE_ROOT}"
pg_restore --clean --if-exists --exit-on-error --dbname="${RESTORE_DATABASE_URL}" "${BACKUP_DIR}/database.dump"
rsync -a --delete "${BACKUP_DIR}/files/" "${RESTORE_FILE_ROOT}/"
printf 'restore drill selesai: database + %s\n' "${RESTORE_FILE_ROOT}"

# Verify application readiness separately before any cutover.
# curl --fail https://HOST/readyz

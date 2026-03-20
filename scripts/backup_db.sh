#!/bin/bash
# Regular SQLite database backup script
# Creates timestamped backups and keeps the last 30 days

set -e

BACKUP_DIR="/root/astranet/backups"
DB_PATH="/root/astranet/data/amino.db"
MAX_BACKUPS=30
TIMESTAMP=$(date +"%Y%m%d_%H%M%S")
BACKUP_FILE="${BACKUP_DIR}/amino_${TIMESTAMP}.db"

echo "[backup] Starting database backup at $(date)"

# Create backup directory if needed
mkdir -p "$BACKUP_DIR"

# Check database exists
if [ ! -f "$DB_PATH" ]; then
    echo "[backup] ERROR: Database not found at $DB_PATH"
    exit 1
fi

# Use SQLite .backup command for a consistent copy (safe even while DB is in use)
sqlite3 "$DB_PATH" ".backup '${BACKUP_FILE}'"

if [ $? -eq 0 ]; then
    # Compress the backup
    gzip "$BACKUP_FILE"
    BACKUP_SIZE=$(du -h "${BACKUP_FILE}.gz" | cut -f1)
    echo "[backup] Success: ${BACKUP_FILE}.gz (${BACKUP_SIZE})"
else
    echo "[backup] ERROR: Backup failed"
    exit 1
fi

# Verify backup integrity
gunzip -t "${BACKUP_FILE}.gz"
if [ $? -ne 0 ]; then
    echo "[backup] ERROR: Backup verification failed (corrupt gzip)"
    rm -f "${BACKUP_FILE}.gz"
    exit 1
fi

# Remove old backups, keep only the last MAX_BACKUPS
BACKUP_COUNT=$(ls -1 "${BACKUP_DIR}"/amino_*.db.gz 2>/dev/null | wc -l)
if [ "$BACKUP_COUNT" -gt "$MAX_BACKUPS" ]; then
    REMOVE_COUNT=$((BACKUP_COUNT - MAX_BACKUPS))
    ls -1t "${BACKUP_DIR}"/amino_*.db.gz | tail -n "$REMOVE_COUNT" | xargs rm -f
    echo "[backup] Cleaned up $REMOVE_COUNT old backup(s), keeping $MAX_BACKUPS"
fi

echo "[backup] Completed at $(date)"

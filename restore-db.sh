#!/bin/bash
set -e

DUMP_FILE="${1:-backup.dump}"
CONTAINER="anigraph-postgres"
DB_USER="anigraph"
DB_NAME="anigraph"

if [ ! -f "$DUMP_FILE" ]; then
  echo "Error: $DUMP_FILE not found"
  exit 1
fi

echo "Restoring $DUMP_FILE into $CONTAINER..."
cat "$DUMP_FILE" | docker exec -i "$CONTAINER" pg_restore -U "$DB_USER" -d "$DB_NAME" --clean --if-exists --no-owner
echo "Done."

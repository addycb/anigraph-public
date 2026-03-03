#!/bin/bash
# Neo4j entrypoint with reset endpoint support

RESET_PORT=${RESET_PORT:-7475}

echo "Starting reset server on port $RESET_PORT..."

# Start socat HTTP server in background
# For each connection, it runs reset-handler.sh
socat TCP-LISTEN:$RESET_PORT,reuseaddr,fork EXEC:/reset-handler.sh &

# Start Neo4j (this is the original entrypoint behavior)
exec /startup/docker-entrypoint.sh neo4j

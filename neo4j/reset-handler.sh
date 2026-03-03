#!/bin/bash
# Handler script called by socat for each HTTP request

# Read the HTTP request
read -r REQUEST_LINE
METHOD=$(echo "$REQUEST_LINE" | cut -d' ' -f1)

# Consume remaining headers
while read -r header; do
    [ -z "$header" ] || [ "$header" = $'\r' ] && break
done

# Send HTTP response
echo -e "HTTP/1.1 200 OK\r"
echo -e "Content-Type: application/json\r"
echo -e "Connection: close\r"
echo -e "\r"
echo '{"status":"resetting"}'

# Only reset on POST
if [ "$METHOD" = "POST" ]; then
    echo "$(date): POST request received, triggering reset..." >> /tmp/reset.log

    # Do the reset in background so response is sent first
    (
        sleep 1

        # Stop Neo4j
        pkill -f "java.*neo4j" 2>/dev/null
        sleep 2

        # Clear data
        rm -rf /data/databases/* 2>/dev/null
        rm -rf /data/transactions/* 2>/dev/null
        rm -rf /data/dbms/* 2>/dev/null

        echo "$(date): Data cleared, killing container..." >> /tmp/reset.log

        # Kill PID 1 to restart container
        kill 1
    ) &
fi

FROM neo4j:2025.11.2

# Install socat for the reset server (more reliable than netcat for HTTP)
USER root
RUN apt-get update && apt-get install -y socat procps && rm -rf /var/lib/apt/lists/*

# Copy reset entrypoint and handler
COPY reset-entrypoint.sh /reset-entrypoint.sh
COPY reset-handler.sh /reset-handler.sh
RUN chmod +x /reset-entrypoint.sh /reset-handler.sh

USER neo4j

ENTRYPOINT ["/reset-entrypoint.sh"]

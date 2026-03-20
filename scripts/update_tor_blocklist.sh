#!/bin/bash
# Update Tor exit node blocklist for Caddy
# Runs periodically via cron (every 6 hours)

set -e

LOG_PREFIX="[tor-blocklist]"
BLOCKLIST_FILE="/root/astranet/tor_block.caddy"
SERVER_IP="144.31.166.46"

echo "$LOG_PREFIX Starting update at $(date)"

# Download fresh Tor exit node list
TOR_IPS=$(curl -sf --max-time 30 "https://check.torproject.org/torbulkexitlist?ip=${SERVER_IP}" | grep -E "^[0-9]" | sort -u)

if [ -z "$TOR_IPS" ]; then
    echo "$LOG_PREFIX ERROR: Failed to download Tor exit node list or list is empty"
    exit 1
fi

IP_COUNT=$(echo "$TOR_IPS" | wc -l)
echo "$LOG_PREFIX Downloaded $IP_COUNT Tor exit node IPs"

# Generate Caddy snippet — one IP per @tor matcher, then respond
# Format for Caddy: @tor remote_ip <ip1> <ip2> ...
IP_LINE=$(echo "$TOR_IPS" | tr '\n' ' ')

cat > "${BLOCKLIST_FILE}.tmp" <<EOF
@tor remote_ip ${IP_LINE}
respond @tor "Access Denied" 403
EOF

# Atomic replace
mv "${BLOCKLIST_FILE}.tmp" "$BLOCKLIST_FILE"

# Reload Caddy
docker exec astranet-caddy caddy reload --config /etc/caddy/Caddyfile 2>&1

if [ $? -eq 0 ]; then
    echo "$LOG_PREFIX Successfully updated blocklist with $IP_COUNT IPs and reloaded Caddy"
else
    echo "$LOG_PREFIX WARNING: Caddy reload failed (may need manual check)"
fi

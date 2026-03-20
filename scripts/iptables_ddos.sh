#!/bin/bash
# DDoS protection via iptables
# Run once at server boot or after reboot

set -e

echo "[iptables] Setting up DDoS protection rules..."

# Flush existing custom chains if they exist
iptables -F DDOS_PROTECT 2>/dev/null || true
iptables -X DDOS_PROTECT 2>/dev/null || true

# Create custom chain
iptables -N DDOS_PROTECT

# Drop invalid packets
iptables -A DDOS_PROTECT -m state --state INVALID -j DROP

# Limit new TCP connections: max 60 per minute per IP (1 per second burst 20)
iptables -A DDOS_PROTECT -p tcp --dport 443 -m state --state NEW -m recent --set --name HTTPS
iptables -A DDOS_PROTECT -p tcp --dport 443 -m state --state NEW -m recent --update --seconds 60 --hitcount 60 --name HTTPS -j DROP

iptables -A DDOS_PROTECT -p tcp --dport 80 -m state --state NEW -m recent --set --name HTTP
iptables -A DDOS_PROTECT -p tcp --dport 80 -m state --state NEW -m recent --update --seconds 60 --hitcount 60 --name HTTP -j DROP

# Limit ICMP (ping) to prevent ping flood
iptables -A DDOS_PROTECT -p icmp --icmp-type echo-request -m limit --limit 1/s --limit-burst 4 -j ACCEPT
iptables -A DDOS_PROTECT -p icmp --icmp-type echo-request -j DROP

# Protection against SYN flood
iptables -A DDOS_PROTECT -p tcp --syn -m limit --limit 10/s --limit-burst 30 -j ACCEPT
iptables -A DDOS_PROTECT -p tcp --syn -j DROP

# Accept everything else in the chain
iptables -A DDOS_PROTECT -j RETURN

# Insert DDOS_PROTECT chain at the top of INPUT (check if already inserted)
if ! iptables -C INPUT -j DDOS_PROTECT 2>/dev/null; then
    iptables -I INPUT 1 -j DDOS_PROTECT
fi

echo "[iptables] DDoS protection rules applied successfully"

# Show active rules
iptables -L DDOS_PROTECT -n -v

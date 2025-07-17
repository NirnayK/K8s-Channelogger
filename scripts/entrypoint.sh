#!/usr/bin/env bash
set -euo pipefail

log() { echo "$(date +'%Y-%m-%dT%H:%M:%S%z') [entrypoint] $*"; }

# 1. Ensure /dev/ppp exists for PPP-based VPN clients
if [[ ! -c /dev/ppp ]]; then
  log "Creating /dev/ppp"
  mknod /dev/ppp c 108 0
  chmod 600 /dev/ppp
fi

# 2. Start VPN once (will bring up ppp0)
log "Starting VPN to $MYVPN_PROD_VPN_IP"
sudo -n openfortivpn "$MYVPN_PROD_VPN_IP" \
    -u "$MYVPN_PROD_VPN_USER" \
    -p "$MYVPN_PROD_VPN_PASSWORD" \
    --trusted-cert "$MYVPN_PROD_VPN_CERT" &
vpn_pid=$!
trap 'log "Shutting down VPN"; kill "$vpn_pid" 2>/dev/null || true' EXIT

# 3. Hand off to channelog (PID 1)
log "Launching channelog"
exec channelog "$@"

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

# 3. Install Git if not already present
if ! command -v git &> /dev/null; then
    log "Installing Git"
    apt-get update && apt-get install -y --no-install-recommends git
    rm -rf /var/lib/apt/lists/*
fi

# 4. Configure Git with user information
log "Configuring Git user settings"
if [[ -n "${USERNAME:-}" ]]; then
    git config --global user.name "$USERNAME"
    log "Set Git username to: $USERNAME"
else
    log "WARNING: USERNAME environment variable not set"
fi

if [[ -n "${USER_EMAIL:-}" ]]; then
    git config --global user.email "$USER_EMAIL"
    log "Set Git email to: $USER_EMAIL"
else
    log "WARNING: USER_EMAIL environment variable not set"
fi

# 5. Set up SSH authentication for Git
if [[ -n "${SSH_PRIVATE_KEY:-}" ]]; then
    log "Setting up SSH key authentication"
    mkdir -p ~/.ssh
    chmod 700 ~/.ssh
    
    # Write the SSH private key
    echo "$SSH_PRIVATE_KEY" > ~/.ssh/id_rsa
    chmod 600 ~/.ssh/id_rsa
    
    # Add GitLab host to known_hosts if GIT_REPO is provided
    if [[ -n "${GIT_REPO:-}" ]]; then
        if [[ "$GIT_REPO" =~ git@([^:]+): ]]; then
            hostname="${BASH_REMATCH[1]}"
            log "Adding $hostname to known_hosts"
            ssh-keyscan -H "$hostname" >> ~/.ssh/known_hosts 2>/dev/null || true
            chmod 644 ~/.ssh/known_hosts
        else
            log "WARNING: Could not extract hostname from GIT_REPO: $GIT_REPO"
        fi
    fi
    
    # Test SSH connectivity
    if [[ -n "${hostname:-}" ]]; then
        log "Testing SSH connectivity to $hostname"
        if ssh -o ConnectTimeout=10 -o StrictHostKeyChecking=no git@"$hostname" exit &>/dev/null; then
            log "✓ SSH authentication successful"
        else
            log "⚠ WARNING: SSH authentication test failed"
        fi
    fi
else
    log "INFO: No SSH_PRIVATE_KEY provided, assuming other authentication method"
fi

# 6. Set additional Git configurations for better compatibility
log "Setting additional Git configurations"
git config --global init.defaultBranch main
git config --global pull.rebase false
git config --global core.autocrlf input

# 7. Verify Git configuration
log "Git configuration summary:"
git config --global --list | grep -E "^(user\.|credential\.|init\.|pull\.|core\.)" || true

# 8. Test Git connectivity if repository is provided (after VPN is up)
if [[ -n "${GIT_REPO:-}" ]]; then
    log "Testing Git connectivity to repository"
    # Give VPN a moment to establish connection
    sleep 5
    
    # Set SSH options for Git operations
    export GIT_SSH_COMMAND="ssh -o UserKnownHostsFile=~/.ssh/known_hosts -o StrictHostKeyChecking=no"
    
    if git ls-remote "$GIT_REPO" HEAD &>/dev/null; then
        log "✓ Successfully connected to Git repository"
    else
        log "⚠ WARNING: Could not connect to Git repository - check SSH key and network"
    fi
fi

# 9. Hand off to channelog (PID 1)
log "Launching channelog"
exec channelog "$@"

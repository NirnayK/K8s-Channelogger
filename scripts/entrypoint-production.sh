#!/usr/bin/env bash
set -euo pipefail

log() { echo "$(date +'%Y-%m-%dT%H:%M:%S%z') [production-entrypoint] $*"; }

# 1. Install Git if not already present
if ! command -v git &> /dev/null; then
    log "Installing Git"
    apt-get update && apt-get install -y --no-install-recommends git
    rm -rf /var/lib/apt/lists/*
fi

# 2. Configure Git with user information
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

# 3. Set up Git authentication if token is provided
if [[ -n "${GIT_TOKEN:-}" ]]; then
    log "Setting up Git token authentication"
    git config --global credential.helper store
    
    # Extract hostname from GIT_REPO URL for credential setup
    if [[ -n "${GIT_REPO:-}" ]]; then
        if [[ "$GIT_REPO" =~ https://([^/]+) ]]; then
            hostname="${BASH_REMATCH[1]}"
            echo "https://oauth2:$GIT_TOKEN@$hostname" > ~/.git-credentials
            chmod 600 ~/.git-credentials
            log "Configured token authentication for: $hostname"
        else
            log "WARNING: Could not extract hostname from GIT_REPO: $GIT_REPO"
        fi
    else
        log "WARNING: GIT_REPO environment variable not set"
    fi
else
    log "INFO: No GIT_TOKEN provided, assuming SSH authentication"
fi

# 4. Set additional Git configurations for better compatibility
log "Setting additional Git configurations"
git config --global init.defaultBranch main
git config --global pull.rebase false
git config --global core.autocrlf input

# 5. Verify Git configuration
log "Git configuration summary:"
git config --global --list | grep -E "^(user\.|credential\.|init\.|pull\.|core\.)" || true

# 6. Test Git connectivity if repository is provided
if [[ -n "${GIT_REPO:-}" ]]; then
    log "Testing Git connectivity to repository"
    if git ls-remote "$GIT_REPO" HEAD &>/dev/null; then
        log "✓ Successfully connected to Git repository"
    else
        log "⚠ WARNING: Could not connect to Git repository - check credentials and network"
    fi
fi

# 7. Launch the main application
log "Launching channelog application"
exec channelog "$@"

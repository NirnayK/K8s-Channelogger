#!/usr/bin/env bash
set -euxo pipefail

# ─── Paths ─────────────────────────────────────────────────────────────────────

CERT_DIR="$(pwd)/tls/certs"
DEPLOY_DIR="${1:-$(pwd)/deploy}"  # Accept deploy dir as first argument, fallback to default

if [ $# -eq 0 ]; then
  echo "Usage: $0 <deploy_directory>"
  echo "Example: $0 deploy/testenv"
  exit 1
fi


# ─── Prerequisites ────────────────────────────────────────────────────────────

if ! command -v yq &>/dev/null; then
  echo "Error: yq is required (https://github.com/mikefarah/yq)"
  exit 1
fi

# ─── Load & encode certs ──────────────────────────────────────────────────────

TLS_CRT_B64=$(base64 < "${CERT_DIR}/server.crt")
TLS_KEY_B64=$(base64 < "${CERT_DIR}/server.key")
CA_CRT_B64=$(base64 < "${CERT_DIR}/ca.crt")

# ─── Patch the Secret manifest ─────────────────────────────────────────────────

# Update secret.yaml if it exists in the deploy directory
if [ -f "${DEPLOY_DIR}/secret.yaml" ]; then
  yq eval \
    ".data[\"tls.crt\"] = \"${TLS_CRT_B64}\" |
     .data[\"tls.key\"] = \"${TLS_KEY_B64}\"" \
    -i "${DEPLOY_DIR}/secret.yaml"
  echo "✅ Updated ${DEPLOY_DIR}/secret.yaml"
fi

# Update secret_test.yaml if it exists in the deploy directory  
if [ -f "${DEPLOY_DIR}/secret_test.yaml" ]; then
  yq eval \
    ".data[\"tls.crt\"] = \"${TLS_CRT_B64}\" |
     .data[\"tls.key\"] = \"${TLS_KEY_B64}\"" \
    -i "${DEPLOY_DIR}/secret_test.yaml"
  echo "✅ Updated ${DEPLOY_DIR}/secret_test.yaml"
fi


# ─── Patch the ValidatingWebhookConfiguration ─────────────────────────────────

yq eval \
  ".webhooks[].clientConfig.caBundle = \"${CA_CRT_B64}\"" \
  -i "${DEPLOY_DIR}/config.yaml"

echo "✅ Updated ${DEPLOY_DIR}/config.yaml with new CA bundle"

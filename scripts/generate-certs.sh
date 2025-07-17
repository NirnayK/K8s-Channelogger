#!/usr/bin/env bash
set -euxo pipefail

# ─── Configuration ────────────────────────────────────────────────────────────

# Directory for cert artifacts
CERT_DIR="$(pwd)/tls/certs"

# Path to your OpenSSL SAN config
SAN_CFG="$(pwd)/tls/channelog-openssl.cnf"

# Certificate validity (in days)
VALIDITY_DAYS=365

# ─── Prepare ──────────────────────────────────────────────────────────────────

mkdir -p "$CERT_DIR"
cp "$SAN_CFG" "$CERT_DIR/channelog-openssl.cnf"

# ─── 1) Generate CA ───────────────────────────────────────────────────────────

openssl genrsa -out "$CERT_DIR/ca.key" 2048
openssl req -x509 -new -nodes \
  -key "$CERT_DIR/ca.key" \
  -subj "/CN=admission-channelog-ca" \
  -days $((VALIDITY_DAYS * 10)) \
  -out "$CERT_DIR/ca.crt"

# ─── 2) Generate server key & CSR ────────────────────────────────────────────

openssl genrsa -out "$CERT_DIR/server.key" 2048
openssl req -new \
  -key "$CERT_DIR/server.key" \
  -out "$CERT_DIR/server.csr" \
  -config "$CERT_DIR/channelog-openssl.cnf"

# ─── 3) Sign server certificate with CA ──────────────────────────────────────

openssl x509 -req \
  -in "$CERT_DIR/server.csr" \
  -CA "$CERT_DIR/ca.crt" \
  -CAkey "$CERT_DIR/ca.key" \
  -CAcreateserial \
  -out "$CERT_DIR/server.crt" \
  -days "$VALIDITY_DAYS" \
  -extensions v3_req \
  -extfile "$CERT_DIR/channelog-openssl.cnf"

echo "✅  Certs created in $CERT_DIR"

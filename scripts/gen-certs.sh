#!/usr/bin/env bash
# Generates a self-signed CA and server+client certificates for local TLS testing.
# Output directory: certs/
# Usage: bash scripts/gen-certs.sh

set -euo pipefail

OUT="$(cd "$(dirname "$0")/.." && pwd)/certs"
mkdir -p "$OUT"

DAYS=3650
SUBJ_CA="/CN=Termwire Test CA/O=Termwire/C=TR"
SUBJ_SRV="/CN=localhost/O=Termwire POS/C=TR"
SUBJ_CLI="/CN=ecr-client/O=Termwire ECR/C=TR"

echo "[1/5] Generating CA key and certificate..."
openssl genrsa -out "$OUT/ca.key" 4096 2>/dev/null
openssl req -new -x509 -key "$OUT/ca.key" -out "$OUT/ca.crt" -days "$DAYS" -subj "$SUBJ_CA"

echo "[2/5] Generating server key and CSR..."
openssl genrsa -out "$OUT/server.key" 2048 2>/dev/null
openssl req -new -key "$OUT/server.key" -out "$OUT/server.csr" -subj "$SUBJ_SRV"

echo "[3/5] Signing server certificate with CA..."
openssl x509 -req -in "$OUT/server.csr" -CA "$OUT/ca.crt" -CAkey "$OUT/ca.key" \
    -CAcreateserial -out "$OUT/server.crt" -days "$DAYS" \
    -extfile <(printf "subjectAltName=DNS:localhost,IP:127.0.0.1\nextendedKeyUsage=serverAuth")

echo "[4/5] Generating client key and CSR..."
openssl genrsa -out "$OUT/client.key" 2048 2>/dev/null
openssl req -new -key "$OUT/client.key" -out "$OUT/client.csr" -subj "$SUBJ_CLI"

echo "[5/5] Signing client certificate with CA..."
openssl x509 -req -in "$OUT/client.csr" -CA "$OUT/ca.crt" -CAkey "$OUT/ca.key" \
    -CAcreateserial -out "$OUT/client.crt" -days "$DAYS" \
    -extfile <(printf "extendedKeyUsage=clientAuth")

rm -f "$OUT"/*.csr "$OUT"/*.srl

echo ""
echo "Certificates written to $OUT/"
echo "  ca.crt      — CA certificate (used by both sides)"
echo "  server.crt  — POS server certificate"
echo "  server.key  — POS server private key"
echo "  client.crt  — ECR client certificate (mTLS)"
echo "  client.key  — ECR client private key (mTLS)"
echo ""
echo "Example POS config:"
echo "  TLSEnabled:  true"
echo "  TLSCertFile: \"certs/server.crt\""
echo "  TLSKeyFile:  \"certs/server.key\""
echo "  TLSCAFile:   \"certs/ca.crt\"   (for mTLS client verification)"
echo ""
echo "Example ECR config:"
echo "  TLSEnabled:  true"
echo "  TLSCAFile:   \"certs/ca.crt\""
echo "  TLSCertFile: \"certs/client.crt\"  (for mTLS)"
echo "  TLSKeyFile:  \"certs/client.key\"  (for mTLS)"

#!/usr/bin/env bash
# Run a Termwire ECR example against a local POS terminal.
# Usage: bash scripts/run-example.sh [example]
#
# Examples: simple_payment (default), loyalty_payment, refund_void, reports
#
# simple_payment uses mTLS — run scripts/gen-certs.sh first if certs/ is missing.

set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
BIN="$ROOT/bin"
EXAMPLE="${1:-simple_payment}"

# ── Validate example name ────────────────────────────────────────────────────

VALID_EXAMPLES=(simple_payment loyalty_payment refund_void reports)
VALID=false
for e in "${VALID_EXAMPLES[@]}"; do
    [[ "$e" == "$EXAMPLE" ]] && VALID=true && break
done
if ! $VALID; then
    echo "Unknown example: $EXAMPLE"
    echo "Available: ${VALID_EXAMPLES[*]}"
    exit 1
fi

# ── Build ────────────────────────────────────────────────────────────────────

echo "[build] pos-terminal"
go build -o "$BIN/pos-terminal" "$ROOT/pos/cmd/"

EXAMPLE_BIN="$BIN/$(echo "$EXAMPLE" | tr '_' '-')"
echo "[build] $EXAMPLE"
go build -o "$EXAMPLE_BIN" "$ROOT/ecr/examples/$EXAMPLE/"

# ── TLS check for simple_payment ─────────────────────────────────────────────

POS_ENV=()
if [[ "$EXAMPLE" == "simple_payment" ]]; then
    for f in ca.crt server.crt server.key client.crt client.key; do
        if [[ ! -f "$ROOT/certs/$f" ]]; then
            echo "Missing certs/$f — run: bash scripts/gen-certs.sh"
            exit 1
        fi
    done
    POS_ENV=(
        POS_TLS_ENABLED=true
        POS_TLS_CERT=certs/server.crt
        POS_TLS_KEY=certs/server.key
        POS_TLS_CA=certs/ca.crt
    )
fi

# ── Start POS server ─────────────────────────────────────────────────────────

POS_LOG="$ROOT/bin/pos-terminal.log"
echo "[server] starting POS terminal (log: bin/pos-terminal.log)"
cd "$ROOT"
env "${POS_ENV[@]+"${POS_ENV[@]}"}" "$BIN/pos-terminal" >"$POS_LOG" 2>&1 &
POS_PID=$!

cleanup() {
    echo ""
    echo "[server] stopping POS terminal (pid $POS_PID)"
    kill "$POS_PID" 2>/dev/null || true
    wait "$POS_PID" 2>/dev/null || true
}
trap cleanup EXIT INT TERM

# Wait for the server to be ready (up to 5 seconds)
echo "[server] waiting for port 8080..."
for i in $(seq 1 10); do
    if nc -z 127.0.0.1 8080 2>/dev/null; then
        echo "[server] ready"
        break
    fi
    if [[ $i -eq 10 ]]; then
        echo "[server] timed out waiting for port 8080"
        cat "$POS_LOG"
        exit 1
    fi
    sleep 0.5
done

# ── Run example ───────────────────────────────────────────────────────────────

echo ""
echo "[run] $EXAMPLE"
echo "────────────────────────────────────────"
"$EXAMPLE_BIN"

#!/usr/bin/env bash
set -euo pipefail

# ─────────────────────────────────────────────────────────────
# Config
# ─────────────────────────────────────────────────────────────
NODES=${1:-5}
NETWORK=${2:-mainnet}                 # mainnet | testnet
CHAINID_MAINNET=2250
CHAINID_TESTNET=2249
BINARY="./setd"
CORE_FILE="./core/default_genesis.go"
DATAROOT="./devnet"
LOGROOT="./logs"
GENESIS="./genesis.json"
BASEPORT=30303
BASEHTTP=8545

# ─────────────────────────────────────────────────────────────
# Extract genesis from Go source
# ─────────────────────────────────────────────────────────────
echo "📦  Extracting $NETWORK genesis from $CORE_FILE..."

case "$NETWORK" in
  mainnet)
    GENESIS_JSON=$(awk '/const defaultMainnetGenesis = `/{flag=1;next}/`/{flag=0}flag' "$CORE_FILE")
    CHAINID=$CHAINID_MAINNET
    ;;
  testnet)
    GENESIS_JSON=$(awk '/const defaultTestnetGenesis = `/{flag=1;next}/`/{flag=0}flag' "$CORE_FILE")
    CHAINID=$CHAINID_TESTNET
    ;;
  *)
    echo "Usage: $0 [NODES] [mainnet|testnet]"
    exit 1
    ;;
esac

if [[ -z "$GENESIS_JSON" ]]; then
  echo "❌  Failed to extract $NETWORK genesis from $CORE_FILE"
  exit 1
fi
echo "$GENESIS_JSON" > "$GENESIS"
echo "✅  Genesis written to $GENESIS"

# ─────────────────────────────────────────────────────────────
# Ensure binary exists
# ─────────────────────────────────────────────────────────────
if [[ ! -f "$BINARY" ]]; then
  echo "❌  Binary not found at $BINARY"
  echo "Run: go build ./cmd/sesa-node"
  exit 1
fi

mkdir -p "$DATAROOT" "$LOGROOT"

# ─────────────────────────────────────────────────────────────
# Initialize node data dirs
# ─────────────────────────────────────────────────────────────
echo "🔧  Initializing $NODES validators..."
for i in $(seq 0 $((NODES - 1))); do
  NODEDIR="$DATAROOT/validator${i}"
  mkdir -p "$NODEDIR/data"
  cp "$GENESIS" "$NODEDIR/"
  "$BINARY" --datadir "$NODEDIR/data" init "$NODEDIR/genesis.json" >/dev/null
done

# ─────────────────────────────────────────────────────────────
# Extract enodes & create static-nodes.json
# ─────────────────────────────────────────────────────────────
echo "🌐  Extracting enodes..."
for i in $(seq 0 $((NODES - 1))); do
  PORT=$((BASEPORT + i))
  HTTP=$((BASEHTTP + i))
  NODEDIR="$DATAROOT/validator${i}"
  echo "   • validator${i}..."
  "$BINARY" --datadir "$NODEDIR/data" --port "$PORT" --http.port "$HTTP" --nodiscover --dev --ipcdisable 2> "$LOGROOT/tmp$i.log" &
  PID=$!
  sleep 2
  ENODE=$(grep -m1 "enode://" "$LOGROOT/tmp$i.log" | sed -E 's/.*(enode:\/\/[^ ]+).*/\1/')
  kill $PID >/dev/null 2>&1 || true
  rm -f "$LOGROOT/tmp$i.log"
  ENODES[i]=$ENODE
done

STATIC_JSON=$(printf '["%s"]' "$(IFS='","'; echo "${ENODES[*]}")")
for i in $(seq 0 $((NODES - 1))); do
  echo "$STATIC_JSON" > "$DATAROOT/validator${i}/data/static-nodes.json"
done

BOOTNODE_ENODE=${ENODES[0]}

# ─────────────────────────────────────────────────────────────
# Launch all validators
# ─────────────────────────────────────────────────────────────
echo "🚀  Starting $NODES validators (chainid=$CHAINID)..."
for i in $(seq 0 $((NODES - 1))); do
  PORT=$((BASEPORT + i))
  HTTP=$((BASEHTTP + i))
  NODEDIR="$DATAROOT/validator${i}"
  LOGFILE="$LOGROOT/validator${i}.log"

  echo "   ▶ validator${i} (p2p:$PORT rpc:$HTTP)"
  "$BINARY" \
    --datadir "$NODEDIR/data" \
    --networkid "$CHAINID" \
    --port "$PORT" \
    --http --http.addr "0.0.0.0" --http.port "$HTTP" \
    --mine --miner.threads=1 \
    --validator \
    --bootnodes "$BOOTNODE_ENODE" \
    --verbosity 3 \
    >"$LOGFILE" 2>&1 &
  sleep 1
done

echo
echo "✅  Sesa devnet running ($NODES validators, $NETWORK)."
echo "   RPC endpoints start at http://127.0.0.1:${BASEHTTP}"
echo "   Logs: $LOGROOT/"
echo
echo "To stop:"
echo "   pkill -f sesa-node"

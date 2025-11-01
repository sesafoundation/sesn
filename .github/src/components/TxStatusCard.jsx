import React, { useState, useEffect } from "react";

/* --- WebSocket watcher for preconfirmation stream --- */
class PreconfWatcher {
  constructor(url) {
    this.url = url;
    this.ws = null;
    this.listeners = new Map();
  }

  connect() {
    this.ws = new WebSocket(this.url);
    this.ws.onopen = () => console.log("✅ Preconf WS connected");
    this.ws.onmessage = (msg) => {
      try {
        const data = JSON.parse(msg.data);
        if (!data.TxHashes) return;
        data.TxHashes.forEach((h) => {
          const cb = this.listeners.get(h.toLowerCase());
          if (cb) cb(data);
        });
      } catch (err) {
        console.warn("Preconf parse error", err);
      }
    };
    this.ws.onclose = () => setTimeout(() => this.connect(), 3000);
  }

  watch(txHash, cb) {
    this.listeners.set(txHash.toLowerCase(), cb);
  }
}

/* --- UI Component --- */
export default function TxStatusCard({ txHash, rpcUrl, sidecarUrl }) {
  const [status, setStatus] = useState("pending");
  const [miniBlock, setMiniBlock] = useState(null);
  const [blockNum, setBlockNum] = useState(null);
  const [latency, setLatency] = useState(null);

  useEffect(() => {
    const watcher = new PreconfWatcher(sidecarUrl);
    watcher.connect();
    const sentTime = Date.now();
    watcher.watch(txHash, (mb) => {
      const now = Date.now();
      const mbTime = mb.TimestampMs ? parseInt(mb.TimestampMs) : now;
      const diff = now - mbTime;
      setLatency(diff > 0 && diff < 5000 ? diff : null);
      setMiniBlock(mb);
      setStatus("preconfirmed");
    });

    const interval = setInterval(async () => {
      if (status === "final") return;
      try {
        const res = await fetch(rpcUrl, {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({
            jsonrpc: "2.0",
            id: 1,
            method: "eth_getTransactionReceipt",
            params: [txHash],
          }),
        }).then((r) => r.json());
        if (res.result && res.result.blockNumber) {
          setBlockNum(parseInt(res.result.blockNumber, 16));
          setStatus("final");
        }
      } catch (_) {}
    }, 2000);

    return () => clearInterval(interval);
  }, [txHash, rpcUrl, sidecarUrl, status]);

  const stageColor = {
    pending: "bg-gray-400",
    preconfirmed: "bg-green-400",
    final: "bg-green-600",
  }[status];

  const stageText = {
    pending: "Pending… waiting for preconfirmation",
    preconfirmed: "Preconfirmed ✓ — included in mini-block",
    final: "Final ✓✓ — included in canonical block",
  }[status];

  return (
    <div className="p-4 rounded-2xl shadow-md bg-gray-900 text-gray-100 w-full max-w-xl mx-auto mt-6">
      <div className="text-sm text-gray-400 mb-2">Transaction</div>
      <div className="truncate font-mono text-sm mb-3">{txHash}</div>

      <div className="w-full bg-gray-700 rounded-full h-2 mb-3">
        <div
          className={`h-2 rounded-full transition-all duration-300 ${stageColor}`}
          style={{
            width:
              status === "pending"
                ? "33%"
                : status === "preconfirmed"
                ? "66%"
                : "100%",
          }}
        />
      </div>

      <div className="flex flex-col gap-1 text-sm">
        <div className="flex justify-between">
          <span>{stageText}</span>
          {status === "preconfirmed" && (
            <span className="text-gray-400">
              Mini-block #{miniBlock?.id ?? "—"}
            </span>
          )}
          {status === "final" && (
            <span className="text-gray-400">Block #{blockNum}</span>
          )}
        </div>

        {status === "preconfirmed" && (
          <div className="flex justify-between text-gray-400 text-xs">
            <span>Proposer: {miniBlock?.signer ?? "0x—"}</span>
            <span>
              Latency:&nbsp;
              {latency ? `${latency.toFixed(0)} ms` : "≈150 ms target"}
            </span>
          </div>
        )}
      </div>
    </div>
  );
}

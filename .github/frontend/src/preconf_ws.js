export class PreconfWatcher {
  constructor(url) {
    this.url = url; // ws://node:8556/ws
    this.ws = null;
    this.listeners = new Map(); // txHash → callback
  }

  connect() {
    this.ws = new WebSocket(this.url);
    this.ws.onmessage = (msg) => {
      try {
        const data = JSON.parse(msg.data);
        if (!data.TxHashes) return;
        for (const h of data.TxHashes) {
          const cb = this.listeners.get(h.toLowerCase());
          if (cb) cb(data);
        }
      } catch (err) {
        console.warn("parse error", err);
      }
    };
    this.ws.onclose = () => setTimeout(() => this.connect(), 3000); // auto-reconnect
  }

  watch(txHash, onPreconfirmed) {
    this.listeners.set(txHash.toLowerCase(), onPreconfirmed);
  }

  unwatch(txHash) {
    this.listeners.delete(txHash.toLowerCase());
  }
}

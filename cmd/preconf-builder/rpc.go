package main

import (
	"encoding/json"
	"log"
	"net/http"
)

func ServeHTTPJSON(builder *Builder, addr string) {
	type reqT struct {
		Jsonrpc string          `json:"jsonrpc"`
		ID      any             `json:"id"`
		Method  string          `json:"method"`
		Params  json.RawMessage `json:"params"`
	}
	type respT struct {
		Jsonrpc string `json:"jsonrpc"`
		ID      any    `json:"id"`
		Result  any    `json:"result,omitempty"`
		Error   any    `json:"error,omitempty"`
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		var req reqT
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), 400); return
		}
		out := respT{Jsonrpc: "2.0", ID: req.ID}
		switch req.Method {
		case "preconf_getReceipt":
			var p []struct{ TxHash string `json:"txHash"` }
			_ = json.Unmarshal(req.Params, &p)
			if len(p) == 0 { out.Error = "missing params"; break }
			out.Result = builder.GetReceipt(hexToHash(p[0].TxHash))
		default:
			out.Error = "method not found"
		}
		_ = json.NewEncoder(w).Encode(out)
	})
	go func() {
		log.Printf("preconf HTTP JSON-RPC on %s", addr)
		log.Fatal(http.ListenAndServe(addr, mux))
	}()
}

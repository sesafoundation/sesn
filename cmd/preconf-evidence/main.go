package main

import (
	"context"
	"crypto/ecdsa"
	"encoding/json"
	"log"
	"math/big"
	"os"
	"time"

	"github.com/sesafoundation/sesn/accounts/abi/bind"
	"github.com/sesafoundation/sesn/common"
	"github.com/sesafoundation/sesn/ethclient"
)

type Evidence struct {
	Proposer common.Address `json:"proposer"`
	Victim   common.Address `json:"victim"`
	TxHash   common.Hash    `json:"txHash"`
	Reason   string         `json:"reason"` // "missing" | "misordered" | "constraint"
	Pay      *big.Int       `json:"payWei"`
}

func loadOwnerKey() *ecdsa.PrivateKey { panic("implement owner key loader") }

func main() {
	if len(os.Args) < 4 {
		log.Fatalf("usage: preconf-evidence <rpc> <bondContractAddr> <evidence.json>")
	}
	rpcURL := os.Args[1]
	contractAddr := common.HexToAddress(os.Args[2])
	file := os.Args[3]

	bz, err := os.ReadFile(file); if err != nil { log.Fatal(err) }
	var evs []Evidence
	if err := json.Unmarshal(bz, &evs); err != nil { log.Fatal(err) }

	cl, err := ethclient.Dial(rpcURL); if err != nil { log.Fatal(err) }
	priv := loadOwnerKey()
	auth, err := bind.NewKeyedTransactorWithChainID(priv, big.NewInt(2250)) // set your chainId
	if err != nil { log.Fatal(err) }

	bond, err := NewPreconfBondManager(contractAddr, cl) // generated via abigen
	if err != nil { log.Fatal(err) }

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	for _, e := range evs {
		// Slash(proposer, victim, pay)
		tx, err := bond.Slash(auth, e.Proposer, e.Victim, e.Pay)
		if err != nil { log.Printf("slash failed for %s: %v", e.TxHash.Hex(), err); continue }
		log.Printf("slash submitted: %s (tx %s)", e.TxHash, tx.Hash())
	}
}

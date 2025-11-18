package main

import (
	"github.com/naoina/toml"
	"github.com/sesafoundation/sesn/log"
)

// Default TOML config
const defaultBuilderConfig = `
IPCPath       = "/node1/setd.ipc"
Cadence       = "100ms"
MaxTxPerSlice = 1500
GasSlice      = 3000000
WSListen      = ":8556"
HTTPListen    = ":8557"
NetworkID     = 2250
KeystorePath  = "/n1/keystore/0xf3a42f681d6070afd77cd206e79ef6dbdf5604eb.json"
KeyPassword   = "/n1/key.txt"
`

func loadDefaultConfig(raw *RawConfig) error {
	log.Trace("Loading preconf-builder default config")
	return toml.Unmarshal([]byte(defaultBuilderConfig), raw)
}
package main

import (
	"github.com/naoina/toml"
	"github.com/sesafoundation/sesn/log"
	"time"
)

const defaultBuilderConfig = `
IPCPath       = "~/.sesa/geth.ipc"
Cadence       = "100ms"
MaxTxPerSlice = 1500
GasSlice      = 3000000
WSListen      = ":8556"
HTTPListen    = ":8557"
NetworkID     = 2250
`

type BuilderConfig struct {
	IPCPath       string        `toml:"IPCPath"`
	Cadence       time.Duration `toml:"Cadence"`
	MaxTxPerSlice int           `toml:"MaxTxPerSlice"`
	GasSlice      uint64        `toml:"GasSlice"`
	WSListen      string        `toml:"WSListen"`
	HTTPListen    string        `toml:"HTTPListen"`
	NetworkID     uint64        `toml:"NetworkID"`
}

func loadDefaultConfig(cfg *BuilderConfig) error {
	log.Trace("Loading preconf-builder default config")
	return toml.Unmarshal([]byte(defaultBuilderConfig), cfg)
}

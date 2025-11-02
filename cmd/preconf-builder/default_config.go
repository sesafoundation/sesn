package main

import (
	"github.com/naoina/toml"
	"github.com/sesafoundation/sesn/log"
)

const defaultBuilderConfig = `
[Builder]
IPCPath = "~/.sesa/geth.ipc"
Cadence = "100ms"
WSPort  = ":8556"
HTTPPort = ":8557"
`

type BuilderConfig struct {
	Builder struct {
		IPCPath  string
		Cadence  string
		WSPort   string
		HTTPPort string
	}
}

// loadDefaultConfig loads the default preconf-builder config.
func loadDefaultConfig(cfg *BuilderConfig) error {
	log.Trace("loading preconf-builder default config")
	return toml.Unmarshal([]byte(defaultBuilderConfig), cfg)
}

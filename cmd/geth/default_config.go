package main

import (
	"github.com/ethereum/go-ethereum/log"
	"github.com/naoina/toml"
)

// defaultMainnetConfig - default config for sesn mainnet
const defaultMainnetConfig = `[Eth]
NetworkId = 2250
SyncMode = "fast"
NoPruning = false
NoPrefetch = false
LightPeers = 100
UltraLightFraction = 75
DatabaseCache = 512
DatabaseFreezer = ""
TrieCleanCache = 256
TrieDirtyCache = 256
TrieTimeout = 100000000000
EnablePreimageRecording = false
EWASMInterpreter = ""
EVMInterpreter = ""

[Eth.Miner]
GasFloor = 30000000
GasCeil = 42000000
GasPrice = 500000000000
Recommit = 10000000000
Noverify = false

[Eth.TxPool]
Locals = []
NoLocals = true
Journal = "transactions.rlp"
Rejournal = 3600000000000
PriceLimit = 500000000000
PriceBump = 10
AccountSlots = 16
GlobalSlots = 4096
AccountQueue = 64
GlobalQueue = 1024
Lifetime = 10800000000000

[Eth.GPO]
Blocks = 20
Percentile = 60

[Node]
IPCPath = "setd.ipc"
HTTPHost = "localhost"
NoUSB = true
InsecureUnlockAllowed = false
HTTPPort = 8545
HTTPVirtualHosts = ["localhost"]
HTTPModules = ["eth", "net", "web3", "txpool", "sonium"]
WSPort = 8546
WSModules = ["eth", "net", "web3", "txpool", "sonium"]

[Node.P2P]
MaxPeers = 200
NoDiscovery = false
StaticNodes = ["enode://84cfa030e398975b92e5dfc7c98be8b79249e4179b2173bf013fef9917eebe0c79087bea1d2f312d590390de3f3cc3f75def460bdf060b73846ff1a1f0542fc7@51.255.214.79:32250", "enode://b77259a26539ed45312dff2882d5ea2226331ef57312328edad0e46b07fdca7e00de056ebdc4cafd815b68f69d3db809300c14f4cb2dcf9a2e84bda6dc8f491a@147.135.201.102:32250", "enode://7fbf4f0f14a808aab87d8cab90707e008fb3664da36c46904f822b365c9a59b13d153b20d574d5d3a3a7ab8f4a1fa42c8c83eb0cbf628acde04b1e05fa749a47@147.145.201.103:32250"]
TrustedNodes = []
ListenAddr = ":32250"
EnableMsgEvents = false

[Node.HTTPTimeouts]
ReadTimeout = 30000000000
WriteTimeout = 30000000000
IdleTimeout = 120000000000`

// defaultTestnetConfig - default config for sesn testnet
const defaultTestnetConfig = `[Eth]
NetworkId = 2249
SyncMode = "fast"
NoPruning = false
NoPrefetch = false
LightPeers = 100
UltraLightFraction = 75
DatabaseCache = 512
DatabaseFreezer = ""
TrieCleanCache = 256
TrieDirtyCache = 256
TrieTimeout = 100000000000
EnablePreimageRecording = false
EWASMInterpreter = ""
EVMInterpreter = ""

[Eth.Miner]
GasFloor = 30000000
GasCeil = 42000000
GasPrice = 500000000000
Recommit = 10000000000
Noverify = false

[Eth.TxPool]
Locals = []
NoLocals = true
Journal = "transactions.rlp"
Rejournal = 3600000000000
PriceLimit = 500000000000
PriceBump = 10
AccountSlots = 16
GlobalSlots = 4096
AccountQueue = 64
GlobalQueue = 1024
Lifetime = 10800000000000

[Eth.GPO]
Blocks = 20
Percentile = 60

[Node]
IPCPath = "setd.ipc"
HTTPHost = "localhost"
NoUSB = true
InsecureUnlockAllowed = false
HTTPPort = 8545
HTTPVirtualHosts = ["localhost"]
HTTPModules = ["eth", "net", "web3", "txpool", "sonium"]
WSPort = 8546
WSModules = ["eth", "net", "web3", "txpool", "sonium"]

[Node.P2P]
MaxPeers = 200
NoDiscovery = false
StaticNodes = ["enode://84cfa030e398975b92e5dfc7c98be8b79249e4179b2173bf013fef9917eebe0c79087bea1d2f312d590390de3f3cc3f75def460bdf060b73846ff1a1f0542fc7@51.255.214.79:32249", "enode://b77259a26539ed45312dff2882d5ea2226331ef57312328edad0e46b07fdca7e00de056ebdc4cafd815b68f69d3db809300c14f4cb2dcf9a2e84bda6dc8f491a@147.135.201.102:32249"]
TrustedNodes = []
ListenAddr = ":32249"
EnableMsgEvents = false

[Node.HTTPTimeouts]
ReadTimeout = 30000000000
WriteTimeout = 30000000000
IdleTimeout = 120000000000`

// loadDefaultConfig - load default config for sesn
func loadDefaultConfig(cfg *gethConfig, isTestnet bool) error {
	if isTestnet {
		log.Trace("load testnet default config")
		return toml.Unmarshal([]byte(defaultTestnetConfig), cfg)
	}
	log.Trace("load mainnet default config")
	return toml.Unmarshal([]byte(defaultMainnetConfig), cfg)
}

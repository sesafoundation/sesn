// Copyright 2017 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package eth

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"math/big"
	"os"
	"runtime"
	"sync"
	"time"

	"github.com/sesafoundation/sesn/common"
	"github.com/sesafoundation/sesn/common/hexutil"
	"github.com/sesafoundation/sesn/consensus"
	"github.com/sesafoundation/sesn/consensus/sonium"
	"github.com/sesafoundation/sesn/core"
	"github.com/sesafoundation/sesn/core/rawdb"
	"github.com/sesafoundation/sesn/core/state"
	"github.com/sesafoundation/sesn/core/types"
	"github.com/sesafoundation/sesn/core/vm"
	"github.com/sesafoundation/sesn/eth/tracers"
	"github.com/sesafoundation/sesn/internal/ethapi"
	"github.com/sesafoundation/sesn/log"
	"github.com/sesafoundation/sesn/params"
	"github.com/sesafoundation/sesn/rlp"
	"github.com/sesafoundation/sesn/rpc"
	"github.com/sesafoundation/sesn/trie"
)

const (
	// defaultTraceTimeout is the amount of time a single transaction can execute
	// by default before being forcefully aborted.
	defaultTraceTimeout = 5 * time.Second

	// defaultTraceReexec is the number of blocks the tracer is willing to go back
	// and reexecute to produce missing historical state necessary to run a specific
	// trace.
	defaultTraceReexec = uint64(128)
)

// TraceConfig holds extra parameters to trace functions.
type TraceConfig struct {
	*vm.LogConfig
	Tracer  *string
	Timeout *string
	Reexec  *uint64
}

// StdTraceConfig holds extra parameters to standard-json trace functions.
type StdTraceConfig struct {
	vm.LogConfig
	Reexec *uint64
	TxHash common.Hash
}

// txTraceResult is the result of a single transaction trace.
type txTraceResult struct {
	Result interface{} `json:"result,omitempty"` // Trace results produced by the tracer
	Error  string      `json:"error,omitempty"`  // Trace failure produced by the tracer
}

// blockTraceTask represents a single block trace task when an entire chain is
// being traced.
type blockTraceTask struct {
	statedb *state.StateDB   // Intermediate state prepped for tracing
	block   *types.Block     // Block to trace the transactions from
	rootref common.Hash      // Trie root reference held for this task
	results []*txTraceResult // Trace results procudes by the task
}

// blockTraceResult represets the results of tracing a single block when an entire
// chain is being traced.
type blockTraceResult struct {
	Block  hexutil.Uint64   `json:"block"`  // Block number corresponding to this trace
	Hash   common.Hash      `json:"hash"`   // Block hash corresponding to this trace
	Traces []*txTraceResult `json:"traces"` // Trace results produced by the task
}

// txTraceTask represents a single transaction trace task when an entire block
// is being traced.
type txTraceTask struct {
	statedb *state.StateDB // Intermediate state prepped for tracing
	index   int            // Transaction offset in the block
}


func (api *PrivateDebugAPI) TraceChain(ctx context.Context, start, end rpc.BlockNumber, config *TraceConfig) (*rpc.Subscription, error) {

    var from, to *types.Block

    if start == rpc.PendingBlockNumber {
        if api.backend.Miner() != nil {
            from = api.backend.Miner().PendingBlock()
        }
    } else if start == rpc.LatestBlockNumber {
        from = api.backend.CurrentBlock()
    } else {
        from, _ = api.backend.BlockByNumber(ctx, start)
    }

    if end == rpc.PendingBlockNumber {
        if api.backend.Miner() != nil {
            to = api.backend.Miner().PendingBlock()
        }
    } else if end == rpc.LatestBlockNumber {
        to = api.backend.CurrentBlock()
    } else {
        to, _ = api.backend.BlockByNumber(ctx, end)
    }

    if from == nil {
        return nil, fmt.Errorf("starting block #%d not found", start)
    }
    if to == nil {
        return nil, fmt.Errorf("end block #%d not found", end)
    }
    if from.Number().Cmp(to.Number()) >= 0 {
        return nil, fmt.Errorf("end block (#%d) must be after start block (#%d)", end, start)
    }

    return api.traceChain(ctx, from, to, config)
}


// traceChain configures a new tracer according to the provided configuration, and
// executes all the transactions contained within. The return value will be one item
// per transaction, dependent on the requested tracer.
func (api *PrivateDebugAPI) traceChain(ctx context.Context, start, end *types.Block, config *TraceConfig) (*rpc.Subscription, error) {

    notifier, ok := rpc.NotifierFromContext(ctx)
    if !ok {
        return &rpc.Subscription{}, rpc.ErrNotificationsUnsupported
    }
    sub := notifier.CreateSubscription()

    // initial state: parent of start block
    database := state.NewDatabaseWithConfig(api.backend.ChainDb(), &trie.Config{Cache: 16, Preimages: true})

    if number := start.NumberU64(); number > 0 {
       // parent := api.backend.BlockChain().GetBlock(start.ParentHash(), number-1)
	   parent := api.backend.BlockChain().GetBlock(block.ParentHash(), block.NumberU64()-1)

        if parent == nil {
            return nil, fmt.Errorf("parent block #%d not found", number-1)
        }
        start = parent
    }

    statedb, err := state.New(start.Root(), database, nil)
    if err != nil {
        reexec := uint64(128)
        if config != nil && config.Reexec != nil {
            reexec = *config.Reexec
        }
        for i := uint64(0); i < reexec; i++ {
            start = api.backend.BlockChain().GetBlock(start.ParentHash(), start.NumberU64()-1)
            if start == nil {
                break
            }
            if statedb, err = state.New(start.Root(), database, nil); err == nil {
                break
            }
        }
        if err != nil {
            return nil, errors.New("required historical state unavailable")
        }
    }

    blocksCount := int(end.NumberU64() - start.NumberU64())
    workers := runtime.NumCPU()
    if workers > blocksCount {
        workers = blocksCount
    }

    pend := new(sync.WaitGroup)
    tasks := make(chan *blockTraceTask, workers)
    results := make(chan *blockTraceTask, workers)

    for w := 0; w < workers; w++ {
        pend.Add(1)
        go func() {
            defer pend.Done()
            for task := range tasks {
                signer := types.MakeSigner(api.backend.BlockChain().Config(), task.block.Number())
                blockCtx := core.NewEVMBlockContext(task.block.Header(), api.backend.BlockChain(), nil)

                for i, tx := range task.block.Transactions() {
                    msg, _ := tx.AsMessage(signer)
                    res, err := api.traceTx(ctx, msg, blockCtx, task.statedb, config)
                    if err != nil {
                        task.results[i] = &txTraceResult{Error: err.Error()}
                        break
                    }
                    task.statedb.Finalise(api.backend.BlockChain().Config().IsEIP158(task.block.Number()))
                    task.results[i] = &txTraceResult{Result: res}
                }

                select {
                case results <- task:
                case <-notifier.Closed():
                    return
                }
            }
        }()
    }

    begin := time.Now()

    go func() {
        var (
            logged time.Time
            number uint64 = start.NumberU64()
            traced uint64
            failed error
            proot  common.Hash
        )

        defer func() {
            close(tasks)
            pend.Wait()
            close(results)
            if failed != nil {
                log.Warn("Chain tracing failed", "start", start.NumberU64(), "end", end.NumberU64(), "err", failed)
            } else {
                log.Info("Chain tracing finished", "start", start.NumberU64(), "end", end.NumberU64(), "txs", traced, "elapsed", time.Since(begin))
            }
        }()

        for number < end.NumberU64() {
            number++

            block := api.backend.BlockChain().GetBlockByNumber(number)
            if block == nil {
                failed = fmt.Errorf("block #%d not found", number)
                return
            }

            if number > start.NumberU64() {
                txs := block.Transactions()
                select {
                case tasks <- &blockTraceTask{statedb: statedb.Copy(), block: block, rootref: proot, results: make([]*txTraceResult, len(txs))}:
                case <-notifier.Closed():
                    return
                }
                traced += uint64(len(txs))
            }

            _, _, _, err := api.backend.BlockChain().Processor().Process(block, statedb, vm.Config{})
            if err != nil {
                failed = err
                return
            }

            root, err := statedb.Commit(api.backend.BlockChain().Config().IsEIP158(block.Number()))
            if err != nil {
                failed = err
                return
            }
            if err := statedb.Reset(root); err != nil {
                failed = err
                return
            }
            database.TrieDB().Reference(root, common.Hash{})
            if proot != (common.Hash{}) {
                database.TrieDB().Dereference(proot)
            }
            proot = root

            if time.Since(logged) > 8*time.Second {
                log.Info("Tracing chain progress", "current", number, "elapsed", time.Since(begin))
                logged = time.Now()
            }
        }
    }()

    go func() {
        done := make(map[uint64]*blockTraceResult)
        next := start.NumberU64() + 1

        for res := range results {
            result := &blockTraceResult{
                Block:  hexutil.Uint64(res.block.NumberU64()),
                Hash:   res.block.Hash(),
                Traces: res.results,
            }
            done[uint64(result.Block)] = result

            for {
                r, ok := done[next]
                if !ok {
                    break
                }
                notifier.Notify(sub.ID, r)
                delete(done, next)
                next++
            }
        }
    }()

    return sub, nil
}

// TraceBlockByNumber returns the structured logs created during the execution of
// EVM and returns them as a JSON object.
func (api *PrivateDebugAPI) TraceBlockByNumber(ctx context.Context, number rpc.BlockNumber, config *TraceConfig) ([]*txTraceResult, error) {
	// Fetch the block that we want to trace
	var block *types.Block

	switch number {
	case rpc.PendingBlockNumber:
		block = api.backend.Miner().PendingBlock()
	case rpc.LatestBlockNumber:
		block = api.backend.BlockChain().CurrentBlock()
	default:
		//block = api.backend.BlockChain().GetBlockByNumber(uint64(number))
		block, _ := api.backend.BlockByNumber(ctx, number)
	}
	// Trace the block if it was found
	if block == nil {
		return nil, fmt.Errorf("block #%d not found", number)
	}
	return api.traceBlock(ctx, block, config)
}

// TraceBlockByHash returns the structured logs created during the execution of
// EVM and returns them as a JSON object.
func (api *PrivateDebugAPI) TraceBlockByHash(ctx context.Context, hash common.Hash, config *TraceConfig) ([]*txTraceResult, error) {
	//block := api.backend.BlockChain().GetBlockByHash(hash)
	block := api.backend.BlockChain().GetBlockByHash(hash)

	if block == nil {
		return nil, fmt.Errorf("block %#x not found", hash)
	}
	return api.traceBlock(ctx, block, config)
}

// TraceBlock returns the structured logs created during the execution of EVM
// and returns them as a JSON object.
func (api *PrivateDebugAPI) TraceBlock(ctx context.Context, blob []byte, config *TraceConfig) ([]*txTraceResult, error) {
	block := new(types.Block)
	if err := rlp.Decode(bytes.NewReader(blob), block); err != nil {
		return nil, fmt.Errorf("could not decode block: %v", err)
	}
	return api.traceBlock(ctx, block, config)
}

// TraceBlockFromFile returns the structured logs created during the execution of
// EVM and returns them as a JSON object.
func (api *PrivateDebugAPI) TraceBlockFromFile(ctx context.Context, file string, config *TraceConfig) ([]*txTraceResult, error) {
	blob, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, fmt.Errorf("could not read file: %v", err)
	}
	return api.TraceBlock(ctx, blob, config)
}

// TraceBadBlock returns the structured logs created during the execution of
// EVM against a block pulled from the pool of bad ones and returns them as a JSON
// object.
func (api *PrivateDebugAPI) TraceBadBlock(ctx context.Context, hash common.Hash, config *TraceConfig) ([]*txTraceResult, error) {
	blocks := api.backend.BlockChain().BadBlocks()
	for _, block := range blocks {
		if block.Hash() == hash {
			return api.traceBlock(ctx, block, config)
		}
	}
	return nil, fmt.Errorf("bad block %#x not found", hash)
}

// StandardTraceBlockToFile dumps the structured logs created during the
// execution of EVM to the local file system and returns a list of files
// to the caller.

func (api *PrivateDebugAPI) StandardTraceBlockToFile(ctx context.Context, hash common.Hash, config *StdTraceConfig) ([]string, error) {
    block := api.backend.BlockChain().GetBlockByHash(hash)
    if block == nil {
        return nil, fmt.Errorf("block %#x not found", hash)
    }

    // If tracing a specific transaction ensure block contains it
    if config != nil && config.TxHash != (common.Hash{}) {
        if !containsTx(block, config.TxHash) {
            return nil, fmt.Errorf("transaction %#x not found in block", config.TxHash)
        }
    }

    // Parent state DB
    parent := api.backend.BlockChain().GetBlock(block.ParentHash(), block.NumberU64()-1)
    if parent == nil {
        return nil, fmt.Errorf("parent %#x not found", block.ParentHash())
    }
    reexec := defaultTraceReexec
    if config != nil && config.Reexec != nil {
        reexec = *config.Reexec
    }
    statedb, err := api.computeStateDB(parent, reexec)
    if err != nil {
        return nil, err
    }

    signer := types.MakeSigner(api.backend.BlockChain().Config(), block.Number())
    vmctx := core.NewEVMBlockContext(block.Header(), api.backend.BlockChain(), nil)

    var (
        logCfg    vm.LogConfig
        dumps     []string
    )
    if config != nil {
        logCfg = config.LogConfig
    }
    logCfg.Debug = true

    for i, tx := range block.Transactions() {
        msg, _ := tx.AsMessage(signer)
        txctx := core.NewEVMTxContext(msg)

        var vmConf vm.Config
        var file *os.File
        var writer *bufio.Writer

        // If this TX must be logged → create trace file + tracer config
        if config == nil || config.TxHash == (common.Hash{}) || config.TxHash == tx.Hash() {
            prefix := fmt.Sprintf("block_%#x_%d_%#x_", block.Hash().Bytes()[:4], i, tx.Hash().Bytes()[:4])
            file, err = ioutil.TempFile(os.TempDir(), prefix)
            if err != nil {
                return nil, err
            }
            writer = bufio.NewWriter(file)
            vmConf = vm.Config{
                Debug:                   true,
                Tracer:                  vm.NewJSONLogger(&logCfg, writer),
                EnablePreimageRecording: true,
            }
            dumps = append(dumps, file.Name())
        }

        // Execute
        evm := vm.NewEVM(vmctx, txctx, statedb, api.backend.BlockChain().Config(), vmConf)
        if _, err := core.ApplyMessage(evm, msg, new(core.GasPool).AddGas(msg.Gas())); err != nil {
            return dumps, err
        }

        // Finalize and sync trie
        statedb.Finalise(evm.ChainConfig().IsEIP158(block.Number()))

        if writer != nil {
            writer.Flush()
        }
        if file != nil {
            file.Close()
            log.Info("Wrote standard trace", "file", file.Name())
        }

        if config != nil && config.TxHash == tx.Hash() {
            break
        }
    }
    return dumps, nil
}


// StandardTraceBadBlockToFile dumps the structured logs created during the
// execution of EVM against a block pulled from the pool of bad ones to the
// local file system and returns a list of files to the caller.
func (api *PrivateDebugAPI) StandardTraceBadBlockToFile(ctx context.Context, hash common.Hash, config *StdTraceConfig) ([]string, error) {
	blocks := api.backend.BlockChain().BadBlocks()
	for _, block := range blocks {
		if block.Hash() == hash {
			return api.standardTraceBlockToFile(ctx, block, config)
		}
	}
	return nil, fmt.Errorf("bad block %#x not found", hash)
}

// traceBlock configures a new tracer according to the provided configuration, and
// executes all the transactions contained within. The return value will be one item
// per transaction, dependent on the requestd tracer.
func (api *PrivateDebugAPI) traceBlock(ctx context.Context, block *types.Block, config *TraceConfig) ([]*txTraceResult, error) {
	// Create the parent state database
	if err := api.backend.Engine().VerifyHeader(api.backend.BlockChain(), block.Header(), true); err != nil {
		return nil, err
	}
	//parent := api.backend.BlockChain().GetBlock(block.ParentHash(), block.NumberU64()-1)
	parent := api.backend.BlockChain().GetBlock(block.ParentHash(), block.NumberU64()-1)

	if parent == nil {
		return nil, fmt.Errorf("parent %#x not found", block.ParentHash())
	}
	reexec := defaultTraceReexec
	if config != nil && config.Reexec != nil {
		reexec = *config.Reexec
	}
	statedb, err := api.computeStateDB(parent, reexec)
	if err != nil {
		return nil, err
	}
	// Execute all the transaction contained within the block concurrently
	var (
		signer = types.MakeSigner(api.backend.BlockChain().Config(), block.Number())

		txs     = block.Transactions()
		results = make([]*txTraceResult, len(txs))

		pend = new(sync.WaitGroup)
		jobs = make(chan *txTraceTask, len(txs))
	)
	threads := runtime.NumCPU()
	if threads > len(txs) {
		threads = len(txs)
	}
	blockCtx := core.NewEVMBlockContext(block.Header(), api.backend.BlockChain(), nil)
	for th := 0; th < threads; th++ {
		pend.Add(1)
		go func() {
			defer pend.Done()
			// Fetch and execute the next transaction trace tasks
			for task := range jobs {
				msg, _ := txs[task.index].AsMessage(signer)
				res, err := api.traceTx(ctx, msg, blockCtx, task.statedb, config)
				if err != nil {
					results[task.index] = &txTraceResult{Error: err.Error()}
					continue
				}
				results[task.index] = &txTraceResult{Result: res}
			}
		}()
	}
	// Feed the transactions into the tracers and return
	var failed error
	for i, tx := range txs {
		// Send the trace task over for execution
		jobs <- &txTraceTask{statedb: statedb.Copy(), index: i}

		// Generate the next state snapshot fast without tracing
		msg, _ := tx.AsMessage(signer)
		txContext := core.NewEVMTxContext(msg)
		if pos, ok := api.backend.Engine().(consensus.PoS); ok {
			if isSystemTx, _ := pos.IsSystemTransaction(tx, block.Header()); isSystemTx {
				balance := statedb.GetBalance(consensus.FeeRecoder)
				if balance.Cmp(common.Big0) > 0 {
					statedb.SetBalance(consensus.FeeRecoder, big.NewInt(0))
				}
				blockReward := sonium.CalcBlockReward(api.backend.BlockChain().Config(), block.Number())
				reward := big.NewInt(0).Set(balance)
				reward = reward.Add(reward, blockReward)
				if reward.Cmp(common.Big0) > 0 {
					statedb.AddBalance(block.Header().Coinbase, reward)
				}
			}
		}
		//vmenv := vm.NewEVM(blockCtx, txContext, statedb, api.backend.BlockChain().Config(), vm.Config{})
		vmenv := vm.NewEVM(vmctx, txContext, statedb, api.backend.BlockChain().Config(), vmConf)

		if _, err := core.ApplyMessage(vmenv, msg, new(core.GasPool).AddGas(msg.Gas())); err != nil {
			failed = err
			break
		}
		// Finalize the state so any modifications are written to the trie
		// Only delete empty objects if EIP158/161 (a.k.a Spurious Dragon) is in effect
		statedb.Finalise(vmenv.ChainConfig().IsEIP158(block.Number()))
	}
	close(jobs)
	pend.Wait()

	// If execution failed in between, abort
	if failed != nil {
		return nil, failed
	}
	return results, nil
}

// standardTraceBlockToFile configures a new tracer which uses standard JSON output,
// and traces either a full block or an individual transaction. The return value will
// be one filename per transaction traced.
func (api *PrivateDebugAPI) StandardTraceBlockToFile(ctx context.Context, hash common.Hash, config *StdTraceConfig) ([]string, error) {
    block := api.backend.BlockChain().GetBlockByHash(hash)
    if block == nil {
        return nil, fmt.Errorf("block %#x not found", hash)
    }

    // If tracing a specific transaction ensure block contains it
    if config != nil && config.TxHash != (common.Hash{}) {
        if !containsTx(block, config.TxHash) {
            return nil, fmt.Errorf("transaction %#x not found in block", config.TxHash)
        }
    }

    // Parent state DB
    parent := api.backend.BlockChain().GetBlock(block.ParentHash(), block.NumberU64()-1)
    if parent == nil {
        return nil, fmt.Errorf("parent %#x not found", block.ParentHash())
    }
    reexec := defaultTraceReexec
    if config != nil && config.Reexec != nil {
        reexec = *config.Reexec
    }
    statedb, err := api.computeStateDB(parent, reexec)
    if err != nil {
        return nil, err
    }

    signer := types.MakeSigner(api.backend.BlockChain().Config(), block.Number())
    vmctx := core.NewEVMBlockContext(block.Header(), api.backend.BlockChain(), nil)

    var (
        logCfg    vm.LogConfig
        dumps     []string
    )
    if config != nil {
        logCfg = config.LogConfig
    }
    logCfg.Debug = true

    for i, tx := range block.Transactions() {
        msg, _ := tx.AsMessage(signer)
        txctx := core.NewEVMTxContext(msg)

        var vmConf vm.Config
        var file *os.File
        var writer *bufio.Writer

        // If this TX must be logged → create trace file + tracer config
        if config == nil || config.TxHash == (common.Hash{}) || config.TxHash == tx.Hash() {
            prefix := fmt.Sprintf("block_%#x_%d_%#x_", block.Hash().Bytes()[:4], i, tx.Hash().Bytes()[:4])
            file, err = ioutil.TempFile(os.TempDir(), prefix)
            if err != nil {
                return nil, err
            }
            writer = bufio.NewWriter(file)
            vmConf = vm.Config{
                Debug:                   true,
                Tracer:                  vm.NewJSONLogger(&logCfg, writer),
                EnablePreimageRecording: true,
            }
            dumps = append(dumps, file.Name())
        }

        // Execute
        evm := vm.NewEVM(vmctx, txctx, statedb, api.backend.BlockChain().Config(), vmConf)
        if _, err := core.ApplyMessage(evm, msg, new(core.GasPool).AddGas(msg.Gas())); err != nil {
            return dumps, err
        }

        // Finalize and sync trie
        statedb.Finalise(evm.ChainConfig().IsEIP158(block.Number()))

        if writer != nil {
            writer.Flush()
        }
        if file != nil {
            file.Close()
            log.Info("Wrote standard trace", "file", file.Name())
        }

        if config != nil && config.TxHash == tx.Hash() {
            break
        }
    }
    return dumps, nil
}


// computeStateDB retrieves the state database associated with a certain block.
// If no state is locally available for the given block, a number of blocks are
// attempted to be reexecuted to generate the desired state.
func (api *PrivateDebugAPI) computeStateDB(block *types.Block, reexec uint64) (*state.StateDB, error) {
	// If we have the state fully available, use that
	statedb, err := api.backend.BlockChain().StateAt(block.Root())
	if err == nil {
		return statedb, nil
	}
	// Otherwise try to reexec blocks until we find a state or reach our limit
	origin := block.NumberU64()
	database := state.NewDatabaseWithConfig(api.backend.ChainDb(), &trie.Config{Cache: 16, Preimages: true})

	for i := uint64(0); i < reexec; i++ {
		block = api.backend.BlockChain().GetBlock(block.ParentHash(), block.NumberU64()-1)
		if block == nil {
			break
		}
		if statedb, err = state.New(block.Root(), database, nil); err == nil {
			break
		}
	}
	if err != nil {
		switch err.(type) {
		case *trie.MissingNodeError:
			return nil, fmt.Errorf("required historical state unavailable (reexec=%d)", reexec)
		default:
			return nil, err
		}
	}
	// State was available at historical point, regenerate
	var (
		start  = time.Now()
		logged time.Time
		proot  common.Hash
	)
	for block.NumberU64() < origin {
		// Print progress logs if long enough time elapsed
		if time.Since(logged) > 8*time.Second {
			log.Info("Regenerating historical state", "block", block.NumberU64()+1, "target", origin, "remaining", origin-block.NumberU64()-1, "elapsed", time.Since(start))
			logged = time.Now()
		}
		// Retrieve the next block to regenerate and process it
		if block = api.backend.BlockChain().GetBlockByNumber(block.NumberU64() + 1); block == nil {
			return nil, fmt.Errorf("block #%d not found", block.NumberU64()+1)
		}
		_, _, _, err := api.backend.BlockChain().Processor().Process(block, statedb, vm.Config{})
		if err != nil {
			return nil, fmt.Errorf("processing block %d failed: %v", block.NumberU64(), err)
		}
		// Finalize the state so any modifications are written to the trie
		root, err := statedb.Commit(api.backend.BlockChain().Config().IsEIP158(block.Number()))
		if err != nil {
			return nil, err
		}
		if err := statedb.Reset(root); err != nil {
			return nil, fmt.Errorf("state reset after block %d failed: %v", block.NumberU64(), err)
		}
		database.TrieDB().Reference(root, common.Hash{})
		if proot != (common.Hash{}) {
			database.TrieDB().Dereference(proot)
		}
		proot = root
	}
	nodes, imgs := database.TrieDB().Size()
	log.Info("Historical state regenerated", "block", block.NumberU64(), "elapsed", time.Since(start), "nodes", nodes, "preimages", imgs)
	return statedb, nil
}

// TraceTransaction returns the structured logs created during the execution of EVM
// and returns them as a JSON object.
func (api *PrivateDebugAPI) TraceTransaction(ctx context.Context, hash common.Hash, config *TraceConfig) (interface{}, error) {
	// Retrieve the transaction and assemble its EVM context
	tx, blockHash, _, index := rawdb.ReadTransaction(api.backend.ChainDb(), hash)
	if tx == nil {
		return nil, fmt.Errorf("transaction %#x not found", hash)
	}
	reexec := defaultTraceReexec
	if config != nil && config.Reexec != nil {
		reexec = *config.Reexec
	}
	// Retrieve the block
	block := api.backend.BlockChain().GetBlockByHash(blockHash)
	if block == nil {
		return nil, fmt.Errorf("block %#x not found", blockHash)
	}
	msg, vmctx, statedb, err := api.computeTxEnv(block, int(index), reexec)
	if err != nil {
		return nil, err
	}
	// Trace the transaction and return
	return api.traceTx(ctx, msg, vmctx, statedb, config)
}

// TraceCall lets you trace a given eth_call. It collects the structured logs created during the execution of EVM
// if the given transaction was added on top of the provided block and returns them as a JSON object.
// You can provide -2 as a block number to trace on top of the pending block.
func (api *PrivateDebugAPI) TraceCall(ctx context.Context, args ethapi.CallArgs, blockNrOrHash rpc.BlockNumberOrHash, config *TraceConfig) (interface{}, error) {
	// First try to retrieve the state
	statedb, header, err := api.backend.StateAndHeaderByNumberOrHash(ctx, blockNrOrHash)
	if err != nil {
		// Try to retrieve the specified block
		var block *types.Block
		if hash, ok := blockNrOrHash.Hash(); ok {
			block = api.backend.BlockChain().GetBlockByHash(hash)
		} else if number, ok := blockNrOrHash.Number(); ok {
			block = api.backend.BlockChain().GetBlockByNumber(uint64(number))
		}
		if block == nil {
			return nil, fmt.Errorf("block %v not found: %v", blockNrOrHash, err)
		}
		// try to recompute the state
		reexec := defaultTraceReexec
		if config != nil && config.Reexec != nil {
			reexec = *config.Reexec
		}
		_, _, statedb, err = api.computeTxEnv(block, 0, reexec)
		if err != nil {
			return nil, err
		}
	}

	// Execute the trace
	msg := args.ToMessage(api.backend.RPCGasCap())
	vmctx := core.NewEVMBlockContext(header, api.backend.BlockChain(), nil)
	return api.traceTx(ctx, msg, vmctx, statedb, config)
}

// traceTx configures a new tracer according to the provided configuration, and
// executes the given message in the provided environment. The return value will
// be tracer dependent.
func (api *PrivateDebugAPI) traceTx(ctx context.Context, message core.Message, vmctx vm.BlockContext, statedb *state.StateDB, config *TraceConfig) (interface{}, error) {


	// Assemble the structured logger or the JavaScript tracer
	var (
		tracer    vm.Tracer
		err       error
		txContext = core.NewEVMTxContext(message)
	)
	switch {
	case config != nil && config.Tracer != nil:
		// Define a meaningful timeout of a single transaction trace
		timeout := defaultTraceTimeout
		if config.Timeout != nil {
			if timeout, err = time.ParseDuration(*config.Timeout); err != nil {
				return nil, err
			}
		}
		// Constuct the JavaScript tracer to execute with
		if tracer, err = tracers.New(*config.Tracer); err != nil {
			return nil, err
		}
		// Handle timeouts and RPC cancellations
		deadlineCtx, cancel := context.WithTimeout(ctx, timeout)
		go func() {
			<-deadlineCtx.Done()
			tracer.(*tracers.Tracer).Stop(errors.New("execution timeout"))
		}()
		defer cancel()

	case config == nil:
		tracer = vm.NewStructLogger(nil)

	default:
		tracer = vm.NewStructLogger(config.LogConfig)
	}
	// Run the transaction with tracing enabled.
	vmenv := vm.NewEVM(vmctx, txContext, statedb, api.backend.BlockChain().Config(), vm.Config{Debug: true, Tracer: tracer})
	if pos, ok := api.backend.Engine().(consensus.PoS); ok && pos.IsSystemContract(message.To()) && message.From() == vmctx.Coinbase && message.GasPrice().Cmp(big.NewInt(0)) == 0 {
		balance := statedb.GetBalance(consensus.FeeRecoder)
		reward := big.NewInt(0)
		if balance.Cmp(common.Big0) > 0 {
			statedb.SetBalance(consensus.FeeRecoder, big.NewInt(0))
			reward = reward.Add(reward, balance)
		}
		blockReward := sonium.CalcBlockReward(api.backend.BlockChain().Config(), vmctx.BlockNumber)
		reward = reward.Add(reward, blockReward)
		if reward.Cmp(common.Big0) > 0 {
			statedb.AddBalance(vmctx.Coinbase, reward)
		}
	}
	result, err := core.ApplyMessage(vmenv, message, new(core.GasPool).AddGas(message.Gas()))
	if err != nil {
		return nil, fmt.Errorf("tracing failed: %v", err)
	}
	// Depending on the tracer type, format and return the output
	switch tracer := tracer.(type) {
	case *vm.StructLogger:
		// If the result contains a revert reason, return it.
		returnVal := fmt.Sprintf("%x", result.Return())
		if len(result.Revert()) > 0 {
			returnVal = fmt.Sprintf("%x", result.Revert())
		}
		return &ethapi.ExecutionResult{
			Gas:         result.UsedGas,
			Failed:      result.Failed(),
			ReturnValue: returnVal,
			StructLogs:  ethapi.FormatLogs(tracer.StructLogs()),
		}, nil

	case *tracers.Tracer:
		return tracer.GetResult()

	default:
		panic(fmt.Sprintf("bad tracer type %T", tracer))
	}
}

// computeTxEnv returns the execution environment of a certain transaction.
func (api *PrivateDebugAPI) computeTxEnv(block *types.Block, txIndex int, reexec uint64) (core.Message, vm.BlockContext, *state.StateDB, error) {
	// Create the parent state database
	//parent := api.backend.BlockChain().GetBlock(block.ParentHash(), block.NumberU64()-1)
	parent := api.backend.BlockChain().GetBlock(block.ParentHash(), block.NumberU64()-1)
	if parent == nil {
		return nil, vm.BlockContext{}, nil, fmt.Errorf("parent %#x not found", block.ParentHash())
	}
	statedb, err := api.computeStateDB(parent, reexec)
	if err != nil {
		return nil, vm.BlockContext{}, nil, err
	}

	if txIndex == 0 && len(block.Transactions()) == 0 {
		return nil, vm.BlockContext{}, statedb, nil
	}

	// Recompute transactions up to the target index.
	signer := types.MakeSigner(api.backend.BlockChain().Config(), block.Number())

	for idx, tx := range block.Transactions() {
		// Assemble the transaction call message and return if the requested offset
		msg, _ := tx.AsMessage(signer)
		txContext := core.NewEVMTxContext(msg)
		if pos, ok := api.backend.Engine().(consensus.PoS); ok {
			if isSystemTx, _ := pos.IsSystemTransaction(tx, block.Header()); isSystemTx {
				balance := statedb.GetBalance(consensus.FeeRecoder)
				if balance.Cmp(common.Big0) > 0 {
					statedb.SetBalance(consensus.FeeRecoder, big.NewInt(0))
				}
				blockReward := sonium.CalcBlockReward(api.backend.BlockChain().Config(), block.Number())
				reward := big.NewInt(0).Set(balance)
				reward = reward.Add(reward, blockReward)
				if reward.Cmp(common.Big0) > 0 {
					statedb.AddBalance(block.Header().Coinbase, reward)
				}
			}
		}
		context := core.NewEVMBlockContext(block.Header(), api.backend.BlockChain(), nil)
		if idx == txIndex {
			return msg, context, statedb, nil
		}
		// Not yet the searched for transaction, execute on top of the current state
		//vmenv := vm.NewEVM(context, txContext, statedb, api.backend.BlockChain().Config(), vm.Config{})
		vmenv := vm.NewEVM(vmctx, txContext, statedb, api.backend.BlockChain().Config(), vmConf)

		if _, err := core.ApplyMessage(vmenv, msg, new(core.GasPool).AddGas(tx.Gas())); err != nil {
			return nil, vm.BlockContext{}, nil, fmt.Errorf("transaction %#x failed: %v", tx.Hash(), err)
		}
		// Ensure any modifications are committed to the state
		// Only delete empty objects if EIP158/161 (a.k.a Spurious Dragon) is in effect
		statedb.Finalise(vmenv.ChainConfig().IsEIP158(block.Number()))
	}
	return nil, vm.BlockContext{}, nil, fmt.Errorf("transaction index %d out of range for block %#x", txIndex, block.Hash())
}

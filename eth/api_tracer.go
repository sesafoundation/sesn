package eth

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	//"math/big"
	"os"
	"runtime"
	"sync"
	"time"

	"github.com/sesafoundation/sesn/common"
	"github.com/sesafoundation/sesn/common/hexutil"
	//"github.com/sesafoundation/sesn/consensus"
	//"github.com/sesafoundation/sesn/consensus/sonium"
	"github.com/sesafoundation/sesn/core"
	"github.com/sesafoundation/sesn/core/rawdb"
	"github.com/sesafoundation/sesn/core/state"
	"github.com/sesafoundation/sesn/core/types"
	"github.com/sesafoundation/sesn/core/vm"
	"github.com/sesafoundation/sesn/eth/tracers"
	"github.com/sesafoundation/sesn/internal/ethapi"
	"github.com/sesafoundation/sesn/log"
	//"github.com/sesafoundation/sesn/params"
	"github.com/sesafoundation/sesn/rlp"
	"github.com/sesafoundation/sesn/rpc"
	"github.com/sesafoundation/sesn/trie"
)

const (
	defaultTraceTimeout = 5 * time.Second
	defaultTraceReexec uint64 = 128
)

type TraceConfig struct {
	*vm.LogConfig
	Tracer  *string
	Timeout *string
	Reexec  *uint64
}

type StdTraceConfig struct {
	vm.LogConfig
	Reexec *uint64
	TxHash common.Hash
}

type txTraceResult struct {
	Result interface{} `json:"result,omitempty"`
	Error  string      `json:"error,omitempty"`
}

type blockTraceTask struct {
	statedb *state.StateDB
	block   *types.Block
	rootref common.Hash
	results []*txTraceResult
}

type blockTraceResult struct {
	Block  hexutil.Uint64   `json:"block"`
	Hash   common.Hash      `json:"hash"`
	Traces []*txTraceResult `json:"traces"`
}

type txTraceTask struct {
	statedb *state.StateDB
	index   int
}

// TraceChain returns the structured logs created during the execution of EVM
// between two blocks (excluding start) and returns them as a JSON object.
func (api *PrivateDebugAPI) TraceChain(
    ctx context.Context,
    start, end rpc.BlockNumber,
    config *TraceConfig,
) (*rpc.Subscription, error) {
    var from, to *types.Block

    // -------- resolve "from" block --------
    switch start {
    case rpc.PendingBlockNumber:
        if m := api.backend.Miner(); m != nil {
            from = m.PendingBlock()
        }
    case rpc.LatestBlockNumber:
        from = api.backend.CurrentBlock()
    default:
        var err error
        from, err = api.backend.BlockByNumber(ctx, start)
        if err != nil {
            return nil, err
        }
    }

    // -------- resolve "to" block --------
    switch end {
    case rpc.PendingBlockNumber:
        if m := api.backend.Miner(); m != nil {
            to = m.PendingBlock()
        }
    case rpc.LatestBlockNumber:
        to = api.backend.CurrentBlock()
    default:
        var err error
        to, err = api.backend.BlockByNumber(ctx, end)
        if err != nil {
            return nil, err
        }
    }

    // -------- basic validation --------
    if from == nil {
        return nil, fmt.Errorf("starting block #%d not found", start)
    }
    if to == nil {
        return nil, fmt.Errorf("end block #%d not found", end)
    }
    if from.Number().Cmp(to.Number()) >= 0 {
        return nil, fmt.Errorf(
            "end block (#%d) must be after start block (#%d)",
            end, start,
        )
    }

    return api.traceChain(ctx, from, to, config)
}

// traceChain is the internal worker that actually traces the chain of blocks
// from `start` (exclusive) up to and including `end`.
func (api *PrivateDebugAPI) traceChain(
    ctx context.Context,
    start, end *types.Block,
    config *TraceConfig,
) (*rpc.Subscription, error) {

    // Tracing a chain is a **long** operation, only allowed via subscriptions
    notifier, supported := rpc.NotifierFromContext(ctx)
    if !supported {
        return &rpc.Subscription{}, rpc.ErrNotificationsUnsupported
    }
    sub := notifier.CreateSubscription()

    // Ensure we have a valid starting state before doing any work
    origin := start.NumberU64()
    database := state.NewDatabaseWithConfig(
        api.backend.ChainDb(),
        &trie.Config{Cache: 16, Preimages: true},
    )

    // Move `start` back to its parent (we trace from after `start`)
    if number := start.NumberU64(); number > 0 {
        parent := api.backend.BlockChain().GetBlock(
            start.ParentHash(),
            start.NumberU64()-1,
        )
        if parent == nil {
            return nil, fmt.Errorf("parent block #%d not found", number-1)
        }
        start = parent
    }


	statedb, err := state.New(start.Root(), database, nil)
	if err != nil {
    // If the starting state is missing, allow some number of blocks to be reexecuted
    var reexec uint64 = defaultTraceReexec
    if config != nil && config.Reexec != nil {
        reexec = *config.Reexec
    }
    // Find the most recent block that has the state available
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
        switch err.(type) {
        case *trie.MissingNodeError:
            return nil, errors.New("required historical state unavailable")
        default:
            return nil, err
        }
    }
}


    // Execute all the transactions contained within the chain concurrently
    blocks := int(end.NumberU64() - origin)
    threads := runtime.NumCPU()
    if threads > blocks {
        threads = blocks
    }

    var (
        pend    = new(sync.WaitGroup)
        tasks   = make(chan *blockTraceTask, threads)
        results = make(chan *blockTraceTask, threads)
    )

    // Worker goroutines
    for th := 0; th < threads; th++ {
        pend.Add(1)
        go func() {
            defer pend.Done()
            for task := range tasks {
                signer := types.MakeSigner(
                    api.backend.ChainConfig(),
                    task.block.Number(),
                )
                blockCtx := core.NewEVMBlockContext(
                    task.block.Header(),
                    api.backend.BlockChain(),
                    nil,
                )

                // Trace all the transactions in the block
                for i, tx := range task.block.Transactions() {
                    msg, _ := tx.AsMessage(signer)
                    res, err := api.traceTx(ctx, msg, blockCtx, task.statedb, config)
                    if err != nil {
                        task.results[i] = &txTraceResult{Error: err.Error()}
                        log.Warn(
                            "Tracing failed",
                            "hash", tx.Hash(),
                            "block", task.block.NumberU64(),
                            "err", err,
                        )
                        break
                    }
                    // Only delete empty objects if EIP158/161 is in effect
                    task.statedb.Finalise(
                        api.backend.ChainConfig().IsEIP158(task.block.Number()),
                    )
                    task.results[i] = &txTraceResult{Result: res}
                }

                // Stream results or abort on teardown
                select {
                case results <- task:
                case <-notifier.Closed():
                    return
                }
            }
        }()
    }

    begin := time.Now()

    // Producer goroutine to feed blocks into tracers
    go func() {
        var (
            logged time.Time
            number uint64
            traced uint64
            failed error
            proot  common.Hash
        )

        defer func() {
            close(tasks)
            pend.Wait()

            switch {
            case failed != nil:
                log.Warn(
                    "Chain tracing failed",
                    "start", start.NumberU64(),
                    "end", end.NumberU64(),
                    "transactions", traced,
                    "elapsed", time.Since(begin),
                    "err", failed,
                )
            case number < end.NumberU64():
                log.Warn(
                    "Chain tracing aborted",
                    "start", start.NumberU64(),
                    "end", end.NumberU64(),
                    "abort", number,
                    "transactions", traced,
                    "elapsed", time.Since(begin),
                )
            default:
                log.Info(
                    "Chain tracing finished",
                    "start", start.NumberU64(),
                    "end", end.NumberU64(),
                    "transactions", traced,
                    "elapsed", time.Since(begin),
                )
            }
            close(results)
        }()

        for number = start.NumberU64() + 1; number <= end.NumberU64(); number++ {
            // Stop tracing if interruption was requested
            select {
            case <-notifier.Closed():
                return
            default:
            }

            // Progress logging
            if time.Since(logged) > 8*time.Second {
                if number > origin {
                    nodes, imgs := database.TrieDB().Size()
                    log.Info(
                        "Tracing chain segment",
                        "start", origin,
                        "end", end.NumberU64(),
                        "current", number,
                        "transactions", traced,
                        "elapsed", time.Since(begin),
                        "memory", nodes+imgs,
                    )
                } else {
                    log.Info(
                        "Preparing state for chain trace",
                        "block", number,
                        "start", origin,
                        "elapsed", time.Since(begin),
                    )
                }
                logged = time.Now()
            }

            // Retrieve block
            block := api.backend.BlockChain().GetBlockByNumber(number)
            if block == nil {
                failed = fmt.Errorf("block #%d not found", number)
                break
            }

            // Send block to workers (after origin)
            if number > origin {
                txs := block.Transactions()
                select {
                case tasks <- &blockTraceTask{
                    statedb: statedb.Copy(),
                    block:   block,
                    rootref: proot,
                    results: make([]*txTraceResult, len(txs)),
                }:
                case <-notifier.Closed():
                    return
                }
                traced += uint64(len(txs))
            }

            // Fast-forward state without tracing
            _, _, _, err := api.backend.BlockChain().
                Processor().Process(block, statedb, vm.Config{})
            if err != nil {
                failed = err
                break
            }

            root, err := statedb.Commit(
                api.backend.ChainConfig().IsEIP158(block.Number()),
            )
            if err != nil {
                failed = err
                break
            }
            if err := statedb.Reset(root); err != nil {
                failed = err
                break
            }

            // Reference tries
            database.TrieDB().Reference(root, common.Hash{})
            if number >= origin {
                database.TrieDB().Reference(root, common.Hash{})
            }
            if proot != (common.Hash{}) {
                database.TrieDB().Dereference(proot)
            }
            proot = root
        }
    }()

    // Consumer goroutine to stream results to the client
    go func() {
        done := make(map[uint64]*blockTraceResult)
        next := origin + 1

        for res := range results {
            result := &blockTraceResult{
                Block:  hexutil.Uint64(res.block.NumberU64()),
                Hash:   res.block.Hash(),
                Traces: res.results,
            }
            done[uint64(result.Block)] = result

            // Dereference parent trie root for this task
            database.TrieDB().Dereference(res.rootref)

            for {
                r, ok := done[next]
                if !ok {
                    break
                }
                if len(r.Traces) > 0 || next == end.NumberU64() {
                    notifier.Notify(sub.ID, r)
                }
                delete(done, next)
                next++
            }
        }
    }()

    return sub, nil
}


//
// TraceBlockByNumber / TraceBlockByHash / TraceBlock / TraceBadBlock
//
func (api *PrivateDebugAPI) TraceBlockByNumber(ctx context.Context, number rpc.BlockNumber, config *TraceConfig) ([]*txTraceResult, error) {
	var blk *types.Block
	if number == rpc.PendingBlockNumber && api.backend.Miner() != nil {
		blk = api.backend.Miner().PendingBlock()
	} else if number == rpc.LatestBlockNumber {
		blk = api.backend.CurrentBlock()
	} else {
		blk, _ = api.backend.BlockByNumber(ctx, number)
	}
	if blk == nil {
		return nil, fmt.Errorf("block #%d not found", number)
	}
	return api.traceBlock(ctx, blk, config)
}

func (api *PrivateDebugAPI) TraceBlockByHash(ctx context.Context, hash common.Hash, config *TraceConfig) ([]*txTraceResult, error) {
	blk := api.backend.BlockChain().GetBlockByHash(hash)
	if blk == nil {
		return nil, fmt.Errorf("block %#x not found", hash)
	}
	return api.traceBlock(ctx, blk, config)
}

func (api *PrivateDebugAPI) TraceBlock(ctx context.Context, blob []byte, config *TraceConfig) ([]*txTraceResult, error) {
	blk := new(types.Block)
	if err := rlp.Decode(bytes.NewReader(blob), blk); err != nil {
		return nil, fmt.Errorf("block decode error: %v", err)
	}
	return api.traceBlock(ctx, blk, config)
}

func (api *PrivateDebugAPI) TraceBadBlock(ctx context.Context, hash common.Hash, config *TraceConfig) ([]*txTraceResult, error) {
	for _, blk := range api.backend.BlockChain().BadBlocks() {
		if blk.Hash() == hash {
			return api.traceBlock(ctx, blk, config)
		}
	}
	return nil, fmt.Errorf("bad block %#x not found", hash)
}

//
// TraceCall
//
func (api *PrivateDebugAPI) TraceCall(ctx context.Context, args ethapi.CallArgs, block rpc.BlockNumberOrHash, config *TraceConfig) (interface{}, error) {
	statedb, header, err := api.backend.StateAndHeaderByNumberOrHash(ctx, block)
	if err != nil {
		return nil, err
	}

	msg := args.ToMessage(api.backend.RPCGasCap())
	vmctx := core.NewEVMBlockContext(header, api.backend.BlockChain(), nil)
	return api.traceTx(ctx, msg, vmctx, statedb, config)
}

//
// TraceTransaction
//
func (api *PrivateDebugAPI) TraceTransaction(ctx context.Context, hash common.Hash, config *TraceConfig) (interface{}, error) {
	tx, blockHash, _, index := rawdb.ReadTransaction(api.backend.ChainDb(), hash)
	if tx == nil {
		return nil, fmt.Errorf("tx %#x not found", hash)
	}

	block := api.backend.BlockChain().GetBlockByHash(blockHash)
	if block == nil {
		return nil, fmt.Errorf("block %#x not found", blockHash)
	}

	msg, vmctx, statedb, err := api.computeTxEnv(block, int(index), defaultTraceReexec)
	if err != nil {
		return nil, err
	}

	return api.traceTx(ctx, msg, vmctx, statedb, config)
}

//
// TraceBlock (internal)
//
func (api *PrivateDebugAPI) traceBlock(ctx context.Context, block *types.Block, config *TraceConfig) ([]*txTraceResult, error) {
	if err := api.backend.Engine().VerifyHeader(api.backend.BlockChain(), block.Header(), true); err != nil {
		return nil, err
	}

	parent := api.backend.BlockChain().GetBlock(block.ParentHash(), block.NumberU64()-1)
	if parent == nil {
		return nil, fmt.Errorf("parent %#x not found", block.ParentHash())
	}

	statedb, err := api.computeStateDB(parent, defaultTraceReexec)
	if err != nil {
		return nil, err
	}

	results := make([]*txTraceResult, len(block.Transactions()))
	signer := types.MakeSigner(api.backend.BlockChain().Config(), block.Number())
	blockCtx := core.NewEVMBlockContext(block.Header(), api.backend.BlockChain(), nil)

	for i, tx := range block.Transactions() {
		msg, _ := tx.AsMessage(signer)
		res, err := api.traceTx(ctx, msg, blockCtx, statedb.Copy(), config)
		if err != nil {
			results[i] = &txTraceResult{Error: err.Error()}
			continue
		}
		results[i] = &txTraceResult{Result: res}

		// Fast-forward state
		txCtx := core.NewEVMTxContext(msg)
		vmenv := vm.NewEVM(blockCtx, txCtx, statedb, api.backend.BlockChain().Config(), vm.Config{})
		if _, err := core.ApplyMessage(vmenv, msg, new(core.GasPool).AddGas(msg.Gas())); err != nil {
			return nil, err
		}
		statedb.Finalise(vmenv.ChainConfig().IsEIP158(block.Number()))
	}

	return results, nil
}

//
// StandardTraceBlockToFile
//
func (api *PrivateDebugAPI) StandardTraceBlockToFile(ctx context.Context, hash common.Hash, config *StdTraceConfig) ([]string, error) {
	block := api.backend.BlockChain().GetBlockByHash(hash)
	if block == nil {
		return nil, fmt.Errorf("block %#x not found", hash)
	}

	if config != nil && config.TxHash != (common.Hash{}) {
		found := false
		for _, tx := range block.Transactions() {
			if tx.Hash() == config.TxHash {
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("tx %#x not in block", config.TxHash)
		}
	}

	parent := api.backend.BlockChain().GetBlock(block.ParentHash(), block.NumberU64()-1)
	if parent == nil {
		return nil, fmt.Errorf("parent %#x not found", block.ParentHash())
	}

	reexec := defaultTraceReexec
	if config != nil && config.Reexec != nil {
    reexec = *config.Reexec
	}	
	statedb, err := api.computeStateDB(parent, reexec)
	////
	if err != nil {
		return nil, err
	}

	logCfg := config.LogConfig
	logCfg.Debug = true

	signer := types.MakeSigner(api.backend.BlockChain().Config(), block.Number())
	vmctx := core.NewEVMBlockContext(block.Header(), api.backend.BlockChain(), nil)
	var dumps []string

	for i, tx := range block.Transactions() {
		if config != nil && config.TxHash != (common.Hash{}) && tx.Hash() != config.TxHash {
			continue
		}

		msg, _ := tx.AsMessage(signer)
		txCtx := core.NewEVMTxContext(msg)

		file, err := ioutil.TempFile(os.TempDir(), fmt.Sprintf("trace-%d-%x", i, tx.Hash().Bytes()[:4]))
		if err != nil {
			return nil, err
		}
		dumps = append(dumps, file.Name())

		writer := bufio.NewWriter(file)
		vmConf := vm.Config{Debug: true, Tracer: vm.NewJSONLogger(&logCfg, writer), EnablePreimageRecording: true}
		vmenv := vm.NewEVM(vmctx, txCtx, statedb, api.backend.BlockChain().Config(), vmConf)

		_, err = core.ApplyMessage(vmenv, msg, new(core.GasPool).AddGas(msg.Gas()))
		writer.Flush()
		file.Close()
		if err != nil {
			return dumps, err
		}
		statedb.Finalise(vmenv.ChainConfig().IsEIP158(block.Number()))
	}

	return dumps, nil
}

//
// computeStateDB
//
func (api *PrivateDebugAPI) computeStateDB(block *types.Block, reexec uint64) (*state.StateDB, error) {
    // If we have the state fully available, use that
    statedb, err := api.backend.BlockChain().StateAt(block.Root())
    if err == nil {
        return statedb, nil
    }

    origin := block.NumberU64()
    database := state.NewDatabaseWithConfig(api.backend.ChainDb(), &trie.Config{Cache: 16, Preimages: true})

    // Rewind to find any state we *do* have
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

	start := time.Now()
	var logged time.Time

	for block.NumberU64() < origin {
		block = api.backend.BlockChain().GetBlockByNumber(block.NumberU64() + 1)
		_, _, _, err := api.backend.BlockChain().Processor().Process(block, statedb, vm.Config{})
		if err != nil {
			return nil, err
		}
		root, err := statedb.Commit(api.backend.BlockChain().Config().IsEIP158(block.Number()))
		if err != nil {
			return nil, err
		}
		if err := statedb.Reset(root); err != nil {
			return nil, err
		}
		if time.Since(logged) > 5*time.Second {
			log.Info("regenerating state", "block", block.NumberU64(), "elapsed", time.Since(start))
			logged = time.Now()
		}
	}

	return statedb, nil
}

//
// computeTxEnv
//
func (api *PrivateDebugAPI) computeTxEnv(block *types.Block, index int, reexec uint64) (core.Message, vm.BlockContext, *state.StateDB, error) {
	parent := api.backend.BlockChain().GetBlock(block.ParentHash(), block.NumberU64()-1)
	if parent == nil {
		return nil, vm.BlockContext{}, nil, fmt.Errorf("parent %#x not found", block.ParentHash())
	}

	statedb, err := api.computeStateDB(parent, reexec)
	if err != nil {
		return nil, vm.BlockContext{}, nil, err
	}

	signer := types.MakeSigner(api.backend.BlockChain().Config(), block.Number())
	blockCtx := core.NewEVMBlockContext(block.Header(), api.backend.BlockChain(), nil)

	for i, tx := range block.Transactions() {
		msg, _ := tx.AsMessage(signer)
		if i == index {
			return msg, blockCtx, statedb, nil
		}
		txCtx := core.NewEVMTxContext(msg)
		vmenv := vm.NewEVM(blockCtx, txCtx, statedb, api.backend.BlockChain().Config(), vm.Config{})
		if _, err := core.ApplyMessage(vmenv, msg, new(core.GasPool).AddGas(tx.Gas())); err != nil {
			return nil, vm.BlockContext{}, nil, err
		}
		statedb.Finalise(vmenv.ChainConfig().IsEIP158(block.Number()))
	}

	return nil, vm.BlockContext{}, nil, fmt.Errorf("tx index out of range")
}

//
// traceTx — final formatter
//
func (api *PrivateDebugAPI) traceTx(ctx context.Context, msg core.Message, blockCtx vm.BlockContext, statedb *state.StateDB, config *TraceConfig) (interface{}, error) {
	var tracer vm.Tracer
	var err error

	if config != nil && config.Tracer != nil {
		timeout := defaultTraceTimeout
		if config.Timeout != nil {
			timeout, err = time.ParseDuration(*config.Timeout)
			if err != nil {
				return nil, err
			}
		}
		if tracer, err = tracers.New(*config.Tracer); err != nil {
			return nil, err
		}

		deadline, cancel := context.WithTimeout(ctx, timeout)
		go func() {
			<-deadline.Done()
			tracer.(*tracers.Tracer).Stop(errors.New("execution timeout"))
		}()
		defer cancel()
	} else {
		if config == nil {
			tracer = vm.NewStructLogger(nil)
		} else {
			tracer = vm.NewStructLogger(config.LogConfig)
		}
	}

	txCtx := core.NewEVMTxContext(msg)
	vmenv := vm.NewEVM(blockCtx, txCtx, statedb, api.backend.BlockChain().Config(), vm.Config{Debug: true, Tracer: tracer})

	result, err := core.ApplyMessage(vmenv, msg, new(core.GasPool).AddGas(msg.Gas()))
	if err != nil {
		return nil, err
	}

	switch t := tracer.(type) {
	case *vm.StructLogger:
		ret := fmt.Sprintf("%x", result.Return())
		if len(result.Revert()) > 0 {
			ret = fmt.Sprintf("%x", result.Revert())
		}
		return &ethapi.ExecutionResult{
			Gas:         result.UsedGas,
			Failed:      result.Failed(),
			ReturnValue: ret,
			StructLogs:  ethapi.FormatLogs(t.StructLogs()),
		}, nil
	case *tracers.Tracer:
		return t.GetResult()
	default:
		panic("invalid tracer type")
	}
}

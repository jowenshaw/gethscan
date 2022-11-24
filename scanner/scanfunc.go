package scanner

import (
	"bytes"
	"context"
	"fmt"
	"math/big"
	"strings"
	"sync"
	"time"

	"github.com/anyswap/CrossChain-Bridge/cmd/utils"
	"github.com/anyswap/CrossChain-Bridge/log"
	"github.com/urfave/cli/v2"

	ethclient "github.com/jowenshaw/gethclient"
	"github.com/jowenshaw/gethclient/common"
	"github.com/jowenshaw/gethclient/types"
)

var (
	// ScanFuncCommand scan func calls on eth like blockchain
	ScanFuncCommand = &cli.Command{
		Action:    scanfunc,
		Name:      "scanfunc",
		Usage:     "scan func calls",
		ArgsUsage: " ",
		Description: `
scan func calls
`,
		Flags: []cli.Flag{
			utils.GatewayFlag,
			toAddressFlag,
			funcHashesFlag,
			startHeightFlag,
			utils.EndHeightFlag,
			utils.StableHeightFlag,
			utils.JobsFlag,
			timeoutFlag,
		},
	}
)

type ethFuncScanner struct {
	gateway string

	chainID *big.Int

	endHeight    uint64
	stableHeight uint64
	jobCount     uint64

	processBlockTimeout time.Duration
	processBlockTimers  []*time.Timer

	client *ethclient.Client
	ctx    context.Context

	rpcInterval   time.Duration
	rpcRetryCount int

	cmpTxTo       string
	cmpFuncHashes [][]byte
}

func scanfunc(ctx *cli.Context) error {
	utils.SetLogger(ctx)

	var scanner = &ethFuncScanner{
		ctx:           context.Background(),
		rpcInterval:   1 * time.Second,
		rpcRetryCount: 3,
	}

	scanner.gateway = ctx.String(utils.GatewayFlag.Name)
	startHeightArgument = ctx.Int64(startHeightFlag.Name)
	scanner.endHeight = ctx.Uint64(utils.EndHeightFlag.Name)
	scanner.stableHeight = ctx.Uint64(utils.StableHeightFlag.Name)
	scanner.jobCount = ctx.Uint64(utils.JobsFlag.Name)
	scanner.processBlockTimeout = time.Duration(ctx.Uint64(timeoutFlag.Name)) * time.Second

	scanner.cmpTxTo = ctx.String(toAddressFlag.Name)
	funcHashesStr := strings.TrimSpace(ctx.String(funcHashesFlag.Name))
	funcHashes := strings.Split(funcHashesStr, ",")
	scanner.cmpFuncHashes = make([][]byte, 0, len(funcHashes))
	for _, funcHash := range funcHashes {
		scanner.cmpFuncHashes = append(
			scanner.cmpFuncHashes,
			common.FromHex(funcHash),
		)
	}

	log.Info("get argument success",
		"gateway", scanner.gateway,
		"toAddress", scanner.cmpTxTo,
		"funcHashes", funcHashes,
		"start", startHeightArgument,
		"end", scanner.endHeight,
		"stable", scanner.stableHeight,
		"jobs", scanner.jobCount,
		"timeout", scanner.processBlockTimeout,
	)

	scanner.initClient()
	scanner.run()
	return nil
}

func (scanner *ethFuncScanner) initClient() {
	ethcli, err := ethclient.Dial(scanner.gateway)
	if err != nil {
		log.Fatal("ethclient.Dail failed", "gateway", scanner.gateway, "err", err)
	}
	log.Info("ethclient.Dail gateway success", "gateway", scanner.gateway)
	scanner.client = ethcli
	scanner.chainID, err = ethcli.ChainID(scanner.ctx)
	if err != nil {
		log.Fatal("get chainID failed", "err", err)
	}
	log.Info("get chainID success", "chainID", scanner.chainID)
}

func (scanner *ethFuncScanner) run() {
	scanner.processBlockTimers = make([]*time.Timer, scanner.jobCount+1)
	for i := 0; i < len(scanner.processBlockTimers); i++ {
		scanner.processBlockTimers[i] = time.NewTimer(scanner.processBlockTimeout)
	}

	wend := scanner.endHeight
	if wend == 0 {
		wend = scanner.loopGetLatestBlockNumber()
	}
	if startHeightArgument != 0 {
		var start uint64
		if startHeightArgument > 0 {
			start = uint64(startHeightArgument)
		} else if startHeightArgument < 0 {
			start = wend - uint64(-startHeightArgument)
		}
		scanner.doScanRangeJob(start, wend)
	}
	if scanner.endHeight == 0 {
		scanner.scanLoop(wend)
	}
}

func (scanner *ethFuncScanner) doScanRangeJob(start, end uint64) {
	log.Info("start scan range job", "start", start, "end", end, "jobs", scanner.jobCount)
	if scanner.jobCount == 0 {
		log.Fatal("zero count jobs specified")
	}
	if start >= end {
		log.Fatalf("wrong scan range [%v, %v)", start, end)
	}
	jobs := scanner.jobCount
	count := end - start
	step := count / jobs
	if step == 0 {
		jobs = 1
		step = count
	}
	wg := new(sync.WaitGroup)
	for i := uint64(0); i < jobs; i++ {
		from := start + i*step
		to := start + (i+1)*step
		if i+1 == jobs {
			to = end
		}
		wg.Add(1)
		go scanner.scanRange(i+1, from, to, wg)
	}
	if scanner.endHeight != 0 {
		wg.Wait()
	}
}

func (scanner *ethFuncScanner) scanRange(job, from, to uint64, wg *sync.WaitGroup) {
	defer wg.Done()
	log.Info(fmt.Sprintf("[%v] scan range", job), "from", from, "to", to)

	for h := from; h < to; h++ {
		scanner.scanBlock(job, h, false)
	}

	log.Info(fmt.Sprintf("[%v] scan range finish", job), "from", from, "to", to)
}

func (scanner *ethFuncScanner) scanLoop(from uint64) {
	stable := scanner.stableHeight
	log.Info("start scan loop job", "from", from, "stable", stable)
	for {
		latest := scanner.loopGetLatestBlockNumber()
		for h := from; h <= latest; h++ {
			scanner.scanBlock(0, h, true)
		}
		if from+stable < latest {
			from = latest - stable
		}
		time.Sleep(1 * time.Second)
	}
}

func (scanner *ethFuncScanner) loopGetLatestBlockNumber() uint64 {
	for { // retry until success
		header, err := scanner.client.HeaderByNumber(scanner.ctx, nil)
		if err == nil {
			log.Info("get latest block number success", "height", header.Number)
			return header.Number.Uint64()
		}
		log.Warn("get latest block number failed", "err", err)
		time.Sleep(scanner.rpcInterval)
	}
}

func (scanner *ethFuncScanner) loopGetBlock(height uint64) (block *types.Block, err error) {
	blockNumber := new(big.Int).SetUint64(height)
	for i := 0; i < 3; i++ { // with retry
		block, err = scanner.client.BlockByNumber(scanner.ctx, blockNumber)
		if err == nil {
			return block, nil
		}
		log.Warn("get block failed", "height", height, "err", err)
		time.Sleep(scanner.rpcInterval)
	}
	return nil, err
}

func (scanner *ethFuncScanner) scanBlock(job, height uint64, cache bool) {
	block, err := scanner.loopGetBlock(height)
	if err != nil {
		return
	}
	blockHash := block.Hash().Hex()
	if cache && cachedBlocks.isScanned(blockHash) {
		return
	}
	log.Info(fmt.Sprintf("[%v] scan block %v", job, height), "hash", blockHash, "txs", len(block.Transactions()))

	scanner.processBlockTimers[job].Reset(scanner.processBlockTimeout)
SCANTXS:
	for i, tx := range block.Transactions() {
		select {
		case <-scanner.processBlockTimers[job].C:
			log.Warn(fmt.Sprintf("[%v] scan block %v timeout", job, height), "hash", blockHash, "txs", len(block.Transactions()))
			break SCANTXS
		default:
			log.Debug(fmt.Sprintf("[%v] scan tx in block %v index %v", job, height, i), "tx", tx.Hash().Hex())
			scanner.scanTransaction(tx)
		}
	}
	if cache {
		cachedBlocks.addBlock(blockHash)
	}
}

func (scanner *ethFuncScanner) scanTransaction(tx *types.Transaction) {
	if tx.To() == nil {
		return
	}

	txtoAddress := tx.To().String()
	if !strings.EqualFold(txtoAddress, scanner.cmpTxTo) {
		return
	}

	txInput := tx.Data()
	if len(txInput) < 4 {
		return
	}

	matched := false
	funcHash := txInput[:4]
	for _, cmpHash := range scanner.cmpFuncHashes {
		if bytes.Equal(funcHash, cmpHash) {
			matched = true
			break
		}
	}

	if matched {
		txHash := tx.Hash().Hex()
		fmt.Println("found match tx", txHash)
	}
}

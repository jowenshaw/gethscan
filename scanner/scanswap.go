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
	"github.com/anyswap/CrossChain-Bridge/rpc/client"
	"github.com/anyswap/CrossChain-Bridge/tokens"
	"github.com/anyswap/CrossChain-Bridge/tokens/tools"
	"github.com/fsn-dev/fsn-go-sdk/efsn/common"
	"github.com/fsn-dev/fsn-go-sdk/efsn/core/types"
	"github.com/fsn-dev/fsn-go-sdk/efsn/ethclient"
	"github.com/jowenshaw/gethscan/params"
	"github.com/urfave/cli/v2"
)

var (
	// VerbosityFlag --verbosity
	VerbosityFlag = &cli.Uint64Flag{
		Name:  "verbosity",
		Usage: "0:panic, 1:fatal, 2:error, 3:warn, 4:info, 5:debug, 6:trace",
		Value: 4,
	}

	scanReceiptFlag = &cli.BoolFlag{
		Name:  "scanReceipt",
		Usage: "scan transaction receipt instead of transaction",
	}

	// ScanSwapCommand scan swaps on eth like blockchain
	ScanSwapCommand = &cli.Command{
		Action:    scanSwap,
		Name:      "scanswap",
		Usage:     "scan cross chain swaps",
		ArgsUsage: " ",
		Description: `
scan cross chain swaps
`,
		Flags: []cli.Flag{
			utils.ConfigFileFlag,
			utils.GatewayFlag,
			scanReceiptFlag,
			utils.StartHeightFlag,
			utils.EndHeightFlag,
			utils.StableHeightFlag,
			utils.JobsFlag,
		},
	}

	transferFuncHash       = common.FromHex("0xa9059cbb")
	transferFromFuncHash   = common.FromHex("0x23b872dd")
	stringSwapoutFuncHash  = common.FromHex("0xad54056d")
	addressSwapoutFuncHash = common.FromHex("0x628d6cba")
)

type ethSwapScanner struct {
	gateway     string
	scanReceipt bool

	startHeight  uint64
	endHeight    uint64
	stableHeight uint64
	jobCount     uint64

	client *ethclient.Client
	ctx    context.Context

	rpcInterval   time.Duration
	rpcRetryCount int
}

// SetLogger set logger
func SetLogger(ctx *cli.Context) {
	logLevel := ctx.Uint64(VerbosityFlag.Name)
	jsonFormat := ctx.Bool(utils.JSONFormatFlag.Name)
	colorFormat := ctx.Bool(utils.ColorFormatFlag.Name)
	log.SetLogger(uint32(logLevel), jsonFormat, colorFormat)
}

func scanSwap(ctx *cli.Context) error {
	SetLogger(ctx)
	params.LoadConfig(utils.GetConfigFilePath(ctx))
	go params.WatchAndReloadScanConfig()

	scanner := &ethSwapScanner{
		ctx:           context.Background(),
		rpcInterval:   3 * time.Second,
		rpcRetryCount: 3,
	}
	scanner.gateway = ctx.String(utils.GatewayFlag.Name)
	scanner.scanReceipt = ctx.Bool(scanReceiptFlag.Name)
	scanner.startHeight = ctx.Uint64(utils.StartHeightFlag.Name)
	scanner.endHeight = ctx.Uint64(utils.EndHeightFlag.Name)
	scanner.stableHeight = ctx.Uint64(utils.StableHeightFlag.Name)
	scanner.jobCount = ctx.Uint64(utils.JobsFlag.Name)

	log.Info("get argument success",
		"gateway", scanner.gateway,
		"scanReceipt", scanner.scanReceipt,
		"start", scanner.startHeight,
		"end", scanner.endHeight,
		"stable", scanner.stableHeight,
		"jobs", scanner.jobCount,
	)

	scanner.verifyOptions()
	scanner.initClient()
	scanner.run()
	return nil
}

func (scanner *ethSwapScanner) verifyOptions() {
	if scanner.endHeight != 0 && scanner.startHeight >= scanner.endHeight {
		log.Fatalf("wrong scan range [%v, %v)", scanner.startHeight, scanner.endHeight)
	}
	if scanner.jobCount == 0 {
		log.Fatal("zero count jobs specified")
	}
}

func (scanner *ethSwapScanner) initClient() {
	ethcli, err := ethclient.Dial(scanner.gateway)
	if err != nil {
		log.Fatal("ethclient.Dail failed", "gateway", scanner.gateway, "err", err)
	}
	log.Info("ethclient.Dail gateway success", "gateway", scanner.gateway)
	scanner.client = ethcli
}

func (scanner *ethSwapScanner) run() {
	start := scanner.startHeight
	wend := scanner.endHeight
	if wend == 0 {
		wend = scanner.loopGetLatestBlockNumber()
	}
	if start == 0 {
		start = wend
	}

	scanner.doScanRangeJob(start, wend)

	if scanner.endHeight == 0 {
		scanner.scanLoop(wend)
	}
}

func (scanner *ethSwapScanner) doScanRangeJob(start, end uint64) {
	if start >= end {
		return
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

func (scanner *ethSwapScanner) scanRange(job, from, to uint64, wg *sync.WaitGroup) {
	defer wg.Done()
	log.Info(fmt.Sprintf("[%v] start scan range", job), "from", from, "to", to)

	for h := from; h < to; h++ {
		scanner.scanBlock(job, h, false)
	}

	log.Info(fmt.Sprintf("[%v] scan range finish", job), "from", from, "to", to)
}

func (scanner *ethSwapScanner) scanLoop(from uint64) {
	stable := scanner.stableHeight
	log.Info("start scan loop", "from", from, "stable", stable)
	for {
		latest := scanner.loopGetLatestBlockNumber()
		for h := latest; h > from; h-- {
			scanner.scanBlock(0, h, true)
		}
		if from+stable < latest {
			from = latest - stable
		}
		time.Sleep(5 * time.Second)
	}
}

func (scanner *ethSwapScanner) loopGetLatestBlockNumber() uint64 {
	for {
		header, err := scanner.client.HeaderByNumber(scanner.ctx, nil)
		if err == nil {
			log.Info("get latest block number success", "height", header.Number)
			return header.Number.Uint64()
		}
		log.Warn("get latest block number failed", "err", err)
		time.Sleep(scanner.rpcInterval)
	}
}

func (scanner *ethSwapScanner) getTxReceipt(txHash common.Hash) (*types.Receipt, error) {
	return scanner.client.TransactionReceipt(scanner.ctx, txHash)
}

func (scanner *ethSwapScanner) loopGetBlock(height uint64) *types.Block {
	blockNumber := new(big.Int).SetUint64(height)
	for {
		block, err := scanner.client.BlockByNumber(scanner.ctx, blockNumber)
		if err == nil {
			return block
		}
		log.Warn("get block failed", "height", height, "err", err)
		time.Sleep(scanner.rpcInterval)
	}
}

func (scanner *ethSwapScanner) scanBlock(job, height uint64, cache bool) {
	block := scanner.loopGetBlock(height)
	blockHash := block.Hash().Hex()
	if cache && cachedBlocks.isScanned(blockHash) {
		return
	}
	log.Info(fmt.Sprintf("[%v] scan block %v", job, height), "hash", blockHash, "txs", len(block.Transactions()))
	for _, tx := range block.Transactions() {
		scanner.scanTransaction(tx)
	}
	if cache {
		cachedBlocks.addBlock(blockHash)
	}
}

func (scanner *ethSwapScanner) scanTransaction(tx *types.Transaction) {
	if tx.To() == nil {
		return
	}
	var (
		txTo   = tx.To().Hex()
		txHash = tx.Hash().Hex()

		selTokenCfg *params.TokenConfig
		receipt     *types.Receipt
		verifyErr   error
	)

	if scanner.scanReceipt {
		r, err := scanner.getTxReceipt(tx.Hash())
		if err != nil {
			log.Warn("get tx receipt error", "txHash", txHash, "err", err)
			return
		}
		receipt = r
	}

	for _, tokenCfg := range params.GetScanConfig().Tokens {
		tokenAddress := tokenCfg.TokenAddress
		depositAddress := tokenCfg.DepositAddress
		switch {
		case depositAddress != "":
			if tokenCfg.IsNativeToken() {
				if strings.EqualFold(txTo, depositAddress) {
					selTokenCfg = tokenCfg
					break
				}
			} else if strings.EqualFold(txTo, tokenAddress) {
				selTokenCfg = tokenCfg
				verifyErr = scanner.verifyErc20SwapinTx(tx, receipt, tokenCfg)
				break
			}
		case !scanner.scanReceipt:
			if strings.EqualFold(txTo, tokenAddress) {
				selTokenCfg = tokenCfg
				verifyErr = scanner.verifySwapoutTx(tx, receipt, tokenCfg)
				break
			}
		default:
			err := parseSwapoutTxLogs(receipt.Logs, tokenAddress, tokenCfg.LogTopics)
			if err == nil {
				selTokenCfg = tokenCfg
				break
			}
		}
	}
	if selTokenCfg != nil {
		if tokens.ShouldRegisterSwapForError(verifyErr) {
			scanner.postSwap(txHash, selTokenCfg)
		} else {
			log.Debug("verify swap error", "txHash", txHash, "err", verifyErr)
		}
	}
}

func (scanner *ethSwapScanner) postSwap(txid string, tokenCfg *params.TokenConfig) {
	swapServer := tokenCfg.SwapServer
	pairID := tokenCfg.PairID
	var subject, rpcMethod string
	if tokenCfg.DepositAddress != "" {
		subject = "post swapin register"
		rpcMethod = "swap.Swapin"
	} else {
		subject = "post swapout register"
		rpcMethod = "swap.Swapout"
	}
	log.Info(subject, "txid", txid, "pairID", pairID)

	var result interface{}
	args := map[string]interface{}{
		"txid":   txid,
		"pairid": pairID,
	}
	for i := 0; i < scanner.rpcRetryCount; i++ {
		err := client.RPCPost(&result, swapServer, rpcMethod, args)
		if tokens.ShouldRegisterSwapForError(err) {
			break
		}
		if tools.IsSwapAlreadyExistRegisterError(err) {
			break
		}
		log.Warn(subject+" failed", "txid", txid, "pairID", pairID, "err", err)
	}
}

func (scanner *ethSwapScanner) verifyErc20SwapinTx(tx *types.Transaction, receipt *types.Receipt, tokenCfg *params.TokenConfig) (err error) {
	if receipt == nil {
		err = parseErc20SwapinTxInput(tx.Data(), tokenCfg.DepositAddress)
	} else {
		err = parseErc20SwapinTxLogs(receipt.Logs, tokenCfg.TokenAddress, tokenCfg.DepositAddress, tokenCfg.LogTopics)
	}
	return err
}

func (scanner *ethSwapScanner) verifySwapoutTx(tx *types.Transaction, receipt *types.Receipt, tokenCfg *params.TokenConfig) (err error) {
	if receipt == nil {
		err = parseSwapoutTxInput(tx.Data())
	} else {
		err = parseSwapoutTxLogs(receipt.Logs, tokenCfg.TokenAddress, tokenCfg.LogTopics)
	}
	return err
}

func parseErc20SwapinTxInput(input []byte, depositAddress string) error {
	if len(input) < 4 {
		return tokens.ErrTxWithWrongInput
	}
	var receiver string
	funcHash := input[:4]
	switch {
	case bytes.Equal(funcHash, transferFuncHash):
		receiver = common.BytesToAddress(common.GetData(input, 4, 32)).Hex()
	case bytes.Equal(funcHash, transferFromFuncHash):
		receiver = common.BytesToAddress(common.GetData(input, 36, 32)).Hex()
	default:
		return tokens.ErrTxFuncHashMismatch
	}
	if !strings.EqualFold(receiver, depositAddress) {
		return tokens.ErrTxWithWrongReceiver
	}
	return nil
}

func parseErc20SwapinTxLogs(logs []*types.Log, targetContract, depositAddress string, logSwapTopics []string) (err error) {
	for _, log := range logs {
		if log.Removed {
			continue
		}
		if !strings.EqualFold(log.Address.Hex(), targetContract) {
			continue
		}
		if len(log.Topics) != 3 || log.Data == nil {
			continue
		}
		logTopic := log.Topics[0].Hex()
		for _, topic := range logSwapTopics {
			if logTopic == topic {
				receiver := common.BytesToAddress(log.Topics[2][:]).Hex()
				if strings.EqualFold(receiver, depositAddress) {
					return nil
				}
				return tokens.ErrTxWithWrongReceiver
			}
		}
	}
	return tokens.ErrDepositLogNotFound
}

func parseSwapoutTxInput(input []byte) error {
	if len(input) < 4 {
		return tokens.ErrTxWithWrongInput
	}
	funcHash := input[:4]
	switch {
	case bytes.Equal(funcHash, addressSwapoutFuncHash):
	case bytes.Equal(funcHash, stringSwapoutFuncHash):
	default:
		return tokens.ErrTxFuncHashMismatch
	}
	return nil
}

func parseSwapoutTxLogs(logs []*types.Log, targetContract string, logSwapTopics []string) (err error) {
	for _, log := range logs {
		if log.Removed {
			continue
		}
		if !strings.EqualFold(log.Address.Hex(), targetContract) {
			continue
		}
		if len(log.Topics) != 3 || log.Data == nil {
			continue
		}
		logTopic := log.Topics[0].Hex()
		for _, topic := range logSwapTopics {
			if logTopic == topic {
				return nil
			}
		}
	}
	return tokens.ErrSwapoutLogNotFound
}

type cachedSacnnedBlocks struct {
	capacity  int
	nextIndex int
	hashes    []string
}

var cachedBlocks = &cachedSacnnedBlocks{
	capacity:  100,
	nextIndex: 0,
	hashes:    make([]string, 100),
}

func (cache *cachedSacnnedBlocks) addBlock(blockHash string) {
	cache.hashes[cache.nextIndex] = blockHash
	cache.nextIndex = (cache.nextIndex + 1) % cache.capacity
}

func (cache *cachedSacnnedBlocks) isScanned(blockHash string) bool {
	for _, b := range cache.hashes {
		if b == blockHash {
			return true
		}
	}
	return false
}

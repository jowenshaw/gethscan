package params

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/anyswap/CrossChain-Bridge/common"
	"github.com/anyswap/CrossChain-Bridge/log"
)

// swap tx types
const (
	TxSwapin   = "swapin"
	TxSwapout  = "swapout"
	TxSwapout2 = "swapout2"
)

var (
	configFile string
	scanConfig = &ScanConfig{}
)

// ScanConfig scan config
type ScanConfig struct {
	Tokens []*TokenConfig
}

// TokenConfig token config
type TokenConfig struct {
	TxType         string
	PairID         string
	SwapServer     string
	CallByContract string `toml:",omitempty" json:",omitempty"`
	TokenAddress   string
	DepositAddress string `toml:",omitempty" json:",omitempty"`
}

// IsNativeToken is native token
func (c *TokenConfig) IsNativeToken() bool {
	return c.TokenAddress == "native"
}

// GetScanConfig get scan config
func GetScanConfig() *ScanConfig {
	return scanConfig
}

// LoadConfig load config
func LoadConfig(filePath string) *ScanConfig {
	log.Println("LoadConfig Config file is", filePath)
	if !common.FileExist(filePath) {
		log.Fatalf("LoadConfig error: config file '%v' not exist", filePath)
	}

	config := &ScanConfig{}
	if _, err := toml.DecodeFile(filePath, &config); err != nil {
		log.Fatalf("LoadConfig error (toml DecodeFile): %v", err)
	}

	var bs []byte
	if log.JSONFormat {
		bs, _ = json.Marshal(config)
	} else {
		bs, _ = json.MarshalIndent(config, "", "  ")
	}
	log.Println("LoadConfig finished.", string(bs))

	if err := config.CheckConfig(); err != nil {
		log.Fatalf("LoadConfig Check config failed. %v", err)
	}

	configFile = filePath // init config file path
	scanConfig = config   // init scan config
	return scanConfig
}

// ReloadConfig reload config
func ReloadConfig() {
	log.Println("ReloadConfig Config file is", configFile)
	if !common.FileExist(configFile) {
		log.Errorf("ReloadConfig error: config file '%v' not exist", configFile)
		return
	}

	config := &ScanConfig{}
	if _, err := toml.DecodeFile(configFile, &config); err != nil {
		log.Errorf("ReloadConfig error (toml DecodeFile): %v", err)
		return
	}

	if err := config.CheckConfig(); err != nil {
		log.Errorf("ReloadConfig Check config failed. %v", err)
		return
	}
	log.Println("ReloadConfig success.")
	scanConfig = config // reassign scan config
}

// CheckConfig check scan config
func (c *ScanConfig) CheckConfig() (err error) {
	if len(c.Tokens) == 0 {
		return errors.New("no token config exist")
	}
	pairIDMap := make(map[string]struct{})
	tokensMap := make(map[string]struct{})
	exist := false
	for _, tokenCfg := range c.Tokens {
		err = tokenCfg.CheckConfig()
		if err != nil {
			return err
		}
		pairIDKey := strings.ToLower(fmt.Sprintf("%v:%v:%v:%v", tokenCfg.TokenAddress, tokenCfg.PairID, tokenCfg.TxType, tokenCfg.SwapServer))
		if _, exist = pairIDMap[pairIDKey]; exist {
			return errors.New("duplicate pairID config" + pairIDKey)
		}
		pairIDMap[pairIDKey] = struct{}{}
		if !tokenCfg.IsNativeToken() {
			tokensKey := strings.ToLower(fmt.Sprintf("%v:%v", tokenCfg.TokenAddress, tokenCfg.DepositAddress))
			if _, exist = tokensMap[tokensKey]; exist {
				return errors.New("duplicate token config " + tokensKey)
			}
			tokensMap[tokensKey] = struct{}{}
		}
	}
	return nil
}

// CheckConfig check token config
func (c *TokenConfig) CheckConfig() error {
	if c.TxType == "" {
		return errors.New("empty 'TxType'")
	}
	if c.PairID == "" {
		return errors.New("empty 'PairID'")
	}
	if c.SwapServer == "" {
		return errors.New("empty 'SwapServer'")
	}
	if c.CallByContract != "" && !common.IsHexAddress(c.CallByContract) {
		return errors.New("wrong 'CallByContract' " + c.CallByContract)
	}
	if c.TxType == TxSwapin && c.CallByContract != "" && c.TokenAddress == "" {
		c.TokenAddress = c.CallByContract // assign token address for swapin if empty
	}
	if !c.IsNativeToken() && !common.IsHexAddress(c.TokenAddress) {
		return errors.New("wrong 'TokenAddress' " + c.TokenAddress)
	}
	if c.DepositAddress != "" && !common.IsHexAddress(c.DepositAddress) {
		return errors.New("wrong 'DepositAddress' " + c.DepositAddress)
	}
	return nil
}

package params

import (
	"encoding/json"
	"errors"

	"github.com/BurntSushi/toml"
	"github.com/anyswap/CrossChain-Bridge/common"
	"github.com/anyswap/CrossChain-Bridge/log"
)

var scanConfig = &ScanConfig{}

// ScanConfig scan config
type ScanConfig struct {
	Tokens []*TokenConfig
}

// TokenConfig token config
type TokenConfig struct {
	PairID         string
	SwapServer     string
	TokenAddress   string
	DepositAddress string   `toml:",omitempty" json:",omitempty"`
	LogTopics      []string `toml:",omitempty" json:",omitempty"`
}

// IsNativeToken is native token
func (c *TokenConfig) IsNativeToken() bool {
	return c.TokenAddress == "native"
}

// CheckConfig check token config
func (c *TokenConfig) CheckConfig() error {
	return nil // TODO
}

// GetScanConfig get scan config
func GetScanConfig() *ScanConfig {
	return scanConfig
}

// LoadConfig load config
func LoadConfig(configFile string) *ScanConfig {
	log.Println("Config file is", configFile)
	if !common.FileExist(configFile) {
		log.Fatalf("LoadConfig error: config file '%v' not exist", configFile)
	}
	config := &ScanConfig{}
	if _, err := toml.DecodeFile(configFile, &config); err != nil {
		log.Fatalf("LoadConfig error (toml DecodeFile): %v", err)
	}
	scanConfig = config // init scan config

	var bs []byte
	if log.JSONFormat {
		bs, _ = json.Marshal(config)
	} else {
		bs, _ = json.MarshalIndent(config, "", "  ")
	}
	log.Println("LoadConfig finished.", string(bs))

	if err := CheckConfig(); err != nil {
		log.Fatalf("Check config failed. %v", err)
	}
	return scanConfig
}

// CheckConfig check config
func CheckConfig() (err error) {
	if len(scanConfig.Tokens) == 0 {
		return errors.New("no token config exist")
	}
	for _, tokenCfg := range scanConfig.Tokens {
		err = tokenCfg.CheckConfig()
		if err != nil {
			return err
		}
	}
	return nil
}

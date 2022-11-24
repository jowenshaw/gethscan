package scanner

import (
	"github.com/urfave/cli/v2"
)

var (
	scanReceiptFlag = &cli.BoolFlag{
		Name:  "scanReceipt",
		Usage: "scan transaction receipt instead of transaction",
	}

	startHeightFlag = &cli.Int64Flag{
		Name:  "start",
		Usage: "start height (start inclusive)",
		Value: -200,
	}

	timeoutFlag = &cli.Uint64Flag{
		Name:  "timeout",
		Usage: "timeout of scanning one block in seconds",
		Value: 300,
	}

	toAddressFlag = &cli.StringFlag{
		Name:  "txto",
		Usage: "tx to address",
	}

	funcHashesFlag = &cli.StringFlag{
		Name:  "funcHashes",
		Usage: "func hashes (comma separated)",
	}
)

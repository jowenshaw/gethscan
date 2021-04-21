package main

import (
	"fmt"
	"os"

	"github.com/anyswap/CrossChain-Bridge/cmd/utils"
	"github.com/anyswap/CrossChain-Bridge/log"
	"github.com/jowenshaw/gethscan/params"
	"github.com/jowenshaw/gethscan/scanner"
	"github.com/urfave/cli/v2"
)

var (
	// The app that holds all commands and flags.
	app = cli.NewApp()
)

func initApp() {
	// Initialize the CLI app and start action
	app.Action = run
	app.Version = params.Version
	app.Usage = "scan eth like blockchain"
	app.Commands = []*cli.Command{
		scanner.ScanSwapCommand,
	}
	app.Flags = []cli.Flag{
		scanner.VerbosityFlag,
		utils.JSONFormatFlag,
		utils.ColorFormatFlag,
	}
}

func main() {
	initApp()
	if err := app.Run(os.Args); err != nil {
		log.Println(err)
		os.Exit(1)
	}
}

func run(ctx *cli.Context) error {
	scanner.SetLogger(ctx)
	if ctx.NArg() > 0 {
		return fmt.Errorf("invalid command: %q", ctx.Args().Get(0))
	}

	_ = cli.ShowAppHelp(ctx)
	fmt.Println()
	log.Fatalf("please specify a sub command to run")
	return nil
}

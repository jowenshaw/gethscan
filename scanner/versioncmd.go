package scanner

import (
	"fmt"
	"os"
	"runtime"

	"github.com/jowenshaw/gethscan/params"
	"github.com/urfave/cli/v2"
)

var (
	// VersionCommand version subcommand
	VersionCommand = &cli.Command{
		Action:    version,
		Name:      "version",
		Usage:     "Print version numbers",
		ArgsUsage: " ",
		Description: `
The output of this command is supposed to be machine-readable.
`,
	}
)

func version(ctx *cli.Context) error {
	fmt.Println("gethscan")
	fmt.Println("Version:", params.Version)
	fmt.Println("Architecture:", runtime.GOARCH)
	fmt.Println("Go Version:", runtime.Version())
	fmt.Println("Operating System:", runtime.GOOS)
	fmt.Printf("GOPATH=%s\n", os.Getenv("GOPATH"))
	fmt.Printf("GOROOT=%s\n", runtime.GOROOT())
	return nil
}

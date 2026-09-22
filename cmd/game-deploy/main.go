package main

import (
	"os"

	"github.com/neko233-com/game-deploy-cli/internal/cli"
)

func main() {
	os.Exit((cli.Runner{}).Run(os.Args[1:]))
}

package main

import (
	"os"

	"github.com/jmattheis/goverter/cli"
)

//go:generate go install .
func main() {
	cli.Run(os.Args, cli.RunOpts{})
}

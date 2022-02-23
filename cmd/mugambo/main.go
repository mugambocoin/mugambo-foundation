package main

import (
	"fmt"
	"os"

	"github.com/mugambocoin/mugambo-foundation/cmd/mugambo/launcher"
)

func main() {
	if err := launcher.Launch(os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

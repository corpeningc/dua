package main

import (
	"fmt"
	"os"

	"github.com/corpeningc/dua/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		fmt.Println(os.Stderr, err)
		os.Exit(1)
	}
}